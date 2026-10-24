package azure

import (
	"context"
	"errors"
	"time"

	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebusexplorer/internal/explorer"
)

// replaySender permits queue destinations in the connected namespace. Only the
// legacy receive-next action can explicitly route back to its source topic.
func (s *Store) replaySender(source explorer.Resource, legacy bool) func(context.Context, explorer.Resource) (deliverySender, error) {
	return func(ctx context.Context, destination explorer.Resource) (deliverySender, error) {
		if !validResource(destination) || destination.Profile != source.Profile || destination.Namespace != source.Namespace ||
			(destination.Queue == "" && (!legacy || destination != source)) {
			return nil, errors.New("destination must be an existing queue in the connected namespace")
		}
		if err := s.validateSource(ctx, destination); err != nil {
			return nil, err
		}
		entity := destination.Queue
		if entity == "" {
			entity = destination.Topic
		}
		return s.client.NewSender(entity, nil)
	}
}

func (s *Store) PrepareDLQCopy(ctx context.Context, source explorer.Resource, sequence int64) (explorer.Replay, error) {
	if sequence < 1 || !validResource(source) {
		return nil, errors.New("invalid selected DLQ identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.validateSource(ctx, source); err != nil {
		return nil, err
	}
	receiver, err := s.dlqReceiver(source)
	if err != nil {
		return nil, errors.New("cannot open selected DLQ peek receiver")
	}
	defer closeReceiver(context.Background(), receiver)
	messages, err := receiver.PeekMessages(ctx, 1, &sb.PeekMessagesOptions{FromSequenceNumber: &sequence})
	if err != nil {
		return nil, errors.New("cannot prepare selected DLQ copy")
	}
	if len(messages) != 1 || messages[0].SequenceNumber == nil || *messages[0].SequenceNumber != sequence {
		return nil, errors.New("selected DLQ message is no longer available; no replacement was selected")
	}
	// The SDK message owns the complete body and properties. No display preview
	// participates in outbound conversion, and no receive or lock was acquired.
	return struct{ explorer.Replay }{&delivery{message: messages[0], copyOnly: true, newSender: s.replaySender(source, false)}}, nil
}

func (s *Store) ReceiveSelectedDLQ(ctx context.Context, source explorer.Resource, sequence int64) (explorer.Delivery, error) {
	if sequence < 1 || !validResource(source) {
		return nil, errors.New("invalid selected DLQ identity")
	}
	s.mu.Lock()
	if s.held {
		s.mu.Unlock()
		return nil, errors.New("a DLQ delivery is already held or being acquired")
	}
	s.held = true
	s.mu.Unlock()
	release := func() { s.mu.Lock(); s.held = false; s.mu.Unlock() }
	owned := false
	defer func() {
		if !owned {
			release()
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.validateSource(ctx, source); err != nil {
		return nil, err
	}
	receiver, err := s.dlqReceiver(source)
	if err != nil {
		return nil, errors.New("cannot open selected DLQ receiver")
	}
	message, err := acquireSelected(ctx, receiver, sequence)
	if err != nil {
		return nil, err
	}
	owned = true
	return &delivery{message: message, receiver: receiver, release: release, newSender: s.replaySender(source, false)}, nil
}

type selectedReceiver interface {
	deliveryReceiver
	ReceiveMessages(context.Context, int, *sb.ReceiveMessagesOptions) ([]*sb.ReceivedMessage, error)
}

// Nonmatches stay locked until search ends, preventing an abandoned first
// message from being received repeatedly. Never settle a nonmatch as complete.
func acquireSelected(ctx context.Context, receiver selectedReceiver, sequence int64) (*sb.ReceivedMessage, error) {
	var nonmatches []*sb.ReceivedMessage
	var target *sb.ReceivedMessage
	for range 100 {
		if ctx.Err() != nil {
			break
		}
		messages, err := receiver.ReceiveMessages(ctx, 1, nil)
		for _, message := range messages {
			if message.SequenceNumber != nil && *message.SequenceNumber == sequence && target == nil {
				target = message
			} else {
				nonmatches = append(nonmatches, message)
			}
		}
		if target != nil || err != nil || len(messages) == 0 {
			break
		}
	}
	cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	uncertain := false
	for _, message := range nonmatches {
		if err := receiver.AbandonMessage(cleanup, message, nil); err != nil {
			uncertain = true
		}
	}
	if target != nil && !uncertain && ctx.Err() == nil && value(target.LockedUntil).After(time.Now()) {
		return target, nil
	}
	if target != nil {
		if err := receiver.AbandonMessage(cleanup, target, nil); err != nil {
			uncertain = true
		}
	}
	closeReceiver(context.Background(), receiver)
	if uncertain {
		return nil, errors.New("DLQ lock release unconfirmed; no replay prepared; locks may remain until expiry")
	}
	return nil, errors.New("could not acquire selected DLQ message within 100 receives / 5 seconds; no replacement was selected")
}

// Package azure implements Service Bus browsing and reviewed DLQ delivery actions
// against only the fixed local emulator. There is no cloud fallback.
package azure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"servicebusexplorer/internal/explorer"
)

type Store struct {
	client *sb.Client
	admin  *admin.Client
	mu     sync.Mutex
	held   bool
}

func NewEmulator() (*Store, error) {
	client, err := sb.NewClientFromConnectionString("Endpoint=sb://"+MessagingAddress+emulatorCredentials,
		&sb.ClientOptions{RetryOptions: sb.RetryOptions{MaxRetries: -1}})
	if err != nil {
		return nil, errors.New("cannot initialize emulator messaging client")
	}
	management, err := admin.NewClientFromConnectionString("Endpoint=sb://"+managementAddress+emulatorCredentials,
		&admin.ClientOptions{ClientOptions: azcore.ClientOptions{
			Transport: newEmulatorTransport(), Retry: policy.RetryOptions{MaxRetries: -1},
		}})
	if err != nil {
		return nil, errors.New("cannot initialize emulator administration client")
	}
	return &Store{client: client, admin: management}, nil
}

func (s *Store) Discover(ctx context.Context) ([]explorer.Resource, error) {
	var resources []explorer.Resource
	queues := s.admin.NewListQueuesPager(nil)
	for queues.More() {
		page, err := queues.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover emulator queues: %w", err)
		}
		for _, q := range page.Queues {
			resources = append(resources, resource(q.QueueName, "", ""))
		}
	}
	topics := s.admin.NewListTopicsPager(nil)
	for topics.More() {
		page, err := topics.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover emulator topics: %w", err)
		}
		for _, topic := range page.Topics {
			subscriptions := s.admin.NewListSubscriptionsPager(topic.TopicName, nil)
			for subscriptions.More() {
				page, err := subscriptions.NextPage(ctx)
				if err != nil {
					return nil, fmt.Errorf("discover emulator subscriptions: %w", err)
				}
				for _, sub := range page.Subscriptions {
					resources = append(resources, resource("", topic.TopicName, sub.SubscriptionName))
				}
			}
		}
	}
	return resources, nil
}

func resource(queue, topic, subscription string) explorer.Resource {
	return explorer.Resource{Profile: "emulator", Namespace: MessagingAddress, Queue: queue, Topic: topic, Subscription: subscription}
}

func validResource(r explorer.Resource) bool {
	return r.Profile == "emulator" && r.Namespace == MessagingAddress &&
		((validEntity(r.Queue) && r.Topic == "" && r.Subscription == "") || (r.Queue == "" && validEntity(r.Topic) && validEntity(r.Subscription)))
}

// Entity names are path segments, never URLs or management/subqueue paths.
func validEntity(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\?#$%:\x00\r\n")
}

func (*Store) Counts(ctx context.Context, r explorer.Resource) (explorer.Counts, error) {
	if err := ctx.Err(); err != nil {
		return explorer.Counts{}, err
	}
	if !validResource(r) {
		return explorer.Counts{}, errors.New("invalid emulator resource identity")
	}
	// Emulator 2.0.1 omits CountDetails. Some SDKs decode that as false zeros.
	// Never infer authoritative totals from a bounded peek or seed expectations.
	return explorer.Counts{State: "unavailable", Detail: "Emulator 2.0.1 does not supply reliable runtime counts"}, nil
}

func (s *Store) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	if !validResource(r) || from < 0 || limit < 1 || limit > 20 {
		return explorer.Page{}, errors.New("invalid emulator resource, sequence or page limit")
	}
	options := &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock}
	if dlq {
		options.SubQueue = sb.SubQueueDeadLetter
	}
	var receiver *sb.Receiver
	var err error
	if r.Queue != "" {
		receiver, err = s.client.NewReceiverForQueue(r.Queue, options)
	} else {
		receiver, err = s.client.NewReceiverForSubscription(r.Topic, r.Subscription, options)
	}
	if err != nil {
		return explorer.Page{}, fmt.Errorf("open peek receiver: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = receiver.Close(closeCtx)
	}()
	// Always pass the explicit cursor. An extra peek result detects a next page;
	// it is still browsing, never receiving or changing a message's delivery count.
	messages, err := receiver.PeekMessages(ctx, limit+1, &sb.PeekMessagesOptions{FromSequenceNumber: &from})
	if err != nil {
		return explorer.Page{}, fmt.Errorf("peek emulator messages: %w", err)
	}
	var page explorer.Page
	if len(messages) > limit {
		next := *messages[limit-1].SequenceNumber + 1
		page.NextSequence = &next
		messages = messages[:limit]
	}
	for _, message := range messages {
		page.Messages = append(page.Messages, (&delivery{message: message}).Snapshot())
	}
	return page, nil
}

func value[T any](p *T) (zero T) {
	if p != nil {
		return *p
	}
	return zero
}

func (s *Store) ReceiveNextDLQ(ctx context.Context, r explorer.Resource) (explorer.Delivery, error) {
	if !validResource(r) {
		return nil, errors.New("invalid emulator resource identity")
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
	if err := s.validateSource(ctx, r); err != nil {
		return nil, err
	}
	receiver, err := s.dlqReceiver(r)
	if err != nil {
		return nil, errors.New("cannot open DLQ receiver")
	}
	messages, err := receiver.ReceiveMessages(ctx, 1, nil)
	if err != nil || len(messages) != 1 {
		closeReceiver(context.Background(), receiver)
		return nil, errors.New("no DLQ delivery acquired within receive deadline")
	}
	owned = true
	return &delivery{message: messages[0], receiver: receiver, release: release,
		newSender: s.replaySender(r, true)}, nil
}

func (s *Store) validateSource(ctx context.Context, r explorer.Resource) error {
	if !validResource(r) {
		return errors.New("invalid emulator resource identity")
	}
	// Check the actual entity before opening a non-session receiver. Failure to
	// inspect it is not permission to attempt an unsupported session mutation.
	var sessions bool
	if r.Queue != "" {
		q, err := s.admin.GetQueue(ctx, r.Queue, nil)
		if err != nil || q == nil {
			return errors.New("cannot validate queue for DLQ mutation")
		}
		sessions = value(q.RequiresSession)
	} else {
		sub, err := s.admin.GetSubscription(ctx, r.Topic, r.Subscription, nil)
		if err != nil || sub == nil {
			return errors.New("cannot validate subscription for DLQ mutation")
		}
		sessions = value(sub.RequiresSession)
	}
	if sessions {
		return errors.New("DLQ mutations on session-enabled entities are unsupported")
	}
	return nil
}

func (s *Store) dlqReceiver(r explorer.Resource) (*sb.Receiver, error) {
	options := &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock, SubQueue: sb.SubQueueDeadLetter}
	if r.Queue != "" {
		return s.client.NewReceiverForQueue(r.Queue, options)
	}
	return s.client.NewReceiverForSubscription(r.Topic, r.Subscription, options)
}

func (s *Store) Close(ctx context.Context) error { return s.client.Close(ctx) }

var _ explorer.Store = (*Store)(nil)

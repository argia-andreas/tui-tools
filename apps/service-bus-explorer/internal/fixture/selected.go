package fixture

import (
	"context"
	"errors"
	"time"

	"servicebusexplorer/internal/explorer"
)

func (s *Store) PrepareDLQCopy(ctx context.Context, source explorer.Resource, sequence int64) (explorer.Replay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sequence < 1 {
		return nil, errors.New("invalid selected DLQ identity")
	}
	for _, message := range s.entries[source].deadLetter {
		if message.Sequence == sequence {
			// Expose only Replay: a read-only preparation has no settlement rights.
			return struct{ explorer.Replay }{&delivery{store: s, resource: source, message: cloneMessage(message), copyOnly: true}}, nil
		}
	}
	return nil, errors.New("selected DLQ message is no longer available; no replacement was selected")
}

func (s *Store) ReceiveSelectedDLQ(ctx context.Context, source explorer.Resource, sequence int64) (explorer.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sequence < 1 {
		return nil, errors.New("invalid selected DLQ identity")
	}
	s.expire()
	if s.held != nil {
		return nil, errors.New("a fixture delivery is already held")
	}
	e, exists := s.entries[source]
	if !exists {
		return nil, errors.New("fixture resource not found")
	}
	for i := 0; i < len(e.deadLetter) && i < 100; i++ {
		if e.deadLetter[i].Sequence == sequence {
			d := &delivery{store: s, resource: source, message: cloneMessage(e.deadLetter[i]), until: s.now().Add(time.Minute)}
			s.held = d
			return d, nil
		}
		// Model the observed count effect of a nonmatch receive and release.
		e.deadLetter[i].DeliveryCount++
	}
	return nil, errors.New("could not acquire selected DLQ message within 100 receives / 5 seconds; no replacement was selected")
}

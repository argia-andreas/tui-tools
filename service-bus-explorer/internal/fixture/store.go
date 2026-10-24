// Package fixture provides deterministic, offline data. It never contacts Azure.
package fixture

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"servicebusexplorer/internal/explorer"
)

//go:embed scenario.json
var scenario []byte

type entry struct {
	active, deadLetter []explorer.Message
}

type Store struct {
	mu        sync.Mutex
	resources []explorer.Resource
	entries   map[explorer.Resource]entry
	held      *delivery
	now       func() time.Time
}

func New() (*Store, error) {
	var seeds []struct {
		Queue, Topic, Subscription string
		Active, DeadLetter         int
	}
	if err := json.Unmarshal(scenario, &seeds); err != nil {
		return nil, fmt.Errorf("read fixture scenario: %w", err)
	}
	s := &Store{entries: make(map[explorer.Resource]entry), now: time.Now}
	for _, seed := range seeds {
		r := explorer.Resource{Profile: "fixture", Namespace: "local-lab", Queue: seed.Queue, Topic: seed.Topic, Subscription: seed.Subscription}
		s.resources = append(s.resources, r)
		s.entries[r] = entry{messages(r, seed.Active, false), messages(r, seed.DeadLetter, true)}
	}
	return s, nil
}

func messages(r explorer.Resource, count int, dlq bool) []explorer.Message {
	result := make([]explorer.Message, count)
	for i := range result {
		kind := "active"
		if dlq {
			kind = "dlq"
		}
		id := fmt.Sprintf("%s-%s-%04d", r.Name(), kind, i+1)
		body, _ := json.MarshalIndent(map[string]any{"id": id, "event": "order.created", "customer": "Northwind", "amount": 49.95 + float64(i), "currency": "EUR", "fixture": true}, "", "  ")
		result[i] = explorer.Message{ID: id, Subject: "order.created", ContentType: "application/json", Sequence: int64(100 + i*7), Body: body, Properties: map[string]any{"region": "eu-north", "schema": "v1"}, EnqueuedAt: time.Date(2026, 10, 23, 10, i%60, 0, 0, time.UTC), DeliveryCount: 1}
		if dlq {
			result[i].DeliveryCount = 10
			result[i].DeadLetterReason = "MaxDeliveryCountExceeded"
			result[i].DeadLetterDescription = "Payment provider timed out after repeated delivery attempts."
		}
	}
	return result
}

func (s *Store) Discover(ctx context.Context) ([]explorer.Resource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return slices.Clone(s.resources), nil
}

func (s *Store) Counts(ctx context.Context, r explorer.Resource) (explorer.Counts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return explorer.Counts{}, err
	}
	s.expire()
	e, ok := s.entries[r]
	if !ok {
		return explorer.Counts{}, errors.New("fixture resource not found")
	}
	return explorer.Counts{Active: int64(len(e.active)), DeadLetter: int64(len(e.deadLetter)), State: "available", ObservedAt: time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC), Detail: "offline fixture values"}, nil
}

func (s *Store) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return explorer.Page{}, err
	}
	s.expire()
	e, ok := s.entries[r]
	if !ok {
		return explorer.Page{}, errors.New("fixture resource not found")
	}
	if limit < 1 || limit > 20 {
		return explorer.Page{}, errors.New("page limit must be between 1 and 20")
	}
	messages := e.active
	if dlq {
		messages = e.deadLetter
	}
	var page explorer.Page
	for _, msg := range messages {
		if msg.Sequence < from {
			continue
		}
		if len(page.Messages) == limit {
			next := page.Messages[len(page.Messages)-1].Sequence + 1
			page.NextSequence = &next
			break
		}
		page.Messages = append(page.Messages, preview(msg))
	}
	return page, nil
}

func (s *Store) ReceiveNextDLQ(ctx context.Context, r explorer.Resource) (explorer.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.expire()
	if s.held != nil {
		return nil, errors.New("a fixture delivery is already held")
	}
	e, ok := s.entries[r]
	if !ok {
		return nil, errors.New("fixture resource not found")
	}
	if len(e.deadLetter) == 0 {
		return nil, errors.New("no available DLQ message")
	}
	d := &delivery{store: s, resource: r, message: cloneMessage(e.deadLetter[0]), until: s.now().Add(time.Minute), legacy: true}
	s.held = d
	return d, nil
}

// Expiry is evaluated on reads, acquisition and mutation; no background goroutine or
// automatic renewal is needed. Peek still includes locked broker messages.
func (s *Store) expire() {
	if s.held != nil && !s.now().Before(s.held.until) {
		s.held.release()
	}
}

type delivery struct {
	store    *Store
	resource explorer.Resource
	message  explorer.Message // full body, independent of all displayed snapshots
	until    time.Time
	settled  bool
	sent     bool
	copyOnly bool
	legacy   bool
}

func (d *delivery) Snapshot() explorer.Message { return preview(d.message) }
func (d *delivery) LockedUntil() time.Time     { return d.until }

func (d *delivery) valid(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.copyOnly {
		return nil
	}
	d.store.expire()
	if d.settled || d.store.held != d {
		return errors.New("fixture delivery is settled or its lock expired; receive again")
	}
	return nil
}

func (d *delivery) Resend(ctx context.Context, destination explorer.Resource, newMessageID string) (explorer.SendResult, error) {
	s := d.store
	s.mu.Lock()
	defer s.mu.Unlock()
	rejected := explorer.SendResult{State: "rejected", MessageID: newMessageID}
	if d.sent {
		return rejected, errors.New("this delivery has already sent a copy")
	}
	// Selected reviews are single-use even on pre-send rejection. Preserve the
	// legacy fixture's validation behavior for its existing receive-next callers.
	if !d.legacy {
		d.sent = true
	}
	if err := d.valid(ctx); err != nil {
		return rejected, err
	}
	if newMessageID == "" || newMessageID == d.message.ID || !utf8.ValidString(newMessageID) || len(newMessageID) > 128 {
		return rejected, errors.New("replay requires a new nonempty message ID")
	}
	if _, exists := s.entries[destination]; !exists || destination.Profile != d.resource.Profile || destination.Namespace != d.resource.Namespace ||
		(destination.Queue == "" && (!d.legacy || destination != d.resource)) {
		return rejected, errors.New("destination must be an existing queue in the connected namespace")
	}
	for _, r := range s.resources {
		if r.Profile != destination.Profile || r.Namespace != destination.Namespace {
			continue
		}
		if destination.Queue != "" {
			if r != destination {
				continue
			}
		} else if r.Topic != destination.Topic || r.Subscription == "" {
			continue
		}
		e := s.entries[r]
		copy := cloneMessage(d.message)
		copy.ID = newMessageID
		copy.Sequence = 100
		for _, msg := range e.active {
			if msg.Sequence >= copy.Sequence {
				copy.Sequence = msg.Sequence + 7
			}
		}
		copy.EnqueuedAt = s.now()
		copy.DeliveryCount = 0
		copy.DeadLetterReason, copy.DeadLetterDescription = "", ""
		copy.PreviewTruncated = false
		delete(copy.Properties, "DeadLetterReason")
		delete(copy.Properties, "DeadLetterErrorDescription")
		e.active = append(e.active, copy)
		s.entries[r] = e
	}
	d.sent = true
	return explorer.SendResult{State: "confirmed", MessageID: newMessageID}, nil
}

func (d *delivery) Complete(ctx context.Context) error {
	s := d.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := d.valid(ctx); err != nil {
		return err
	}
	e := s.entries[d.resource]
	index := slices.IndexFunc(e.deadLetter, func(m explorer.Message) bool { return m.Sequence == d.message.Sequence })
	if index < 0 {
		return errors.New("selected original is no longer available")
	}
	e.deadLetter = slices.Delete(e.deadLetter, index, index+1)
	s.entries[d.resource] = e
	d.settled, s.held = true, nil
	return nil
}

func (d *delivery) Abandon(ctx context.Context) error {
	d.store.mu.Lock()
	defer d.store.mu.Unlock()
	if err := d.valid(ctx); err != nil {
		return err
	}
	d.release()
	return nil
}

func (d *delivery) release() {
	e := d.store.entries[d.resource]
	for i := range e.deadLetter {
		if e.deadLetter[i].Sequence == d.message.Sequence {
			e.deadLetter[i].DeliveryCount++
			break
		}
	}
	d.store.entries[d.resource] = e
	d.settled, d.store.held = true, nil
}

func (s *Store) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.held != nil {
		s.held.release()
	}
	return nil
}

func cloneValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, value := range v {
			m[k] = cloneValue(value)
		}
		return m
	case []any:
		values := make([]any, len(v))
		for i, value := range v {
			values[i] = cloneValue(value)
		}
		return values
	case []byte:
		return slices.Clone(v)
	default:
		return v
	}
}

func cloneMessage(m explorer.Message) explorer.Message {
	m.Body = slices.Clone(m.Body)
	if m.Properties != nil {
		m.Properties = cloneValue(m.Properties).(map[string]any)
	}
	return m
}

func preview(m explorer.Message) explorer.Message {
	if len(m.Body) > 64*1024 {
		m.Body = m.Body[:64*1024]
		m.PreviewTruncated = true
	}
	return cloneMessage(m)
}

var _ explorer.Store = (*Store)(nil)

// Package fixture provides deterministic, offline data. It never contacts Azure.
package fixture

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"servicebusexplorer/internal/explorer"
)

//go:embed scenario.json
var scenario []byte

type entry struct {
	active, deadLetter []explorer.Message
}

type Store struct {
	resources []explorer.Resource
	entries   map[explorer.Resource]entry
}

func New() (*Store, error) {
	var seeds []struct {
		Queue, Topic, Subscription string
		Active, DeadLetter         int
	}
	if err := json.Unmarshal(scenario, &seeds); err != nil {
		return nil, fmt.Errorf("read fixture scenario: %w", err)
	}
	s := &Store{entries: make(map[explorer.Resource]entry)}
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
	if err := ctx.Err(); err != nil {
		return explorer.Counts{}, err
	}
	e, ok := s.entries[r]
	if !ok {
		return explorer.Counts{}, errors.New("fixture resource not found")
	}
	return explorer.Counts{Active: int64(len(e.active)), DeadLetter: int64(len(e.deadLetter)), State: "available", ObservedAt: time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC), Detail: "offline fixture values"}, nil
}

func (s *Store) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	if err := ctx.Err(); err != nil {
		return explorer.Page{}, err
	}
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
		msg.Body = slices.Clone(msg.Body)
		msg.Properties = maps.Clone(msg.Properties)
		page.Messages = append(page.Messages, msg)
	}
	return page, nil
}

func (*Store) ReceiveNextDLQ(context.Context, explorer.Resource) (explorer.Delivery, error) {
	return nil, errors.New("fixture browsing is read-only; reviewed DLQ actions arrive in a later slice")
}

func (*Store) Close(context.Context) error { return nil }

var _ explorer.Store = (*Store)(nil)

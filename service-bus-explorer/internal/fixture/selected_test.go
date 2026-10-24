package fixture

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"servicebusexplorer/internal/explorer"
)

func TestPeekMessageFullContentIndependentOfPreviews(t *testing.T) {
	for _, dlq := range []bool{false, true} {
		s, resources := newStore(t)
		source := resources[0]
		e := s.entries[source]
		messages := e.active
		if dlq {
			messages = e.deadLetter
		}
		messages[1].Body = bytes.Repeat([]byte("full text é\n"), 9000)
		messages[1].Properties["binary"] = []byte{0, 255}
		messages[1].Properties["nested"] = map[string]any{"values": []any{[]byte{1, 2}}}
		want := cloneMessage(messages[1])
		s.entries[source] = e
		var store explorer.Store = s
		ctx := context.Background()
		page, err := store.Peek(ctx, source, dlq, want.Sequence, 1)
		if err != nil || len(page.Messages) != 1 || len(page.Messages[0].Body) != 64*1024 || !page.Messages[0].PreviewTruncated {
			t.Fatalf("preview contract: %+v %v", page, err)
		}
		full, err := store.PeekMessage(ctx, source, dlq, want.Sequence)
		if err != nil || full.PreviewTruncated || !reflect.DeepEqual(full, want) {
			t.Fatalf("full content/metadata not preserved: %v", err)
		}
		full.Body[0] = '!'
		full.Properties["binary"].([]byte)[0] = 42
		full.Properties["nested"].(map[string]any)["values"].([]any)[0].([]byte)[0] = 42
		full.Properties["region"] = "changed"
		again, err := store.PeekMessage(ctx, source, dlq, want.Sequence)
		if err != nil || !reflect.DeepEqual(again, want) || !reflect.DeepEqual(messages[1], want) || s.held != nil {
			t.Fatal("full peek aliases source or acquired delivery")
		}
		if dlq {
			delivery, err := store.ReceiveSelectedDLQ(ctx, source, want.Sequence)
			if err != nil {
				t.Fatal(err)
			}
			preview := delivery.Snapshot()
			if len(preview.Body) != 64*1024 || !preview.PreviewTruncated {
				t.Fatal("delivery preview contract changed")
			}
			// Reading full content must not expire a lock and increment counters.
			s.now = func() time.Time { return delivery.LockedUntil().Add(time.Second) }
			before := cloneMessage(s.entries[source].deadLetter[1])
			if _, err := store.PeekMessage(ctx, source, dlq, want.Sequence); err != nil || !reflect.DeepEqual(s.entries[source].deadLetter[1], before) || s.held == nil {
				t.Fatal("full peek expired delivery or changed its count")
			}
		}
	}
}

func TestSelectedCopyPreservesFullPrivateContent(t *testing.T) {
	s, resources := newStore(t)
	source, destination := resources[0], resources[11]
	e := s.entries[source]
	body := bytes.Repeat([]byte{0, 255, 65}, 30000)
	e.deadLetter[1].Body = body
	e.deadLetter[1].Properties["binary"] = []byte{0, 255}
	s.entries[source] = e
	replay, err := s.PrepareDLQCopy(context.Background(), source, e.deadLetter[1].Sequence)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := replay.Snapshot()
	if len(snapshot.Body) != 64*1024 || !snapshot.PreviewTruncated {
		t.Fatal("selected preview not bounded")
	}
	snapshot.Body[0] = 99
	snapshot.Properties["binary"].([]byte)[0] = 99
	if result, err := replay.Resend(context.Background(), destination, "selected-full"); err != nil || result.State != "confirmed" {
		t.Fatalf("selected copy: %+v %v", result, err)
	}
	copy := s.entries[destination].active[0]
	if !bytes.Equal(copy.Body, body) || len(copy.Body) != 90000 || copy.Properties["binary"].([]byte)[0] != 0 || copy.PreviewTruncated || len(s.entries[source].deadLetter) != 12 || s.held != nil {
		t.Fatal("selected copy lost full content, mutated originals, or acquired lock")
	}
}

func TestSelectedBoundExpiryAndCancellationNeverDeleteAnotherOriginal(t *testing.T) {
	s, resources := newStore(t)
	source, destination := resources[0], resources[11]
	e := s.entries[source]
	e.deadLetter = messages(source, 101, true)
	s.entries[source] = e
	ctx := context.Background()
	if _, err := s.ReceiveSelectedDLQ(ctx, source, e.deadLetter[100].Sequence); err == nil || s.held != nil {
		t.Fatal("selected acquisition exceeded 100-message bound")
	}
	if e.deadLetter[99].DeliveryCount != 11 || e.deadLetter[100].DeliveryCount != 10 {
		t.Fatal("bounded scan affected an unreceived message")
	}
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	delivery, err := s.ReceiveSelectedDLQ(ctx, source, e.deadLetter[1].Sequence)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if result, err := delivery.Resend(ctx, destination, "expired-selected"); err == nil || result.State != "rejected" {
		t.Fatal("expired selected lock sent")
	}
	if err := delivery.Complete(ctx); err == nil {
		t.Fatal("expired selected lock completed")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.PrepareDLQCopy(canceled, source, 107); err == nil {
		t.Fatal("canceled preparation succeeded")
	}
	if _, err := s.ReceiveSelectedDLQ(canceled, source, 107); err == nil {
		t.Fatal("canceled acquisition succeeded")
	}
	if len(s.entries[source].deadLetter) != 101 || len(s.entries[destination].active) != 0 {
		t.Fatal("failed selected action sent or deleted")
	}
}

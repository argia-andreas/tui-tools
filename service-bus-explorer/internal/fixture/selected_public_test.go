package fixture_test

import (
	"bytes"
	"context"
	"testing"

	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
)

func selectedFixture(t *testing.T) (explorer.Store, explorer.Resource, explorer.Resource, explorer.Resource) {
	t.Helper()
	s, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	resources, err := s.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]explorer.Resource)
	for _, r := range resources {
		byName[r.Name()] = r
	}
	return s, byName["orders"], byName["empty"], byName["events/shipping"]
}

func TestSelectedCopyIsReadOnlyAndRoutesOnlyToChosenQueue(t *testing.T) {
	ctx := context.Background()
	s, source, destination, _ := selectedFixture(t)
	before, _ := s.Peek(ctx, source, true, 0, 20)
	selected := before.Messages[1]
	replay, err := s.PrepareDLQCopy(ctx, source, selected.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	if _, isDelivery := replay.(explorer.Delivery); isDelivery {
		t.Fatal("copy preparation exposes settlement")
	}
	preview := replay.Snapshot()
	preview.Body[0] = '!'
	preview.Properties["region"] = "changed"
	// Preparation did not acquire a delivery or prevent another receive.
	held, err := s.ReceiveNextDLQ(ctx, source)
	if err != nil || held.Snapshot().ID != before.Messages[0].ID {
		t.Fatalf("copy preparation acquired original: %v", err)
	}
	if result, err := replay.Resend(ctx, destination, "selected-copy"); err != nil || result.State != "confirmed" {
		t.Fatalf("send: %+v %v", result, err)
	}
	if _, err := replay.Resend(ctx, destination, "selected-copy-again"); err == nil {
		t.Fatal("copy sent twice")
	}
	copy, _ := s.Peek(ctx, destination, false, 0, 20)
	if len(copy.Messages) != 1 || copy.Messages[0].ID != "selected-copy" || !bytes.Equal(copy.Messages[0].Body, selected.Body) || copy.Messages[0].Properties["region"] != selected.Properties["region"] {
		t.Fatalf("wrong queue copy: %+v", copy)
	}
	after, _ := s.Peek(ctx, source, true, 0, 20)
	for i := range before.Messages {
		if after.Messages[i].ID != before.Messages[i].ID || after.Messages[i].DeliveryCount != before.Messages[i].DeliveryCount {
			t.Fatal("copy changed original DLQ")
		}
	}
	if err := held.Abandon(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedCleanupRemovesOnlyNonFirstSelection(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		s, queue, destination, sub := selectedFixture(t)
		source := queue
		if subscription {
			source = sub
		}
		ctx := context.Background()
		before, _ := s.Peek(ctx, source, true, 0, 20)
		selected := before.Messages[1]
		delivery, err := s.ReceiveSelectedDLQ(ctx, source, selected.Sequence)
		if err != nil || delivery.Snapshot().ID != selected.ID {
			t.Fatalf("selected receive: %v", err)
		}
		if result, err := delivery.Resend(ctx, destination, "selected-clean"); err != nil || result.State != "confirmed" {
			t.Fatalf("send: %+v %v", result, err)
		}
		if err := delivery.Complete(ctx); err != nil {
			t.Fatal(err)
		}
		after, _ := s.Peek(ctx, source, true, 0, 20)
		if len(after.Messages) != len(before.Messages)-1 {
			t.Fatal("wrong deletion count")
		}
		for _, message := range after.Messages {
			if message.Sequence == selected.Sequence {
				t.Fatal("selected original not removed")
			}
		}
		neighbor, err := s.ReceiveNextDLQ(ctx, source)
		if err != nil || neighbor.Snapshot().ID != before.Messages[0].ID || !bytes.Equal(neighbor.Snapshot().Body, before.Messages[0].Body) {
			t.Fatalf("neighbor lost or unavailable: %v", err)
		}
		if err := neighbor.Abandon(ctx); err != nil {
			t.Fatal(err)
		}
		copy, _ := s.Peek(ctx, destination, false, 0, 20)
		if len(copy.Messages) != 1 || !bytes.Equal(copy.Messages[0].Body, selected.Body) {
			t.Fatal("selected queue copy missing")
		}
	}
}

func TestSelectedMissingIdentityAndInvalidDestinationsNeverSend(t *testing.T) {
	s, source, destination, subscription := selectedFixture(t)
	ctx := context.Background()
	for _, sequence := range []int64{-1, 0, 101, 999999} {
		if _, err := s.PrepareDLQCopy(ctx, source, sequence); err == nil {
			t.Fatalf("prepared substitute for sequence %d", sequence)
		}
		if _, err := s.ReceiveSelectedDLQ(ctx, source, sequence); err == nil {
			t.Fatalf("acquired substitute for sequence %d", sequence)
		}
	}
	for _, invalid := range []explorer.Resource{
		{Profile: "other", Namespace: source.Namespace, Queue: destination.Queue},
		{Profile: source.Profile, Namespace: "other", Queue: destination.Queue},
		{Profile: source.Profile, Namespace: source.Namespace, Queue: "empty/$DeadLetterQueue"},
		{Profile: source.Profile, Namespace: source.Namespace, Queue: "missing"},
		subscription,
	} {
		replay, err := s.PrepareDLQCopy(ctx, source, 107)
		if err != nil {
			t.Fatal(err)
		}
		if result, err := replay.Resend(ctx, invalid, "invalid-destination"); err == nil || result.State != "rejected" {
			t.Fatalf("accepted invalid destination: %+v", invalid)
		}
	}
	page, _ := s.Peek(ctx, destination, false, 0, 20)
	if len(page.Messages) != 0 {
		t.Fatal("failure sent a copy")
	}
}

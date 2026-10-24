package fixture

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"servicebusexplorer/internal/explorer"
)

func TestBrowsePreservesMessagesAndSequencePaging(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resources, err := s.Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 14 {
		t.Fatalf("resources: %d", len(resources))
	}
	r := resources[0]
	counts, err := s.Counts(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Active != 128 || counts.DeadLetter != 12 {
		t.Fatalf("counts: %+v", counts)
	}
	page, err := s.Peek(ctx, r, false, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[0].Sequence != 100 || page.Messages[1].Sequence != 107 {
		t.Fatalf("page: %+v", page)
	}
	next, err := s.Peek(ctx, r, false, *page.NextSequence, 2)
	if err != nil || next.Messages[0].Sequence != 114 {
		t.Fatalf("next: %+v, %v", next, err)
	}
	page.Messages[0].Body[0] = '!'
	page.Messages[0].Properties["region"] = "changed"
	again, err := s.Peek(ctx, r, false, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if again.Messages[0].Body[0] != '{' || again.Messages[0].Properties["region"] != "eu-north" {
		t.Fatal("preview changed source content")
	}
	after, _ := s.Counts(ctx, r)
	if after != counts {
		t.Fatal("browsing changed counts")
	}
	dlq, err := s.Peek(ctx, r, true, 0, 20)
	if err != nil || len(dlq.Messages) != 12 || dlq.Messages[0].DeadLetterReason != "MaxDeliveryCountExceeded" {
		t.Fatalf("DLQ: %+v, %v", dlq, err)
	}
	d, err := s.ReceiveNextDLQ(ctx, r)
	if err != nil || d.Snapshot().ID != dlq.Messages[0].ID {
		t.Fatalf("browse consumed source message: %v", err)
	}
}

func newStore(t *testing.T) (*Store, []explorer.Resource) {
	t.Helper()
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	resources, err := s.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s, resources
}

func receive(t *testing.T, s *Store, r explorer.Resource) explorer.Delivery {
	t.Helper()
	d, err := s.ReceiveNextDLQ(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestResendRetainsOriginalAndCleanupIsSeparate(t *testing.T) {
	ctx := context.Background()
	s, resources := newStore(t)
	r := resources[0]
	before, _ := s.Counts(ctx, r)
	// The displayed row can differ from the next received identity.
	selected, _ := s.Peek(ctx, r, true, 107, 1)
	d := receive(t, s, r)
	original := d.Snapshot()
	if original.ID == selected.Messages[0].ID {
		t.Fatal("receive followed a peek cursor rather than next available")
	}
	if _, err := s.ReceiveNextDLQ(ctx, resources[1]); err == nil {
		t.Fatal("allowed a second held delivery")
	}
	if result, err := d.Resend(ctx, "replay-1"); err != nil || result.State != "confirmed" || result.MessageID != "replay-1" {
		t.Fatalf("resend: %+v %v", result, err)
	}
	after, _ := s.Counts(ctx, r)
	if after.Active != before.Active+1 || after.DeadLetter != before.DeadLetter {
		t.Fatalf("resend must copy, not move: %+v", after)
	}
	dlq, _ := s.Peek(ctx, r, true, 0, 1)
	if dlq.Messages[0].ID != original.ID {
		t.Fatal("original missing after resend")
	}
	copy, _ := s.Peek(ctx, r, false, 100+128*7, 1)
	if len(copy.Messages) != 1 {
		t.Fatalf("missing replay: %+v", copy)
	}
	m := copy.Messages[0]
	if m.ID != "replay-1" || !bytes.Equal(m.Body, original.Body) || m.Subject != original.Subject || m.ContentType != original.ContentType || m.Properties["region"] != "eu-north" || m.DeadLetterReason != "" || m.DeadLetterDescription != "" {
		t.Fatalf("incorrect copy: %+v", m)
	}
	if _, err := d.Resend(ctx, "replay-2"); err == nil {
		t.Fatal("allowed a repeated send")
	}
	if err := d.Complete(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ = s.Counts(ctx, r)
	if after.Active != before.Active+1 || after.DeadLetter != before.DeadLetter-1 {
		t.Fatalf("separate cleanup: %+v", after)
	}
	if err := d.Complete(ctx); err == nil {
		t.Fatal("completed twice")
	}
	if err := d.Abandon(ctx); err == nil {
		t.Fatal("abandoned settled message")
	}
	if _, err := d.Resend(ctx, "replay-3"); err == nil {
		t.Fatal("sent settled message")
	}
	next := receive(t, s, r)
	if next.Snapshot().ID == original.ID {
		t.Fatal("cleanup deleted wrong identity")
	}
}

func TestTopicResendFansOutAndRetainedOriginalCanBeReleased(t *testing.T) {
	ctx := context.Background()
	s, resources := newStore(t)
	billing, shipping := resources[12], resources[13]
	d := receive(t, s, billing)
	original := d.Snapshot()
	if _, err := d.Resend(ctx, "topic-replay"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []explorer.Resource{billing, shipping} {
		page, _ := s.Peek(ctx, r, false, 0, 20)
		m := page.Messages[len(page.Messages)-1]
		if m.ID != "topic-replay" || !bytes.Equal(m.Body, original.Body) || m.DeadLetterReason != "" {
			t.Fatalf("missing topic replay in %s: %+v", r.Name(), m)
		}
	}
	counts, _ := s.Counts(ctx, resources[0])
	if counts.Active != 128 {
		t.Fatal("topic replay leaked into queue")
	}
	if err := d.Abandon(ctx); err != nil {
		t.Fatal(err)
	}
	again := receive(t, s, billing).Snapshot()
	if again.ID != original.ID || again.DeliveryCount != original.DeliveryCount+1 {
		t.Fatalf("release lost original or delivery count: %+v", again)
	}
}

func TestExpiryDisablesOldDeliveryAndRestoresAvailability(t *testing.T) {
	ctx := context.Background()
	s, resources := newStore(t)
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	d := receive(t, s, resources[0])
	original := d.Snapshot()
	if d.LockedUntil() != now.Add(time.Minute) {
		t.Fatal("lock is not one minute")
	}
	now = now.Add(time.Minute)
	if _, err := d.Resend(ctx, "expired-replay"); err == nil {
		t.Fatal("expired lock sent")
	}
	if err := d.Complete(ctx); err == nil {
		t.Fatal("expired lock completed")
	}
	if err := d.Abandon(ctx); err == nil {
		t.Fatal("expired lock abandoned")
	}
	again := receive(t, s, resources[0])
	if again.Snapshot().ID != original.ID || again.Snapshot().DeliveryCount != original.DeliveryCount+1 {
		t.Fatal("expiry failed to restore original")
	}
	counts, _ := s.Counts(ctx, resources[0])
	if counts.Active != 128 || counts.DeadLetter != 12 {
		t.Fatalf("expiry changed backlog: %+v", counts)
	}
}

func TestCanceledContextsDoNotMutate(t *testing.T) {
	ctx := context.Background()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	s, resources := newStore(t)
	r := resources[0]
	if _, err := s.ReceiveNextDLQ(canceled, r); !errors.Is(err, context.Canceled) {
		t.Fatalf("receive: %v", err)
	}
	d := receive(t, s, r)
	if _, err := d.Resend(canceled, "canceled-replay"); !errors.Is(err, context.Canceled) {
		t.Fatalf("resend: %v", err)
	}
	if err := d.Complete(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("complete: %v", err)
	}
	if err := d.Abandon(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("abandon: %v", err)
	}
	if err := s.Close(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("close: %v", err)
	}
	if _, err := s.Counts(canceled, r); !errors.Is(err, context.Canceled) {
		t.Fatalf("counts: %v", err)
	}
	if _, err := s.Peek(canceled, r, true, 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("peek: %v", err)
	}
	counts, _ := s.Counts(ctx, r)
	if counts.Active != 128 || counts.DeadLetter != 12 {
		t.Fatalf("cancellation changed backlog: %+v", counts)
	}
	if _, err := d.Resend(ctx, "normal-replay"); err != nil {
		t.Fatalf("canceled send consumed opportunity: %v", err)
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, s, r).Snapshot(); got.DeliveryCount != d.Snapshot().DeliveryCount+1 {
		t.Fatal("close failed to release lock")
	}
}

func TestPreviewCloningCannotTruncateReplayBody(t *testing.T) {
	ctx := context.Background()
	s, resources := newStore(t)
	r := resources[0]
	e := s.entries[r]
	body := bytes.Repeat([]byte("x"), 70*1024)
	e.deadLetter[0].Body = body
	e.deadLetter[0].Properties["nested"] = map[string]any{"data": []byte("original")}
	e.deadLetter[0].Properties["DeadLetterReason"] = "reason"
	e.deadLetter[0].Properties["DeadLetterErrorDescription"] = "description"
	s.entries[r] = e
	d := receive(t, s, r)
	for _, m := range []explorer.Message{d.Snapshot(), d.Snapshot()} {
		if !m.PreviewTruncated || len(m.Body) != 64*1024 {
			t.Fatal("preview is not bounded")
		}
		m.Body[0] = '!'
		m.Properties["nested"].(map[string]any)["data"].([]byte)[0] = '!'
	}
	peek, _ := s.Peek(ctx, r, true, 0, 1)
	if !peek.Messages[0].PreviewTruncated || peek.Messages[0].Body[0] != 'x' || d.Snapshot().Body[0] != 'x' {
		t.Fatal("snapshot mutated source")
	}
	if _, err := d.Resend(ctx, "full-replay"); err != nil {
		t.Fatal(err)
	}
	// Public preview is bounded; inspect the fixture's stored outbound data to
	// prove truncation did not discard bytes before enqueueing the copy.
	active := s.entries[r].active
	copy := active[len(active)-1]
	if !bytes.Equal(copy.Body, body) || copy.PreviewTruncated || copy.Properties["nested"].(map[string]any)["data"].([]byte)[0] != 'o' {
		t.Fatal("replay did not retain independent full content")
	}
	if _, ok := copy.Properties["DeadLetterReason"]; ok {
		t.Fatal("replayed DLQ reason")
	}
	if _, ok := copy.Properties["DeadLetterErrorDescription"]; ok {
		t.Fatal("replayed DLQ description")
	}
}

func TestConcurrentReadsAndSingleSend(t *testing.T) {
	ctx := context.Background()
	s, resources := newStore(t)
	r := resources[0]
	d := receive(t, s, r)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			_, _ = d.Resend(ctx, "concurrent-replay")
			_, _ = s.Counts(ctx, r)
			_, _ = s.Peek(ctx, r, true, 0, 20)
			_ = d.Snapshot()
		})
	}
	wg.Wait()
	counts, _ := s.Counts(ctx, r)
	if counts.Active != 129 || counts.DeadLetter != 12 {
		t.Fatalf("concurrent send duplicated or deleted: %+v", counts)
	}
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			_ = d.Complete(ctx)
			_ = d.Abandon(ctx)
			_, _ = s.Peek(ctx, r, true, 0, 20)
			_, _ = s.Counts(ctx, r)
		})
	}
	wg.Wait()
	counts, _ = s.Counts(ctx, r)
	if counts.DeadLetter != 11 {
		t.Fatalf("concurrent settlement deleted multiple: %+v", counts)
	}
}

func TestDeleteWithoutResendAndReceiveValidation(t *testing.T) {
	ctx := context.Background()
	s, resources := newStore(t)
	for _, r := range []explorer.Resource{{Queue: "missing"}, resources[11], {Profile: "fixture", Namespace: "local-lab", Topic: "events"}} {
		if _, err := s.ReceiveNextDLQ(ctx, r); err == nil {
			t.Fatalf("received from empty or invalid target: %+v", r)
		}
	}
	r := resources[0]
	d := receive(t, s, r)
	for _, id := range []string{"", d.Snapshot().ID} {
		if result, err := d.Resend(ctx, id); err == nil || result.State != "rejected" {
			t.Fatalf("accepted invalid replay identity: %+v %v", result, err)
		}
	}
	if err := d.Complete(ctx); err != nil {
		t.Fatal(err)
	}
	counts, _ := s.Counts(ctx, r)
	if counts.Active != 128 || counts.DeadLetter != 11 {
		t.Fatalf("delete sent a copy or deleted multiple: %+v", counts)
	}
	if receive(t, s, r).Snapshot().ID == d.Snapshot().ID {
		t.Fatal("deleted identity remained receivable")
	}
}

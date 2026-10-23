package fixture_test

import (
	"context"
	"testing"

	"servicebusexplorer/internal/fixture"
)

func TestBrowsePreservesMessagesAndSequencePaging(t *testing.T) {
	s, err := fixture.New()
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
	if _, err := s.ReceiveNextDLQ(ctx, r); err == nil {
		t.Fatal("tracer bullet unexpectedly allows mutations")
	}
}

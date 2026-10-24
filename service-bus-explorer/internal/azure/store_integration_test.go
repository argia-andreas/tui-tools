//go:build integration

package azure_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebusexplorer/internal/azure"
	"servicebusexplorer/internal/explorer"
)

// Requires the isolated lab seeded by scripts/lab.py; never targets Azure.
func TestSeededEmulatorBrowsing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	store, err := azure.NewEmulator()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := store.Close(closeCtx); err != nil {
			t.Error(err)
		}
	}()
	resources, err := store.Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 14 {
		t.Fatalf("discovery returned %d resources, want 12 queues + 2 subscriptions", len(resources))
	}
	byName := make(map[string]explorer.Resource)
	for _, r := range resources {
		byName[r.Name()] = r
		counts, err := store.Counts(ctx, r)
		if err != nil || counts.State != "unavailable" || counts.Detail == "" {
			t.Fatalf("emulator counts must be unavailable, not manufactured zero: %+v, %v", counts, err)
		}
	}
	for _, name := range []string{"orders", "empty", "events/billing", "events/shipping"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("missing discovered resource %s", name)
		}
	}
	orders := byName["orders"]
	first, err := store.Peek(ctx, orders, false, 0, 20)
	if err != nil || len(first.Messages) != 20 || first.NextSequence == nil {
		t.Fatalf("first orders page: %d messages, next=%v, err=%v", len(first.Messages), first.NextSequence, err)
	}
	wantBody := []byte(`{
  "amount": 49.95,
  "currency": "EUR",
  "customer": "Northwind",
  "event": "order.created",
  "fixture": true,
  "id": "orders-active-0001"
}`)
	if first.Messages[0].ID != "orders-active-0001" || !bytes.Equal(first.Messages[0].Body, wantBody) || first.Messages[0].Properties["region"] != "eu-north" {
		t.Fatalf("seeded message ID/body/properties differ: %+v", first.Messages[0])
	}
	refresh, err := store.Peek(ctx, orders, false, 0, 20)
	if err != nil || len(refresh.Messages) != 20 {
		t.Fatalf("refresh: %+v, %v", refresh, err)
	}
	for i, message := range first.Messages {
		if refresh.Messages[i].ID != message.ID || !bytes.Equal(refresh.Messages[i].Body, message.Body) || refresh.Messages[i].DeliveryCount != message.DeliveryCount {
			t.Fatalf("browse/refresh changed message at index %d", i)
		}
	}
	next, err := store.Peek(ctx, orders, false, *first.NextSequence, 20)
	if err != nil || len(next.Messages) != 20 || next.Messages[0].Sequence <= first.Messages[19].Sequence || next.Messages[0].ID != "orders-active-0021" {
		t.Fatalf("explicit next-page cursor failed: %+v, %v", next, err)
	}
	dlq, err := store.Peek(ctx, orders, true, 0, 20)
	if err != nil || len(dlq.Messages) != 12 {
		t.Fatalf("orders DLQ: %d messages, err=%v", len(dlq.Messages), err)
	}
	if dlq.Messages[0].ID != "orders-dlq-0001" || dlq.Messages[0].DeadLetterReason != "MaxDeliveryCountExceeded" || len(dlq.Messages[0].Body) == 0 {
		t.Fatal("DLQ inspection lost body or failure reason")
	}
	for _, test := range []struct {
		name, firstID string
		active, dead  int
	}{
		{"events/billing", "events/billing-active-0001", 7, 1},
		{"events/shipping", "events/billing-active-0003", 5, 3},
	} {
		page, err := store.Peek(ctx, byName[test.name], false, 0, 20)
		if err != nil || len(page.Messages) != test.active || page.Messages[0].ID != test.firstID || !bytes.Contains(page.Messages[0].Body, []byte(test.firstID)) {
			t.Fatalf("topic subscription %s: %+v, %v", test.name, page, err)
		}
		dead, err := store.Peek(ctx, byName[test.name], true, 0, 20)
		if err != nil || len(dead.Messages) != test.dead || dead.Messages[0].ID != "events/billing-dlq-0001" || dead.Messages[0].DeadLetterReason != "MaxDeliveryCountExceeded" {
			t.Fatalf("subscription DLQ %s: %+v, %v", test.name, dead, err)
		}
	}
	empty, err := store.Peek(ctx, byName["empty"], false, 0, 20)
	if err != nil || len(empty.Messages) != 0 || empty.NextSequence != nil {
		t.Fatalf("empty queue: %+v, %v", empty, err)
	}
	if _, err := store.ReceiveNextDLQ(ctx, orders); err == nil {
		t.Fatal("read-only backend allowed lock acquisition")
	}

	// Independent consumer proves browsing did not acquire or settle the message.
	// Use the known-empty lab queue and remove only this probe after verification.
	client, err := sb.NewClientFromConnectionString("Endpoint=sb://127.0.0.1:5673;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=SAS_KEY_VALUE;UseDevelopmentEmulator=true;", &sb.ClientOptions{RetryOptions: sb.RetryOptions{MaxRetries: -1}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	sender, err := client.NewSender("empty", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close(context.Background())
	id, contentType := "browse-proof-1", "application/octet-stream"
	probeBody := []byte{0x00, 0x41, 0xff, 0x0a, 0x1b, 0x7f}
	if err := sender.SendMessage(ctx, &sb.Message{MessageID: &id, ContentType: &contentType, Body: probeBody}, nil); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		page, err := store.Peek(ctx, byName["empty"], false, 0, 20)
		if err != nil || len(page.Messages) != 1 || page.Messages[0].ID != id || !bytes.Equal(page.Messages[0].Body, probeBody) {
			t.Fatalf("probe browse lost binary content: %+v, %v", page, err)
		}
	}
	receiver, err := client.NewReceiverForQueue("empty", &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close(context.Background())
	receiveCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	received, err := receiver.ReceiveMessages(receiveCtx, 1, nil)
	stop()
	if err != nil || len(received) != 1 || received[0].MessageID != id || !bytes.Equal(received[0].Body, probeBody) {
		t.Fatalf("peek consumed/locked/changed the message: %+v, %v", received, err)
	}
	if err := receiver.CompleteMessage(ctx, received[0], nil); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: Go SDK discovery, byte-checked bodies/properties, paging/refresh, Active/DLQ/subscriptions, unavailable counters; independent receive proves peek preserved availability")
}

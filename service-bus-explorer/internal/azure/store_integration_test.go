//go:build integration

package azure_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebusexplorer/internal/azure"
	"servicebusexplorer/internal/explorer"
)

// Requires the isolated lab freshly seeded by scripts/lab.py; never targets Azure.
// SERIAL: do not use t.Parallel or run another lab command/test process during
// this test. Mutation subtests run only after all seeded browsing assertions.
// Reseed afterwards: the topic fanout probes intentionally leave active copies.
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
	t.Run("DLQReplayRetentionAndSeparateCleanup", func(t *testing.T) {
		testQueueReplay(t, ctx, store, client, byName["empty"])
	})
	t.Run("DLQTopicReplayFansOutAndRetainsOriginal", func(t *testing.T) {
		testTopicReplay(t, ctx, store, byName["events/billing"], byName["events/shipping"])
	})
}

func testQueueReplay(t *testing.T, ctx context.Context, store *azure.Store, client *sb.Client, target explorer.Resource) {
	t.Helper()
	sender, err := client.NewSender(target.Queue, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close(context.Background())
	receiver, err := client.NewReceiverForQueue(target.Queue, &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close(context.Background())
	id, correlation, subject, contentType := fmt.Sprintf("slice4-original-%d", time.Now().UnixNano()), "slice4-correlation", "binary-preservation", "application/octet-stream"
	// The lab queue's default one-hour lifetime caps a longer requested TTL.
	// Use an asymmetric value below that cap and verify it on the source too.
	ttl := 37 * time.Minute
	pastSchedule := time.Now().Add(-time.Hour)
	body := bytes.Repeat([]byte{0, 255, 65, 10, 27}, 18000)
	// Emulator 2.0.1 rejects AMQP binary application properties during broker
	// serialization, mapped by the SDK to ErrMessageTooLarge (even for 1 byte).
	// Assert that boundary explicitly; binary-property cloning remains unit-tested.
	unsupportedID := id + "-unsupported"
	if err := sender.SendMessage(ctx, &sb.Message{MessageID: &unsupportedID, Body: []byte{65}, ApplicationProperties: map[string]any{"binary": []byte{0, 255}}}, nil); !errors.Is(err, sb.ErrMessageTooLarge) {
		t.Fatalf("expected emulator binary application-property rejection, got %v", err)
	}
	rejected, err := store.Peek(ctx, target, false, 0, 20)
	if err != nil || len(rejected.Messages) != 0 {
		t.Fatalf("rejected property probe was enqueued: %+v, %v", rejected, err)
	}
	if err := sender.SendMessage(ctx, &sb.Message{MessageID: &id, CorrelationID: &correlation, Subject: &subject, ContentType: &contentType, TimeToLive: &ttl,
		ScheduledEnqueueTime: &pastSchedule, Body: body, ApplicationProperties: map[string]any{"business": "preserve", "enabled": true, "number": int64(42)}}, nil); err != nil {
		t.Fatal(err)
	}
	active := receiveOne(t, ctx, receiver)
	if active.MessageID != id {
		t.Fatal("expected isolated empty queue; reseed lab before this test")
	}
	if active.TimeToLive == nil || *active.TimeToLive != ttl {
		t.Fatalf("source TTL changed before replay: %v", active.TimeToLive)
	}
	reason, description := "slice4-reason", "slice4-description"
	if err := receiver.DeadLetterMessage(ctx, active, &sb.DeadLetterOptions{Reason: &reason, ErrorDescription: &description}); err != nil {
		t.Fatal(err)
	}
	d, err := store.ReceiveNextDLQ(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Abandon(context.Background())
	preview := d.Snapshot()
	if preview.ID != id || preview.DeadLetterReason != reason || preview.DeadLetterDescription != description || !preview.PreviewTruncated || len(preview.Body) != 64*1024 || !d.LockedUntil().After(time.Now()) {
		t.Fatalf("held delivery preview: %+v", preview)
	}
	preview.Body[0] = 99
	preview.Properties["business"] = "changed"
	if _, err := store.ReceiveNextDLQ(ctx, target); err == nil {
		t.Fatal("store allowed another held delivery")
	}
	replayID := id + "-replay"
	result, err := d.Resend(ctx, replayID)
	if err != nil || result.State != "confirmed" || result.MessageID != replayID {
		t.Fatalf("resend: %+v, %v", result, err)
	}
	if second, err := d.Resend(ctx, id+"-duplicate"); err == nil || second.State != "rejected" {
		t.Fatal("repeated send was allowed")
	}
	copy := receiveOne(t, ctx, receiver)
	if copy.MessageID != replayID || !bytes.Equal(copy.Body, body) || valueString(copy.CorrelationID) != correlation || valueString(copy.Subject) != subject || valueString(copy.ContentType) != contentType || copy.TimeToLive == nil || *copy.TimeToLive != ttl || copy.ApplicationProperties["business"] != "preserve" || copy.ApplicationProperties["number"] != int64(42) || copy.ApplicationProperties["enabled"] != true {
		t.Fatalf("replay mismatch: id=%q len=%d bytesEqual=%v correlation=%q subject=%q contentType=%q TTL=%v properties=%#v", copy.MessageID, len(copy.Body), bytes.Equal(copy.Body, body), valueString(copy.CorrelationID), valueString(copy.Subject), valueString(copy.ContentType), copy.TimeToLive, copy.ApplicationProperties)
	}
	if _, ok := copy.ApplicationProperties["DeadLetterReason"]; ok {
		t.Fatal("replay retained DLQ reason")
	}
	if _, ok := copy.ApplicationProperties["DeadLetterErrorDescription"]; ok {
		t.Fatal("replay retained DLQ description")
	}
	if copy.ScheduledEnqueueTime != nil {
		t.Fatal("replay retained scheduled enqueue time")
	}
	if err := receiver.CompleteMessage(ctx, copy, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.Abandon(ctx); err != nil {
		t.Fatal(err)
	}
	retained, err := store.ReceiveNextDLQ(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer retained.Abandon(context.Background())
	if retained.Snapshot().ID != id || retained.Snapshot().DeadLetterReason != reason {
		t.Fatal("confirmed replay removed/changed original")
	}
	// Separate explicit authorization: delete the retained original without sending.
	if err := retained.Complete(ctx); err != nil {
		t.Fatal(err)
	}
	page, err := store.Peek(ctx, target, true, 0, 20)
	if err != nil || len(page.Messages) != 0 {
		t.Fatalf("cleanup did not remove only original: %+v, %v", page, err)
	}
	page, err = store.Peek(ctx, target, false, 0, 20)
	if err != nil || len(page.Messages) != 0 {
		t.Fatalf("repeated submission made another copy: %+v, %v", page, err)
	}
	t.Log("PASS: full 90KB binary body and scalar properties preserved independently of preview; new ID/correlation/TTL, no schedule/DLQ fields; original retained until separate Complete; unsupported binary property explicitly rejected")
}

func testTopicReplay(t *testing.T, ctx context.Context, store *azure.Store, billing, shipping explorer.Resource) {
	t.Helper()
	d, err := store.ReceiveNextDLQ(ctx, billing)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Abandon(context.Background())
	original := d.Snapshot()
	id := fmt.Sprintf("slice4-topic-replay-%d", time.Now().UnixNano())
	result, err := d.Resend(ctx, id)
	if err != nil || result.State != "confirmed" {
		t.Fatalf("topic resend: %+v, %v", result, err)
	}
	for _, target := range []explorer.Resource{billing, shipping} {
		copy := awaitPeekID(t, ctx, store, target, false, id)
		if !bytes.Equal(copy.Body, original.Body) || copy.Subject != original.Subject || copy.ContentType != original.ContentType || copy.DeadLetterReason != "" || copy.DeadLetterDescription != "" {
			t.Fatalf("topic replay changed business content in %s", target.Name())
		}
	}
	if err := d.Abandon(ctx); err != nil {
		t.Fatal(err)
	}
	retained := awaitPeekID(t, ctx, store, billing, true, original.ID)
	if retained.DeadLetterReason != original.DeadLetterReason || !bytes.Equal(retained.Body, original.Body) {
		t.Fatal("topic replay removed/changed original")
	}
	t.Log("PASS: original billing DLQ retained; replay reached BOTH billing and shipping; reseed required to remove test fanout copies")
}

func receiveOne(t *testing.T, parent context.Context, receiver *sb.Receiver) *sb.ReceivedMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	messages, err := receiver.ReceiveMessages(ctx, 1, nil)
	if err != nil || len(messages) != 1 {
		t.Fatalf("receive one: %d messages, %v", len(messages), err)
	}
	return messages[0]
}

func valueString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func awaitPeekID(t *testing.T, parent context.Context, store *azure.Store, target explorer.Resource, dlq bool, id string) explorer.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		var from int64
		for range 100 {
			page, err := store.Peek(ctx, target, dlq, from, 20)
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range page.Messages {
				if message.ID == id {
					return message
				}
			}
			if page.NextSequence == nil {
				break
			}
			from = *page.NextSequence
		}
		select {
		case <-ctx.Done():
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatalf("message %s not found in %s dlq=%v", id, target.Name(), dlq)
	return explorer.Message{}
}

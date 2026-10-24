//go:build integration

package azure_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebusexplorer/internal/azure"
	"servicebusexplorer/internal/explorer"
)

// Called only after seeded browsing and legacy mutations in the SERIAL opt-in
// emulator test. The empty source and invoices active queue must be unused.
func testSelectedQueueReplay(t *testing.T, ctx context.Context, store *azure.Store, client *sb.Client, source, destination explorer.Resource) {
	t.Helper()
	sender, err := client.NewSender(source.Queue, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close(context.Background())
	sourceReceiver, err := client.NewReceiverForQueue(source.Queue, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sourceReceiver.Close(context.Background())
	destinationReceiver, err := client.NewReceiverForQueue(destination.Queue, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer destinationReceiver.Close(context.Background())
	prefix := fmt.Sprintf("selected-%d", time.Now().UnixNano())
	body := bytes.Repeat([]byte{0, 255, 65, 10, 27}, 18000)
	ttl := 37 * time.Minute
	for i, payload := range [][]byte{[]byte("left-neighbor"), body, []byte("right-neighbor")} {
		id := fmt.Sprintf("%s-%d", prefix, i)
		if err := sender.SendMessage(ctx, &sb.Message{MessageID: &id, Body: payload, TimeToLive: &ttl, ApplicationProperties: map[string]any{"business": "selected", "number": int64(42), "enabled": true}}, nil); err != nil {
			t.Fatal(err)
		}
		message := receiveOne(t, ctx, sourceReceiver)
		if message.MessageID != id {
			t.Fatal("source queue was not isolated; reseed lab")
		}
		if err := sourceReceiver.DeadLetterMessage(ctx, message, nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.Peek(ctx, source, true, 0, 20)
	if err != nil || len(page.Messages) != 3 {
		t.Fatalf("source DLQ: %+v %v", page, err)
	}
	selected := page.Messages[1]
	if selected.ID != prefix+"-1" || !selected.PreviewTruncated {
		t.Fatal("expected non-first 90KB selection")
	}
	prepared, err := store.PrepareDLQCopy(ctx, source, selected.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	preview := prepared.Snapshot()
	preview.Body[0] = 99
	preview.Properties["business"] = "changed"
	copyID := prefix + "-copy"
	if result, err := prepared.Resend(ctx, destination, copyID); err != nil || result.State != "confirmed" {
		t.Fatalf("prepared send: %+v %v", result, err)
	}
	if _, err := prepared.Resend(ctx, destination, copyID+"-again"); err == nil {
		t.Fatal("prepared replay sent twice")
	}
	assertCopy := func(id string) {
		t.Helper()
		copy := receiveOne(t, ctx, destinationReceiver)
		if copy.MessageID != id || !bytes.Equal(copy.Body, body) || copy.TimeToLive == nil || *copy.TimeToLive != ttl || copy.ApplicationProperties["business"] != "selected" || copy.ApplicationProperties["number"] != int64(42) || copy.ApplicationProperties["enabled"] != true {
			t.Fatalf("destination full copy differs: id=%s bytes=%d ttl=%v properties=%v", copy.MessageID, len(copy.Body), copy.TimeToLive, copy.ApplicationProperties)
		}
		if _, exists := copy.ApplicationProperties["DeadLetterReason"]; exists {
			t.Fatal("copy retained DLQ reason")
		}
		if err := destinationReceiver.CompleteMessage(ctx, copy, nil); err != nil {
			t.Fatal(err)
		}
	}
	assertCopy(copyID)
	afterCopy, err := store.Peek(ctx, source, true, 0, 20)
	if err != nil || len(afterCopy.Messages) != 3 {
		t.Fatalf("copy removed an original: %+v %v", afterCopy, err)
	}
	for i := range page.Messages {
		if afterCopy.Messages[i].ID != page.Messages[i].ID || afterCopy.Messages[i].DeliveryCount != page.Messages[i].DeliveryCount {
			t.Fatal("copy-only acquired or changed originals")
		}
	}
	delivery, err := store.ReceiveSelectedDLQ(ctx, source, selected.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	defer delivery.Abandon(context.Background())
	if delivery.Snapshot().ID != selected.ID {
		t.Fatal("selected acquisition substituted another original")
	}
	cleanID := prefix + "-clean"
	if result, err := delivery.Resend(ctx, destination, cleanID); err != nil || result.State != "confirmed" {
		t.Fatalf("selected send: %+v %v", result, err)
	}
	if err := delivery.Complete(ctx); err != nil {
		t.Fatal(err)
	}
	assertCopy(cleanID)
	// Independent SDK receives prove both neighbors are available and unchanged.
	dead, err := client.NewReceiverForQueue(source.Queue, &sb.ReceiverOptions{SubQueue: sb.SubQueueDeadLetter})
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close(context.Background())
	want := map[string]string{prefix + "-0": "left-neighbor", prefix + "-2": "right-neighbor"}
	for range 2 {
		message := receiveOne(t, ctx, dead)
		payload, exists := want[message.MessageID]
		if !exists || !bytes.Equal(message.Body, []byte(payload)) {
			t.Fatalf("wrong/deleted neighbor: %s", message.MessageID)
		}
		delete(want, message.MessageID)
		if err := dead.CompleteMessage(ctx, message, nil); err != nil {
			t.Fatal(err)
		}
	}
	remaining, err := store.Peek(ctx, source, true, 0, 20)
	if err != nil || len(remaining.Messages) != 0 {
		t.Fatalf("selected original or extra messages remain: %+v %v", remaining, err)
	}
	t.Log("PASS: non-first 90KB full-byte selected copy to different queue retained all originals without acquisition; selected cleanup deleted only target; independent receives verified both neighbors")
}

func testSelectedSubscriptionToQueue(t *testing.T, ctx context.Context, store *azure.Store, client *sb.Client, source, destination explorer.Resource) {
	t.Helper()
	before, err := store.Peek(ctx, source, true, 0, 20)
	if err != nil || len(before.Messages) < 3 {
		t.Fatalf("need asymmetric subscription DLQ neighbors: %+v %v", before, err)
	}
	selected := before.Messages[1]
	delivery, err := store.ReceiveSelectedDLQ(ctx, source, selected.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	defer delivery.Abandon(context.Background())
	if delivery.Snapshot().ID != selected.ID {
		t.Fatal("subscription selection substituted next")
	}
	id := fmt.Sprintf("selected-subscription-%d", time.Now().UnixNano())
	if result, err := delivery.Resend(ctx, destination, id); err != nil || result.State != "confirmed" {
		t.Fatalf("subscription to queue: %+v %v", result, err)
	}
	if err := delivery.Complete(ctx); err != nil {
		t.Fatal(err)
	}
	receiver, err := client.NewReceiverForQueue(destination.Queue, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close(context.Background())
	copy := receiveOne(t, ctx, receiver)
	if copy.MessageID != id || !bytes.Equal(copy.Body, selected.Body) || copy.ApplicationProperties["region"] != selected.Properties["region"] {
		t.Fatal("subscription copy did not arrive unchanged at queue")
	}
	if err := receiver.CompleteMessage(ctx, copy, nil); err != nil {
		t.Fatal(err)
	}
	after, err := store.Peek(ctx, source, true, 0, 20)
	if err != nil || len(after.Messages) != len(before.Messages)-1 {
		t.Fatalf("subscription cleanup: %+v %v", after, err)
	}
	for _, message := range after.Messages {
		if message.Sequence == selected.Sequence {
			t.Fatal("selected subscription original retained after cleanup")
		}
	}
	// Independent receive verifies the left neighbor was released, not completed.
	dead, err := client.NewReceiverForSubscription(source.Topic, source.Subscription, &sb.ReceiverOptions{SubQueue: sb.SubQueueDeadLetter})
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close(context.Background())
	neighbor := receiveOne(t, ctx, dead)
	if neighbor.MessageID != before.Messages[0].ID || !bytes.Equal(neighbor.Body, before.Messages[0].Body) {
		t.Fatal("subscription neighbor changed or remained locked")
	}
	if err := dead.AbandonMessage(ctx, neighbor, nil); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS: non-first subscription DLQ target resent to a queue, not topic fanout; only selected original deleted; neighbor independently available")
}

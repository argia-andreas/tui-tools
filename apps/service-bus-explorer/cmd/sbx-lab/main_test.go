package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"servicebusexplorer/internal/fixture"
)

func TestCommandGuards(t *testing.T) {
	for _, args := range [][]string{
		{"seed"}, {"seed", "--confirm-disposable"}, {"send"}, {"drain"}, {"dead-letter"},
		{"peek", "--endpoint", "cloud.invalid"}, {"peek", "--connection-string", "Endpoint=sb://cloud.invalid"},
		{"peek", "--entity", "sb://cloud.invalid/orders"}, {"peek", "--entity", "events"},
		{"send", "--confirm-disposable", "--entity", "events/billing"},
		{"peek", "--max", "0"}, {"peek", "--max", "101"}, {"peek", "--from", "-1"},
		{"drain", "--confirm-disposable", "--max", "10001"},
		{"dead-letter", "--confirm-disposable", "--dlq"}, {"peek", "unexpected"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parse(args, io.Discard); err == nil {
				t.Fatal("unsafe command accepted")
			}
		})
	}
	for _, args := range [][]string{
		{"seed", "--confirm-disposable", "--accept-eula", "--sudo", "--orb"},
		{"peek", "--entity", "events/shipping", "--dlq", "--from", "100"},
		{"send", "--confirm-disposable", "--entity", "events"},
		{"drain", "--confirm-disposable", "--max", "10000"},
	} {
		if _, err := parse(args, io.Discard); err != nil {
			t.Fatalf("valid command %v: %v", args, err)
		}
	}
	if !strings.Contains(connectionString, "Endpoint=sb://127.0.0.1:5673;") || !strings.Contains(connectionString, "UseDevelopmentEmulator=true;") {
		t.Fatal("fixed emulator target changed")
	}
}

func TestLabRootCannotFallThroughToParentProject(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Dir(root))
	if found, err := moduleRoot(); err == nil {
		t.Fatalf("must not run another project's scripts/lab.py: %s", found)
	}
}

func TestScenarioSeedIDsBodiesAndPaging(t *testing.T) {
	s, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	r, _ := resource("orders", false)
	for _, dlq := range []bool{false, true} {
		messages, err := seedMessages(s, r, dlq)
		if err != nil {
			t.Fatal(err)
		}
		expected, kind := 128, "active"
		if dlq {
			expected, kind = 12, "dlq"
		}
		if len(messages) != expected {
			t.Fatalf("got %d messages want %d", len(messages), expected)
		}
		if messages[0].ID != "orders-"+kind+"-0001" {
			t.Fatal(messages[0].ID)
		}
		var body map[string]any
		if err := json.Unmarshal(messages[0].Body, &body); err != nil {
			t.Fatal(err)
		}
		if body["id"] != messages[0].ID || body["amount"] != 49.95 || body["event"] != "order.created" || body["fixture"] != true {
			t.Fatalf("unexpected seed body %v", body)
		}
		if !strings.Contains(string(messages[0].Body), "\n") {
			t.Fatal("JSON lost its legitimate newlines")
		}
	}
	notifications, _ := resource("notifications", false)
	text, err := seedMessages(s, notifications, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(text) != 418 || string(text[0].Body) != "Local lab notification\nsecond line\n" || !strings.Contains(string(text[1].Body), "\x1b]0;not-a-title\x07") || len(text[1].Subject) != 156 {
		t.Fatal("text/control/long-subject seed coverage changed")
	}
	billing, _ := resource("events/billing", false)
	active, _ := seedMessages(s, billing, false)
	dead, _ := seedMessages(s, billing, true)
	shipping, _ := resource("events/shipping", false)
	shippingCounts, _ := s.Counts(context.Background(), shipping)
	if len(active)+len(dead) != 8 || int64(len(active)+len(dead))-shippingCounts.DeadLetter != 5 {
		t.Fatal("topic fanout expectation must be billing 7/1 and shipping 5/3, not fixture's synthetic 2/3")
	}
}

type fakeReceiver struct {
	remaining, calls, attempts, failAt int
	dead                               bool
}

func (f *fakeReceiver) ReceiveMessages(_ context.Context, max int, _ *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	if max != 1 {
		panic("must receive only one held message")
	}
	f.calls++
	if f.remaining == 0 {
		return nil, context.DeadlineExceeded
	}
	f.remaining--
	return []*azservicebus.ReceivedMessage{{}}, nil
}

func (f *fakeReceiver) CompleteMessage(context.Context, *azservicebus.ReceivedMessage, *azservicebus.CompleteMessageOptions) error {
	f.attempts++
	if f.failAt == f.attempts {
		return errors.New("lost acknowledgement")
	}
	return nil
}

func (f *fakeReceiver) DeadLetterMessage(ctx context.Context, m *azservicebus.ReceivedMessage, options *azservicebus.DeadLetterOptions) error {
	f.dead = options.Reason != nil && *options.Reason == "test-reason"
	return f.CompleteMessage(ctx, m, nil)
}

func TestBoundedSettlementReportsActualAcknowledgedCount(t *testing.T) {
	for _, dead := range []bool{false, true} {
		for _, tc := range []struct{ remaining, max, fail, want, calls int }{
			{10, 3, 0, 3, 3}, {2, 5, 0, 2, 3}, {0, 5, 0, 0, 1}, {5, 5, 2, 1, 2},
		} {
			f := &fakeReceiver{remaining: tc.remaining, failAt: tc.fail}
			got, err := settleReceiver(f, dead, tc.max, "test-reason")
			if got != tc.want || f.calls != tc.calls || (err != nil) != (tc.fail != 0) {
				t.Fatalf("dead=%v case=%+v got=%d err=%v calls=%d", dead, tc, got, err, f.calls)
			}
			if dead && tc.want > 0 && !f.dead {
				t.Fatal("dead-letter reason not passed to actual settlement")
			}
		}
	}
}

package azure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"github.com/Azure/go-amqp"
	"servicebusexplorer/internal/explorer"
)

func ptr[T any](v T) *T { return &v }

func standardDeliveryMessage() *sb.ReceivedMessage {
	body := []byte{0, 1, 255, 10, 27}
	return &sb.ReceivedMessage{
		MessageID: "original", Body: body, ContentType: ptr("application/octet-stream"),
		CorrelationID: ptr("correlation"), Subject: ptr("subject"), ReplyTo: ptr("replies"),
		ReplyToSessionID: ptr("reply-session"), SessionID: ptr("session-metadata"), To: ptr("logical-to"), PartitionKey: ptr("partition"),
		TimeToLive: ptr(90 * time.Minute), ScheduledEnqueueTime: ptr(time.Now().Add(time.Hour)), LockedUntil: ptr(time.Now().Add(time.Minute)),
		ApplicationProperties: map[string]any{"business": "value", "binary": []byte{5, 6}, "number": int64(42), "DeadLetterReason": "failed", "DeadLetterErrorDescription": "details"},
		DeadLetterReason:      ptr("failed"), DeadLetterErrorDescription: ptr("details"),
		RawAMQPMessage: &sb.AMQPAnnotatedMessage{Body: sb.AMQPAnnotatedMessageBody{Data: [][]byte{body}},
			Properties: &sb.AMQPAnnotatedMessageProperties{MessageID: "original", CorrelationID: "correlation"}},
	}
}

func TestReplayConversionPreservesAndClones(t *testing.T) {
	m := standardDeliveryMessage()
	out, err := replayMessage(m, "replay-1")
	if err != nil {
		t.Fatal(err)
	}
	if *out.MessageID != "replay-1" || *out.CorrelationID != "correlation" || *out.TimeToLive != 90*time.Minute || out.ScheduledEnqueueTime != nil {
		t.Fatalf("identity/lifetime conversion: %+v", out)
	}
	if !bytes.Equal(out.Body, []byte{0, 1, 255, 10, 27}) || !reflect.DeepEqual(out.ApplicationProperties, map[string]any{"business": "value", "binary": []byte{5, 6}, "number": int64(42)}) {
		t.Fatalf("body/properties changed: %+v", out)
	}
	for name, pair := range map[string][2]*string{
		"content-type": {out.ContentType, m.ContentType}, "correlation": {out.CorrelationID, m.CorrelationID},
		"partition": {out.PartitionKey, m.PartitionKey}, "reply-to": {out.ReplyTo, m.ReplyTo}, "reply-session": {out.ReplyToSessionID, m.ReplyToSessionID},
		"session": {out.SessionID, m.SessionID}, "subject": {out.Subject, m.Subject}, "to": {out.To, m.To},
	} {
		if pair[0] == pair[1] || *pair[0] != *pair[1] {
			t.Fatalf("%s not independently preserved", name)
		}
		*pair[0] = "changed"
		if *pair[1] == "changed" {
			t.Fatalf("%s aliases source", name)
		}
	}
	out.Body[0] = 99
	out.ApplicationProperties["binary"].([]byte)[0] = 99
	out.ApplicationProperties["business"] = "changed"
	*out.TimeToLive = time.Second
	if m.Body[0] != 0 || m.ApplicationProperties["binary"].([]byte)[0] != 5 || m.ApplicationProperties["business"] != "value" || *m.TimeToLive != 90*time.Minute || m.MessageID != "original" {
		t.Fatal("outgoing message mutated held delivery")
	}
}

func TestReplayPreviewCannotTruncateOrMutateRaw(t *testing.T) {
	m := standardDeliveryMessage()
	m.Body = bytes.Repeat([]byte{0, 255, 65}, 30000)
	m.RawAMQPMessage.Body.Data[0] = m.Body
	d := &delivery{message: m}
	preview := d.Snapshot()
	if len(preview.Body) != 64*1024 || !preview.PreviewTruncated {
		t.Fatal("missing preview bound")
	}
	preview.Body[0] = 42
	preview.Properties["binary"].([]byte)[0] = 42
	preview.Properties["business"] = "changed"
	out, err := replayMessage(m, "replay-full")
	if err != nil || !bytes.Equal(out.Body, bytes.Repeat([]byte{0, 255, 65}, 30000)) || out.ApplicationProperties["binary"].([]byte)[0] != 5 {
		t.Fatalf("preview corrupted full replay: %v", err)
	}
	if d.Snapshot().Properties["business"] != "value" {
		t.Fatal("preview map aliases held properties")
	}
}

func TestFullSnapshotPreservesContentAndMetadataIndependently(t *testing.T) {
	m := standardDeliveryMessage()
	m.Body = bytes.Repeat([]byte("full text é\n"), 9000)
	m.SequenceNumber = ptr(int64(123))
	m.EnqueuedTime = ptr(time.Date(2026, 10, 24, 1, 2, 3, 0, time.UTC))
	m.DeliveryCount = 7
	want := (&delivery{message: m}).Snapshot()
	want.Body = bytes.Clone(m.Body)
	want.PreviewTruncated = false
	full := snapshot(m, false)
	if !reflect.DeepEqual(full, want) {
		t.Fatal("full snapshot lost bytes, identity or metadata")
	}
	full.Body[0] = '!'
	full.Properties["binary"].([]byte)[0] = 42
	full.Properties["business"] = "changed"
	if !reflect.DeepEqual(snapshot(m, false), want) || m.Body[0] != 'f' || m.ApplicationProperties["binary"].([]byte)[0] != 5 {
		t.Fatal("full snapshot aliases source")
	}
	preview := (&delivery{message: m}).Snapshot()
	if len(preview.Body) != 64*1024 || !preview.PreviewTruncated {
		t.Fatal("full snapshot changed delivery preview contract")
	}
}

func TestPeekMessageRejectsInvalidIdentityAndCancellationBeforeAMQP(t *testing.T) {
	s := &Store{} // nil client: validation must not open a receiver
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.PeekMessage(ctx, resource("orders", "", ""), false, 123); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled full peek: %v", err)
	}
	for _, dlq := range []bool{false, true} {
		if _, err := s.PeekMessage(context.Background(), explorer.Resource{}, dlq, 123); err == nil {
			t.Fatal("accepted invalid resource")
		}
		if _, err := s.PeekMessage(context.Background(), resource("orders", "", ""), dlq, -1); err == nil {
			t.Fatal("accepted negative sequence")
		}
	}
}

func TestReplayRejectsUnsupportedRepresentation(t *testing.T) {
	for name, mutate := range map[string]func(*sb.ReceivedMessage){
		"missing raw":            func(m *sb.ReceivedMessage) { m.RawAMQPMessage = nil },
		"multiple data":          func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Body.Data = [][]byte{{1}, {2}} },
		"no data":                func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Body.Data = nil },
		"value":                  func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Body.Value = "value" },
		"sequence":               func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Body.Sequence = [][]any{{"item"}} },
		"numeric message ID":     func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.MessageID = uint64(123) },
		"binary message ID":      func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.MessageID = []byte{1} },
		"UUID correlation ID":    func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.CorrelationID = amqp.UUID{1} },
		"numeric correlation ID": func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.CorrelationID = uint64(7) },
		"encoding":               func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.ContentEncoding = ptr("gzip") },
		"group sequence":         func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.GroupSequence = ptr(uint32(7)) },
		"user ID":                func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Properties.UserID = []byte("user") },
		"footer":                 func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Footer = map[any]any{"checksum": "x"} },
		"custom annotation":      func(m *sb.ReceivedMessage) { m.RawAMQPMessage.MessageAnnotations = map[any]any{"business": "x"} },
		"priority":               func(m *sb.ReceivedMessage) { m.RawAMQPMessage.Header = &sb.AMQPAnnotatedMessageHeader{Priority: 9} },
		"compound app value":     func(m *sb.ReceivedMessage) { m.ApplicationProperties["nested"] = map[string]any{"x": "y"} },
	} {
		t.Run(name, func(t *testing.T) {
			m := standardDeliveryMessage()
			mutate(m)
			if _, err := replayMessage(m, "new"); err == nil {
				t.Fatal("accepted lossy conversion")
			}
		})
	}
	for _, id := range []string{"", "original", strings.Repeat("x", 129), string([]byte{255})} {
		if _, err := replayMessage(standardDeliveryMessage(), id); err == nil {
			t.Fatal("accepted invalid replay identity")
		}
	}
	// Empty Data is still a legitimate single section. Absent IDs are not a
	// non-string representation, and nil optional fields must remain nil.
	m := &sb.ReceivedMessage{RawAMQPMessage: &sb.AMQPAnnotatedMessage{Body: sb.AMQPAnnotatedMessageBody{Data: [][]byte{{}}}}}
	if out, err := replayMessage(m, "new"); err != nil || out.CorrelationID != nil || out.TimeToLive != nil {
		t.Fatalf("optional fields: %+v, %v", out, err)
	}
}

type settlementProbe struct {
	complete, abandon, closed int
	err                       error
	deadline                  time.Time
	closeDeadline             time.Time
	message                   *sb.ReceivedMessage
}

func (p *settlementProbe) CompleteMessage(ctx context.Context, m *sb.ReceivedMessage, _ *sb.CompleteMessageOptions) error {
	p.complete++
	p.deadline, _ = ctx.Deadline()
	p.message = m
	return p.err
}
func (p *settlementProbe) AbandonMessage(ctx context.Context, m *sb.ReceivedMessage, _ *sb.AbandonMessageOptions) error {
	p.abandon++
	p.deadline, _ = ctx.Deadline()
	p.message = m
	return p.err
}
func (p *settlementProbe) Close(ctx context.Context) error {
	p.closed++
	p.closeDeadline, _ = ctx.Deadline()
	return nil
}

func TestSettlementOnceClosesAndReleasesEvenOnFailure(t *testing.T) {
	for _, complete := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			p := &settlementProbe{}
			if fail {
				p.err = errors.New("transport timeout")
			}
			released := 0
			m := standardDeliveryMessage()
			m.LockedUntil = ptr(time.Now().Add(2 * time.Second))
			d := &delivery{message: m, receiver: p, release: func() { released++ }}
			err := d.settle(context.Background(), complete)
			if (err != nil) != fail || p.closed != 1 || released != 1 || p.message != m || p.deadline.After(*m.LockedUntil) || p.closeDeadline.After(*m.LockedUntil) {
				t.Fatalf("settlement contract: %+v, %v", p, err)
			}
			if complete && p.complete != 1 || !complete && p.abandon != 1 {
				t.Fatal("wrong settlement")
			}
			if err := d.Abandon(context.Background()); err == nil || p.closed != 1 || released != 1 {
				t.Fatal("settlement retried")
			}
			if result, err := d.Resend(context.Background(), resource("orders", "", ""), "another"); err == nil || result.State != "rejected" {
				t.Fatal("settled delivery resent")
			}
		}
	}
}

func TestDeliveryDeadlineCancellationAndSingleFlight(t *testing.T) {
	for _, kind := range []string{"caller", "expired", "missing-lock"} {
		m := standardDeliveryMessage()
		ctx := context.Background()
		if kind == "caller" {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		if kind == "expired" {
			m.LockedUntil = ptr(time.Now().Add(-time.Second))
		}
		if kind == "missing-lock" {
			m.LockedUntil = nil
		}
		p := &settlementProbe{}
		d := &delivery{message: m, receiver: p, release: func() {}}
		if result, err := d.Resend(ctx, resource("orders", "", ""), "new"); err == nil || result.State != "rejected" {
			t.Fatalf("%s allowed send", kind)
		}
		if err := d.Complete(ctx); err == nil || p.complete != 0 || p.closed != 1 {
			t.Fatalf("%s settlement not canceled safely", kind)
		}
	}
	d := &delivery{message: standardDeliveryMessage()}
	d.mu.Lock()
	if _, err := d.Resend(context.Background(), resource("orders", "", ""), "new"); err == nil {
		t.Fatal("concurrent send accepted")
	}
	if err := d.Complete(context.Background()); err == nil {
		t.Fatal("concurrent settlement accepted")
	}
	d.mu.Unlock()
	if _, err := d.Resend(context.Background(), resource("orders", "", ""), "original"); err == nil {
		t.Fatal("same ID accepted")
	}
	if result, err := d.Resend(context.Background(), resource("orders", "", ""), "new"); err == nil || result.State != "rejected" {
		t.Fatal("validation failure allowed a second attempt")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	op, stop := d.operationContext(ctx)
	defer stop()
	deadline, _ := op.Deadline()
	caller, _ := ctx.Deadline()
	if deadline.After(caller) {
		t.Fatal("operation extended caller deadline")
	}
	p := &settlementProbe{}
	d = &delivery{message: standardDeliveryMessage(), receiver: p, release: func() {}}
	if err := d.Abandon(ctx); err != nil || p.deadline.After(caller) || p.closeDeadline.After(caller) {
		t.Fatal("settlement or close extended caller deadline")
	}
}

type adminResponseTransport struct {
	body     string
	requests int
}

func (p *adminResponseTransport) Do(r *http.Request) (*http.Response, error) {
	p.requests++
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/atom+xml"}}, Body: io.NopCloser(strings.NewReader(p.body)), Request: r}, nil
}

func TestReceiveRejectsSessionEntitiesBeforeAMQP(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		kind := "QueueDescription"
		r := resource("orders", "", "")
		if subscription {
			kind = "SubscriptionDescription"
			r = resource("", "events", "billing")
		}
		transport := &adminResponseTransport{body: `<entry xmlns="http://www.w3.org/2005/Atom"><content type="application/xml"><` + kind + ` xmlns="http://schemas.microsoft.com/netservices/2010/10/servicebus/connect"><RequiresSession>true</RequiresSession></` + kind + `></content></entry>`}
		management, err := admin.NewClientFromConnectionString("Endpoint=sb://"+managementAddress+emulatorCredentials, &admin.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: transport, Retry: policy.RetryOptions{MaxRetries: -1}}})
		if err != nil {
			t.Fatal(err)
		}
		s := &Store{admin: management} // nil messaging client: must never open AMQP
		if _, err := s.ReceiveNextDLQ(context.Background(), r); err == nil || !strings.Contains(err.Error(), "session-enabled") || transport.requests != 1 || s.held {
			t.Fatalf("session validation: err=%v requests=%d held=%v", err, transport.requests, s.held)
		}
		if _, err := s.PrepareDLQCopy(context.Background(), r, 1); err == nil || !strings.Contains(err.Error(), "session-enabled") || transport.requests != 2 {
			t.Fatalf("selected copy session validation: %v", err)
		}
		if _, err := s.ReceiveSelectedDLQ(context.Background(), r, 1); err == nil || !strings.Contains(err.Error(), "session-enabled") || transport.requests != 3 || s.held {
			t.Fatalf("selected acquisition session validation: %v", err)
		}
		if !subscription {
			if _, err := s.replaySender(r, false)(context.Background(), r); err == nil || !strings.Contains(err.Error(), "session-enabled") || transport.requests != 4 {
				t.Fatalf("destination session validation: %v", err)
			}
		}
	}
}

func TestMutationResourceBoundary(t *testing.T) {
	for _, name := range []string{"", ".", "..", "orders/$DeadLetterQueue", "https://host", "orders?x", "orders%2fother", "orders\n"} {
		if validResource(resource(name, "", "")) {
			t.Fatalf("accepted entity path %q", name)
		}
	}
	s := &Store{held: true}
	if _, err := s.ReceiveNextDLQ(context.Background(), resource("orders", "", "")); err == nil {
		t.Fatal("acquired concurrent held delivery")
	}
}

type sendProbe struct {
	err           error
	calls, closed int
	message       *sb.Message
	deadline      time.Time
}

func (p *sendProbe) SendMessage(ctx context.Context, message *sb.Message, _ *sb.SendMessageOptions) error {
	p.calls++
	p.message = message
	p.deadline, _ = ctx.Deadline()
	return p.err
}

func (p *sendProbe) Close(context.Context) error { p.closed++; return nil }

func TestReplayOutcomesAreTerminalAndNeverSettleOriginal(t *testing.T) {
	for _, test := range []struct {
		name, state string
		err         error
	}{
		{"acknowledged", "confirmed", nil},
		{"lost acknowledgement", "unknown", context.DeadlineExceeded},
		{"transport failure", "unknown", errors.New("sensitive SDK error")},
		{"explicit size rejection", "rejected", sb.ErrMessageTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			sender := &sendProbe{err: test.err}
			receiver := &settlementProbe{}
			opens, releases := 0, 0
			m := standardDeliveryMessage()
			d := &delivery{message: m, receiver: receiver, release: func() { releases++ }, newSender: func(context.Context, explorer.Resource) (deliverySender, error) { opens++; return sender, nil }}
			result, err := d.Resend(context.Background(), resource("orders", "", ""), "replay")
			if result.State != test.state || result.MessageID != "replay" || (err != nil) != (test.err != nil) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if err != nil && strings.Contains(err.Error(), "sensitive") {
				t.Fatal("leaked SDK error")
			}
			if sender.calls != 1 || sender.closed != 1 || opens != 1 || releases != 0 || receiver.closed != 0 || receiver.complete != 0 || receiver.abandon != 0 {
				t.Fatal("send retried or settled/released original")
			}
			if *sender.message.MessageID != "replay" || !bytes.Equal(sender.message.Body, m.Body) || sender.deadline.After(*m.LockedUntil) || sender.deadline.After(time.Now().Add(10*time.Second)) {
				t.Fatal("wrong send body/identity/deadline")
			}
			if second, err := d.Resend(context.Background(), resource("orders", "", ""), "another"); err == nil || second.State != "rejected" || sender.calls != 1 || opens != 1 {
				t.Fatal("terminal send retried")
			}
			// Retaining the original still requires explicit bounded release,
			// including after an ambiguous send. No second send is involved.
			if err := d.Abandon(context.Background()); err != nil || receiver.abandon != 1 || receiver.closed != 1 || releases != 1 {
				t.Fatalf("release after send: %v", err)
			}
		})
	}
}

func TestReplaySenderOpenFailureIsRejectedWithoutRetry(t *testing.T) {
	opens := 0
	d := &delivery{message: standardDeliveryMessage(), newSender: func(context.Context, explorer.Resource) (deliverySender, error) {
		opens++
		return nil, errors.New("sensitive SDK error")
	}}
	result, err := d.Resend(context.Background(), resource("orders", "", ""), "replay")
	if err == nil || result.State != "rejected" || opens != 1 || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("open result=%+v err=%v", result, err)
	}
	if _, err := d.Resend(context.Background(), resource("orders", "", ""), "another"); err == nil || opens != 1 {
		t.Fatal("sender open failure retried")
	}
}

func TestPreparedReplayUsesFullContentWithoutLockAndOnlyOneAttempt(t *testing.T) {
	m := standardDeliveryMessage()
	m.LockedUntil = nil
	m.Body = bytes.Repeat([]byte{0, 255, 65}, 30000)
	m.RawAMQPMessage.Body.Data[0] = m.Body
	destination := resource("empty", "", "")
	sender := &sendProbe{}
	d := &delivery{message: m, copyOnly: true, newSender: func(ctx context.Context, got explorer.Resource) (deliverySender, error) {
		if got != destination {
			t.Fatal("lost destination identity")
		}
		return sender, nil
	}}
	preview := d.Snapshot()
	preview.Body[0] = 99
	preview.Properties["binary"].([]byte)[0] = 99
	result, err := d.Resend(context.Background(), destination, "copy")
	if err != nil || result.State != "confirmed" || !bytes.Equal(sender.message.Body, bytes.Repeat([]byte{0, 255, 65}, 30000)) || sender.message.ApplicationProperties["binary"].([]byte)[0] != 5 {
		t.Fatalf("full prepared copy: result=%+v err=%v", result, err)
	}
	if _, err := d.Resend(context.Background(), destination, "duplicate"); err == nil {
		t.Fatal("prepared copy sent twice")
	}
}

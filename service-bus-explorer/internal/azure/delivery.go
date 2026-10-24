package azure

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/go-amqp"
	"servicebusexplorer/internal/explorer"
)

type deliveryReceiver interface {
	CompleteMessage(context.Context, *sb.ReceivedMessage, *sb.CompleteMessageOptions) error
	AbandonMessage(context.Context, *sb.ReceivedMessage, *sb.AbandonMessageOptions) error
	Close(context.Context) error
}

type delivery struct {
	mu                     sync.Mutex
	message                *sb.ReceivedMessage // full SDK delivery, never the display preview
	receiver               deliveryReceiver
	client                 *sb.Client
	destination            string
	release                func()
	sendAttempted, settled bool
}

func (d *delivery) LockedUntil() time.Time { return value(d.message.LockedUntil) }

func (d *delivery) Snapshot() explorer.Message {
	m := d.message
	body := m.Body
	truncated := len(body) > 64*1024
	if truncated {
		body = body[:64*1024]
	}
	properties := make(map[string]any, len(m.ApplicationProperties))
	for k, v := range m.ApplicationProperties {
		// Received application properties are AMQP scalars; binary is the only
		// mutable supported scalar. Never share it with a preview consumer.
		if b, ok := v.([]byte); ok {
			v = slices.Clone(b)
		}
		properties[k] = v
	}
	return explorer.Message{ID: m.MessageID, Subject: value(m.Subject), ContentType: value(m.ContentType),
		Sequence: value(m.SequenceNumber), Body: slices.Clone(body), Properties: properties,
		EnqueuedAt: value(m.EnqueuedTime), DeliveryCount: m.DeliveryCount,
		DeadLetterReason: value(m.DeadLetterReason), DeadLetterDescription: value(m.DeadLetterErrorDescription), PreviewTruncated: truncated}
}

func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// replayMessage follows azservicebus v1.11.0 message.go and amqp_message.go:
// ReceivedMessage.Message() is shallow, and non-string IDs and non-single-Data
// bodies are silently absent from its convenience fields. Inspect raw first.
func replayMessage(m *sb.ReceivedMessage, id string) (*sb.Message, error) {
	if id == "" || id == m.MessageID || !utf8.ValidString(id) || len(id) > 128 {
		return nil, errors.New("replay requires a different nonempty message ID of at most 128 bytes")
	}
	raw := m.RawAMQPMessage
	if raw == nil || len(raw.Body.Data) != 1 || raw.Body.Value != nil || len(raw.Body.Sequence) != 0 {
		return nil, errors.New("replay supports only a single AMQP Data body section")
	}
	if raw.Properties != nil {
		p := raw.Properties
		for _, v := range []any{p.MessageID, p.CorrelationID} {
			if v != nil {
				if _, ok := v.(string); !ok {
					return nil, errors.New("replay supports only string AMQP identifiers")
				}
			}
		}
		if p.ContentEncoding != nil || p.GroupSequence != nil || len(p.UserID) != 0 {
			return nil, errors.New("replay cannot preserve advanced AMQP properties")
		}
	}
	if len(raw.Footer) != 0 {
		return nil, errors.New("replay cannot preserve AMQP footers")
	}
	if raw.Header != nil && raw.Header.Priority != 0 && raw.Header.Priority != 4 {
		return nil, errors.New("replay cannot preserve custom AMQP priority")
	}
	for k := range raw.MessageAnnotations {
		switch k {
		case "x-opt-partition-key", "x-opt-scheduled-enqueue-time", "x-opt-locked-until", "x-opt-sequence-number", "x-opt-enqueued-time", "x-opt-deadletter-source", "x-opt-enqueue-sequence-number", "x-opt-message-state":
		default:
			return nil, errors.New("replay cannot preserve custom AMQP annotations")
		}
	}
	out := m.Message()
	out.Body = slices.Clone(raw.Body.Data[0])
	out.MessageID = clonePointer(&id)
	out.ContentType = clonePointer(m.ContentType)
	out.CorrelationID = clonePointer(m.CorrelationID)
	out.PartitionKey = clonePointer(m.PartitionKey)
	out.ReplyTo = clonePointer(m.ReplyTo)
	out.ReplyToSessionID = clonePointer(m.ReplyToSessionID)
	out.SessionID = clonePointer(m.SessionID)
	out.Subject = clonePointer(m.Subject)
	out.TimeToLive = clonePointer(m.TimeToLive)
	out.To = clonePointer(m.To)
	out.ScheduledEnqueueTime = nil
	out.ApplicationProperties = make(map[string]any, len(m.ApplicationProperties))
	for k, v := range m.ApplicationProperties {
		if k == "DeadLetterReason" || k == "DeadLetterErrorDescription" {
			continue
		}
		switch x := v.(type) {
		case nil, bool, string, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64, time.Time, amqp.UUID:
		case []byte:
			v = slices.Clone(x)
		default:
			return nil, errors.New("replay contains unsupported application property representation")
		}
		out.ApplicationProperties[k] = v
	}
	return out, nil
}

func (d *delivery) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(10 * time.Second)
	if lock := d.LockedUntil(); lock.Before(deadline) {
		deadline = lock
	}
	return context.WithDeadline(ctx, deadline)
}

func (d *delivery) Resend(ctx context.Context, id string) (explorer.SendResult, error) {
	result := explorer.SendResult{State: "rejected", MessageID: id}
	if !d.mu.TryLock() {
		return result, errors.New("delivery operation already in progress")
	}
	defer d.mu.Unlock()
	if d.settled || d.sendAttempted {
		return result, errors.New("delivery is settled or resend already attempted")
	}
	// Even validation failure is terminal for this review: no automatic retry.
	d.sendAttempted = true
	message, err := replayMessage(d.message, id)
	if err != nil {
		return result, err
	}
	ctx, cancel := d.operationContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	sender, err := d.client.NewSender(d.destination, nil)
	if err != nil {
		return result, errors.New("cannot open replay sender")
	}
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = sender.Close(closeCtx)
	}()
	if err := sender.SendMessage(ctx, message, nil); err != nil {
		// No transport error proves rejection. A lost acknowledgement may hide
		// broker acceptance. The SDK's explicit message-size rejection is known.
		if errors.Is(err, sb.ErrMessageTooLarge) {
			return result, errors.New("replay rejected: message exceeds link size")
		}
		result.State = "unknown"
		return result, errors.New("replay outcome unknown; check destination before retrying")
	}
	result.State = "confirmed"
	return result, nil
}

func closeReceiver(receiver deliveryReceiver) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = receiver.Close(ctx)
}

func (d *delivery) settle(ctx context.Context, complete bool) error {
	if !d.mu.TryLock() {
		return errors.New("delivery operation already in progress")
	}
	defer d.mu.Unlock()
	if d.settled {
		return errors.New("delivery settlement already attempted")
	}
	d.settled = true
	defer d.release()
	defer closeReceiver(d.receiver)
	ctx, cancel := d.operationContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	if complete {
		err = d.receiver.CompleteMessage(ctx, d.message, nil)
	} else {
		err = d.receiver.AbandonMessage(ctx, d.message, nil)
	}
	if err != nil {
		return errors.New("settlement unconfirmed; message may remain locked until expiry")
	}
	return nil
}

func (d *delivery) Complete(ctx context.Context) error { return d.settle(ctx, true) }
func (d *delivery) Abandon(ctx context.Context) error  { return d.settle(ctx, false) }

var _ explorer.Delivery = (*delivery)(nil)

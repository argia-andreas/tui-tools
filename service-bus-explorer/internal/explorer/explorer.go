// Package explorer defines the broker-independent explorer contract.
package explorer

import (
	"context"
	"time"
)

// Resource identifies a queue, topic node or topic subscription within a profile.
type Resource struct {
	Profile, Namespace, Queue, Topic, Subscription string
}

func (r Resource) Name() string {
	if r.Queue != "" {
		return r.Queue
	}
	if r.Subscription != "" {
		return r.Topic + "/" + r.Subscription
	}
	return r.Topic
}

type Counts struct {
	Active, DeadLetter int64
	ObservedAt         time.Time
	State              string // available, stale, unavailable
	Detail             string
}

type Message struct {
	ID, Subject, ContentType                string
	Sequence                                int64
	Body                                    []byte
	Properties                              map[string]any
	EnqueuedAt                              time.Time
	DeliveryCount                           uint32
	DeadLetterReason, DeadLetterDescription string
	PreviewTruncated                        bool
}

type Page struct {
	Messages     []Message
	NextSequence *int64
}

type Store interface {
	Discover(context.Context) ([]Resource, error)
	Counts(context.Context, Resource) (Counts, error)
	Peek(ctx context.Context, target Resource, deadLetter bool, fromSequence int64, limit int) (Page, error)
	ReceiveNextDLQ(context.Context, Resource) (Delivery, error)
	Close(context.Context) error
}

type Delivery interface {
	Snapshot() Message
	LockedUntil() time.Time
	Resend(ctx context.Context, newMessageID string) (SendResult, error)
	Complete(context.Context) error
	Abandon(context.Context) error
}

type SendResult struct {
	State     string // confirmed, rejected, unknown
	MessageID string
}

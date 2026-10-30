// Package explorer defines the broker-independent explorer contract.
package explorer

import (
	"context"
	"time"
)

// Resource identifies a queue, topic node or topic subscription within a profile.
type Resource struct {
	Profile, Namespace, Queue, Topic, Subscription string
	// Management identity prevents a moved/recreated namespace from inheriting
	// a saved cloud entity selection solely because its address is unchanged.
	AzureSubscriptionID, ResourceGroup string
}

// Namespace is an explicit discovery target. AzureSubscriptionID is never a
// topic subscription. Address is the fully qualified messaging namespace;
// local providers use their fixed local address instead.
type Namespace struct {
	Profile, Address, Name                     string
	AzureSubscriptionID, AzureSubscriptionName string
	ResourceGroup                              string
}

type AzureSubscription struct{ ID, Name string }

// NamespaceDiscovery reports completeness separately for each selected target.
// Failed or incomplete listings must not be mistaken for an empty namespace.
type NamespaceDiscovery struct {
	Namespace Namespace
	Resources []Resource
	Err       error
}

// ScopeProvider lists namespace metadata only on explicit request. A resource
// group is a selection aid; DiscoverScope accepts only namespace snapshots.
type ScopeProvider interface {
	Subscriptions(context.Context) ([]AzureSubscription, error)
	Namespaces(context.Context, string) ([]Namespace, error)
	ResolveNamespace(context.Context, string) (Namespace, error)
	DiscoverScope(context.Context, []Namespace) []NamespaceDiscovery
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
	// PeekMessage returns an independent, full-content copy of the exact sequence
	// in Active or DLQ, without receiving, locking or granting replay authority.
	// A missing sequence is an error; a later message must never be substituted.
	PeekMessage(ctx context.Context, target Resource, deadLetter bool, sequence int64) (Message, error)
	ReceiveNextDLQ(context.Context, Resource) (Delivery, error)
	PrepareDLQCopy(ctx context.Context, source Resource, sequence int64) (Replay, error)
	ReceiveSelectedDLQ(ctx context.Context, source Resource, sequence int64) (Delivery, error)
	Close(context.Context) error
}

type Replay interface {
	Snapshot() Message
	Resend(ctx context.Context, destination Resource, newMessageID string) (SendResult, error)
}

type Delivery interface {
	Replay
	LockedUntil() time.Time
	Complete(context.Context) error
	Abandon(context.Context) error
}

type SendResult struct {
	State     string // confirmed, rejected, unknown
	MessageID string
}

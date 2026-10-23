// Package azure implements real, read-only Service Bus browsing. This slice
// exposes only the fixed local emulator; no cloud fallback or mutations exist.
package azure

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	sb "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
	"servicebusexplorer/internal/explorer"
)

type Store struct {
	client *sb.Client
	admin  *admin.Client
}

func NewEmulator() (*Store, error) {
	client, err := sb.NewClientFromConnectionString("Endpoint=sb://"+MessagingAddress+emulatorCredentials,
		&sb.ClientOptions{RetryOptions: sb.RetryOptions{MaxRetries: -1}})
	if err != nil {
		return nil, errors.New("cannot initialize emulator messaging client")
	}
	management, err := admin.NewClientFromConnectionString("Endpoint=sb://"+managementAddress+emulatorCredentials,
		&admin.ClientOptions{ClientOptions: azcore.ClientOptions{
			Transport: newEmulatorTransport(), Retry: policy.RetryOptions{MaxRetries: -1},
		}})
	if err != nil {
		return nil, errors.New("cannot initialize emulator administration client")
	}
	return &Store{client: client, admin: management}, nil
}

func (s *Store) Discover(ctx context.Context) ([]explorer.Resource, error) {
	var resources []explorer.Resource
	queues := s.admin.NewListQueuesPager(nil)
	for queues.More() {
		page, err := queues.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover emulator queues: %w", err)
		}
		for _, q := range page.Queues {
			resources = append(resources, resource(q.QueueName, "", ""))
		}
	}
	topics := s.admin.NewListTopicsPager(nil)
	for topics.More() {
		page, err := topics.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover emulator topics: %w", err)
		}
		for _, topic := range page.Topics {
			subscriptions := s.admin.NewListSubscriptionsPager(topic.TopicName, nil)
			for subscriptions.More() {
				page, err := subscriptions.NextPage(ctx)
				if err != nil {
					return nil, fmt.Errorf("discover emulator subscriptions: %w", err)
				}
				for _, sub := range page.Subscriptions {
					resources = append(resources, resource("", topic.TopicName, sub.SubscriptionName))
				}
			}
		}
	}
	return resources, nil
}

func resource(queue, topic, subscription string) explorer.Resource {
	return explorer.Resource{Profile: "emulator", Namespace: MessagingAddress, Queue: queue, Topic: topic, Subscription: subscription}
}

func validResource(r explorer.Resource) bool {
	return r.Profile == "emulator" && r.Namespace == MessagingAddress &&
		((r.Queue != "" && r.Topic == "" && r.Subscription == "") || (r.Queue == "" && r.Topic != "" && r.Subscription != ""))
}

func (*Store) Counts(ctx context.Context, r explorer.Resource) (explorer.Counts, error) {
	if err := ctx.Err(); err != nil {
		return explorer.Counts{}, err
	}
	if !validResource(r) {
		return explorer.Counts{}, errors.New("invalid emulator resource identity")
	}
	// Emulator 2.0.1 omits CountDetails. Some SDKs decode that as false zeros.
	// Never infer authoritative totals from a bounded peek or seed expectations.
	return explorer.Counts{State: "unavailable", Detail: "Emulator 2.0.1 does not supply reliable runtime counts"}, nil
}

func (s *Store) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	if !validResource(r) || from < 0 || limit < 1 || limit > 20 {
		return explorer.Page{}, errors.New("invalid emulator resource, sequence or page limit")
	}
	options := &sb.ReceiverOptions{ReceiveMode: sb.ReceiveModePeekLock}
	if dlq {
		options.SubQueue = sb.SubQueueDeadLetter
	}
	var receiver *sb.Receiver
	var err error
	if r.Queue != "" {
		receiver, err = s.client.NewReceiverForQueue(r.Queue, options)
	} else {
		receiver, err = s.client.NewReceiverForSubscription(r.Topic, r.Subscription, options)
	}
	if err != nil {
		return explorer.Page{}, fmt.Errorf("open peek receiver: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = receiver.Close(closeCtx)
	}()
	// Always pass the explicit cursor. An extra peek result detects a next page;
	// it is still browsing, never receiving or changing a message's delivery count.
	messages, err := receiver.PeekMessages(ctx, limit+1, &sb.PeekMessagesOptions{FromSequenceNumber: &from})
	if err != nil {
		return explorer.Page{}, fmt.Errorf("peek emulator messages: %w", err)
	}
	var page explorer.Page
	if len(messages) > limit {
		next := *messages[limit-1].SequenceNumber + 1
		page.NextSequence = &next
		messages = messages[:limit]
	}
	for _, message := range messages {
		body := message.Body
		truncated := len(body) > 64*1024
		if truncated {
			body = body[:64*1024]
		}
		page.Messages = append(page.Messages, explorer.Message{
			ID: message.MessageID, Subject: value(message.Subject), ContentType: value(message.ContentType),
			Sequence: value(message.SequenceNumber), Body: slices.Clone(body), Properties: maps.Clone(message.ApplicationProperties),
			EnqueuedAt: value(message.EnqueuedTime), DeliveryCount: message.DeliveryCount,
			DeadLetterReason: value(message.DeadLetterReason), DeadLetterDescription: value(message.DeadLetterErrorDescription),
			PreviewTruncated: truncated,
		})
	}
	return page, nil
}

func value[T any](p *T) (zero T) {
	if p != nil {
		return *p
	}
	return zero
}

func (*Store) ReceiveNextDLQ(context.Context, explorer.Resource) (explorer.Delivery, error) {
	return nil, errors.New("browsing is read-only; reviewed DLQ actions arrive in a later slice")
}

func (s *Store) Close(ctx context.Context) error { return s.client.Close(ctx) }

var _ explorer.Store = (*Store)(nil)

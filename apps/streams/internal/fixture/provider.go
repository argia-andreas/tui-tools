// Package fixture supplies labelled fictional events without a network client.
package fixture

import (
	"context"
	"fmt"
	"sync"
	"time"

	"streams/internal/stream"
)

type backend struct{}

func New() stream.Backend { return &backend{} }
func (*backend) Close()   {}
func (*backend) Topics(context.Context) ([]stream.Topic, error) {
	var topics []stream.Topic
	for _, name := range []string{"orders.created", "orders.failed", "orders.shipped"} {
		t := stream.Topic{Ref: stream.TopicRef{Name: name, ID: "fixture-" + name}}
		for i := int32(0); i < 3; i++ {
			t.Partitions = append(t.Partitions, stream.Partition{ID: i, Leader: 0, Replicas: []int32{0}, InSync: []int32{0}})
		}
		topics = append(topics, t)
	}
	return topics, nil
}
func (*backend) Groups(context.Context) ([]string, error) {
	return []string{"checkout-workers", "fulfilment", "new-consumer"}, nil
}
func (*backend) Group(_ context.Context, name string) (stream.GroupObservation, error) {
	g := stream.GroupObservation{Name: name, State: "Stable (fixture)", Started: time.Now(), Finished: time.Now()}
	for i := int32(0); i < 3; i++ {
		p := stream.GroupPartition{Topic: "orders.created", Partition: i, Start: stream.OffsetValue{Value: 10, Known: true}, Committed: stream.OffsetValue{Value: 95 + int64(i*9), Known: name != "new-consumer"}, HighWatermark: stream.OffsetValue{Value: 120 + int64(i*12), Known: true}}
		g.Partitions = append(g.Partitions, p)
	}
	return g, nil
}
func (*backend) Open(_ context.Context, q stream.ReadRequest) (stream.Reader, error) {
	if len(q.Partitions) == 0 || len(q.Partitions) > 32 {
		return nil, stream.ErrLimit
	}
	r := &reader{q: q, c: stream.Cursor{Topic: q.Topic}}
	for _, p := range q.Partitions {
		next := int64(100 + p*100)
		if q.Mode == "offset" {
			next = q.Offset
		}
		if q.Mode == "time" && q.Time.After(time.Date(2026, 10, 30, 12, 4, 20, 0, time.UTC)) {
			next += 6
		}
		r.c.Positions = append(r.c.Positions, stream.Position{Partition: p, Next: next, End: int64(106 + p*100)})
	}
	if q.Cursor != nil {
		r.c = stream.CloneCursor(*q.Cursor)
	}
	if q.Mode == "follow" {
		for i := range r.c.Positions {
			r.c.Positions[i].Next = r.c.Positions[i].End
		}
	}
	return r, nil
}

type reader struct {
	mu     sync.Mutex
	q      stream.ReadRequest
	c      stream.Cursor
	closed bool
}

func (r *reader) Cursor() stream.Cursor {
	r.mu.Lock()
	defer r.mu.Unlock()
	return stream.CloneCursor(r.c)
}
func (r *reader) Close() { r.mu.Lock(); r.closed = true; r.mu.Unlock() }
func (r *reader) Next(ctx context.Context) stream.Batch {
	select {
	case <-ctx.Done():
		return stream.Batch{Stop: "error", Issues: []stream.PartitionIssue{{Partition: -1, Err: ctx.Err()}}}
	default:
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b := stream.Batch{Stop: "end"}
	if r.closed {
		b.Stop = "error"
		b.Issues = append(b.Issues, stream.PartitionIssue{Partition: -1, Err: stream.ErrClosed})
		return b
	}
	for i := range r.c.Positions {
		p := &r.c.Positions[i]
		end := p.End
		if r.q.Follow || r.q.Mode == "follow" {
			end = p.Next + 1
			b.Stop = "continue"
		}
		for p.Next < end {
			n := p.Next
			v := []byte(fmt.Sprintf(`{"orderId":"ord-%d","total":249.50,"currency":"SEK","status":"created"}`, n))
			if n%6 == 1 {
				v = nil
			}
			if n%6 == 2 {
				v = []byte{}
			}
			if n%6 == 3 {
				v = []byte{0, 255, 27, ']', '5', '2', 7}
			}
			b.Records = append(b.Records, stream.Record{ID: stream.RecordID{Topic: r.c.Topic, Partition: p.Partition, Offset: n}, Time: time.Date(2026, 10, 30, 12, 4, 10, int(n%100)*1000000, time.UTC), TimestampType: "CreateTime", Key: []byte(fmt.Sprintf("ord-%d", n)), Value: v, Headers: []stream.Header{{Key: "content-type", Value: []byte("application/json")}, {Key: "source", Value: []byte("fixture")}, {Key: "source", Value: []byte("seed")}}})
			p.Next++
		}
	}
	b.Next = stream.CloneCursor(r.c)
	return b
}

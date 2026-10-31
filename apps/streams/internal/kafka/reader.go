package kafka

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"streams/internal/stream"
)

var errIncomplete = errors.New("scan incomplete; invisible gaps possible")

type recordClient interface {
	PollRecords(context.Context, int) kgo.Fetches
	PauseFetchPartitions(map[string][]int32) map[string][]int32
	Close()
}

type reader struct {
	mu            sync.Mutex // serializes Next; Close cancels before waiting for it
	once          sync.Once
	ctx           context.Context
	cancel        context.CancelFunc
	client        recordClient
	opening       stream.Cursor // never modified
	current       stream.Cursor
	follow        bool
	firstDeadline time.Time
	pending       []*kgo.Record // polled, but not yet accepted: never discard on a budget stop
	failed        map[int32]error
}

func (c *Client) Open(ctx context.Context, req stream.ReadRequest) (stream.Reader, error) {
	ctx, done := c.operation(ctx)
	defer done()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.ctx.Err() != nil {
		return nil, stream.ErrClosed
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	if c.active != nil {
		c.active.Close()
		c.active = nil
	}
	ts, err := c.admin.ListTopicsWithInternal(ctx, req.Topic.Name)
	if err != nil {
		return nil, safe(err)
	}
	t, ok := ts[req.Topic.Name]
	if !ok {
		return nil, stream.ErrTopic
	}
	if t.Err != nil {
		return nil, safe(t.Err)
	}
	ref := topicRef(t)
	if req.Topic.ID != "" && req.Topic.ID != ref.ID {
		return nil, stream.ErrTopic
	}
	parts, err := selected(req, t)
	if err != nil {
		return nil, err
	}
	starts, err := c.offsets(ctx, req.Topic.Name, parts, -2)
	if err != nil {
		return nil, safe(err)
	}
	ends, err := c.offsets(ctx, req.Topic.Name, parts, -1)
	if err != nil {
		return nil, safe(err)
	}
	var times kadm.ListedOffsets
	if req.Mode == "time" {
		times, err = c.offsets(ctx, req.Topic.Name, parts, req.Time.UnixMilli())
		if err != nil {
			return nil, safe(err)
		}
	}
	cur, err := resolve(req, t, starts, ends, times)
	if err != nil {
		return nil, err
	}
	assigned := map[int32]kgo.Offset{}
	for _, p := range cur.Positions {
		assigned[p.Partition] = kgo.NoResetOffset().At(p.Next)
	}
	opts := append([]kgo.Opt(nil), c.opts...)
	opts = append(opts, kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{ref.Name: assigned}))
	k, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, safe(err)
	}
	if ctx.Err() != nil {
		k.Close()
		return nil, ctx.Err()
	}
	rctx, cancel := context.WithCancel(c.ctx)
	deadline, _ := ctx.Deadline()
	r := &reader{ctx: rctx, cancel: cancel, client: k, opening: stream.CloneCursor(cur), current: stream.CloneCursor(cur), follow: req.Mode == "follow" || req.Follow, firstDeadline: deadline, failed: map[int32]error{}}
	c.active = r
	return r, nil
}

// Unlike the topic-wide kadm helpers, request only the selected partitions.
// Committed timestamp requests leave a no-match at -1, without HWM fallback.
func (c *Client) offsets(ctx context.Context, topic string, parts []int32, timestamp int64) (kadm.ListedOffsets, error) {
	req := kmsg.NewPtrListOffsetsRequest()
	req.IsolationLevel = 1
	req.TimeoutMillis = 5000
	t := kmsg.NewListOffsetsRequestTopic()
	t.Topic = topic
	for _, id := range parts {
		p := kmsg.NewListOffsetsRequestTopicPartition()
		p.Partition = id
		p.Timestamp = timestamp
		t.Partitions = append(t.Partitions, p)
	}
	req.Topics = append(req.Topics, t)
	out := kadm.ListedOffsets{topic: {}}
	for _, shard := range c.metadata.RequestSharded(ctx, req) {
		if shard.Err != nil {
			return nil, shard.Err
		}
		resp := shard.Resp.(*kmsg.ListOffsetsResponse)
		for _, t := range resp.Topics {
			for _, p := range t.Partitions {
				out[topic][p.Partition] = kadm.ListedOffset{Topic: topic, Partition: p.Partition, Offset: p.Offset, LeaderEpoch: p.LeaderEpoch, Err: kerr.ErrorForCode(p.ErrorCode)}
			}
		}
	}
	return out, nil
}

func selected(req stream.ReadRequest, t kadm.TopicDetail) ([]int32, error) {
	parts := append([]int32(nil), req.Partitions...)
	if len(parts) == 0 {
		if req.Mode == "resume" {
			for _, p := range req.Cursor.Positions {
				parts = append(parts, p.Partition)
			}
		} else {
			parts = t.Partitions.Numbers()
		}
	}
	if len(parts) == 0 {
		return nil, stream.ErrTopic
	}
	if len(parts) > 32 {
		return nil, stream.ErrLimit
	}
	seen := map[int32]bool{}
	for _, id := range parts {
		if _, ok := t.Partitions[id]; !ok || seen[id] {
			return nil, stream.ErrTopic
		}
		seen[id] = true
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i] < parts[j] })
	return parts, nil
}

func validateRequest(r stream.ReadRequest) error {
	if r.Topic.Name == "" {
		return stream.ErrTopic
	}
	if len(r.Partitions) > 32 {
		return stream.ErrLimit
	}
	seen := map[int32]bool{}
	for _, p := range r.Partitions {
		if p < 0 || seen[p] {
			return errors.New("invalid or duplicate selected partition")
		}
		seen[p] = true
	}
	switch r.Mode {
	case "recent", "earliest", "follow":
	case "offset":
		if len(r.Partitions) != 1 || r.Offset < 0 {
			return errors.New("offset seek requires one partition and nonnegative offset")
		}
	case "time":
		if r.Time.IsZero() || r.Time.UnixMilli() < 0 {
			return errors.New("timestamp seek requires a timestamp")
		}
	case "resume":
		if r.Cursor == nil || r.Cursor.Topic.Name != r.Topic.Name || r.Cursor.Topic.ID != r.Topic.ID || len(r.Cursor.Positions) == 0 {
			return stream.ErrTopic
		}
		if len(r.Cursor.Positions) > 32 {
			return stream.ErrLimit
		}
	default:
		return errors.New("unsupported read mode")
	}
	if r.Mode != "resume" && r.Cursor != nil || r.Follow && r.Mode != "resume" && r.Mode != "follow" {
		return errors.New("saved cursor requires resume; follow flag requires follow or resume")
	}
	return nil
}

func resolve(req stream.ReadRequest, t kadm.TopicDetail, starts, ends, times kadm.ListedOffsets) (stream.Cursor, error) {
	ref := topicRef(t)
	cur := stream.Cursor{Topic: ref}
	if req.Mode == "resume" && req.Cursor.Topic.ID != ref.ID {
		return cur, stream.ErrTopic
	}
	saved := map[int32]stream.Position{}
	if req.Cursor != nil {
		for _, p := range req.Cursor.Positions {
			if _, ok := saved[p.Partition]; ok || p.Partition < 0 || p.Next < 0 || p.End < 0 {
				return cur, stream.ErrExpired
			}
			saved[p.Partition] = p
		}
	}
	parts, err := selected(req, t)
	if err != nil {
		return cur, err
	}
	if req.Mode == "resume" && len(parts) != len(saved) {
		return cur, stream.ErrTopic
	}
	for _, id := range parts {
		p, ok := t.Partitions[id]
		if !ok {
			return cur, stream.ErrTopic
		}
		if p.Err != nil {
			return cur, safe(p.Err)
		}
		start, e := observedOffset(starts, ref.Name, id, nil)
		if e != nil {
			return cur, e
		}
		end, e := observedOffset(ends, ref.Name, id, nil)
		if e != nil {
			return cur, e
		}
		if end.Value < start.Value {
			return cur, stream.ErrExpired
		}
		x := stream.Position{Partition: id, Next: start.Value, End: end.Value}
		switch req.Mode {
		case "recent":
			x.Next = max(start.Value, end.Value-500)
		case "offset":
			x.Next = req.Offset
		case "time":
			v, ok := times.Lookup(ref.Name, id)
			if !ok {
				return cur, stream.ErrTopic
			}
			if v.Err != nil {
				return cur, safe(v.Err)
			}
			if v.Offset < 0 {
				x.Next = end.Value
			} else {
				x.Next = min(v.Offset, end.Value)
			}
		case "follow":
			x.Next = end.Value
		case "resume":
			v, ok := saved[id]
			if !ok {
				return cur, stream.ErrTopic
			}
			x.Next = v.Next
			if !req.Follow {
				if v.End > end.Value || v.Next > v.End {
					return cur, stream.ErrExpired
				}
				x.End = v.End
			}
		}
		if x.Next < start.Value || x.Next > end.Value {
			return cur, stream.ErrExpired
		}
		cur.Positions = append(cur.Positions, x)
	}
	return cur, nil
}

func (r *reader) Cursor() stream.Cursor { return stream.CloneCursor(r.opening) }

func (r *reader) Close() {
	r.once.Do(func() { r.cancel(); r.client.Close(); r.mu.Lock(); r.pending = nil; r.mu.Unlock() })
}

func (r *reader) Next(ctx context.Context) (b stream.Batch) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b.Stop = "continue"
	defer func() {
		b.Next = stream.CloneCursor(r.current)
		for _, p := range r.current.Positions {
			if e := r.failed[p.Partition]; e != nil {
				b.Issues = append(b.Issues, stream.PartitionIssue{Partition: p.Partition, Err: e})
			}
		}
	}()
	if r.ctx.Err() != nil {
		b.Stop = "error"
		b.Issues = append(b.Issues, stream.PartitionIssue{Partition: -1, Err: stream.ErrClosed})
		return
	}
	count, byteLimit, duration := 500, 4<<20, 5*time.Second
	if r.follow {
		count, byteLimit, duration = 100, 1<<20, 500*time.Millisecond
	}
	deadline := time.Now().Add(duration)
	if !r.follow && !r.firstDeadline.IsZero() {
		deadline = r.firstDeadline
		r.firstDeadline = time.Time{}
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	used := 0
	for {
		if r.complete() {
			if len(r.failed) > 0 {
				b.Stop = "error"
			} else {
				b.Stop = "end"
			}
			return
		}
		if ctx.Err() != nil {
			b.Stop = "deadline"
			if r.ctx.Err() != nil {
				b.Stop = "error"
				b.Issues = append(b.Issues, stream.PartitionIssue{Partition: -1, Err: stream.ErrClosed})
			} else if ctx.Err() == context.Canceled {
				b.Stop = "error"
				b.Issues = append(b.Issues, stream.PartitionIssue{Partition: -1, Err: context.Canceled})
			} else if !r.follow {
				for _, p := range r.current.Positions {
					if p.Next < p.End && r.failed[p.Partition] == nil {
						b.Issues = append(b.Issues, stream.PartitionIssue{Partition: p.Partition, Err: errors.Join(context.DeadlineExceeded, errIncomplete)})
					}
				}
			}
			return
		}
		if len(r.pending) == 0 {
			fs := r.client.PollRecords(ctx, count-len(b.Records))
			fs.EachTopic(func(t kgo.FetchTopic) {
				if t.Topic == r.current.Topic.Name && r.current.Topic.ID != "" && t.TopicID != ([16]byte{}) && kadm.TopicID(t.TopicID).String() != r.current.Topic.ID {
					for _, p := range r.current.Positions {
						r.failed[p.Partition] = stream.ErrTopic
						r.pause(p.Partition)
					}
				}
			})
			fs.EachError(func(_ string, p int32, e error) {
				if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
					return
				}
				if p < 0 {
					b.Issues = append(b.Issues, stream.PartitionIssue{Partition: p, Err: safe(e)})
					b.Stop = "error"
					return
				}
				r.failed[p] = safe(e)
				r.pause(p)
			})
			r.pending = fs.Records()
			if b.Stop == "error" {
				return
			}
			if len(r.pending) == 0 {
				continue
			}
		}
		x := r.pending[0]
		p := r.position(x.Partition)
		if p == nil || x.Topic != r.current.Topic.Name || r.failed[x.Partition] != nil || x.Offset < p.Next {
			r.pop()
			continue
		}
		if !r.follow && x.Offset >= p.End {
			p.Next = p.End
			r.pause(p.Partition)
			r.pop()
			continue
		}
		n := rawBytes(x)
		if n > byteLimit {
			b.Stop = "limit"
			b.Issues = append(b.Issues, stream.PartitionIssue{Partition: x.Partition, Err: stream.ErrLimit})
			return
		}
		if used+n > byteLimit {
			b.Stop = "bytes"
			return
		}
		b.Records = append(b.Records, owned(x, r.current.Topic))
		used += n
		p.Next = x.Offset + 1
		r.pop()
		if !r.follow && p.Next >= p.End {
			r.pause(p.Partition)
		}
		if len(b.Records) == count {
			if r.complete() && len(r.failed) == 0 {
				b.Stop = "end"
			} else {
				b.Stop = "records"
			}
			return
		}
	}
}

func (r *reader) pop() { r.pending[0] = nil; r.pending = r.pending[1:] }
func (r *reader) position(id int32) *stream.Position {
	for i := range r.current.Positions {
		if r.current.Positions[i].Partition == id {
			return &r.current.Positions[i]
		}
	}
	return nil
}
func (r *reader) pause(p int32) {
	r.client.PauseFetchPartitions(map[string][]int32{r.current.Topic.Name: {p}})
}
func (r *reader) complete() bool {
	for _, p := range r.current.Positions {
		if r.failed[p.Partition] == nil && (r.follow || p.Next < p.End) {
			return false
		}
	}
	return true
}
func rawBytes(r *kgo.Record) int {
	n := len(r.Topic) + len(r.Key) + len(r.Value)
	for _, h := range r.Headers {
		n += len(h.Key) + len(h.Value)
	}
	return n
}
func clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte{}, b...)
}
func owned(r *kgo.Record, t stream.TopicRef) stream.Record {
	x := stream.Record{ID: stream.RecordID{Topic: t, Partition: r.Partition, Offset: r.Offset}, Time: r.Timestamp, Key: clone(r.Key), Value: clone(r.Value), TimestampType: "unknown"}
	switch r.Attrs.TimestampType() {
	case 0:
		x.TimestampType = "create-time"
	case 1:
		x.TimestampType = "log-append-time"
	}
	for _, h := range r.Headers {
		x.Headers = append(x.Headers, stream.Header{Key: h.Key, Value: clone(h.Value)})
	}
	return x
}

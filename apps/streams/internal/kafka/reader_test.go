package kafka

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"streams/internal/config"
	"streams/internal/stream"
)

type scriptedRecords struct {
	fetches []kgo.Fetches
	closed  atomic.Int32
}

func (s *scriptedRecords) PollRecords(ctx context.Context, _ int) kgo.Fetches {
	if len(s.fetches) > 0 {
		f := s.fetches[0]
		s.fetches = s.fetches[1:]
		return f
	}
	<-ctx.Done()
	return nil
}
func (s *scriptedRecords) PauseFetchPartitions(map[string][]int32) map[string][]int32 { return nil }
func (s *scriptedRecords) Close()                                                     { s.closed.Add(1) }
func sample(p int32, o int64, b []byte) *kgo.Record {
	return &kgo.Record{Topic: "t", Partition: p, Offset: o, Value: b}
}
func fetch(rs ...*kgo.Record) kgo.Fetches {
	var ps []kgo.FetchPartition
	for _, r := range rs {
		i := 0
		for i < len(ps) && ps[i].Partition != r.Partition {
			i++
		}
		if i == len(ps) {
			ps = append(ps, kgo.FetchPartition{Partition: r.Partition})
		}
		ps[i].Records = append(ps[i].Records, r)
	}
	return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "t", Partitions: ps}}}}
}
func testReader(t *testing.T, follow bool, positions []stream.Position, fs ...kgo.Fetches) (*reader, *scriptedRecords) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s := &scriptedRecords{fetches: fs}
	cur := stream.Cursor{Topic: stream.TopicRef{Name: "t"}, Positions: positions}
	r := &reader{ctx: ctx, cancel: cancel, client: s, opening: stream.CloneCursor(cur), current: stream.CloneCursor(cur), follow: follow, failed: map[int32]error{}}
	t.Cleanup(r.Close)
	return r, s
}

func TestDeliveryPositionsAndOwnedRecords(t *testing.T) {
	x := sample(0, 7, []byte("first"))
	x.Key = []byte{}
	x.Headers = []kgo.RecordHeader{{Key: "dup", Value: nil}, {Key: "dup", Value: []byte{}}}
	r, _ := testReader(t, false, []stream.Position{{Partition: 0, Next: 7, End: 10}, {Partition: 2, Next: 40, End: 45}}, fetch(x, sample(0, 9, nil), sample(2, 40, []byte{}), sample(2, 44, []byte("last"))))
	opening := r.Cursor()
	opening.Positions[0].Next = 999
	b := r.Next(context.Background())
	if b.Stop != "end" || len(b.Records) != 4 || b.Next.Positions[0].Next != 10 || b.Next.Positions[1].Next != 45 {
		t.Fatal(b)
	}
	if r.Cursor().Positions[0].Next != 7 {
		t.Fatal("opening cursor changed")
	}
	if b.Records[0].Key == nil || b.Records[1].Value != nil || b.Records[2].Value == nil || b.Records[0].Headers[0].Value != nil || b.Records[0].Headers[1].Value == nil {
		t.Fatal("nullness or duplicate headers lost")
	}
	x.Value[0] = 'X'
	if string(b.Records[0].Value) != "first" {
		t.Fatal("broker buffer escaped")
	}
	b.Next.Positions[0].Next = 0
	if r.Next(context.Background()).Next.Positions[0].Next != 10 {
		t.Fatal("delivered cursor aliases reader")
	}
}

func TestByteBudgetKeepsRejectedRecordsUnread(t *testing.T) {
	r, _ := testReader(t, false, []stream.Position{{Partition: 0, Next: 7, End: 10}}, fetch(sample(0, 7, make([]byte, 2<<20)), sample(0, 9, make([]byte, 3<<20))))
	b := r.Next(context.Background())
	if b.Stop != "bytes" || len(b.Records) != 1 || b.Next.Positions[0].Next != 8 {
		t.Fatal(b.Stop, b.Next)
	}
	saved := stream.CloneCursor(b.Next)
	b = r.Next(context.Background())
	if b.Stop != "end" || len(b.Records) != 1 || b.Records[0].ID.Offset != 9 {
		t.Fatal("pending record lost", b.Stop)
	}
	// A new reader at the application's saved delivery cursor must see it too.
	r2, _ := testReader(t, false, saved.Positions, fetch(sample(0, 9, []byte("unread"))))
	if b = r2.Next(context.Background()); len(b.Records) != 1 || b.Records[0].ID.Offset != 9 {
		t.Fatal("reopen skipped unread record")
	}
}

func TestRecordAndOversizeLimits(t *testing.T) {
	for _, follow := range []bool{false, true} {
		t.Run(map[bool]string{false: "history", true: "follow"}[follow], func(t *testing.T) {
			n := 500
			if follow {
				n = 100
			}
			rs := make([]*kgo.Record, n)
			for i := range rs {
				rs[i] = sample(0, int64(i), nil)
			}
			r, _ := testReader(t, follow, []stream.Position{{Partition: 0, Next: 0, End: int64(n + 1)}}, fetch(rs...), fetch(sample(0, int64(n), nil)))
			b := r.Next(context.Background())
			if len(b.Records) != n || b.Stop != "records" || b.Next.Positions[0].Next != int64(n) {
				t.Fatal(b.Stop, b.Next, len(b.Records))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			b = r.Next(ctx)
			if len(b.Records) != 1 || b.Records[0].ID.Offset != int64(n) {
				t.Fatal("count cap dropped next record", b.Stop)
			}
		})
	}
	r, _ := testReader(t, false, []stream.Position{{Partition: 0, Next: 4, End: 5}}, fetch(sample(0, 4, make([]byte, 4<<20))))
	b := r.Next(context.Background())
	if b.Stop != "limit" || len(b.Records) != 0 || b.Next.Positions[0].Next != 4 || !errors.Is(b.Issues[0].Err, stream.ErrLimit) {
		t.Fatal(b)
	}
	if b = r.Next(context.Background()); b.Stop != "limit" || b.Next.Positions[0].Next != 4 {
		t.Fatal("oversize skipped or retried network")
	}
}

func TestStableEndsAndInvisibleGaps(t *testing.T) {
	r, _ := testReader(t, false, []stream.Position{{Partition: 0, Next: 7, End: 10}}, fetch(sample(0, 8, nil), sample(0, 12, []byte("beyond captured LSO"))))
	b := r.Next(context.Background())
	if b.Stop != "end" || len(b.Records) != 1 || b.Next.Positions[0].Next != 10 {
		t.Fatal(b)
	}
	r, _ = testReader(t, false, []stream.Position{{Partition: 0, Next: 7, End: 10}}, fetch(sample(0, 8, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	b = r.Next(ctx)
	if b.Stop != "deadline" || b.Next.Positions[0].Next != 9 || len(b.Issues) != 1 || !errors.Is(b.Issues[0].Err, context.DeadlineExceeded) {
		t.Fatal("invisible gap falsely complete", b)
	}
	r, _ = testReader(t, false, []stream.Position{{Partition: 0, Next: 10, End: 10}})
	if b = r.Next(context.Background()); b.Stop != "end" {
		t.Fatal("empty range not complete")
	}
}

func TestPartitionFailureAndConcurrentClose(t *testing.T) {
	f := fetch(sample(2, 44, []byte("healthy")))
	f[0].Topics[0].Partitions = append(f[0].Topics[0].Partitions, kgo.FetchPartition{Partition: 0, Err: kerr.OffsetOutOfRange})
	r, _ := testReader(t, false, []stream.Position{{Partition: 0, Next: 7, End: 10}, {Partition: 2, Next: 44, End: 45}}, f)
	b := r.Next(context.Background())
	if b.Stop != "error" || len(b.Records) != 1 || len(b.Issues) != 1 || !errors.Is(b.Issues[0].Err, stream.ErrExpired) || b.Next.Positions[0].Next != 7 {
		t.Fatal(b)
	}
	r, s := testReader(t, true, []stream.Position{{Partition: 0, Next: 0, End: 0}})
	ch := make(chan stream.Batch, 1)
	go func() { ch <- r.Next(context.Background()) }()
	r.Close()
	r.Close()
	select {
	case b = <-ch:
		if b.Stop != "error" {
			t.Fatal(b.Stop)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock Next")
	}
	if s.closed.Load() != 1 {
		t.Fatal("Close not idempotent")
	}
}

func TestResolveModesRetentionAndTopicIdentity(t *testing.T) {
	topic := kadm.TopicDetail{Topic: "t", Partitions: kadm.PartitionDetails{0: {Partition: 0}, 2: {Partition: 2}}}
	starts := kadm.ListedOffsets{"t": {0: {Offset: 7}, 2: {Offset: 40}}}
	ends := kadm.ListedOffsets{"t": {0: {Offset: 10}, 2: {Offset: 700}}}
	for _, test := range []struct {
		mode string
		want []int64
	}{{"earliest", []int64{7, 40}}, {"recent", []int64{7, 200}}, {"follow", []int64{10, 700}}} {
		cur, err := resolve(stream.ReadRequest{Topic: stream.TopicRef{Name: "t"}, Mode: test.mode}, topic, starts, ends, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := []int64{cur.Positions[0].Next, cur.Positions[1].Next}; !reflect.DeepEqual(got, test.want) {
			t.Fatal(test.mode, got)
		}
	}
	req := stream.ReadRequest{Topic: stream.TopicRef{Name: "t"}, Mode: "time"}
	cur, err := resolve(req, topic, starts, ends, kadm.ListedOffsets{"t": {0: {Offset: -1}, 2: {Offset: 999}}})
	if err != nil || cur.Positions[0].Next != 10 || cur.Positions[1].Next != 700 {
		t.Fatal("timestamp no-match or unstable range rewound", cur, err)
	}
	saved := stream.Cursor{Topic: stream.TopicRef{Name: "t"}, Positions: []stream.Position{{Partition: 0, Next: 8, End: 9}}}
	req = stream.ReadRequest{Topic: saved.Topic, Mode: "resume", Cursor: &saved}
	cur, err = resolve(req, topic, starts, ends, nil)
	if err != nil || cur.Positions[0].End != 9 {
		t.Fatal("resume changed historical end", cur, err)
	}
	saved.Positions[0].Next = 6
	if _, err = resolve(req, topic, starts, ends, nil); !errors.Is(err, stream.ErrExpired) {
		t.Fatal("expired resume accepted", err)
	}
	saved.Positions[0].Next = 8
	topic.ID[0] = 1
	if _, err = resolve(req, topic, starts, ends, nil); !errors.Is(err, stream.ErrTopic) {
		t.Fatal("recreated topic accepted", err)
	}
}

func TestOpenValidationDoesNotContactBroker(t *testing.T) {
	c, err := New(config.Profile{Name: "local", Brokers: []string{"127.0.0.1:1"}, LocalPlaintext: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, req := range []stream.ReadRequest{
		{Topic: stream.TopicRef{Name: "t"}, Mode: "offset", Offset: 1},
		{Topic: stream.TopicRef{Name: "t"}, Mode: "earliest", Partitions: make([]int32, 33)},
		{Topic: stream.TopicRef{Name: "t"}, Mode: "earliest", Partitions: []int32{2, 2}},
		{Topic: stream.TopicRef{Name: "t"}, Mode: "time"},
		{Topic: stream.TopicRef{Name: "t"}, Mode: "resume"},
	} {
		if _, err := c.Open(context.Background(), req); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	c.Close()
	if _, err := c.Open(context.Background(), stream.ReadRequest{Topic: stream.TopicRef{Name: "t"}, Mode: "earliest"}); !errors.Is(err, stream.ErrClosed) {
		t.Fatal(err)
	}
}

func TestExactByteCapsAndChangedFetchIdentity(t *testing.T) {
	for _, follow := range []bool{false, true} {
		limit := 4 << 20
		if follow {
			limit = 1 << 20
		}
		r, _ := testReader(t, follow, []stream.Position{{Partition: 0, Next: 0, End: 1}}, fetch(sample(0, 0, make([]byte, limit-1))))
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		b := r.Next(ctx)
		cancel()
		if len(b.Records) != 1 || b.Records[0].Bytes() != limit || b.Next.Positions[0].Next != 1 {
			t.Fatal("exact byte cap refused", follow, b.Stop)
		}
	}
	oldID, newID := kadm.TopicID{1}, kadm.TopicID{2}
	f := fetch(sample(0, 7, []byte("wrong topic generation")))
	f[0].Topics[0].TopicID = [16]byte(newID)
	r, _ := testReader(t, false, []stream.Position{{Partition: 0, Next: 7, End: 8}}, f)
	r.opening.Topic.ID = oldID.String()
	r.current.Topic.ID = oldID.String()
	b := r.Next(context.Background())
	if b.Stop != "error" || len(b.Records) != 0 || b.Next.Positions[0].Next != 7 || len(b.Issues) != 1 || !errors.Is(b.Issues[0].Err, stream.ErrTopic) {
		t.Fatal("changed fetch topic identity accepted", b.Stop, b.Next, b.Issues)
	}
}

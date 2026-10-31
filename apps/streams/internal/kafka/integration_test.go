package kafka_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"streams/internal/config"
	"streams/internal/kafka"
	"streams/internal/stream"
)

// Opt-in, read-only smoke contract against an explicitly selected disposable
// quiescent lab. Seeding, ACLs, transactional scenarios and lifecycle are separate.
// Set STREAMS_KAFKA_TEST_CONFIG, STREAMS_KAFKA_TEST_CONNECTION and
// STREAMS_KAFKA_TEST_TOPIC. Optional STREAMS_KAFKA_TEST_GROUP observes a group.
func TestRealBrokerReadOnlyContract(t *testing.T) {
	path := os.Getenv("STREAMS_KAFKA_TEST_CONFIG")
	if path == "" {
		t.Skip("real broker contract requires explicit lab configuration")
	}
	name, topic := os.Getenv("STREAMS_KAFKA_TEST_CONNECTION"), os.Getenv("STREAMS_KAFKA_TEST_TOPIC")
	if name == "" || topic == "" {
		t.Fatal("set explicit test connection and seeded topic")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := cfg.Connection(name)
	if err != nil {
		t.Fatal(err)
	}
	c, err := kafka.New(p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ts, err := c.Topics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var selected *stream.Topic
	for i := range ts {
		if ts[i].Ref.Name == topic {
			selected = &ts[i]
			break
		}
	}
	if selected == nil {
		t.Fatal("seeded test topic is not discoverable")
	}
	// Replay one partition so a shared batch cap cannot select a different
	// subset due to nondeterministic cross-partition broker response order.
	parts := make([]int32, 0, 1)
	for _, p := range selected.Partitions {
		if p.Err == nil && len(parts) < 1 {
			parts = append(parts, p.ID)
		}
	}
	if len(parts) == 0 {
		t.Fatal("no readable test partitions")
	}
	r, err := c.Open(ctx, stream.ReadRequest{Topic: selected.Ref, Partitions: parts, Mode: "recent"})
	if err != nil {
		t.Fatal(err)
	}
	start := r.Cursor()
	b := r.Next(ctx)
	r.Close()
	if b.Stop == "error" || b.Stop == "limit" {
		t.Fatal("real read failed", b.Stop, b.Issues)
	}
	for _, issue := range b.Issues {
		if b.Stop != "deadline" || !errors.Is(issue.Err, context.DeadlineExceeded) {
			t.Fatal("unexpected real read issue", issue.Err)
		}
	}
	if b.Stop == "deadline" {
		t.Log("historical scan explicitly incomplete; invisible gaps may remain")
	}
	if len(b.Records) == 0 {
		t.Fatal("seed test topic with visible committed records")
	}
	if len(b.Records) > 500 {
		t.Fatal("record cap exceeded")
	}
	total := 0
	for _, v := range b.Records {
		total += v.Bytes()
		for _, p := range b.Next.Positions {
			if p.Partition == v.ID.Partition && (v.ID.Offset >= p.End || v.ID.Offset >= p.Next) {
				t.Fatal("record outside delivered history")
			}
		}
	}
	if total > 4<<20 {
		t.Fatal("byte cap exceeded")
	}
	// Reopening the immutable opening snapshot reproduces the delivered records;
	// it must not be secretly replaced by the live broker fetch position.
	r, err = c.Open(ctx, stream.ReadRequest{Topic: start.Topic, Mode: "resume", Cursor: &start})
	if err != nil {
		t.Fatal(err)
	}
	again := r.Next(ctx)
	r.Close()
	if !reflect.DeepEqual(b.Records, again.Records) || !reflect.DeepEqual(b.Next, again.Next) {
		t.Fatal("saved opening cursor did not reproduce immutable records and positions")
	}
	r, err = c.Open(ctx, stream.ReadRequest{Topic: b.Next.Topic, Mode: "resume", Cursor: &b.Next})
	if err != nil {
		t.Fatal(err)
	}
	continued := r.Next(ctx)
	r.Close()
	for _, v := range continued.Records {
		for _, p := range b.Next.Positions {
			if p.Partition == v.ID.Partition && v.ID.Offset < p.Next {
				t.Fatal("resume repeated accepted record")
			}
		}
	}
	if group := os.Getenv("STREAMS_KAFKA_TEST_GROUP"); group != "" {
		obs, err := c.Group(ctx, group)
		if err != nil {
			t.Fatal(err)
		}
		if obs.Partial || obs.Started.IsZero() || obs.Finished.Before(obs.Started) {
			t.Fatal("group observation incomplete")
		}
	}
	// Missing topic reads must not create topics, even with permissive lab ACLs.
	missing := "streams-absent-test-" + time.Now().UTC().Format("20060102150405.000000000")
	_, err = c.Open(ctx, stream.ReadRequest{Topic: stream.TopicRef{Name: missing}, Mode: "recent"})
	if err == nil || (!errors.Is(err, stream.ErrTopic) && !errors.Is(err, stream.ErrPermission)) {
		t.Fatal("missing topic did not fail explicitly", err)
	}
	ts, err = c.Topics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range ts {
		if v.Ref.Name == missing {
			t.Fatal("missing topic was automatically created")
		}
	}
	t.Logf("read %d owned records (%d raw bytes); replay/resume and noncreation verified", len(b.Records), total)
}

// This stronger contract deliberately requires the opt-in lab's exact dataset,
// not an arbitrary user's topic. It uses only the viewer profile.
func TestSeededLabWorkflows(t *testing.T) {
	if os.Getenv("STREAMS_SEEDED_LAB") != "1" {
		t.Skip("requires explicit streams seeded lab")
	}
	cfg, err := config.Load(os.Getenv("STREAMS_KAFKA_TEST_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := cfg.Connection(os.Getenv("STREAMS_KAFKA_TEST_CONNECTION"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := kafka.New(p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	before, err := c.Group(ctx, "orders-reader")
	if err != nil || before.Partial {
		t.Fatal("group observation", err, before)
	}
	read := func(q stream.ReadRequest) stream.Batch {
		t.Helper()
		r, err := c.Open(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		b := r.Next(ctx)
		if b.Stop == "error" || b.Stop == "limit" {
			t.Fatal(b.Stop, b.Issues)
		}
		return b
	}
	for _, name := range []string{"orders.failed", "orders.created"} {
		b := read(stream.ReadRequest{Topic: stream.TopicRef{Name: name}, Mode: "earliest", Partitions: []int32{0, 1, 2}})
		counts := [3]int{}
		for _, r := range b.Records {
			counts[r.ID.Partition]++
			if bytes.Contains(r.Value, []byte("txn-aborted")) || bytes.Contains(r.Value, []byte("txn-open")) {
				t.Fatal("uncommitted transaction exposed")
			}
			if r.ID.Offset == 1 && (r.Key != nil || r.Value != nil) {
				t.Fatal("null bytes changed")
			}
			if r.ID.Offset == 2 && (r.Key == nil || r.Value == nil || len(r.Key)+len(r.Value) != 0) {
				t.Fatal("empty bytes changed")
			}
			if r.ID.Offset == 3 && r.ID.Partition == 2 {
				if !bytes.Equal(r.Key, []byte{0, 255, 1}) || !bytes.Equal(r.Value, []byte{0, 255, 128, 10, 27}) {
					t.Fatal("binary content changed")
				}
				want := []stream.Header{{Key: "trace", Value: []byte("first")}, {Key: "trace", Value: []byte("second")}, {Key: "nullable"}, {Key: "empty", Value: []byte{}}}
				if !reflect.DeepEqual(r.Headers, want) {
					t.Fatal("ordered/null/duplicate headers changed", r.Headers)
				}
			}
		}
		want := [3]int{3, 5, 7}
		if name == "orders.created" {
			want[0]++ // independently seeded committed transaction
		}
		if counts != want {
			t.Fatalf("%s committed counts %v, want %v", name, counts, want)
		}
	}
	q := stream.ReadRequest{Topic: stream.TopicRef{Name: "orders.failed"}, Partitions: []int32{2}, Mode: "offset", Offset: 3}
	b := read(q)
	if len(b.Records) != 4 || b.Records[0].ID.Offset != 3 {
		t.Fatal("exact partition-offset seek", b)
	}
	q.Mode, q.Time = "time", time.Date(2026, 10, 1, 0, 3, 22, 0, time.UTC)
	b = read(q) // p2 timestamp +202 seconds => offset 2, not offset 3
	if len(b.Records) != 5 || b.Records[0].ID.Offset != 2 {
		t.Fatal("partition-local timestamp seek", b)
	}
	q.Mode, q.Follow = "follow", true
	r, err := c.Open(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	start := r.Cursor()
	b = r.Next(ctx)
	if len(b.Records) != 0 || !reflect.DeepEqual(start, b.Next) {
		t.Fatal("follow from now replayed history or invented progress", b)
	}
	finished := make(chan struct{})
	go func() { r.Next(ctx); close(finished) }()
	r.Close()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not stop in-flight follow")
	}
	after, err := c.Group(ctx, "orders-reader")
	if err != nil || after.Partial || !reflect.DeepEqual(before.Partitions, after.Partitions) {
		t.Fatal("viewer changed independently observed group offsets", err)
	}
	t.Log("31 committed records, exact binary/null/headers, offset/time seeks, follow/close and unchanged group positions verified")
}

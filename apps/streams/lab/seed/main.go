// Command seed is a deliberately independent, mutating lab tool. The app must
// never import this package. All endpoints are constants, never profile input.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

var topics = []string{"orders.created", "orders.failed"}
var broker, state string

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}
func read(name string) []byte { b, e := os.ReadFile(filepath.Join(state, name)); must(e); return b }

func client(user string, extra ...kgo.Opt) *kgo.Client {
	endpoint := "127.0.0.1:29092"
	if broker == "redpanda" {
		endpoint = "127.0.0.1:19092"
	}
	opts := []kgo.Opt{kgo.SeedBrokers(endpoint), kgo.RecordPartitioner(kgo.ManualPartitioner()), kgo.RequestTimeoutOverhead(5 * time.Second)}
	if broker == "redpanda" {
		roots := x509.NewCertPool()
		require(roots.AppendCertsFromPEM(read("ca.crt")), "invalid lab CA")
		pass := "intentionally-invalid"
		if user == "invalid" {
			user = "viewer"
		} else {
			pass = string(read(user + ".password"))
		}
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}), kgo.SASL(scram.Auth{User: "lab-" + user, Pass: pass}.AsSha256Mechanism()))
	}
	c, e := kgo.NewClient(append(opts, extra...)...)
	must(e)
	return c
}

func fixture(topic string, p int32, i int) *kgo.Record {
	r := &kgo.Record{Topic: topic, Partition: p, Timestamp: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(p*100+int32(i)) * time.Second),
		Key: []byte(fmt.Sprintf("order-%d-%d", p, i)), Value: []byte(fmt.Sprintf(`{"order":%d,"partition":%d,"status":"created"}`, i, p)),
		Headers: []kgo.RecordHeader{{Key: "trace", Value: []byte("first")}, {Key: "trace", Value: []byte("second")}, {Key: "nullable", Value: nil}, {Key: "empty", Value: []byte{}}}}
	switch i % 5 {
	case 1:
		r.Key = nil
		r.Value = nil
	case 2:
		r.Key = []byte{}
		r.Value = []byte{}
	case 3:
		r.Key = []byte{0, 255, 1}
		r.Value = []byte{0, 255, 128, 10, 27}
	case 4:
		r.Value = []byte("fictional\x1b]52;c;bm90LWNsaXBib2FyZA==\a\u202esafe")
	}
	return r
}

func setupViewer(ctx context.Context, a *kadm.Client) {
	body, _ := json.Marshal(map[string]string{"username": "lab-viewer", "password": string(read("viewer.password")), "algorithm": "SCRAM-SHA-256"})
	// Redpanda's admin listener is loopback-only; no remote admin API is accepted.
	h := &http.Client{Timeout: 5 * time.Second}
	resp, e := h.Post("http://127.0.0.1:19644/v1/security/users", "application/json", bytes.NewReader(body))
	must(e)
	resp.Body.Close()
	require(resp.StatusCode == 200 || resp.StatusCode == 201 || resp.StatusCode == 409, "cannot create viewer")
	for _, b := range []*kadm.ACLBuilder{
		kadm.NewACLs().Allow("User:lab-viewer").Topics(topics...).Operations(kadm.OpRead, kadm.OpDescribe),
		kadm.NewACLs().Allow("User:lab-viewer").Groups("orders-reader").Operations(kadm.OpDescribe),
	} {
		results, e := a.CreateACLs(ctx, b)
		must(e)
		for _, r := range results {
			must(r.Err)
		}
	}
}

func seed(ctx context.Context) {
	c := client("admin")
	defer c.Close()
	a := kadm.NewClient(c)
	// A partially seeded/unknown topic is not safe to overwrite or append to.
	metadata, e := a.ListTopics(ctx, topics...)
	must(e)
	for _, t := range metadata {
		require(errors.Is(t.Err, kerr.UnknownTopicOrPartition), "topics already exist: refusing duplicate or partial seed")
	}
	retention := "-1"
	results, e := a.CreateTopics(ctx, 3, 1, map[string]*string{"retention.ms": &retention}, append(topics, "private.denied")...)
	must(e)
	for _, r := range results {
		must(r.Err)
	}
	for _, t := range topics {
		for p := int32(0); p < 3; p++ {
			for i := 0; i < 3+int(p)*2; i++ {
				must(c.ProduceSync(ctx, fixture(t, p, i)).FirstErr())
			}
		}
	}
	for _, commit := range []bool{true, false} {
		tx := client("admin", kgo.TransactionalID(fmt.Sprintf("streams-%s-%t", broker, commit)))
		must(tx.BeginTransaction())
		p := int32(0)
		value := "txn-committed"
		if !commit {
			p = 1
			value = "txn-aborted"
		}
		must(tx.ProduceSync(ctx, &kgo.Record{Topic: topics[0], Partition: p, Key: []byte(value), Value: []byte(value)}).FirstErr())
		end := kgo.TryAbort
		if commit {
			end = kgo.TryCommit
		}
		must(tx.EndTransaction(ctx, end))
		tx.Close()
	}
	offsets := kadm.Offsets{}
	for _, t := range topics {
		for p, v := range []int64{1, 2, 4} {
			offsets.Add(kadm.Offset{Topic: t, Partition: int32(p), At: v})
		}
	}
	commits, e := a.CommitOffsets(ctx, "orders-reader", offsets)
	must(e)
	must(commits.Error())
	if broker == "redpanda" {
		setupViewer(ctx, a)
	}
	must(os.WriteFile(filepath.Join(state, broker+".seeded"), []byte("streams-seed-v1\n"), 0600))
	fmt.Println("seeded: 3 partitions/topic; 15 base records/topic; committed+aborted transactions; group offsets 1,2,4")
}

func open(ctx context.Context) {
	c := client("admin", kgo.TransactionalID("streams-"+broker+"-open"), kgo.TransactionTimeout(24*time.Hour))
	defer c.Close()
	must(c.BeginTransaction())
	must(c.ProduceSync(ctx, &kgo.Record{Topic: topics[0], Partition: 2, Key: []byte("txn-open"), Value: []byte("txn-open")}).FirstErr())
	fmt.Println("open transaction written; intentionally uncommitted until service stops")
	<-ctx.Done()
	// Leave the transaction unresolved. Restart fences it and opens a new one.
}

func group(ctx context.Context, a *kadm.Client) kadm.OffsetResponses {
	offsets, e := a.FetchOffsets(ctx, "orders-reader")
	must(e)
	must(offsets.Error())
	for _, t := range topics {
		for p, v := range []int64{1, 2, 4} {
			o, ok := offsets[t][int32(p)]
			require(ok && o.At == v, "group offsets differ from seeded 1,2,4")
		}
	}
	return offsets
}

func check(ctx context.Context) {
	user := "admin"
	if broker == "redpanda" {
		user = "viewer"
	}
	meta := client(user)
	defer meta.Close()
	a := kadm.NewClient(meta)
	before := group(ctx, a)
	ends, e := a.ListEndOffsets(ctx, topics...)
	must(e)
	must(ends.Error())
	stable, e := a.ListCommittedOffsets(ctx, topics...)
	must(e)
	must(stable.Error())
	require(stable[topics[0]][2].Offset < ends[topics[0]][2].Offset, "open transaction not holding last stable offset")
	assignments := map[string]map[int32]kgo.Offset{}
	for _, t := range topics {
		assignments[t] = map[int32]kgo.Offset{}
		for p := int32(0); p < 3; p++ {
			assignments[t][p] = kgo.NewOffset().AtStart()
		}
	}
	c := client(user, kgo.ConsumePartitions(assignments), kgo.FetchIsolationLevel(kgo.ReadCommitted()))
	defer c.Close()
	seen := map[string]bool{}
	count := 0
	for count < 31 && ctx.Err() == nil {
		fetches := c.PollRecords(ctx, 100)
		for _, e := range fetches.Errors() {
			must(e.Err)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			id := fmt.Sprintf("%s/%d/%d", r.Topic, r.Partition, r.Offset)
			require(!seen[id], "duplicate record")
			seen[id] = true
			require(!bytes.Equal(r.Value, []byte("txn-aborted")) && !bytes.Equal(r.Value, []byte("txn-open")), "transaction isolation leaked")
			baseCount := int64(3 + r.Partition*2)
			if r.Offset < baseCount {
				want := fixture(r.Topic, r.Partition, int(r.Offset))
				require(reflect.DeepEqual(r.Key, want.Key) && reflect.DeepEqual(r.Value, want.Value) && reflect.DeepEqual(r.Headers, want.Headers), "raw bytes/nullness/ordered duplicate headers mismatch")
				require(r.Timestamp.Equal(want.Timestamp), "timestamp mismatch")
			} else {
				require(r.Topic == topics[0] && r.Partition == 0 && bytes.Equal(r.Value, []byte("txn-committed")), "unexpected committed record")
			}
			count++
		})
	}
	require(count == 31, "committed read incomplete")
	// Independently prove abort/open records really exist on the broker rather
	// than merely accepting that the committed reader did not return them.
	raw := client(user, kgo.ConsumePartitions(assignments), kgo.FetchIsolationLevel(kgo.ReadUncommitted()))
	aborted, opened := false, false
	for (!aborted || !opened) && ctx.Err() == nil {
		f := raw.PollRecords(ctx, 100)
		for _, issue := range f.Errors() {
			must(issue.Err)
		}
		f.EachRecord(func(r *kgo.Record) {
			if bytes.Equal(r.Value, []byte("txn-aborted")) {
				aborted = true
			}
			if bytes.Equal(r.Value, []byte("txn-open")) && r.Offset >= stable[topics[0]][2].Offset {
				opened = true
			}
		})
	}
	raw.Close()
	require(aborted && opened, "uncommitted control reader did not find aborted/open records")
	after := group(ctx, a)
	require(reflect.DeepEqual(before, after), "reader changed group offsets")
	if broker == "redpanda" {
		writer := client("viewer", kgo.DisableIdempotentWrite())
		err := writer.ProduceSync(ctx, &kgo.Record{Topic: topics[0], Partition: 0, Value: []byte("must-be-denied")}).FirstErr()
		writer.Close()
		require(errors.Is(err, kerr.TopicAuthorizationFailed), "viewer topic write was not denied")
		denied, e := a.ListTopics(ctx, "private.denied")
		require(errors.Is(e, kerr.TopicAuthorizationFailed) || errors.Is(denied["private.denied"].Err, kerr.TopicAuthorizationFailed), "private topic describe was not denied")
		creates, e := a.CreateTopics(ctx, 1, 1, nil, "viewer-must-not-create")
		require(errors.Is(e, kerr.TopicAuthorizationFailed) || errors.Is(e, kerr.ClusterAuthorizationFailed) || errors.Is(creates["viewer-must-not-create"].Err, kerr.TopicAuthorizationFailed) || errors.Is(creates["viewer-must-not-create"].Err, kerr.ClusterAuthorizationFailed), "viewer create was not denied")
		same := kadm.Offsets{}
		same.Add(kadm.Offset{Topic: topics[0], Partition: 0, At: 1})
		commits, e := a.CommitOffsets(ctx, "orders-reader", same)
		require(errors.Is(e, kerr.GroupAuthorizationFailed) || errors.Is(commits.Error(), kerr.GroupAuthorizationFailed), "viewer offset commit was not denied")
		bad := client("invalid")
		authctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		require(bad.Ping(authctx) != nil, "invalid credentials accepted")
		cancel()
		bad.Close()
		untrusted := client("viewer", kgo.DialTLSConfig(&tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}))
		tlsctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		require(untrusted.Ping(tlsctx) != nil, "untrusted certificate accepted")
		cancel()
		untrusted.Close()
	}
	fmt.Printf("PASS %s: 31 committed records; exact bytes/null-empty/headers/timestamps; open LSO=%d HWM=%d; group offsets unchanged (1,2,4); ACL negatives=%t\n", broker, stable[topics[0]][2].Offset, ends[topics[0]][2].Offset, broker == "redpanda")
}

func reset(ctx context.Context) {
	require(string(read(broker+".seeded")) == "streams-seed-v1\n", "reset requires completed owned seed")
	c := client("admin")
	defer c.Close()
	a := kadm.NewClient(c)
	r, e := a.DeleteTopics(ctx, append(topics, "private.denied")...)
	must(e)
	for _, t := range r {
		if !errors.Is(t.Err, kerr.UnknownTopicOrPartition) {
			must(t.Err)
		}
	}
	g, e := a.DeleteGroups(ctx, "orders-reader")
	must(e)
	if !errors.Is(g.Error(), kerr.GroupIDNotFound) {
		must(g.Error())
	}
	for ctx.Err() == nil {
		m, e := a.ListTopics(ctx, topics...)
		must(e)
		done := true
		for _, t := range m {
			if !errors.Is(t.Err, kerr.UnknownTopicOrPartition) {
				done = false
			}
		}
		if done {
			must(os.Remove(filepath.Join(state, broker+".seeded")))
			fmt.Println("reset only seeded topics/group; broker data preserved")
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	panic("reset deletion did not complete")
}

func main() {
	// Error text from brokers is deliberately not printed (may contain credentials).
	defer func() {
		if r := recover(); r != nil {
			if message, ok := r.(string); ok {
				fmt.Fprintln(os.Stderr, "lab seed/check failed:", message)
			} else {
				fmt.Fprintf(os.Stderr, "lab seed/check operation failed (%T); inspect local broker health/permissions\n", r)
			}
			os.Exit(1)
		}
	}()
	require(len(os.Args) == 4, "usage: seed seed|open|check redpanda|kafka /absolute/app/.lab")
	mode := os.Args[1]
	broker = os.Args[2]
	state = os.Args[3]
	require(broker == "redpanda" || broker == "kafka", "unknown broker")
	abs, e := filepath.Abs(".lab")
	must(e)
	require(state == abs, "state must be app-local .lab")
	st, e := os.Lstat(state)
	must(e)
	require(st.IsDir() && st.Mode().Perm() == 0700, "private owned state required")
	require(string(read("owner")) == "streams-local-lab-v1\n", "unknown lab ownership")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if mode != "open" {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, 45*time.Second)
		defer stop()
	}
	switch mode {
	case "seed":
		seed(ctx)
	case "open":
		open(ctx)
	case "check":
		check(ctx)
	case "reset":
		reset(ctx)
	default:
		panic("unknown mode")
	}
}

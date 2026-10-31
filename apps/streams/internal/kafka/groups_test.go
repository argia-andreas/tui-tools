package kafka

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"streams/internal/stream"
)

func TestGroupObservationStates(t *testing.T) {
	set := kadm.TopicsSet{}
	starts := kadm.ListedOffsets{"t": {}}
	ends := kadm.ListedOffsets{"t": {}}
	commits := kadm.OffsetResponses{"t": {}}
	for p := int32(0); p < 6; p++ {
		set.Add("t", p)
		starts["t"][p] = kadm.ListedOffset{Offset: 20}
		ends["t"][p] = kadm.ListedOffset{Offset: 120}
	}
	for p, v := range map[int32]int64{0: 95, 1: -1, 2: 140, 3: 10, 4: 80, 5: 110} {
		commits["t"][p] = kadm.OffsetResponse{Offset: kadm.Offset{At: v}}
	}
	ends["t"][4] = kadm.ListedOffset{Err: kerr.TopicAuthorizationFailed}
	delete(starts["t"], 5)
	rows := observeGroup(set, commits, starts, ends, nil, nil, nil)
	want := []string{"", "uncommitted", "inconsistent snapshot", "expired commit", stream.ErrPermission.Error(), stream.ErrTopic.Error()}
	for i, row := range rows {
		lag, state := stream.Lag(row)
		if state != want[i] {
			t.Fatalf("partition %d: %q, want %q", i, state, want[i])
		}
		if i == 0 && lag != 25 {
			t.Fatal(lag)
		}
	}
	if rows[4].HighWatermark.Known || !rows[4].Committed.Known || rows[2].Committed.Value != 140 {
		t.Fatal("raw observations hidden or clamped")
	}
	// A partition-level error must not contaminate unrelated partitions (map
	// iteration order is deliberately unspecified).
	for i := 0; i < 20; i++ {
		rows = observeGroup(set, commits, starts, ends, nil, nil, nil)
		if rows[0].Err != nil || !rows[0].Start.Known {
			t.Fatal("partition failure leaked")
		}
	}
}

func TestPublicErrorsAreSanitizedAndTyped(t *testing.T) {
	for _, pair := range []struct{ in, want error }{
		{kerr.TopicAuthorizationFailed, stream.ErrPermission}, {kerr.GroupAuthorizationFailed, stream.ErrPermission},
		{kerr.SaslAuthenticationFailed, stream.ErrAuthentication}, {kerr.OffsetOutOfRange, stream.ErrExpired},
		{&kgo.ErrDataLoss{}, stream.ErrExpired},
		{kerr.UnknownTopicID, stream.ErrTopic}, {kgo.ErrMaxDecompress, stream.ErrLimit},
		{errors.New("response exceeds BrokerMaxReadBytes"), stream.ErrLimit},
		{context.Canceled, context.Canceled}, {context.DeadlineExceeded, context.DeadlineExceeded},
		{errors.New("credential=do-not-print payload=hostile"), stream.ErrTransport},
	} {
		got := safe(pair.in)
		if !errors.Is(got, pair.want) || strings.Contains(got.Error(), "do-not-print") || strings.Contains(got.Error(), "hostile") {
			t.Fatal("unsafe error", got)
		}
	}
}

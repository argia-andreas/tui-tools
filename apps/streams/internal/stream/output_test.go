package stream_test

import (
	"bytes"
	"strings"
	"testing"

	"streams/internal/stream"
)

func TestExactRecordOutput(t *testing.T) {
	r := stream.Record{Key: []byte{}, Value: []byte("\x1b]52;c;bad\a\xff\u202e"), Headers: []stream.Header{{Key: "x", Value: []byte("first")}, {Key: "x", Value: []byte("second")}}}
	safe := stream.SafeText(r.Value)
	if strings.ContainsAny(safe, "\x1b\a\u202e") || !strings.Contains(safe, "\\xff") {
		t.Fatalf("unsafe output %q", safe)
	}
	got, err := stream.FieldPayload(r, "value", 0)
	if err != nil || !bytes.Equal(got, r.Value) {
		t.Fatal("raw copy changed")
	}
	got, _ = stream.FieldPayload(r, "header", 1)
	if string(got) != "second" {
		t.Fatal("lost duplicate header order")
	}
	if _, err = stream.FieldPayload(r, "key", 0); err != nil {
		t.Fatal("empty is not null")
	}
	r.Key = nil
	if _, err = stream.FieldPayload(r, "key", 0); err == nil {
		t.Fatal("null copied as bytes")
	}
	json := []byte(`{"total":249.50,"currency":"SEK"}`)
	_ = stream.Content(json, "json")
	if string(json) != `{"total":249.50,"currency":"SEK"}` {
		t.Fatal("format mutated bytes")
	}
}

func TestGroupObservationStates(t *testing.T) {
	p := stream.GroupPartition{Start: stream.OffsetValue{Value: 10, Known: true}, Committed: stream.OffsetValue{Value: 95, Known: true}, HighWatermark: stream.OffsetValue{Value: 120, Known: true}}
	if lag, state := stream.Lag(p); lag != 25 || state != "" {
		t.Fatal(lag, state)
	}
	p.Committed.Known = false
	if _, s := stream.Lag(p); s != "uncommitted" {
		t.Fatal(s)
	}
	p.Committed = stream.OffsetValue{Value: 121, Known: true}
	if _, s := stream.Lag(p); s != "inconsistent snapshot" {
		t.Fatal(s)
	}
	p.Committed.Value = 9
	if _, s := stream.Lag(p); s != "expired commit" {
		t.Fatal(s)
	}
}

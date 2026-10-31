// Package stream defines the read-only contracts of the streams workbench.
package stream

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPermission     = errors.New("permission denied")
	ErrAuthentication = errors.New("authentication failed")
	ErrTransport      = errors.New("broker unavailable; check advertised addresses and TLS")
	ErrTopic          = errors.New("topic missing or changed; reopen topic")
	ErrExpired        = errors.New("position expired or truncated; seek explicitly")
	ErrLimit          = errors.New("read limit exceeded; select fewer partitions or seek past oversized data")
	ErrClosed         = errors.New("reader closed")
)

type TopicRef struct{ Name, ID string }
type Topic struct {
	Ref        TopicRef
	Internal   bool
	Partitions []Partition
}
type Partition struct {
	ID, Leader       int32
	Replicas, InSync []int32
	Err              error
}
type RecordID struct {
	Topic     TopicRef
	Partition int32
	Offset    int64
}
type Header struct {
	Key   string
	Value []byte
}
type Record struct {
	ID            RecordID
	Time          time.Time
	TimestampType string
	Key, Value    []byte
	Headers       []Header
}
type Position struct {
	Partition int32
	Next, End int64
}
type Cursor struct {
	Topic     TopicRef
	Positions []Position
}
type ReadRequest struct {
	Topic      TopicRef
	Partitions []int32
	Mode       string
	Offset     int64
	Time       time.Time
	Cursor     *Cursor
	Follow     bool
}
type PartitionIssue struct {
	Partition int32
	Err       error
}
type Batch struct {
	Records []Record
	Next    Cursor
	Stop    string
	Issues  []PartitionIssue
}
type Reader interface {
	Cursor() Cursor
	Next(context.Context) Batch
	Close()
}
type Backend interface {
	Topics(context.Context) ([]Topic, error)
	Groups(context.Context) ([]string, error)
	Group(context.Context, string) (GroupObservation, error)
	Open(context.Context, ReadRequest) (Reader, error)
	Close()
}
type OffsetValue struct {
	Value int64
	Known bool
}
type GroupPartition struct {
	Topic                           string
	Partition                       int32
	Start, Committed, HighWatermark OffsetValue
	Err                             error
}
type GroupObservation struct {
	Name, State       string
	Started, Finished time.Time
	Partitions        []GroupPartition
	Partial           bool
}

func (r Record) Bytes() int {
	n := len(r.ID.Topic.Name) + len(r.Key) + len(r.Value)
	for _, h := range r.Headers {
		n += len(h.Key) + len(h.Value)
	}
	return n
}

// CloneCursor separates a saved delivery position from a live reader's state.
func CloneCursor(c Cursor) Cursor {
	c.Positions = append([]Position(nil), c.Positions...)
	return c
}

// Lag describes an observation, not a count of pending business events.
func Lag(p GroupPartition) (int64, string) {
	if p.Err != nil {
		return 0, p.Err.Error()
	}
	if !p.Committed.Known {
		return 0, "uncommitted"
	}
	if !p.HighWatermark.Known || !p.Start.Known {
		return 0, "unavailable"
	}
	if p.Committed.Value < p.Start.Value {
		return 0, "expired commit"
	}
	if p.Committed.Value > p.HighWatermark.Value {
		return 0, "inconsistent snapshot"
	}
	return p.HighWatermark.Value - p.Committed.Value, ""
}

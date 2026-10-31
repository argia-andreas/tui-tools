package kafka

import (
	"context"
	"sort"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"streams/internal/stream"
)

func (c *Client) Group(ctx context.Context, name string) (out stream.GroupObservation, err error) {
	out.Name, out.Started = name, time.Now()
	defer func() {
		out.Finished = time.Now()
		if err != nil {
			out.Partial = true
		}
	}()
	ctx, done := c.operation(ctx)
	defer done()
	ds, err := c.admin.DescribeGroups(ctx, name)
	if err != nil {
		return out, safe(err)
	}
	d, ok := ds[name]
	if !ok {
		return out, stream.ErrTransport
	}
	if d.Err != nil {
		return out, safe(d.Err)
	}
	switch d.State {
	case "Empty", "Dead", "Stable", "PreparingRebalance", "CompletingRebalance", "Assigning", "Reconciling":
		out.State = d.State
	default:
		out.State = "unknown"
	}
	commits, commitErr := c.admin.FetchOffsets(ctx, name)
	set := d.AssignedPartitions()
	for t, ps := range commits {
		for p := range ps {
			set.Add(t, p)
		}
	}
	var topics []string
	n := 0
	for t, ps := range set {
		topics = append(topics, t)
		n += len(ps)
	}
	if n > catalogLimit {
		return out, stream.ErrLimit
	}
	if len(topics) == 0 {
		if commitErr != nil {
			return out, safe(commitErr)
		}
		return out, nil
	}
	starts, startErr := c.admin.ListStartOffsets(ctx, topics...)
	ends, endErr := c.admin.ListEndOffsets(ctx, topics...)
	out.Partitions = observeGroup(set, commits, starts, ends, commitErr, startErr, endErr)
	for _, p := range out.Partitions {
		if p.Err != nil || !p.Start.Known || !p.HighWatermark.Known {
			out.Partial = true
		}
	}
	return out, nil
}

// Observations deliberately preserve raw negative/missing commits and do not
// use kadm.Lag, whose substitutions and clamps would hide important states.
func observeGroup(set kadm.TopicsSet, commits kadm.OffsetResponses, starts, ends kadm.ListedOffsets, commitErr, startErr, endErr error) []stream.GroupPartition {
	var out []stream.GroupPartition
	for t, ps := range set {
		for p := range ps {
			x := stream.GroupPartition{Topic: t, Partition: p}
			if v, ok := commits.Lookup(t, p); ok {
				if v.Err != nil {
					x.Err = safe(v.Err)
				} else if v.At >= 0 {
					x.Committed = stream.OffsetValue{Value: v.At, Known: true}
				}
			}
			if commitErr != nil {
				x.Err = safe(commitErr)
			}
			var se, ee error
			x.Start, se = observedOffset(starts, t, p, startErr)
			if se != nil && x.Err == nil {
				x.Err = se
			}
			x.HighWatermark, ee = observedOffset(ends, t, p, endErr)
			if ee != nil && x.Err == nil {
				x.Err = ee
			}
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Topic != out[j].Topic {
			return out[i].Topic < out[j].Topic
		}
		return out[i].Partition < out[j].Partition
	})
	return out
}

func observedOffset(all kadm.ListedOffsets, t string, p int32, requestErr error) (stream.OffsetValue, error) {
	v, ok := all.Lookup(t, p)
	if ok {
		if v.Err != nil {
			return stream.OffsetValue{}, safe(v.Err)
		}
		if v.Offset >= 0 {
			return stream.OffsetValue{Value: v.Offset, Known: true}, nil
		}
	}
	// kadm represents metadata failures as a topic-level partition -1 entry.
	if v, ok := all.Lookup(t, -1); ok && v.Err != nil {
		return stream.OffsetValue{}, safe(v.Err)
	}
	if requestErr != nil {
		return stream.OffsetValue{}, safe(requestErr)
	}
	return stream.OffsetValue{}, stream.ErrTopic
}

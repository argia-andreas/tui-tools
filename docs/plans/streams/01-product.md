# Product: streams

## Problem

“Let me see what is in a Kafka topic, inspect the exact event, and understand
whether a consumer is falling behind—without opening a heavyweight web console
or accidentally affecting a running application.”

`streams` is the event-log sibling of transfer, queue and logs. It treats topics,
partitions and offsets as first-class concepts, rather than pretending Kafka is
a queue whose messages can be peeked, acknowledged or individually removed.

## Success metric

Proposed target, not a measured result: complete at least four of five realistic
investigations in under 60 seconds each from a configured connection: find a
topic, inspect an event at an exact partition/offset, find a loaded event by key,
follow new events, and identify the lagging partition of a consumer group.
Use fictional or authorized test events; measure correctness as well as time.

## Announcement — the blog post before the feature

Meet streams: a small, opinionated terminal workbench for Kafka-compatible event
streams. Find a topic, browse its partitions, jump to an offset or time, and
inspect keys, headers and values without losing your place. Follow new events
or pause to investigate, with clear limits on what has actually been read.
See consumer-group offsets and lag without joining the group or changing its
position. The same compact controls, command palette and navigation as the rest
of the suite keep the focus on your events, not on learning another console.

## Product decisions proposed for approval

### 1. Read-only first

- Browse topics and partition metadata, read retained events and inspect consumer
  groups/lag. No producing, topic creation/deletion, configuration edits, offset
  resets, acknowledgements or replay in the first release.
- Reading must not join an application's consumer group or commit offsets.
  Read-only does not mean free: reads consume broker bandwidth and must be bounded.
- Explicit connection choice; never silently fall back to sample data. Provide a
  clearly labelled offline fixture for exploring the interface without a cluster.
- One active cluster and one topic at a time. Saved connections are useful;
  cross-cluster dashboards are not an initial goal.

### 2. Three useful views, no dashboard wall

- **Topics:** searchable list with topic name, partition count and available
  replication/health information. Unknown and permission-denied are not zero.
- **Events:** collapsible topic navigation, central event rows and a right-hand
  inspector. Choose all partitions or one explicit partition. Each row identifies
  partition, offset, timestamp and key preview; selection does not start a new read.
- **Groups:** group state and per-topic/partition committed position, observed end
  position and lag when available. Show observation time and unavailable values.
  Lag is an offset distance, not an exact count of pending business events, and
  separately observed positions are not an atomic cluster snapshot.

### 3. Kafka semantics stay visible

- Ordering is guaranteed only inside a partition. An all-partitions view is an
  arrival-order interleave, not a globally ordered timeline. Show that label.
- Offer a bounded recent window, earliest retained, explicit partition + offset,
  timestamp seek and Follow from now. Every request shows its bounds. Timestamp
  seek selects a starting position per partition, not an exact-time global event.
- Retention, compaction and transactions can leave offset gaps. Do not promise
  every integer offset exists or infer message counts from offset spans.
- Show only committed transactional records initially. Explain that open
  transactions can delay visibility relative to the observed partition end.
- Pause stops fetching and freezes the visible buffer. Resume continues from the
  next unread positions; report expired positions and any bounded catch-up gaps.
  Jump to latest is a separate, deliberate action, never an implicit resume.
- Bound reads by record count, bytes and time; bound retained UI memory too.
  Report truncation, eviction, partial partition failures and canceled reads.

### 4. Inspect exact content, without guessing

- Inspector exposes topic, partition, offset, timestamp/type, key, ordered headers
  (including duplicate names), and value. Distinguish null from empty bytes;
  a null value is visibly a tombstone, not a missing preview.
- Start with UTF-8/text, formatted JSON when valid, and a byte-safe hex view.
  Binary data must not be presented as corrupted text or silently discarded.
- Copy an explicitly selected field/value; copy success is conditional on the
  clipboard transport actually accepting it. Never copy only a truncated preview.
- Separate local find in loaded events from broker reads. Search by literal text
  or key within the loaded window; show that scope. No promise of cluster-wide
  full-text search, arbitrary queries or a key index.
- Schema Registry, Avro, Protobuf and schema-aware decoding are follow-up work,
  not implied by Kafka compatibility. Undecoded records remain inspectable bytes.

### 5. Minimal and familiar

- Shared suite language: compact square controls, one obvious focus owner,
  Ctrl+P Actions, contextual Help, Tab between visible panes, select-first rows,
  explicit Inspect, and complete mouse/keyboard paths with mouse-off fallback.
- Wide terminals show topic navigation, events and inspector. Narrow terminals
  show the list or reader, not three squeezed columns. Preserve selection on return.
- Status always identifies cluster, read-only mode, selected topic/partitions,
  active read position, follow/paused state and limits. UTC timestamps by default.
- Connection failures and missing permissions offer useful explanations without
  exposing credentials or putting event content into diagnostic logs by default.

## Compatibility boundary

Target Apache Kafka and Kafka-protocol-compatible brokers, not every product
that offers an event API. Initial acceptance should name and test an Apache Kafka
release and a Redpanda release; exact versions and authentication support belong
in the architecture gate. TLS and authenticated remote clusters must be covered
before claiming production readiness. Vendor-specific cloud login, Azure Event
Hubs' Kafka surface, managed-service administration and every SASL mechanism are
not automatically promised by “Kafka-compatible.” Unsupported operations must
explain their limitation rather than fabricate results.

## Deliberate exclusions

No broker administration, writes, consumer-offset mutation, automatic credential
acquisition, cross-cluster merge, monitoring/alerting service, stream processing,
automatic AI upload or MCP server in the initial scope. These can be considered
after the read-only workflow is useful and verified; none is a hidden prerequisite.

## Screens

- `mockups/workbench.html`: static fictional Events workbench, wide three-pane
  layout and narrow list-only adaptation. Illustrates the existing suite contract,
  not a working terminal or an approved shortcut map. Topics/Groups surfaces and
  detailed failure/seek editors will be specified after this product gate.

## Next gates, not yet designed or approved

1. Architecture: inspect the shared UI and current renamed app structure; choose
   the Kafka client, connection/security contract, read lifecycle and local lab.
2. Program design: define ownership, cancellation/stale-result rules, limits,
   persistence and behavioral tests before implementation.
3. Vertical slices: approve independently runnable increments, beginning with an
   offline UI tracer, then a real bounded topic read, followed by seeking/follow,
   inspection, group lag and compatibility/hardening. This is a proposed order,
   not the Gate 4 implementation plan.

Safe handoff: these documents capture all current proposals. No later gate is
approved and no implementation should start from this document alone.

# Architecture: streams

Gate 2 proposal. Product approved 2026-10-30; implementation is not authorized
until Gates 2–4 are approved.

## Fit

- Add independent Go module `streams` under `apps/streams`, executable
  `cmd/streams`, build output `bin/streams`. Follow the suite's Go 1.27.3,
  Bubble Tea v1.3.10 and Lip Gloss v1.1.0 baseline.
- Require `terminalui v0.0.0` with the existing local replacement pattern.
  Reuse ActionBar, Palette, Help, Surface and current-frame navigation regions.
  The app owns topic/group/event layouts, reader grammar and checked actions.
  Do not introduce a shared Kafka framework or import peer apps' internal code.
- Add the module to `go.work`, add `streams` to the root build script's explicit
  allowlist and terminal-test target, and document it in the suite README when
  implementation starts. Root vet/race checks and setup dependency downloads
  already discover `apps/*/go.mod`; no parallel setup path is necessary.
- App boundaries: `config` owns explicit connection profiles and local preferences;
  `stream` owns read contracts, positions and budgets; `kafka` implements broker
  access; `fixture` implements deterministic offline access; `ui` owns presentation
  and user intent. Theme is app-owned, mapped to shared semantic colors.
  Exact types/files are Gate 3 work, not a new generic service layer by default.

## Endpoints

None. A terminal executable connects directly to configured Kafka bootstrap
addresses and the broker addresses returned by metadata. No web server, proxy,
daemon, MCP endpoint or cluster-side installation.

## Data

No application database or durable event cache.

- An explicit configuration file holds named profiles: bootstrap addresses,
  security mode, CA/client certificate references, SASL method/username and a
  password-file reference. Choose `--config <path> --connection <name>` or
  `--fixture`; reject ambiguous combinations. No ambient auto-connect.
- Default preference location is the OS user-config directory under `streams`;
  store only theme/layout and safe navigation preferences on explicit Save.
  Credentials, payloads and consumer positions are not persisted. Restarting
  does not pretend to resume an earlier broker snapshot.
- Strict, size-bounded configuration parsing; reject unknown/duplicate keys and
  invalid combinations. Secrets are loaded from private files, not command-line
  values, saved UI fields or interpolated shell commands. Local writes are
  atomic and private, following existing suite conventions.
- Event identity includes connection generation, topic identity where available,
  partition and offset. Recreated topics must invalidate previous cursors;
  brokers without topic IDs require explicit warning/reopen after detected
  disappearance or incompatible metadata. No claim of perfect detection there.
- Preserve key/value bytes and nullness, ordered duplicate headers and native
  timestamp metadata. Render terminal-safe text: escape controls including ESC,
  OSC and misleading invisible characters. Hex is lossless. Copy raw data only
  through an explicit field/raw action, never through the display sanitizer.

## Flow

### Connect and discover

CLI validates mode → loads selected profile → creates read-only Kafka adapter →
requests topic metadata → UI displays topics or a specific, sanitized failure.
Metadata requests must not opt into automatic topic creation. Refresh is explicit;
no background polling of every topic or group. Topic selection previews metadata;
Open starts a bounded read. Internal topics are hidden by default with an explicit
Show internal topics action.

Use `github.com/twmb/franz-go` (`kgo`, and `kadm` for read-only metadata/group
operations). Directly assign partitions; do not configure a consumer group,
autocommit, a producer or any mutation action. The app-facing adapter exposes only
reads and Close, not the general client. Independent metadata/group and record
clients avoid a slow fetch blocking discovery; there is only one active record
reader. Client and broker release pins must be recorded before real integration.

Pin franz-go **v1.22.2** and its independently versioned kadm module **v1.19.0**.
The client's Go 1.26 minimum fits the repository's Go 1.27.3 baseline.
Use `ConsumePartitions` without `ConsumerGroup`/`ShareGroup`; do not add
`DisableAutoCommit`, which is a group-only option rejected without a group.
Do not enable `AllowAutoTopicCreation` (default is false).

### Seek and bounded read

1. Resolve topic partitions and per-partition retained start, readable end and
   requested start. A timestamp lookup with no matching event is an empty start,
   not an automatic rewind. An expired explicit offset needs visible recovery,
   never a silent default-client offset reset.
2. Recent means a bounded **offset window**, not “the last N actual events.”
   Gaps and aborted transactions can yield fewer records. Capture finite end
   positions before a historical read, so a busy topic cannot make it infinite.
3. Assign exact positions and fetch with read-committed isolation. Retain each
   partition's next unread position independently. Buffered records not accepted
   into the delivered batch must remain readable after reopening. Do not derive
   progress from row count or from “largest visible offset + 1” across all
   partitions. Distinguish the broker fetch cursor from the app's delivery cursor.
4. Stop at captured ends, request budgets, cancellation or deadline. Report why,
   which partitions completed and which failed. A partition with no visible
   records may still have advanced across control/aborted records.
5. Return owned immutable bytes and continuation state. The UI accepts results
   only for the current connection/read generation; stale results cannot replace
   a new topic or revive a canceled Follow.

Explicitly set `FetchIsolationLevel(ReadCommitted())`, since the default is
read-uncommitted. Capture history boundaries with `kadm.ListCommittedOffsets`
(last stable offsets), not `ListEndOffsets` (high watermarks). Despite its name,
ListCommittedOffsets does not read consumer-group commits. Timestamp helper
`ListOffsetsAfterMilli` uses uncommitted isolation and substitutes end for no
match: enforce the captured stable boundary independently, or issue a committed
ListOffsets request. Never wait for a visible record exactly at end minus one.

Proposed initial limits: 500 delivered records, 4 MiB raw event bytes and 5 seconds
per historical read; 2,000 records / 16 MiB retained raw data in the reader, plus
bounded display/JSON expansion. Follow uses small scheduled batches with no
unbounded event channel. Bound active partitions and concurrent fetches as well;
very large topics require explicit partition selection instead of fan-out to all.
Gate 3 sets these caps and their exact accounting.

Kafka fetch byte settings are not hard memory limits: an oversized first batch
and decompression can exceed them. Separate accepted-record/retained-byte limits
from transport limits, cap broker response size/concurrency, and expose an
oversized-record refusal rather than silently advancing past uninspectable data.
Do not claim a fixed process RSS ceiling based on a fetch setting.

Use `MaxConcurrentFetches(0)` for poll-driven fetching without background fetch
buffering, plus explicit `FetchMaxBytes`, `FetchMaxPartitionBytes`,
`BrokerMaxReadBytes` and `MaxDecompressBatchBytes`. `PollRecords(ctx, n)` alone
only limits delivered record count. A decompression-limit failure is a stopped
partition with a visible reason, not an automatic retry/skip loop. Exact settings
and delivery-cursor handling across empty/control batches are Gate 3 decisions.

### Follow, pause and lifecycle

Follow starts at resolved current readable ends, then issues one bounded fetch
batch at a time. Help/palette can remain open while data arrives, but selection
stays anchored by native identity and is not forced to the newest row.

Pause cancels the active batch and closes its record client so client prefetch
cannot continue invisibly. Freeze visible rows and retain the next positions after
the last accepted batch. Resume creates a new reader there; bounded catch-up and
retention failures are explicit. An in-flight response may have consumed network
bytes before cancellation; promise no subsequent scheduled fetch, not retroactive
network cancellation. Switching cluster/topic or seeking invalidates the old read,
cancels and closes it before starting the replacement. Quit closes all clients.

Eviction removes oldest loaded events and labels that loss. If the selected event
would be evicted, pause rather than silently moving the user's selection. Under
minimum terminal dimensions only safe cancellation/quit work; hidden controls
cannot start reads, save settings or copy data.

### Groups and lag

Explicit group discovery → selected group description/committed offsets →
per-partition end positions → timestamped table. No group membership or offset
commit. Report each authorization failure separately so topic access can work
without group-list permissions. Refresh only the selected group on request.

Label the end offset basis (high watermark versus last stable offset) rather than
mixing it with committed-only visibility. Missing commits, negative/inconsistent
differences, deleted partitions and missing permissions are distinct states, not
clamped zero. Never sum unavailable values into an apparently complete total.

Compute the displayed lag from raw group commits and high watermarks. Do not
reuse `kadm.Lag`'s derived values/Total directly: it substitutes retained span for
missing commits, clamps negative lag, and totals can omit failed partitions.
Keep those underlying observations and errors visible instead.

### Inspector, find and copy

Selection reads already loaded owned bytes → sanitized preview → Inspect opens
reader retaining list selection. JSON formatting is size/depth bounded and does
not alter the raw view. Find examines only loaded key/value text using a visible
literal/case scope; binary exact content remains available in hex.

Use app-local clipboard handling following the suite's native helper and explicit
OSC52-trust policy; do not expand `terminalui` for a one-app need. Invoke helpers
without a shell; show bytes/field name but not sensitive copied content. OSC52
transmission or a successful native command does not prove clipboard receipt.

## External

### Security and compatibility

- Initial remote support: verified TLS with system roots or explicit CA; optional
  mutual TLS; SASL PLAIN, SCRAM-SHA-256 and SCRAM-SHA-512 over TLS. No skip-verify
  option. Anonymous TLS is supported when the broker permits it.
- Plaintext is allowed only for an explicitly labelled local development profile;
  it is not the default for remote clusters. TLS/SASL failures never downgrade.
- No OAuth/cloud-login plugins, Kerberos, Schema Registry or credential command
  execution initially. No secret environment variables are required by the app.
- Document required read/describe permissions for selected topics and describe
  permissions for groups. The application cannot make an overprivileged
  credential read-only; use a broker-enforced read-only principal for acceptance.
- Bootstrap access alone is insufficient: advertised broker addresses must also
  be reachable and certificate-valid. Show this distinction without dumping
  credentials or raw broker error payloads.

### Lab and verification

Keep fixtures and real broker tests separate. Deterministic fixtures cover null
versus empty, duplicate headers, binary/escape content, offset gaps, asymmetric
partition positions, partial errors, retention and stale results.

Provide an explicit opt-in local lab with pinned Apache Kafka and Redpanda
releases (and immutable image digests when containers are used). Only a separate
test seeder/admin creates test topics, writes events and configures ACLs/groups;
the viewer uses read-only credentials. Never seed through production app code.
Lab startup/seeding/destruction must not run as part of build or ordinary checks.

Proposed compatibility baselines: Apache Kafka **4.3.2** and Redpanda **v26.2.5**.
These are explicit test targets, not a claim of universal or latest-version
support. Resolve and verify downloadable artifacts/image digests before lab code
lands; image availability and live compatibility have not yet been tested.

Acceptance must exercise real transactional commit/abort and open transactions,
seek boundaries, pause/resume without offset mutation, read-only ACLs, TLS/SASL,
oversized records, cancellation and unavailable permissions on both brokers.
Capture application-visible output and independently inspect committed offsets
before/after, not just mocked method calls. Compaction/retention scenarios must
wait for observed broker state, not assume cleanup has happened after a sleep.

This orb has the Docker CLI but no running Docker daemon/socket. Do not present
fixture PTYs as broker integration evidence. During implementation establish a
working isolated local broker (container runtime or broker-native supervised
process), or report live compatibility as blocked. No remote cluster is assumed.

Use independent module race/vet checks and real-binary PTYs as the suite does.
Inspect wide, compact, reader, seek, group, error and disabled states, plus record
mouse/keyboard transitions. Full root checks guard the other apps after build
integration. Native-terminal clipboard receipt/usability remains separate from
orb screenshots and PTY checks.

## Open decisions before Gate 3

- Finalize delivery-cursor handling across empty/control batches against the
  pinned client; preserve unread buffered records when closing/reopening.
- Define explicit caps for partition fan-out, wire responses, decompression and
  derived render data. Reject unsupported oversized reads honestly.
- Confirm no broader auth mechanism or schema decoding is needed for the first
  target cluster. Current scope is deliberate, not universal Kafka compatibility.

## Research provenance

Source inspection, not executed broker validation:

- [Pinned client configuration and bounds](https://github.com/twmb/franz-go/blob/7b6cac30a57a462216fd27e2408eadb7af80a224/pkg/kgo/config.go)
- [Pinned client polling and assignment](https://github.com/twmb/franz-go/blob/7b6cac30a57a462216fd27e2408eadb7af80a224/pkg/kgo/consumer.go)
- [kadm v1.19.0 metadata/offset helpers](https://github.com/twmb/franz-go/blob/85ea1416bff369c2e896e08acfed466d25d05004/pkg/kadm/metadata.go)
- [kadm v1.19.0 lag calculations](https://github.com/twmb/franz-go/blob/85ea1416bff369c2e896e08acfed466d25d05004/pkg/kadm/groups.go)
- [Kafka 4.3.2 announcement](https://kafka.apache.org/blog/2026/10/23/apache-kafka-4.3.2-release-announcement/)
  (release evidence from search excerpts; direct page retrieval timed out).
- [Redpanda release notes](https://docs.redpanda.com/streaming/current/reference/releases/redpanda/#v2625-2026-10-16):
  v26.2.5 has a detailed entry; page metadata mentions v26.2.6. Use the documented
  baseline rather than implying v26.2.5 is the latest.

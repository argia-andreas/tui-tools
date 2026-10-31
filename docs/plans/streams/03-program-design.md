# Program design: streams

Gate 3 proposal. Product and architecture approved 2026-10-30. These are design
signatures, not implementation. Gate 4 must be approved before app code exists.

## Files

Paths below are under `apps/streams/` unless stated otherwise. Add files with
their owning slice, not empty scaffolding for the entire plan up front.

| Files | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | Independent `streams` module; approved pinned clients/UI dependencies |
| `cmd/streams/main.go`, `main_test.go` | Mode/flag validation, signal context, dependency lifecycle, terminal startup |
| `internal/config/config.go`, `config_test.go` | Strict profile parsing, TLS/SASL inputs and private secret references |
| `internal/config/settings.go`, `settings_test.go` | Small explicit-save appearance preferences; no payloads/cursors |
| `internal/stream/stream.go` | App-facing read-only contracts, native identities and typed outcomes |
| `internal/stream/output.go`, `output_test.go` | Safe terminal text, bounded JSON/hex and exact field-copy payloads |
| `internal/kafka/client.go` | Metadata client lifecycle, topics/groups and sanitized errors |
| `internal/kafka/reader.go`, `reader_test.go` | Resolve positions, direct assignment, bounded polling and delivery cursors |
| `internal/kafka/groups.go`, `groups_test.go` | Raw group commit/end observations; completeness-aware lag |
| `internal/kafka/integration_test.go` | Opt-in real broker contract scenarios, not automatically run against remote hosts |
| `internal/fixture/provider.go` | Deterministic implementation of the same backend/reader contracts |
| `internal/theme/theme.go` | App palette and shared semantic-style mapping; Midnight initially |
| `internal/ui/model.go`, `model_test.go` | Bubble Tea state, commands, generation gates and resource ownership |
| `internal/ui/view.go` | Topics/events/groups layout, compact reader and resize notice |
| `internal/ui/actions.go`, `navigation.go`, `help.go` | Shared controls/palette/help adapters and checked app intents |
| `internal/ui/editor.go` | Seek/find/connection/settings surfaces; Apply/Cancel drafts |
| `internal/ui/inspector.go`, `clipboard.go` | Explicit field selection, reader modes, native/opt-in OSC52 copy |
| `scripts/terminal-smoke.py`, `scripts/requirements.txt` | Real executable PTYs with pinned dependencies following suite convention |
| `lab/compose.yaml`, `lab/README.md`, `scripts/lab.py` | Explicit isolated broker lifecycle, owned volumes and release/digest pins |
| `lab/seed/main.go` | Separate opt-in test-only admin/seeder; never imported by viewer |
| `Makefile`, `.gitignore`, `README.md`, `docs/ui-click-inventory.md` | Build/check/run targets, private state exclusions and operational/interaction contracts |

Root edits: `go.work` includes streams; `build` accepts/builds streams;
`Makefile` adds its terminal suite; `README.md` lists usage and compatibility.
No peer-app or shared-library API changes are planned. Setup already discovers
module and Python requirements; only change setup if an actual new dependency
needs it, with the orb-setup workflow. Document any approved revision to this list.

## Types & signatures

### Domain contract (`internal/stream`)

```go
type TopicRef struct { Name, ID string } // ID empty only when unavailable
type Topic struct {
    Ref TopicRef
    Internal bool
    Partitions []Partition
}
type Partition struct {
    ID, Leader int32
    Replicas, InSync []int32
    Err error // sanitized typed issue; unknown is not an empty healthy replica set
}
type RecordID struct { Topic TopicRef; Partition int32; Offset int64 }
type Header struct { Key string; Value []byte }
type Record struct {
    ID RecordID
    Time time.Time
    TimestampType string
    Key, Value []byte // nil is null, nonnil empty is empty
    Headers []Header // duplicate names and original order preserved
}
type Position struct {
    Partition int32
    Next, End int64 // End exclusive; historical LSO snapshot
}
type Cursor struct { Topic TopicRef; Positions []Position }
type ReadRequest struct {
    Topic TopicRef
    Partitions []int32 // resolved explicit IDs; maximum 32
    Mode string // recent | earliest | offset | time | follow | resume
    Offset int64 // offset mode requires exactly one partition
    Time time.Time
    Cursor *Cursor // resume only; never accepted across connection generations
    Follow bool // resume distinguishes following from finite continuation
}
type PartitionIssue struct { Partition int32; Err error }
type Batch struct {
    Records []Record
    Next Cursor
    Stop string // continue | end | records | bytes | deadline | limit | error
    Issues []PartitionIssue
}
type Reader interface {
    Cursor() Cursor // immutable opening snapshot; defensive copy
    Next(context.Context) Batch // serialized by owner, never overlapping
    Close() // idempotent, concurrent with Next; waits for owned resources
}
type Backend interface {
    Topics(context.Context) ([]Topic, error)
    Groups(context.Context) ([]string, error)
    Group(context.Context, string) (GroupObservation, error)
    Open(context.Context, ReadRequest) (Reader, error)
    Close() // record readers close first; no mutations exposed
}
type OffsetValue struct { Value int64; Known bool }
type GroupPartition struct {
    Topic string
    Partition int32
    Start, Committed, HighWatermark OffsetValue
    Err error
}
type GroupObservation struct {
    Name, State string
    Started, Finished time.Time
    Partitions []GroupPartition
    Partial bool
}
```

Use a small set of typed/sentinel errors for authorization, authentication,
transport, missing topic, changed topic, expired position and exceeded limits.
Keep context cancellation/deadline detectable with `errors.Is`. Public errors
contain no payload/credential text. Do not expose underlying client errors for
the renderer to print. Records are immutable owned copies after delivery.
No generic pagination, event bus, plugin registry or pluggable serializer API.

### Construction and UI boundary

```go
// internal/config
func Load(path string) (Config, error)
func (c Config) Connection(name string) (Profile, error)
func LoadSettings(path string) (Settings, error)
func SaveSettings(path string, s Settings) error

// internal/kafka; profiles expose paths, not printable loaded secret fields
func New(profile config.Profile) (*Client, error)

// internal/fixture
func New() stream.Backend

// internal/ui
type Options struct {
    Context context.Context
    Backend stream.Backend
    Connection string
    Connections []string // names only, no secret fields
    OpenConnection func(context.Context, string) (stream.Backend, error)
    Fixture, TrustOSC52 bool
    SettingsPath string
}
func New(options Options) Model
func (m Model) Init() tea.Cmd
func (m Model) Update(tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
func (m Model) Close() // closes shared session owner, including replaced backends

// internal/stream/output.go
func SafeText(raw []byte) string
func FieldPayload(record Record, field string, headerIndex int) ([]byte, error)
```

`Config`, `Profile` and `Settings` are concrete config-owned JSON structures, not
runtime service interfaces. Profile contains name/bootstrap list, explicit
TLS/local-plaintext mode, CA/cert/key paths and optional SASL mechanism/user/
password-file path. Cert/key must appear together. Secret paths resolve relative
to the config directory. Reject symlinks/nonregular or group/world-accessible
secret files on Unix. No saved secret values or command execution.
`Settings` contains sidebar/inspector/ASCII/density preferences only. Flag/config errors
exit 2; initialization/terminal failures exit 1. `--help` exits 0.

Main binds OpenConnection to the loaded config; the UI sees only profile names
and read-only backends. Fixture mode has no connection factory. Backend ownership
transfers to the session owner at New; main defers Model.Close even when terminal
startup fails. Closing a copied Model closes the same shared owner. A late
connection result must be closed, just like a late reader result.

`FieldPayload` distinguishes exact raw fields from explicit encoded/JSON views;
headers use indexes, never a map lookup by name. Null copy is refused with an
explanation rather than copied as the four bytes `null`. Hex mode copies its
explicit hex representation; raw binary copy is a separately labelled action.

## Read, cursor and limit decisions

- Historical reads: maximum 500 records / 4 MiB raw content / 5 seconds from Open
  through batch completion. “Continue” is a new bounded operation at returned
  positions, preserving the original finite ends. No automatic full-topic scan.
- Recent starts at max(retained start, LSO − 500) **per selected partition**;
  the total delivered budget remains 500. Show this offset-window policy. A hot
  partition may use the shared budget; do not imply a fair sample across partitions.
- Follow uses up to 100 records / 1 MiB / 500 ms per Next, with a 250 ms UI tick
  after completion, no overlapping command or hidden event queue. Each accepted
  batch advances only its own positions. Budget stops are resumable, not gaps.
- Maximum selected partitions: 32. Larger topics remain discoverable but require
  explicit selection of a smaller subset. Catalog acceptance caps: 10,000 topics,
  10,000 groups and 10,000 partition rows per group. Limit errors state that the
  view is incomplete. Broker responses are bounded before application parsing.
- FetchMaxBytes 4 MiB; FetchMaxPartitionBytes 1 MiB; BrokerMaxReadBytes 16 MiB;
  MaxDecompressBatchBytes 8 MiB; MaxConcurrentFetches 0; FetchMaxWait 250 ms.
  Metadata/admin operations have 5-second deadlines. These do not cap process RSS.
- Count raw bytes as topic/key/value/header-name/header-value bytes per record;
  preserve nil versus empty when copying. Stop before accepting a record that
  would exceed the batch. Keep that record's position unread. A record larger
  than a whole operation budget yields an explicit limit stop, not endless
  Continue. No automatic skip; Seek is the explicit route past it.
- UI retention: 2,000 records / 16 MiB raw content; oldest eviction is labelled.
  If eviction would remove selection, freeze before accepting that batch and
  keep the prior cursor so reopening does not lose undelivered records.
- Format only selected payloads: 256 KiB formatted output and depth 64 for JSON;
  larger/deeper content offers bounded raw/hex windows. No full-body formatting
  per row. Raw/hex windows and local find work incrementally on retained bytes.

### Invisible gaps: do not invent a scan cursor

In the pinned kgo source `FetchPartition` exposes records and watermarks, not a
public next-scanned offset. Group-only uncommitted-offset APIs cannot fix this
for direct assignment. Keep delivery positions at last accepted offset + 1 per
partition (or the original start if none accepted), not at the returned watermark.

A historical partition is proven complete when its start is already at its
captured end, an accepted record advances to the boundary, or a later visible
record at/after the boundary proves all earlier records have been traversed.
Do not display that later record in the historical window. When none of those
holds, the operation deadline returns “scan incomplete; invisible gaps possible,”
even if the only remaining offsets are aborted/control/compacted records.
No guessed fast-forward on an empty poll. Resume may conservatively rescan
invisible gaps, but cannot skip a visible record. This is a deliberate limitation
of the initial public-client implementation, not a complete-scan claim.

Use `ConsumeResetOffset(kgo.NoResetOffset())` so a retention race yields an
expired-position error rather than silently rewinding. Before reopening validate
topic identity and retained bounds; leader-epoch/truncation errors also stop the
partition instead of silently recovering to a different position.

## Call stack and state ownership

1. **Startup:** CLI validates flags → config/profile → Kafka backend or fixture →
   ui.New → Init → Topics command → generation-checked result → Topics view.
   Main defers Model.Close after New; session owner closes readers then backends
   on every exit path, including UI startup/runtime failure.
2. **Open/seek:** checked action → validate draft → invalidate old generation →
   serialized close-old/open-new command → reader result → first Next command →
   batch acceptance → selection/preview. Draft Cancel never opens a reader.
3. **Session ownership:** one app-local session owner holds the active reader,
   cancel function and a mutex for replacement/Close. Async commands refer to
   this owner, not copied Bubble Tea model fields. A late Open closes its returned
   reader before discarding it. Replacement waits for close; never two active
   record readers. Main can always close the owner after Program.Run returns.
4. **Batch:** Next → owned batch tagged with connection/read generation → Update
   verifies generation and active state → checks retention → accepts bytes and
   cursor atomically. Pause increments generation, cancels and closes. A stale
   batch may neither advance the saved delivery cursor nor append rows.
5. **Follow:** one completion schedules the next tick only while active. Pause
   cancels collection; Resume reopens at last accepted cursor. Follow-from-now
   and Jump-to-latest resolve new LSOs deliberately. Failed partitions are shown
   separately while successful ones can proceed; retry failures is explicit.
6. **Groups:** view action → bounded group list → select/Open → Group observation →
   raw offsets → app calculation and status. HWM 120, commit 95 means lag 25;
   no commit is “uncommitted,” not retained span. Commit above HWM or below start
   is inconsistent/expired, not zero. Totals are labelled partial when necessary.
7. **Inspector/copy:** select row → existing bytes → Inspect/field selection →
   explicit payload → native helper with deadline or trusted OSC52 → factual
   status. UI displays safe text; clipboard receives only explicitly selected
   bytes. Native helper completion and OSC52 transmission say receipt unverified.
8. **Preferences/connection:** draft → explicit Apply to session; Save preferences
   writes appearance only. Switching profiles cancels readers and catalog/group
   jobs, clears old payloads and increments connection generation. Profiles are
   edited in the explicit config file initially, not in a credential wizard.

UI geometry: minimum 60×16; list-only below 110 columns; list + inspector at 110;
three panes at 140. Enter opens a floating payload reader at all usable sizes,
using shared `SurfaceSpec`, `RenderSurface`, `HandleSurface` and `Overlay` capture.
Keyboard selection (not mouse hover) updates the right inspector immediately.
Compact one-line events are default; Comfortable uses bordered cards with two
content lines: partition/offset/time left, key/payload right. Its topic selector
is a header action strip rather than a persistent sidebar.
Density changes preserve selection/cursor and persist only on Save appearance.
Both layouts use record hit targets; a Comfortable card selects from its border
or either content line. Valid JSON is coloured after safe-text escaping; selected
Compact rows use a uniform blue highlight, while selected cards retain syntax
colours with a cyan outline. Esc closes the modal without
moving the selected event; Tab selects Value/Key/Headers and footer controls.
Topics/Groups use the central list; group details replace it with partition rows.
Tab traverses only current visible controls/panes; Enter opens the selected row;
Ctrl+P and F1 use shared capture. Printable shortcuts never steal text-input
keystrokes. All event content goes through safe output before terminal rendering.

## Test plan

Keep behavioral tests in owning files; avoid tests of private call order or
one test per helper. Integration tests use only an explicitly selected owned lab.

| Test/scenario | Observable assertion and competing mistake it catches |
| --- | --- |
| `TestModeAndProfileValidation` | Fixture/config ambiguity, unknown profile and insecure credential combinations fail; fixture performs no network access |
| `TestConfigAndSettingsPrivacy` | Duplicate/unknown keys, oversized configs and unsafe secret files refused; explicit Save round-trips only preferences |
| `TestReadPartitionPositions` | Partition 0 offsets 7,9 and partition 2 offsets 40,44 preserve independent continuation; an unaccepted byte-budget record appears on resume |
| `TestCommittedHistoryBoundaries` | Stable end 10 versus HWM 15 never waits for or displays uncommitted 10–14; a window ending in invisible gaps reports deadline/incomplete rather than false end |
| `TestReadLimitsAndOversizedRecord` | Inputs at and one beyond record/byte/partition caps differ; oversized record yields limit without cursor skip or retry loop |
| `TestPauseSeekAndStaleResults` | Held old Open/Next completed after Pause/topic switch cannot append, advance cursors or leave a live obsolete reader; Resume returns the next visible unread event |
| `TestRetentionAndTopicReplacement` | Old cursor below retained start and changed topic ID refuse; no automatic earliest/latest reset |
| `TestSelectionAndEviction` | Follow keeps selected identity; retention pressure pauses before selected-record loss; discarded batch remains readable |
| `TestExactRecordOutput` | Null/empty/binary, duplicate headers and ESC/OSC data retain exact raw bytes while display emits no source-controlled terminal sequences; JSON formatting never alters raw copy |
| `TestGroupObservationStates` | Lag 25, missing commit, commit beyond end, expired commit and denied partition remain distinct; total cannot hide incomplete partitions |
| `TestVisibleInteractionAndDrafts` | Row click selects, Inspect opens, Cancel preserves prior position, Help retains drafts, mouse release does not duplicate, hidden/undersized controls cannot act |
| Real broker contracts on both baselines | Committed/aborted/open transactions, null/empty/headers, seeks, ACL denials, TLS/mTLS/SASL and oversized batches behave as described; independently queried group commits unchanged before/after browsing |
| Real executable PTYs | Offline Topics → Events → reader → seek/cancel → Follow/Pause → Groups; mouse-on/off, NO_COLOR, compact/resize, hostile bytes and quitting |

Lab seeder creates asymmetric multi-partition samples, transactional cases and
group offsets with a separate credential. TLS/auth tests include wrong CA and
invalid credentials (no fallback). Observe real retained/compacted state before
asserting behavior. Resolve release artifacts/digests before lab manifests land.
Use targeted checks per slice, then `make check`, `./build`, `make test-terminal`.
Visual acceptance needs inspected actual-terminal screenshots and interaction
recordings, not this plan's HTML study. Native clipboard receipt remains a
separate manual check; no integration evidence exists yet.

## Least confident decisions

1. **Conservative gap completion:** safe but can label an actually exhausted
   compacted/transactional window incomplete. Prefer this to private kgo internals
   or a second low-level Kafka decoder. Revisit only with an approved public API
   or an explicitly revised architecture, not a guessed watermark shortcut.
2. **Opinionated caps:** 32 selected partitions, 4 MiB historical data and bounded
   JSON display should keep the viewer responsive but may be restrictive for the
   first real workload. Change based on measured representative data, not hidden
   auto-expansion. Remote catalog limits may require later scoped discovery.
3. **Connection setup:** explicit files and the approved TLS/SASL set keep scope
   small. Cloud-specific OAuth and schema decoding remain deliberate exclusions;
   needing either for the first cluster would require revisiting scope.

Safe handoff: Gate 3 is pending approval; no implementation or slice plan exists.

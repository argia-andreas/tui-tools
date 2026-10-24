# Program Design: Service Bus Explorer terminal UI

Gate 3 approved 2026-10-23. This document contains approved future contracts, not implemented application code. Application implementation follows a separately approved Gate 4 slice plan in `04-slices.md`.

## Gate 3 retry adjustment approved — 2026-10-24

The pinned SDK contradicts the approved assumption that `MaxRetries: -1`
disables every automatic send/settlement retry. Andreas explicitly approved
trusting SDK-internal retry/recovery on 2026-10-24, narrowing the contract to
**no application-level automatic retries**. No SDK patch is authorized or needed
for this decision; unrelated approved contracts remain unchanged.

Verified directly in azservicebus 1.11.0 source (not inferred from a failing
broker run):

- `messageSettler.go`, `settleWithRetries`: passes `RetryOptions{}` instead of
  the stored client options. Complete/Abandon can also fall through from the
  receiver to the management link after a settlement error.
- `internal/amqpLinks.go`, `AMQPLinksImpl.Retry`: on the first link error, calls
  `args.ResetAttempts()` for an immediate recovery attempt, even when configured
  with negative maximum retries. It invokes the operation again after recovery.

The draft app submits each operation once and treats transport errors as unknown,
but that does not establish a single wire attempt. Keeping the original/new ID
reduces some risks; it does not eliminate duplicate processing or make send and
cleanup atomic.

The application submits each send/settlement once and never automatically
re-invokes it or the full replay. SDK recovery/fallback may repeat wire attempts;
neither a single wire attempt nor exactly-once processing is promised. Unknown
send means **check destination before retrying**, retain the original and never
automatically clean up. Unknown settlement is not confirmed deletion; confirmed
send with failed/unknown cleanup remains a distinct partial result.

Current Slice 4 implementation: receive-next reviews the actual acquired message,
not the selected peek row; Cancel is the default, with no automatic lock renewal.
Resend creates a new copy at its original queue/topic with a new message ID;
cleanup defaults off and typed `delete` authorizes deletion. Session-enabled
mutation and advanced AMQP replay representations remain unsupported. Fixture
held deliveries, reviewed-action UI, late-acquisition release and terminal
outcomes, Azure conversion/acquisition/settlement and serial integration checks
are present. Race/vet/Python guard and fixture PTY checks pass; after fresh seed,
real broker integration and emulator PTY also pass (2026-10-24). The 90 KB body,
supported scalar properties, source/replay 37-minute TTL, new ID/correlation,
original retention, separate cleanup and topic fan-out were executed. Binary
application-property rejection is tested separately as an emulator limitation.
Final review/theme/compact/outcome renders and keyboard/pointer video were
inspected in the continuation thread on 2026-10-24. Fresh fixture/race/vet/guard
and serial seeded emulator integration/PTY checks also passed there. Unknown,
expired and partial outcomes use explicitly offline simulations; actual emulator
captures cover review, release, typed delete and topic warnings. Slice 4 is
complete; no cloud or MCP implementation is implied. See `00-status.md` for
commands, evidence and remaining validation boundaries.

## Boundaries and dependency baseline

- Independent module and binary under `service-bus-explorer/`, working binary name `sbx` (not a final product name). No imports of LazyFTP internals or changes to its files, root build/setup, lab or documentation.
- Go 1.27.3, matching the repository's current toolchain declaration; Bubble Tea 1.3.10, Lip Gloss 1.1.0, ANSI 0.10.2 and fuzzy 0.1.1. Pin Azure `azservicebus` 1.11.0, which requires Go 1.25+ and includes the `admin` package. A compiled Go/emulator path remains to be proved during implementation; the existing executable broker probe used JavaScript/.NET.
- Adapt semantic theme roles and terminal geometry techniques from LazyFTP revision `cbad916d52dda16fa8ca885e7fbfcefdd1a21b3c`; preserve source/license notices. Do not copy its FTP model or newline-stripping text helper into the message viewer. Start dark, warm and light presets; no shared toolkit or FTP migration now. Revisit extraction only after two working consumers show the same small contract.
- Initial connection-string credentials are session-only. Persist nonsecret preferences, not keys or bodies. OS-vault persistence and Microsoft sign-in are later increments, not plaintext fallbacks.

## Files

All paths below are relative to the future `service-bus-explorer/` directory. These are the planned implementation files, not files being created by this documentation change. Tests live beside their owning code; add no generic framework directories.

| File | Responsibility |
| --- | --- |
| `go.mod`, `go.sum` | Independent module and pinned dependency graph. |
| `README.md`, `Makefile`, `.gitignore` | This app's prerequisites, commands and ignored local state; no root changes. |
| `cmd/sbx/main.go` | Parse fixture/emulator/cloud selection, acquire masked credentials, run TUI, close resources. |
| `cmd/sbx-lab/main.go` | Local-only seed/send/peek/dead-letter/drain command entry point, never a cloud administration CLI. |
| `internal/explorer/explorer.go` | Resource identity, count/page snapshots and the two backend contracts below. |
| `internal/fixture/store.go`, `scenario.json`, `store_test.go` | Deterministic backend and shared seed specification; visibly offline, no network. Embed the scenario for both fixture and lab commands. |
| `internal/azure/store.go`, `delivery.go`, `emulator.go` | SDK discovery/counts/peek, held-lock mutation, and isolated emulator-only admin transport. |
| `internal/azure/store_integration_test.go`, `delivery_test.go`, `emulator_test.go` | Opt-in real broker contract checks; replay conversion and transport safety cases. |
| `internal/ui/model.go`, `view.go`, `actions.go`, `model_test.go`, `view_test.go` | Bubble Tea state/effects, cell-safe rendering, scoped palette actions and observable UI tests. |
| `internal/theme/theme.go`, `provenance.go`, `theme_test.go` | Semantic theme selection/loading and source notices; no FTP-specific roles. |
| `internal/config/config.go`, `config_test.go` | Versioned nonsecret profiles/watchlist/preferences, validated atomic save. |
| `lab/compose.yaml`, `config.json` | Pinned isolated emulator/SQL topology, 12 queues and a topic with two subscriptions. |
| `scripts/lab.py`, `lab_test.py` | Python-stdlib Compose lifecycle/readiness/reset and local-target guard tests; generate ignored local SQL credentials. |
| `scripts/terminal-smoke.py`, `requirements.txt` | Real PTY keyboard/resize checks with pinned pyte; adapt FTP's testing method, not its screens. |

The existing plan documents remain the design record. No application files are created before Gate 4 approval.

## Types & signatures

Illustrative Go declarations: signatures only, no method implementations. Azure SDK objects stay inside `internal/azure`; the fixture backend implements the same explorer contracts. Contexts bound network work and cancellation. Finite string states below become typed constants in implementation.

```go
type Resource struct {
    Profile, Namespace, Queue, Topic, Subscription string
}
// Queue xor Topic+Subscription is a receive target. Topic alone is a tree node.
// Full identity, not a display label, keys watchlists, requests and confirmations.

type Counts struct {
    Active, DeadLetter int64
    ObservedAt time.Time
    State string // available, stale, unavailable; unavailable never renders as 0
    Detail string
}

type Message struct {
    ID, Subject, ContentType string
    Sequence int64
    Body []byte
    Properties map[string]any
    EnqueuedAt time.Time
    DeliveryCount uint32
    DeadLetterReason, DeadLetterDescription string
    PreviewTruncated bool
}

type Page struct {
    Messages []Message
    NextSequence *int64 // explicit cursor, not a total or stable snapshot
}

type Store interface {
    Discover(context.Context) ([]Resource, error)
    Counts(context.Context, Resource) (Counts, error)
    Peek(ctx context.Context, target Resource, deadLetter bool,
        fromSequence int64, limit int) (Page, error)
    ReceiveNextDLQ(context.Context, Resource) (Delivery, error)
    Close(context.Context) error
}

type Delivery interface {
    Snapshot() Message
    LockedUntil() time.Time
    Resend(ctx context.Context, newMessageID string) (SendResult, error)
    Complete(context.Context) error
    Abandon(context.Context) error
}

type SendResult struct {
    State string // confirmed, rejected, unknown
    MessageID string
}
```

- A `Delivery` owns the actual SDK-received message and receiver, not a reconstructed peek snapshot. One held delivery at a time. No general-purpose arbitrary-sequence receive method, active-message mutations or bulk API in this first scope.
- `Resend` routes only to that delivery's original queue/topic; it cannot change destination or settle the original. `Complete` is a separate authorized effect. An error with `State=unknown` is not proof of rejection. Settlement transport timeouts also have unknown outcomes.
- UI owns a registry of actions with stable ID, title, resource scope and availability. Matching uses fuzzy search over available actions; no command-string execution or backend calls from rendering. Action submission produces a Bubble Tea command and a typed result message.
- UI state holds selected resource/subqueue, current page, focus, palette/review state, theme/preferences, count cache, request generation and optional held delivery. UI-only result messages carry generation and resource identity. No package-global mutable selection.
- Config API: `Load(path string) (Preferences, error)` and `Save(path string, value Preferences) error`. Preferences contain schema version, theme, expanded sidebar, inspector visibility, grid/table and named profiles/watchlist. Profiles contain namespace/entity paths and explicit emulator/cloud mode, never connection strings. Use `os.UserConfigDir` and `filepath`; reject unsupported versions rather than overwriting them.

## Read behavior and responsiveness

- Representative acceptance size: 160 columns × 48 rows. Expanded sidebar width 24; grid shows 3×4 cards, table at least 20 entity rows. Below the space requirement reduce columns and scroll, rather than truncate identities or pretend 12 fit. At 80×24 use one primary pane; tree/inspector remain togglable. Test ASCII fallback, Unicode cell width and all three themes.
- `Tab` cycles visible panes; arrows and `j/k` move within focused lists; Enter opens the selected resource; Escape closes the top overlay. Ctrl+P opens actions everywhere outside secret input. `[`/`]` select Active/DLQ on the entity screen. Text fields own printable keys. Theme/watchlist preferences live in a single Settings entry. Mutation actions are explicitly named “Receive next DLQ message…”; they are not applied to a highlighted peek row.
- Refresh watched counts every 30 seconds, manual refresh available; max four concurrent reads and no overlapping sweep. Retain successful values with timestamps; failure marks cached values stale. Off-screen watched entities can refresh in later bounded batches; mark pending/unobserved clearly. Topic rows do not invent backlog totals; subscriptions have counts. Known emulator 2.0.1 counters are unavailable, even if an SDK silently supplies zeros.
- Page size 20; explicit `FromSequenceNumber` on every peek. Refresh starts at the displayed page's start; Next starts after its last sequence. Keep only current page plus cursor history, not a namespace-wide body cache. A short/empty page describes that request, not a trustworthy aggregate queue count. Concurrent consumers may change results; navigation is not a snapshot.
- Display at most 64 KiB of any body with a truncation notice, preserving legitimate newlines while escaping terminal control sequences/OSC/ANSI from all broker-sourced text. This is a rendering limit, not a claim that the SDK avoids downloading larger bodies. Raw held-message content remains independent of display truncation.
- Read context deadline 10 seconds; receive-next deadline 5 seconds; mutation deadline up to 10 seconds, bounded by remaining lock time. Navigation cancels irrelevant reads and increments generation so late responses cannot replace a different resource. Quit cancels work, attempts bounded release and closes clients. Network work never blocks `Update` or `View`.

## Mutation contract and call stacks

### Browse and dashboard

`main → config + selected backend → UI Init → asynchronous Discover/Counts → generation-checked Update → View`.

`open queue/subscription → Peek(explicit cursor, Active/DLQ) → page result → selected preview → inspector`. This path never receives or settles messages.

### Acquire and review

`Ctrl+P → Receive next DLQ message → ReceiveNextDLQ(PeekLock, max=1) → review actual received identity + body + source/destination + lock deadline`.

This is the user's approved correction to peek-selected mutation. It may receive a different message from the highlighted preview. No receive-and-scan, arbitrary-sequence management workaround, receive-and-delete mode or background acquisition. Explain that receiving/releasing is not side-effect-free: abandonment/expiry can increase delivery count. Session-enabled entity mutation is initially disabled with an explanation, not silently handled with a non-session receiver.

Escape/Cancel invokes `Abandon` and shows confirmed release versus unknown release/expiry. There is no automatic lock renewal initially. Expiry disables Send/Delete, releases local review state and requires receiving/reviewing again; it never silently acquires a replacement. Warn when insufficient lock time remains. Mutation requests are single-flight, including repeated Enter/key-repeat.

Acquisition/review holds navigation until cancellation/release finishes. A receive result arriving after cancellation must still be released, not discarded as a stale read result. Settlement/release closes that delivery's receiver; quit attempts bounded release before client close. Once a mutation has started, cancellation cannot imply the broker rolled it back: preserve its eventual confirmed/unknown result.

### Resend

`review → explicit confirmation → Delivery.Resend → confirmed/rejected/unknown send result → optional separately authorized Complete → result summary`.

- Default review focus is Cancel. “Remove original after confirmed send” defaults **off** (proposal for approval). Topic replay warns that other matching subscriptions can receive copies.
- Generate/show a new replay message ID before confirmation, avoiding accidental suppression by original-ID duplicate detection. Preserve original correlation ID, supported application properties and standard single-Data-section body. Preserve TTL duration as a fresh enqueue lifetime, clear scheduled-enqueue time for immediate replay, and strip the broker DLQ reason/description properties from the outgoing copy. Show these changes in review. Do not edit business content or append surprise application properties.
- SDK `ReceivedMessage.Message()` is shallow and not lossless for arbitrary AMQP bodies/IDs. Clone mutable fields before the above changes; reject unsupported AMQP representations from resend rather than corrupting them. Browsing can still explain their unsupported format. No raw-envelope passthrough in this scope.
- Set messaging `RetryOptions.MaxRetries=-1`: zero means the SDK default of three retries, not disabled. This limits configurable retries but does not disable all SDK recovery/settlement paths. Per the Gate 3 adjustment approved 2026-10-24, accept SDK-internal retry/recovery without a patch and use bounded contexts. The application never automatically re-invokes a send/settlement or entire replay, including after a timeout. SDK recovery can repeat wire attempts; no single-wire-attempt or exactly-once guarantee.
- Only a confirmed send allows optional cleanup. An acknowledgement can be lost after broker acceptance: show **unknown—check destination before retrying**, retain the original, and never claim “not sent.” Confirmed send + failed/unknown cleanup is a distinct partial result; a later replay could duplicate processing. No atomic move or exactly-once promise.
- If retaining the original, explicitly release its lock when leaving the result screen. If release is unconfirmed, report that it may remain locked until expiry. Do not leave a successful-send review able to resend again.

### Delete

`acquire next → review actual identity → type delete to authorize → Complete → confirmed deletion or unknown/failed settlement`.

Selecting Delete in the palette cannot itself delete a peek-selected message. A held delivery after confirmed send may offer explicit cleanup without sending again. A terminal send result cannot be submitted again from that review.

## Connection and local lab commands

- Explicit `--fixture`, `--emulator` or `--profile <name>` selects one mode; never fall through between them. Cloud/emulator credentials come from a masked prompt or deliberately named `SBX_CONNECTION_STRING` environment input, never command-line arguments or saved config. Redact SDK errors/logs. Environment input is a convenience, not secret storage.
- Namespace administration and message permissions are independent. If discovery is forbidden, a manually specified entity can still be tried. Show per-operation access failures, not an empty namespace. No promise that SAS listen-only credentials can enumerate or fetch all counters.
- Emulator admin transport rewrites only the exact configured loopback management host/port to HTTP; reject redirects and non-loopback targets. Cloud mode never installs this transport. Test the signed-request/path behavior against the real Go client before relying on discovery. Emulator AMQP uses `UseDevelopmentEmulator=true` explicitly.
- Proposed project-local interface: `make run-fixture`, `make test`, `make terminal-test`, `make build`; `make lab-up`, `lab-status`, `lab-down`, `lab-reset`, `lab-seed`, `lab-send`, `lab-peek`, `lab-dead-letter`, `lab-drain`, `integration-test`. Entity/count options are arguments to the lab CLI, not arbitrary connection strings. All names are planned, not existing commands.
- `lab.py` fixes the Compose project name and loopback ports, checks the target's emulator health, and accepts no cloud profile/endpoint overrides. Destructive operations require the known disposable lab topology. Reset is explicit teardown/recreate to an empty broker; seed always performs that reset before inserting fixtures and states that it destroys existing local lab data. Do not infer cleanliness from unreliable counters or bounded peeks. Drain is bounded by a maximum and prints actual settled count, not an inferred total.
- Pin the tested emulator/SQL manifest digests from `local-development-notes.md`. Lab startup requires explicit EULA acceptance and generates local-only SQL credentials outside Git. Separate ports/project prevent affecting FTP/Azurite. Orb services use supervised `amp orb service`; do not start daemonized services with shell `&` or edit root orb setup.
- Seed 12 queues with asymmetric/empty active/DLQ states plus topic `events` with subscriptions `billing` and `shipping`; include text/JSON, long names, failure reasons and controlled invalid-terminal text. Seed expectations are assertions about inserted messages, not runtime counter fallback. Fixture mode additionally covers multiple namespaces, permission errors, stale/unavailable counts and large watchlists.
- Linux/macOS are app targets; Apple Silicon lab execution remains unverified and SQL Edge is retired. Keep that limitation visible. Cross-build Windows without introducing shell/path assumptions into core code; PTY harness itself is initially POSIX-only. No real-Azure writes or cloud test harness are authorized by this proposal.

## Test plan

Names are proposed contract tests, not claims of existing passing tests. Prefer extending tests by behavior rather than one per helper.

| Test | Observable assertion / plausible mistake it catches |
| --- | --- |
| `TestDashboardViews` | Same asymmetric 17-resource watchlist in grid/table; 12 cards fit at 160×48, table shows at least 20 rows with a larger fixture, remaining resources reachable; expanded/collapsed navigation and dark/warm/light states. Unknown is not zero and labels survive monochrome. |
| `TestBrowseIsNonDestructive` | Fixture and emulator: exact IDs/bodies remain receivable after browsing; refreshing a page doesn't silently advance its start. Nonconsecutive sequences distinguish explicit paging from index arithmetic. |
| `TestLateReadCannotReplaceSelection` | Delayed resource A reply cannot replace selected B, including same entity's opposite subqueue. Keyboard navigation remains responsive during a pending request. |
| `TestCountStates` | Known zero, unavailable and stale nonzero render differently; subscription resources stay distinct; emulator mode never renders misleading SDK zero counters. |
| `TestUntrustedMessageText` | ESC/OSC/control payloads cannot change terminal state; multiline JSON remains legible, Unicode width fits, truncation is explicit. |
| `TestReviewUsesAcquiredMessage` | Peek shows A but receive supplies B: review/action identifies B; cancellation/expiry never sends/completes A or silently replaces B. |
| `TestReplayOutcomes` | Confirmed send with retained original; confirmed send with cleanup failure; ambiguous send with original retained; repeated submission does not create another send. Assertions target resulting messages/state, not internal call order. |
| `TestReplayConversion` | Independent expected body/properties, new ID, preserved correlation/TTL, cleared schedule/DLQ fields; preview truncation cannot truncate outgoing data; unsupported AMQP body/ID is refused. |
| `TestDeleteReview` | Palette/Enter alone cannot delete; exact held identity plus explicit delete authorization required; lock loss and unknown settlement are not reported as success. |
| `TestConfigurationAndCredentials` | Round-trip preferences, invalid-version handling, same-named queues in different profiles, no secret/body in disk output or rendered errors; session-only credential entry. |
| `TestEmulatorTransportBoundary` | Only exact loopback emulator destination can downgrade; cloud, alternate hosts and redirects rejected. Lab cloud-target guard tested separately in `lab_test.py`. |
| `TestSeededEmulatorContract` | Serial reset/seed, discovery of queues/subscriptions, body/reason preservation, receive-next/abandon, send-copy and separate cleanup, replay reaches both topic subscriptions, bounded drain and repeat reset. Counters explicitly unavailable, not a skipped correctness claim. |

Real PTY smoke drives the binary: fixture banner, grid/table, sidebar expansion, Ctrl+P fuzzy selection, Escape/focus return, Settings/theme choice, Active/DLQ and inspector, 160×48 and 80×24 resize, cancellation and risky-action review. Capture and inspect actual terminal renders/interaction recordings when implementation exists. Read-only code assertions or Painter mockups do not substitute for that run.

Opt-in cloud validation remains a separately authorized later activity for authoritative counters, duplicate detection/locks, Entra and role restrictions. Build checks cover Linux/macOS targets and Windows cross-compilation; no claim of native macOS/Windows execution from a Linux build.

## Least confident decisions — challenge before implementation

1. **Replay defaults:** retain original by default, new MessageID, immediate enqueue and renewed original TTL duration. These are visible proposals, not previously agreed product decisions. They favor avoiding unintended deletion/dedup suppression, but change replay identity/lifetime.
2. **Initial mutation scope:** only receive-next, single-message, non-session standard-body DLQ operations. This is deliberately narrower than a mature explorer; session receive and advanced AMQP replay require additional design, not optimistic conversion.
3. **Layout target:** 160×48 for 12 expanded-sidebar cards. Smaller terminals degrade to fewer visible cards; tune against a real PTY before claiming usability. Default count interval 30 seconds is a starting point, not an Azure cost/performance guarantee.
4. **Go emulator administration:** official source confirms the HTTPS construction and available transport hook, not a tested Go integration. If the isolated transport cannot work correctly, revisit this narrow adapter decision rather than introduce a broker bridge silently. Accurate emulator counters remain unavailable regardless.
5. **Apple Silicon SQL dependency:** the current reproducible pin uses retired SQL Edge and an older ARM child. Validate on the user's Mac before promising parity; do not substitute a new SQL pairing without integration checks.

## SDK research references

Read-only source inspection pinned Azure SDK commit `7792debc7e87e22baca89191fde85d1f334fb532` (azservicebus 1.11.0), not a broker execution proof:

- [Peek, receive and deferred receive contracts](https://github.com/Azure/azure-sdk-for-go/blob/7792debc7e87e22baca89191fde85d1f334fb532/sdk/messaging/azservicebus/receiver.go).
- [Message conversion and AMQP representation](https://github.com/Azure/azure-sdk-for-go/blob/7792debc7e87e22baca89191fde85d1f334fb532/sdk/messaging/azservicebus/message.go).
- [Retry defaults: negative disables; zero selects defaults](https://github.com/Azure/azure-sdk-for-go/blob/7792debc7e87e22baca89191fde85d1f334fb532/sdk/messaging/azservicebus/internal/utils/retrier.go).
- [Admin entity manager constructs HTTPS host](https://github.com/Azure/azure-sdk-for-go/blob/7792debc7e87e22baca89191fde85d1f334fb532/sdk/messaging/azservicebus/internal/atom/entity_manager.go).

## Selected-message resend amendment — Gate 3 pending, 2026-10-24

Gate 2 amendment approved. This section proposes only the selected-message
extension; it supersedes origin-only routing for this flow, not the legacy
receive-next action or topic replay. No extension implementation exists yet.

### Files and contract changes

- `internal/explorer/explorer.go`: add a prepared, single-use replay contract;
  held deliveries add settlement. No SDK objects or raw bodies in UI state.
- `internal/azure/store.go`, `delivery.go`: exact-sequence read-only preparation,
  bounded selected acquisition, shared existing replay conversion and validated
  destination routing. No second conversion path or SDK patch.
- `internal/fixture/store.go`: same selected preparation/acquisition/routing
  semantics against full fixture messages, not display previews.
- `internal/ui/model.go`, `actions.go`, `review.go`, `review_view.go`, `view.go`:
  selected-message open, destination picker and cleanup configuration, using the
  existing compact controls and asynchronous review lifecycle.
- Existing adjacent Azure/fixture/UI tests and `scripts/terminal-smoke.py`:
  contract regressions, actual keyboard flow and serial broker coverage.
- App `README.md` and these plans: commands, side effects and verified limits.

Proposed additions/changes (other existing methods stay):

```go
type Replay interface {
    Snapshot() Message // display-limited copy, never the outbound body source
    Resend(ctx context.Context, destination Resource, newMessageID string) (SendResult, error)
}
type Delivery interface {
    Replay
    LockedUntil() time.Time
    Complete(context.Context) error
    Abandon(context.Context) error
}
// Added to Store; source must identify a queue/subscription DLQ.
PrepareDLQCopy(ctx context.Context, source Resource, sequence int64) (Replay, error)
ReceiveSelectedDLQ(ctx context.Context, source Resource, sequence int64) (Delivery, error)
```

A prepared Replay retains full raw message content privately, without a receiver
or lock, and allows one application send attempt. Prepare re-peeks exactly the
selected sequence; a higher returned sequence is missing, not a replacement.
Full resource identity + sequence identifies the source; UI also verifies the
returned ID against the selected row. Reuse existing raw AMQP validation, byte/
property copying, renewed TTL, new-ID and unknown-outcome rules. Validate source
session restrictions and destination queue existence/session restrictions before
send. Picker lists existing queues in the connected profile/namespace, including
unwatched ones; no arbitrary endpoint/path input. Legacy replay explicitly passes
its origin Resource, including a topic for subscription replay.

### User flow and runtime order

1. Enter opens the selected DLQ message read-only; Ctrl+P also exposes “Send
   selected DLQ message to queue…”. No selected row means no such action.
2. Choose destination from a keyboard-filterable queue list, then configure
   “Delete original after confirmed resend” (unchecked). No destination silently
   chosen. Back/cancel never sends. Existing receive-next is still clearly named.
3. Proceed to final review: unchecked prepares the exact peeked copy without
   acquiring a lock; checked acquires the exact selected original via the bounded
   search below. Review shows source entity/ID/sequence, destination, full-content
   display truncation notice, cleanup choice, and scan side effects if applicable.
4. Final review defaults to Cancel. Destination/cleanup are frozen; Back releases
   any held message before editing. Existing Tab/arrows/Enter controls apply.
   Read-only body pages cannot submit hidden confirmations. Only locked reviews
   show a lock deadline/timer; copy-only must not pretend it holds the original.
5. Confirm invokes Resend once; only confirmed send plus checked cleanup invokes
   Complete. Unknown send never completes. Confirmed send plus failed/unknown
   Complete is partial. Cancellation/quit/late replies account for held locks and
   pending outcomes. Refresh affected source/destination views, not fake counts.

### Selected acquisition bounds and cleanup

- One acquisition/held delivery at a time. Non-session PeekLock DLQ receiver,
  prefetch disabled. At most 100 individual receives and five seconds of search,
  whichever ends first; an earlier caller cancellation also stops it. No automatic
  restart, scan continuation, deferral or ReceiveAndDelete.
- Retain nonmatching locks until the scan ends so the same message is not
  abandoned and reacquired repeatedly. Match source/sequence exactly. Other
  locked messages may be skipped by the broker; a bound/timeout is “could not
  acquire selected message,” not proof of absence or emptiness.
- Attempt each nonmatch Abandon once within a shared two-second cleanup context
  independent of caller cancellation. On any unconfirmed release, abort before
  send; release the target too, close the receiver with a separate two-second
  bound, and report release uncertainty. Never claim all locks were released
  merely because Close returned. Lock expiry may be needed after uncertain cleanup.
- On success, only the selected lock remains owned by Delivery. On failure,
  cancel, expiry or late response, retain ownership until bounded cleanup has
  been attempted. No silent lock renewal; expiration disables send/Complete.
- Search can take up to five seconds plus bounded release/close work. Selected
  messages beyond the 100-receive/time budget require explicit user action;
  the UI must explain the limit rather than promise arbitrary-depth access.

### Test plan before implementation

- `TestSelectedDLQCopyPreservesFullMessage`: select the second of asymmetric
  messages, copy a 90 KB binary body/scalar properties to a different queue;
  independently assert complete bytes/TTL/new ID and both originals retained.
- `TestSelectedDLQCleanupTargetsOnlySelection`: acquire a non-first selected
  message, send to a different queue, remove only that original; independently
  receive untouched neighbors and assert they remain available (delivery count
  may increase). Include source subscription to destination queue, not topic fanout.
- `TestSelectedDLQCannotSubstituteNext`: selected sequence missing, ID mismatch,
  concurrently locked target, 100-message bound, deadline/cancel and expired lock
  produce no send or completion of another message. Deterministic fixture/fake
  clocks cover boundaries; serial emulator verifies non-first acquisition.
- `TestSelectedDLQUncertainEffects`: accepted send/lost acknowledgement retains
  original; confirmed send/uncertain Complete reports partial; uncertain nonmatch
  release prevents send. Repeat Enter and late cancel cannot send twice or leak
  ownership. Assert public outcomes/content, not mock call ordering.
- `TestSelectedDLQDestinationValidation`: same-name resource identities, missing
  destination, cross-profile/namespace or injected paths, sessions and unsupported
  AMQP data fail without sending. Legacy origin queue/topic replay still works.
- Real PTY: select a non-first row, open, filter/choose destination, unchecked
  default and checked cleanup flows, Back/Escape, resize/read-only body page;
  inspect all themes/80×24 and a recorded keyboard/pointer tour. Race/vet/guards
  plus fresh-seed serial broker integration remain required. No cloud proof.

### Least confident decisions

1. Five seconds/100 messages deliberately favors bounded disruption over finding
   a deeply queued selection. Raise limits only after measured lab evidence and
   user agreement, never silently scan the whole DLQ.
2. Treating uncertain nonmatch release as a pre-send failure is conservative;
   it avoids creating a replay while unrelated lock cleanup is unresolved.
3. Preparing a copy re-peeks the selected identity instead of trusting a cached,
   truncated display snapshot. If the original disappeared, the operation stops.

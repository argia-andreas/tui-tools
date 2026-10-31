# Vertical slices: streams

Gate 4 APPROVED 2026-10-30. Andreas explicitly requested “NO slice approval.
Build the thing!” Implement through the slices without approval stops, recording
milestone evidence in 00-status.md. No push, deployment or publication is
authorized. Do not build all backends, then all screens: every slice ends in a
runnable user workflow.

## Build order

1. **Offline tracer:** launch streams → select topic → open events → inspect a
   fictional event, using shared Actions/Help and mouse/keyboard controls.
2. **Real local Redpanda:** start isolated broker → seed sample topic → discover →
   bounded recent read → inspect actual broker-supplied bytes.
3. **Precise browsing:** seek by partition/offset/time or earliest → read to
   explicit limits → Continue or recover visibly from expired/changed positions.
4. **Live investigation:** Follow → Pause → inspect → Resume at unread positions,
   with bounded memory and safe topic/connection replacement.
5. **Content investigation:** find within loaded events → inspect JSON/raw/hex
   and duplicate headers → copy selected content → explicitly save preferences.
6. **Consumer-group investigation:** choose group → inspect partition positions
   and lag → distinguish missing commits, partial permissions and stale snapshots.
7. **Compatibility and security acceptance:** run the same real workflows on
   Apache Kafka and authenticated/TLS profiles; prove read-only behavior and suite
   conformance before describing streams as production-ready.

## Local Kafka-compatible event hub

The user's testing request means a **real isolated Kafka-protocol broker**, not
an in-memory imitation and not an Azure Event Hubs emulator. Andreas selected
Redpanda as the primary local lab. Use Redpanda v26.2.5 first and Apache Kafka
4.3.2 as the second tested implementation.
Verify release artifacts/digests before committing runtime definitions; if a pin
cannot be obtained, resolve it explicitly rather than silently using latest.

The orb currently has neither Java nor a running Docker daemon. In slice 2,
first inspect supported runtime options. Prefer isolated Docker Compose when a
working local runtime can be established safely; otherwise investigate supported
native Redpanda execution with verified pinned artifacts under an orb-managed
service. Do not silently substitute Kafka or fixtures if Redpanda cannot run.
Report the specific runtime blocker. Slice 7 also requires a viable Kafka runtime;
a verified Kafka distribution plus supported pinned JRE is an alternative to
containers there. No runtime installation has been attempted in this planning gate.

Lab requirements:

- Dedicated app-local private `.lab/` state, checked ownership marker and fixed
  local endpoints. Refuse remote Docker contexts or remote broker configuration
  in lab mutation commands. Refuse unknown existing state/port collisions.
- Explicit `lab-install`, `lab-up`, `lab-seed`, `lab-check`, `lab-down` commands.
  Startup, seeding and data deletion are never implicit in builds or checks.
  Down stops only owned services and preserves data. Reset, if added, requires
  an explicit command and verified disposable ownership.
- Loopback-only broker listeners and correctly reachable advertised addresses;
  distinct broker/controller ports and no accidentally public administration.
  Orb processes use `amp orb service start`, not detached CLI children.
- Seed with a separate lab-only admin/writer, never viewer code. A three-partition
  orders topic has deliberately different offsets and timestamps; add null/empty,
  binary, duplicate headers and hostile terminal text. Add committed/aborted/open
  transactions, then group positions and limited-permission topics as slices
  require them. Small deterministic data, not a load generator.
- Plaintext/anonymous is acceptable only for the explicitly labelled initial
  loopback development lab. By slice 7, use separate broker-enforced read-only
  viewer credentials, TLS/SASL and negative authorization tests. Do not claim
  broker-enforced safety from the initial anonymous lab.
- Never print generated passwords, private keys or connection strings. Keep
  transient payload captures out of diagnostics; use only fictional test data.
- No cloud account or production connection is needed for any slice acceptance.

## Slice acceptance

### 1 — Offline executable

Create the independent module, CLI `--fixture`, minimal fixture/backend contracts
and visible topic/events/inspector flow. Integrate root build/workspace/checks.
Reuse terminalui controls, Help, palette and select-first navigation immediately;
do not draw lookalike private controls for later replacement. Disable later
capabilities with truthful reasons; no controls pretending to contact a broker.
Initial fixture has enough asymmetric records to verify selection and identity.

Evidence: independent module vet/race tests, root build/check, actual executable
PTYs with mouse on/off, inspected wide/compact screenshots and a selection →
Inspect → Help → return recording. Existing apps remain runnable. This slice
proves UI wiring only, not Kafka support.

### 2 — Actual topic read

Establish the local lab, resolve artifact pins, seed through the separate tool,
and add explicit profile/config support plus the Kafka adapter. Run streams
against the local broker: discover partitions, Open one topic, fetch a bounded
recent window and inspect an actual record. Owned byte copies, sanitized terminal
output, committed isolation, bounded transport/decompression and close-on-exit
are required from the first real read, not postponed to hardening.

Evidence: lab health plus actual-binary discovery/read/inspection; compare selected
partition/offset/key/value against an independent lab reader. An open transaction
must not leak into the viewer, and missing topics must not be auto-created.
Capture statuses identifying the real local broker rather than fixture mode.

### 3 — Positions and bounded continuation

Wire seek/partition selection and Continue through the editor into the real
reader. Cover earliest, recent, exact offset and timestamp, with stable historical
ends, batch/deadline limits, partial failures and no silent resets. Test records
at both sides of each boundary and retain a byte-budget-rejected record for the
next read. Empty/invisible gaps may end as explicitly incomplete, never false EOF.
Reject over-32 partition fan-out and oversized records with actionable statuses.

Evidence: same real broker scenarios before/after fixes where applicable; exact
partition positions independently checked. Observe actual retention before
testing expired cursors. Inspect valid/invalid seek, partial/empty/incomplete and
compact states. Draft Cancel must leave the existing read position unchanged.

### 4 — Follow lifecycle

Add Follow-from-now, Pause, Resume and explicit Jump-to-latest; bounded UI
retention; selection anchoring; profile switching and cancellation-safe resource
ownership. Apply generation checks to metadata and group jobs as they are added,
not just payloads. Verify stale Open results are closed and stale batches rejected.

Evidence: seed events while paused, resume and see all next visible unread events
within the displayed catch-up bounds. Verify no viewer offset mutation and no
new scheduled fetches after Pause/close. Held-command tests exercise otherwise
hard-to-time stale completions; real broker checks verify the normal lifecycle.
Record Follow → Pause → inspect → Resume plus selection-preserving retention.

### 5 — Exact inspection and preferences

Finish JSON/raw/hex windows, ordered header inspection, field/raw copy, literal
loaded-window find and appearance settings. Format only selected data with the
approved size/depth caps. Native copy/OSC52 statuses describe transport outcomes,
not unverified clipboard receipt. Profiles remain file-managed; no secret editor.

Evidence: public output/UI tests for null versus empty, invalid UTF-8, duplicate
headers and escape sequences; exact selected copy bytes using a controlled sink.
PTY find/reader/settings/save paths, compact states and explicit Apply/Cancel.
Inspect reader modes and record field selection/copy feedback without displaying
any real sensitive content. No remote or cross-topic search is implied.

### 6 — Honest groups and lag

Seed dedicated consumer-group offsets with the lab tool, then wire group list →
partition detail → explicit refresh. Derive lag from raw commits/HWMs; retain
observation times, absent commits, expired/inconsistent positions, authorization
failures and partial-total status. Do not join groups or configure autocommit.

Evidence: independent raw offset observations match displayed values on an
asymmetric fixture (including HWM 120 / commit 95 → lag 25). Group commits before
and after browsing remain unchanged. Cover idle groups and missing permissions;
inspect default, unavailable and compact group detail states.

### 7 — Compatibility, credentials and final acceptance

Run the real broker contract suite against the pinned Apache Kafka baseline as
well as the primary Redpanda lab. Exercise verified TLS, mTLS, PLAIN and both SCRAM variants with explicit
profiles, negative trust/auth cases and read-only ACLs. Read-only viewer credentials
must be denied writes/admin actions through an independent test client, while
viewer browsing works. Seeder credentials must never enter the viewer process.

Prove transactions, seeks, compaction/retention, limits and pause/resume on both
brokers; report any unsupported difference rather than marking the matrix green.
Verify original consumer commits independently and include topic noncreation.
Inspect actual terminal wide/compact/resize, palette, reader and error states;
mouse/keyboard parity, ASCII/NO_COLOR and clipboard transport limits stay explicit.

Final evidence: independent module race/vet tests; `make check`; `./build`;
`make test-terminal`; opt-in real broker suites with versions and outcomes;
inspected screenshots/recordings; startup/lab instructions; compatibility matrix.
No cloud/native-terminal/human usability acceptance claim without those checks.

## Reporting and handoff

After each slice, record commands, decisive results, visual evidence and limits
in the status document (or a linked validation document when it grows). Keep
fixture, real-broker and manual results distinct. Remove transient debug helpers;
retain tests of observable contracts. Do not amend an approved earlier gate
silently if runtime evidence contradicts it: describe the issue and re-approve.

Report the exact delivery state (local, committed, pushed, etc.) at completion.
The user's latest direction supersedes per-slice approval questions. Implementation
progress and actual lab state belong in 00-status.md and validation notes.

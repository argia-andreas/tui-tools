# Log viewer exploration

- Gate 1 — Product: APPROVED 2026-10-23 by Andreas (“Yes, this sounds good”).
- Gate 2 — Architecture: APPROVED 2026-10-23 by Andreas (“Yeah. That sounds good. Let's go for it.”).
- Gate 3 — Program Design: APPROVED 2026-10-24 by Andreas (“Sounds good. Go ahead.”).
- Gate 4 — Slice plan: APPROVED 2026-10-24 by Andreas (“Go for it”).

## Scope

Slices 1–2 implement the independent fixture/local Elastic workbench under `log-viewer/`.
No LazyFTP/Service Bus code, docs or root setup changes. Midnight semantic colors
and shared layout/focus conventions are reused without cross-module internal
imports or changes to the existing apps. Local private lab startup is authorized;
no deployment or publishing.
SIGNAL is a provisional mockup label, not an approved product name.

## Slices

- [x] Slice 1 — fixture workbench tracer bullet, verified 2026-10-24.
- [x] Slice 2 — real local Elasticsearch happy path, verified 2026-10-24.
- [ ] Slice 3 — app/time/level/message search, history and saved searches.
- [ ] Slice 4 — field yank, inspector and command palette.
- [ ] Slice 5 — context, trace and follow/pause/resume.
- [ ] Slice 6 — default-deny read-only MCP.

## Slice 1 result and evidence

- `make -C log-viewer run-fixture`: real Bubble Tea terminal app, 22 deterministic
  fictional events, chronological list and scrollable right inspector. Multiline
  messages/stacks and typed native fields survive the service boundary. Missing
  severity is UNKNOWN, not INFO. Only the focused panel owns bright selection.
- Explicit fixture mode, fixed historical UTC window; no network, credential
  loading, clipboard, persistence or MCP server. Later actions explain unavailable;
  sidebar provider names are labeled planned. This fake is not an Elastic emulator.
- Only the used Search subset of the target provider contract exists. The service
  enforces one explicit source/output policy, configured path/pattern redaction,
  source read budgets, cancellation and sanitized errors before UI retention.
  Cryptographically random page handles have no event-lookup API yet. Source
  registry, grant authorization, sessions/cursors and MCP remain later slices.
- Configured redaction is not general secret detection. It applies before and
  after terminal cleanup: adversarial seeds initially failed because removing
  escape/format characters reconstructed a secret. The corrected boundary passes.
  All Unicode format characters are conservatively removed, including zero-width
  joiners. Unsafe field keys are dropped rather than renamed into path collisions.
- Reads cap events at 500, retained field data at 32 MiB and encoded events at
  1 MiB. Budget-limited pages disclose partial results; oversized/invalid events
  fail closed. The finite embedded fake does not prove bounded HTTP decoding,
  real Elastic semantics, production redaction policy or whole-process memory use.
- `make -C log-viewer check test-terminal`: PASS (format check, vet, race tests,
  real executable PTY). Decisive output: `PASS: fixture PTY — explicit mode, 22 safe
  events, multiline inspector, focus, navigation, resize recovery, unavailable
  actions, clean exit`. Source/byte/time bounds, typed fields and safe failures
  pass public service tests. UI bounds test covers 7 widths × 3 heights, including
  panel breakpoints, with long multiline messages and hostile multiline metadata.
- `go -C log-viewer test ./internal/logs -run '^$' -fuzz FuzzSafeOutput -fuzztime 15s -parallel 2`:
  PASS, 71,504 executions. Tests include canaries in messages, nested arrays and
  stack traces; field allowlisting; ANSI/OSC52/hyperlink/bidi exclusion; large
  integers/null/booleans; and the cleanup/redaction ordering regression.
- Inspected actual 2× terminal screenshots: [160×48 default](../../../.amp/in/artifacts/log-viewer/slice1/01-workbench.png),
  [inspector focus/scroll](../../../.amp/in/artifacts/log-viewer/slice1/02-inspector.png),
  [80×24 logs](../../../.amp/in/artifacts/log-viewer/slice1/03-narrow-logs.png),
  [80×24 full-width inspector](../../../.amp/in/artifacts/log-viewer/slice1/04-narrow-inspector.png).
  Screenshots are real application renders, unlike the earlier Painter studies.
- Inspected [13.2-second keyboard/pointer clip](../../../.amp/in/artifacts/log-viewer/slice1/05-keyboard.mp4)
  and its 5-fps contact sheet: neighboring log selection, Enter → inspector,
  G → multiline stack, honest unavailable-yank feedback, Esc → log focus,
  honest unavailable-palette feedback. No successful copy or palette behavior is
  claimed. The PTY independently checks the resulting states.

### Rendering method

Pinned ttyd 1.7.7 (the same SHA-256 as `.agents/setup`) ran the actual binary under
a supervised private loopback preview, inspected with agent-browser at DPR 2.
Its old WebGL renderer had a Chromium/DPR capture defect; disposing that addon
selected xterm's DOM fallback. Reapplying font size recalculated character metrics.
This changed the preview renderer, not Go application code. Wide viewport was
1365×884 CSS pixels (160×48 cells); narrow was 691×452 (80×24). Waits must read
`buffer.active.viewportY + row`, not row zero: xterm can retain an alternate-buffer
base offset after resize. Do not mistake old buffer lines for the visible screen.
The using-agent-browser action-video helper drove real keyboard and moving-pointer
input. A temporary copy changed only capture rate from 30 to 10 fps after the
2× encoder fell behind. Final MP4/loop and screenshots are retained; no preview
helpers or browser-renderer changes enter the application. Raw capture is retained
locally but not tracked. The preview/browser are stopped after verification.

## Notes for a fresh session

- Read all documents in this folder before continuing.
- Andreas likes the right-hand event inspector and requests field-level `y` yank,
  excellent keyboard ergonomics, and a provider abstraction.
- The current source is NOT identified. Andreas recalls a hostname resembling
  `logs.insights…` and suspects Elastic. That substring is insufficient evidence.
  Ask for only a sanitized hostname or visible product name, never credentials.
- Approved product direction: stream workbench, with focus-reader and context
  modes rather than three separate products. Architecture and keyboard contract
  are approved in Gates 2–3; SIGNAL remains an unapproved name.
- Andreas requested a fast local Elastic development/test loop. Recommend a fake
  provider for instant UI tests plus a real, pinned single-node Elasticsearch for
  adapter/security integration tests. See provider-research.md and Slice 2 below.
- On Andreas's request to continue, pulled main before preparing Gate 2. Approved
  architecture: independent log-viewer module, shared policy-safe investigation path,
  capability-aware adapters, stdio-only/default-deny MCP and a standalone native
  Elasticsearch lab in orbs (Docker equivalent on hosts with a daemon).
- Gate 3 pins the local native lab to Elasticsearch 9.5.5 with the official published
  SHA-512, proposes logview/logview-mcp executable names, concrete contracts and
  behavioral tests. Archive bytes/startup are now verified by Slice 2; clipboard
  receipt is NOT verified. Docker image digest remains a prerequisite before
  optional Docker support.
- Gate 4 is approved; Slices 1–2 are implemented and verified. Stop at this checkpoint
  and ask “Continue to slice 3, or re-steer?” Read 04-slices.md for acceptance and
  delivery rules. Keep subsequent work/questions here; do not create another thread
  unless Andreas asks. Scoped verified commits/pushes to main are authorized.
- Painter boards 01–04 under `.amp/in/artifacts/log-viewer/` remain static fictional
  references; actual terminal evidence is separately under `slice1/` and `slice2/`.
- Product and research notes take precedence over generated labels and shortcuts.
- The requested Painter exploration substitutes for HTML screen sketches at this
  product gate; the subsequent Slice 1 is now an actual executable.
- Do not edit FTP or Service Bus code/docs. Their current work is independent.
- Andreas requested a fresh thread/orb after Slice 1 to use the updated RAM default.
  Slice 2 was authorized here on 2026-10-24 (“Yes, go ahead. Keep shipping features.
  Parallize work where you can.”). The new orb has about 8 GB RAM. Lab implementation
  ran independently in a subagent while the main agent implemented the adapter/UI;
  a separate read-only security review found no important issues under the trusted
  same-UID/repository model. Originating design/implementation history:
  https://ampcode.com/threads/T-01a16ae8-e353-76b7-ae0c-8fe1908f1687

## Slice 2 result and evidence

- `make -C log-viewer lab-install lab-up lab-seed lab-check`: PASS. Downloaded
  official 9.5.5 bundled-JDK archive matched the committed SHA-512 before safe
  extraction (75.97 seconds initial install). Non-root uid 1000, descriptor soft
  limit raised from 1024 to 65535 within hard limit 1048576; available host/cgroup
  RAM checked. No privileged sysctl, Docker daemon or public portal.
- Authenticated TLS readiness: 18.71 seconds initially, Java RSS 872.58 MiB;
  warm read/check RSS 910.56 MiB. Stop/restart was exercised through `lab-down
  lab-up lab-check`: readiness 20.82 seconds, RSS 879.45 MiB. These are observations,
  not whole-cgroup ceilings or load benchmarks. Heap stays 512 MiB, mmap/ML off,
  license Basic. HTTP and transport bind loopback ports 19200/19300 only.
- Private ownership-checked `.lab/` holds CA/server keys, separate provisioning
  and read-only API keys, fixed source config and node data. Viewer key cannot
  write the owned target or read the forbidden target. Decisive check output:
  `TLS verified; read=22; owned_write=403; forbidden_read=403`. Repeated seeding
  overwrites fixed IDs without adding events or deleting indices. Lab tools accept
  no endpoint/index input. `lab-down` preserves private state, not a reset.
- `--config .lab/source.json` uses the real HTTPS adapter → same safe service →
  existing log list/inspector. Strict bounded config rejects duplicates, unknown
  fields and trailing values. This slice intentionally allows only the exact local
  lab source/origin/target and fixed ECS mapping: fictional-data policy must not
  accidentally be used on production. Native structured `fields` are retained
  unindexed; no arbitrary field search/schema discovery is claimed.
- HTTP responses cap at 8 MiB, encoded events at 1 MiB, request timeout 15 seconds;
  CA verification stays on, automatic decompression off, redirects refused. Backend
  bodies/credentials do not enter errors. Timeout/shard failures fail closed.
  More results/budget limits disclose partial results; only one newest bounded
  window is fetched. Stable PIT history/equal-time pagination waits for Slice 3.
- `make -C log-viewer check test-terminal test-integration test-terminal-lab`: PASS
  (format, vet, race, existing fixture PTY, actual TLS lab integration and lab PTY).
  Integration verifies 22 safe events, multiline stack, UNKNOWN, canary exclusion,
  typed values and real half-open time filtering. Lab PTY decisively prints:
  `PASS: local Elastic TLS PTY — explicit mode, 22 safe events, multiline inspector,
  focus, navigation, resize recovery, unavailable actions, clean exit`.
- TLS transport tests cover bounded/invalid/oversized responses, 403, timeout/shard
  failures, cancellation, redirect credential exclusion, plaintext rejection and
  exact large integers. Lab harness validation also exercised checksum mismatch
  before extraction and private-state symlink rejection. Real integration is
  explicitly opted in via Makefile; ordinary checks require no running node.
- Ran the `run-lab` executable path in real PTYs and private ttyd preview. Inspected
  final 2× renders: [160×48 logs](../../../.amp/in/artifacts/log-viewer/slice2/01-local-elastic.png),
  [inspector/stack](../../../.amp/in/artifacts/log-viewer/slice2/02-inspector.png),
  [80×24 logs](../../../.amp/in/artifacts/log-viewer/slice2/03-narrow.png),
  [80×24 inspector](../../../.amp/in/artifacts/log-viewer/slice2/04-narrow-inspector.png).
  Local TLS mode is distinct from offline fixtures; only the focused panel is bright.
- Inspected [9.3-second keyboard/pointer clip](../../../.amp/in/artifacts/log-viewer/slice2/05-local-elastic.mp4)
  at 5 fps: neighboring rows, j/k, Enter inspector, G stack, honest unavailable-yank,
  Esc logs. Same ttyd/xterm DOM fallback method as Slice 1. Initial preview omitted
  Midnight background and showed gray default-background blocks; fixed preview
  theme and recaptured, with no application change. Preview/browser stopped after
  verification; warm private Elastic service remains available for the next slice.
- `y/Y`, filters, Ctrl+P, follow, context and MCP are NOT implemented. The company
  source remains unidentified; local Elastic proves neither its API nor network,
  SSO, cloud hosting or production policy. Docker remains deferred.
- Current continuation thread:
  https://ampcode.com/threads/T-01a16e7e-a5c4-753b-abc3-acfe490a8fa2

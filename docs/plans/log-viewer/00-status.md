# Log viewer exploration

- Gate 1 — Product: APPROVED 2026-10-23 by Andreas (“Yes, this sounds good”).
- Gate 2 — Architecture: APPROVED 2026-10-23 by Andreas (“Yeah. That sounds good. Let's go for it.”).
- Gate 3 — Program Design: APPROVED 2026-10-24 by Andreas (“Sounds good. Go ahead.”).
- Gate 4 — Slice plan: APPROVED 2026-10-24 by Andreas (“Go for it”).

## Scope

Slices 1–3 implement the independent fixture/local Elastic workbench under `log-viewer/`.
No LazyFTP/Service Bus code, docs or root setup changes. Midnight semantic colors
and shared layout/focus conventions are reused without cross-module internal
imports or changes to the existing apps. Local private lab startup is authorized;
no deployment or publishing.
SIGNAL is a provisional mockup label, not an approved product name.

## Slices

- [x] Slice 1 — fixture workbench tracer bullet, verified 2026-10-24.
- [x] Slice 2 — real local Elasticsearch happy path, verified 2026-10-24.
- [x] Slice 3 — app/time/level/message search, history and saved searches, verified 2026-10-24.
- [ ] Slice 4 — field yank, inspector and command palette.
  - [x] Command palette only, verified 2026-10-24; remaining Slice 4 work is not claimed.
  - [x] Inspector field selection/expansion and y/Y, verified 2026-10-24; help and independent panel toggles remain pending.
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
- Gate 4 is approved; Slices 1–3 are implemented and verified. Andreas explicitly
  requested parallel command-palette work; that part of Slice 4 continues in its
  own thread after the pushed Slice 3 checkpoint. Read 04-slices.md for acceptance and
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

## Slice 3 result and evidence

- Exact app multiselect (OR), explicit half-open RFC3339 UTC windows, severity
  including UNKNOWN/WARN+, analyzed message phrase, and exact trace IDs compose
  with AND across facets. App suggestions use bounded case-insensitive substring
  matching, not backend query syntax. Local literal find searches full decoded
  policy-safe values with case toggle and n/N navigation; fixture phrase behavior
  is labeled approximate rather than pretending to emulate Elasticsearch.
- Andreas requested saved app filters and easy inspect-to-filter. Inspector `f`
  now offers trace and explicitly indexed typed scalar fields, preserving existing
  facets. Unindexed/object/null/redacted values explain unavailability. Individual
  predicates can be removed. Production `app.keyword` mapping is not inferred;
  the lab uses ECS `service.name` and ten approved normalized pointer mappings.
- `S`/`R` explicitly save/restore whole named queries for the same source. Private
  owned directory/file, atomic replacement, strict bounded JSON, at most 50 names.
  No automatic writes, credentials, events or cursors; saved filter values may
  themselves be sensitive. Startup does not create a saved-search file.
- Frozen PIT/search_after history uses descending timestamp/native tie-break and
  chronological page display. Older pages prepend without changing selected event
  identity. Random one-use query-bound cursors expire after two minutes idle;
  replay/concurrent advancement cannot both succeed. PIT resources close on end,
  error, expiry and shutdown. At most 16 sessions; history explicitly stops at
  10,000 events / 32 MiB rather than silently evicting the selected event.
- `make -C log-viewer check test-terminal test-integration test-terminal-lab`:
  PASS (format, vet, race tests, fixture and actual local TLS PTY). Both PTYs print
  `PASS: search PTY` for app OR, WARN+/UNKNOWN, offset validation, phrase/empty,
  safe structured literal find/case/n/N, inspector trace+typed field AND, explicit
  private save/restore, anchored older history and narrow editor. Stale search and
  advanced-history completions close their returned cursors.
- Actual TLS integration: 600 unique equal-time references exactly once across
  500+100 rows; append excluded from existing PIT, fresh search returns 601;
  frozen/closed/replayed/foreign cursors fail. Verified all ten typed field paths,
  exact 9007199254740993 versus 9007199254740992, missing/custom UNKNOWN, phrase
  semantics, and literal operator characters in suggestions. An opt-in temporary
  public-service probe waited 122 seconds: expired real history rejected without
  data or broader fallback; probe removed after PASS.
- Explicit `reseed-indexed-fields` migrated only the owned disposable test index.
  Scenario IDs use a separate historical window and were removed after integration.
  Final `python3 log-viewer/scripts/lab.py check`: `TLS verified; read=22;
  owned_write=403; forbidden_read=403; rss_mib=939.81`. No production connection.
- Inspected real 2× screenshots under
  [slice3](../../../.amp/in/artifacts/log-viewer/slice3/): default, filter editor,
  exact app results, inspected-field picker, saved-search picker, narrow editor,
  empty results and offset validation. At 80×24 the editor shows the current field
  instead of all fields; Tab still reaches every field. Narrow summaries/help use
  ellipsis rather than overflowing. No preview renderer changes enter the app.
- Inspected 5-fps contact sheet of the 11.6-second real keyboard/moving-pointer
  [workflow clip](../../../.amp/in/artifacts/log-viewer/slice3/09-filter-workflow.mp4):
  filter editor, reduced app results, inspector field picker, further narrowed
  results and saved picker. Independent PTY assertions verify save/restore results.
  Same private ttyd/xterm fallback method as previous slices. Warm lab remains;
  preview/browser stopped after review. No production, clipboard, palette, follow,
  context or MCP success is claimed by this checkpoint.
- Andreas requested verified improvements pushed early/often and explicitly asked
  for command palette in parallel. Its owner is
  https://ampcode.com/threads/T-01a16ebc-d13a-7286-b3a8-29fb6f28b215
  and must pull this checkpoint before implementation. Full Slice 4 is not complete.

## Slice 4 — command-palette checkpoint

- Pulled the pushed Slice 3 checkpoint before implementation. Ctrl+P now opens
  a fuzzy, contextual action picker using the approved fuzzy v0.1.1 dependency
  and existing editor boundary. Arrows/Tab select; Enter dispatches the existing
  action; Esc restores the previous visible focus. Ordinary text, including
  q/y/Space, cannot dispatch global actions. No field yank, follow, MCP, panel
  toggles, help screen, sidebar or inspector redesign is included.
- Only implemented actions appear. Missing selected event/history/find and an
  embedded UI without a saved-search path show disabled reasons. Availability
  is recomputed on rendering and execution, including asynchronous reads/resize.
  Save still requires separate explicit editor submission; no automatic writes.
  Palette text passes the same input/output cleanup as the existing editors.
- `make -C log-viewer check test-terminal`: PASS (format, vet, race, actual
  executable fixture PTY and existing search workflow). Decisive output:
  `PASS: palette PTY — modal q/y/Space, fuzzy field action, disabled history,
  find handoff, focus restore and narrow resize, safe output`. Unit behavior
  checks cover available older-history dispatch with anchor retention, disabled
  event/save actions, and reachability/bounds at 7 widths × 3 heights (60×16 min).
  Real Elasticsearch integration was not rerun for this UI-only checkpoint.
- Inspected real 2× ttyd screenshots: [wide palette](../../../.amp/in/artifacts/log-viewer/palette/01-palette.png),
  [unavailable history](../../../.amp/in/artifacts/log-viewer/palette/02-unavailable.png),
  [no matches](../../../.amp/in/artifacts/log-viewer/palette/03-no-match.png),
  [80×24 palette](../../../.amp/in/artifacts/log-viewer/palette/04-narrow.png),
  [80×24 inspector-origin actions](../../../.amp/in/artifacts/log-viewer/palette/05-contextual.png).
  Same private ttyd/xterm DOM fallback method as Slice 3. Resized once after
  renderer preparation to remove stale initial cells, and waited for ttyd's
  resize badge to disappear before final narrow captures. No preview changes
  enter the app. All palette renders reuse the existing full-screen modal style.
- Inspected the 5-fps contact sheet of the 14.6-second real keyboard/moving-pointer
  [clip](../../../.amp/in/artifacts/log-viewer/palette/06-keyboard.mp4): open palette,
  select unavailable history, printable q/y/Space no-match query, cancel back to
  logs, inspect, reopen contextual palette, Enter into field picker, cancel back
  to inspector. PTY independently verifies resulting focus and safe output.
- Shared-file overlap for the next visual revision: only Ctrl+P dispatch in
  `model.go`, palette switch hooks in `editor.go`, and Ctrl+P footer hints in
  `view.go`; palette behavior is owned by `actions.go`. Pull this checkpoint before
  editing these files. No LazyFTP/Service Bus code/docs changed; no deployment or
  publishing. Field yank and the rest of Slice 4 remain pending.

### Palette visual correction requested by Andreas

- Andreas rejected the initial full-screen-editor appearance in favor of a real
  command-palette modal like the other apps. Reused the actual LazyFTP compositor
  method and Midnight styling: dim/recolor the existing workspace, center an
  opaque raised surface, cyan outline, mint selected row, right-aligned shortcuts,
  selected-action availability and concise keyboard footer. Geometry caps at
  82 columns / nine visible actions, shrinks with matches, and respects 60×16.
  All actions, modal input/focus, disabled reasons and output policy are unchanged.
- The only shared rendering hook is a palette-only branch before the generic
  editor branch in `View`; filter facets/sidebar/inspector revisions remain owned
  by the source thread. `actions.go` owns the ANSI-cell-aware overlay. No imports
  across app internals or edits to the other applications.
- `make -C log-viewer check test-terminal`: PASS, including real fixture palette
  and search PTYs, race/vet and existing 21-size bounds/focus cases. Extended the
  palette regression to require workbench panels remain visible behind the modal.
  Real Elastic integration was not rerun for this rendering-only correction.
- Inspected final real 2× screenshots: [default modal](../../../.amp/in/artifacts/log-viewer/palette-modal/01-modal.png),
  [filtered unavailable action](../../../.amp/in/artifacts/log-viewer/palette-modal/02-unavailable.png),
  [no matches](../../../.amp/in/artifacts/log-viewer/palette-modal/03-no-match.png),
  [80×24 modal](../../../.amp/in/artifacts/log-viewer/palette-modal/04-narrow.png),
  [80×24 inspector context](../../../.amp/in/artifacts/log-viewer/palette-modal/05-inspector.png).
  These supersede the initial full-screen palette images for current appearance.
- Inspected the entire 14.4-second real keyboard/moving-pointer
  [clip](../../../.amp/in/artifacts/log-viewer/palette-modal/06-keyboard.mp4)
  at 5 fps: unavailable history, query-driven shrinking/recentering, cancel,
  inspector, contextual palette, field picker and return to inspector. Private
  ttyd preview/browser stopped after review; temporary helpers removed. No
  deployment, publishing, clipboard, follow or MCP behavior added.

## Facet/sidebar/inspector visual refinement

- Andreas selected **B: Facet navigator** after reviewing the
  [two-direction Painter study](../../../.amp/in/artifacts/log-viewer/design-refinement/01-filter-inspector-options.png).
  This refines the approved workbench, not backend semantics or an additional
  provider. Filter navigation keeps App/Time/Levels/Message/Trace/Fields summaries
  on the left and only the current editor on the right. Time retains two inputs;
  existing Tab/Shift+Tab, Ctrl+S apply, Esc cancel and app-selection keys remain.
  Contextual help replaces the full-screen instruction wall. User-entered labels
  and values are cleaned before trusted styling, never after it.
- Saved searches replace planned-provider sidebar filler. Names are filtered to
  the current source, j/k/arrows navigate, Enter restores the actual query. An
  explicit save updates cached metadata; workspace entry / R reload the private
  file. Rendering has no filesystem side effects. Narrow terminals retain R and
  repair hidden-sidebar focus. Tests use a different-source save as a negative
  control, prove restored app predicates, immediate saved-name updates and resize.
- Inspector keys/headings are cyan, values bright, ERROR red, numbers amber,
  booleans mint and null/punctuation muted. Go stack function names, absolute
  paths and line numbers receive conservative highlighting in place. Unknown
  formats remain text, exact values/indentation/types remain intact, and the
  existing policy-safe boundary is unchanged. Selected log rows retain selection
  contrast; unselected ERROR retains semantic red.
- Integrated the parallel compact-palette checkpoint before final checks,
  preserving both palette-only and filter-only rendering branches and all palette
  behavior. No other app code/docs changed. Full Slice 4 remains unchecked.
- `make -C log-viewer check test-terminal test-terminal-lab`: PASS after integration
  (format/vet/race, actual fixture and authenticated local TLS PTYs). Existing
  app/trace/typed-predicate/find/save/history/palette paths pass. PTY now also
  restores from the sidebar. Facet bounds cover all seven inputs at seven widths
  × three heights. The initially incorrect use of ANSI left-truncation hid input
  text; existing editor ownership/PTTY assertions caught it, and corrected input
  display passes unchanged assertions. No backend adapter changes in this update.
- Inspected final real 2× captures: [workbench with saves/colors](../../../.amp/in/artifacts/log-viewer/facets/01-workbench.png),
  [App facet](../../../.amp/in/artifacts/log-viewer/facets/02-app-facet.png),
  [Time facet](../../../.amp/in/artifacts/log-viewer/facets/03-time-facet.png),
  [80×24 offset validation](../../../.amp/in/artifacts/log-viewer/facets/04-narrow-validation.png),
  [80×24 stack/types](../../../.amp/in/artifacts/log-viewer/facets/05-narrow-stack.png).
  Narrow validation wraps to preserve the full error. Inspected the 10.5-second
  real keyboard/moving-pointer [clip](../../../.amp/in/artifacts/log-viewer/facets/06-keyboard.mp4)
  at 5 fps: facet changes, cancel, saved sidebar restoration and inspector scroll.
  Private ttyd/xterm fallback method is unchanged. Preview stopped after review;
  private Elastic stays warm. No deployment or publishing.
- Andreas then requested visible inspector field selection and y yank in a new
  parallel thread: https://ampcode.com/threads/T-01a16ed6-8036-70ad-a14a-a93278d526bd.
  Its baseline real PTY confirmed Enter focuses inspector and j/Up scroll at both
  160×48 and 80×24; there is currently no field cursor and y remains unavailable.
  That owner must pull this visual checkpoint before implementing. This checkpoint
  does not claim field selection, clipboard receipt, follow, context or MCP.

## Slice 4 — inspector cursor and safe yank checkpoint

- Pulled the pushed facet/sidebar/colors checkpoint before editing shared UI.
  The reported case was investigated on the unchanged executable in actual PTYs:
  at both 160×48 and 80×24, Enter focused the inspector, j moved its line offset
  1→2, and Up restored it. Keys were not dead: the missing behavior was a visible
  field cursor, distinct from line scrolling. No guessed input-routing fix.
- j/k/arrows now select visible native JSON-pointer fields with a mint highlight
  only while inspector owns focus. Enter expands/collapses objects, arrays and
  shortened multiline values. PgUp/PgDn scroll expanded text and retain visible
  field ownership; g/G/Home/End select first/last visible field. Resize repairs
  hidden focus while retaining the field for full-width detail entry. Existing
  key/value/severity/Go-span semantic styling and policy-safe data are preserved.
- y copies the entire selected safe string or stable JSON value; Y prefixes the
  exact JSON pointer and `=`. Objects/arrays retain nested types and exact numbers.
  The f picker starts at that same field, keeps unavailable reasons, and applies
  the existing typed AND predicates without changing other facets. Palette adds
  contextual yank actions; modal q/y/Space ownership, facets and saved sidebar pass.
- Native pbcopy/wl-copy/xclip detection uses argv + stdin, never a shell with log
  values, and a two-second timeout. Failure is explicit and never falls back to a
  disk file. OSC52 requires deliberate `--trust-osc52` and reports “Copy sent”.
  Both native completion and OSC52 explicitly label receipt unverified. Actual
  receiving clipboard contents were NOT inspected; no host clipboard success is
  claimed. Test-only capture commands validate payloads, not a real clipboard.
- Added provider→SafeEvent incompleteness propagation. Incomplete events refuse
  copy conservatively; a visual ellipsis alone does not restrict full-value yank.
  Current adapters retain whole bounded events or reject them; page partiality
  alone is not field truncation. Unit behavior checks exercise a long UTF-8 value
  containing literal shell syntax, nested array/object, escaped pointer key,
  9007199254740993, null/bool, redaction, collapse, selected f, resize, native exit
  failure and timeout, and explicitly incomplete input. No field values in feedback.
- `make -C log-viewer check test-terminal test-integration test-terminal-lab`: PASS
  (format/vet/race, real fixture + authenticated TLS PTYs, actual Elastic tests).
  Both PTYs additionally print `PASS: copy PTY — exact y/Y OSC52 payloads, full
  multiline value, stable redacted object, selected f, 160×48→80×24 recovery;
  transmission only, receiving clipboard NOT verified`. Default no-trust mode
  excludes OSC52 and explains unavailability. Existing search/history/save/palette
  and bounds checks pass. Real TLS integration retains 600 unique equal-time IDs
  across 500+100 pages and exact typed predicates. Final lab check: `TLS verified;
  read=22; owned_write=403; forbidden_read=403; rss_mib=952.20`.
- Inspected actual DPR-2 terminal captures: [default](../../../.amp/in/artifacts/log-viewer/inspector/01-default.png),
  [field selected](../../../.amp/in/artifacts/log-viewer/inspector/02-field-selected.png),
  [unavailable clipboard](../../../.amp/in/artifacts/log-viewer/inspector/03-unavailable.png),
  [expanded Go stack](../../../.amp/in/artifacts/log-viewer/inspector/04-expanded-stack.png),
  [80×24 selected stack](../../../.amp/in/artifacts/log-viewer/inspector/05-narrow-selection.png),
  [80×24 native failure](../../../.amp/in/artifacts/log-viewer/inspector/06-narrow-failure.png),
  [80×24 paged stack continuation](../../../.amp/in/artifacts/log-viewer/inspector/08-paged-stack.png).
  Failure capture uses a disposable wl-copy command returning exit 1, not a host
  clipboard. Removed the transient ttyd size badge by waiting and recapturing.
  A stronger paging regression initially failed because PageUp selected
  the first viewport field even while the original field remained visible; fixed
  to retain selection while any of its rows are visible. Actual narrow PageDown
  and PageUp checks retain the stack path, with mint focus on the continuation.
- Inspected all 5-fps frames of the final 19.8-second real keyboard/moving-pointer
  [clip](../../../.amp/in/artifacts/log-viewer/inspector/07-keyboard.mp4): Enter,
  j/arrows, neighboring rows, message expansion, y/Y native failures, selected
  trace f picker and Esc restoring inspector. Same private ttyd/xterm DOM fallback;
  this run uses 1473×884 and 751×452 CSS at 15px font for 160×48/80×24 cells.
  Keep `--restore` consistent across setup and recorder commands, and wait for
  actual Search loaded before input: a mixed-mode blank recording and a startup
  input-race take were rejected, not published as evidence. Final commands also
  assert the visible terminal state between actions. Capture rate is 10 fps.
  Preview/browser stopped; disposable helpers removed; private TLS lab stays warm.
- Scope is this inspector/yank checkpoint, not full Slice 4: keyboard help and
  independent panel toggles remain pending. No follow/context/MCP, production
  provider claim, deployment/publishing, or LazyFTP/Service Bus edits.
- Delivery rebase preserved the concurrently pushed facet-pane navigation/draft
  reset changes. Reran the full check/fixture/TLS integration/TLS PTY command on
  that combined tree; pane wrapping, Time inputs and draft clear checks also pass.

# Status: Coherent mouse-and-keyboard terminal interactions

- Gate 1 — Product: APPROVED 2026-10-25 (Andreas; revised v2)
- Gate 2 — Architecture: APPROVED 2026-10-25 (Andreas)
- Gate 3 — Program Design: APPROVED 2026-10-25 (Andreas)
- Gate 4 — Slice plan: APPROVED 2026-10-25 (Andreas; unified build command added)

## Slices
- [x] 0 — Behavior-preserving app relocation and root build/setup entry points
- [x] 1 — Visible shared buttons/palette tracer in FTP and Logs fixtures
- [x] 2 — Service Bus buttons/palette plus contextual Help in all apps
- [x] 3 — FTP surfaces, chooser/wizard and checked operation routes
- [x] 4 — Service Bus docked review/reader surfaces and safety routes
- [x] 5 — Logs draft/settings/field surfaces and apply/cancel routes
- [x] 6 — Complete navigation and local control clicking in every app
- [x] 7 — Cross-app engineering conformance, fallback, visual and regression evidence; manual acceptance pending

## Approved quality cleanup — 2026-10-28

Andreas authorized a behavior-preserving cleanup after reviewing the 30.9% source
growth. Removed parallel row geometry, duplicate surface/reader construction and
reader-hint reverse parsing; centralized each app's minimum-size predicate.
Shared public APIs, app layouts, domain checks and input grammars remain unchanged.
Root checks/build/terminal suites passed, as did differential comparisons of
34,940 shared frame/input results and 31 actual-binary terminal frames.
This improves ownership rather than shrinking source: runtime nonblank lines
increased by 57 to 12,548. See 05-validation.md for method and limits.
Review focus-state consolidation and PTY harness extraction remain deferred.
All changes are local/uncommitted; manual/native-terminal acceptance is pending.

## Notes for a fresh session

This began as exploration requested by parent thread
https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937.
Andreas approved Gate 1 directly in this thread on 2026-10-25 and requested
three separate app directories plus a lib folder for shared code. Slice 1 now
modifies FTP/Logs runtime code and adds the real shared library. Read 01-product.md, research.md,
interaction-design.md, 02-architecture.md and 03-program-design.md. Andreas
approved Gate 2 directly with “Sounds good” and Gate 3 with “Ok, let's go” on
2026-10-25. Andreas approved Gate 4 with “Ok. Let's go” and requested one build
command for all apps or a named app. Use `./build [lazyftp|service-bus-explorer|log-viewer]`,
delegating to app Makefiles; no argument builds all. Slices 0–7 are engineering
validated locally; manual/usability acceptance remains pending. Andreas authorized
slice 7 with “Yeah. Let's go for slide seven. Go for it.” Read 05-validation.md
for evidence, measured costs and acceptance gaps. Andreas authorized slice 6 with
“Go for it!” Andreas authorized slice 5 and slice 4 with “Go for it.” Andreas authorized slice 2
with “Okay, so let's bring in the service bus” and slice 3 with “Yeah, I mean, it's
looking good. Go for it.” Nothing committed or pushed.
Read 04-slices.md next. Do not repeat design-gate approvals. Each implementation
slice ends with verification and a request to continue or re-steer.
The latest direction REQUIRES mouse clicking in all three apps and visible shared
action-button, palette, surface and navigation contracts, not only utility reuse.
Version 1's utilities-only recommendation is superseded. Research/API examples
are inputs to later gates, not approved architecture or implementation slices.
The apps remain three separate Go modules with independent layouts/domain safety.
Review per-app static HTML mockups in mockups/. Slice 0 changed source locations
and build/setup entry points only; slice 1 adds shared buttons and palettes.

## Slice 7 evidence — 2026-10-28

- Engineering validated; manual/usability acceptance pending. FTP now supports
  `--mouse=false`; all three apps have real-binary keyboard-only fallback checks.
  Seven additional cross-app truecolor/NO_COLOR/Service Bus ASCII PTY cases passed,
  with zero mouse enable sequences when disabled and zero color SGR under NO_COLOR.
- Conformance audit found hidden keyboard operations below minimum dimensions.
  FTP palette editing/activation, Logs editor submission and Service Bus settings/
  palette actions now freeze while the resize notice owns input. Safe cancel/quit
  policy and asynchronous lifecycle handling remain local; drafts survive resize.
  Regression tests failed before and passed after. Identical before/after real
  binary PTYs reproduced hidden Logs/SBX private-file writes and proved refusal
  after the fix, with visible saving still working after resize recovery.
- Service Bus typed-delete prompt now emits a current visible field region and
  focuses its existing authorization owner. Tests cover exact authorization,
  release ignoring, hidden/stale pixels and explicit final confirmation.
- Final `make check`, `./build`, `make test-terminal` and explicit four-module
  workspace tests passed. FTP four-target archives/checksums passed; extracted
  Linux amd64 package exposes the mouse flag. Cross-build is not native execution.
- Three palettes became one shared implementation; no pointer-to-synthetic-key
  dispatch or peer-app production imports remain. Including adapters and themes,
  nonblank UI/CLI Go lines grew 9,540 → 12,491 (+30.9%), including 1,028 shared
  lines. This buys new interaction coverage/safety, not net code shrink. Three
  app-local PTY harnesses remain; no test-setup reduction target is claimed met.
- Inspected actual monochrome/ASCII terminal states, Logs resize/draft recovery,
  and keyboard Help/return recording. Evidence: `.amp/in/artifacts/slice7/`.
  Theme calculations identify existing FTP Dracula muted/surface contrast 1.94:1;
  no universal contrast/accessibility certification. No live labs or data changes.
- Manual acceptance still needs five participants, both input methods, native
  terminal/tmux/SSH, receiving clipboard and screen-reader checks. Nothing
  committed, pushed or deployed. No next implementation slice is implied.

## Slice 6 evidence — 2026-10-28

- All three apps consume shared `HandleNavigation` over current clipped regions.
  Pane/control/destination Tab traversal, select-only data rows, explicit Open/
  Inspect, independent disclosure/marks and pointed-pane wheel handling retain
  app-owned effects and native target identities. Hidden or partially clipped
  controls cannot activate. App-local keyboard grammars and domain gates remain.
- FTP file rows now select without opening; the mark checkbox remains independent.
  Explicit Open/Inspect, pane/source controls, jobs, filter footer and inline rename
  use checked named operations, not synthetic key events. Chooser/wizard controls
  have visible traversal. Live terminal scripts were inspected/adjusted, not run.
- Service Bus adds card/table/message selection, explicit Active/DLQ/Inspect,
  tree disclosure, paging, settings/watch toggles and reader-local controls. Exact
  CRLF/grapheme copy, review expiry/identity/release and UNKNOWN/partial outcomes
  retain existing checked routes. Logs adds native event/field/saved-search regions,
  wrapped-field ownership, separate disclosure and reader controls; filter Tab
  traverses editor → sidebar → footer while preserving INSERT text.
- `make check`, `./build`, all-app `make test-terminal`, independent four-module
  builds and explicit workspace tests passed. Root checks include race/vet and ten
  Python lab unit tests. Actual SGR PTYs cover select-versus-open, independent marks,
  disclosure, wheel locality, reader controls, footer traversal and existing safety
  regressions. Per-app `docs/ui-click-inventory.md` records controls, key/pointer
  routes, visibility gates and scenarios; it is not a human acceptance claim.
- Inspected actual fixture-terminal wide/compact states, FTP select/mark/Open,
  Service Bus select/Open, Logs disclosure/footer focus and moving-pointer clips
  in every app. Evidence: `.amp/in/artifacts/slice6/`. No live broker, Elastic,
  remote file or lab operations. All settings paths were private/disposable.
- Slice 7 must close FTP's preexisting missing `--mouse=false` switch and validate
  all-app fallback/conformance, packaging and developer-duplication measurements.
  Human usability, native terminal/tmux/SSH, receiving clipboard and screen-reader
  acceptance remain unmeasured. Nothing committed, pushed or deployed. Safe session
  boundary: continue from these approved docs. Next decision: continue to slice 7,
  or re-steer.

## Slice 5 evidence — 2026-10-28

- Log Viewer filter/settings/find/save/restore/field editors consume shared Surface
  chrome, current regions and capture. The filter keeps its app-local two-pane
  layout and NORMAL/INSERT grammar. Semantic checked operations replace key-branch
  coupling: explicit Apply validates directly, Cancel discards immediately, while
  keyboard Enter and first INSERT Esc retain their existing meanings.
- Facet/input presses focus; app/field/saved-search row presses select only.
  Checkbox/Toggle app, Remove field and explicit Apply/Restore perform operations.
  Typed field bytes, redaction/support gates, exact app OR, cross-facet AND,
  same-source restore, private-file validation and suggestion generations remain
  app-owned. Header Actions cannot leak through a disabled control into dismissal.
  Validation remains above the footer at 60×16; obscured targets cannot activate.
- `make check`, `./build`, all-app `make test-terminal`, independent four-module
  builds and explicit workspace tests passed. New real-binary SGR scenarios check
  Cancel/Apply from INSERT, no keyword-as-app toggle, settings toggle/cancel without
  write, case-sensitive Find and explicit typed-field Apply. Public tests cover
  exact persisted queries, select-only Restore, compact validation and refusal of
  redacted-field Apply. Targeted race checks passed after the final test extension.
- Inspected actual fixture-terminal NORMAL/INSERT, Help/retained draft, compact
  validation, settings, find, save, restore and field-picker frames, plus a real
  moving-pointer Help/return/Cancel recording. Evidence: `.amp/in/artifacts/slice5/`.
  Private disposable files only; no live Elastic/lab operations. Native terminal,
  tmux/SSH, receiving clipboard and screen-reader acceptance remain unverified.
- Nothing committed, pushed or deployed. Complete workspace navigation, remaining
  local clicks and full facet/footer traversal remain slice 6. Next decision:
  continue to slice 6, or re-steer.

## Slice 4 evidence — 2026-10-27

- Service Bus docked review, destination/configuration/preparation, reader and
  full-content loading now consume shared Surface chrome, regions and capture.
  Checked app-local intents retain phase/expiry/size/visibility gates, exact typed
  delete, frozen prepared identity, release-before-edit and late-delivery release.
  UNKNOWN send and partial cleanup remain distinct from success. No shared callback
  calls the broker. Destination rows select only; Choose/Enter activates.
- Cancel remains the default. At undersized dimensions visible Cancel may accept
  Enter; submission remains unavailable. Body-only review hides submission controls.
  Wide review keeps identity and prepared body vertically within the dock instead
  of the old internal side-by-side body panel; app workspace layout stays local.
  Explicit reader Back leaves immediately; keyboard Esc keeps staged selection
  clearing. Reader grammar/grapheme selection and exact clipboard payloads remain
  local. Full-load cancellation invalidates late results without copying.
- Final `make check`, `./build`, independent four-module builds, explicit workspace
  tests, all-app `make test-terminal` and whitespace checks passed. Fixture PTYs
  exercise real SGR reader Back/Resend, cleanup toggle, prepare/send and Cancel/release,
  plus mouse-off keyboard fallback. Model tests cover 79×24/80×23 refusal versus
  80×24 submission, expiry before compact gating, hidden controls, exact delete,
  late acquisition, frozen identity, UNKNOWN/partial and release-before-edit.
- Inspected actual fixture-terminal reader, configuration, final review, Help and
  80×24/body-only captures, plus moving-pointer prepare/Help/Cancel recording.
  Busy sending and full-content confirmation/loading were deterministic model
  frames with held commands; temporary capture tests were removed. No live broker
  or lab operations, native terminal/tmux/SSH or screen-reader acceptance claimed.
- Evidence: `.amp/in/artifacts/slice4/`. Navigation/message-toolbar clicking remains
  slice 6. Nothing committed, pushed or deployed. Next decision: continue to slice 5
  (Log Viewer draft/settings/field surfaces and checked Apply/Cancel), or re-steer.

## Slice 3 evidence — 2026-10-27

- Shared Surface now owns bounded title/close/footer chrome, current regions,
  capture and visible focus traversal. FTP consumes it for delete, folder,
  rename review, copy/name/conflict, batch, preview, connection picker and wizard.
  App-local named operations replace modal synthetic-key dispatch; workspace
  file navigation remains unchanged until slice 6.
- Connection-row clicks select only; Open/Enter opens details. Explicit chooser
  dismissal returns to retained files instead of translating Escape+Quit. Wizard
  Close follows Back/invalidation, not unconditional dismissal. Save/Connect,
  candidate ownership, async reconciliation and secret-field editing stay local.
- Confirmations start on Cancel; fields keep Enter grammar; preview body does not
  activate on Enter and Tab reaches Close. Noncolor button markers reserve stable
  geometry; clipped controls cannot activate. Checking exposes Cancel only;
  queue Start still pauses at conflicts; running rename refusal remains local.
- `make check`, `./build`, independent four-module builds and explicit workspace
  tests passed together. `make test-terminal` passed all three apps, including
  the new real-binary local-files scenario: select-only/Open, wizard Help/Back,
  chooser Close, delete/rename Cancel and bounded preview Close. It verifies
  asymmetric source bytes unchanged and no renamed file; no remote connection.
- Inspected real-terminal chooser, wizard input/validation, Cancel-default delete,
  compact delete and preview focus captures plus a pointer Open/return recording.
  Copy checking was rendered deterministically with fake storage and a held Stat
  command, not a live backend. Temporary renderer test removed. Busy/refusal and
  stale completion checks run through the public model tests.
- No live lab or cloud state changed. Native terminal/tmux/SSH, screen readers,
  human usability and live integrations remain unverified. Nothing committed,
  pushed or deployed. Next decision: continue to slice 4 (Service Bus docked
  review/reader surfaces and checked safety routes), or re-steer.

## Slice 2 evidence — 2026-10-27

- All three apps now consume real shared ActionBar, Palette and contextual Help.
  Service Bus's duplicate palette renderer/filter/input path is replaced; mouse
  reporting is enabled with `--mouse=false` fallback. Workspace/review/reader
  navigation clicks remain outside this slice.
- Shared Help wraps contextual topics and action metadata with scrollable,
  width-dependent geometry. F1 or the visible Help control opens it; close/Esc/F1
  closes only Help. Help captures background clicks without dismissing its parent.
  Apps keep parent drafts, queries, selection and checked domain operations local.
- Service Bus review/full-load states visibly block Actions while allowing Help.
  Completion and lock expiry continue beneath Help. Tests preserve prepared target
  identity, typed authorization and safe release; Logs retains INSERT drafts and
  existing Ctrl+C ownership; FTP retains rename refusal and Cancel defaults.
- `make check`, `./build`, `make test-terminal`, independent four-module builds,
  explicit workspace-pattern tests and `git diff --check` passed together. Includes
  race/vet, ten Python tests, eight FTP terminal variants, Service Bus mouse-on/off
  lifecycle fixtures and Logs Help/INSERT/search/Vim/copy scenarios.
- Actual fixture-terminal captures and pointer recordings cover shared palette,
  Help and return with a retained query; contextual Logs filter Help was inspected.
  These complement real-binary PTY assertions, not live backend/native-terminal
  acceptance testing. No lab state or live data changed.
- Next decision: continue to slice 3 (FTP surfaces, chooser/wizard and checked
  operation routes), or re-steer. No generic surface/navigation framework has been
  introduced. Nothing committed, pushed or deployed.

## Slice 1 evidence — 2026-10-27

- Real `lib/terminalui` module provides ActionBar, Palette, geometry, styles and
  typed intents, consumed by FTP and Logs through independent module requirements
  and local replacements. Their two palette renderers/input handlers are replaced
  by one implementation; app layouts and checked domain dispatch remain local.
- Logs palette actions no longer synthesize keyboard events. Logs enables mouse
  reporting by default, with `--mouse=false` retaining keyboard-only operation.
  Mouse scope in this slice is shared toolbar/palette, not all workspace controls.
- Capture consumes outside dismissal without activating the workspace; disabled
  actions cannot execute, pointer releases do not repeat activation, and current
  geometry follows resize. FTP retains Ctrl+C cleanup/quit before palette capture.
- `make check`, `./build`, `make test-terminal`, independent four-module builds,
  explicit workspace-pattern tests and `git diff --check` passed. This includes
  race/vet checks, all eight FTP PTY variants, SBX lifecycle fixtures and Logs
  mouse/search/Vim/copy checks; keyboard-only copy checks use `--mouse=false`.
- Actual fixture terminals were rendered through browser-hosted ttyd, not HTML
  mockups. Review captures cover normal, open, unavailable and compact states;
  recordings exercise pointer movement and safe inspector actions. These visuals
  complement PTY assertions, not proof of live backend behavior.
- Service Bus integration and contextual Help remain slice 2; surfaces, complete
  navigation/click coverage and native-terminal accessibility remain later work.
  No lab state or live data was mutated. Nothing committed, pushed or deployed.

## Slice 0 evidence — 2026-10-27

- All three apps relocated under `apps/`; all 113 Go files remain byte-identical.
  Seven relocated user documents changed only for paths/setup/build guidance.
- Root `./build` builds all apps (including FTP MCP); each named app also built
  from an unrelated working directory without rebuilding other apps. Unknown,
  empty and extra arguments rejected before building. `make build APP=log-viewer`
  also passed. Binaries remain app-local.
- `make check` passed independent `GOWORK=off` vet/race checks for every app and
  all 10 SBX Python lab tests. `make test-terminal` passed all existing offline
  real-binary suites, before and after relocation. Auxiliary sbx-lab also builds.
- Six decoded 160×48 terminal frames (workspace and palette per app) exactly
  matched baseline. FTP four-target packaging/checksums passed; setup ran twice
  successfully with pinned dependencies and app-local Python environments.
- Workspace tests pass with explicit app-module patterns. Root `go test ./...`
  (or `go test ./apps/...`) is not a workspace-wide command; use `make check`.
- Lab state/credentials and preexisting ignored environments were not moved or
  adopted. No app lab services were started, stopped or seeded.
- `go.work` currently lists three apps. Add the real `lib/terminalui` module and
  consuming requirements/replacements with slice 1, not an empty library stub.
- Remaining work: shared interactions (slices 1–7); live integration/user studies
  and native-terminal accessibility remain unverified. Safe session boundary:
  continue from the approved docs without repeating design gates.

## Revised Gate 1 review state

Version 2 was approved on 2026-10-25. All three static studies (four states
each) were rendered and visually inspected at 1280px and 390px browser widths;
the proposal and all linked studies returned HTTP 200. Artifact checks passed
for the proposal and three studies. These are HTML design checks, not application
tests or proof of terminal behavior. No runtime files changed; no commits made.
The stable review page is:
https://t-03h3zrzzb2n894aouxsjn8a9u-p27852.onamp.dev/shared-terminal-ui.html
Its floating review button accepts comments on individual blocks.

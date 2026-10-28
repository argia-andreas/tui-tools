# Vertical slices: coherent terminal interactions

Gate 4 APPROVED by Andreas on 2026-10-25. Gates 1–3 approved 2026-10-25.
Approval includes a unified `./build [app]` command: default all three, or a named
lazyftp/service-bus-explorer/log-viewer; recipes remain in app Makefiles.
Approval permits starting slice 0, not silently
running through every slice: after each, show evidence, update 00-status.md and
ask “Continue to slice N+1, or re-steer?” No push/publish/deploy authorization.

## Build order

0. Relocate the three apps and repair suite entry points without changing behavior.
1. Visible tracer: shared Actions buttons and palette in FTP and Logs fixtures.
2. Service Bus buttons/palette and safe contextual Help in all three apps.
3. FTP shared surfaces and picker/wizard controls, with checked local operations.
4. Service Bus docked review/reader surfaces, with lock and confirmation safety.
5. Logs draft/settings/field surfaces, with exact Apply/Cancel and INSERT grammar.
6. Complete all-app navigation, select-first rows and remaining local control clicks.
7. Full cross-app conformance, fallback and rendered interaction validation.

Slice 0 is the explicitly separate relocation prerequisite approved in Gate 2.
Slice 1 is the first UI tracer, using existing offline fixtures as its stubbed
data source; do not invent another mock backend. Subsequent slices are complete
user interactions, not “all utilities, then all rendering, then all handlers.”
Only introduce the part of lib/terminalui needed by the current visible slice.

## Baseline and common checks

Before moving source, record existing module tests/builds and offline PTYs. A
preexisting failure is a baseline, not permission to hide/skip it or fix unrelated
behavior. Record the old→new tracked-file manifest and preserve unexpected edits.
Inspect lab/service state read-only before touching source paths. If active
services need an old-path handoff, obtain the specific operational approval first;
do not stop/reset/reseed services, adopt ownership markers or move ignored data.

After relocation, common module checks (from repository root) are:

```sh
for module in apps/lazyftp apps/service-bus-explorer apps/log-viewer lib/terminalui; do
  GOWORK=off go -C "$module" build ./...
  GOWORK=off go -C "$module" vet ./...
  GOWORK=off go -C "$module" test -race ./...
done
```

Slice 0 visits the three app modules only. Add lib/terminalui and the fourth
go.work entry in slice 1 when real shared code exists, not an empty module stub.
App dependency requirements/replacements are added with actual consumption.
Run root aggregate build/check once added; it must visit all existing modules
explicitly and not rely on root `go test ./...`. Check formatting on tracked/new
Go sources without scanning .venv/node_modules. All edited scripts get syntax
checks; Python bytecode/scratch output goes outside tracked source.

Offline real-binary checks preserve their current app-root invocation:

```sh
make -C apps/lazyftp test-terminal
make -C apps/service-bus-explorer terminal-test
make -C apps/log-viewer test-terminal
python3 -m unittest discover -s apps/service-bus-explorer/scripts -p lab_test.py
```

Recreate app-local Python environments/dependencies at their new paths. Load
orb-setup before changing .agents/setup; preserve dependency pins. No copied
virtualenvs, hidden reliance on an old root environment, or automatic live labs.
Extend existing PTYs for new behavior rather than keeping old contradictory
assertions. Unit fixtures/fakes cover operation safety without external mutations.

## Slice 0 — Apps can still build and run after relocation

**Outcome:** apps/ contains three peer applications; root provides suite discovery
and build/check entry points. Existing behavior, names, flags/config locations and
fixtures remain unchanged. Move tracked scripts/lab/docs with their app; history
plans stay at root. No shared UI/alias/selection policy changes in this slice.

**Evidence:** common three-module checks; root aggregate build/check; all existing
offline PTYs and SBX Python lab tests; current packaging cross-build matrix in a
fresh temporary output directory with checksum verification. Inspect setup's new
discovery paths, ignore coverage and app README links. Build auxiliary sbx-lab.
Compare baseline representative frames to relocated versions; movement alone
must not change them. Do not execute live integration tests solely for relocation.

**Stop condition:** any unaccounted source loss, skipped module, changed runtime
behavior or unmanaged active-service path. No broad compatibility symlink layer.
Rollback tracked moves/path edits only; never touch preexisting ignored lab data.

## Slice 1 — Two apps use the same visible palette

**Outcome:** FTP and Logs fixtures show common Actions controls and palette,
with click/key action parity, contextual availability and consistent clamped
navigation. Non-mutating Inspect exercises a real semantic request end to end;
other implemented action IDs dispatch to existing checked local operations.
Replace each migrated palette handler, do not leave a second active key route.
Add current Frame/Region geometry and semantic style code only as needed here.
Logs gains mouse reporting with real palette routing and --mouse=false fallback;
mouse support is explicitly incomplete until later navigation/editor slices.

**Evidence:** shared action/palette public-contract tests; FTP/Logs existing
model tests and offline PTYs; q/j/? query capture, unavailable activation,
no results, first/last clamp, outside dismiss over a background control,
press/release single activation and resize while palette open. Render/inspect
default, palette, disabled and compact frames in both running apps; record a
pointer crossing neighboring controls, opening/selecting/canceling the palette.

**Stop condition:** shared handlers need domain callbacks or raw event simulation.
Refactor a local checked operation instead; return to design if public API changes
materially. Do not migrate destructive workflows just to finish palette plumbing.

## Slice 2 — All palettes and Help are coherent

**Outcome:** SBX uses common Actions/palette, real pointer routes and mouse-off
fallback. All three have Actions/Help/F1; grouped help uses current action keys
and local reader/editor topics. F1 returns without losing draft/prepared review
state. Correct misleading SBX Ctrl+P report-copy hint and Logs unavailable help.

**Evidence:** all three offline PTYs and relevant app tests; help while a draft
or broker review exists, keyboard/mouse return, current availability and no
workspace escape. Keep lifecycle/expiry processing active during help. Inspect
and record all three palettes/help states including blocked Actions reasons.

**Stop condition:** Help resets a domain state or introduces a generic modal
stack. Retain parent state explicitly and use the agreed view-only help scope.

## Slice 3 — FTP surfaces and connection controls share the contract

**Outcome:** confirmation/destination/preview/queue surfaces, connection chooser
and wizard use shared chrome/actions and current hit geometry. Both inputs call
named local operations; explicit chooser Dismiss returns to files. Cancel-first
confirmation does not replace input-form focus or wizard Back semantics. Apply
chooser-row select-first and explicit Open when that chooser migrates here;
workspace file-row selection changes remain in slice 6.

**Evidence:** existing model/delete/rename/transfer/queue/preview and chooser/
wizard/workspace tests extended with pointer counterparts. Cover single highlighted
delete versus marked neighbor, queue guard, checking/conflict button sets, running
rename refusal, endpoint retention, test candidate invalidation and explicit
bookmark save versus connect. Use fake storage/session outcomes, not real remote
writes. Inspect/record input, validation, confirmation, busy and compact states.

**Stop condition:** Close/Cancel bypasses running-operation policy or source/session
ownership. External-editor upload behavior is not refactored as part of UI reuse.

## Slice 4 — Broker review is clickable without weaker safety

**Outcome:** SBX docked review, destination/preparation and full-load/reader
surfaces use common chrome and local checked intents. No shared click calls
operate directly. Cancel/Back still reconcile acquisition or release correctly.

**Evidence:** review/inspector/model tests and fixture PTY. Exercise 79×24 and
80×23 hidden-confirm refusal versus valid80×24, body-only view, expiry before
press, prepared identity after list change, exact delete authorization, late
delivery after cancel, unknown send and cleanup partial. Verify exact copy/reader
selection behavior unchanged. Inspect/record docked review, disabled/busy/help,
Cancel default and compact body-only frames using fixtures/fakes.

**Stop condition:** pointer bypasses phase/expiry/visibility/authorization or turns
unknown/partial into success. No live Service Bus reset/seed/drain required.

## Slice 5 — Log drafts and fields are clickable without ambiguous Apply

**Outcome:** draft filters/settings/find/saved-search/field controls use common
surfaces with local grammar. Apply calls checked query/apply, not fake Enter;
Cancel closes the draft, not fake Esc. First keyboard Esc in INSERT stays local.

**Evidence:** existing model/search tests and fixture PTY with isolated settings/
search files. Contrast Apply with Enter on suggestion/predicate/INSERT; verify
cancel leaves results/files unchanged, apply refreshes, save requires an explicit
path, safe field values retain types/redaction, and suggestion generations reject
stale results. Inspect/record INSERT/NORMAL, validation, unavailable save, Help
return and compact states. No live Elastic writes or credentials needed.

**Stop condition:** common Surface changes filter scope, history budget or copy
policy. Those remain local sources of truth, not opportunities for cleanup.

## Slice 6 — Complete the all-app click inventory

**Outcome:** pane focus, navigation destinations, row selection, explicit Open/
Inspect, disclosure/checkboxes, editor fields, settings and remaining visible
actions have paired click/key routes in each app. Apply FTP workspace file-row
select-first deliberately and verify the already migrated connection rows.
Shared traversal uses visible IDs only;
apps retain selection/reader/INSERT transition effects and remembered endpoints.

**Evidence:** per-app inventory listing screen/control, key route, pointer target,
enabled/disabled/hidden conditions and scenario. Target100% visible controls in
each app, not a cross-app average. Test row selection versus activation, independent
marks, wrapped field owners, wheel locality, hidden focus repair and current
geometry after async/resize. Retire remaining synthetic-key control clicks and
parallel migrated handlers. Inspect/record neighboring controls and wide/compact
navigation in every app. Update FTP live-script expectations by inspection, but
execute live scripts only with separately authorized disposable-lab access.

**Stop condition:** missing local control is relabelled unsupported just to reach
100%. Explicitly excluded drag/drop and arbitrary text-range mouse selection do
not expand this inventory; actual visible controls do.

## Slice 7 — Combined proof and honest acceptance report

**Outcome:** all common/module/offline PTY checks, actual app captures and coverage
inventory are collected against the approved states. Component implementation
is shared in all three apps, without peer-app imports or domain effects in lib.

**Evidence:** common four-module checks with GOWORK=off plus workspace checks;
all offline PTYs; root build/check; local packaging/checksums; mouse=false keyboard
operation and reporting toggles; themes/ASCII/color-off fallback with explicit
limits. Check zero input leakage, duplicate requests, hidden/disabled activation
and domain safety bypass; compare developer duplication including adapters, not
just library LOC. Collect screenshots/clips per representative affected state.

Use deterministic asymmetric fixtures and stale-target/resize/expiry cases; never
weaken an assertion or hardcode a special case to make a suite green. Remove
scratch programs/temp archives and keep only useful regression tests/evidence.

Usability ≥90% (five participants × three apps × both methods), native terminal/
tmux/SSH behavior and screen-reader checks require actual people/environments.
Report what ran and what remains unmeasured; do not claim these passed from PTY
or screenshots. If unavailable, hand back “engineering validated; manual/usability
acceptance pending,” not an unqualified all-metrics completion. No publishing or
release merely because local checks pass. Preserve any preexisting failures in
the report and distinguish them from introduced regressions.

## Approval and continuation

Approve Gate 4, or what should change?

On approval, mark this gate approved and start slice0's read-only baseline/service
inventory, then source relocation if no operational handoff blocks it. After its
verification, stop with “Continue to slice 1, or re-steer?” Later slices follow
the same continuation rule. Gates and scope changes remain explicit decisions.

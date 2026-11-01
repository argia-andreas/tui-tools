# Slice plan: Shared panel rendering

## Build order

1. **Queue tracer:** implement the approved shared renderer and public contract test, replace Queue's ordinary-panel loop, and demonstrate the real fixture app with unchanged panels and interaction.
2. **Logs:** migrate workspace/list/inspector and nested filter-editor panels, preserving focus markers, clipping, styling, and editor ownership.
3. **Streams:** migrate ordinary panels, preserving space-filled titles, hard clipping, ASCII mode, nested-style backgrounds, density, and regions.
4. **Transfer:** migrate block rendering, preserving accent colors, surface-backed side panels, jobs drawers, detach controls, and focus highlighting; finish full-suite verification.

This is a behavior-preserving refactor of working apps, not a new feature needing
mock endpoints or stub screens. The first tracer therefore connects the real
renderer to one real consumer. There is no library-only slice, temporary fake
production panel, or unconnected framework scaffold.

## Before the first runtime edit

- Record the starting worktree/commit; preserve unrelated changes.
- Capture the unchanged panel output and deterministic fixture states needed for
  all four consumers using the Gate 3 matrix. Keep scratch data and any reference
  implementation copies outside tracked source and outside reviewable artifacts.
- Use private disposable preferences, fictional/fake data, and deterministic
  timestamps. Do not connect live cloud services or mutate remote storage.
- Retain only user-reviewable captures in `.amp/in/artifacts/`; capture and inspect
  actual terminal renders, not HTML imitations of the intended panel design.

Baseline collection is a prerequisite within Slice 1, not a separate horizontal
implementation batch. No shared renderer or app adapter is written before
Gate 4 approval.

## Slice 1 — Queue tracer

**Change:** add `lib/terminalui/panel.go`, extend the existing public render tests,
and replace only Queue's ordinary `panel` drawing loop with the approved API.
Queue is first because its body content is already caller-painted, avoiding the
paint-order change while establishing real shared use and multiple-theme coverage.

**Preserve:** existing title cleaning, rule fill, focus border, Unicode ordinary
panels even when shared controls use ASCII, and app-owned layout/regions. Do not
change populated reader Surfaces or environment/review behavior.

**Evidence:** run the shared contract tests and Queue race suite, then
`make -C apps/queue terminal-test` (its existing mouse-on/off and environment
checks). Compare Queue's Gate 3 panel cell/color outputs and regions with the
baseline. Inspect captures across its three themes and representative focused,
narrow and wide states. Exercise and record pointer/focus movement across
neighboring panels. Run all module race tests after adding the shared renderer
to check the additive library change does not disturb existing consumers.

**Exit:** Queue uses the real shared renderer, its local drawing loop is gone,
and required comparisons/checks pass without changing tests' expected behavior.

## Slice 2 — Logs

**Change:** replace the local `panel` drawing loop in `view.go`; keep its current
signature so workspace/list/inspector and both filter-editor panes migrate
together without content-builder rewrites.

**Preserve:** `●` focus decoration, rule fill, ellipsis, foreground preparation,
selection styles, and existing string/Frame composition. Do not modify Surface
capture or NORMAL/INSERT grammar.

**Evidence:** run shared and Logs race tests and
`make -C apps/logs test-terminal`. Compare the Logs Gate 3 matrix with baseline,
including the 60×16 nested editor layout, workspace visibility boundaries, and
styled inspector content. Inspect default and editor/focus captures; exercise
pointer and keyboard traversal without losing the draft or changing targets.

**Exit:** workspace and nested editor panels share drawing; no local assembly
loop remains and their cell/style/region comparisons match.

## Slice 3 — Streams

**Change:** replace local panel assembly in `view.go`, keeping Frame construction,
pane/row IDs and current reset-aware content preparation app-owned.

**Preserve:** space-filled headers, `>` focus prefix, no clipping marker,
Unicode/ASCII borders, syntax/selection styles, sidebar visibility and density.

**Evidence:** run shared and Streams race tests and
`make -C apps/streams test-terminal`. Compare its Gate 3 matrix with baseline,
especially color versus NO_COLOR fallback, JSON resets, groups/metadata panes,
both density modes and 109/110 and 139/140 width boundaries. Inspect captures
and a pointer/focus walkthrough with neighboring targets visible.

**Exit:** all existing ordinary panel consumers use the same shared API without
new flags, app-name branches, paint callbacks, or region changes.

## Slice 4 — Transfer and final validation

**Change:** replace `block` assembly in `view.go`; retain title/body preparation,
existing helper signature, detach-title patching, and navigation highlighting.

**Preserve:** title accent versus border colors, copied-model Background/Surface
selection, plain versus styled content, jobs drawer sizing, and visible controls.
Exercise live-style detach chrome using a fake/local backend, not remote services.

**Evidence:** run shared and Transfer race tests and
`make -C apps/transfer test-terminal`. Compare Transfer's Gate 3 matrix, including
the 3-row minimum demo drawer, every preset, a custom contrasting palette, and
long titles. Inspect captures and pointer/focus traversal with detach/job controls
where applicable. Then run root `make check` and `make test-terminal`, rechecking
all consumers if the shared renderer changed during any migration.

**Exit:** four ordinary-panel assembly loops have been replaced, real differences
remain in small app-owned adapters, all required checks pass, and final captures
show the affected variants without unintended changes. Remove temporary probes,
reference copies and scratch data. Report verification limits explicitly.

## Stop and approval rules

- If preserving appearance needs a new API field or a different ownership
  boundary, stop and revise the relevant earlier gate before continuing. Do not
  hide incompatibilities with app-identity switches or normalize visible behavior.
- Do not weaken tests to accommodate a refactor. Investigate any baseline failure
  separately from a new regression, and report what was actually exercised.
- At each slice boundary, save verification results and remaining work in
  00-status.md, show the running result and inspected evidence, and ask
  "Continue to slice N+1, or re-steer?" For the final slice, report completion
  and delivery state instead of asking for a nonexistent next slice.
- Gate approvals do not authorize pushing, publishing, deployment, or external
  mutations. No such action is part of this plan.

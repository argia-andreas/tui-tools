# Status: Shared panel rendering

- Gate 1 — Product: APPROVED 2026-11-01
- Gate 2 — Architecture: APPROVED 2026-11-01
- Gate 3 — Program Design: APPROVED 2026-11-01
- Gate 4 — Slice plan: APPROVED 2026-11-01

## Slices

- [x] Slice 1 — Shared renderer used by Queue ordinary panels
- [x] Slice 2 — Logs workspace and filter-editor panels
- [x] Slice 3 — Streams panels, ASCII and nested styles
- [x] Slice 4 — Transfer panels and full-suite validation

All four migrations are implemented and verified locally. The user's later
instruction to finish removing duplication superseded the per-slice pauses.
The user subsequently authorized committing all changes and shipping to
origin/main after rebasing and running the full suite. Delivery status is
reported in the thread.

## Result and verification

- Four independent ordinary-panel assembly loops became one RenderPanel in the
  existing terminalui module. Adapters retain theme/focus/title/body preparation;
  layout, regions, controls, and theme catalogs remain app-owned.
- Production line count: 39 lines removed net from app helpers, 55 lines added
  in the shared renderer: **16 more lines overall**, not a code-size reduction.
  Permanent public-contract coverage adds 67 test lines. Planning docs excluded.
- Root `make check` passed: formatting, vet, all module race suites, and the
  Queue/Streams Python lab unit suites (11 and 5 tests).
- Root `make test-terminal` passed all four apps' offline PTY suites, including
  Queue mouse/environment variants, Logs search/editor, Streams color/ASCII and
  mouse variants, and Transfer copy-target/drawer/responsive checks.
- Queue before/after comparison: 198 panel cases and 576 workspace Frames with
  no cell/style/region differences, covering three themes and responsive layouts.
- Additional helper comparisons: Logs 50, Streams 200, Transfer 600 cases with
  no cell/style differences (and unchanged Streams regions). Include focus,
  Unicode, overflow, selected/nested ANSI bodies, empty content, no-color/ASCII,
  and Transfer's five presets plus contrasting custom colors.
- The broader scratch probes also exposed differences for raw tabs, injected
  ANSI titles, and degenerate dimensions. These are not claimed equivalent:
  app titles are static or sanitized; provider tabs are sanitized before panel
  rendering. Supported layout minima are documented in 03-program-design.md.
  Those synthetic inputs were excluded from the equivalence count, not hidden
  by changing existing app tests. The shared tiny-size contract has its own tests.
- Real fixture screenshots were inspected for Queue themes/compact layout,
  Logs workspace/filter editor, Streams normal/ASCII, and Transfer workspace and
  inspector/transfers drawer. A real pointer/focus walkthrough was recorded and
  its contact sheet inspected; no blank frames remained in the final recording.
- Comparison probes were temporary and removed. This is representative
  validation, not exhaustive screenshot comparison of every Gate 3 state. No
  live brokers, remote storage, or live detach session were exercised; existing
  unit and offline PTY tests cover their local rendering/input boundaries.

## Notes for a fresh session

- The user requested planning shared panel rendering as the first refactoring
  following the code quality review.
- The intended scope is behavior-preserving consolidation of ordinary panel
  chrome across Transfer, Queue, Logs, and Streams, not a common workspace or
  interaction framework.
- Existing panels differ in title decoration, focus appearance, truncation,
  background/ANSI handling, and ASCII support. Preserve those differences unless
  a later explicit decision changes scope; do not normalize them incidentally.
- Existing shared interaction design remains authoritative for app ownership of
  layouts, region identities, input routing, and domain safety.
- The user explicitly approved Gate 1. Product scope is in 01-product.md.
- The user explicitly approved Gate 2. Architecture is in 02-architecture.md.
- The user asked whether theming is shared: terminalui.Style is shared, but theme
  presets, selection/loading, and app-specific roles are not. This refactor reuses
  Style without expanding into shared theme management.
- The user explicitly approved Gate 3. API and validation matrix are in
  03-program-design.md.
- The user explicitly approved Gate 4 and subsequently requested completion of
  duplication removal across the remaining apps.
- Start with Queue because it supplies already-painted body content and tests
  rule-filled titles plus multiple themes without moving painting order.
- All slices now use the shared renderer. Further work should follow the user's
  next request, not repeat implementation or the approved gates.
- Baseline source and executables were frozen from starting commit
  21e48765e1ffcc4860ddd0595934b13d619fb7a1 for before/after validation across
  the slices; the temporary baseline and probes were removed after verification.

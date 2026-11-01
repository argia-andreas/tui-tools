# Architecture: Shared panel rendering

## Fit

Extend the existing `lib/terminalui` module with a pure ordinary-panel renderer.
All four apps already depend on it; no new module, dependency, service, or common
application layer is needed. Keep shared interaction APIs and behavior unchanged.

The extraction replaces the drawing loops in these existing helpers:

| App | Existing helper | Presentation to retain |
| --- | --- | --- |
| Transfer | `internal/ui/view.go`: `block` | Rule-filled title, separate title accent and focus-border colors, ellipsis clipping, existing styled-body handling |
| Queue | `internal/ui/view.go`: `panel` | Rule-filled title, focus-border color, ellipsis clipping, caller-painted body content |
| Logs | `internal/ui/view.go`: `panel` | Rule-filled title, focused `●` prefix and border color, ellipsis clipping, foreground-painted body |
| Streams | `internal/ui/view.go`: `panel` | Space-filled title, focused `>` prefix and border color, no clipping marker, existing ASCII glyphs and nested-reset background handling |

Logs also uses its helper for two panels inside the filter editor. Migrate their
ordinary panel drawing with the helper, but do not change the containing shared
Surface, editor grammar, focus ownership, or capture behavior.

### Ownership

Shared renderer owns:

- Border assembly and supported border glyph selection.
- Cell-aware clipping of safe styled title/body lines, title fill, body padding,
  and the number of rendered body rows.
- Painting panel chrome and padding using supplied semantic colors.

Apps retain:

- Layout dimensions, responsive visibility, panel placement, and composition.
- Focus state, title wording/decorations, title sanitation, and color selection.
- Domain body construction, row selection styling, safe provider-text handling,
  and content windows.
- Frame bounds, pane/row/control IDs and regions, hit testing, input routing,
  lifecycle, effects, and all authorization.

The shared result is rendered lines, not a new interaction scope. Existing local
helpers may remain as thin adapters where they translate model/theme/focus into
rendering inputs, join lines, or construct an existing Frame. They must not retain
duplicate border/clipping/padding loops. A simple call site need not get an adapter.

### Handling real presentation differences

Use explicit presentation inputs for differences already required by consumers:
decorated title, title fill, title/border colors, truncation marker, and ASCII
glyph choice. Do not add app-name switches, per-app presets, callbacks into app
models, or a configurable general-purpose widget system.

Keep body preparation app-owned. Transfer distinguishes styled from unstyled
lines, Queue supplies prepainted content, and Logs/Streams paint their content
with different nested-style behavior. Preserve that preparation at the adapter
boundary, including Streams' reset restoration. The shared renderer clips the
prepared lines and paints padding separately; it does not strip selection styles
or attempt to infer domain-specific backgrounds. Gate 3 must establish the exact
clipping-versus-painting order from the existing behavior before fixing the API.

Reuse the existing Style and terminal-cell dependencies rather than introducing
a second theme model. Leave `Fit` and unrelated local `fit` callers unchanged:
this extraction does not require a repository-wide text-fitting migration.

Current helpers differ for degenerate dimensions. The supported dimensions and
call-site guards must be audited in Gate 3. Preserve reachable rendering behavior;
do not add a broad collection of tiny-size compatibility switches without a real
consumer. If a reachable difference cannot be preserved by the proposed boundary,
revise the architecture rather than silently normalize it.

### Alternatives rejected

- Reusing RenderSurface: adds close/footer/capture semantics to ordinary panes
  and reserves different body space.
- Sharing complete workspace/inspector models: merges distinct layouts, data,
  and lifecycles without reducing the targeted drawing duplication.
- Merely sharing a border-string helper: leaves clipping, padding, and most of
  the repeated loops in four places.
- One renderer with app-identity branches or paint callbacks: relocates app
  behavior into the library instead of establishing a coherent boundary.
- Normalizing panel appearance: violates approved behavior-preserving scope.

## Endpoints

None. No HTTP, CLI, MCP, or interaction API changes.

## Data

No persisted data changes, migrations, configuration changes, or new runtime
state. Rendering remains synchronous and stateless. Existing Frames retain their
local text lines and absolute region coordinates.

## Flow

1. The app computes its existing layout, visible content, selection, and regions.
2. Its existing panel helper or call site selects colors, decorates/sanitizes the
   title, and prepares styled body lines using the current app behavior.
3. The shared renderer clips, pads, paints chrome, and assembles panel lines.
4. The app places those lines in its existing Frame or string composition.
5. Existing input routing uses app-owned regions and shared interaction handlers
   exactly as before. Rendering emits no intents and creates no control regions.

## External

None. Existing lipgloss and ANSI-cell libraries remain the rendering dependencies.
No network, filesystem, clipboard, terminal-control transmission, or new
environment-variable behavior is introduced.

## Validation boundary

Before implementation, Gate 3 defines a representative matrix and captures the
unchanged renderers' observable output. Compare visible cells and style/background
coverage, not only stripped strings or raw escape-byte equality. Include focused
and unfocused panels, all existing presentation variants, Unicode clipping,
selected and syntax-styled lines, resize boundaries, and supported themes/ASCII.

Run shared renderer contract tests, existing app tests and race checks, and the
affected fixture terminal suites. Inspect before/after rendered captures and
exercise focus/resize interactions to ensure visible content and hit regions
still agree. Differential comparison can be temporary validation; do not retain
four old implementations as permanent test or production code.

Migration order and exact signatures remain Gate 3/4 decisions. Each migration
must use the same shared renderer without introducing a parallel legacy engine.

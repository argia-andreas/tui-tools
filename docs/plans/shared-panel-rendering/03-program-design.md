# Program Design: Shared panel rendering

## Files

| File | Change and ownership |
| --- | --- |
| `lib/terminalui/panel.go` (new) | Pure ordinary-panel renderer, beside existing shared renderers |
| `lib/terminalui/render_test.go` | Extend public rendering contracts for panels; reuse the existing test package and ANSI dependencies |
| `apps/transfer/internal/ui/view.go` | Replace `block` drawing loop with shared call and local theme/title/body preparation; keep detach-title patching and all layout code |
| `apps/queue/internal/ui/view.go` | Replace `panel` drawing loop; retain title cleaning and color/focus selection |
| `apps/logs/internal/ui/view.go` | Replace `panel` drawing loop; retain focus marker, style preparation, and string joining expected by callers |
| `apps/streams/internal/ui/view.go` | Replace `panel` drawing loop; retain reset-aware body painting and existing Frame/region construction |
| `docs/plans/shared-panel-rendering/00-status.md` | Gate approvals and eventual slice progress |
| `docs/plans/shared-panel-rendering/04-slices.md` (Gate 4) | Ordered migration/verification batches, not defined by this gate |

No changes planned to module dependencies, theme packages, shared Style/Surface/
Fit/navigation, app configuration, lab scripts, or other review findings.
Existing app tests should pass unchanged; if a real contract gap is discovered,
extend its existing test file rather than adding a private-helper test suite.

## Types & signatures

```go
// lib/terminalui/panel.go
type PanelSpec struct {
    Title      string
    Lines      []string
    TitleColor string
    TitleRule  bool
    Ellipsis   string
}

func RenderPanel(spec PanelSpec, width, height int, style Style) []string
```

Contract:

- Safe single-line content only, like the existing shared renderers. This API
  does not sanitize provider data, interpret newlines, or accept callbacks.
- Title already contains app-owned decoration and spacing. The renderer clips
  it to `width-2` cells using Ellipsis. Unused title cells are filled with the
  horizontal border glyph when TitleRule is true, otherwise spaces.
- TitleColor overrides the title/fill foreground; empty means style.Border.
  Border foreground comes from style.Border, already resolved for focus by the
  app. Chrome and padding use style.Background, not dialog style.Surface.
- Body lines are already styled as needed by the app. The renderer clips them
  with the same Ellipsis and adds separately foreground/background-painted
  padding. It does not repaint supplied body content or strip nested styles.
- Empty Ellipsis means hard clipping. Current consumers use only `""` or `"…"`.
- style.ASCII chooses existing `|`, `-`, `+` border glyphs; otherwise Unicode box
  drawing. Only Streams enables it for ordinary panels today. Queue's existing
  ordinary panels stay Unicode even when other controls use ASCII.
- For width and height at least 2, return exactly height lines, each exactly
  width terminal cells: title/top border, height-2 body rows, bottom border.
  Extra body rows are ignored; missing rows are empty padded rows. Inputs are
  not mutated. No regions or intents are created.
- For width < 2 or height < 2, return max(0,height) empty strings, matching the
  existing guarded Transfer helper. Other apps' reachable callers must satisfy
  the normal size contract; no per-app tiny-panel modes are added.

No new local signatures are required. Keep the existing block/panel helper
signatures to avoid touching their many content-building callers.

### Reachable size audit

All ordinary-panel call paths are behind workspace size gates; none reaches
width < 2 or height < 2. Minimum widths and heights below need not occur in the
same panel:

| App | Smallest width | Smallest height | Relevant sizing |
| --- | --- | --- | --- |
| Transfer | 22 (navigation) | 3 (demo jobs drawer) | Workspace >=60×12; drawer is min(5/6/9, availableHeight-6), leaving file panes >=6 rows |
| Queue | 24 (sidebar) | 7 (cards or legacy-mode message list) | Workspace >=60×18 scoped or 60×16 legacy; message toolbar reduces body by 3 rows; nested cards have fixed height 7 |
| Logs | 19 (filter navigation) | 9 (workspace) | Workspace >=60×16; workspace height is terminalHeight-7; filter panels have widths min(28,(terminalWidth-2)/3) and remainder, height terminalHeight-6 |
| Streams | 22 (events sidebar) | 9 | Workspace >=60×16; panel height terminalHeight-7; inspector is at least 44 cells when visible |

Thus the normal >=2 contract covers every supported layout without tiny-size
compatibility modes. Transfer's side panels copy the model with Background set
to Surface before calling block; pass that resolved Style, not the outer model's
original background. Keep its post-render control highlighting app-owned too.

### Consumer mapping

| App | Title | TitleRule | Ellipsis | Style and body preparation |
| --- | --- | --- | --- | --- |
| Transfer | `"─ " + text(title) + " "` | true | `"…"` | Existing TerminalStyle(false), chosen border/focus and title accent; paint plain input lines with foreground, preserve styled lines |
| Queue | `"─ " + clean(title,false) + " "` | true | `"…"` | Existing TerminalStyle(false), chosen border/focus; supply body lines as today without wrapping them in a new default style |
| Logs | `" " + decoratedTitle + " "` | true | `"…"` | Existing Interactions(), chosen border/focus, existing `●` prefix; prepare lines with foreground style |
| Streams | `" " + decoratedTitle + " "` | false | `""` | Existing style(), chosen border/focus, existing `>` prefix and ASCII setting; prepare lines through current reset-aware paint |

Local body preparation maps only supplied visible lines. Shared drawing handles
missing lines, clipping, and padding. No adapter keeps its own border/row assembly
loop or clips lines to work around the shared API.

The existing clipping-before-painting order becomes painting-before-clipping for
the app-prepared content. A temporary Go/pyte probe compared these orders for
plain text, CJK/combining text, nested selection/foreground resets, narrow/wide
widths, and both truncation markers. All 120 comparisons matched visible cells
and foreground/background attributes. This is evidence for the boundary, not
exhaustive proof: actual adapter differential checks remain required, especially
Transfer's styled-versus-plain handling and whitespace-only lines.

### Theming boundary

Reuse terminalui.Style as the shared semantic color vocabulary. Do not share
theme catalogs or configuration in this batch. Transfer has five presets and
custom color loading; Queue has three presets; Logs supplies its own Midnight
theme; Streams supplies a Style directly. Preserve these capabilities and all
current colors, including distinct palettes that happen to share a name.

## Call stack

### Transfer

`View` / interaction frame construction → existing pane builder → `block`
resolves theme/focus/title and prepares lines → `terminalui.RenderPanel` →
existing pane composition. `filePane` retains its existing detach-button title
patch and associated app-owned hit region after drawing.

### Queue

`View` / workspace frame → sidebar/dashboard/message/settings builder → `panel`
resolves colors and cleans title → `terminalui.RenderPanel` → existing composition.
Do not move panelBody/addHit or resource identities into the renderer.

### Logs

`View` → workspace/list/inspector builder, or editor Surface → filterBody →
existing `panel` decorates title and styles lines → `terminalui.RenderPanel` →
join returned lines for existing callers. Region ownership and editor input
capture remain unchanged.

### Streams

`View` / navFrame → frames/topic/list/inspector builder → `panel` selects theme,
decorates title, and reset-aware paints lines → `terminalui.RenderPanel` →
existing Frame with original pane/body regions → existing placement/navigation.

## Test plan

### Permanent tests

Extend `lib/terminalui/render_test.go` with one table-driven public contract test,
`TestPanelPresentationAndCellBounds`, covering:

- Asymmetric title/border colors and body background versus Surface to catch
  role mixups; rule-filled and space-filled headers, Unicode and ASCII glyphs.
- Widths 0, 1, 2, 3, 11, and 24; heights 0, 1, 2, and 5; missing and excess body
  rows. Assert the documented normal/tiny size behavior.
- Overlong title and body with hard clipping versus ellipsis, CJK, combining
  graphemes and emoji; assert expected retained text and exact cell widths.
- Styled selection/token content followed by normal text and separately painted
  padding. Force color rendering in the test and assert public style preservation
  without requiring incidental escape-sequence byte order.

Keep app regressions unchanged, notably Transfer's
`TestLayoutVisibilityAndCellBounds` and navigation tests; Queue's
`TestFixtureWorkspaceNavigationAndResponsiveFrames`,
`TestAllThemesResponsivePaletteAndLargeTable` and pointer tests; Logs'
`TestMultilineDataCannotBreakLayout`,
`TestNavigationSelectFirstDisclosureAndWrappedFieldOwner`, and
`TestFacetPaneNavigationApplyAndClear`; Streams'
`TestDensitySelectionAndPayloadModal` and existing highlight tests.

### Temporary before/after validation

After Gate 4 approval, before any runtime edit, capture baseline ordinary-panel
output and real fixture-terminal states. Use the same inputs on migrated code.
Compare text cells and foreground/background attributes via the existing pyte
terminal tooling; do not use stripped output or byte equality as the only oracle.
Include blank/whitespace-only bodies and a style sequence appearing after the
clipping boundary. Compare existing frame regions unchanged alongside output.
Remove temporary probes and baseline implementation copies after validation.

Representative state matrix:

| App | Widths / heights | Required states |
| --- | --- | --- |
| Transfer | 60×12; widths 89/90, 119/120 at height 32; 160×48 | Focus left/right; navigation and inspector visibility; jobs drawer empty/populated; selected and unselected styled rows; long title and live-style detach chrome using fake/local backend; all five presets plus one custom contrasting palette |
| Queue | 60×18; widths 109/110 and 121/122 at height 40; 160×48 | Scoped fixture dashboard/list, sidebar and inspector layouts, focused/unfocused panels, empty/loading panels and cards; all three themes, both ASCII settings and mouse modes |
| Logs | 60×16; widths 119/120 and 139/140 at height 40; 160×48 | Focused list/inspector/workspace, long/wrapped selected field, filter-editor navigation/edit panels including INSERT focus; Midnight |
| Streams | 60×16; widths 109/110 and 139/140 at height 40; 160×48 | Topics/events/groups/partition metadata, selected JSON syntax-styled inspector, both density modes, focused/unfocused and sidebar visibility; color and NO_COLOR ASCII fallback |

Use fixed fictional data and private disposable preferences for comparisons;
freeze timestamps where displayed and wait for deterministic loaded states.
Inspect rendered before/after captures for each presentation variant. Record a
short pointer/focus walkthrough across neighboring panels after migration and
inspect it to catch disagreement between drawn content and hit geometry.

Run `make check` and `make test-terminal` from the repository root. These cover
independent module race tests and the existing offline terminal suites; no live
brokers, remote storage, or new lab provisioning is needed. For visual evidence,
use the existing terminal-preview tooling and supervised orb service workflow;
save only reviewable captures under `.amp/in/artifacts/`.

## Least confident decisions

1. Prepainting preserves visible behavior in the styling probe, but actual
   Transfer/Logs/Streams content may expose an order-sensitive case. Differential
   validation must cover it before calling the shared boundary proven; revise
   this design rather than adding an app-identity branch or paint callback.
2. Keeping thin local helpers limits migration churn but is justified only if
   they contain policy translation, not retained duplicate rendering loops.
3. The proposed small input set must stay sufficient. If further flags become
   necessary, compare their complexity with leaving an exceptional panel local;
   do not grow a widget framework just to claim complete adoption.

# Ordinary panel rendering

Transfer, Logs, QE (currently the `queue` executable), and Streams use
[`terminalui.RenderPanel`](../../lib/terminalui/panel.go) for ordinary panel
chrome. This is a drawing boundary, not a common workspace, widget framework,
or interaction scope. The broader
[shared interaction architecture](../plans/shared-terminal-ui/02-architecture.md)
retains its separate ownership and acceptance requirements.

## Ownership and safety

The shared renderer assembles borders, clips safe styled lines by terminal-cell
width, fills titles, pads body rows, and paints chrome using supplied colors.
It returns lines only: no regions, intents, effects, lifecycle, or authorization.

Apps own layout, responsive visibility, panel dimensions and placement, focus,
title wording/decoration, theme selection, body windows and selection styles.
They also own provider-text sanitization, input routing, region identities and
coordinates, domain operations, and all safety checks. Rendering cannot grant
permissions or make a clipped/hidden control actionable.

**Inputs must already be safe single-line content.** `RenderPanel` does not
sanitize provider data or interpret embedded newlines. Body lines are prepared
and styled by the app, including nested ANSI/reset handling. The renderer clips
but does not repaint supplied body content; it paints padding separately.
Do not pass unsanitized service text or use this API as a security filter.

Use small app-owned adapters to translate existing theme/focus/content policy.
Do not add app-identity switches, model callbacks, or duplicate border/padding
loops to the shared renderer. Ordinary panels deliberately do not use
`RenderSurface`: surfaces add capture, close/footer and body-space semantics
that ordinary panes do not own.

## Public presentation contract

`PanelSpec` supplies `Title`, pre-styled `Lines`, `TitleColor`, `TitleRule`, and
`Ellipsis`; `RenderPanel` also receives width, height, and `terminalui.Style`.

- The app supplies title decoration and spacing. Title and body clip to
  width − 2 terminal cells. Empty `Ellipsis` hard-clips; consumers otherwise
  use `…`.
- `TitleRule` fills unused header cells with the horizontal border glyph;
  otherwise they are spaces. Empty `TitleColor` uses `Style.Border`.
- Border/focus colors are resolved by the app. Chrome and padding use
  `Style.Background`, not `Style.Surface`. Apps needing a surface-backed panel
  pass an appropriately resolved style.
- `Style.ASCII` selects `|`, `-`, `+`; otherwise borders use Unicode box drawing.
- At width and height ≥ 2, output has exactly height lines of exactly width
  terminal cells. Extra body rows are ignored and missing rows are padded.
- At width < 2 or height < 2, output has max(0, height) empty strings. Inputs are
  not mutated. Supported app layouts guard normal panels above these minima;
  the tiny-size behavior is a library contract, not a promise of usable tiny UIs.

## App-specific presentation remains intentional

| App | Adapter behavior |
| --- | --- |
| Transfer | Rule-filled title, separate title accent and focus-border colors, ellipsis; paints plain lines while preserving styled lines. Surface-backed side panels resolve their own background. Detach/control highlights and hit regions remain app-owned. |
| QE | Rule-filled title, focus-border color, ellipsis, caller-painted body. Ordinary panels remain Unicode even when other controls use ASCII. |
| Logs | Rule-filled title, `●` focus prefix, ellipsis, foreground-painted body. The same renderer draws filter-editor panels without changing editor capture or grammar. |
| Streams | Space-filled title, `>` focus prefix, hard clipping, optional ASCII borders and reset-aware body preparation. Regions, density and visibility remain app-owned. |

Themes share the semantic `Style` vocabulary, not catalogs or configuration.
Do not normalize colors or similarly named palettes as part of renderer work.

## Historical migration verification

The completed migration was recorded on 2026-11-01. These are historical results
from the retired plan, **not fresh checks performed by this documentation cleanup**.

- Root `make check` passed formatting, vet, independent module race suites, and
  QE/Streams Python lab-script unit suites (11 and 5 tests).
- Root `make test-terminal` passed all four offline PTY suites, including QE
  mouse/environment variants, Logs search/editor, Streams color/ASCII and mouse
  variants, and Transfer copy-target/drawer/responsive checks.
- Differential comparisons reported no cell/style differences for 198 QE panel
  cases and 576 workspace frames (including unchanged regions), 50 Logs helper
  cases, 200 Streams cases (unchanged regions), and 600 Transfer cases. Coverage
  included focus, Unicode/overflow, selected/nested ANSI bodies, empty content,
  supported themes, no-color/ASCII, and Transfer custom colors.
- A separate 120-case paint-order probe matched visible cells and
  foreground/background attributes for plain, CJK/combining and nested-style
  content. This supported app-owned prepainting; it was not exhaustive proof.
- Fixture captures were inspected for QE themes/compact layout, Logs workspace
  and filter editor, Streams normal/ASCII, and Transfer workspace/drawers. A
  pointer/focus walkthrough and contact sheet were inspected. Captures and
  temporary comparison probes are not distributed with this documentation.
- Raw tabs, injected ANSI titles, and degenerate dimensions were excluded from
  equivalence claims: supported callers sanitize text and enforce layout minima.
  Tiny-size behavior has its own public-contract tests. These exclusions are
  not evidence that arbitrary unsafe input is supported.
- No live brokers, remote storage or live detach session were exercised. Coverage
  was representative, not exhaustive comparison of every proposed state, and
  does not close native-terminal or human-acceptance gaps.

For future changes, run the public rendering contracts in
[`render_test.go`](../../lib/terminalui/render_test.go), app tests, root
`make check` and `make test-terminal`. Compare cell content, style/background
coverage **and regions**, not only stripped text or raw escape-byte equality.
Inspect representative rendered states and focus/resize behavior with safe,
deterministic fixtures. Keep fresh results distinct from the history above.

# Evidence: shared terminal interactions (v2)

Source inspection, 24–25 October 2026; no app runtime changes or application
tests executed. The revised product direction explicitly requires all-app mouse
support and coherent reusable interactions. Version 1's utilities-only direction
is superseded. See `01-product.md` for Gate 1 scope and `interaction-design.md`
for unapproved technical sketches, alternatives, migration and validation.

## What changes in the recommendation

The evidence does not change: mouse support and coherent behavior are missing,
not already implemented everywhere. The goal now goes beyond removing duplicate
code: **ActionButton/ActionBar, CommandPalette, Surface and Navigation contracts**
should deliberately normalize shared experiences. Cell helpers and tests support
those components; they are not the deliverable. Preserve app layouts, domain
state machines and output policies, not every incidental difference in UI behavior.

## Duplication and gaps

F = LazyFTP root, B = service-bus-explorer, L = log-viewer. Paths are repository
relative. Source facts are separated from intended changes; none implies a
reproduced runtime failure. File ranges identify ownership, not duplicate LOC.

| Area | Source evidence | Observed implementation | Revised disposition |
| --- | --- | --- | --- |
| Dependencies/modules | Three `go.mod` files | All use Bubble Tea 1.3.10, Lip Gloss 1.1.0, x/ansi 0.10.2, sahilm/fuzzy 0.1.1; three module graphs | Small reusable interaction package, not a new terminal framework; Gate 2 resolves module delivery |
| Action descriptors | F `internal/ui/model.go:518–585`; B `service-bus-explorer/internal/ui/actions.go:15–64`; L `log-viewer/internal/ui/actions.go:15–81` | Three registries/matchers. F IDs/keys/keywords/reasons, B IDs/titles with context omission, L labels/keys/reasons and synthetic-key dispatch | One Action contract feeds buttons, palette and action help; app determines authorization, relevance, availability and execution |
| Palette state/keys | F `internal/ui/model.go:587–663`; B `service-bus-explorer/internal/ui/actions.go:66–105`; L `log-viewer/internal/ui/actions.go:83–121` | F wraps, Ctrl+P closes; B clamps, Ctrl+P moves up; L clamps and uses Tab. Input limits differ | Shared component owns normalized query/navigation/activation, not three local handlers. Explicit product changes, tested separately from extraction |
| Palette rendering | F `internal/ui/view.go:467–556`; B `service-bus-explorer/internal/ui/view.go:444–471`; L `log-viewer/internal/ui/actions.go:123–186` | Query/results/hints, centered overlay, dimmed background; widths/rows/reasons differ | One recognizable palette frame and result layout, common empty/unavailable/help states and geometry |
| Action buttons | F `internal/ui/view.go:75–86`, `internal/ui/mouse.go:169–178`; B `service-bus-explorer/internal/ui/view.go:45–53,348–387`; L `log-viewer/internal/ui/view.go:126–150` | F toolbar hit tests, B bordered keyboard-only buttons, L mostly text hints | Reusable button/action bar owns render + hit region + focused/disabled/busy states. Add actual click paths to B/L |
| Mouse | F `cmd/lazyftp/main.go:87`, `internal/ui/mouse.go:11–283`, `cmd/lazyftp/picker.go:68–107`, `wizard.go:359–377`; B `service-bus-explorer/cmd/sbx/main.go:54`; L `log-viewer/cmd/logview/main.go:88` | Only F enables mouse reporting; B/L have no UI MouseMsg routing | All three must support clicking, including app-local entry points. Normalize events at component boundary; no domain-wide mouse dispatcher |
| Input precedence | F `internal/ui/model.go:324–385`, `mouse.go:23–168`; B `service-bus-explorer/internal/ui/model.go:218–225,292–315`, `review.go:180–240`; L `log-viewer/internal/ui/model.go:90–125`, `editor.go:160–185` | F rename before global quit, B inspector/review messages first, L editor before globals | Shared Surface capture/consumption contract for both input methods; app still owns lifecycle and cancellation permission |
| Navigation/focus | F `internal/ui/model.go:178–207,392–421`, `internal/ui/mouse.go:256–281`; B `service-bus-explorer/internal/ui/model.go:319–390`; L `log-viewer/internal/ui/model.go:94–102,140–185` | F preserves active endpoint and click opens files/dirs; B string focus with reader controls; L returnFocus + detail mode | Shared click-to-focus, select-vs-activate, visible focus order and focus repair. App supplies ordered visible targets, rows/identities and primary activation |
| Help | F `internal/ui/view.go:558–566`, `cmd/lazyftp/picker.go:111–132,329–345`; B `service-bus-explorer/internal/ui/inspector.go:280–301,498–543`; L `log-viewer/internal/ui/model.go:249–253`, `view.go:126–135` | F global F1/? guide, B reader help, L footers and ? unavailable | Shared Help entry/rendering from action descriptors plus local navigation/reader topics. Add F1/Help to all apps without destroying drafts/reviews |
| Surface/confirmations | F `internal/ui/delete.go:46–85`, rename/transfer/queue files; B `service-bus-explorer/internal/ui/review.go:242–308`, `selected.go`, `review_view.go`; L `log-viewer/internal/ui/editor.go`, `fieldfilter.go` | Cancel-default file deletion, broker lock/send/cleanup review, log draft saves are distinct. B review replaces reader content | Share chrome, actions, capture and close request; allow overlay or docked mounting. Do not share operation state machines or generic confirm-and-run callback |
| List/detail/inspector | F `internal/ui/model.go:462–467`, `view.go:444–465`, `preview.go`; B `service-bus-explorer/internal/ui/inspector.go:66–106,303–349,545–562`; L `log-viewer/internal/ui/inspector.go:22–169` | File metadata/preview ≠ original-message logical lines/graphemes ≠ redacted typed JSON fields | Shared row selection/primary action affordance only; no universal inspector, schema or selection-copy model |
| Responsive layout | F `internal/ui/view.go:98–155`; B `service-bus-explorer/internal/ui/model.go:203–216`, `view.go:308–328`, `inspector.go:113–130`, `review.go:242–308`; L `log-viewer/internal/ui/view.go:49–123`, `model.go:94–102` | F min 60×12, nav90/inspector120; B min60×16, sidebar110/122, split content98, confirmation80×24; L min60×16, workspace140/inspector120 | Keep topology/breakpoints/safety gates; shared components adapt internally and return visible hit/focus targets only |
| Cell/style rendering | F `internal/ui/view.go:23–71,127–140`; B `service-bus-explorer/internal/ui/view.go:28–79`; L `log-viewer/internal/ui/view.go:18–44` | Three fit/frame implementations, ANSI cuts and padding | Foundation of common component appearance and matching hit geometry, not sole shared abstraction |
| Themes | F `internal/theme/theme.go:16–139`; B `service-bus-explorer/internal/theme/theme.go:9–58`; L `log-viewer/internal/theme/theme.go:7–20` | Eight identical Midnight core roles; F five presets + strict JSON; B three presets + broker roles; L only Midnight | Common role-to-component styling, not hardcoded colors in widgets. Keep theme loaders/catalogs and domain accents local |
| Status/errors | F `internal/ui/model.go:285–318`, `delete.go:86–112`; B `service-bus-explorer/internal/ui/model.go:235–289`, `inspector.go:458–493`; L `log-viewer/internal/ui/view.go:136–157`, `clipboard.go:19–79` | Transfer partials, broker unknown/partial outcomes, log reading/partial/budget and unverified copy receipt | Shared inline feedback placement/labels; app owns truth, severity, persistence and lifecycle |
| Data safety/clipboard | F `internal/ui/view.go:14–21`; B `service-bus-explorer/internal/ui/view.go:14–26`, `inspector.go`; L `log-viewer/internal/logs/output.go:22–110` | All strip ANSI but newline/tab/bidi policies differ. L redacts and rechecks after cleaning; B source-byte copy is deliberately separate | Do not share generic Clean/clipboard policy. Components accept presentation-safe content; domain output remains owned upstream |
| Model tests | F `internal/ui/model_test.go:22–69`; B `service-bus-explorer/internal/ui/model_test.go:203–238,458–491`; L `log-viewer/internal/ui/search_test.go:80–235`, `model_test.go:30–162` | Input ownership/frame/selection tests already exist. F drains batch commands synchronously | Shared component conformance tests + pure frame assertions; keep async/domain tests local, no unlimited shared command drainer |
| PTY tests | `scripts/terminal-smoke.py:18–64`; `service-bus-explorer/scripts/terminal-smoke.py:21–88`; `log-viewer/scripts/terminal-smoke.py:23–76` | Three PTY/resize/decode/send/wait clusters, distinct frame stability/clipboard/fixture env | Share bounded session lifecycle as needed to exercise common contracts in real apps; not a substitute for interaction delivery |
| Accessibility | B `service-bus-explorer/internal/theme/theme_test.go:13–53,81–104`, README:121–125; F `view.go:361`, icon option; L `view.go:28–31` | B contrast ≥4.5:1 tests and ASCII glyph path, F textual cursor/icons-off, L focus dot. PTYs force color; no screen-reader proof | Require keyboard parity, non-color focus/disabled/busy states, contrast test reuse, explicit manual limitations |

## Current mismatches to resolve deliberately

- B help at `inspector.go:286` and loaded status at `inspector.go:492` say Ctrl+P
  copies a report; normal `model.go:306–308` opens palette, reader help consumes
  it. Revised shared help makes palette/report distinction explicit.
- L `?` reports help unavailable; new Help/F1 must be real, not merely restyled.
- F row click currently opens; revised row click selects/updates preview, with
  explicit Open/Inspect or Enter for activation. This is an intentional interaction
  migration, not a claim that existing behavior is defective.
- B `reviewKey` checks expiry before accepting action; below 80×24 (or body-only
  view) it refuses hidden confirmation. New mouse must use the same checked path,
  not call an operation directly. Pointer events must not bypass lifecycle checks.
- L Vim filter first Esc exits INSERT; second leaves draft. Shared close/cancel
  button requests explicit draft cancellation, while keyboard Esc keeps this
  documented local submode. Do not normalize away data-entry safety/grammar.

## Boundaries supported by build evidence

`scripts/package.sh` cross-builds LazyFTP/MCP from root; B/L Makefiles build in
their module roots. Root tests do not traverse child modules. A shared public
package cannot live in app internals; Gate 2 must choose local module/replacement
delivery and preserve independent build contexts. No publication, consolidation
or release action has been authorized. Gate 1 does not choose packaging.

## Evidence limits

No runtime defect reproduction, app execution, before/after savings, usability
study, native macOS verification, terminal mouse compatibility matrix or
screen-reader test is claimed. The HTML studies and style board are authored/
generated design references, not screenshots of running terminal apps. Later
verification requirements and success thresholds are in interaction-design.md.

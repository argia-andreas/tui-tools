# Revised interaction proposal — research inputs, not approved Gates 2–4

Version 2 replaces the utilities-only recommendation. Gate 1 asks for approval of
the user experience in `01-product.md` and the HTML studies. The API sketches and
sequence here make that direction concrete, but do not pre-approve architecture,
final signatures, file changes or implementation slices.

## Recommendation: a small component family with shared semantics

Use **ActionButton/ActionBar, CommandPalette, Surface and Navigation** as the
smallest coherent family. They own appearance and input interpretation, not just
formatting helpers. Each returns semantic requests; the application decides
whether and how to fulfill them. Actions, Help, palette rows and surface buttons
must not have independent key/mouse handlers that disagree about availability.

One common component implementation can serve all three applications without
making their workspaces alike. Cells/geometry, semantic styles and a bounded PTY
session are supporting tools. Domain renderers remain independent.

## Concrete interaction contract

| Component | Shared look/behavior | App-owned inputs and responsibilities |
| --- | --- | --- |
| ActionButton / ActionBar | Compact square `[Label Key]`; default/focused/disabled/busy/danger states; one activation on left press, ignore release/motion; reason remains available for disabled items; click and keyboard emit same ActionID | Action label/shortcut/context/reason, what the request means, final availability and safety recheck; layout places the bar |
| CommandPalette | Same surface/query/result/shortcut/reason/footer structure; Ctrl+P opens/toggles closed when allowed; arrows/Tab forward, Shift+Tab backward, clamp ends; text owns printable keys; Enter or result click activates; no-results/unavailable stays open; Esc/outside press closes consumed | Current authorized/applicable actions, local keywords, context explanation, action execution and target identity |
| Surface | Same title/close affordance, body/error area, cancel-first footer, focus outline and busy state; captures pointer/key input; Escape/close/Cancel all request same domain cancellation, with blocked/busy reason; overlay and docked mounts | Whether cancellation permitted, exact validation/authentication, operation lifecycle, lock expiry, release, pending writes, draft discard and domain copy |
| Navigation | Visible focus dot + border, selected-row marker separate from multi-select, click pane focuses; data row click selects/updates preview only; Enter or labelled Open/Inspect requests activation; click explicit nav destination opens; Tab/Shift+Tab follows visible order | Ordered visible panes/controls, selected stable identity, domain row content, primary activation and preview; local Vim/reader grammar |
| Help / feedback | `[Actions Ctrl+P] [Help F1]` everywhere, same help frame and grouped key/description rows; F1 preserves current surface state; disabled/blocked reasons never color-only; status location consistent within component | Reader-specific multi-key topics, redacted/private text, outcomes and their duration/severity; no generic automatic success toast |

### Activation details and explicit behavior deltas

- Data-row selection and navigation-entry activation are intentionally different
  labelled roles. No double-click requirement. F file click changes from open to
  select-first; its checkbox marker still only toggles selection. The explicit
  Open/Inspect affordance is required so mouse users can actually activate rows.
- Palette selection is an action choice, not a data row: clicking its row requests
  the action immediately. Destructive IDs mean **open review**, never perform the
  operation. Disabled rows may become highlighted to explain why; they cannot run.
- Same matching rules across apps: fuzzy labels first, keywords fallback only
  when no label result; current candidate ordering stable. Show applicable but
  temporarily unavailable actions with reasons. App omits unauthorized/irrelevant
  actions; common discoverability never reveals hidden capabilities/data.
- Standardize clamped palette selection and Ctrl+P toggle; B loses its palette
  Ctrl+P-up alias, F loses wraparound. Retain F Ctrl+N/K aliases only as documented
  optional aliases; core behavior never depends on them. Gate 3 should either make
  aliases shared or leave them explicitly out, not accidentally promise parity.
- Actions and Help stay reachable at all supported sizes. At narrow widths hide
  secondary bar actions into palette, never retain invisible hitboxes or tab stops.
  The shared component can reduce decoration/rows; app still owns panel breakpoints.
- Wheel scrolls only the targeted visible list/reader/palette; inside a capturing
  surface it cannot scroll background. No requirement for drag/drop, text-range
  mouse selection, right-click menus, hover-only affordances or touch behavior.
- Tab order covers component controls within their active scope and app-supplied
  pane order, not every data row. Click-to-focus and keyboard focus share an owner.
  App repairs hidden return focus to the visible primary pane; preserve selected
  file/message/event identities and F's remembered file endpoint.
- Help may replace the active surface body and return to it; it cannot replace a
  review with a workspace palette or lose a draft/authorization state. No generic
  nested modal stack required. Ctrl+P is blocked during safety review/text draft
  unless that surface explicitly allows it; a visible disabled Actions reason
  explains this. F1 is safe view-only help, not permission to reset the workflow.
- Ctrl+C/quit stays app-owned because cleanup differs. Help must accurately state
  whether it cancels an operation, quits, or is consumed by an editor. This is a
  deliberate safety exception to common navigation, not a hidden global shortcut.
- L INSERT Esc remains mode-specific. Explicit close/Cancel means cancel draft;
  first keyboard Esc can mean exit INSERT, with NORMAL/INSERT visible. Shared
  chrome must not flatten specialized text grammar or typed authorization.

## Proposed API vocabulary and concise examples

Not production code and not finalized signatures. Prefer a single UI package
with these cohesive component types over separate registries, routers and plugins.
Applications keep the outer Bubble Tea Model/Update; components consume UI input
and return intents without acquiring connections, performing writes or owning
application goroutines.

```go
type Action struct {
    ID, Label, Keys, Keywords, Unavailable string
    Busy, Dangerous bool
}
type Intent struct {
    Kind string // activate, select, focus, dismiss, help
    ID   string // action, row or focus-target ID
}
type Result struct {
    Consumed bool // true with no Intent still captures the event
    Intent   *Intent
}
type Rect struct { X, Y, W, H int }
type Region struct { ID, Role string; Bounds Rect }
type Rendered struct { Rows []string; Regions []Region }

// Same returned geometry drives display and pointer handling.
func RenderActionBar(actions []Action, bounds Rect, focusedID string, style Style) Rendered
func HandleActionBar(msg tea.Msg, frame Rendered, actions []Action) Result

type Palette struct { Query, SelectedID string; Offset int }
func (p Palette) Render(actions []Action, bounds Rect, style Style) Rendered
func (p *Palette) Handle(msg tea.Msg, frame Rendered, actions []Action) Result
```

Action examples are local descriptions, not callback closures:

```go
// All three share the same action/palette/button contract.
fileDelete := Action{ID: "review-delete", Label: "Delete selected file…", Keys: "D", Dangerous: true}
messageSend := Action{ID: "review-resend", Label: "Review selected message resend…", Dangerous: true}
logFilters := Action{ID: "filters", Label: "Edit source filters…", Keys: "f"}
// The app constructs/revalidates Unavailable on every input/async change.
```

Buttons and palette emit an action ID, never simulate the action's key. The app
resolves the current target and eligibility, then opens the existing file confirm,
broker review, or log draft. During a review, it retains the prepared/frozen
domain target; a later list refresh cannot substitute another one.

```go
type SurfaceSpec struct {
    ID, Title, CancelID, DefaultActionID, CancelBlocked string
    Mount string // overlay or docked; same chrome/interaction semantics
    Actions []Action
}
type Surface struct { FocusID, ReturnFocusID string; Help bool }
func (s Surface) Render(spec SurfaceSpec, body []string, bounds Rect, style Style) Rendered
func (s *Surface) Handle(msg tea.Msg, spec SurfaceSpec, frame Rendered) Result

type FocusTarget struct { ID string; Bounds Rect }
func NextFocus(visible []FocusTarget, current string, backwards bool) string
func HandleRows(msg tea.Msg, visibleRows []Region, selectedID string) Result
```

SurfaceSpec has presentation fields, not `Run()`/`Confirm()` callbacks or lock
state. File delete can mount overlay with Cancel default; B mounts a shared
surface inside its inspector with Cancel default and app-owned body/authorization;
Logs mounts a draft editor with Cancel/Apply. A busy/expired B review can disable
submission and cancellation independently as its existing lifecycle dictates.
Mouse close never means “discard lock” or “send now.” Inputs/text editors remain
app-owned content regions whose typing is routed inside the captured surface.

Pseudocode ownership (illustrative; runtime not changed):

```text
process app lifecycle messages first (including expiry/completion)
recompute authorized actions + visible geometry for current state
if active surface:
    offer input to its focused local editor or shared controls
    consume all remaining pointer/keyboard input in that scope
else:
    palette > action bar > app navigation/row regions > local keys
if intent:
    app rechecks current capability, target, generation and review permission
    app handles intent through existing checked domain entry point
```

Do not cache clickable regions from a previous terminal size. Rendering and input
must use the same layout snapshot or recompute it from identical current state.
Preserve selected ActionID across refreshed choices when present; if it vanishes,
move to a safe visible selection and do not auto-activate a neighbor. The sequence
from pointer press → layout change → pointer release must produce only one request.
Stable row ID is app-specific identity, not a visible-row index or shared schema.

### Consumers and ownership

- **All three:** ActionBar/Button, Palette, Surface chrome/capture, Help, visible
  focus/row contract and semantic styles. This is a product commitment, not an
  optional third consumer left behind indefinitely.
- **F:** toolbar, picker/wizard/source buttons, file rows, delete/rename/copy/queue
  surfaces. Reuse checked handlers; stop synthesizing arbitrary keys for clicks.
- **B:** navigation, dashboard/resource controls, message selection/Inspect,
  reader Back/Resend, review actions and full-load prompt. Local handlers still
  own exact logical text selection, read epochs, locks, unknown/partial outcomes.
- **L:** workspace saved-search navigation, log rows/Inspect, field expansion,
  filters/settings/saved-search editor controls and palette. Redaction/query
  validation, provider scope, source budgets and field-copy policy stay local.
- Shared UI owner maintains appearance, event contracts, hit geometry and contract
  tests. App owners maintain layout/authorization/domain entry points and paired
  mouse/keyboard scenario tests. Assign the actual maintainer at Gate 2.
- Three Go modules remain. Gate 2 compares small public local module/replacements
  with a public root UI package; verify existing root cross-packager and child
  builds, not just a go.work. No broad config system or module merge is assumed.

## Options and tradeoffs, revised

| Option | User-visible outcome | Cost / tradeoff | Decision |
| --- | --- | --- | --- |
| Separate code + visual guidelines | Could match initially, but three mouse/palette implementations drift | Low initial coupling, repeated fixes and behavior audits | Temporary migration state only, not desired end |
| Utilities only (v1) | Cell widths/colors agree; clicking/discovery can still differ or be missing | Small diff, fails revised product goal | Superseded |
| Small shared interaction components + local app adapters | Same buttons/palette/surface/navigation language and click-key parity, different workspaces | Modest package and explicit interaction migrations; common contract must remain narrow | Recommend |
| Shared workspace/router/inspector framework | Uniformity extends to domain structures unnecessarily | Broad migration, callback safety rules, config explosion, layout constraints | Reject |

Net LOC reduction is useful but not a veto on adding missing mouse support. That
feature necessarily adds tests and integration code. Measure duplicated control
logic separately from total application size. Shared contracts are warranted by
the approved coherence goal, not only preexisting identical implementations.

## Candidate migration sequence (not Gate 4 slices)

1. **Approve experience and mockups, then complete Gates 2–4.** Record changed
   palette aliases/row activation and preserved app exceptions. Baseline existing
   tests and keyboard tasks; B/L mouse task baseline is explicitly unsupported.
2. **Visible tracer after approval:** ActionBar + CommandPalette in LazyFTP and
   Logs, including pointer activation and same keyboard intents. Establish shared
   styles, region geometry and conformance tests in service of that feature,
   not as a long utilities-only phase. No destructive execution from buttons.
3. **Complete palette/buttons in B; shared Help in all.** Mouse-enable B/L only
   alongside usable event routing. Do not advertise complete mouse support until
   navigation/editor/review paths also work. Correct copy hints with the new help.
4. **Surface family across all apps:** F confirmation/preview/picker/wizard,
   B docked review/full-load/help, L draft editors. Prove close/blocked/busy,
   validation and restoration via both input methods before broader navigation.
5. **Navigation and app-local controls:** common focus/row semantics, explicit
   Open/Inspect, field/tree expand, saved-search selection and app controls. Apply
   F select-first mouse change deliberately. Review compact states and hidden
   controls; retire parallel local handlers as each surface migrates.
6. **Finish the coverage matrix:** keyboard/mouse parity, PTY reports, resize,
   real terminal compatibility/fallback and visual inspection in each app. Remove
   duplicated render/key/hit logic, not domain state machines. Any incomplete app
   is unfinished scope, not a successful optional non-consumer.

Every phase ends in a working user interaction; no layout migration is required.
Rollback one consumer integration independently while retaining local domain
handlers. Later Gate 4 must turn this order into reviewable vertical slices and
obtain approval; this list is not permission to begin them.

## Risks and exclusions

- **Mouse can bypass safety if wired to effects.** Route semantic requests through
  existing checked entry points, including expiry and visibility gates. Reject
  hidden/disabled/out-of-bounds targets; fake expired lock cases must stay refused.
- **Terminal support and native text selection.** All-app support means terminals
  with mouse reporting; raw byte PTY tests are necessary but insufficient across
  native terminals/tmux/SSH. Keep keyboard complete, document terminal-native
  selection modifiers and provide an explicit mouse-off escape hatch if reporting
  conflicts with native copy. Do not promise arbitrary original-text mouse range
  selection or alter clipboard transport as part of this work.
- **State loss through shared chrome.** F1 and close requests cannot reset a draft,
  lock or prepared target. Busy/blocked reasons must be visible and identical for
  mouse/keyboard. Help is view-only, not a generic workflow transition.
- **UI consistency can become config-driven framework.** Surface mounts may differ;
  controls/semantics do not need dozens of style flags. Reject shared schemas for
  files/messages/logs, page loaders, cursor lifetimes or error state machines.
- **Stale click geometry/action identity.** Resize/async change while open must
  invalidate hit regions and update action eligibility; preserve ID where possible.
  No hidden duplicate actions from release, paste or synthetic-key recursion.
- **Accessibility regression.** Do not depend on pointer, hover, color or Nerd
  Fonts. Contrast/theme/ASCII tests and keyboard parity supplement, not establish,
  screen-reader compatibility. Status is honest text, not a transient color flash.

## Visual and mockup guidance

Recommend compact square controls over the roomier study: terminal height is
scarce, especially for B review and L filter validation. All components use semantic
roles, explicit focus dot/selection marker, plain-text unavailable/busy labels and
one active input owner. App chooses panel placement and palette/theme values;
shared widgets map those roles consistently rather than hardcode Midnight.

Required reference states per app: default/select-first workbench, palette with
selected+unavailable result and reason, app-specific Surface using shared chrome,
and compact state with visible Actions/Help/close affordances. F review is overlay,
B review docked, L draft editor can be full content. Keep source/target/warning/
validation visible before decorative spacing. Resize below an app's safe operation
threshold disables/refuses confirmation; shrinking shared chrome never overrides it.

The generated A/B board is illustration only: it invented “Open in new pane”/
Ctrl+Enter and a universal “Delete file” example. Those are NOT proposed actions
or domain screens; authored HTML and written contracts are authoritative. Do not
copy generated shortcuts. No screenshot or mockup proves terminal behavior.

## Success criteria and validation

All numbers below are proposed acceptance thresholds, not measured outcomes.

| Goal | Required evidence |
| --- | --- |
| All-app mouse delivery | In EACH app, 100% of the agreed visible control inventory supports click: Action/Help, palette results, navigation entries, data-row selection, explicit activation, applicable editor controls/close/apply, review buttons. Inventory hidden/disabled/unsupported cases explicitly |
| Mouse/key parity | Every inventoried control has a keyboard route to the same semantic request/outcome; paired scenarios check target, focus, state changes and side effects, not just text |
| Cross-app task coherence | ≥90% completion separately in each input method over 5 participants × 3 apps; record per-app failures and counterbalance order. Compare baseline, mark unsupported mouse tasks rather than excluding them |
| Common visual language | All three use shared component implementation for buttons/palette/surface chrome and navigation contract; inspect default/focus/disabled/busy/no-results/error/compact states against authored references. App themes/layouts may differ |
| Safe input ownership | Zero background activations from outside dismissal, zero duplicate action per press/release pair, zero hidden/disabled confirms; one consuming owner per key/pointer event; async expiry/completion still handled |
| Domain preservation | Existing cancel/authorization defaults, exact targets, frozen B review choices, unknown/partial outcomes, F endpoint, L query/redaction/budget and editor grammar preserved except explicit UI deltas |
| Developer simplification | One component action interpretation and geometry implementation per family, all three consumers; no parallel local button/palette key/hit handlers. Count net duplicated render/routing/setup reduction including adapters; target ≥50% less duplicated PTY setup if helper is extracted |
| Accessibility/fallback | All tasks keyboard-complete, focus/disabled/error/busy have textual cues, intended component foreground/selection pairs ≥4.5:1. Validate color-off/ASCII and mouse-off fallback; record screen-reader/native-terminal limits honestly |

Future validation after approval: `go test ./...` and `go vet ./...` in each module,
existing offline PTY suites plus shared contract scenarios, isolated preferences,
synthetic terminal SGR press/release/wheel reports and actual keyboard bytes.
Inspect representative frames and record pointer/key interactions in all three
running terminal apps; exercise neighboring controls, not only the target button.
No real broker/filesystem mutations are needed to prove shared controls; use
fixtures/fakes for destructive requests and dedicated disposable labs only where
domain integration genuinely changes and is authorized.

Discriminating cases: click unavailable palette row versus enabled neighbor;
press at old row then resize/async removal before release; outside dismissal over
a destructive background button; confirmation hidden at 79×24/80×23 versus visible
80×24; expired B lock before click; duplicate press/release; text `q`, `j`, `?`,
Ctrl+P in editor; F1 then Back with typed authorization/draft intact; L first Esc
INSERT→NORMAL; selecting F directory must not open until Open/Enter. Confirm exact
outcomes, not mere absence of crash. Combine widths 60/80/90/110/120/122/140 and
B content98/review-height24/35 boundaries with Unicode/ANSI-safe cell bounds.

Gate 1 approval covers this intended experience and changes to interaction
contracts; it does not approve final APIs, packaging, implementation or shipping.

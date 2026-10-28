# FTP input inventory — shared-terminal-ui slice 6

This is the app-owned inventory, not a claim of live-backend or human usability
acceptance. Geometry is recomputed on every input from the current layout; row
IDs encode pane, native remote/path and entry name (not the sanitized label or
cursor index). Jobs encode source and destination identity. Release/motion cannot
repeat an activation. Text entry and arbitrary text-range selection are different:
click focuses an editor; typing edits it. Drag/drop and arbitrary range selection
are not implemented controls.

All workspace panes precede controls in Tab order. Shared `HandleNavigation`
interprets workspace input, while existing shared Surface/Palette/Help handlers
retain capture. FTP owns execution, bounds, endpoint memory and storage checks.
Tab/Shift+Tab and Enter are the keyboard route for visible controls without a
dedicated shortcut. Control focus is highlighted/underlined; shared buttons also
have the shared textual focus marker. Narrow panes omit whole toolbar labels
rather than displaying a partial active control; the palette retains the action.

| Screen / control | Keyboard route | Pointer target | Enabled / hidden conditions | Representative scenario |
| --- | --- | --- | --- | --- |
| Workspace Actions / Help | Ctrl+P / F1 or ?; Tab+Enter | Header buttons | Actions unavailable during filter/review; Help retains parent | PTY palette, Help/return and capture |
| Demo Nav / Info / Jobs | `[`, `]`, `t`; Tab+Enter | Header buttons | Demo toolbar only; live uses Actions | Model keyboard/mouse and compact resize |
| Left/right pane focus | Ctrl+H/L, Tab/Shift+Tab | Pane title / empty body | Always when workspace fits 60×12 | Directional shortcuts; pointer title retains cursor |
| Navigation pane focus | Tab/Shift+Tab | Sidebar title / empty body | Width ≥90 and Nav enabled | Hidden-focus repair on resize |
| Navigation destination | Up/Down/j/k then Enter; Tab+Enter | Location row | Only visible sidebar rows; source-change queue check still applies | Demo Blob destination opens remembered right endpoint |
| File/directory/parent data row | Arrows/j/k highlight; Enter activates | Row highlights **only** | Current visible row window; no stale/index target | Select src, explicitly Open; click `..`, then Enter |
| Mark checkbox | Space; Tab+Enter on mark | First two row cells | No checkbox for `..`; no mark on failed/loading listing | Mark neighbor without opening or changing delete target |
| Explicit Open/Inspect | Enter from file pane; Actions → Open highlighted entry; Tab+Enter | Pane Open/Inspect label / palette result | Whole label only when it fits; palette provides compact route; empty pane is no-op | Directory opens; file inspector opens without stealing file focus |
| Parent | Backspace/h; Tab+Enter | ↑ Up | Attached pane; no-op in detached pane | Parent during refresh retains listing generation guard |
| Filter field | `/`, type, Backspace | / Filter label | Attached pane; input owns printable shortcuts | Filter Unicode names; no workspace `q` leakage |
| Filter Apply / Clear | Enter / Esc | Footer Enter apply / Esc clear | While filtering, fully visible footer text only | Apply preserves filter; Clear restores rows |
| Refresh / retry listing | Ctrl+R; Tab+Enter | ↻ Refresh | Attached pane; detached refuses | Stale completion after navigation cannot replace listing |
| Detach | Actions → Detach; Tab+Enter | Pane header Detach | Live attached pane only; current queue gate rechecked | Opposite pane location/filter/marks retained |
| Detached Local files / Saved connection / New connection | Arrows/j/k+Enter, Actions, Tab+Enter | Centered source buttons | Visible body rows only; live source-change gate | Both panes can choose independent sources |
| Inspector focus / bounded Preview | Tab / p | Inspector title/body; p / Actions → Preview | Width ≥120, inspector enabled; preview checks reader, listing and file | Inspector retains endpoint; explicit bounded content read |
| Jobs focus / row selection | Tab; Up/Down/j/k | Drawer title/body; job rows select only | Drawer enabled; current queue window | Wheel over jobs leaves file cursor and endpoint unchanged |
| Jobs Stop / Retry | Ctrl+X / r; Tab+Enter | Drawer Ctrl+X stop / r retry | Queue hint visible; retry opens existing checked review | Unfinished queue cancellation and retry guard tests |
| Live footer Mark / Copy / Delete / Help | Space / c / D / ? | Corresponding footer label | Hidden when status/filter replaces footer; operation gates rechecked | Copy honors marks; delete honors highlighted file only |
| Palette query / results / Close / Help | Type, Ctrl+U, arrows/Tab, Enter, Esc/Ctrl+P, F1 | Result activates; Close; Help; outside dismiss | Only available actions execute; captured outside press never falls through | PTY query capture, unavailable, resize, press/release |
| Help body / Close | Arrows, page keys, F1/Esc | Wheel body / close button | Parent retained; outside press captured | Retained filter/review while Help shown |
| Local folder path / Open / Cancel | Field text/Ctrl+U, Enter; Tab+Enter; Esc | Path field / Open / Cancel / Close | Existing-directory validation and connection gate | Invalid folder doesn't attach or change other pane |
| Inline rename input / Review / Cancel | Text, Enter; Tab+Enter; Esc | Filename row / Review / Cancel | Before confirmation; running rename refuses input/cancellation | Review a draft, cancel without renaming |
| Rename confirmation / details scroll | Tab+Enter, Esc; Up/Down | Cancel / Rename / Close; wheel in review body | Cancel default; started rename cannot be rolled back | Current generation and existing-name refusal tests |
| Delete review | Tab+Enter, Esc | Cancel / Delete / Close / outside dismiss | Cancel default; highlighted file frozen; directories excluded | Marked neighbor stays untouched |
| Copy checking / destination review | Tab+Enter, Esc | Current Cancel / Copy / Overwrite / Rename / Skip buttons | Checking exposes only Cancel; current conflict/busy state decides controls | Name/conflict checks remain app-owned |
| Copy destination name | Text/Ctrl+U, Enter checks; Tab+Enter; Esc | Name field / Check name / Cancel | Only rendered field owns text; invalid destination refuses | Unicode name; invalid/path traversal rejection |
| Batch planning / queue review | Tab+Enter, Esc | Cancel / Start queue / Close | Start hidden during planning; conflicts still pause | Queue guard and cancel tests |
| Preview body / Close | Arrows, page keys; Tab+Enter; Esc | Body focus/wheel / Close | Bounded read; binary/error/loading remain truthful | Body Enter does not close; Close cancels pending read |
| Connection chooser row / Open | Arrows/j/k, Enter; Tab+Enter | Row selects only; Open opens details | Current visible row window; no selected connection disables Open | Offline chooser select-only/Open PTY |
| Chooser New / Browse local / Close / Help | Tab+Enter, Esc, F1 | Body controls; footer Close/Help; Close chrome | Responsive surface hides clipped controls | Close returns to retained files, not unconditional Quit |
| Connection root / local / name / Connect / Save / Back / Help | Tab/Shift+Tab; field text; Enter; Esc/F1 | Fields and matching actions | Companion local hidden for destination chooser; Save explicit, not Connect | Reverse traversal skips hidden companion and reaches Back/Help |
| Wizard FTP/Azure/S3 / fields / options | Arrows/j/k or Tab+Enter; text, Ctrl+U, left/right options | Provider buttons; fields focus; option control cycles | Current step/visible rows only; edited details invalidate tested candidate | Fixture wizard provider/option/secret editing |
| Wizard Test / Remember / Connect / Back / Help | Tab+Enter; Space on Remember; Esc/F1 | Matching body/footer controls; Close means Back | Test read-only; Connect requires verified candidate; Remember explicitly saves; busy allows Back/Help | Candidate invalidation, late completion, save/connect separation |

## Verification and live-script inspection

Run from this app root with `GOWORK=off`: `go test -race ./...`, `go vet ./...`,
`go build ./...`, and `make test-terminal`. PTYs launch real binaries against
fixture/demo and private local files; no remote credentials are needed.

Inspected `scripts/terminal-live.py`, `terminal-rename.py` and
`terminal-panes.py` for row-open expectations. Live/rename navigate file rows
with keyboard; panes clicks source/pane-title/chooser action controls, not file
rows. Their existing clicks do not need fake Enter additions. Updated their
button-label expectations to shared chrome and changed endpoint-return Tab
assumptions to explicit Ctrl+H/L, since controls now participate in traversal.
**Do not run these
live scripts without separate disposable-lab authorization.**

Graphical review recipes: `bin/lazyftp --demo --icons=false` at 160×40 for Nav,
Open/Inspect, Info and Jobs; 80×24 and 60×12 for compact controls/Actions fallback.
For offline source/chooser/wizard/rename/delete/preview states, follow the private
local-files fixture in `scripts/terminal-smoke.py` (`surfaces`): isolated
rclone config and bookmark files plus an asymmetric source file. Do not reuse a
real user connection file. Model fakes cover checking/running/conflict states
without live mutations. No live lab or native-terminal accessibility acceptance
is implied by these engineering scenarios.

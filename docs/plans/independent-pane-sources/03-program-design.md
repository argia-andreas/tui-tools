# Program design: Independent pane sources

## Files
- `internal/ui/model.go`: per-pane attachment state, symmetric message handling/navigation/actions.
- `internal/ui/destination.go`: pane-targeted host messages, detached chooser and local-folder prompt.
- `internal/ui/view.go`, `mouse.go`: actual source labels, Detach control, shared rendering/hit geometry, targeted inspector and folder modal.
- `internal/ui/transfers.go`, `preview.go`, `editor.go`: detached guards and stale-completion protection.
- `cmd/lazyftp/workspace.go`: retain pane context through chooser and wizard; startup-only companion folder; append-only backend snapshot ownership.
- `cmd/lazyftp/picker.go`, `wizard.go`: target-pane headings and pane-specific fields/actions; existing startup details remain compatible.
- `internal/connections/connections.go`: allow empty bookmark Local, preserve absolute validation when nonempty.
- Existing UI, host/picker/wizard/bookmark tests: observable independent behavior, transition matrix, snapshot continuity, safety.
- `scripts/terminal-panes.py`: retained PTY acceptance for symmetric real sources, keyboard/mouse, error/cancel/resize and copy safety; existing smoke scripts remain regression checks.
- `README.md`, `docs/connections.md`: update pane targeting and detach semantics.

## Types & signatures
```go
// Existing options keep Disconnected for initial right-pane compatibility.
type pane struct {
    // existing location/listing/generation fields remain
    detached bool
    sourceChoice int
    lastLocal string
}
type ConnectRequested struct {
    Local string
    Destination bool // true: in-workspace, false: startup compatibility flow
    Pane int         // 0 left, 1 right
    New bool         // enter existing wizard directly
}
type OpenLocalFolderRequested struct { Pane int }
type ConnectionOpened struct {
    Pane int
    Location storage.Location
    Local string // optional STARTUP-only left companion
    Locations []storage.Location
    Browser storage.Browser
    Transferer storage.Transferer
}
func (m Model) destinationPane(i, w, h int) []string
func (m Model) connectionDisabled() string
```

## Call stack
- Keyboard/mouse/palette → `execute(detach/folder/connect/new)` → shared connection guard → pane-local transition or `ConnectRequested`.
- Workspace `Update` → chooser context retained in host → `updatePicker` or wizard completion → `ConnectionOpened` → UI `navigate(target)`.
- Sidebar `Enter` → guard → `active()` (remembered pane) → `navigate`.
- Detach → increment generation, cancel preview/increment preview generation, clear only target → old listing/preview ignored.
- Copy/retry → existing captured locations → unchanged serialized queue and confirmation paths.

## Test plan
- Every pane × initial local/remote × replacement local/remote: detached choice, attachment and opposite state preserved.
- Both panes detached: no invisible listing, navigation, preview/edit/copy or refresh; no stale results repopulate them.
- Saved/new target survives details, cancel/back and late discovery/test results; non-startup bookmark local never moves opposite pane.
- Startup bookmark companion/explicit left override and startup Esc remain compatible; no-local bookmarks work.
- Sequential session-only connections preserve prior remotes; detach one does not break other/retained jobs; duplicate names rejected.
- Sidebar opens either kind in either last-focused pane; busy queue blocks all source changes.
- Remote-left/local-right upload/download and local/local copying retain byte/conflict behavior; remote/remote rejected without writes.
- Real terminal: selection, detach, each source choice, saved/new heading, invalid folder, Esc, both detached, two remotes, narrow resize, pointer hit regions and focus.
- Run `make check`, race tests, both existing terminal scripts, new pane PTY script, and real FTP/Blob/S3 integration against disposable lab.

## Least confident decisions
1. Append-only shared snapshot avoids endpoint wrappers; explicitly test prior session-only remotes survive a second wizard and retry. Do not permit remote-name replacement.
2. Startup-only companion local behavior is intentionally different from later pane choices, preserving current launch contract.
3. Source switching from sidebar is now busy-guarded, unlike ordinary folder navigation; captured jobs remain unchanged.

## Approval
User waived additional pauses for this approved scope, 2026-10-24.

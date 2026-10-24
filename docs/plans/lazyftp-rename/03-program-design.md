# Program design: LazyFTP file rename

Gate pauses waived by the user on 2026-10-24.

## Files
- internal/storage/storage.go and rename implementation/tests: optional file rename contract and local/rclone behavior.
- internal/ui/rename.go and tests: operation state, inline Unicode editing, confirmation, async completion and refresh.
- internal/ui/model.go: dispatch, action registration and post-list focus restoration.
- internal/ui/view.go: inline editor, confirmation and shortcut guide.
- internal/ui/mouse.go: confirmation buttons, scrolling and dismiss; prevent navigation through the inline operation.
- README.md and terminal checks: user contract and real-terminal acceptance.

## Types & signatures
```go
type Renamer interface {
    Rename(context.Context, Location, Location) error
}
```
UI state captures source, originating pane, editable rune buffer/cursor, confirmation scroll and confirming/running flags. Completion carries source, destination and error. Existing lowercase-r retry and copy-conflict rename keep their existing contracts.

## Call stack
Update → startRename → renameKey → Renamer.Rename → renameFinished → finishRename → list matching panes → listed restores focus.

## Test plan
- Both panes rename only the highlighted file, never a marked batch; stored bytes preserved.
- No writes after inline Enter alone, default Cancel, Escape, outside-click or unchanged name.
- Text editing handles Unicode/spaces/navigation/delete; workspace shortcut letters remain text.
- Invalid filenames and path input remain editable and do not write.
- Existing names, including identical bytes, never intentionally overwrite; errors refresh and permit retry with a fresh confirmation.
- Lowercase r still retries failed/canceled transfers; active transfers block rename.
- Read-only demo, directories and disconnected pane explain unavailability.
- Minimum/normal terminal sizes and long names: confirmation scroll and no layout overflow.

## Least confident decisions
Remote backend move semantics and race windows need actual lab verification. Started operations intentionally finish rather than promising cancel/rollback; timeout/failure must describe uncertain outcomes. Case-only rename on case-insensitive storage may be refused as an existing destination, prioritizing preservation.

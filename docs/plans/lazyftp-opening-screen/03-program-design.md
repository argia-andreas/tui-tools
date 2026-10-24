# Program design: LazyFTP opening screen

User waived further approval pauses for this scope on 2026-10-24.

## Files
- `cmd/lazyftp/picker.go`: continuous chooser movement, viewport, loading state, aligned opening rows, safe compact rendering. Preserve existing details actions and editing.
- `cmd/lazyftp/workspace.go`: identify loading, retain interacted-with action/help selection on discovery completion.
- Existing `picker_test.go` and `workspace_test.go`: observable keyboard/mouse routing, boundary traversal and late discovery tests; adjust previous Tab-section expectations to the newly approved Tab alias behavior.

## Types & signatures
Keep existing picker/row/layout types and tea.Model methods. Extend picker with scroll offset, loading and navigation-interaction state. Extend row metadata only for muted endpoint text, not a new component system. No external API, new dependency or new persistence format.

## Call stack
- Key/wheel → picker.Update → move → choose if on an entry → View using shared layout rows.
- Enter/click → existing details/setup/local action → workspace.updatePicker.
- Discovery → connectionsLoaded → retain chosen action if user navigated → redraw without side effects.

## Test plan
- Visit 0/1/3/12 entries and both actions using Up/Down and j/k; Enter on each route has the expected observable outcome, no wrap, no implicit activation.
- Tab/Shift+Tab are equivalent chooser aliases, while detail fields still accept literal j/k.
- Reverse across both actions into the last scrolled entry; help and details restore the selection.
- Mouse and wheel reach the same actions; separators do nothing.
- Late discovery keeps a user's selected action and help state, but chooses the first entry on untouched startup.
- Existing saved-folder, explicit save, Connect and local-destination controls remain covered.
- Real PTY checks at 60×21, 80×24 and larger sizes; rendered-terminal captures of default, both actions, empty, long, compact, loading and failure. Real pointer/keyboard clip of crossing and activation.

## Least confident decisions
Long names must truncate without moving gutters or obscuring actions. Keep widths measured in terminal cells, not bytes. At minimum height, the opening card and help must not collide with header/footer. Existing details styles remain untouched.

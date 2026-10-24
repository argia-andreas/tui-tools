# Product: A calmer LazyFTP opening screen

## Problem
The first screen looks uneven, and moving down the saved connections unexpectedly stops before New connection and Browse local files. I want one clear place to choose where to work, without learning that Tab switches to another section.

## Success metric
For 0, 1, 3 and 12 saved entries, 100% of entries and both actions are reachable with Up/Down and Enter alone, with zero Tab presses. A complete down-and-up traversal visits every choice in visible order, without skipping or activating anything. This is a task-success measure, not an invented revenue metric.

## Announcement — the blog post before the feature
LazyFTP opens with a calmer, aligned connection list and two clear ways to begin. Move up or down through saved connections, New connection and Browse local files as one uninterrupted menu. The current choice is always visible, even with a long list or a smaller terminal. Opening a saved connection still lets you review its folders before connecting, and creating a connection never silently saves credentials.

## Proposed experience
- Preserve the centered square-corner card, Midnight colors, breathing room and small square-tile logo. The logo keeps a fixed footprint; only its accent tile may animate. It is decoration, never a loading or authentication indicator.
- Put the logo beside the title, rather than spending several otherwise empty rows above the list. Use a common label edge and single-cell marker gutter for every entry and action. Remove multi-row pixel icons from action rows. New connection remains easy to find through its plus sign and label, not a permanent selection-like fill.
- Use the honest heading “Saved & configured”: the chooser includes both bookmarks and configured remotes. Show a name and muted remote/root when available. An unnamed remote uses its remote name; do not repeat the same name as redundant metadata. Do not invent backend type, availability, last-used time or connection health.
- Keep the two action rows immediately below the list viewport, separated by one quiet rule. Headings, separator, path metadata and empty-state text are not focus stops. There is exactly one highlighted choice and one selection marker.
- Initial selection is the first saved/configured entry, or New connection when there are none. Existing ordering is retained; do not introduce recent-first sorting or deduplication in this visual redesign.
- Down advances through every saved/configured entry, then New connection, then Browse local files. Up reverses exactly that route. Clamp at the first and last choices: no wraparound. j/k remain equivalent to Down/Up only on this chooser, never inside editable fields.
- Tab/Shift+Tab are optional aliases for next/previous choice, using the same order and boundaries. They are not required and do not switch sections. Home/End and new letter shortcuts are not added in this proposal.
- Enter on an entry opens its existing connection-details screen; it does not immediately connect. Enter on New connection opens existing guided setup. Enter on Browse local files opens the local workspace. Esc returns to files; Ctrl+C quits. F1/? opens help, and dismissing it restores selection.
- Returning from connection details or canceled guided setup restores the launching row and scroll position. Moving the highlight performs no network operation, connection, save or local-folder change.
- Mouse clicks retain the existing open behavior and target the same logical rows as keyboard input. Wheel movement follows the same selection order; separators and padding never activate. There is no separate mouse-only selection.

## Screen states
- Default, 80 × 24: three illustrative entries, first selected; names and remote/root share a line when space permits.
- Boundary: last entry → New connection → Browse local files, and the reverse; same full-row selection treatment across the rule.
- Empty: “No connections yet” / “Create one, or browse files on this computer.” New connection initially selected; Down reaches local browsing.
- Many entries: scroll only the saved-entry viewport; keep actions and footer visible. Show an honest visible range such as “9–12 of 12”. Preserve the last visible list window while either action is selected; Up from New returns to the last entry.
- Compact, 60 × 21: reduce padding and visible entries before hiding essential choices; omit inline path metadata when it would compete with names. Retain full action labels and a shortened footer. Long names truncate with an ellipsis. The existing smaller-than-supported resize notice remains out of scope.
- Loading: show “Loading connections…” rather than the empty state. Both actions remain reachable. If discovery finishes after the user moves to an action, keep that action selected instead of stealing focus; if they have not moved, select the first entry when available.
- Discovery failure: show “Could not load connections. F1 for details.” Preserve any successfully loaded entries and both actions. Do not expose raw provider errors or secrets in the main list. Do not label failure as an empty account. No new retry control in this proposal.

## Screens
- `mockups/opening-screen.html`: authoritative, cell-aligned browser simulation of this one terminal screen, with default, action-selection, empty, many, compact, loading and error states. Use Up/Down inside the terminal to try the navigation; activation is explained, never executed.
- Painter comparison board: two visual directions, continuous list recommended over a separate anchored action dock. The generated board is exploratory, not the contract: its numeric row labels, inconsistent gutters, folder-like glyph and glow are not proposed shortcuts or exact styling.

## Out of scope
No changes to the file workspace, setup wizard, connection-details form, authentication, bookmark persistence, MCP permissions, other applications, or provider discovery capabilities. No implementation or claims of real-terminal verification in this design-only phase.

## Approval
Approve Gate 1, or what should change?

Approval accepts the product/layout/navigation direction only. Architecture, program design and slice planning remain separate future gates, not implied implementation permission.

# Product: LazyFTP file rename

## Problem
I can browse and copy files but cannot fix a file's name where I am working. I want to edit the name directly in either file pane and review exactly what will change before anything is renamed.

## Success metric
100% of the acceptance scenarios preserve the original file and its contents until explicit confirmation. Measure with automated interaction and storage checks across both panes and local, FTP/FTPS, Azure Blob and S3 storage; separately verify successful renames preserve exact file contents. Local emulators establish mechanics, not production cloud/TLS parity.

## Announcement — the blog post before the feature
Rename a file without leaving LazyFTP: focus it in either pane and press uppercase R. Edit its name right in the file list, then press Enter to review the old and new names before confirming. Lowercase r still retries transfers. Existing names are never intentionally replaced, and canceling before confirmation leaves your files unchanged. The same workflow is available for local files and supported remote connections.

## Screens
- `./mockups/workspace.html` — one workspace shown in three review states: inline rename in the left pane, confirmation for the right pane, and a recoverable name-conflict error. Static design reference only; preserve the existing terminal layout and theme.

## Approved product decisions
1. Uppercase R renames the highlighted file in the focused file pane, not every marked file. Lowercase r remains Retry. Expose Rename selected file in Actions, the command palette, and the shortcut guide; distinguish R from r explicitly.
2. Start with an inline editor prefilled with the complete filename. Support ordinary text input, spaces, Unicode, arrows, Home/End and Backspace/Delete. While editing, letters such as q, r and c are text, not workspace shortcuts. Keep focus on this operation until it is canceled or submitted.
3. Enter opens a confirmation showing the pane/location, old name and new name unambiguously. Cancel is selected by default; changing the selection to Rename and confirming is the only way to apply it. No second name-editing modal.
4. Escape cancels inline editing. Escape, Cancel, close or outside click dismisses confirmation without writing. An unchanged name exits quietly with no write; invalid/empty names remain editable with an explanation. A filename cannot move the file into another folder.
5. Rename is file-only and stays in the same folder on the same storage connection. Directories, parent rows, disconnected panes, read-only demo and active transfer work explain why rename is unavailable. Existing multi-selection does not expand this single-file operation.
6. Existing destination names are refused rather than overwritten or merged. On a name conflict or failure, clearly explain the outcome and allow editing/retry; each retry requires confirmation. Show progress while applying; refresh affected listings afterward and keep the renamed file focused when visible. If a filter hides it, report success with its new name rather than pretend it disappeared without explanation.
7. Before confirmation, cancellation guarantees no rename. After starting, remote rename may involve copying then deleting; do not promise atomicity or rollback. If cancellation or failure leaves the outcome uncertain, say so and refresh before offering a retry. Exact backend limitations and running-operation cancellation will be settled at the architecture gate.

## Out of scope
Folder/bulk rename, cross-folder moves, cross-connection moves, overwriting, undo, new backend support and changes to the separate MCP server. The copy-conflict destination-name workflow remains separate.

## Approval
Gate 1 APPROVED 2026-10-24. User approved the mockup and authorized implementation, testing and configured Ship delivery without further gate pauses.

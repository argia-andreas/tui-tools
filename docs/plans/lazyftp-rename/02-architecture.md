# Architecture: LazyFTP file rename

Gate pauses waived by the user on 2026-10-24 after approving Gate 1.

## Fit
The existing Bubble Tea workspace owns one rename operation, separate from copy destination editing. The existing storage adapter gains an optional Renamer contract, discovered from the browser. Read-only fixture browsers do not implement it. No MCP operations are added.

## Endpoints / Data
None. No persistence or configuration changes.

## Flow
Focused file → R → captured pane/source and inline editor → Enter validates one filename → confirmation defaults to Cancel → explicit Rename → asynchronous storage operation → refresh every pane showing that directory, retaining the renamed file's selection and focus when visible.

Input and navigation are isolated while editing/confirming/running so captured paths cannot drift. Failures refresh first; retry starts a fresh R operation and requires confirmation again. Names are escaped in confirmation, wrapped and scrollable rather than silently truncated.

## External
Existing rclone configuration and executable only. Local rename and remote moves must refuse pre-existing destinations, including identical-content files. Remote moves can be copy/delete and cannot universally promise atomic no-clobber against concurrent writers or rollback. The bounded started operation runs to completion; Esc only cancels before starting, and the progress message explains that running cancellation cannot roll it back.

## Verification
Public UI tests drive key/mouse events and inspect actual stored names/bytes. Storage tests check local and lab FTP/Blob/S3 moves and collision preservation; live PTY scenarios exercise real terminal input. Inspect actual terminal screenshots and keyboard/pointer recording. Emulators do not establish production identity/TLS parity.

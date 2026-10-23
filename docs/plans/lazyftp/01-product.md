# Product: LazyFTP

## Problem
I like TUIs, but connecting to FTP with Midnight Commander was unpleasant and buggy for me. Browsing Blob storage required Azure Storage Explorer, which I disliked. I want a dependable keyboard-first way to browse remote files and transfer them without leaving the terminal.

## Success metric
In the repeatable practice environment, complete 10 consecutive browse → download → upload round trips on each initial storage type, with byte-identical returned files and zero unconfirmed overwrites. Record elapsed time for each developer verification run so feedback-loop improvements are measurable; timing targets will follow the first baseline rather than an invented benchmark.

## Announcement — the blog post before the feature
LazyFTP lets you browse local files alongside FTP or Azure Blob storage in your terminal. Select files or folders and copy them to the opposite pane with clear progress and explicit conflict choices. Failed and cancelled jobs remain visible and never masquerade as successful transfers. A resettable practice environment lets contributors try the same workflows safely, without personal accounts or production files.

## First usable scope
- Local-left, remote-right navigation, connection selection, refresh, filtering, and keyboard help.
- Existing configured remote connections first; guided credential setup comes later.
- File and folder upload/download, a transfer queue, explicit skip/overwrite/rename conflict decisions, cancellation, and retry of unfinished work.
- Accurate per-job outcomes, including partial failure; completed files are distinguished from unfinished files.
- FTP and Azure Blob are the initial tested storage types. Virtual prefixes are not presented as ordinary filesystem directories where behavior differs.
- Changing the visible connection does not change destinations already captured by queued jobs.
- Independently collapsible left navigation and right inspector, spanning the full workspace height. The central transfer drawer never shortens either sidebar.
- Ctrl+P opens a fuzzy, context-aware action palette with shortcut hints; choosing a risky action does not bypass its confirmation modal.
- Configurable semantic colors, familiar coding-theme presets, and optional filetype icons with a plain-text fallback. No promise of arbitrary Neovim/VS Code theme-file compatibility.
- Inspector metadata follows focus; remote content previews are explicitly requested and bounded.

## Safe, quick feedback
- A repeatable practice environment with known local and remote contents, requiring no production access.
- A short automated check for each change, separate from broader real-transfer checks.
- Real-transfer checks cover nested folders, empty files, spaces and Unicode in names, conflicts, cancellation, disconnection, and retry.
- Verify returned bytes, not merely success messages. Backend-specific limitations must be reported honestly.
- Reset only explicitly designated disposable practice data, never arbitrary user locations.
- The contributor can run the TUI against the same environment used by automated checks.

## Out of scope for the first usable build
SFTP/S3 validation, remote-to-remote copies, synchronization, automatic edit/upload, rename/move/delete tools, storage administration, tabs, and an in-app connection wizard. No claim that an emulator proves production authentication, service limits, or all production behavior. No claim of cross-platform support until tested.

## Screens
- `mockups/workspace.html`: earlier static proposal, superseded visually by the sidebar and theme studies linked in `00-status.md`. All mockups are design references, not functioning applications.

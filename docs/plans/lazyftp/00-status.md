# Status: LazyFTP

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23
- Gate 3 — Program Design: APPROVED 2026-10-23
- Gate 4 — Slice plan: in progress

## Slices
- [ ] Slice 1 — Interactive fixture-backed workspace, command palette and themes; fast tests and real-terminal smoke.
- [ ] Slice 2 — Disposable FTP/Blob lab, backend spike and real remote browsing.
- [ ] Slice 3 — Single-file transfers and conflict decisions, verified by returned bytes.
- [ ] Slice 4 — Folder jobs, queue, cancellation, interruption and retry of unfinished work.
- [ ] Slice 5 — Bounded previews, edge-case coverage and alpha packaging.

## Notes for a fresh session
- The repository was empty at the start.
- The user wants a keyboard-first TUI after poor experiences with Midnight Commander for FTP and Azure Storage Explorer for Blob storage.
- FTP and Azure Blob are the first validation targets; SFTP and S3 remain later scope.
- The user explicitly requests good testing and quick feedback loops, suggesting local Docker FTP and a Blob emulator.
- Proposed architecture questions for Gate 2: Go/Bubble Tea, rclone CLI versus RC, Docker FTP and Azurite, deterministic fixtures, network-failure controls, terminal automation, and local developer commands.
- Docker CLI is installed in this orb; daemon availability is not yet checked. Go and rclone were not found on PATH.
- No implementation has begun. Product, architecture and program design are approved; the slice plan still requires approval.
- User requires design and shortcut proposals before implementation and a better product name; Tandem is only a candidate, not approved or availability-checked.
- User requests a collapsible left sidebar, exploration of an optional right file-info/preview sidebar, and confirmations in modals (interpreting the typo "morals").
- Design influences: LazyGit panel structure, Amp CLI workspace, Neovim navigation. No requirement for full Vim modes.
- Latest generated comparison: https://ampcode.com/user-content/attachments/8f56898349b71d5b69a116fd3f45e5f3936cc922256e80d1b85ac83ba4ac8d83-file.png
- That image is illustrative: inspector date and hide shortcut conflict with other labels; global transfer text is ambiguous during a per-file conflict. The implementation must not inherit those inconsistencies.
- User approved the refined direction and requested an implementation plan and first slice, with excellent local testing and mocks. Required remaining design gates still precede implementation.
- Approved direction includes Ctrl+P fuzzy action palette, independently collapsible full-height sidebars, central dual file panes and transfer drawer, configurable themes, useful filetype icons, and confirmation modals. Tabs are future scope.
- Common coding-theme studies: https://ampcode.com/user-content/attachments/f102d7162f22472d5a8be4f450da04869ccfaf1e31f5305274583de8176b1db7-file.png
- Theme compatibility means semantic theme configuration initially, not executing Neovim Lua or importing arbitrary VS Code theme files.
- Docker daemon check failed: /var/run/docker.sock does not exist. Architecture proposes equivalent direct-process FTP/Azurite services in this orb, plus Compose for machines with Docker.
- User accepted proceeding without Docker in this orb and authorized committing and pushing project changes to main. This does not authorize deployments or shared infrastructure changes.
- Gate 3 now specifies the first runnable slice in detail. Later real-storage slices must extend that design before their implementation; do not invent transfer guarantees from the demo.
- All six generated mockup boards are saved locally with an index in `mockups/README.md`, as requested. Later boards supersede earlier layout/color studies; none are implementation evidence.

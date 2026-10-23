# Status: LazyFTP

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23
- Gate 3 — Program Design: APPROVED 2026-10-23
- Gate 4 — Slice plan: APPROVED 2026-10-23; user waived further approval gates for the proof of concept

## Slices
- [x] Slice 1 — Interactive fixture-backed workspace, command palette and themes; fast tests and real-terminal smoke.
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
- First interactive fixture-backed slice implemented; final evidence and delivery are recorded below. Real FTP/Blob connections and transfers remain unimplemented.
- User requires design and shortcut proposals before implementation and a better product name; Tandem is only a candidate, not approved or availability-checked.
- User requests a collapsible left sidebar, exploration of an optional right file-info/preview sidebar, and confirmations in modals (interpreting the typo "morals").
- Design influences: LazyGit panel structure, Amp CLI workspace, Neovim navigation. No requirement for full Vim modes.
- Latest generated comparison: https://ampcode.com/user-content/attachments/8f56898349b71d5b69a116fd3f45e5f3936cc922256e80d1b85ac83ba4ac8d83-file.png
- That image is illustrative: inspector date and hide shortcut conflict with other labels; global transfer text is ambiguous during a per-file conflict. The implementation must not inherit those inconsistencies.
- User approved the refined direction and explicitly requested moving ahead with a working proof of concept, delegation as useful, and no further detailed approval gates. Continue iteratively rather than waiting for gate or slice approval.
- Approved direction includes Ctrl+P fuzzy action palette, independently collapsible full-height sidebars, central dual file panes and transfer drawer, configurable themes, useful filetype icons, and confirmation modals. Tabs are future scope.
- Common coding-theme studies: https://ampcode.com/user-content/attachments/f102d7162f22472d5a8be4f450da04869ccfaf1e31f5305274583de8176b1db7-file.png
- Theme compatibility means semantic theme configuration initially, not executing Neovim Lua or importing arbitrary VS Code theme files.
- Docker daemon check failed: /var/run/docker.sock does not exist. Architecture proposes equivalent direct-process FTP/Azurite services in this orb, plus Compose for machines with Docker.
- User accepted proceeding without Docker in this orb and authorized committing and pushing project changes to main. This does not authorize deployments or shared infrastructure changes.
- The program design specifies the first runnable slice. Keep design notes current during real-storage work; do not invent transfer guarantees from the demo or require more approval gates.
- All six generated mockup boards are saved locally with an index in `mockups/README.md`, as requested. Later boards supersede earlier layout/color studies; none are implementation evidence.

## First-slice implementation notes
- Go 1.27.3, Bubble Tea 1.3.10, Lip Gloss 1.1.0. Fixture storage and semantic theme packages were independently delegated; combined verification stays in this checkout.
- Dual-pane navigation, selection, filtering, sidebar connection switching, full-height sidebar toggles, metadata inspector, empty central job drawer, fuzzy Ctrl+P palette, help and four theme presets work.
- Demo data only: no network or disk writes; copy and content-preview actions explicitly explain that they are not implemented.
- Mouse reporting enables single-click directory navigation, marker selection, endpoint switching, panel controls, parent/filter/refresh buttons and palette actions. Scroll wheel works over lists and modals. Outside clicks dismiss modals without activating underlying rows.
- F1 opens a grouped, scrollable shortcut guide from the workspace or palette; the palette also exposes a clickable F1 link and Keyboard shortcuts action. Keyboard navigation remains supported.
- `make check`, `go test -race ./...`, `make test-terminal` and two idempotent setup runs exercised. Terminal smoke verifies actual input/resize handling and styled cell backgrounds, not just raw ANSI strings.
- `.agents/setup` installs the pinned toolchain and PTY test dependencies; warm setup measured about 0.04s after dependency download.
- A supervised one-off ttyd service provides a browser demo at the portal recorded in `.amp/portals/lazyftp-demo.json` (ignored). It launches only the read-only executable, not a shell. It is not yet a persisted repository service.
- Artifacts under `.amp/in/artifacts/` are ignored local evidence, not generated mockups. Current sources and demo commands are documented in README.
- A separate user-requested Service Bus mockup thread is complete: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1 . No Service Bus implementation belongs in this repository.

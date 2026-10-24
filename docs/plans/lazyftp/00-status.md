# Status: LazyFTP

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23
- Gate 3 — Program Design: APPROVED 2026-10-23
- Gate 4 — Slice plan: APPROVED 2026-10-23; user waived further approval gates for the proof of concept

## Slices
- [x] Slice 1 — Interactive fixture-backed workspace, command palette and themes; fast tests and real-terminal smoke.
- [x] Slice 2 — Disposable FTP/Blob/S3 lab, backend spike and real remote browsing.
- [x] Slice 3 — Single-file transfers and conflict decisions, verified by returned bytes.
- [x] Slice 4 — Folder jobs, queue, cancellation, interruption and retry of unfinished work.
- [x] Slice 5 — Bounded previews, edge-case coverage and local alpha packaging (cross-builds, not native platform or production-cloud validation).

## Notes for a fresh session
- The repository was empty at the start.
- The user wants a keyboard-first TUI after poor experiences with Midnight Commander for FTP and Azure Storage Explorer for Blob storage.
- FTP and Azure Blob were the initial validation targets; user subsequently authorized S3-compatible local testing. SFTP remains later scope.
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
- Service Bus work continues independently in its own thread and `service-bus-explorer/` subtree; FTP changes must not alter that application's files.

## Current usable-alpha work
- User authorized all five next steps plus S3 in parallel, then the FTP project's own default-deny MCP server. Verified scoped commits may be pushed to main; no deployment or publishing is authorized.
- S3 lab uses pinned standalone SeaweedFS 4.49 because the archived MinIO binary returned HTTP 410. FTP, Azurite and all required SeaweedFS listeners bind to loopback. Lab seed preserves existing bytes.
- Default startup (also `--live`) opens local files with a disconnected destination; the in-app Connect action opens the themed mouse/keyboard picker. Explicit `--right` bypasses discovery. Repeatable `--remote` adds navigation roots. Non-secret bookmarks are separate from rclone credentials. `docs/connections.md` documents this contract.
- Serial batch/folder queue freezes endpoints, plans before writes, defaults to Cancel and checks each file destination afresh. New files in a confirmed queue run automatically; conflicts pause for per-file decisions. Rename requires rechecking and confirmation. Empty directories are not created; plans cap at 10,000 files / 64 levels.
- Ctrl+X / Stop cancels planning, checks and running copies; canceled and failed work is retryable within the current in-memory queue. Completed/skipped files never retry. Partial output is possible, and retries do not inherit overwrite authorization. Drawer supports keyboard/mouse review of individual outcomes.
- Preview requests read at most 64 KiB + one lookahead byte, display at most 64 KiB, reject binary/invalid UTF-8, sanitize terminal controls and label truncation. Closing cancels; late results cannot replace a new preview.
- `lazyftp-mcp` exposes list/upload/download over stdio using the official Go SDK. Default deny; independent operation + exact remote/path-root grants and transfer local roots. No overwrites/delete/execute, no implicit permissions from interactive credentials or bookmarks. `docs/mcp.md` explains trusted staging and non-race-proof path checks.
- Actual PTY validation interrupted throttled running transfers on FTP, Blob and S3, retried unfinished files, verified exact bytes and confirmed completed files were not repeated. Kept public UI tests cover conflict/failure/retry and nested round trips; `make test-integration` includes storage, UI and MCP on all three services.
- New inspected visual evidence: `.amp/in/artifacts/alpha-*.png`, `alpha-interaction.mp4` / `.loop.webp`. These are real terminal captures, not mockups. Real Azure/AWS identity, IAM/TLS/provider behavior and non-Linux native execution remain unverified.
- Combined acceptance: `make check`, `go test -race ./...`, terminal smoke (0.64s), three-backend live PTY smoke (3.12s), and tagged storage/UI/MCP integration (29.611s / 7.516s / 7.186s respectively) passed. Local alpha matrix builds both binaries for Linux/macOS amd64/arm64; all four archive checksums verified, extracted Linux binaries' help and MCP EOF startup exercised. Nothing published or deployed.

## Files-first startup and connection setup
- User approved mockup B (board 08): real local files at default startup, disconnected destination, collapsed sidebars and transfer drawer, one quiet footer. `--demo` is now explicit; `--live` remains compatible but unnecessary. No transfer occurs without confirmation.
- User approved in-app connection setup in boards 10–11. Keep the provider-backed read-only test separate from explicit credential persistence. Session-only config is the default; remembered credentials use a private unencrypted rclone config, not a keychain. Azure CLI authentication reuses an existing login rather than pretending browser sign-in is implemented.
- User requested `..` rows with Enter and mouse parent navigation for real local and remote panes. Parent is navigation-only, never transferable/selectable, and is omitted at roots.
- Boards 07–12 are saved under `mockups/` with provenance and generated inconsistencies in the index. Board 12 proposes an MCP permission editor; no changes to MCP policy or server authorization are implied by connection setup.
- User reiterated frequent verified commits/pushes to main without asking for approval. No publishing/deployment or production writes authorized.
- Combined verification: `make check`, `go test -race ./...`, terminal smoke, three-backend live PTY transfers and tagged storage/UI/MCP integration pass. Real connection setup was independently exercised read-only against FTP/Azurite/SeaweedFS, including disposable explicit saves and session cleanup. Narrow/default/cloud/error/verified states and pointer navigation were inspected in the actual TUI; artifacts are `.amp/in/artifacts/files-first-*` and `connection-*`.

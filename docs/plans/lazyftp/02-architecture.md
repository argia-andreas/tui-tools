# Architecture: remote-file workspace

Status: Gate 2 approved 2026-10-23, including direct-process test services in this orb. Product name remains undecided; do not bake Tandem into module paths or configuration directories yet.

## Fit
The repository contains planning documents only. Build one Go terminal application using Bubble Tea, Lip Gloss, and Bubbles where suitable. Select and pin mutually compatible versions during implementation, rather than guessing current APIs.

Keep three responsibilities: terminal presentation and keyboard state; storage/job orchestration; backend access. The UI receives asynchronous results rather than blocking on directory listings or transfers. A small storage boundary supports deterministic fake storage and the real rclone adapter; avoid a plugin framework or four protocol implementations.

## Endpoints
No application HTTP API or server. Use rclone CLI subprocesses initially: structured directory listings and JSON log/progress records, explicit argument arrays, context-bound process lifetimes. Do not parse human-formatted tables. Validate output and cancellation in a backend spike before real transfer functionality depends on it. Revisit managed RC only if that spike exposes an actual unmet need.

The test lab exposes loopback-only FTP control/passive ports and Azurite Blob service. No externally exposed storage servers or production credentials.

## Data
No database. Application configuration holds theme selection, semantic color overrides, icon preference, and UI preferences. Use existing rclone remotes for credentials and connection configuration; never put secrets in application logs or command arguments. Do not implement Neovim Lua execution or claim VS Code theme import.

In-memory UI state records pane locations, cursors/selections, focus, sidebar visibility, inspector content, palette query and modal state. Tabs are deferred. Themes provide background/foreground, muted text, borders, focus, selection, local/remote accents, success/warning/error, and filetype roles. Offer familiar coding-theme presets and plain glyph fallback; do not require Nerd Fonts.

Jobs capture immutable source/destination locations and per-file outcomes. Keep completed, unfinished, skipped, failed and cancelled outcomes distinct. Start with one running transfer job for understandable queue/cancellation behavior. Queue persistence across application restarts is not promised in this build.

## Flow
Keyboard event → UI action resolution → state change or asynchronous storage command → result message → state update → render.

Ctrl+P searches the same action catalog used by direct shortcuts. Actions expose availability and reason; input fields consume typing instead of triggering global letter shortcuts. A modal owns input until dismissed. Opening a palette and typing a query cannot initiate a copy.

Copy captures both pane endpoints → enumerates work/conflicts → requests explicit conflict choices → executes confirmed work → records backend-confirmed outcomes → refreshes affected listing. Unknown or changed destination state must not silently inherit overwrite permission. Backend guarantees and race limitations need explicit verification before claiming unconditional overwrite safety.

Inspector follows the focused entry. Metadata is cheap; preview content is loaded only on request, with a size bound. Late results are associated with the originating location so stale responses cannot replace a newer view.

Both sidebars fill the space between header and footer. The optional job drawer consumes only the center column. At narrow sizes, hide optional regions before collapsing usable file panes; expose hidden regions through the action palette. Exact size thresholds belong in Gate 3.

## External
- Go toolchain and pinned Go modules.
- rclone binary and user-managed remotes for real operations.
- Local test FTP server using pyftpdlib and Blob emulator using Azurite, pinned when setup is implemented.
- Docker Compose lab on Docker-capable hosts; equivalent direct processes in this orb, where the Docker CLI exists but the daemon socket is absent.
- No cloud account, production service, telemetry, or external write is needed for development.

## Feedback loops
1. **Fast default:** ordinary Go tests, no network, Docker, rclone or cloud account. Test keyboard-driven observable state/rendering using deterministic fake storage. Fake responses can be delayed, failed and cancelled explicitly; avoid timing sleeps and private call-order assertions.
2. **Process boundary:** exercise real subprocess lifecycle and structured output/error handling with controlled fixtures. Reject malformed output and ensure cancellation cannot produce success. Keep fixtures limited to actual rclone formats captured in the spike.
3. **Real local integration:** run rclone against seeded FTP and Azurite, with identical test scenarios for both. Check returned bytes, overwrite decisions, nested paths, Unicode/spaces, empty files, failure, cancellation and retry. Use service stop/start and bounded transfer rates to make interruptions repeatable.
4. **Real terminal smoke:** drive an actual PTY for focus, palette typing, modal ownership, resize, theme changes and sidebar toggles. Inspect terminal renders and record representative interactions; string snapshots alone do not establish usability or correct terminal I/O.

Developer command contract, to be implemented: `make test` (fast), `make lab-up`, `make lab-seed`, `make test-integration`, `make demo`, `make lab-down`. Integration must fail clearly if prerequisites are missing, not silently skip. Reset touches only explicitly marked disposable lab roots and the designated test container; no arbitrary path or production remote may be reset.

Fixtures include asymmetric source/destination content so a wrong copy direction or accidental overwrite is detectable. Run both upload and download. Measure test durations after the first working baseline. The emulator validates mechanics, not production authentication or cloud-service parity.

## Build trajectory (detailed slice plan remains Gate 4)
1. Interactive tracer bullet: fixture-backed workspace, full-height collapsible sidebars, focus/navigation, Ctrl+P action palette, theme switching and icon fallback. Clearly mark demo data; no fake successful network operations. Establish fast tests and PTY smoke here.
2. Backend spike and disposable FTP/Blob lab; real listing path from UI to rclone on both services.
3. Single-file copy, confirmed conflict behavior and byte-verified round trips.
4. Folder jobs and queue, interruption/cancellation and retry of unfinished files.
5. Inspector previews, broader edge cases and packaging/documentation.

## Risks and decisions
- CLI progress and cancellation semantics are not yet validated. The backend spike is the decision point, not an assumed success.
- The orb has no Docker daemon, Go or rclone on PATH. Setup must install the toolchain and run equivalent supervised lab services; do not report integration as tested until executed.
- Theme studies are palette-inspired references. Preset provenance/licensing and font behavior must be checked when distributing actual theme/icon assets.
- The first slice intentionally proves UI interaction rather than pretending to solve storage reliability. Both fast fakes and real-service tests are required before calling the transfer workflow usable.

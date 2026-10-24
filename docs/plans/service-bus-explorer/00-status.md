# Status: Service Bus Explorer terminal UI

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23 for independent app/lab, layered testing and independent-first reuse boundary
- Gate 3 — Program Design: APPROVED 2026-10-23
- Gate 4 — Slice plan: APPROVED 2026-10-23

## Project boundary

This is an independent side project in the same repository as LazyFTP, not a LazyFTP feature. Its documentation lives exclusively in this directory and its application in `service-bus-explorer/`. Do not change LazyFTP code, documentation, plans or mockups while working on this project. Shared code extraction requires a separate explicit decision; do not assume shared runtime, credentials or configuration.

## Notes for a fresh session

- Slices 1–3 run as an independent terminal application with explicit `--fixture` or `--emulator` selection. The app's emulator connection is real and read-only; no cloud connection or MCP server exists. Runtime emulator counters remain unavailable, never synthesized from seed or peek results.
- Read all documents in this folder before continuing, including `02-architecture.md`, `local-development-notes.md` and `mockups/README.md`. Preserve the distinction between user requirements, recommendations and open decisions.
- Current dashboard default has expanded collapsible navigation, 12 visible grid boxes, optional denser table, and queue inclusion editing behind Settings. Twelve is not a watchlist limit.
- Preserve read-only peek defaults and explicit mutation review. Resending is not an atomic move; send and original cleanup are separate effects.
- Linux and macOS are initial targets; preserve Windows portability. Start local development with connection strings; Microsoft sign-in follows.
- Fast local feedback is a core requirement: deterministic fixture tests plus seeded broker integration, with commands to send, drain, dead-letter and reset disposable data.
- Topic-subscription replay sends to the original topic with an explicit warning about delivery to other matching subscriptions. Queue replay sends to the original queue.
- User approved receive-next DLQ acquisition followed by review of the actual locked message. A peek-selected message cannot be acquired by sequence using the supported SDK contract. Never imply that acquiring next acts on the selected preview; cancellation/release can affect delivery count.
- Gates 1–4 approved. User approved continuing through Slice 3 after reviewing earlier results. Slice 3 is implemented and verified; pause for review before Slice 4. Interactive mutations and bulk operations remain deferred. MCP has its own parallel product proposal, not UI-scope implementation authorization.
- Docker daemon and emulator now run as supervised orb services `servicebus-docker` and `servicebus-lab`. No resize was needed for the tested ~4 GB orb. Live lab configuration is outside the repo at `/home/user/.local/share/servicebus-explorer-lab/`.
- Messaging/discovery/queue replay/topic fan-out/drain probes passed. JavaScript SDK count reads failed to parse missing details; the .NET SDK returned incorrect zeros. Do not fake aggregate dashboard totals from bounded peeks or seed expectations.
- Architecture selects a separate Go module and terminal UI under `service-bus-explorer/`, with direct Azure SDK access and its own reproducible lab commands. Read `04-slices.md` for the approved sequence. Go SDK 1.11.0 discovery and message browsing now run against the emulator; cloud access remains unimplemented.
- User agreed to independent-first development: adapt proven small theme/layout/testing patterns, extract a minimal shared library only once common contracts stabilize. Gate 3 pins the FTP reference revision; no live dependency, FTP migration or framework now. Do not change LazyFTP files or contact its thread for updates.
- Approved Gate 3 includes session-only initial credentials, 160×48 dashboard acceptance, 30-second bounded count refresh and explicit unknown mutation outcomes. Replay uses a new message ID, immediate enqueue and renewed original TTL; original cleanup defaults off. Initial mutations exclude sessions and advanced AMQP representations.
- User requested parallel implementation where useful. Gate 4 records disjoint lab/theme task ownership; main thread integrates and verifies. Do not send work or updates to the FTP thread.
- Future agent MCP must deny by default and separately allowlist reads and optional resends. Dashboard visibility must not grant agent access.
- User confirmed parallel Service Bus MCP work, prioritizing finishing the current UI result first. Separate proposal: `../service-bus-mcp/01-product.md`. It adds independent DLQ deletion grants and queue-only resend destinations; lock/acquisition authority and read-only-first launch need product review. No MCP code exists.
- Slice 1 commands from repo root: `make -C service-bus-explorer run-fixture`, `test`, `terminal-test`. Go 1.27.3 required; Python/pyte setup in the app README. This orb's Go is `/home/user/.local/toolchains/go/bin/go`; override Make's `GO` when absent from PATH.
- Verification: race-enabled Go tests, `go vet`, and real-PTY navigation/layout/paging/resize/exit checks passed. Inspected actual dark-theme renders for grid/table, collapsed sidebar, Active/DLQ inspector and compact layout; saved keyboard recording under workspace `.amp/in/artifacts/service-bus-slice1/`. No claim of Azure/native macOS/Windows verification.
- Slice 2 adds `run-emulator`, `integration-test`, and `lab-up/status/down/reset/seed/send/peek/dead-letter/drain`. See app README for explicit EULA/confirmation flags. This orb needs `LAB_FLAGS="--sudo --orb ..."`; pass the Go override above. `lab-seed` always destroys/recreates only the disposable project's data.
- New reusable lab: supervised service / Compose project `sbx-explorer-lab`, fixed loopback AMQP 5673 and management 5301. SQL credentials live outside Git at `~/.local/share/sbx-explorer-lab/compose.env`, private permissions. The older 5672/5300 spike was not changed.
- Verified two complete reset/seed runs after pacing successful batches (first unpaced run correctly stopped on broker throttling/unknown send). Repeated seeded Go integration + real-emulator PTY passed: 14 resources, exact bytes/properties, Active/DLQ/subscription pages, explicit refresh/cursors, and independent receive after peek. Actual lab CLI send → repeated peek → dead-letter → bounded drain reported 1 then 0; temporary probe was removed.
- Queue seed data follows the fixture. Topic fanout uses one eight-message publication cohort: Billing 7 Active/1 DLQ, Shipping 5 Active/3 DLQ. These are independently checked seed expectations, not runtime counts or independent subscription sends. Seed also includes text/control-sequence bodies. Security/guard tests and the original fixture PTY control pass.
- Inspected real emulator dashboard/Active/DLQ screenshots and keyboard-recording contact sheet in workspace `.amp/in/artifacts/service-bus-slice2/` (brief index included). Temporary browser capture service stopped; seeded lab remains running. No FTP files were authored.
- Slice 3 exposes Midnight, Gruvbox and Light through Settings and fuzzy Ctrl+P actions. Adds a collapsible full-identity entity tree, visible-pane focus cycling, independent inspector scrolling and runtime `--ascii` fallback. Printable palette text owns q/j/k; Escape restores prior focus. No mutation actions exist in the palette.
- Versioned, validated nonsecret preferences use the platform user config directory (or explicit `--config`). Explicit Save atomically persists private files; unsaved edits are marked. A save owns its snapshot, so a late completion cannot claim newer edits were saved. Malformed/unsupported existing files are not overwritten.
- Watchlists key full profile/namespace/entity identity; absent profiles initially watch all discovered resources, explicit empty lists watch none. Queues/Topics/tree still browse unwatched resources. Saving watchlists never grants MCP access and stores no credentials or message bodies.
- Watched counts refresh 30 seconds after each bounded sweep; at most four reads, no overlapping sweeps, ten-second sweep context. Failure retains prior values marked stale (~), with observation time/error details; unknown is distinct from zero. Identity-keyed count replies cannot change message selection or page. The emulator limitation still renders unavailable.
- Slice 3 checks: race-enabled Go suite, vet, Python lab guards, expanded fixture real-PTY save/restart/theme/palette/resize checks, seeded Go broker integration and emulator PTY. A temporary production-timer probe fired after 30.03 seconds and refreshed all 14 resources without input; removed after validation to avoid permanent slow tests. Public UI tests cover same-name namespaces, empty watchlists, 29-resource table, all themes/ASCII widths, controlled stale reads, tree groups and inspector scrolling/control-text safety.
- Actual Slice 3 captures/keyboard tour live in workspace `.amp/in/artifacts/service-bus-slice3/`, with a reference index. Default/theme/Settings/empty/narrow/focused-inspector states were inspected; stale-count rendering uses a deliberately failing offline fixture wrapper, not a claim about cloud counters. Temporary capture helpers are removed and services stopped after verification.
- Source design conversation: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1
- Continue discussion in that Service Bus thread. Do not send progress updates to the FTP thread.
- User authorized implementation, frequent commits/pushes and pulling when needed, including delegated threads; put that instruction in handoffs. Use disjoint ownership and separate branches for independent threads, integrate with checks, never force-push. No Azure writes/deployments are authorized.

## Slices

- [x] Slice 1 — runnable fixture workspace
- [x] Slice 2 — real local broker browsing and reproducible lab
- [x] Slice 3 — complete read-only workspace and themes
- [ ] Slice 4 — reviewed single-message DLQ actions
- [ ] Slice 5 — portable developer handoff and polish

Approved sequence. See `04-slices.md` for checks and ownership. Show actual terminal captures/recordings when results are ready.

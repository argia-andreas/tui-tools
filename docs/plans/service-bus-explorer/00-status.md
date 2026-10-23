# Status: Service Bus Explorer terminal UI

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23 for independent app/lab, layered testing and independent-first reuse boundary
- Gate 3 — Program Design: APPROVED 2026-10-23
- Gate 4 — Slice plan: APPROVED 2026-10-23

## Project boundary

This is an independent side project in the same repository as LazyFTP, not a LazyFTP feature. Its documentation lives exclusively in this directory and its application in `service-bus-explorer/`. Do not change LazyFTP code, documentation, plans or mockups while working on this project. Shared code extraction requires a separate explicit decision; do not assume shared runtime, credentials or configuration.

## Notes for a fresh session

- Slice 1 now runs as an independent fixture-only terminal application. No application cloud/emulator connection or MCP server exists. The separate bounded emulator experiment runs in this orb; its messaging operations were verified, but runtime counter responses were not accurate.
- Read all documents in this folder before continuing, including `02-architecture.md`, `local-development-notes.md` and `mockups/README.md`. Preserve the distinction between user requirements, recommendations and open decisions.
- Current dashboard default has expanded collapsible navigation, 12 visible grid boxes, optional denser table, and queue inclusion editing behind Settings. Twelve is not a watchlist limit.
- Preserve read-only peek defaults and explicit mutation review. Resending is not an atomic move; send and original cleanup are separate effects.
- Linux and macOS are initial targets; preserve Windows portability. Start local development with connection strings; Microsoft sign-in follows.
- Fast local feedback is a core requirement: deterministic fixture tests plus seeded broker integration, with commands to send, drain, dead-letter and reset disposable data.
- Topic-subscription replay sends to the original topic with an explicit warning about delivery to other matching subscriptions. Queue replay sends to the original queue.
- User approved receive-next DLQ acquisition followed by review of the actual locked message. A peek-selected message cannot be acquired by sequence using the supported SDK contract. Never imply that acquiring next acts on the selected preview; cancellation/release can affect delivery count.
- Gates 1–4 approved. Slice 1 implemented and verified; pause for user review before Slice 2. Read-only dashboard/browsing first, reviewed mutations next; bulk operations remain deferred. MCP has its own parallel product proposal, not UI-scope implementation authorization.
- Docker daemon and emulator now run as supervised orb services `servicebus-docker` and `servicebus-lab`. No resize was needed for the tested ~4 GB orb. Live lab configuration is outside the repo at `/home/user/.local/share/servicebus-explorer-lab/`.
- Messaging/discovery/queue replay/topic fan-out/drain probes passed. JavaScript SDK count reads failed to parse missing details; the .NET SDK returned incorrect zeros. Do not fake aggregate dashboard totals from bounded peeks or seed expectations.
- Architecture selects a separate Go module and terminal UI under `service-bus-explorer/`, with direct Azure SDK access and its own reproducible lab commands. Read `04-slices.md` for the approved sequence. Slice 1 is fixture-only; no implemented cloud/emulator application access yet.
- User agreed to independent-first development: adapt proven small theme/layout/testing patterns, extract a minimal shared library only once common contracts stabilize. Gate 3 pins the FTP reference revision; no live dependency, FTP migration or framework now. Do not change LazyFTP files or contact its thread for updates.
- Approved Gate 3 includes session-only initial credentials, 160×48 dashboard acceptance, 30-second bounded count refresh and explicit unknown mutation outcomes. Replay uses a new message ID, immediate enqueue and renewed original TTL; original cleanup defaults off. Initial mutations exclude sessions and advanced AMQP representations.
- User requested parallel implementation where useful. Gate 4 records disjoint lab/theme task ownership; main thread integrates and verifies. Do not send work or updates to the FTP thread.
- Future agent MCP must deny by default and separately allowlist reads and optional resends. Dashboard visibility must not grant agent access.
- User confirmed parallel Service Bus MCP work, prioritizing finishing the current UI result first. Separate proposal: `../service-bus-mcp/01-product.md`. It adds independent DLQ deletion grants and queue-only resend destinations; lock/acquisition authority and read-only-first launch need product review. No MCP code exists.
- Slice 1 commands from repo root: `make -C service-bus-explorer run-fixture`, `test`, `terminal-test`. Go 1.27.3 required; Python/pyte setup in the app README. This orb's Go is `/home/user/.local/toolchains/go/bin/go`; override Make's `GO` when absent from PATH.
- Verification: race-enabled Go tests, `go vet`, and real-PTY navigation/layout/paging/resize/exit checks passed. Inspected actual dark-theme renders for grid/table, collapsed sidebar, Active/DLQ inspector and compact layout; saved keyboard recording under workspace `.amp/in/artifacts/service-bus-slice1/`. No claim of Azure/native macOS/Windows verification.
- Three semantic theme presets are implemented internally, but Slice 1 exposes Midnight only. Ctrl+P, theme selection and saved watchlist settings arrive in Slice 3; Settings currently explains that limitation. Fixture mutation calls reject access; no receive/resend/delete controls exist yet.
- Source design conversation: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1
- Continue discussion in that Service Bus thread. Do not send progress updates to the FTP thread.
- User authorized implementation, frequent commits/pushes and pulling when needed, including delegated threads; put that instruction in handoffs. Use disjoint ownership and separate branches for independent threads, integrate with checks, never force-push. No Azure writes/deployments are authorized.

## Slices

- [x] Slice 1 — runnable fixture workspace
- [ ] Slice 2 — real local broker browsing and reproducible lab
- [ ] Slice 3 — complete read-only workspace and themes
- [ ] Slice 4 — reviewed single-message DLQ actions
- [ ] Slice 5 — portable developer handoff and polish

Approved sequence. See `04-slices.md` for checks and ownership. Show actual terminal captures/recordings when results are ready.

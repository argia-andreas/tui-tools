# Status: Service Bus Explorer terminal UI

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: in progress; proposal ready for review
- Gate 3 — Program Design: pending
- Gate 4 — Slice plan: pending

## Project boundary

This is an independent side project in the same repository as LazyFTP, not a LazyFTP feature. Its documentation lives exclusively in this directory. Do not change LazyFTP code, documentation, plans or mockups while working on this project. Future implementation location and any shared code require an explicit architecture decision; do not assume shared runtime, credentials or configuration.

## Notes for a fresh session

- No Service Bus application, cloud connection or MCP server exists. A bounded local emulator experiment now runs in this orb; real messaging operations were verified, but runtime counter responses were not accurate.
- Read all documents in this folder before continuing, including `02-architecture.md`, `local-development-notes.md` and `mockups/README.md`. Preserve the distinction between user requirements, recommendations and open decisions.
- Current dashboard default has expanded collapsible navigation, 12 visible grid boxes, optional denser table, and queue inclusion editing behind Settings. Twelve is not a watchlist limit.
- Preserve read-only peek defaults and explicit mutation review. Resending is not an atomic move; send and original cleanup are separate effects.
- Linux and macOS are initial targets; preserve Windows portability. Start local development with connection strings; Microsoft sign-in follows.
- Fast local feedback is a core requirement: deterministic fixture tests plus seeded broker integration, with commands to send, drain, dead-letter and reset disposable data.
- Topic-subscription replay sends to the original topic with an explicit warning about delivery to other matching subscriptions. Queue replay sends to the original queue.
- Gate 1 approved after these scope decisions. Read-only dashboard/browsing first, reviewed mutations next; bulk operations and MCP deferred. Architecture and implementation remain unapproved.
- Docker daemon and emulator now run as supervised orb services `servicebus-docker` and `servicebus-lab`. No resize was needed for the tested ~4 GB orb. Live lab configuration is outside the repo at `/home/user/.local/share/servicebus-explorer-lab/`.
- Messaging/discovery/queue replay/topic fan-out/drain probes passed. JavaScript SDK count reads failed to parse missing details; the .NET SDK returned incorrect zeros. Do not fake aggregate dashboard totals from bounded peeks or seed expectations.
- Architecture proposes a separate Go module and terminal UI under `service-bus-explorer/`, with direct Azure SDK access and its own reproducible lab commands. Those commands are not implemented in the repository yet. Approval still required before program design/application implementation.
- Future agent MCP must deny by default and separately allowlist reads and optional resends. Dashboard visibility must not grant agent access.
- Source design conversation: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1
- Continue discussion in that Service Bus thread. Do not send progress updates to the FTP thread.
- User authorized pulling and pushing these standalone documents; that does not authorize Azure writes or implementation.

## Slices

Not planned. Complete the design gates before implementation.

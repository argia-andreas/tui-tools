# Status: Service Bus Explorer terminal UI

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23 for independent app/lab, layered testing and independent-first reuse boundary
- Gate 3 — Program Design: APPROVED 2026-10-23
- Gate 4 — Slice plan: proposal ready for review; not approved

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
- User approved receive-next DLQ acquisition followed by review of the actual locked message. A peek-selected message cannot be acquired by sequence using the supported SDK contract. Never imply that acquiring next acts on the selected preview; cancellation/release can affect delivery count.
- Gates 1–3 approved. Read-only dashboard/browsing first, reviewed mutations next; bulk operations and MCP deferred. Implementation slice plan awaits Gate 4 approval.
- Docker daemon and emulator now run as supervised orb services `servicebus-docker` and `servicebus-lab`. No resize was needed for the tested ~4 GB orb. Live lab configuration is outside the repo at `/home/user/.local/share/servicebus-explorer-lab/`.
- Messaging/discovery/queue replay/topic fan-out/drain probes passed. JavaScript SDK count reads failed to parse missing details; the .NET SDK returned incorrect zeros. Do not fake aggregate dashboard totals from bounded peeks or seed expectations.
- Architecture selects a separate Go module and terminal UI under `service-bus-explorer/`, with direct Azure SDK access and its own reproducible lab commands. Those commands are not implemented in the repository yet. Review `04-slices.md`; no application implementation before Gate 4 approval.
- User agreed to independent-first development: adapt proven small theme/layout/testing patterns, extract a minimal shared library only once common contracts stabilize. Gate 3 pins the FTP reference revision; no live dependency, FTP migration or framework now. Do not change LazyFTP files or contact its thread for updates.
- Approved Gate 3 includes session-only initial credentials, 160×48 dashboard acceptance, 30-second bounded count refresh and explicit unknown mutation outcomes. Replay uses a new message ID, immediate enqueue and renewed original TTL; original cleanup defaults off. Initial mutations exclude sessions and advanced AMQP representations.
- User requested parallel implementation where useful. Gate 4 records disjoint lab/theme task ownership; main thread integrates and verifies. Do not send work or updates to the FTP thread.
- Future agent MCP must deny by default and separately allowlist reads and optional resends. Dashboard visibility must not grant agent access.
- Source design conversation: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1
- Continue discussion in that Service Bus thread. Do not send progress updates to the FTP thread.
- User authorized frequent commits/pushes and pulling when needed, including delegated threads; put that instruction in handoffs. Use disjoint ownership and separate branches for independent threads, integrate with checks, never force-push. Application implementation still requires Gate 4 approval; no Azure writes/deployments are authorized.

## Slices

- [ ] Slice 1 — runnable fixture workspace
- [ ] Slice 2 — real local broker browsing and reproducible lab
- [ ] Slice 3 — complete read-only workspace and themes
- [ ] Slice 4 — reviewed single-message DLQ actions
- [ ] Slice 5 — portable developer handoff and polish

Proposed sequence, not implementation authorization. See `04-slices.md` for checks and ownership.

# Status: streams

- Gate 1 — Product: APPROVED 2026-10-30 (Andreas: “Looks great! Go for it.”)
- Gate 2 — Architecture: APPROVED 2026-10-30 (Andreas: “GO for it!”)
- Gate 3 — Program Design: APPROVED 2026-10-30 (Andreas: “Go for it.”)
- Gate 4 — Slice plan: APPROVED 2026-10-30 (Andreas: “NO slice approval. Build the thing!”)

## Slices

- [x] 1 — Offline executable: topic list, event rows, inspector and shared controls
- [x] 2 — Real local Redpanda: seed → discover → bounded topic read → inspect
- [x] 3 — Seek/continue: precise positions, limits and retention/error recovery
- [x] 4 — Follow/pause/resume: cancellation, stale results and bounded retention
- [x] 5 — Investigation: exact content, local find, copy and explicit preferences
- [x] 6 — Consumer groups: partition lag with honest missing/error states
- [x] 7 — Implemented compatibility/security and suite integration; acceptance is
  partial for the full proposed stress/authentication matrix (see below)

## Delivery and evidence

Implementation and suite integration are complete. Root builds/checks and
all four real-binary PTY suites pass. App integration passes against Redpanda
v26.2.5 (TLS/SCRAM-SHA-256/read-only ACLs) and Apache Kafka 4.3.2 (loopback
plaintext). Real transactions, bytes/null/headers, seeks, cursor replay/resume,
follow/close and unchanged group positions were checked. See
[validation and explicit limitations](05-validation.md), the
[app README](../../../apps/streams/README.md), and the
[lab README](../../../apps/streams/lab/README.md).

The approved A/B UI refinement is implemented: Compact default, optional
Comfortable density, keyboard-selection-driven inspector (no mouse hover),
syntax-coloured JSON and a shared-library payload modal on Enter. Esc preserves
the event selection. Density persists only with explicit Save appearance.
Rendered fixture screens and actual-binary PTY coverage include both densities,
modal capture, JSON token colours/backgrounds, save/restart and NO_COLOR.

mTLS/PLAIN/SCRAM-SHA-512 live handshakes, compaction/retention stress races,
oversized-batch live acceptance, external clipboard receipt and human usability
remain unverified. No production-readiness or universal compatibility claim.

## Notes for a fresh session

Andreas requested a plan for `streams`, a Kafka-compatible event/topic viewer,
on 2026-10-30. Gate 1 is approved, including read-only-first scope. The user also
authorized pulling main; local main fast-forwarded to origin/main's rename commit
on 2026-10-30. Current app directories are transfer, queue and logs. Existing
module identities/config paths were intentionally preserved by that rename.
Read all numbered documents and mockups/workbench.html before continuing.
Implementation is authorized through all slices without per-slice approval stops.
Report milestone evidence and blockers, but do not ask whether to continue.
Andreas authorized shipping the completed implementation to main on 2026-10-31.
The initial orb had no Docker socket. Implementation established a private,
loopback-only lab daemon using vfs and no bridge/iptables changes. Both real
brokers are running with private app-local state; builds never start them.
Andreas explicitly requested a local Kafka-compatible test event hub at Gate 3
approval, then selected Redpanda as the primary local broker (“Ok, lets go1”).
Plan: a real local Redpanda broker, plus Apache Kafka compatibility validation,
with fictional seeded events. No Azure Event Hubs emulation is claimed.
After each implemented slice, record evidence and continue automatically.

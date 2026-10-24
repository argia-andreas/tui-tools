# Log viewer exploration

- Gate 1 — Product: APPROVED 2026-10-23 by Andreas (“Yes, this sounds good”).
- Gate 2 — Architecture: APPROVED 2026-10-23 by Andreas (“Yeah. That sounds good. Let's go for it.”).
- Gate 3 — Program Design: APPROVED 2026-10-24 by Andreas (“Sounds good. Go ahead.”).
- Gate 4 — Slice plan: in progress; 04-slices.md awaiting approval.

## Scope

Approved product, architecture and program design; vertical-slice plan proposed.
No implementation, approved slice plan, lab startup, deployment or publishing.
SIGNAL is a provisional mockup label, not an approved product name.

## Slices (proposed, not started)

- [ ] Slice 1 — fixture workbench tracer bullet.
- [ ] Slice 2 — real local Elasticsearch happy path.
- [ ] Slice 3 — app/time/level/message search, history and saved searches.
- [ ] Slice 4 — field yank, inspector and command palette.
- [ ] Slice 5 — context, trace and follow/pause/resume.
- [ ] Slice 6 — default-deny read-only MCP.

## Notes for a fresh session

- Read all documents in this folder before continuing.
- Andreas likes the right-hand event inspector and requests field-level `y` yank,
  excellent keyboard ergonomics, and a provider abstraction.
- The current source is NOT identified. Andreas recalls a hostname resembling
  `logs.insights…` and suspects Elastic. That substring is insufficient evidence.
  Ask for only a sanitized hostname or visible product name, never credentials.
- Approved product direction: stream workbench, with focus-reader and context
  modes rather than three separate products. Architecture and keyboard contract
  are approved in Gates 2–3; SIGNAL remains an unapproved name.
- Andreas requested a fast local Elastic development/test loop. Recommend a fake
  provider for instant UI tests plus a real, pinned single-node Elasticsearch for
  adapter/security integration tests. See provider-research.md. No lab was started.
- On Andreas's request to continue, pulled main before preparing Gate 2. Approved
  architecture: independent log-viewer module, shared policy-safe investigation path,
  capability-aware adapters, stdio-only/default-deny MCP and a standalone native
  Elasticsearch lab in orbs (Docker equivalent on hosts with a daemon).
- Gate 3 pins the local native lab to Elasticsearch 9.5.5 with the official published
  SHA-512, proposes logview/logview-mcp executable names, concrete contracts and
  behavioral tests. Checksum availability and non-root/kernel limits were inspected;
  archive bytes/startup and clipboard receipt are NOT verified. Docker image digest
  remains a prerequisite before optional Docker support.
- Gate 4 proposes six vertical slices. Stop for Gate 4 approval; then implement
  Slice 1 only, prove it and ask whether to continue. Verification is part of each
  slice, not a postponed phase. Read 04-slices.md for acceptance and delivery rules.
- Painter images live only under `.amp/in/artifacts/log-viewer/`; they are static,
  fictional references, not rendered application screenshots or behavior tests.
- Product and research notes take precedence over generated labels and shortcuts.
- The requested Painter exploration substitutes for HTML screen sketches at this
  gate. No interactive prototype or implementation has been created.
- Do not edit FTP or Service Bus code/docs. Their current work is independent.
- Keep questions and subsequent exploration with Andreas in this thread:
  https://ampcode.com/threads/T-01a16ae8-e353-76b7-ae0c-8fe1908f1687

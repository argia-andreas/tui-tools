# Log viewer exploration

- Gate 1 — Product: APPROVED 2026-10-23 by Andreas (“Yes, this sounds good”).
- Gate 2 — Architecture: APPROVED 2026-10-23 by Andreas (“Yeah. That sounds good. Let's go for it.”).
- Gate 3 — Program Design: in progress; 03-program-design.md awaiting approval.
- Gate 4 — Slice plan: pending.

## Scope

Approved product/architecture, design references, research and a program-design draft.
No implementation, approved program design, slice commitments, deployment or publishing.
SIGNAL is a provisional mockup label, not an approved product name.

## Notes for a fresh session

- Read all documents in this folder before continuing.
- Andreas likes the right-hand event inspector and requests field-level `y` yank,
  excellent keyboard ergonomics, and a provider abstraction.
- The current source is NOT identified. Andreas recalls a hostname resembling
  `logs.insights…` and suspects Elastic. That substring is insufficient evidence.
  Ask for only a sanitized hostname or visible product name, never credentials.
- Approved product direction: stream workbench, with focus-reader and context
  modes rather than three separate products. Exact bindings/architecture still
  require their later gates; SIGNAL remains an unapproved name.
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
  remains a prerequisite before optional Docker support. Stop for Gate 3 approval.
- Painter images live only under `.amp/in/artifacts/log-viewer/`; they are static,
  fictional references, not rendered application screenshots or behavior tests.
- Product and research notes take precedence over generated labels and shortcuts.
- The requested Painter exploration substitutes for HTML screen sketches at this
  gate. No interactive prototype or implementation has been created.
- Do not edit FTP or Service Bus code/docs. Their current work is independent.
- Keep questions and subsequent exploration with Andreas in this thread:
  https://ampcode.com/threads/T-01a16ae8-e353-76b7-ae0c-8fe1908f1687

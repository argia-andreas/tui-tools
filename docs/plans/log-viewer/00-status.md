# Log viewer exploration

- Gate 1 — Product: APPROVED 2026-10-23 by Andreas (“Yes, this sounds good”).
- Gate 2 — Architecture: in progress; draft in 02-architecture.md awaiting approval.
- Gate 3 — Program Design: pending.
- Gate 4 — Slice plan: pending.

## Scope

Product/design references, provider research, and an architecture proposal only.
No implementation, approved architecture, slice commitments, deployment, or publishing.
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
- On Andreas's request to continue, pulled main before preparing Gate 2. The draft
  proposes an independent log-viewer module, shared policy-safe investigation path,
  capability-aware adapters, stdio-only/default-deny MCP and a standalone native
  Elasticsearch lab in orbs (Docker equivalent on hosts with a daemon). Exact
  release/digest, files and signatures wait for Gate 3. Stop for Gate 2 approval.
- Painter images live only under `.amp/in/artifacts/log-viewer/`; they are static,
  fictional references, not rendered application screenshots or behavior tests.
- Product and research notes take precedence over generated labels and shortcuts.
- The requested Painter exploration substitutes for HTML screen sketches at this
  gate. No interactive prototype or implementation has been created.
- Do not edit FTP or Service Bus code/docs. Their current work is independent.
- Keep questions and subsequent exploration with Andreas in this thread:
  https://ampcode.com/threads/T-01a16ae8-e353-76b7-ae0c-8fe1908f1687

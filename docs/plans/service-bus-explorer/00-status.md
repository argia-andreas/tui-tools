# Status: Service Bus Explorer terminal UI

- Gate 1 — Product: in progress; exploration documented, formal approval pending
- Gate 2 — Architecture: pending
- Gate 3 — Program Design: pending
- Gate 4 — Slice plan: pending

## Project boundary

This is an independent side project in the same repository as LazyFTP, not a LazyFTP feature. Its documentation lives exclusively in this directory. Do not change LazyFTP code, documentation, plans or mockups while working on this project. Future implementation location and any shared code require an explicit architecture decision; do not assume shared runtime, credentials or configuration.

## Notes for a fresh session

- This request documents prior design exploration only. No Service Bus application, Azure connection, MCP server or tested interaction exists.
- Read `01-product.md`, `auth-and-mcp-notes.md` and `mockups/README.md` before continuing. Preserve the distinction between user requirements, recommendations and open decisions.
- Current dashboard default has expanded collapsible navigation, 12 visible grid boxes, optional denser table, and queue inclusion editing behind Settings. Twelve is not a watchlist limit.
- Preserve read-only peek defaults and explicit mutation review. Resending is not an atomic move; send and original cleanup are separate effects.
- Auth research recommends Microsoft sign-in plus optional SAS connection strings; this is not an approved technical design.
- Future agent MCP must deny by default and separately allowlist reads and optional resends. Dashboard visibility must not grant agent access.
- Source design conversation: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1
- Continue discussion in that Service Bus thread. Do not send progress updates to the FTP thread.
- User authorized pulling and pushing these standalone documents; that does not authorize Azure writes or implementation.

## Slices

Not planned. Complete the design gates before implementation.

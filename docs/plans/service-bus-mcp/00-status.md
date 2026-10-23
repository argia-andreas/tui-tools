# Status: Service Bus MCP

- Gate 1 — Product: in progress; proposal **NOT APPROVED**.
- Gate 2 — Architecture: pending; not authorized.
- Gate 3 — Program Design: pending; not authorized.
- Gate 4 — Slice plan: pending; not authorized.

## Slices

None planned or authorized. **No MCP server exists.** This is a separate product gate, not an extension of the approved Explorer UI implementation scope.

## Notes for a fresh session

- Andreas's requirement is captured in `01-product.md` for review only. No SDK, server, tool contracts or implementation are approved here.
- Access is denied by default. Explicit full-identity read grants, DLQ deletion grants and DLQ-to-queue resend grants are independent; watchlists and interactive credentials confer no MCP access.
- Only queue destinations may be considered for resend. Topic resend, active-message writes, bulk operations and arbitrary destinations are out of scope.
- Essential unresolved decision: launch read-only first (recommended), or wait for an explicitly approved acquisition policy before enabling any locks/writes. Receive-next is not a read, and arbitrary peek-selected DLQ messages cannot be locked by sequence under the supported contract.
- Existing Explorer documents and application belong to the main thread. No changes to them, FTP, or the FTP thread are part of this proposal.
- Next action: review the product proposal and ask: **Approve Gate 1, or what should change?** Approval must explicitly resolve launch scope; it must not silently authorize acquisition or writes.

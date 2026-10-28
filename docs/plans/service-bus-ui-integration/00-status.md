# Status: Service Bus environment/shared-UI integration

- Product requirements: inherited from the approved shared-terminal-ui and upstream service-bus-scopes plans; no new product gate
- Gate 2 — Integration architecture: authorized without further pauses
- Gate 3 — Program design: authorized without further pauses
- Gate 4 — Integration slice plan: authorized without further pauses

## Slices

- [x] Preserve upstream provider/config semantics and reconcile model/startup ownership
- [x] Adapt environment screens and Settings to shared surfaces/semantic controls
- [x] Verify combined domain, pointer/keyboard, persistence and rendered behavior
- Delivery: main push authorized; exact remote result is recorded in the implementation thread

## Notes for a fresh session

Andreas requested shipping the shared terminal UI, then explicitly authorized
integrating upstream saved environments and adapting their controls to our shared
library, with preparation before implementation and thorough testing.

- Local main: commit 32ca95d30951bf4a250ab337d40c1e99db389fb1, shared UI plus relocation/cleanup.
- Fetched origin/main: commit 23feda3, saved Service Bus environments and read-only Azure connection at the old source path.
- One rebase was attempted, found substantive conflicts, then was aborted cleanly.
- Shipping authorization carries forward after integration and validation; see [validation](03-validation.md).
- Andreas explicitly waived further gate approvals: make it work, exercise engineering judgment, and test the combination before delivery.
- Do not treat the prior shipping checks as verification of the combined app.
- Preserve the incoming providers/preferences and app-owned safety; adapt new UI to existing terminalui APIs rather than accepting a second keyboard-only UI implementation.
- No live Azure writes, login/credential changes, lab seeding/reset or deployment authorization.

Upstream plans can currently be read with
`git show origin/main:docs/plans/service-bus-scopes/<file>`.
They will be retained at their root docs paths during integration. Shared plans
remain under `docs/plans/shared-terminal-ui/`. Historical approvals/evidence are
not proof that the new combination works.

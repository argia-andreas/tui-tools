# Status: Service Bus Explorer saved environments

- Gate 1 — Product: APPROVED 2026-10-24 by user via parent thread
- Gate 2 — Architecture: authorized 2026-10-24 without further approval pauses
- Gate 3 — Program Design: authorized 2026-10-24 without further approval pauses
- Gate 4 — Slice plan: authorized 2026-10-24 without further approval pauses

## Implementation authorization

The user approved implementation, full testing and real-terminal verification against the mockups, then authorized committing and shipping to main after rebasing onto origin/HEAD. The additive integration with the shipped unified inspector, clipboard/Vim behavior and selected-review API was explicitly approved. No Azure writes or credential changes are authorized.

## Slices

- [x] Explicit namespace contracts, environment preferences and compatible local migration
- [x] Scoped providers and read-only Azure CLI connection
- [x] Environment management, switching and scoped dashboard inclusion in the TUI
- [x] Automated regressions, real fixture PTYs and inspected terminal screenshots/interaction recording

## Verification and delivery

`go test -race ./...`, `go vet ./...`, both real-binary PTY suites, the 10 Python lab tests and `git diff --check` pass. Creation, cross-subscription snapshots, inclusion, save/restart, edit/duplicate/delete and compact 80×24 / 60×18 states were exercised. See `05-verification.md` for evidence and corrected intermediate captures.

Live Azure is unverified (no Azure CLI/target account in the orb). Seeded-emulator integration was not run (Docker daemon/socket unavailable). Do not present mocks or offline fixture permissions as live Azure proof. See the evidence thread for final integration checks and delivery state.

## Notes for a fresh session

- Product, implementation and shipping approved by parent thread https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937.
- Read 01-product.md and research.md. Only Service Bus Explorer is in scope; no LazyFTP, log-viewer or MCP changes.
- The baseline was emulator/fixture-only. Implementation now includes a read-only public-Azure CLI provider with scoped discovery and immutable principal/tenant checks. No live Azure connection was tested; do not claim live validation.
- Recommend named environments containing explicitly selected namespace sets. Azure subscriptions and resource groups organize namespace selection, not implicit wildcard discovery. Selecting groups resolves to a visible namespace snapshot; future namespaces require approval.
- Reuse dashboard inclusion/watchlists inside Settings, scoped per environment; do not introduce a parallel entity filter. Global visual preferences remain global.
- Plain HTML mockups are fictional, static product references, not application code or proof of Azure behavior. Painter comparison is supplementary; the product document wins over generated details.
- Review page and mockups are the original product references. Shipping to main is authorized; Azure writes are not.
- Review portal: https://t-03h3zgypthdehx6998kv83klk-p23520.onamp.dev/index.html . All four screen references and the narrow proposal diagram were rendered and inspected; artifact checker passes for five pages. Evidence/limitations are recorded in research.md.
- All remaining gates were authorized without further pauses. Report tested results and remaining limitations to the parent when the implementation is ready.

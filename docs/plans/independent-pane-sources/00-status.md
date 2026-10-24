# Status: Independent pane sources

- Gate 1 — Product: APPROVED 2026-10-24
- Gate 2 — Architecture: approved to proceed without pause, 2026-10-24
- Gate 3 — Program Design: approved to proceed without pause, 2026-10-24
- Gate 4 — Slice plan: approved to proceed without pause, 2026-10-24

## Slices
- [x] Slice 1 — Pane-targeted attachment and detached-source UI, wired to the existing connection host.
- [x] Slice 2 — Symmetric saved/new/local source flows and preserved startup/bookmark behavior.
- [x] Slice 3 — Safety regressions, terminal acceptance matrix and inspected real-UI evidence.

Implementation and verification are complete. The user subsequently authorized committing, rebasing onto latest origin/main, rerunning the full checks and pushing to main. See [verification](05-verification.md) for executed checks and real-terminal evidence.

## Notes for a fresh session
- Scope is only the LazyFTP terminal app. Do not change log-viewer, service-bus-explorer or MCP permissions.
- The current request explicitly requires the software-factory approval gates, notwithstanding the earlier proof-of-concept waiver in the original LazyFTP plan.
- User approved the mockups and remote-to-remote copy exclusion, and explicitly waived subsequent gate/slice pauses for this scoped design. The original no-shipping restriction was superseded by explicit authorization to ship to main.
- Parent thread: https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937
- This proposal lives in this thread's separate checkout; it is not automatically present in the parent's checkout.
- Recommendation: an inline source chooser in either detached pane; keep existing remote setup as a pane-targeted overlay, not a new connection system.
- Preserve connection-first startup and existing CLI launch behavior in this feature. Startup still targets the right pane; subsequent source choices target the selected pane only.
- Remote/remote browsing is included. Remote-to-remote copying is not; retain an explicit unavailable reason rather than silently staging files.
- Source changes, including detaching, remain blocked during active transfer planning or execution. This avoids adding cancellation semantics to this feature.
- Mockups are static design references, not actual TUI renders or working interactions. Generated comparison is exploratory only; canonical behavior is in 01-product.md.

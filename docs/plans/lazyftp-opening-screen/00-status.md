# Status: LazyFTP opening screen

- Gate 1 — Product: APPROVED 2026-10-24; opening-screen mockup approved as shown
- Gate 2 — Architecture: documented; further pause waived by user 2026-10-24
- Gate 3 — Program Design: documented; further pause waived by user 2026-10-24
- Gate 4 — Slice plan: documented; further pause waived by user 2026-10-24

## Slices
- [x] Slice 1 — Continuous picker navigation and matching centered opening-screen layout.
- [x] Slice 2 — Async discovery selection retention, regression checks, real-terminal visual and interaction verification.

## Implementation and verification — 2026-10-24
- Implemented the approved centered card, single-row connections/actions, endpoint metadata at wider sizes, and one selection marker/highlight. Up/Down, j/k, Tab/Shift+Tab, and wheel movement share a clamped sequence through every connection and both actions.
- Selection and list viewport survive details/help/setup returns. Late discovery retains an interacted-with action and open help. Loading, empty, and discovery-error states remain actionable.
- Passed `make check`, `go test -race ./...`, `make test-terminal`, and `git diff --check`.
- The unchanged baseline failed the real-PTY navigation matrix (0/12); the final binary passed 12/12 cases: 0/1/3/12 bookmarks at 80×24, 60×21, and 120×40. Exercised forward/reverse traversal, clamped boundaries, details and help returns, j/k, SGR wheel input, wizard cancellation, and local workspace activation. Disposable fixtures remained unchanged; no real remote operations were performed.
- The chooser supports 60×21. The existing wizard requires 22 rows: verified its resize notice at 21 rows, then grew to 22 to exercise setup and cancellation.
- Inspected the actual Go binary rendered through ttyd, not just the HTML mockup. Final screenshots cover default, both actions selected, empty, long-list, compact, loading, and error states. Inspected the interaction recording and its 5fps contact sheet.
- Evidence lives in `.amp/in/artifacts/terminal-opening-*`: before/after logs, eight state screenshots, and the navigation recording.
- Shipping authorized subsequently on 2026-10-24: commit all changes, fetch/rebase onto origin/HEAD (main), run the full LazyFTP test suite, then push. Stop for substantive conflicts. Delivery result is recorded in the thread.

## Notes for a fresh session
- Request from parent thread https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937. User subsequently approved the mockup and implementation, waived remaining gate pauses, and requested real terminal verification. Do not ship until asked.
- Scope is only the LazyFTP terminal opening chooser: saved connections, New connection, Browse local files, and their navigation. Do not touch log-viewer or service-bus-explorer.
- Implementation authorization is scoped to this approved redesign, not earlier proof-of-concept approval waivers.
- Recommendation: one continuous selection sequence, with a restrained centered card, consistent one-cell gutters and one-row actions. Keep the approved small square logo and semantic theme colors.
- Gate 1 contract and screen states are in 01-product.md. The plain HTML mockup is mockups/opening-screen.html, also served as a reviewable portal from this thread.
- The HTML remains a throwaway browser simulation. Actual application verification must use the built Go binary in a PTY and rendered terminal.
- No credentials, real remotes, or existing saved connections may be changed for verification. Use disposable fixtures. Commit and push to main are now authorized; no manual release or deployment is requested.

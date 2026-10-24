# Status: LazyFTP file rename

- Gate 1 — Product: APPROVED 2026-10-24
- Gate 2 — Architecture: approval pause waived 2026-10-24
- Gate 3 — Program Design: approval pause waived 2026-10-24
- Gate 4 — Slice plan: approval pause waived 2026-10-24

## Slices
- [x] Inline editing and confirmation wired to the live rename contract.
- [x] Local and remote rename safety, error handling and regression tests.
- [x] Full checks and real terminal/backend verification; configured Ship delivery authorized.

## Notes for a fresh session
- Scope is the LazyFTP terminal app only; no MCP, log viewer or Service Bus changes.
- Request came from parent thread https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937; report proposals and decisions there, with matching findings in this thread's final response.
- The current request explicitly restores software-factory approval gates for this feature, despite the older proof-of-concept waiver in docs/plans/lazyftp/00-status.md.
- Parent relayed user approval of the mockup and explicit authorization to implement, fully test and Ship without additional gate pauses. Remaining design decisions are recorded in 02–04.
- Existing lowercase r retries unfinished transfers. Uppercase R is currently unused. Preserve lowercase r and assign uppercase R to source-file rename.
- Existing copy-conflict Rename changes a copy destination only; do not conflate it with source-file rename.
- Supported live backends are local files, FTP/FTPS, Azure Blob and S3 via rclone. The demo remains read-only; SFTP is not an established supported target.
- Gate 1 proposes file-only, same-directory rename, no overwrite, highlighted row as the single target, inline editing followed by explicit confirmation defaulting to Cancel.
- Product mockup is a static HTML design reference, not implementation or behavior evidence.

## Implementation and verification
- Uppercase R starts a rune-aware inline editor; lowercase r remains queue Retry. Copy-conflict destination rename is untouched. Highlighted row wins over marked-file selection.
- Enter reviews quoted old/new names and location; Cancel is highlighted by default. Confirmation wraps and scrolls, including at 60 × 12. Escape, outside click, close and Cancel never write. Known collisions return to inline editing; other errors refresh before a fresh R/review attempt.
- A confirmed bounded operation runs to completion with navigation and quit input isolated. Remote moves may partially succeed; no cancellation/rollback guarantee is made after confirmation.
- Local hard-link/remove prevents destination replacement but requires hard-link support and may leave both names on removal failure. Remote moveto uses --ignore-existing, not --immutable, then checks both names. Identical existing destinations are refused. External concurrent writers remain outside atomic no-clobber guarantees.
- PASS: make check; go test -race ./...; make test-terminal; make test-terminal-live (existing transfer tests plus new local-both-panes/FTP/Blob/S3 rename PTY checks); make test-integration (storage/UI/MCP).
- Acceptance checks verify exact binary/empty/Unicode bytes, old-name removal, distinct/identical collision preservation, cancellation, error recovery, R versus r, filter behavior, cursor editing, long-name review, mouse confirmation, and input isolation during running work.
- Actual terminal screenshots inspected: .amp/in/artifacts/rename-inline.png, rename-confirm.png, rename-error.png, rename-success.png and rename-confirm-narrow.png. Final rename-interaction.mp4 inspected through a 5-fps contact sheet; pointer crosses neighboring rows and both buttons, cancellation/collision/success are exercised. Independent reads confirm original bytes and collision target preservation.
- FTP, Azurite and SeaweedFS mechanics are verified locally. FTPS uses the same FTP adapter but TLS-specific behavior, real cloud identity/permissions and native macOS filesystems were not independently verified.
- No application outside LazyFTP or MCP permissions changed. Verification uses disposable owned local directories and isolated lab prefixes only. No deployment or package publishing is authorized or performed.
- Shipping rebase preserves newer upstream deletion, pane-focus and opening-screen work. Rename also waits for an in-flight deletion; running rename isolates deletion input. Public UI tests cover both orderings.

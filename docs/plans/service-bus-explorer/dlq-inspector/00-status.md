# Status: DLQ inspector and message copying

- Gate 1 — Product: APPROVED 2026-10-24, mockup as shown
- Gate 2 — Architecture: authorized 2026-10-24, recorded in 02-architecture.md
- Gate 3 — Program Design: authorized 2026-10-24, recorded in 03-program-design.md
- Gate 4 — Slice plan: authorized 2026-10-24, recorded in 04-slices.md

## Slices

- [x] Slice 1 — unified reader, cursor, focus and selection
- [x] Slice 2 — real clipboard and exact full-content loading
- [x] Slice 3 — inspector-local resend, responsive rendering and safety
- [x] Slice 4 — integrated terminal, broker and visual verification

## Notes for a fresh session

- This amendment affects only the independent Service Bus Explorer application.
- Read `01-product.md` and all HTML mockups in `mockups/` before continuing.
- Recommend one inspector for reading, selection and copy; Enter focuses it.
  Keep an explicit safe resend review; do not reuse Enter as a mutation shortcut.
- Mockups are static HTML, not terminal or broker verification evidence.
- User approved the shown mockup and explicitly authorized proceeding through
  all remaining gates without further pauses, including full real-terminal
  verification. Do not ship, push or merge until asked. Report to parent.
- Parent: https://ampcode.com/threads/T-01a16f8c-3573-7217-a851-a7c24d29a937
- Proposal thread: https://ampcode.com/threads/T-01a16f9f-2222-700a-9739-28a0f45be4e6
- Review portal: https://t-03h3zgdaundgxegt5dpngpome-p24185.onamp.dev/index.html
- Validated four HTML pages with the artifact checker (no errors); inspected all
  nine rendered states. Both compact references contain 24 rows, at most 76
  text columns. Desktop review page fits 1280 px; corrected narrow keyboard
  reference fits a 390 px viewport with no document overflow.
- Screenshot sheets: `.amp/in/artifacts/sbx-inspector-design/`. Static design
  validation only; no clipboard, terminal-input or broker behavior was exercised.
- Implementation and design files remain local and uncommitted. Nothing has
  been pushed, merged, deployed or shipped. LazyFTP and log viewer are untouched.

## Implementation verification — 2026-10-24

- `make test`: all Go packages passed with race detection; 10 Python lab guard
  tests passed. `go vet ./...` and `git diff --check` passed.
- `make terminal-test PYTHON=/home/user/workspace/repo/.venv/bin/python`:
  real fixture PTY passed. Decoded OSC52 output independently verifies logical
  line, visual-line and whole-body bytes, in addition to focus/resend controls,
  exact-selected cleanup, receive-next, settings restart and compact resize.
- User explicitly authorized Microsoft emulator/SQL Edge license acceptance.
  Started the loopback-only disposable Docker lab, installed the missing Compose
  plugin, and ran the existing guarded seed with explicit disposable flags.
- `make integration-test PYTHON=/home/user/workspace/repo/.venv/bin/python`:
  real broker suite and real-emulator PTY passed serially. Includes exact 90 KB
  full reads in Active/DLQ, neighbor substitution rejection, full-content resend,
  selected-only cleanup, subscription-to-queue and existing topic fan-out control.
- UI regression tests exercise public keystrokes and clipboard outputs: Unicode
  graphemes, mixed/CRLF endings, reversed character/line ranges, soft wrapping,
  word-end/preferred-column movement, exact Cancel-default full loading, stale,
  missing and cancelled reads, focus/resize/help, and failure-retained selection.
- Clipboard subprocess/PTY tests cover native writer success/failure/timeouts,
  exact control/newline/UTF-8 bytes, remote preference, tmux/screen and 1 MiB
  boundary. No clipboard read-back or broker authority is used for copying.
- Inspected real ttyd renders against the approved static mockups: baseline
  duplicate screen; unified normal/character/line modes; inspector-local
  configuration, Cancel-default review and outcome; 80×24 and 60×16; themes and
  ASCII. Artifacts: `.amp/in/artifacts/sbx-inspector/` at repository root.
- Recorded actual moving-pointer and keyboard focus/selection/copy/review/cancel
  interaction; inspected its frame contact sheet, not only its final frame.
- Limitations: OSC52 acceptance is intentionally unconfirmed; native macOS
  clipboard execution and terminal screen-reader compatibility were not tested
  on this Linux orb. Long compact descriptions/status are ellipsized; full
  identity/description remains available in the selectable metadata document.
- Local Docker lab data is disposable and has been mutated by verification;
  reseed before rerunning integration tests. No cloud/production state touched.

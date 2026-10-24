# Status: Service Bus Explorer terminal UI

- Gate 1 — Product: APPROVED 2026-10-23
- Gate 2 — Architecture: APPROVED 2026-10-23 for independent app/lab, layered testing and independent-first reuse boundary
- Gate 3 — Program Design: APPROVED 2026-10-23; SDK-internal recovery accepted 2026-10-24 (no application-level retries)
- Gate 4 — Slice plan: APPROVED 2026-10-23

### Selected-message extension — 2026-10-24

- Product change approved by Andreas: select/open a DLQ message, choose a queue,
  resend a copy, optional “Delete original after confirmed resend” checkbox off
  by default. Existing delivered Slice 4 remains unchanged.
- Gate 2 amendment: **APPROVED 2026-10-24** after peek/copy vs locked deletion
  clarification. Exact-selected cleanup uses bounded receive-and-match; unrelated
  temporary locks and delivery-count effects are disclosed. The earlier scan
  prohibition is superseded only for this flow.
- Gate 3 amendment: **APPROVED 2026-10-24** in `03-program-design.md`:
  full-content prepared copy, same-namespace destination picker, single-use send,
  five-second/100-message search limit, bounded release and no send on uncertain
  nonmatch release. Tests are specified before implementation.
- Gate 4 amendment: **APPROVED 2026-10-24** by Andreas's explicit instruction
  to implement all of it in one go and ship once confirmed working. Proceed
  through all extension checkpoints without additional approval stops. Frequent
  scoped main commits/pushes remain authorized; no deployment/cloud writes.
- Extension complete: fixture flow, real backend and final rendered interaction
  verified. Implementation pushed to main; Base Slice 5/MCP remain separate.
- [x] Selected-message fixture workflow: exact highlighted row, queue picker,
  optional cleanup, frozen final review and correct retained/deleted outcomes.
- [x] Real backend: exact peek preparation, full-content replay, bounded selected
  acquisition and separate cleanup. Two fresh-seed serial emulator runs pass,
  including 90 KB bytes/scalar properties/TTL, queue and subscription sources to
  a different queue, original retention and independently received neighbors.
- [x] Final evidence index, inspected theme/compact/keyboard tour and presentation.
- Latest race suite, vet, ten Python guards and fixture PTY pass. Real-emulator
  PTY exercises both selected copy and exact-selected cleanup as well as legacy
  receive-next/release/resend/typed delete. No cloud writes or SDK patch.
- Rendered 80×24 configuration initially hid cleanup warnings. The final
  horizontal three-button configuration bar restores destination and the full
  scan-budget/delivery-count warning; all-theme/ASCII frame tests guard it.
- Final render inspection also caught misleading lock-release help in copy-only
  review and confirm help on the read-only body page. Both corrected, with
  public rendered-state regressions; race/vet/guards/fixture PTY passed again.
- Inspected Midnight copy review/outcome, Gruvbox review, Light 80×24 unchecked
  and checked configuration plus locked review, Midnight 80×24 and 160×24
  review/body pages. Critical controls remain visible; compact status lines
  intentionally ellipsize. Inspected final keyboard/pointer video and every
  frame at 5 fps, including checkbox change and confirmed separate cleanup.
  Pointer movement is not a claim of terminal mouse activation.
- Evidence and successful logs are indexed in this orb at
  `.amp/in/artifacts/service-bus-selected/README.md`. Video/captures use the
  offline fixture; real SDK byte/neighbor/cleanup claims come from integration
  assertions, not screenshots. Temporary preview/browser stopped after capture;
  isolated disposable broker services remain running. Seed before baseline reruns.
- Deletion is not an atomic move. Unknown send retains the original; no
  application retries or exactly-once guarantee. Selected cleanup has a bounded
  100-message/five-second search, with independently bounded release/close;
  missing/locked/out-of-budget selection never falls back to another message.

## Project boundary

This is an independent side project in the same repository as LazyFTP, not a LazyFTP feature. Its documentation lives exclusively in this directory and its application in `service-bus-explorer/`. Do not change LazyFTP code, documentation, plans or mockups while working on this project. Shared code extraction requires a separate explicit decision; do not assume shared runtime, credentials or configuration.

## Notes for a fresh session

- Slices 1–4 run as an independent terminal application with explicit `--fixture` or `--emulator` selection. The app's emulator connection is real: browsing remains read-only, with explicit reviewed single-message DLQ actions. No cloud connection or MCP server exists. Runtime emulator counters remain unavailable, never synthesized from seed or peek results.
- Read all documents in this folder before continuing, including `02-architecture.md`, `local-development-notes.md` and `mockups/README.md`. Preserve the distinction between user requirements, recommendations and open decisions.
- Current dashboard default has expanded collapsible navigation, 12 visible grid boxes, optional denser table, and queue inclusion editing behind Settings. Twelve is not a watchlist limit.
- Preserve read-only peek defaults and explicit mutation review. Resending is not an atomic move; send and original cleanup are separate effects.
- Linux and macOS are initial targets; preserve Windows portability. Start local development with connection strings; Microsoft sign-in follows.
- Fast local feedback is a core requirement: deterministic fixture tests plus seeded broker integration, with commands to send, drain, dead-letter and reset disposable data.
- Topic-subscription replay sends to the original topic with an explicit warning about delivery to other matching subscriptions. Queue replay sends to the original queue.
- User approved receive-next DLQ acquisition followed by review of the actual locked message. A peek-selected message cannot be acquired by sequence using the supported SDK contract. Never imply that acquiring next acts on the selected preview; cancellation/release can affect delivery count.
- Slice 4 is complete with final evidence below. User explicitly accepted trusting SDK recovery rather than patching it. No app-level send/settlement/replay retries; SDK recovery/fallback may repeat wire attempts. Do not promise exactly-once. Bulk operations remain deferred. MCP has its own parallel product proposal, not UI-scope implementation authorization. Ask before continuing Slice 5.
- Current orb's Docker daemon and reusable emulator run as supervised services `servicebus-docker` and `sbx-explorer-lab`; actual RAM is 7.8 GiB. Private generated lab credentials are outside Git at `~/.local/share/sbx-explorer-lab/`. Earlier `servicebus-lab` / ~4 GB observations below describe the old capability-spike orb, not this one.
- Messaging/discovery/queue replay/topic fan-out/drain probes passed. JavaScript SDK count reads failed to parse missing details; the .NET SDK returned incorrect zeros. Do not fake aggregate dashboard totals from bounded peeks or seed expectations.
- Architecture selects a separate Go module and terminal UI under `service-bus-explorer/`, with direct Azure SDK access and its own reproducible lab commands. Read `04-slices.md` for the approved sequence. Go SDK 1.11.0 discovery and message browsing now run against the emulator; cloud access remains unimplemented.
- User agreed to independent-first development: adapt proven small theme/layout/testing patterns, extract a minimal shared library only once common contracts stabilize. Gate 3 pins the FTP reference revision; no live dependency, FTP migration or framework now. Do not change LazyFTP files or contact its thread for updates.
- Approved Gate 3 includes session-only initial credentials, 160×48 dashboard acceptance, 30-second bounded count refresh and explicit unknown mutation outcomes. Replay uses a new message ID, immediate enqueue and renewed original TTL; original cleanup defaults off. Initial mutations exclude sessions and advanced AMQP representations.
- User requested parallel implementation where useful. Gate 4 records disjoint lab/theme task ownership; main thread integrates and verifies. Do not send work or updates to the FTP thread.
- Future agent MCP must deny by default and separately allowlist reads and optional resends. Dashboard visibility must not grant agent access.
- User confirmed parallel Service Bus MCP work, prioritizing finishing the current UI result first. Separate proposal: `../service-bus-mcp/01-product.md`. It adds independent DLQ deletion grants and queue-only resend destinations; lock/acquisition authority and read-only-first launch need product review. No MCP code exists.
- Slice 1 commands from repo root: `make -C service-bus-explorer run-fixture`, `test`, `terminal-test`. Go 1.27.3 required; Python/pyte setup in the app README. Current orb has `go` 1.27.3 on PATH (`/home/user/.local/bin/go`); the earlier `/home/user/.local/toolchains/go/bin/go` override belongs to the old orb.
- Verification: race-enabled Go tests, `go vet`, and real-PTY navigation/layout/paging/resize/exit checks passed. Inspected actual dark-theme renders for grid/table, collapsed sidebar, Active/DLQ inspector and compact layout; saved keyboard recording under workspace `.amp/in/artifacts/service-bus-slice1/`. No claim of Azure/native macOS/Windows verification.
- Slice 2 adds `run-emulator`, `integration-test`, and `lab-up/status/down/reset/seed/send/peek/dead-letter/drain`. See app README for explicit EULA/confirmation flags. This orb needs `LAB_FLAGS="--sudo --orb ..."`; pass the Go override above. `lab-seed` always destroys/recreates only the disposable project's data.
- New reusable lab: supervised service / Compose project `sbx-explorer-lab`, fixed loopback AMQP 5673 and management 5301. SQL credentials live outside Git at `~/.local/share/sbx-explorer-lab/compose.env`, private permissions. The older 5672/5300 spike was not changed.
- Verified two complete reset/seed runs after pacing successful batches (first unpaced run correctly stopped on broker throttling/unknown send). Repeated seeded Go integration + real-emulator PTY passed: 14 resources, exact bytes/properties, Active/DLQ/subscription pages, explicit refresh/cursors, and independent receive after peek. Actual lab CLI send → repeated peek → dead-letter → bounded drain reported 1 then 0; temporary probe was removed.
- Queue seed data follows the fixture. Topic fanout uses one eight-message publication cohort: Billing 7 Active/1 DLQ, Shipping 5 Active/3 DLQ. These are independently checked seed expectations, not runtime counts or independent subscription sends. Seed also includes text/control-sequence bodies. Security/guard tests and the original fixture PTY control pass.
- Inspected real emulator dashboard/Active/DLQ screenshots and keyboard-recording contact sheet in workspace `.amp/in/artifacts/service-bus-slice2/` (brief index included). Temporary browser capture service stopped; seeded lab remains running. No FTP files were authored.
- Slice 3 exposes Midnight, Gruvbox and Light through Settings and fuzzy Ctrl+P actions. Adds a collapsible full-identity entity tree, visible-pane focus cycling, independent inspector scrolling and runtime `--ascii` fallback. Printable palette text owns q/j/k; Escape restores prior focus. No mutation actions exist in the palette.
- Versioned, validated nonsecret preferences use the platform user config directory (or explicit `--config`). Explicit Save atomically persists private files; unsaved edits are marked. A save owns its snapshot, so a late completion cannot claim newer edits were saved. Malformed/unsupported existing files are not overwritten.
- Watchlists key full profile/namespace/entity identity; absent profiles initially watch all discovered resources, explicit empty lists watch none. Queues/Topics/tree still browse unwatched resources. Saving watchlists never grants MCP access and stores no credentials or message bodies.
- Watched counts refresh 30 seconds after each bounded sweep; at most four reads, no overlapping sweeps, ten-second sweep context. Failure retains prior values marked stale (~), with observation time/error details; unknown is distinct from zero. Identity-keyed count replies cannot change message selection or page. The emulator limitation still renders unavailable.
- Slice 3 checks: race-enabled Go suite, vet, Python lab guards, expanded fixture real-PTY save/restart/theme/palette/resize checks, seeded Go broker integration and emulator PTY. A temporary production-timer probe fired after 30.03 seconds and refreshed all 14 resources without input; removed after validation to avoid permanent slow tests. Public UI tests cover same-name namespaces, empty watchlists, 29-resource table, all themes/ASCII widths, controlled stale reads, tree groups and inspector scrolling/control-text safety.
- Actual Slice 3 captures/keyboard tour live in workspace `.amp/in/artifacts/service-bus-slice3/`, with a reference index. Default/theme/Settings/empty/narrow/focused-inspector states were inspected; stale-count rendering uses a deliberately failing offline fixture wrapper, not a claim about cloud counters. Temporary capture helpers are removed and services stopped after verification.
- Source design conversation: https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1
- Current continuation: https://ampcode.com/threads/T-01a16e75-df92-720e-b3c2-4052f34bb7f5. Deliver to Andreas here, not the FTP thread.
- User authorized implementation, frequent commits/pushes and pulling when needed, including delegated threads; put that instruction in handoffs. Use disjoint ownership and separate branches for independent threads, integrate with checks, never force-push. No Azure writes/deployments are authorized.

## Slice 4 continuation handoff — 2026-10-24 (historical; completed below)

- Andreas requested a good checkpoint committed/pushed to `main`, then a new
  thread using the newly configured 8 GB orb settings. The old orb still reports
  3.8 GiB; do not claim it resized. New orbs do not inherit this orb's services,
  toolchain, temporary files or evidence. Use the configured project default.
- Code is now present for one held delivery, actual-message review with default
  Cancel/cleanup off, resend-to-origin, typed delete, bounded release/expiry,
  terminal confirmed/partial/unknown outcomes and no application retry. Azure
  verifies session metadata before acquisition and rejects unsupported replay
  representations. Fixture backend is concurrent-safe and exercises the same UI.
- `make test terminal-test GO=/home/user/.local/toolchains/go/bin/go` and
  `go vet ./...` passed at the handoff checkpoint. UI tests include differing peek
  selection/acquisition, accepted-but-lost-ack send with no cleanup, failed cleanup,
  single-flight submission, expiry, late-acquisition release and pending-quit
  outcome preservation. PTY covers retained-original resend and typed delete.
- After a fresh reset/seed at 500 ms pacing, `make integration-test` also passed:
  exact 90 KB body/scalar properties, source/replay 37-minute TTL, new ID/correlation,
  cleared schedule/DLQ fields, retained original, separate Complete, topic fan-out
  to both subscriptions, and real emulator PTY cancel/release/resend/typed delete.
  Final outputs are in the source thread artifact directory below as
  `fixture-checks.log`, `emulator-checks.log` and `seed.log`. The tests leave
  deliberate topic copies and the PTY deletes one acquired Orders DLQ message;
  reseed before repeating exact baseline assertions. No cloud validation occurred.
- Live broker debugging found binary application properties rejected by emulator
  serialization (broker log names `System.Byte[]`), misleadingly mapped to SDK
  `ErrMessageTooLarge`. The integration test asserts rejection with a one-byte
  body, then uses string/bool/int properties for the 90 KB binary-body round trip;
  binary-property cloning remains unit-tested. No claim of broker binary-property
  preservation is justified by that unit test.
- The first round-trip test's requested 90-minute TTL was capped at one hour.
  The actual queue management response confirms `DefaultMessageTimeToLive=PT1H`.
  Integration now requests 37 minutes and independently checks the source and
  replay TTL, rather than deriving expected values from the conversion helper.
- A reseed hit namespace throttle 50002 at the previous 100 ms pacing. It stopped
  on the unknown outcome, not resumed/retried. Acknowledged 20-message lab batches
  now pause 500 ms. Recreate/reset before repeating broker checks; never continue
  a partial seed or invent totals. Run integration/PTY/captures serially.
- The fixture PTY's old fixed-sleep `j` sequence proved flaky under load; it now
  awaits each selected Settings row. Actual space key events no longer insert
  two spaces in palette queries. The typed-delete field owns `q` only during
  review, not on the terminal result screen. Release wording now explicitly says
  **Lock release**, distinct from retention of the original message.
- Intermediate inspected fixture screenshots and a keyboard/pointer recording
  live in the source thread at absolute path
  `/home/user/workspace/.amp/in/artifacts/service-bus-slice4/` with `README.md`.
  Use `download_thread_file` from the source thread if useful. Recording shows
  older “Original release” wording and needs recapture before final delivery.
  Final emulator, theme/compact review and unknown/expiry captures remain pending.
- Capture recipe: temporary ttyd 1.7.7, explicit port 7682, DOM renderer, 14px
  monospace; 1380×790 browser viewport at DPR2 yields 160×48. 700×406 gives 80×24.
  Hover-capable Chrome wrapper and official action-video helper worked with
  recording at **5 fps / 1100×680 / DPR2**; 10 fps at 1380×790 overloaded encoder.
  Inspect final screenshots and a 5-fps video contact sheet. Preview service and
  browser are stopped; do not preserve an obsolete portal as the live app.
- Finish Slice 4 verification/evidence, update its checkbox only once proven,
  commit/push scoped changes, show actual screenshots/recording and ask whether
  to continue to Slice 5. Do not restart approved gates or patch the SDK. Do not
  contact the FTP thread or modify its files. MCP remains separately gated.

## Slice 4 completed — 2026-10-24

- Continuation orb used the project's configured default size and reports
  **7.8 GiB RAM** (`free -h`), unlike the old 3.8 GiB orb. Go 1.27.3 and ttyd
  1.7.7 were already installed. Docker daemon was started as supervised
  `servicebus-docker`; missing Compose plugin was installed outside Git as
  v2.39.4. The initial seed attempt stopped before messaging because Compose
  was absent; after installation, the guarded reset/seed completed from scratch.
- Repeated `make test terminal-test PYTHON=/home/user/workspace/repo/.venv/bin/python`
  and `go vet ./...` in `service-bus-explorer/`: all race-enabled Go tests,
  ten Python guards, fixture PTY and vet passed. Then ran
  `make lab-seed LAB_FLAGS="--sudo --orb --accept-eula --confirm-disposable"`
  and serial `make integration-test PYTHON=/home/user/workspace/repo/.venv/bin/python`:
  Go broker checks and real-emulator PTY passed. No integration consumers ran
  concurrently. The body/scalar-property, independent TTL, retention, separate
  cleanup and topic fan-out assertions passed again on this new orb.
- Inspected actual 160×48 held-message review in Midnight, Gruvbox and Light,
  plus 80×24 Light review and its read-only body page. Default Cancel and
  cleanup-off controls remain visible. The compact status line deliberately
  ellipsizes; full action help and critical review controls remain readable.
  Actual emulator captures cover Orders review/cancel/release, typed delete,
  confirmed deletion and Billing subscription review with topic fan-out warning.
- Inspected fixture confirmed-send/retained-original outcome, and deterministic
  unknown-send, expired-lock and confirmed-send/uncertain-cleanup renders.
  These last three use temporary offline Store/Delivery wrappers, not induced
  broker failures. Unknown send simulation first accepts the fixture copy,
  then hides its acknowledgement. Expiry simulation shortens the presented
  deadline; partial simulation accepts the send but reports cleanup uncertainty.
  Production behavior and permanent regression tests were not changed.
- Recaptured the real keyboard/pointer tour at 1100×680/DPR2, 5 fps. Inspected
  every frame through a 5-fps contact sheet: palette → actual held-message review
  with Cancel default → Tab to resend → confirmed send, original retained and
  **Lock release confirmed**. Pointer passes across neighboring terminal rows.
  The TUI is keyboard-first; pointer movement is browser capture evidence, not
  a claim of mouse-activated TUI controls.
- Final evidence in this continuation orb:
  `/home/user/workspace/repo/.amp/in/artifacts/service-bus-slice4/README.md`
  indexes 14 inspected screenshots, `review-tour.mp4`, `review-tour.loop.webp`,
  `sbx-fixture-checks.log`, `emulator-checks.log` and `seed.log`. The files are
  thread artifacts, not tracked source files; this thread's final presentation
  embeds the renders and recording.
- Removed temporary Go capture harness; all five ttyd preview services/browser
  sessions stopped. Disposable `sbx-explorer-lab` and `servicebus-docker` remain
  running. The integration/PTY and subsequent visual delete leave replay copies
  and remove two reviewed Orders DLQ messages in total; reset/seed before exact
  baseline assertions. No cloud writes, SDK patch, root/LazyFTP changes or FTP
  thread contact occurred. Native Mac/Windows, Azure counters/auth and broker
  binary-property preservation remain unverified. MCP remains separately gated.
- Slice 4 requires no further implementation. Next decision: continue Slice 5
  portable developer handoff/polish, or re-steer; do not start it implicitly.

## Compact button styling — requested follow-up, 2026-10-24

- Andreas selected mockup H (option A in the conversation): compact horizontal
  terminal buttons, not the action rail. Implementation authorized separately
  from the remaining Slice 5 portability work. Continue to commit/push scoped
  verified improvements early so Andreas can test while work continues.
- First checkpoint: bordered Active/DLQ/Refresh/Actions shortcut toolbar and
  a bottom review bar with horizontal Cancel/Resend/Delete controls. Cancel is
  focused by default; cleanup stays unchecked; typed delete and keyboard order
  are unchanged. No mouse mode, broker action or retry behavior was added.
- Review body becomes a separate read-only page at short heights as well as
  narrow widths; confirmations hidden by that page cannot submit. Tests cover
  all three themes, Unicode/ASCII and sizes around the 35-row split boundary.
  Toolbar space is accounted for in inspector scroll bounds.
- Completed and pushed the compact controls and theme-gap correction in separate
  checkpoints. Initial frame tests caught unpadded toolbar rows; actual Light
  captures caught dark strips between buttons. Both were fixed before final
  validation; final screenshots replace intermediate captures.
- Race tests, ten Python guards, fixture PTY and vet pass, including Enter
  rejection on the short-height read-only body page. Fresh guarded lab seed,
  serial Go broker integration and real-emulator PTY also pass. No application
  retry, SDK patch, cloud write or mutation contract change was introduced.
- Inspected final Midnight browse/review, Gruvbox and Light review, Light browse,
  80×24 review/typed delete, 160×24 review/body page, and focused resend,
  checked-cleanup and typed-delete states. Compact status text intentionally
  ellipsizes; action controls and safety warnings remain visible.
- Inspected the 5-fps real-pointer/keyboard tour and every-frame contact sheet:
  browse shortcuts → default Cancel → Resend focus → cleanup on/off → typed
  Delete → Escape → canceled outcome with Lock release confirmed. Final-frame
  inspection independently verifies the release result. Pointer movement does
  not imply mouse activation of terminal controls.
- Evidence is indexed in this orb's
  `.amp/in/artifacts/service-bus-buttons/README.md` (12 final screenshots,
  keyboard/pointer video and successful check/seed logs). Preview service and
  browser are stopped after capture; disposable broker services remain running.
  Seed again before exact baseline assertions. This UI follow-up is complete;
  remaining Slice 5 portability/handoff work still needs Andreas's go-ahead.

## Slices

- [x] Slice 1 — runnable fixture workspace
- [x] Slice 2 — real local broker browsing and reproducible lab
- [x] Slice 3 — complete read-only workspace and themes
- [x] Slice 4 — reviewed single-message DLQ actions
- [ ] Slice 5 — portable developer handoff and polish

Approved sequence. See `04-slices.md` for checks and ownership. Show actual terminal captures/recordings when results are ready.

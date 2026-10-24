# Gate 4 — Vertical slices

Approved by Andreas on 2026-10-24 (“Go for it”). Gates 1–4 are approved.
Only Slice 1 is authorized for the first implementation checkpoint; see
`00-status.md` for delivered work and verification. All implementation paths below
are relative to `log-viewer/`. Target contracts remain in `03-program-design.md`.

## Build order

1. **Tracer bullet — fixture workbench:** run `logview --fixture` through the real investigation service into a Midnight terminal log list and right inspector, with deterministic records, safe output, navigation, resize and clean exit.
2. **Real local Elastic happy path:** install/check the pinned native node, seed an owned authenticated TLS lab, and open its logs/inspector with the actual Elastic adapter and read-only key through the same service/UI.
3. **Find the relevant logs:** add app-name suggestions/exact multi-select, time/level/message filters, local literal find, stable older-history paging and explicit private saved searches, exercised against fake and real Elastic.
4. **Excellent inspector and action discovery:** add field-tree expansion, full safe `y`/`Y` copy, Ctrl+P fuzzy/contextual actions, keyboard help, independent panel toggles and explicit clipboard availability/feedback.
5. **Investigate around the event:** add scoped context, exact trace filtering, polling follow/pause/resume, stable identity merging, bounded catch-up and honest ingestion/gap/partial states.
6. **Permissioned assistant access:** run `logview-mcp` over stdio with the five approved read tools, explicit source/field/operation grants, scope-bound handles, revocation and bounded redacted output against the real local lab.

No horizontal “all models”, “all adapters”, “all tests” or “polish at the end”
phases. Each slice ships one usable path with its own checks. Add files/dependencies
only when that slice needs them. Fixture and live modes must always be unmistakable.

## Acceptance and evidence

### Slice 1 — fixture workbench

- Build the independent module and minimum provider/service/output path; read
  through that service, never wire the UI directly to the fake. Start with source
  output restrictions and redaction; don't defer terminal safety to a later slice.
- Show fictional data, one clear focus owner, a readable multiline event and the
  static field inspector. Both sidebars use the approved full-height layout.
- Implement only the minimal navigation needed to inspect events, resize and quit.
  Later features show an honest unavailable explanation; no simulated successful
  copy, live follow, backend query or MCP grant.
- Verify `make check test-terminal`: expected event/inspector contents, no canary
  or terminal-control leakage, focus recovery on resize, clean process exit.
  Inspect actual rendered 160×48 and narrow terminal states; show screenshots and
  a keyboard-interaction clip. A Painter board is not acceptance evidence.

### Slice 2 — real local Elastic happy path

- Hash the downloaded archive against the approved pin before extraction. Complete
  non-root/descriptor/memory preflight and measure startup, readiness and memory.
  Use the supervised private loopback service; no public portal or host sysctl.
- Create the local CA/cert, fixture mappings and separate bootstrap/read-only
  credentials under private owned lab state. Viewer can neither write the owned
  index nor read the forbidden target. No arbitrary configured live URL in lab tools.
- Run `make lab-install lab-up lab-seed lab-check`, then `make test-integration`
  and `make run-lab`; prove source → real HTTP query → safe service → rendered
  log → inspector. A real-lab PTY check must distinguish live mode from fixture.
- Report the measured budget. If startup requires changing approved assumptions,
  revise the affected design before promising support. Do not silently skip the
  integration checks. Optional Docker packaging stays deferred until its digest
  and equivalent checks are verified; it does not block the native lab.

### Slice 3 — search and history

- Exercise app OR/facet AND, Unknown severity, source message phrase versus local
  substring find, explicit UTC/time-zone offsets and half-open time boundaries.
- A 501+ equal-timestamp dataset must page every unique event exactly once;
  inserting new data must not destabilize PIT history. Keep event selection
  anchored while loading older rows; close/expire read contexts correctly.
- Saved searches and preferences write only on explicit user action to private
  configuration. Never persist event payloads, credentials or automatic query history.
- Run unit/race checks, real Elastic integration and PTY filter/search/paging
  scenarios. Render and inspect active-filter, empty, partial/error and narrow
  states. Persisted searches must restore the same source/filter semantics.

### Slice 4 — inspector and palette

- Copy a long fully available value, not its on-screen ellipsis; refuse a false
  completeness claim for an actually truncated field. Check scalar, object,
  nested array, redacted field and clipboard failure. Never invoke a shell with
  log data. Inspect the receiving clipboard where supported; otherwise distinguish
  captured payload/OSC52 transmission from actual clipboard receipt.
- Ctrl+P owns modal input/focus, finds relevant actions, explains unavailable ones
  and returns to the previous panel. Text input must not trigger `q`, `y` or Space
  shortcuts. Panel collapse/resize cannot trap focus.
- Run PTY copy/palette/help scenarios and inspect default, field-focused and
  palette-open renders. Record the keyboard interaction, including failure feedback.

### Slice 5 — context and follow

- Context includes relevant INFO neighbors but excludes another instance/app;
  it preserves source/security constraints and restores the original search.
  Trace filtering can span services only inside the permitted scope.
- Append and delay refresh of real events, including repeated messages with
  distinct IDs and late timestamps. Prove polling sees them, overlap deduplicates
  only actual identities, pause stops collection and resume discloses incomplete
  catch-up. Fake-clock tests cover timing/backoff deterministically.
- Exercise 429, partial results, cancellation, stale completions and buffer limits.
  Render context, follow, paused and degraded states; inspect a recording of the
  real append → follow → pause → resume path. No “lossless tail” claim.

### Slice 6 — MCP reads

- Start the actual stdio binary with empty policy, then explicit lab-only grants.
  No grant means no source discovery/data, even with usable Elastic credentials.
  Test all five tools and strict malformed/oversized input rejection.
- Prove field allowlists cover normalized values and native fields, context never
  broadens authority, foreign/expired handles fail, policy changes revoke future
  reads, and cursor advancement is safe under concurrent calls.
- Check output row/byte/time limits, secret canaries, sanitized backend failures
  and untrusted prompt-like strings. Stdio stdout must contain protocol only.
- Run the complete module checks, fixture/live PTY checks and authenticated real
  lab integration, including the actual MCP process. Report what local Elastic
  proves and what remains unverified about the company's real provider/network.

## Execution and delivery rules

After Gate 4 approval, implement **Slice 1 only** first. At each slice boundary:
verify the usable path, update `00-status.md` with exact commands/results and any
limitations, commit/push only scoped authorized work, then ask “Continue to slice
N+1, or re-steer?” Do not silently treat plan approval as permission to skip these
checkpoints or to mark future tests passing. No deployment or publishing.

The production provider/version is still unidentified. It does not block a clearly
labeled local Elastic lab, but a production-connectivity claim requires identifying
and exercising the actual source with appropriate read authorization. Kubernetes,
Loki, Azure Monitor, cross-provider merge and final branding remain outside these
slices; add the second real provider as a separately scoped continuation.

Gate 4 approval does not bypass the continuation question at each slice boundary.

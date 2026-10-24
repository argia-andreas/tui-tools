# Vertical slices: Service Bus Explorer terminal UI

Gate 4 approved 2026-10-23. All design gates are approved. Build and verify one runnable slice at a time below.

## Build order

1. **Runnable fixture workspace:** independent Go module/binary, visibly offline dashboard with expanded/collapsible navigation, 12-card grid/table and one resource's Active/DLQ preview wired end to end; prove keyboard navigation and resize in a real PTY, inspect the rendered dark-theme result, and publish the project-local run command.
2. **Real local broker browsing:** reproducible pinned Compose lab, guarded reset/seed/send/peek/dead-letter/drain commands and Go SDK backend connected to the same UI; prove seeded message IDs/bodies, non-destructive peek, topic/subscription discovery, deterministic reseed and unavailable—not zero—emulator counters in serial integration and PTY runs.
3. **Complete read-only workspace:** Ctrl+P fuzzy palette, entity tree, optional body/metadata inspector, Settings-only watchlist editing, persisted nonsecret preferences, dark/warm/light themes and bounded refresh/error states; prove stale-response isolation, terminal-text safety and all representative layouts/themes with tests and inspected terminal captures.
4. **Reviewed single-message DLQ actions:** receive-next acquisition, actual-message review, resend-to-origin and explicit delete/optional cleanup, with lock expiry/cancellation, single-flight operations and confirmed/partial/unknown outcomes; prove byte/property preservation, retained-original behavior, separate cleanup and topic replay fan-out against the emulator, plus ambiguous failures with deterministic fixtures.
5. **Portable developer handoff and polish:** consolidate project-local commands/documentation and repeat the full fixture, PTY and emulator suites from clean state; inspect final responsive/theme states, build Linux/macOS targets and cross-build Windows, and document native-platform/cloud validation gaps without claiming unexecuted checks passed.

Each slice ends with a runnable result, relevant checks, an updated status checklist, a reviewable local commit and the question “Continue to slice N+1, or re-steer?” Do not turn the slices into horizontal layers with nothing runnable between them. If a slice invalidates an approved design decision, stop and seek re-approval of that decision rather than silently changing it.

## Selected-message extension — Gate 4 amendment approved, 2026-10-24

Product and Gate 2/3 amendments approved. This is a separate extension to the
delivered Slice 4, not permission to start the remaining Slice 5 portability work.
Keep the current compact button styling and all legacy receive-next behavior.
Andreas explicitly requested implementing all of this in one go and shipping
after confirmation it works; proceed without approval stops between checkpoints.

1. **Runnable selected-message fixture flow:** selected DLQ open → destination
   queue picker → cleanup checkbox → final review → observable copy/cleanup result,
   using full fixture messages and the approved Replay/Delivery contracts. Prove
   a non-first selection, different destination, original retained vs removed,
   Cancel/Back, missing selection and repeated Enter through public UI/fixture
   tests and actual PTY; inspect a representative rendered review. Real backend
   selected actions must remain explicitly unavailable until supported, never
   silently use receive-next or fixture data. Commit/push this usable checkpoint.
2. **Real selected-message replay and cleanup:** implement exact peek preparation,
   validated queue routing and bounded receive-and-match acquisition through the
   same UI. Fresh-seed serial emulator checks must prove 90 KB full content/scalar
   properties/TTL, non-first queue and subscription selections sent to a different
   queue, retained original for copy-only, only selected original completed for
   checked cleanup, and independently receivable neighbors. Race/vet/guards and
   existing legacy integration/PTY stay green. Commit/push the working checkpoint.
3. **Failure boundaries and final rendered verification:** finish the approved
   deterministic missing/locked/expired/canceled/unknown/partial/release-failure
   cases and 100-receive/five-second boundaries; no fallback send or unrelated
   completion. Exercise fixture/emulator PTY, all themes, 80×24 and short-wide
   body pages; inspect final screenshots and real keyboard/pointer tour. Document
   actual limits and repeat required suites from fresh seed serially. Commit/push
   final evidence/status and ask whether to return to Slice 5 or re-steer.

Each slice includes its own regression tests and failure handling needed to be
safe on main; the final slice is not permission to postpone cleanup ownership,
uncertain outcomes or safety guards from either earlier runnable checkpoint.
Record completion at each checkpoint; the user authorized continuous execution
of this extension, overriding the original per-slice approval stops here only.

## Parallel work ownership

The user requested parallel work where useful. Keep integration and verification in the owning checkout; use bounded subagents for non-overlapping implementation tasks. Create separate persistent threads only for independently owned work that actually needs them. Do not contact or delegate into the FTP thread.

- Main thread owns explorer contracts, UI orchestration, integration, project build commands, final evidence and status.
- After contracts and the fixture entry point exist, a lab task can own only `service-bus-explorer/lab/`, `scripts/lab.py`, `scripts/lab_test.py` and the lab CLI. It must consume the agreed scenario/contracts and leave UI/module/build files alone; required dependency/build edits return to the main owner.
- Theme adaptation can be a separate bounded task owning only `service-bus-explorer/internal/theme/` once semantic roles are fixed. Preserve provenance; no shared-library extraction or LazyFTP edits.
- Do not assign the same files to concurrent writers. Run combined verification before accepting delegated work; a task's completion report is not proof of the integrated application.

## Commit, pull and push discipline

The user explicitly authorized frequent commits/pushes and pulling when needed, including delegated threads. Put that instruction in every implementation handoff. Commit small coherent checkpoints after relevant checks; push regularly rather than retaining a large unpushed batch. Fetch/pull before starting and before integration/push; rebase unpublished local work where appropriate. Never force-push or rewrite others' published work.

Independent threads use their own feature branches and checkouts, not concurrent writes to shared `main`. Before integration, establish the exact branch/commit and required setup, inspect the changes and run combined checks in the owning checkout. A shared-workspace subagent may stage only its owned files and must coordinate commits with the main owner. Preserve unrelated files and staged changes. Push permission does not authorize deployments, releases or cloud writes.

## Scope and delivery limits

All implementation stays under `service-bus-explorer/`; plans stay here. Do not alter root setup/build files, LazyFTP code/docs/lab or credentials. Initial credentials remain session-only. Bulk operations, MCP, Microsoft sign-in, credential-vault persistence, session mutations and advanced AMQP replay stay deferred as approved in Gate 3.

Documentation and, once Gate 4 is approved, scoped application commits/pushes are authorized. Cloud writes, deployments and releases still need explicit authorization. Emulator lab work is disposable local development, not Azure connectivity proof. Native Mac execution needs a Mac; accurate live counters and Entra/RBAC need separately authorized Azure validation.

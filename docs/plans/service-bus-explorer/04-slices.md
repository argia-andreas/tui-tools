# Vertical slices: Service Bus Explorer terminal UI

Gate 4 proposal, prepared 2026-10-23. Gates 1–3 are approved. This is the last design approval before application implementation; no implementation files exist yet.

## Build order

1. **Runnable fixture workspace:** independent Go module/binary, visibly offline dashboard with expanded/collapsible navigation, 12-card grid/table and one resource's Active/DLQ preview wired end to end; prove keyboard navigation and resize in a real PTY, inspect the rendered dark-theme result, and publish the project-local run command.
2. **Real local broker browsing:** reproducible pinned Compose lab, guarded reset/seed/send/peek/dead-letter/drain commands and Go SDK backend connected to the same UI; prove seeded message IDs/bodies, non-destructive peek, topic/subscription discovery, deterministic reseed and unavailable—not zero—emulator counters in serial integration and PTY runs.
3. **Complete read-only workspace:** Ctrl+P fuzzy palette, entity tree, optional body/metadata inspector, Settings-only watchlist editing, persisted nonsecret preferences, dark/warm/light themes and bounded refresh/error states; prove stale-response isolation, terminal-text safety and all representative layouts/themes with tests and inspected terminal captures.
4. **Reviewed single-message DLQ actions:** receive-next acquisition, actual-message review, resend-to-origin and explicit delete/optional cleanup, with lock expiry/cancellation, single-flight operations and confirmed/partial/unknown outcomes; prove byte/property preservation, retained-original behavior, separate cleanup and topic replay fan-out against the emulator, plus ambiguous failures with deterministic fixtures.
5. **Portable developer handoff and polish:** consolidate project-local commands/documentation and repeat the full fixture, PTY and emulator suites from clean state; inspect final responsive/theme states, build Linux/macOS targets and cross-build Windows, and document native-platform/cloud validation gaps without claiming unexecuted checks passed.

Each slice ends with a runnable result, relevant checks, an updated status checklist, a reviewable local commit and the question “Continue to slice N+1, or re-steer?” Do not turn the slices into horizontal layers with nothing runnable between them. If a slice invalidates an approved design decision, stop and seek re-approval of that decision rather than silently changing it.

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

# Verification: Saved Service Bus environments

2026-10-24–25. Implemented, verified and integrated with the latest main; shipping authorized.
Evidence thread: https://ampcode.com/threads/T-01a16fa1-ebdf-72d9-bdfd-b97aa795af08.

## What was exercised

- Real built `sbx --fixture` binary through PTYs: Production created from a two-namespace `rg-core` preview plus a known namespace in another sample Azure subscription. Persisted membership contains only those three explicit namespace identities, not a resource-group rule.
- Apply without switching left the preferences file unchanged. Save persisted all applied preferences. New environments started Selected/empty; filtered bulk selection changed dashboard inclusion without expanding discovery.
- Staging had its own namespace and inclusion list. Restart at 80×24 retained Staging; switching back restored Production's different choices. Existing theme/layout, peek, selected DLQ copy/cleanup, receive-next/release and typed-delete PTY scenarios still passed.
- Public TUI regression tests exercise rename, duplicate, namespace exclusion and re-addition restoring retained selections, deletion without fallback, first-run, successful empty discovery, failed Save, partial/unverified versus missing resources, stale counts, late previous-environment counts and blocked switching during message review.
- Real ttyd terminal renders at 160×48, 80×24 and exactly 60×18 were inspected against the approved references. The 50×10 PTY cases exercise both clean quit and dirty Discard. Inputs retain q/j/k as text.
- A real keyboard/pointer clip switches Staging → Production, opens the single Settings dashboard-inclusion screen, opens full identity and returns to Dashboard. Pointer traverses neighboring rows; the terminal intentionally has keyboard-only controls, not clickable simulated buttons.
- On the final built binary, browser-driven duplication and deletion of an active Temporary environment showed Environment: None, a blank chooser query, and both Production and Staging still available. Reload restored the disk-backed Production selection; no temporary environment was saved.

## Decisive commands

Run from `service-bus-explorer/`:

```sh
go test -race ./...
go vet ./...
make terminal-test
python3 -m unittest discover -s scripts -p lab_test.py
git diff --check
```

All passed. The terminal target builds the binary and reports:

```text
PASS: real PTY — layouts/palette/themes, selected copy to queue, exact-selected cleanup, receive-next/review, cancel/release, retained-original resend, typed delete, save/restart watchlist, resize, clean exit
PASS: real fixture PTY — group snapshot, cross-subscription known namespace, Apply != Save, empty Selected, filtered bulk inclusion, input ownership, isolated environments/save/restart, 80x24/60x18/undersized quit
```

The Python lab suite ran 10 tests and reported OK. Azure package tests use injected CLI/HTTP behavior, including real administration SDK XML decoding and paging; they are not live Azure tests.

## Security and failure checks

Cloud tests cover explicit namespaces, per-namespace failure, original ARM identity, moved mappings before counts/peek, unsafe/cyclic and wrong-collection pagination, absent versus genuine-zero counts, sanitized errors and refused cloud mutations. Identity checks pin tenant plus immutable object ID; wrong principal, tenant and audience fail. JWT inspection is a consistency check on trusted CLI output, not signature validation or an authorization grant.

The review found and fixed local display-name/resource-group persistence mismatches, username-only principal pinning, same-origin metadata continuation escaping its selected collection, stale-result issues and a delete-confirmation query leaking into the switcher. The added edit/delete regression failed before that last fix and passed after it.

## Rendered evidence and corrections

Final captures are under `.amp/in/artifacts/service-bus-environments/` in the evidence thread:
`dashboard.png`, `switcher.png`, `editor.png`, `group-preview.png`, `inclusion.png`, `selected-empty.png`, `first-run.png`, `partial-discovery.png`, `partial-details.png`, `compact-switcher.png`, `minimum-switcher.png`, and `environment-flow.mp4` / `.loop.webp`.

The partial/missing namespace screen is an explicitly offline simulation using a removed fixture identity, not an Azure permission failure. Stale counts and permission failures additionally have public TUI regression coverage. Descriptive status/sidebar text intentionally truncates in narrow cells; operational keys stay visible, and full identities/errors have a wrapped Details screen.

Invalid intermediate captures were replaced: a recording session without consistent restore flags navigated to a blank page; ttyd without `COLORTERM=truecolor` rendered some count text invisibly; early resize captures included the terminal bridge's transient size overlay. Final captures use a consistent restored browser session, the existing PTY true-color environment and waits for the resize overlay to disappear. DOM color checks confirmed visible 128 queued / 3 dead-letter values. The final recording's contact sheet was inspected through the return to Dashboard.

## Limits and delivery

- **No live Azure validation.** Azure CLI is not installed in this orb and no target account/tenant was supplied. CLI/HTTP/token tests cannot establish actual RBAC behavior, network access or successful AMQP peek. Supported live mode is one tenant in public Azure with a modern existing CLI login; sovereign clouds and cross-tenant environments are not implemented.
- **No emulator integration run for this change.** Docker CLI and daemon binary exist, but the daemon is not running: `/var/run/docker.sock` is absent. Existing broker unit tests and the entire real-fixture DLQ PTY control pass; they do not substitute for a seeded-emulator run.
- No moderated 500-entity usability study or 15-second success-rate measurement was performed.
- Implementation was committed and rebased onto main after shipping authorization. The evidence thread records the final push and remote-state confirmation. No releases, Azure writes or credential changes were made.
- A one-off supervised offline terminal preview is available at https://t-03h3zgypthdehx6998kv83klk-p22363.onamp.dev/ . Its service definition is not tracked. It can be persisted separately if desired; it is not a deployment or cloud connection.

## Integration with the shipped inspector

The approved additive rebase preserves the unified inspector, precise Vim/clipboard behavior and no-argument selected-review API. Environment panels remain full-screen, including when opened from compact inspector mode. Compact message headers retain the environment name. Switching cancels and invalidates inspector callbacks as well as scoped discovery/count results. The cloud provider implements the newer exact-sequence, full-content read-only peek contract without receive or settlement.

The integrated code passed root `make check && make test-terminal`, log-viewer `make check && make test-terminal`, and Service Bus `make test && go vet ./... && make terminal-test`. No failing checks were waived. New public TUI regressions cover compact environment panels, late clipboard results after switching and the updated selected-review guard. The rebuilt real terminal was inspected at 80×24 and 60×18, with a fresh 60×18 session replacing a capture containing ttyd resize artifacts. Final integration captures and the short keyboard/pointer recording are named `integrated-*` in the same artifact directory.

# Combined environment/shared-UI validation

The upstream environment baseline was independently built and tested at
`23feda384c55bc3e77c06b4b9e767c6aa09a64b9` in a detached worktree. Its race tests,
vet, build and actual-binary environment PTY passed before adaptation. Config and
explorer contracts remain byte-identical to that baseline after relocation.

## Combined checks

- `make check`: passed independent `GOWORK=off` vet and race tests in all three
  app modules and `lib/terminalui`, formatting checks, and ten Python lab tests.
- `./build`: built LazyFTP, its MCP binary, Service Bus Explorer and Log Viewer.
- `GOWORK=off go -C apps/service-bus-explorer build -o bin/sbx-lab ./cmd/sbx-lab`:
  auxiliary lab CLI built without running a lab operation.
- `make test-terminal`: all offline real-binary suites passed. Service Bus runs
  both its existing reader/review/workspace suite and its environment suite with
  mouse enabled and with `--mouse=false`; FTP and Logs retain their PTY suites.

The upstream environment tests retain assertions for exact namespace/group
snapshots, environment isolation, retained excluded selections, Apply versus Save,
failed saves, stale/failed discovery, late results and deletion without fallback.
Input sequences changed only for deliberate shared interaction semantics:
cancel-first confirmations and separate authorization/activation.

New interaction coverage includes select-only rows versus explicit checkboxes,
whole-control visibility, same-name/different-native-identity targets, ignored
release events, contextual Help restoration, literal field text, blocked Ctrl+P,
late catalog invalidation, hidden targets after resize, and refusal during review
or full-content loading. A public Model regression completes a real preferences
write while Help remains open and verifies the persisted Selected-mode watchlist.
Another checks visible compact quit/refusal at 48×3 and inactive hidden discard
routes at 47×2 and 20×1. The compact notice is clipped to actual terminal bounds.

The environment PTY independently reads saved JSON and restarts the binary. It
covers cross-subscription known namespaces, exact group preview/Add, explicit
Apply-only, filtered bulk inclusion, Selected-empty defaults, Help over a draft,
80×24, 60×18 and undersized quit. Its mouse variant sends real SGR press/release;
the fallback variant verifies reporting is disabled.

## Rendered inspection and limits

The actual combined fixture binary was inspected in ttyd/xterm at 2× browser
scale, including the workspace, All/Selected inclusion and contextual Help, with
an actual moving-pointer recording. The rendering wrapper is for inspection,
not a substitute for the decoded real-PTY assertions. Terminal-emulator fitting
was corrected before retaining captures; an initially cropped capture was
discarded. Screenshots and the recording are attached to the implementation
thread, together with captured command output.

No live Azure connection, credential changes, broker mutation or lab reset/seed
was performed. Azure injected CLI/HTTP tests do not establish actual RBAC/network
access. Native macOS terminals, assistive technology usability and the receiving
system clipboard remain unverified. OSC52 PTYs establish transmission only.

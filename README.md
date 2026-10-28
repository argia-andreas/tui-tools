# Terminal applications

Three independently buildable Go applications, with repository-wide development
commands and a shared interaction library:

| App | Source and instructions | Offline mode | Build output |
| --- | --- | --- | --- |
| LazyFTP | [apps/lazyftp](apps/lazyftp/) | `--demo` | `apps/lazyftp/bin/lazyftp`, `lazyftp-mcp` |
| Service Bus Explorer | [apps/service-bus-explorer](apps/service-bus-explorer/) | `--fixture` | `apps/service-bus-explorer/bin/sbx` |
| Log Viewer | [apps/log-viewer](apps/log-viewer/) | `--fixture` | `apps/log-viewer/bin/logview` |

App-local `cmd/`, `internal/`, scripts, lab definitions and user documentation
live together. Repository setup and [design plans](docs/plans/) remain at root.
Shared buttons, command palette, contextual Help and terminal geometry live in
`lib/terminalui`, consumed by all three apps. Shared surfaces and navigation
support visible controls, select-first rows and explicit activation, with keyboard
fallback through `--mouse=false`. Layouts, editor grammar and domain operations
remain app-owned. See each app's control inventory for coverage and limitations.

## Build

Install the Go version in [.go-version](.go-version), Bash and Make. From the
repository root:

```sh
./build                         # all apps, including LazyFTP's MCP server
./build lazyftp
./build service-bus-explorer
./build log-viewer
```

The command delegates to each app's Makefile; binaries stay in that app's `bin/`.
It also works when invoked by absolute path from another directory. An unknown
app name or extra argument is rejected before building anything.
`make build` and `make build APP=log-viewer` are equivalent entry points.
Individual app Makefiles remain usable, for example `make -C apps/lazyftp demo`
or `make -C apps/log-viewer run-fixture`.

## Orb setup and checks

In a Linux amd64 Amp orb with `curl`, `tar`, Node.js/npm, Python 3 and
[uv](https://docs.astral.sh/uv/):

```sh
./.agents/setup
make check
make test-terminal
```

Setup installs checksum-verified Go 1.27.3, downloads module dependencies, and
creates app-local Python test environments with pinned requirements. It also
installs ttyd 1.7.7 for terminal previews and the pinned FTP/Blob/S3 lab clients
(rclone 1.70.3, pyftpdlib 2.1.0, Azurite 3.35.0 and SeaweedFS 4.49).
Setup is repeatable and installs dependencies only: no cloud authentication,
service startup, lab seeding or deletion. See each app's README for its own lab;
lab operation is never an implicit part of building or checking the suite.

`make check` checks formatting, vets and race-tests every module independently
with `GOWORK=off`, and runs Service Bus lab-script unit tests. `make test-terminal`
builds and exercises all three real executables in offline PTYs. The root
`go.work` is an editor/development convenience, not a substitute for module checks;
root `go test ./...` is not the suite test command. App module identities and user
configuration locations are unchanged.

Generated binaries, environments and private `.lab` state are app-local and
ignored. Source relocation does not move/adopt old lab state or rewrite ownership
markers. Stop and review any active old-path services before recreating a lab;
do not blindly move data directories or reset existing volumes.

## Design and progress

The approved shared-interaction design and incremental implementation state live
in [docs/plans/shared-terminal-ui](docs/plans/shared-terminal-ui/00-status.md).
The apps keep their distinct layouts and domain safety while shared components
are introduced in verified slices.

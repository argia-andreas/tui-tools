# TUI Tools

**Opinionated, minimalist terminal tools for moving files, inspecting messages,
understanding logs, and investigating event streams.**

Each tool focuses on a specific job, with a small set of discoverable controls
and explicit actions where mistakes matter. They share an interaction vocabulary,
not a single layout or an all-purpose application framework.

These are experimental tools under active development. Read each app's
limitations and safety guidance before connecting real services.

## The tools

| Tool | What it does | Documentation |
| --- | --- | --- |
| **Transfer** | Browse and transfer files over FTP/FTPS, S3, and Azure Blob via rclone. | [Transfer](apps/transfer/README.md) |
| **Logs** | View and filter logs from the authenticated local Elasticsearch lab. | [Logs](apps/logs/README.md) |
| **QE** | Service Bus queue/topic browsing, dashboards, and Active/DLQ inspection. Azure access is read-only. | [QE](apps/queue/README.md) |
| **Streams** | Read-only Kafka-compatible event browsing and consumer-group lag inspection. | [Streams](apps/streams/README.md) |

### What “opinionated and minimalist” means here

- **Focused workflows.** Each app owns its domain and layout. Shared code earns
  its place by simplifying concrete interactions across the tools.
- **Discoverable controls.** `Ctrl+P` opens Actions; `F1` opens contextual Help.
  Keyboard navigation and mouse controls coexist. Use `--mouse=false` for
  terminal-native selection and keyboard-only operation.
- **Explicit consequences.** Browsing is separate from mutation. Transfers and
  destructive operations require review; confirmations default to cancellation.
- **Honest feedback.** Sample data, unavailable counts, incomplete discovery,
  unsaved changes, and operation outcomes should be visible—not disguised as success.

Minimalism is about reducing decisions and surprises, not hiding useful context
or making every application look the same.

## Connecting services

The tools are intended for working with storage, messages, logs, and event
streams on real services. Follow each app's documentation for supported
connections, credentials, and limitations. Transfer uses an external **rclone**
executable for storage operations; Neovim is optional. QE's Azure mode uses an
existing Azure CLI login and never logs in for you. Streams uses explicit broker
connection profiles and should be paired with broker-enforced read-only ACLs.
Logs currently supports only its authenticated local Elasticsearch lab, not
arbitrary production sources.

## Build

Install the Go version in [.go-version](.go-version), Bash, and Make.
Build the suite or a single app:

```sh
./build                         # all apps, including transfer's MCP server
./build transfer
./build logs
./build queue
./build streams
```

The command delegates to each app's Makefile; binaries stay in that app's `bin/`.
It also works when invoked by absolute path from another directory. An unknown
app name or extra argument is rejected before building anything.
`make build` and `make build APP=logs` are equivalent entry points.
Individual app Makefiles remain usable, for example `make -C apps/logs build`.

QE currently builds as `apps/queue/bin/queue`; use `./build queue` as shown above.

### MCP servers

Transfer includes the implemented `transfer-mcp` server, built alongside the
terminal app. See its [MCP documentation](apps/transfer/docs/mcp.md) for setup and
permissions. The intended companion server names for Logs, QE, and Streams are
`logs-mcp`, `qe-mcp`, and `streams-mcp`, respectively; these three servers are not
implemented or built yet. Do not configure them as working commands.

## Development and checks

Each app is an independently buildable Go module. Its `cmd/`, `internal/`,
scripts, lab definitions, and documentation live together under `apps/`.
[lib/terminalui](lib/terminalui/) contains the shared Actions, Help, surfaces,
and navigation primitives. Layouts, editor grammar, and domain operations stay
app-owned. The root `go.work` connects the modules for local development.

### Amp orb setup

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

### Testing

Fixtures support testing the programs, including testing with agents, without
connecting external services. After building, run one tool at a time:

```sh
./apps/transfer/bin/transfer --demo
./apps/logs/bin/logs --fixture
./apps/queue/bin/queue --fixture
./apps/streams/bin/streams --fixture
```

These modes use fictional data and need no cloud account, Docker daemon, or
rclone installation. transfer's demo is read-only; queue's fixture mutations
affect only the in-memory sample scenario. Fixture tests do not prove that a
live backend has been configured or that real-service integration works.
App Makefile targets include `make -C apps/transfer demo` and
`make -C apps/logs run-fixture`.

For agent-assisted integration testing against real protocols, use the opt-in
disposable labs documented by each app. Labs are development/test environments,
not required onboarding steps. Building does not start services, authenticate
accounts, seed data, or delete lab state. Local QE modes support explicitly
reviewed message operations; Azure mode remains read-only.

`make check` checks formatting, vets and race-tests every module independently
with `GOWORK=off`, and runs Service Bus and streams lab-script unit tests. `make test-terminal`
builds and exercises all four real executables in offline PTYs. The root
`go.work` is an editor/development convenience, not a substitute for module checks;
root `go test ./...` is not the suite test command.

Generated binaries, environments and private `.lab` state are app-local and
ignored. Source relocation does not move/adopt old lab state or rewrite ownership
markers. Stop and review any active old-path services before recreating a lab;
do not blindly move data directories or reset existing volumes.

## Credits and acknowledgements

These tools build on substantial work by other projects and their contributors.
In particular, **rclone is transfer's storage engine**: transfer supplies the
terminal workflow and confirmations, not its own implementations of FTP, Blob,
or S3 transfers.

### Application foundations

| Project | How it is used |
| --- | --- |
| [rclone](https://rclone.org/) | transfer's real storage listing, file operations, and transfers. |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and [Charm's terminal libraries](https://github.com/charmbracelet/x) | Event-driven terminal applications, styling, ANSI handling, and terminal utilities. |
| [Azure SDK for Go](https://github.com/Azure/azure-sdk-for-go) and [go-amqp](https://github.com/Azure/go-amqp) | Service Bus messaging clients and AMQP support. |
| [franz-go](https://github.com/twmb/franz-go) and kadm | streams' Kafka protocol client and read-only metadata/group observations. |
| [Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk) | transfer's separately built MCP server. |
| [go-osc52](https://github.com/aymanbagabas/go-osc52), [uniseg](https://github.com/rivo/uniseg), and [fuzzy](https://github.com/sahilm/fuzzy) | Terminal clipboard sequences, Unicode segmentation/width handling, and command matching. |
| [Neovim](https://neovim.io/) and [Nerd Fonts](https://www.nerdfonts.com/) | Optional external editing and file-type glyphs. |

### Labs and development tools

- [pyftpdlib](https://github.com/giampaolo/pyftpdlib),
  [Azurite](https://github.com/Azure/Azurite), and
  [SeaweedFS](https://github.com/seaweedfs/seaweedfs) provide the local FTP,
  Azure Blob, and S3-compatible storage labs.
- [Azure Service Bus Emulator](https://github.com/Azure/azure-service-bus-emulator-installer)
  and Azure SQL Edge support the opt-in Service Bus lab. Their separate license
  terms and explicit EULA acceptance still apply.
- [Elasticsearch](https://www.elastic.co/elasticsearch) provides the authenticated
  TLS log lab; its distribution has its own license terms.
- [Redpanda](https://github.com/redpanda-data/redpanda) and
  [Apache Kafka](https://kafka.apache.org/) provide streams' opt-in real broker
  labs; their separate distribution/license terms apply.
- [pyte](https://github.com/selectel/pyte) supports real-terminal regression checks;
  [ttyd](https://github.com/tsl0922/ttyd) provides browser terminal previews;
  [uv](https://docs.astral.sh/uv/) manages the Python test environments.

### Palette inspirations

The optional theme presets adapt palette values from
[Catppuccin](https://github.com/catppuccin/catppuccin),
[Tokyo Night](https://github.com/folke/tokyonight.nvim),
[Gruvbox](https://github.com/morhetz/gruvbox), and
[Dracula](https://github.com/dracula/dracula-theme).
Midnight is an original palette, not an editor-theme port. Source attribution
and notices are retained in the
[transfer](apps/transfer/internal/theme/provenance.go) and
[queue](apps/queue/internal/theme/provenance.go)
theme provenance files.

This is a highlights list, not a complete dependency or license inventory. See
the app and library `go.mod`/`go.sum` files and the lab's
[package manifest](apps/transfer/lab/package.json) and lockfile for pinned
dependencies. Third-party projects retain their own copyrights and licenses;
acknowledgement here does not replace their notices or imply endorsement.

## Project license

This project's original code is licensed under the [MIT License](LICENSE).
You may copy, modify, and distribute it, including commercially, provided you
retain the copyright and permission notice. It is provided without warranty.
Third-party code, palette notices, dependencies, and external tools retain their
own licenses and attribution requirements; the project license does not replace
them.

## Design notes

See the [maintainer documentation index](docs/README.md) for current architecture,
safety contracts, historical verification, and remaining plans/acceptance work.
The apps retain their distinct layouts and domain safety around shared components.

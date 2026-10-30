# TUI Tools

**Opinionated, minimalist terminal tools for moving files, inspecting messages,
and understanding logs.**

Each tool focuses on a specific job, with a small set of discoverable controls
and explicit actions where mistakes matter. They share an interaction vocabulary,
not a single layout or an all-purpose application framework.

These are experimental tools under active development. Start with the offline
modes and read each app's limitations before connecting real services.

## The tools

| Tool | What it does | Documentation |
| --- | --- | --- |
| **transfer** | Two-pane file browsing and confirmed copies through rclone, with local/remote previews, transfer queues, and optional Neovim editing. | [transfer](apps/transfer/README.md) |
| **logs** | Chronological log investigation with structured inspection, filters, literal find, and saved searches. Currently supports fictional fixtures and an authenticated local Elasticsearch lab—not arbitrary production sources. | [logs](apps/logs/README.md) |
| **queue** | Scoped Azure Service Bus queue/topic browsing, Active/DLQ inspection, and saved environments. Azure mode is read-only; local modes support explicitly reviewed message operations. | [queue](apps/queue/README.md) |

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

## Try it without connecting anything

Install the Go version in [.go-version](.go-version), Bash, and Make. Build from
the repository root, then choose a tool:

```sh
./build

# Run one at a time. These modes use fictional data, not external services.
./apps/transfer/bin/transfer --demo
./apps/logs/bin/logs --fixture
./apps/queue/bin/queue --fixture
```

No cloud account, Docker daemon, or rclone installation is needed for these
offline modes. They are a way to explore the UI, not proof that a live backend
has been configured. transfer's demo is read-only; queue's fixture mutations
affect only the in-memory sample scenario.

For real connections and disposable labs, follow the app documentation above.
transfer uses an external **rclone** executable for real storage operations;
Neovim is optional. queue's Azure mode uses an existing Azure CLI login and
never logs in for you. Lab services are opt-in and are not started by a build.

## Build

Build the suite or a single app:

```sh
./build                         # all apps, including transfer's MCP server
./build transfer
./build logs
./build queue
```

The command delegates to each app's Makefile; binaries stay in that app's `bin/`.
It also works when invoked by absolute path from another directory. An unknown
app name or extra argument is rejected before building anything.
`make build` and `make build APP=logs` are equivalent entry points.
Individual app Makefiles remain usable, for example `make -C apps/transfer demo`
or `make -C apps/logs run-fixture`.

### Existing installations

The apps were previously named LazyFTP, Log Viewer (`logview`), and Service Bus
Explorer (`sbx`). Their executable names are now `transfer`, `logs`, and `queue`;
the companion commands are `transfer-mcp` and `queue-lab`. Update shell aliases,
scripts, and MCP launch commands accordingly.

Existing user configuration locations and formats remain unchanged: `lazyftp`
for connection bookmarks, `logview` for saved searches/settings, and
`service-bus-explorer` for preferences under your OS configuration directory.
The internal Go module identities also remain stable. Historical design notes
and screenshots may use the former names.

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

### Validation

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

A license for this repository has not been selected yet. The third-party
dependencies and external tools above remain subject to their own licenses.

## Design notes

The approved shared-interaction design and incremental implementation state live
in [docs/plans/shared-terminal-ui](docs/plans/shared-terminal-ui/00-status.md).
The apps keep their distinct layouts and domain safety while shared components
are introduced in verified slices.

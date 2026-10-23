# lazyftp

A terminal file-manager proof of concept with mouse/keyboard browsing and
confirmed **single-file upload/download for FTP and Azure Blob via rclone**.
The default fixture demo is read-only; real operations require `--live`.

## Setup and run

Requires Linux amd64, `curl`, `tar`, Node.js/npm, Python 3, and
[uv](https://docs.astral.sh/uv/). Managed lab startup uses an Amp orb; no Docker
daemon or cloud account is needed.

```sh
./.agents/setup
make demo
```

Setup installs checksum-verified Go **1.27.3** under
`~/.local/toolchains/go1.27.3`, links `go` and `gofmt` into `~/.local/bin`, and
adds that directory to the login-shell PATH. When passwordless sudo is available,
it also links those commands into `/usr/local/bin`. Setup downloads Go module
dependencies and creates `.venv` with `pyte==0.8.2` for terminal smoke tests.
It also installs checksum-verified rclone **1.70.3**, pyftpdlib **2.1.0**, and
locked Azurite **3.35.0** dependencies. It is safe to run repeatedly.

`make build` writes `bin/lazyftp`; `make run` and `make demo` build and launch
`bin/lazyftp --demo`. You can also launch the binary directly:

```sh
./bin/lazyftp --demo --theme dracula
./bin/lazyftp --demo --config /path/to/config.json
```

Use `--theme` to select a theme and `--config` to select the local configuration
file. Theme choices are also available in the command palette.

Presets: `midnight` (default), `catppuccin-mocha`, `tokyo-night`, `gruvbox-dark`,
`dracula`. Midnight is an original lazyftp palette inspired by the project's
dark-blue mockup, with mint selections and contrasting borders; it is not an
editor-theme port. The four existing editor-inspired presets remain unchanged.
Configuration uses semantic color roles, not executable editor theme files:

```json
{
  "theme": "tokyo-night",
  "colors": { "focus": "#7dcfff", "folder": "#7aa2f7" },
  "icons": false
}
```

`--theme` overrides the configured preset; explicit color overrides still apply.
When neither specifies a preset, Midnight is used, including with custom colors.
`--icons` enables Nerd Font glyphs if your terminal font supports them; the
default uses portable filetype symbols with semantic colors, without requiring
a special font. Unknown presets, roles, and invalid colors
produce errors rather than silently falling back.

## Real local FTP + Blob lab

```sh
make lab-up
make lab-seed
make run-lab
```

The left pane is `.lab/local`; the right starts at FTP. Click the Blob entry
in the sidebar to browse Azurite. Select one file and press `c`, click Copy,
or use Ctrl+P → Copy. Every operation shows captured source/destination paths
and defaults to **Cancel**. Existing destinations offer Skip, Overwrite or
Rename; a renamed destination is checked again. Rename copies under a new
name; it does not move or delete the source.

The transfer drawer reports running/completed/failed outcomes, not invented
percentage progress. Navigation cannot retarget an already started copy.
Directory/batch copies, queues, interactive cancellation and previews are not
implemented yet. Quitting cancels subprocesses, but interrupted transfers can
leave partial destination data; this build does not implement recovery.

Services bind only to loopback (FTP 2121/passive 30000–30009; Blob 10000).
Only public development credentials are generated in private-mode
`.lab/rclone.conf`. No production account is needed. Data has an ownership
marker; seed adds missing fixtures and **preserves existing files**.
`make lab-down` stops only these named services and keeps data.

For another existing rclone profile:

```sh
./bin/lazyftp --live --left /absolute/local/directory \
  --rclone-config /path/to/rclone.conf --right my-ftp:/incoming \
  --blob my-blob:/container/prefix
```

The config path is optional when rclone's usual config/environment is set.
Credentials remain in rclone; raw backend stderr is not exposed in the UI.
FTP is unencrypted unless TLS is configured in that profile. The lab verifies
mechanics, not Azure production authentication or full cloud-service parity.
Destination checks plus rclone immutable mode guard unconfirmed replacement,
but **FTP cannot guarantee atomic no-clobber against concurrent writers**.
Do not use this prototype on valuable production files.

## Keys

| Key | Action |
| --- | --- |
| Ctrl+P | Open the command palette (including theme selection) |
| F1 / ? | Open the grouped, scrollable shortcut guide (F1 also works in the palette) |
| Tab | Move focus between panes |
| Arrow keys / j / k | Navigate lists |
| Enter / Backspace | Open directory / go to parent |
| Space | Toggle file selection |
| c | Confirm copying one file to the opposite pane (live mode only) |
| `/` / Ctrl+R | Filter the focused file pane / refresh |
| `[` / `]` | Toggle the sidebars |
| t | Toggle the transfer drawer |
| q | Quit |

In the palette, type to fuzzy-search, use arrows to choose, Enter to run, and
Esc to return. In the navigation sidebar, Enter opens the selected location in
the corresponding file pane. Local and remote selections remain independent.

## Mouse navigation

- Click a directory name to open it, or a file to focus it for the inspector.
- Click a row's leading marker to toggle selection without opening it.
- Click sidebar locations to switch endpoints. Up, Filter, and Refresh controls
  are available above each file list.
- Use the top Nav, Info, Jobs, ?, and Actions buttons to control the workspace.
- Click palette results to run them; F1 shortcuts opens the keyboard guide.
- Scroll the mouse wheel over lists, the palette, or the shortcut guide.
- Close modals with ×, Esc, or an outside click. Outside clicks never activate
  the files underneath.

Your terminal must support mouse reporting (the browser demo does). At narrow
widths optional sidebars hide automatically and return when there is room.

## Checks

```sh
make test           # go test ./...
make check          # gofmt check, go vet, and go test
make test-terminal  # build and run the PTY smoke test using .venv
make test-integration   # real FTP/Blob byte checks; lab must be running
make test-terminal-live # actual TUI upload/download and conflict safety
```

Default checks need no storage server or cloud account. Integration checks fail
clearly when services or dependencies are missing, rather than silently skip.
They use unique disposable prefixes and verify binary/empty/Unicode/space-name
round trips, conflict preservation, explicit overwrite and renaming.

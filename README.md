# lazyftp

A fast, runnable terminal file-manager proof of concept. **Fixture only: no real
connections or file transfers yet.** The demo is safe to explore without server
credentials.

## Setup and run

Requires Linux amd64, `curl`, `tar`, and [uv](https://docs.astral.sh/uv/).

```sh
./.agents/setup
make demo
```

Setup installs checksum-verified Go **1.27.3** under
`~/.local/toolchains/go1.27.3`, links `go` and `gofmt` into `~/.local/bin`, and
adds that directory to the login-shell PATH. When passwordless sudo is available,
it also links those commands into `/usr/local/bin`. Setup downloads Go module
dependencies and creates `.venv` with `pyte==0.8.2` for terminal smoke tests.
It is safe to run repeatedly.

`make build` writes `bin/lazyftp`; `make run` and `make demo` build and launch
`bin/lazyftp --demo`. You can also launch the binary directly:

```sh
./bin/lazyftp --demo --theme dracula
./bin/lazyftp --demo --config /path/to/config.json
```

Use `--theme` to select a theme and `--config` to select the local configuration
file. Theme choices are also available in the command palette.

Presets: `catppuccin-mocha`, `tokyo-night`, `gruvbox-dark`, `dracula`.
Configuration uses semantic color roles, not executable editor theme files:

```json
{
  "theme": "tokyo-night",
  "colors": { "focus": "#7dcfff", "folder": "#7aa2f7" },
  "icons": false
}
```

`--theme` overrides the configured preset; explicit color overrides still apply.
`--icons` enables Nerd Font glyphs if your terminal font supports them; the
default uses portable text glyphs. Unknown presets, roles, and invalid colors
produce errors rather than silently falling back.

## Keys

| Key | Action |
| --- | --- |
| Ctrl+P | Open the command palette (including theme selection) |
| Tab | Move focus between panes |
| Arrow keys / j / k | Navigate lists |
| Enter / Backspace | Open directory / go to parent |
| Space | Toggle file selection |
| `/` / Ctrl+R | Filter the focused file pane / refresh |
| `[` / `]` | Toggle the sidebars |
| t | Toggle the transfer drawer |
| ? / q | Help / quit |

In the palette, type to fuzzy-search, use arrows to choose, Enter to run, and
Esc to return. In the navigation sidebar, Enter opens the selected location in
the corresponding file pane. Local and remote selections remain independent.

**Keyboard-only for now:** clicking a browser-hosted terminal focuses keyboard
input; file rows and actions do not yet support mouse interaction. At narrow
widths optional sidebars hide automatically and return when there is room.

## Checks

```sh
make test           # go test ./...
make check          # gofmt check, go vet, and go test
make test-terminal  # build and run the PTY smoke test using .venv
```

This is a working prototype, not a production FTP client. The fixture UI and
local preferences are the current scope; remote operations are not implemented.

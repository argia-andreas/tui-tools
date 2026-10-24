# lazyftp

A terminal file-manager proof of concept with mouse/keyboard browsing and
confirmed **file/folder upload/download for FTP, Azure Blob and S3 via rclone**.
Start with a focused connection chooser: open a bookmark or configure a remote
in-app. Esc opens local browsing. Copies always require confirmation; `--demo`
explicitly opens the read-only fixture workspace.

## Setup and run

Requires Linux amd64, `curl`, `tar`, Node.js/npm, Python 3, and
[uv](https://docs.astral.sh/uv/). Managed lab startup uses an Amp orb; no Docker
daemon or cloud account is needed.

```sh
./.agents/setup
make run
```

Setup installs checksum-verified Go **1.27.3** under
`~/.local/toolchains/go1.27.3`, links `go` and `gofmt` into `~/.local/bin`, and
adds that directory to the login-shell PATH. When passwordless sudo is available,
it also links those commands into `/usr/local/bin`. Setup downloads dependencies
for the root and top-level Go modules and creates their Python terminal-test
environments (including `service-bus-explorer/.venv`). It pins `pyte==0.8.2`
and ttyd **1.7.7** for repeatable terminal checks and browser-terminal previews.
It also installs checksum-verified rclone **1.70.3**, pyftpdlib **2.1.0**, and
locked Azurite **3.35.0** dependencies, plus pinned SeaweedFS **4.49** for
S3-compatible local storage. It is safe to run repeatedly.

Setup installs dependencies only: it does not authenticate cloud accounts,
start storage servers, or require a Docker daemon. Start the disposable lab
when needed with `make lab-up lab-seed`; no resume hook is required for these
supervised services. The log viewer is design-only and has no runtime to install yet.

`make build` writes `bin/lazyftp`; `make run` opens the connection chooser.
`make demo` explicitly launches the read-only fixtures. You can also launch the
binary directly:

```sh
./bin/lazyftp --theme dracula
./bin/lazyftp --config /path/to/config.json
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
Nerd Font filetype icons are enabled by default, including when `icons` is omitted
from the configuration. Select a Nerd Font (for example JetBrainsMono Nerd Font
Mono) in your terminal settings. Use `--icons=false`, `"icons": false` in the
configuration (as above), or **Toggle filetype icons** in the command palette to
switch to portable filetype symbols with semantic colors and no special font
requirement. An explicit `--icons` or `--icons=false` overrides the configuration.
Icons cover common code, document, data, image and archive
extensions, including uppercase extensions; filenames share a fixed-width gutter.
Unknown presets, roles, and invalid colors
produce errors rather than silently falling back.

## Edit files in Neovim

Install `nvim` on PATH, focus a file in either pane, and press **e** (or choose
“Edit selected file in Neovim” from Ctrl+P). Your existing Neovim configuration
is used unchanged: no plugins, configuration edits, or injected save hooks.
Local files open directly. Remote files open in a private temporary directory
with their original filename and extension. Lazyftp watches saved contents and
uploads stable snapshots while Neovim remains open, normally starting about
300 ms after a save. Rapid saves may be combined; the latest saved contents
are uploaded in order. Unsaved buffer changes are not uploaded.

Exit Neovim to return to Lazyftp; it finishes outstanding saves and refreshes
the listing. Upload failures stop automatic uploads and are reported on return,
including the preserved working-copy path for recovery. Detected remote changes
also stop uploads rather than overwrite them. Conflict detection uses size and
modification time and is best-effort: FTP cannot atomically guard against another
writer, and changes with identical metadata cannot be detected. Editing is
disabled in the read-only demo and while a transfer queue is active.

## Real local FTP + Blob + S3 lab

```sh
make lab-up
make lab-seed
make run-lab
```

The left pane is `.lab/local`; the right starts at FTP. Press `[` to reveal the
navigation sidebar, then click Blob or S3 to browse Azurite or SeaweedFS.
Select files/folders with Space or their leading markers, then press `c` or use
Ctrl+P → Copy. A single file
opens a confirmation; multiple files or folders first produce a read-only plan
and a batch confirmation. Both default to **Cancel**. A confirmed batch copies
new files serially and pauses on each conflict for Skip, Overwrite or Rename.
A renamed destination is checked and confirmed again. Rename copies under a
new name; this copy-conflict option does not move or delete the source.

## Rename files

Focus a file in either pane and press **R (Shift+r)**, or choose **Rename selected
file** in Actions/Ctrl+P. Edit the filename inline; arrows, Home/End,
Backspace/Delete and Ctrl+U work with Unicode and spaces. Enter opens a review
showing the location and quoted old/new names. **Cancel is the default**; choose
Rename and confirm to apply it. Long confirmation text wraps and scrolls with
↑/↓ or the wheel. Esc cancels before confirmation, and Escape, Cancel, × or an
outside click dismisses the confirmation without changing files.

Rename changes only the highlighted file, not a marked batch, within its current
folder. It works on local files and configured FTP/FTPS, Azure Blob and S3
connections. Existing names are refused, even when contents match. The demo,
directories and active transfer queues cannot be renamed. **Lowercase r still
retries transfers.** After success, matching panes refresh and follow the renamed
file if its new name matches the current filter. A known name collision returns
to inline editing; change the name and review again. Other errors refresh the
panes before you press R to retry with a fresh confirmation.

Once confirmed, a rename runs to its bounded completion; Esc cannot undo it.
Object stores may copy then delete, so failures can leave both names or an
uncertain outcome. Inspect the refreshed listing before retrying. Remote checks
cannot guarantee atomic no-clobber against concurrent external writers. Local
rename uses no-clobber hard-link creation then source removal, requires hard-link
support, and may leave both names if removal fails. No rollback or undo is promised.

## Transfer queue

The drawer distinguishes queued/running/completed/skipped/failed/canceled files,
not invented percentages. Scroll or focus it to review outcomes. Navigation
cannot retarget captured jobs. Ctrl+X or Stop cancels running/unfinished work;
`r` or Retry confirms retrying only failed/canceled files in the current queue.
Completed and skipped files are not repeated. Cancellation is **not rollback**:
partial destination files may remain, and retry checks conflicts afresh without
inheriting overwrite permission. Queues are in-memory, not restart-persistent.
Empty directories are not created (object-store prefixes need actual objects).
Plans are limited to 10,000 files and 64 directory levels; local symlinks and
special files are not transferred. These checks are not a sandbox against
concurrent hostile filesystem mutations.

Press `p`, click Preview, or choose Preview in Actions for an explicit read-only
text preview up to 64 KiB. Binary/invalid UTF-8 files are not displayed; terminal
escapes are stripped, truncation is labeled, and the preview scrolls. Merely
browsing files never loads their content.

Services bind only to loopback (FTP 2121/passive 30000–30009; Blob 10000;
S3 8333 plus internal SeaweedFS listeners). See [lab details](lab/README.md).
Only public development credentials are generated in private-mode
`.lab/rclone.conf`. No production account is needed. Data has an ownership
marker; seed adds missing fixtures and **preserves existing files**.
`make lab-down` stops only these named services and keeps data.

For another existing rclone profile:

```sh
./bin/lazyftp --live --left /absolute/local/directory \
  --rclone-config /path/to/rclone.conf --right my-ftp:/incoming \
  --remote my-blob:/container/prefix --remote my-s3:/bucket/prefix
```

Or launch `./bin/lazyftp` to choose an existing connection or **New connection**
to configure FTP/FTPS, Azure Blob or S3 in-app. Selecting an existing connection
opens its root/local-folder settings with separate Connect and Save bookmark actions.
The wizard tests a read-only listing before connecting; new credentials are
session-only unless you explicitly choose to remember them. `--live` remains a
compatible alias for real mode. `--connections` selects a separate non-secret
bookmark JSON file. See [connections and authentication](docs/connections.md).

The config path is optional when rclone's usual config/environment is set.
Credentials remain in rclone; raw backend stderr is not exposed in the UI.
FTP is unencrypted unless TLS is configured in that profile. The lab verifies
mechanics, not real Azure/AWS authentication or full cloud-service parity.
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
| Enter / Backspace | Open directory (including `..`) / go to parent |
| Space | Toggle file selection |
| c | Confirm copying selected files/folders to the opposite pane (live mode only) |
| D (Shift+d) | Confirm permanent deletion of the highlighted file (live mode only; Cancel is the default; folders are excluded) |
| R (Shift+r) | Rename the highlighted file inline, then review and confirm |
| Ctrl+X / r | Cancel unfinished work / confirm retry of failed and canceled files |
| p | Explicit bounded text preview |
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
- Use the top Actions button to find workspace controls; sidebars and the
  transfer drawer start collapsed. The explicit demo retains its exploration toolbar.
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
make test-integration   # real FTP/Blob/S3 bytes + UI folder round trips; lab required
make test-terminal-live # actual TUI upload/download and conflict safety
```

Default checks need no storage server or cloud account. Integration checks fail
clearly when services or dependencies are missing, rather than silently skip.
They use unique disposable prefixes and verify binary/empty/Unicode/space-name
round trips, conflict preservation, explicit overwrite and renaming.

See [alpha packaging and limitations](docs/alpha.md) for local Linux/macOS
cross-build archives. Cross-building is not native platform verification.

## Agent MCP access

`make build` also builds `bin/lazyftp-mcp`, a separate stdio MCP server with
`list_files`, `upload_file`, and `download_file`. **Everything is denied by
default.** Operator-owned JSON grants each operation for exact remote/path
roots and, for transfers, dedicated local staging roots. Interactive app
credentials/bookmarks do not grant MCP access. Overwrite requests are refused;
there are no delete, move, execute or credential tools. Read the
[permission policy and security limits](docs/mcp.md) before enabling access.

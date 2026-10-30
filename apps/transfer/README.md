# Transfer

A terminal file-manager proof of concept with mouse/keyboard browsing and
confirmed **file/folder upload/download for FTP, Azure Blob and S3 via rclone**.
Start with a focused connection chooser: open a bookmark or configure a remote
in-app. Esc opens local browsing. Copies always require confirmation; `--demo`
explicitly opens the read-only fixture workspace.

## Setup and run

See the [suite README](../../README.md) for common prerequisites and setup
(`./.agents/setup` runs from the repository root). Managed lab startup uses an
Amp orb; no Docker daemon or cloud account is needed.

From the repository root, `./build` builds all apps and `./build transfer` builds
only Transfer. Run the app-local recipes and commands below from `apps/transfer`:

```sh
cd apps/transfer
make run
```

Setup installs dependencies only: it does not authenticate cloud accounts,
start storage servers, or require a Docker daemon. Start the disposable lab
when needed with `make lab-up lab-seed`; no resume hook is required for these
supervised services. The independent [Logs app](../logs/README.md) has
its own fixture mode and local Elasticsearch lab.

`make build` writes `bin/transfer`; `make run` opens the connection chooser.
`make demo` explicitly launches the read-only fixtures. You can also launch the
binary directly:

```sh
./bin/transfer --theme dracula
./bin/transfer --config /path/to/config.json
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
with their original filename and extension. Transfer watches saved contents and
uploads stable snapshots while Neovim remains open, normally starting about
300 ms after a save. Rapid saves may be combined; the latest saved contents
are uploaded in order. Unsaved buffer changes are not uploaded.

Exit Neovim to return to Transfer; it finishes outstanding saves and refreshes
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
./bin/transfer --live --left /absolute/local/directory \
  --rclone-config /path/to/rclone.conf --right my-ftp:/incoming \
  --remote my-blob:/container/prefix --remote my-s3:/bucket/prefix
```

Or launch `./bin/transfer` to choose an existing connection or **New connection**
to configure FTP/FTPS, Azure Blob or S3 in-app. A connection-row click selects only;
**Open** or Enter opens its root/local-folder settings with separate Connect and
Save bookmark actions. Close returns to the retained file workspace.
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
| Tab / Shift+Tab | Traverse visible panes and controls; Enter activates the focused control |
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

FTP confirmations, folder/destination-name forms, preview, queue review, chooser
and wizard use shared surface chrome and current click geometry. Tab/Shift+Tab
traverse visible fields and controls; a `›` marker identifies focused buttons
without relying on color. Confirmations start on Cancel, but forms retain their
field-first Enter behavior and wizard Close follows Back/invalidation semantics.
Preview starts on its scrollable body; Tab reaches Close, while Enter in the body
does nothing. Hidden controls cannot activate after a resize. Workspace file-row
clicks highlight only. Enter or **Open/Inspect** explicitly opens the highlighted
directory or shows the file inspector. The leading marker independently toggles
marks, used for copying; it never opens a directory. Delete still targets only
the highlighted file, not marked neighbors. Compact panes omit whole controls
that do not fit; Actions → Open highlighted entry provides the pointer fallback.

In the palette, type to fuzzy-search (up to 128 Unicode characters), use arrows
or Tab/Shift+Tab to choose without wrapping, and Enter to run an available action.
Backspace removes a character and Ctrl+U clears the query. Esc, Ctrl+P, or an
outside left click returns without activating anything underneath; F1 opens the
shared contextual Help view, as does the clickable Help control. Closing Help
returns to the same palette query, filter draft or operation review; it does not
cancel or submit the parent. Help is view-only: arrows, page keys and wheel scroll;
F1, Esc or its close button returns. Outside clicks are consumed without closing
Help or activating the parent. `?` remains text inside a filter or input field.
Ctrl+N/K are not palette navigation aliases. The
nonmutating **Toggle file inspector** action uses the same operation as `]`.
Either pane can show local files or any saved/new remote. Click
**Detach** in a pane header (or use Ctrl+P → Detach) to disconnect only that pane.
Its inline chooser offers **Local files**, **Saved connection**, and **New
connection**; use arrows/j/k and Enter, or click. Esc cancels setup without
reattaching anything. Local files opens an existing-folder prompt for that pane.
Source actions and sidebar locations open in the **last focused file pane**,
not a fixed local-left/remote-right slot. The other pane's folder, filter and
selections stay unchanged. Two remote panes can browse, preview and edit, but
remote-to-remote copying remains unsupported. Finish or explicitly cancel active
transfers before detaching or changing sources. Detach never deletes files,
bookmarks or credentials; session configurations remain alive until app exit.

## Mouse navigation

- Click a directory or file row to highlight it only, then use Enter or
  **Open/Inspect** (also available in Actions). No double click is required.
- Click a row's leading marker to toggle selection without opening it.
- Click sidebar locations to switch endpoints. Up, Filter, and Refresh controls
  are available above each file list.
- Use the top Actions button to find workspace controls; sidebars and the
  transfer drawer start collapsed. The explicit demo retains its exploration toolbar.
- Click available palette results to run them; F1 or Help opens contextual help.
- Actions is unavailable while a review or filter owns input; Help remains available.
- Scroll the mouse wheel over lists, the palette, or the shortcut guide.
- Close modals with their close button (× or [x]), Esc, or an outside click.
  Outside clicks never activate the files underneath. Help only closes with its
  own close button, F1 or Esc, preserving the parent surface.

Your terminal must support mouse reporting (the browser demo does). At narrow
widths optional sidebars hide automatically and return when there is room.
Control focus is highlighted and underlined; Ctrl+H/L directly focuses either
file pane without traversing controls. Wheel scrolling changes only the pointed
pane (three rows), not the remembered endpoint. See the app-owned
[click inventory](docs/ui-click-inventory.md) for capture and safety conditions.

Mouse reporting is enabled by default. Launch `./bin/transfer --mouse=false`
(also supported with `--demo`) to leave pointer gestures to the terminal instead.
Application clicks and wheel navigation are then unavailable; use arrows/j/k to
highlight, Space to mark files, Enter to open/inspect, Tab/Shift+Tab to reach
controls, Ctrl+P for Actions, F1 for Help, Esc to return/cancel, and q to quit.
This is a launch-time boolean flag, not a saved preference. It does not change
connection, confirmation, or file-operation behavior.

Disabling reporting permits terminal-native text selection where the terminal
supports it; it does not implement selection, clipboard copying, or scrollback.
Alternate-screen selection and modifier-key bypasses depend on your terminal.
The offline PTY checks verify emitted mouse modes and keyboard workflows, not
native drag selection or clipboard behavior in a graphical terminal.

## Checks

```sh
make test           # go test ./...
make check          # gofmt check, go vet, and go test
make test-terminal  # build and run the PTY smoke test using .venv
make test-terminal-panes # independent pane sources via actual terminal; lab required
make test-integration   # real FTP/Blob/S3 bytes + UI folder round trips; lab required
make test-terminal-live # actual TUI upload/download and conflict safety
```

Default checks need no storage server or cloud account. Integration checks fail
clearly when services or dependencies are missing, rather than silently skip.
They use unique disposable prefixes and verify binary/empty/Unicode/space-name
round trips, conflict preservation, explicit overwrite and renaming.
The terminal smoke check retains all eight demo/icon configuration variants and
also exercises default mouse and `--mouse=false` keyboard-only flows with private
temporary local files and connection settings. It checks default cell-motion/SGR
enable codes and no mouse enable codes in the disabled run, explicit chooser
Open, palette/Help return, canceled deletion/rename, preview, and clean exit
without connecting to a remote or changing fixture bytes.

See [alpha packaging and limitations](docs/alpha.md) for local Linux/macOS
cross-build archives. Cross-building is not native platform verification.

## Agent MCP access

`make build` also builds `bin/transfer-mcp`, a separate stdio MCP server with
`list_files`, `upload_file`, and `download_file`. **Everything is denied by
default.** Operator-owned JSON grants each operation for exact remote/path
roots and, for transfers, dedicated local staging roots. Interactive app
credentials/bookmarks do not grant MCP access. Overwrite requests are refused;
there are no delete, move, execute or credential tools. Read the
[permission policy and security limits](docs/mcp.md) before enabling access.

## Intentional legacy identities

The app and executables are named Transfer (`transfer` and `transfer-mcp`), but
the Go module and imports remain `lazyftp`. Existing bookmarks intentionally
remain in the OS user config directory at `lazyftp/connections.json`; no config
migration occurs. Rclone's usual config path is unchanged, as are private
`lazyftp-session-*.conf` files, `.lazyftp.lock` sidecars and
`.lazyftp-config-*` atomic-save files. Editor and test temporary prefixes also
retain their historical names. The Midnight theme's original lazyftp provenance
and all upstream theme attribution are retained.

The disposable lab deliberately keeps the `lazyftp-lab-{ftp,blob,s3}` service
names, `lazyftp-disposable-lab` npm package identity, development account and
credential constants, and `.lazyftp-lab-owner` marker format. The marker embeds
the absolute app root: state belonging to the former `apps/lazyftp` root must
not be adopted by rewriting its marker. Ignored `.lab`, `.venv`, `bin` and
`lab/node_modules` directories are not relocated with tracked source. Provision
fresh dependencies/state for `apps/transfer`; stop or resolve old lab services
explicitly before starting a new lab if their stable names or ports are occupied.

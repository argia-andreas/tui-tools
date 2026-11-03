# Transfer workspace architecture

This guide maintains the implemented opening chooser, independent pane sources
and file rename contracts, consolidated from the completed 2026-10-24 LazyFTP
plans. Transfer is the current app name; historical artifact names and some
internal identifiers still use LazyFTP. This is not an execution plan or new
shipping authorization.

For credential setup, CLI precedence and bookmark formats, use
[connections](connections.md). For packaging, preview API boundaries and alpha
limitations, use [alpha](alpha.md). [MCP](mcp.md) is a separate, default-deny
stdio server: none of these terminal features adds operations or broadens grants.

## Ownership and implementation map

Paths below are relative to `apps/transfer`.

| Owner | Responsibility |
| --- | --- |
| `cmd/transfer/workspace.go` | Bubble Tea host, discovery, startup versus pane-targeted picker/wizard routing, backend handoff |
| `cmd/transfer/picker.go` | Chooser rendering, input, retained details, viewport and shared mouse row geometry |
| `cmd/transfer/wizard.go`, `internal/connections/setup.go` | Tested candidate configuration, explicit persistence and session-config lifetime |
| `internal/ui/model.go`, `destination.go`, `view.go`, `mouse.go` | Per-pane attachments, focus, navigation, detached choices, guards and hit regions |
| `internal/ui/transfers.go`, `preview.go`, `editor.go` | Captured transfer locations and stale listing/preview/editor completion protection |
| `internal/ui/rename.go`, `internal/storage/rename.go` | Captured rename workflow and optional storage rename implementation |
| `scripts/terminal-panes.py`, `terminal-rename.py` and Go tests | Observable terminal, UI, storage and host regression coverage |

The host owns discovery and overlays; the UI owns pane focus, navigation and
busy guards. There is no new HTTP API, persistence format, provider protocol or
dependency from these features. Read-only fixture browsers do not implement the
optional `storage.Renamer` interface:

```go
type Renamer interface {
    Rename(context.Context, Location, Location) error
}
```

## Opening chooser

The centered, square-corner chooser uses semantic theme colors and a fixed-size
decorative logo, not a connection-health or authentication indicator. Saved
bookmarks retain their order, followed by sorted discovered remote names. The
heading “Saved & configured” does not imply availability, successful auth or
recent use. Names and remote/root metadata share a row when space permits;
compact rendering truncates in terminal cells and drops metadata before actions.

Connections, **New connection**, and **Browse local files** form one clamped
sequence. Up/Down, j/k, Tab/Shift+Tab and wheel movement traverse the same order;
there is no wraparound or separate action section. These aliases apply to the
chooser, not editable fields. Exactly one logical choice is highlighted.
Headings, padding and separators are not focus stops. Movement never connects,
saves, tests or changes a local folder. Enter opens connection details, guided
setup or local browsing through the existing routes; it does not immediately
connect a saved entry. Esc returns to files; Ctrl+C quits.

Keep one explicit saved-entry scroll offset: actions and footer remain visible,
and returning from an action reaches the last saved entry without losing the
viewport. Details, help and canceled setup preserve the launching selection.
Startup begins loading, not empty. Late discovery retains an action the user
has navigated to and does not close help; untouched startup may select the first
entry. Empty and failed discovery remain distinct and actionable, with both
actions available. Main-list errors are generic; detailed diagnostics belong in
help, not credential-bearing provider stderr. No retry control or new sorting
policy was introduced here.

The picker supports 60×21; the existing wizard needs 22 rows and shows a resize
notice at 21. The workspace's 60×12 checks are not wizard-size acceptance.

## Independent pane sources

Left and right are positions, not source types. Each pane owns its detached
state, source-choice cursor, last local folder, listing generation, location,
filter and selections. `Location.Remote == ""` means local, never detached.
Headers identify LEFT/RIGHT and the actual LOCAL/REMOTE/No source state.

Detach clears only its attachment, listing, filter and selection, keeps focus
there, invalidates listing/preview results and exposes **Local files**, **Saved
connection**, and **New connection** inline. Both detached panes remain
focusable; Tab changes pane, Up/Down or j/k selects a source, and Enter or a
click opens it. Esc dismisses a prompt/overlay without choosing a source; Esc
on the inline chooser does nothing. The palette keeps Detach reachable when a
narrow header cannot show it. The read-only demo cannot detach.

Local Open validates an existing directory, defaulting to that pane's last
local folder or the starting folder. Saved/new flows retain an explicit pane
index through discovery, details, tests and completion. `ConnectionOpened`
returns one target location, with optional startup-only companion `Local` and
backend/navigation-root handoff. Subsequent Open replaces and focuses only its
target pane; cancel preserves an existing attachment. Sidebar and palette source
actions target the last focused file pane regardless of source type.

Connection-first startup remains local-left/remote-right. Startup bookmarks may
restore their companion local folder, with explicit `--left` taking precedence;
startup Esc leaves local-left and detached-right. In-workspace remote choices
need no companion local folder. Existing bookmark companion metadata is retained
on save but never moves the opposite pane; a new bookmark may have empty `local`,
which startup resolves to its starting folder. Nonempty stored local paths retain
absolute-path validation. Existing CLI meanings are unchanged; no symmetric pane
flags or workspace-layout persistence were added.

### Session and asynchronous ownership

Use one shared browser/transfer backend with append-only session snapshots, not
per-pane adapters or duplicate credential contexts. `connections.Prepare` copies
the active config and adds exactly one new remote; names already present are
rejected case-insensitively. The active snapshot is distinct from the persistent
save target. Saving a later remote must not persist earlier session-only remotes.
`connectionSessions` owns successful candidate configs until application exit,
including those needed by the other pane and in-flight work.

Captured jobs/retries retain their original locations. Their remote names keep
the same definitions in later snapshots, so retry must never be retargeted to
current pane contents. Different credentials under the same remote name are
unsupported, rather than ambiguously routed. External user-config edits are
outside this snapshot guarantee. Detach is neither credential erasure nor
server-side logout and never deletes files or bookmarks.

Detach/rebind increments listing generations and cancels/invalidates previews;
late results cannot repopulate a detached pane or inspector. Editor completion
refreshes only its originating unchanged attachment. Listing failure leaves an
attached error state with Retry and Detach, not automatic detachment. Canceled
or stale connection tests cannot attach a pane.

### Copy and source-change safety

Copy requires two attached panes with successful listings. Local/local and both
directions of local/remote retain existing confirmation/conflict behavior.
Two remote panes may browse, preview and edit independently, but all
remote-to-remote copies, including within one remote, are explicitly refused;
there is no hidden staging path. Detached panes cannot preview, edit, navigate,
refresh, filter, select or copy files.

Detach, replacement and sidebar source switching are blocked while transfer
planning/checking/execution is active: finish or explicitly cancel first.
Pending confirmations own input. Hiding a pane must never implicitly cancel a
write or imply rollback. Queue cancellation can leave partial files; completed
history and captured retry endpoints survive source changes.

## File rename

Uppercase **R** (also Actions/palette/shortcut guide) renames only the highlighted
file in the focused pane, never the marked batch. Lowercase **r** remains queue
Retry; copy-conflict destination-name editing is a separate operation. Rename
is file-only, same-directory and same-connection: no folder/bulk rename, move,
overwrite, merge or undo. Detached/read-only panes, unsuccessful/pending listings,
active transfers and in-flight deletion explain why it is unavailable. Running
rename likewise isolates deletion input.

The UI captures pane and source, then opens a rune-aware inline filename editor
with spaces, Unicode and cursor/delete controls. Ordinary workspace shortcut
letters are text while editing. Invalid/empty/path-like names remain editable;
an unchanged name exits without writing. Enter only reviews: confirmation quotes
old/new names and location, wraps and scrolls rather than hiding long names, and
defaults to **Cancel**. Only explicit **Rename** writes. Escape, Cancel, close
and outside click before starting never write.

After confirmation, a bounded asynchronous operation runs to completion with
navigation and quit input isolated. Do not promise running cancellation or
rollback: remote moves may copy then delete. On completion, refresh every pane
showing that directory and retain the renamed file's focus/selection when
visible. If a filter hides it, report the new name. A known unchanged-source
collision returns to inline editing; other failures refresh before a fresh
R/review attempt. Every retry requires confirmation, and uncertain/partial
outcomes must be described honestly.

Local storage uses hard-link creation followed by source removal instead of
replacement-prone `os.Rename`. This needs hard-link support; removal failure may
leave both names. Existing destinations, including dangling local symlinks and
identical-content files, are refused. Remote `rclone moveto --ignore-existing`
(not `--immutable`) preserves sources on skipped identical destinations. It
checks both names afterward to distinguish a skipped/incomplete move from
success. Remote operations cannot universally guarantee atomic no-clobber
against external concurrent writers or rollback; the post-check is not an
independent full-content integrity proof. Case-only renames on case-insensitive
storage may be refused as collisions, prioritizing preservation.

## Security and support boundaries

The [connection guide](connections.md) owns the full credential contract:
name-only discovery, explicit read-only root testing, session-only private 0600
configs by default, explicit credential persistence, separate non-secret
bookmarks, and no implicit MCP grants. Detaching does not erase a config;
normal exit cleans session files, but a crash may leave sensitive temporary
files. Configs are not a keychain or encrypted-at-rest guarantee; FTP obscuring
is reversible. Encrypted/symlink config paths are not modified by the wizard.
Cooperating saves use a lock and atomic replacement, not protection against an
external writer ignoring that lock. Successful listing proves neither write
permission nor object-store prefix existence. Never put secrets in bookmark
paths or CLI locations, and never weaken TLS verification for convenience.

The established guided targets are FTP/FTPS, Azure Blob and S3 via rclone;
SFTP is not an established verified target of these features. This remains an
experimental alpha, not production storage assurance. No package publication,
deployment, native macOS runtime, production cloud identity/permissions or
TLS-specific FTP validation follows from emulator mechanics or cross-builds.

## Historical verification (2026-10-24)

The completed plans recorded the following executed checks. These are retained
historical results, **not checks rerun during documentation consolidation**.
Commands were run from the app under its former LazyFTP name. Evidence paths
below are historical feature-orb `.amp/in/artifacts/` identifiers, not guaranteed
files in this checkout; no fresh inspection or availability claim is made here.

### Independent panes

- Passed `make check`, `go test -race ./...`, `make test-terminal`,
  `make test-terminal-panes`, `make test-terminal-live`, `make test-integration`
  and `git diff --check`.
- Pane PTY acceptance passed all eight left/right × initial local/remote ×
  replacement local/remote transitions, preserving opposite rendered rows,
  selection and applied filter. Covered both detached, action guards,
  target/cancel, invalid folder, keyboard choices, 60×12 and explicit remote-copy
  refusal. The unchanged baseline failed at missing `[ Detach ]` (exit 1).
- Reversed-pane FTP round trip independently matched 4,370 bytes and cleaned
  only owned files. FTP/Blob/S3 TUI round trips matched a 33,044-byte fixture per
  provider, including default Cancel, overwrite handling and clean exit.
- Disposable real-PTY setup opened session-only FTP in LEFT then RIGHT, refreshed
  the earlier connection after the second snapshot and left persistent config
  and bookmarks unchanged. New Blob/S3 connection creation was covered by shared
  routing/unit tests, not newly provisioned cloud accounts.
- Inspected actual-terminal captures: `panes-live-default.png`,
  `panes-live-{left,right,both}-detached.png`, `panes-live-reversed.png`,
  `panes-live-two-remotes.png`, `panes-live-local-folder.png`,
  `panes-live-folder-error.png`, `panes-live-saved-{picker,details}.png`,
  `panes-live-new-wizard.png`, `panes-live-narrow.png`; moving-pointer recording
  `panes-interaction.mp4` and `.loop.webp`, reviewed via contact sheet. A stretched
  browser WebGL renderer was disposed before final DPR-2 captures; no app
  workaround was introduced. The 60×12 capture was checked after resize overlay
  disappearance.

### Opening chooser

- Passed `make check`, `go test -race ./...`, `make test-terminal` and
  `git diff --check`. Unchanged baseline passed 0/12 real-PTY navigation cases;
  final binary passed 12/12: 0/1/3/12 bookmarks at 80×24, 60×21 and 120×40.
- Covered forward/reverse/clamped traversal, details/help returns, j/k, SGR
  wheel input, wizard cancel and local activation. At 21 rows the wizard's
  resize notice was checked, then resized to 22 for setup/cancel.
- Actual Go binary rendered via ttyd was inspected, not just HTML. Historical
  `terminal-opening-*` artifacts include before/after logs, eight screenshots
  (default, both actions, empty, long, compact, loading, error), navigation
  recording and 5-fps contact sheet. Disposable fixtures stayed unchanged; no
  real remote operations were performed in this chooser verification.

### Rename

- Passed `make check`, `go test -race ./...`, `make test-terminal`,
  `make test-terminal-live` and `make test-integration` (storage/UI/MCP).
  Live PTY covered local files in both panes and FTP/Blob/S3 rename.
- Checked exact binary/empty/Unicode bytes, old-name removal, distinct and
  identical collisions, cancellation, error recovery, R versus r, filters,
  cursor editing, long-name review, pointer confirmation and running-input
  isolation. Independent reads confirmed original bytes and collision targets.
  Public UI tests covered rename/deletion isolation in both orderings.
- Inspected `rename-inline.png`, `rename-confirm.png`, `rename-error.png`,
  `rename-success.png`, `rename-confirm-narrow.png` (60×12), and
  `rename-interaction.mp4` via a 5-fps contact sheet, including pointer movement,
  cancel, collision and success.
- Verification used disposable owned directories and isolated lab prefixes.
  FTP, Azurite and SeaweedFS established local mechanics. FTPS shares the FTP
  adapter, but TLS-specific behavior, production cloud identity/permissions and
  native macOS filesystems were not independently verified.

## Preserved design studies

The completed plan directories now retain mockups only as **historical design
studies, not runtime evidence**. Their old execution, gate and handoff documents
are retired; this guide and the linked operational guides own maintained behavior.

- [Independent pane studies](../../../docs/plans/independent-pane-sources/README.md)
- [Opening-screen study](../../../docs/plans/lazyftp-opening-screen/README.md)
- [Rename study](../../../docs/plans/lazyftp-rename/README.md)

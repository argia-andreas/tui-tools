# queue — Service Bus Explorer

An independent keyboard-first terminal workspace in the lazy suite.
**Three explicit modes:** deterministic offline fixtures, the independent local
Service Bus emulator, or read-only Azure using an existing Azure CLI login.
Named environments constrain discovery to saved, explicit namespaces; dashboard
inclusion remains a separate per-environment preference. Local modes retain
explicitly reviewed single-message DLQ actions; cloud message mutations are disabled.
Layout, theme, environments and dashboard inclusion can be saved explicitly;
credentials, message bodies and MCP grants cannot. No MCP access exists.
Fixture counts are sample data. Emulator counts are **unavailable**, never zero
or totals inferred from peek pages.

## Run

Install Go 1.27.3, then from the repository root:

```sh
make -C apps/queue run-fixture
```

See the [suite README](../../README.md) for common setup. From the repository
root, `./build` builds all apps; `./build queue` builds only this app.

Or inside this directory: `go run ./cmd/queue --fixture`. Select exactly one of
`--fixture`, `--emulator`, or `--azure --tenant <tenant-id>`; the app never silently
falls back to sample data or logs in for you.

### Names retained for compatibility

The app directory is `apps/queue`, the executable is `bin/queue`, the UI brand
is **QUEUE**, and the local lab helper is `go run ./cmd/queue-lab`.
The Go module/import path remains `servicebusexplorer`; the lab helper finds its
root by that module declaration, not by the app directory's name.
Preferences intentionally remain at
`os.UserConfigDir()/service-bus-explorer/preferences.json` so saved environments
and appearance load without migration. The fixed Docker Compose project and orb
service remain `sbx-explorer-lab`, and generated credentials remain under
`~/.local/share/sbx-explorer-lab/`. Existing `SBX_*` variables, credential format,
owner markers and message ID prefixes are unchanged. These compatibility names
are not alternate executable names. Historical plan links retain their old paths.
The rename moves tracked source only: ignored binaries, virtual environments and
private state in the old directory are not moved or deleted.

## Saved environments and scoped discovery

Use **Ctrl+P → New environment** or **Settings → Environments**. Name the
environment, browse namespace metadata under an Azure subscription, choose
individual namespaces or preview a resource-group batch, then review and Apply.
Repeat across groups/subscriptions in the same tenant. A group selection saves
only the listed namespace snapshot: **future namespaces never join automatically**.
Add known namespace accepts a full public-Azure namespace address or ARM ID
without requiring broad subscription listing. Unknown management metadata stays
unknown; discovery still needs operation-specific permissions and connectivity.

**Apply changes this session; Save preferences makes all applied changes durable.**
Ctrl+P → Switch environment restores that environment's inclusion choices. Edit,
rename, duplicate and name-confirmed deletion are available in Settings; deletion
only removes local preferences. Switching is blocked during message review and
until outcomes are acknowledged. Unsaved quit offers Save / Discard / Cancel.

**Settings → Dashboard inclusion** controls the same grid/table entity list:
new environments start Selected/empty; `/` searches, `a` selects matching results,
Space toggles, `v` shows full identity, `m` changes Selected/All with an All warning,
and `s` saves all pending preferences. All includes future entities only inside
saved namespaces. Unwatched entities remain browsable in scoped Queues/Topics/tree.
Appearance remains global. Existing local watchlists migrate once to a local
environment; deleted defaults are not recreated. Missing selections are retained
and distinguished from failed/unverified discovery. Stale counts use `~`, never
authoritative zeroes.

The offline fixture includes a sample multi-subscription namespace catalog to try
Production/Staging without Azure. Live mode requires modern Azure CLI token JSON
(`expires_on`), one tenant and the **public Azure cloud**. It pins the principal's
immutable object ID and tenant, refuses changed identities and does not acquire
credentials or permissions on your behalf. Sovereign clouds and cross-tenant
environments are not supported. Live Azure and the emulator were not exercised
for this change; SDK/CLI/HTTP fault cases, race tests and the real fixture PTYs were.
The orb has no running Docker daemon for the emulator integration suite.

The workspace includes a 12-card dashboard at 160×48, denser table over the
same watched queues/subscriptions (14 by default), a collapsible namespace/entity tree, bounded
Active/DLQ peek pages, and a read-only metadata/body inspector. Smaller terminals
show fewer cards; move through the list to reach the rest. Under 110 columns the
sidebar hides automatically. The full inspector appears when enough width exists.

| Key | Action |
| --- | --- |
| `j/k`, arrows | Move selected entity or message |
| `Enter` | Open entity's Active preview; in message list, focus its existing inspector |
| `]` | Open selected entity's DLQ / switch to DLQ |
| `[` | Switch to Active in message view |
| `g` / `t` | Grid / table dashboard |
| `Ctrl+P` | Toggle shared fuzzy Actions; type to filter, arrows/Tab/Shift+Tab choose, Enter runs, Esc closes |
| `F1` | View-only grouped Help; F1, Esc or its close button returns without changing the parent |
| `Tab` / `Shift+Tab` | Traverse visible panes, destinations and controls in frame order (data rows are not Tab stops) |
| `Ctrl-H` / `Ctrl-L` | Focus left / right across visible sidebar, content and inspector; clamp at either edge (Ctrl-H still deletes in text inputs) |
| `h` / `l` | Collapse / expand the selected sidebar group |
| `b` | Collapse/expand sidebar outside inspector; previous word inside text |
| `i` | Toggle inspector in message view |
| `n` / `p` / `r` | Next / previous / refresh peek page |
| `r` on Dashboard | Refresh watched counts |
| `h/j/k/l`, arrows in inspector | Move read-only text cursor without changing message |
| `w/b/e`, `0/$`, `gg/G` | Words, logical line start/end, document start/end |
| Page Up/Down, `Ctrl+U/D` | Page / half-page movement in inspector |
| `yy`, `v`, `V`, `y`, `Y` | Copy logical line; character/line selection; copy selection; full body |
| `?`, `Ctrl+R` in inspector | Copy/navigation help; configure DLQ resend (never sends immediately) |
| `s` in Settings | Save preferences explicitly |
| `Esc` | Clear selection/operator, then return to list; list Esc leaves browser |
| `q`, `Ctrl+C` | Quit |

Settings and Ctrl+P provide Midnight, Gruvbox and Light themes, layout controls,
environments, dashboard inclusion and Save preferences. Printable keys—including
`q`, `j`, and `k`—belong to a focused search/name input. Compact environment panels
support 80×24 and 60×18; smaller sizes ask for resize without hiding quit choices.

The shared top toolbar exposes clickable Help and Actions, with current refusal
reasons during review/full-content loading. Palette rows and its Help/close
controls are clickable too; left press activates once, release does not repeat.
Docked reader, full-content loading and review surfaces also use shared controls.
Click the reader body to focus it; Back explicitly returns to the list, while
keyboard Esc first clears a selection or pending reader command. Resend remains
blocked during selection. Full-content loading starts on Cancel, and cancellation
rejects late results without changing the clipboard.
Review clicks recheck phase, expiry, authorization and currently visible controls;
destination rows select only, then Choose or Enter confirms the destination.
Sending/deleting/releasing cannot be cancelled as if the broker operation rolled
back. Narrow confirmation surfaces keep Cancel available but hide submission.
The compact bordered message toolbar exposes clickable Active, DLQ, Refresh and
Actions controls. Dashboard grid cards/table rows and message rows **select only**;
use Open Active / Peek DLQ, Inspect / Open, or Enter to activate the selected
entity/message. Workspace destinations open directly. Resource-tree disclosure
glyphs expand/collapse independently from branch-label selection. Settings rows
and watch checkboxes have paired pointer/keyboard routes; Save is explicit.
Wheel input moves only the pointed pane (three rows), without moving focus to a
neighbor. Hidden controls are removed from focus traversal after resize or update.
Reader-local copy/visual-selection/help hints are clickable, while cursor/word
grammar and exact grapheme-aware copy semantics remain local to the reader.
See the complete [click inventory and fixture recipes](docs/ui-click-inventory.md).
Review uses horizontal
Cancel / release, Resend copy and Delete original buttons with visible focus.
At larger sizes the prepared body follows identity inside the docked surface.
Below 110 columns or 35 rows, the held body moves to a separate read-only page
(`PgDn` / `PgUp`); Enter cannot submit a hidden confirmation from that page.

Help captures input without discarding palette query/selection, reader selection,
resend configuration or a prepared review. Lock expiry and pending results keep
processing underneath. Actions cannot open over a review or full-content load.
Ctrl+C retains safe quit/release behavior even in Help; q is captured by Help and
is query text in the palette, with the existing local q policy elsewhere.
Use `--mouse=false` for keyboard-only operation and native terminal selection.
With mouse reporting enabled, text-selection modifiers depend on your terminal;
this does not add application drag selection or claim native/tmux/SSH validation.

## Resend the selected DLQ message

Highlight a DLQ message and press `Enter` to focus the same right-hand inspector
without acquiring a lock. At compact widths it becomes full-width; reading and
copying work down to 60×16. Resend confirmation requires 80×24.
For a queue's DLQ, use `Ctrl+R` or Tab to **Resend to queue**, optionally enable deletion below,
then choose **Review resend**. The destination is fixed to that same queue's
Active messages; there is no queue picker. Ctrl+P also offers **Resend selected
DLQ message to this queue…**.

Subscription DLQs have no parent queue, so they retain the explicit **Send copy
to…** queue picker. Unwatched queues in the connected namespace are included;
this flow does not silently send back to the topic or cause topic fan-out.
Topic destinations and cross-namespace copies are not part of this flow.

**Delete original after confirmed resend** is unchecked by default:

- **Off:** re-peek that exact sequence and send its full content, not the truncated
  display preview. No receive, scan or lock acquisition; original stays in the DLQ.
- **On:** first receive/lock that exact original for the final review. This can
  temporarily lock other DLQ messages and releasing them can increase delivery
  counts. Search stops after 100 messages or five seconds; bounded release/close
  work can take additional time. If unavailable or cleanup of unrelated locks is
  uncertain, nothing is sent. It never substitutes the next message.

Final review defaults to Cancel and freezes destination/cleanup; `b` returns to
editing after releasing any lock. Send is a single application invocation; only
a confirmed send permits the checked original cleanup. An unknown send retains
the original; confirmed send with unconfirmed deletion is a partial result.
This is not an atomic move or an exactly-once guarantee. The existing explicitly
named receive-next workflow below remains available independently.

Configuration, final review and outcomes replace the inspector's reading area,
not the workspace. Returning keeps the same message snapshot and cursor; use
Esc then `r` to explicitly refresh the list after an operation.

## Precise read-only copying

`yy` and `V` use original logical lines, not soft-wrapped terminal rows. `v`
selects grapheme-safe characters with inclusive endpoints. Source UTF-8, tabs,
CRLF/newlines and final-newline presence are preserved; gutters and wrapping
are not copied. `Y` copies only the complete body. Ctrl+P separately offers
**Copy message report** (labelled metadata, body and readable properties),
**Copy line**, and **Copy selection**. Binary/non-text body copying is unavailable.

Truncated previews are marked. Full-body/report copy offers a Cancel-default,
read-only exact-message load; after loading, the size is shown and a second copy
command is required. A missing or changed message never substitutes a neighbor.
Full-load failures leave the clipboard unchanged. Clipboard errors keep visual
selection for retry and never claim a successful copy.

Clipboard transport uses local `pbcopy`, `wl-copy`, `xclip` or `xsel` where
available. SSH/Mosh and terminals without a local writer use OSC52, with a 1 MiB
limit and no silent trimming. Native utility success says **Copied**; OSC52 says
**Copy requested · terminal confirmation unavailable**. No clipboard read-back
occurs. Terminal permissions may block remote requests. tmux needs
`set-clipboard on`; GNU screen wrapping is supported. `?` explains the explicit
manual terminal-copy fallback, which may include gutters and soft wraps.
Text labels, mode names, focus markers and help supplement theme colors;
real screen-reader compatibility and native macOS clipboard execution remain
unverified on this Linux orb.

## Slice 4 — reviewed single-message DLQ actions

“Receive next DLQ message…” acquires one message for review; it does **not** act
on the highlighted peek row. Review shows the actual received message, source,
destination and lock deadline. Cancel is the default. Cancel releases the lock;
release can be unconfirmed and receiving/releasing can increase delivery count.
There is no automatic lock renewal or silent replacement after expiry.

Resend creates a new copy at the original queue/topic with a new message ID;
topic replay can reach other matching subscriptions. Removing the original after
a confirmed send defaults **off** and is a separate authorized settlement.
Delete requires typing `delete` for the actual held message. Session-enabled
mutation and advanced AMQP replay representations are unsupported, not silently
converted. Race/fixture/PTY and seeded real-broker checks pass, including the
90 KB body and supported scalar properties, TTL/correlation, retained original,
separate cleanup and topic fan-out. Emulator binary application properties are
explicitly rejected by the broker; cloning them is unit-tested, not a successful
broker round trip. Actual review renders were inspected in all three themes and
at 80×24, including the compact read-only body page. Real-emulator cancel/release,
typed delete/deletion outcome and topic fan-out warnings were also inspected.
Unknown send, expired review and confirmed-send/uncertain-cleanup screens were
inspected with temporary offline fixture wrappers, not induced broker failures.
The inspected keyboard/pointer recording shows a retained-original resend and
the final **Lock release confirmed** wording. Evidence is indexed in the
[Slice 4 completion record](../../docs/plans/service-bus-explorer/00-status.md#slice-4-completed--2026-10-24).

**Gate 3 retry adjustment approved by Andreas on 2026-10-24:** trust the pinned
SDK's internal retry/recovery, without an SDK patch. The application submits each
send/settlement once and never automatically re-invokes it or an entire replay.
SDK recovery can nevertheless repeat wire attempts, including settlement
fallback; this is not a single-wire-attempt or exactly-once guarantee. An unknown
send outcome means **check destination before retrying**, retain the original,
and perform no automatic cleanup. Unknown settlement is not confirmed deletion.
Confirmed send plus failed/unknown cleanup is a distinct partial result; neither
send and cleanup atomicity nor duplicate-free processing is promised.

Preferences use `os.UserConfigDir()/service-bus-explorer/preferences.json`
(`$XDG_CONFIG_HOME` or `~/.config` on Linux; `~/Library/Application Support` on
macOS). Override with `--config /path/preferences.json`; `--ascii` substitutes
ASCII UI glyphs. Changes apply immediately but persist **only on Save**. The
footer marks unsaved edits. Malformed/unsupported files stop startup and are
never silently replaced. Files are written privately via atomic replacement.

Watchlists retain full profile/namespace/queue or topic/subscription identity.
An absent profile initially watches all discovered resources; an explicit empty
list watches none. Unwatched resources remain browsable under Queues, Topics and
the tree. Dashboard inclusion never grants MCP access. No connection strings,
credentials or message contents are persisted.

Counts refresh 30 seconds after each bounded sweep, with manual refresh and at
most four concurrent reads. Discovery/browsing remains responsive during reads.
Failed reads retain prior values marked `~` (stale), with observation time and
error details for the selected entity; `—` means unavailable/unobserved, not zero.
The emulator's known counter limitation remains unavailable after refresh.

## Reproducible local lab

Requires Docker Engine/Desktop with Compose, Python 3 and Go. The pinned
development-only emulator and SQL Edge images require acceptance of Microsoft's
license terms. SQL Edge is retired; this lab ran on x86_64 Linux, **not yet on
macOS/Apple Silicon**. No Azure account or cloud credentials are used.

From the repository root (after reviewing the license terms):

```sh
make -C apps/queue lab-seed LAB_FLAGS="--accept-eula --confirm-disposable"
make -C apps/queue run-emulator
```

`lab-seed` **destroys/recreates only `sbx-explorer-lab`**, then seeds 12 queues
and topic `events` with subscriptions `billing` and `shipping`. Queue IDs/body
values come from the same fixture scenario. Topic publications fan out: Billing
has seven active/one DLQ seed messages; Shipping has five active/three DLQ, not
the fixture's independent subscription samples. These are insertion expectations,
not runtime counters. Notifications includes plain-text and terminal-control
test bodies. JSON output escapes control characters.

In this orb, add `--sudo --orb` to `LAB_FLAGS`. Compose runs as the supervised
`sbx-explorer-lab` service, using the existing local Docker daemon. On a normal
developer machine it runs via Docker Compose detached mode. SQL credentials are
generated under `~/.local/share/sbx-explorer-lab/`, never committed. AMQP binds
only `127.0.0.1:5673`, management only `127.0.0.1:5301`; these differ from the
earlier experimental lab's ports. Nothing is exposed publicly.

Other commands (add `LAB_FLAGS="--sudo --orb ..."` in this orb):

```sh
make -C apps/queue lab-status
make -C apps/queue lab-peek LAB_ARGS="--entity orders --max 20"
make -C apps/queue lab-peek LAB_ARGS="--entity events/shipping --dlq"
make -C apps/queue lab-send LAB_FLAGS="--confirm-disposable" LAB_ARGS="--entity empty --body hello --id manual-1"
make -C apps/queue lab-dead-letter LAB_FLAGS="--confirm-disposable" LAB_ARGS="--entity empty --max 1"
make -C apps/queue lab-drain LAB_FLAGS="--confirm-disposable" LAB_ARGS="--entity empty --dlq --max 10"
make -C apps/queue lab-down LAB_FLAGS="--confirm-disposable"
```

`lab-up` starts without seeding; `lab-reset` recreates an empty broker and needs
both `--accept-eula` and `--confirm-disposable`. Drain/dead-letter report actual
acknowledged settlements up to the requested maximum, not an aggregate total.
Send/settlement failures may have unknown outcomes; don't blindly repeat them.
Lab commands reject endpoint/project overrides and verify the pinned image,
Compose project and fixed loopback ports before messaging. Neither the TUI nor
lab CLI accepts a cloud connection string. Never point a lab command at Azure.

## Check

```sh
make -C apps/queue test
python3 -m venv apps/queue/.venv
apps/queue/.venv/bin/pip install -r apps/queue/scripts/requirements.txt
make -C apps/queue terminal-test # mouse-on and mouse-off fixture runs
# Or run just the real --mouse=false keyboard fallback:
(cd apps/queue && .venv/bin/python scripts/terminal-smoke.py --mouse-off)
```

The PTY test runs the real binary and decodes its screen; no Docker, Azure or
credentials are needed. It covers 12-card layout, grid/table, navigation, Active/
DLQ previews, paging, resize, shared Actions/palette/Help keyboard and SGR mouse
press/release capture/return, reporting enable/disable, actual theme colors and
saved-watchlist reload after restarting the binary. Go tests additionally cover
full identities/empty watchlists, stale counts, nonoverlapping bounded reads,
tree grouping, inspector scrolling, Unicode and ASCII frames across themes,
Help retaining drafts/selection/prepared targets while expiry/full-load completes,
current palette busy availability, select-only cards/table/message rows versus
explicit activation, independent disclosure/settings/watch toggles, wheel locality,
visible focus traversal and exact reader-local pointer copying. Review safety
tests retain compact/hidden confirmation refusal, expiry, frozen identity, late
delivery/release, typed delete and partial/unknown outcome coverage.
The environment PTY suite also exercises exact group snapshots, known namespaces
across subscriptions, Apply versus Save, filtered inclusion, private-file reload,
field input ownership and compact quit. Environment screens reuse shared Surface
controls and contextual Help: row clicks select, separate checkboxes/actions
activate, and confirmations start on Back / Cancel. Hidden controls cannot submit
after resize. These offline checks do not establish Azure permission/network behavior.
PTY tests currently require POSIX (Linux/macOS).
Native macOS execution is not yet validated; cross-compilation is not that proof.

After `lab-seed`, run the separate real-broker checks **serially**, without other
consumers mutating the lab:

```sh
make -C apps/queue integration-test
```

This exercises the real Go SDK and real TUI against the emulator: discovery,
Active/DLQ/subscription browsing, explicit page cursors, refresh, message bodies,
empty queues and unavailable counters. Slice 4 checks additionally cover the
90 KB body/scalar-property replay, independent source/replay TTL, original
retention, separate cleanup and topic fan-out; real-emulator PTY covers
receive/review, cancel/release, retained-original resend and typed delete.
Run against a fresh seed: the checks leave topic replay copies, delete two reviewed
Orders DLQ messages and one selected Shipping subscription DLQ message. Selected
queue replay checks also create and drain their own isolated probe messages.
These checks do not prove cloud authentication, accurate
Azure runtime counts, binary-property broker preservation or native Mac/Windows
execution. Unknown outcomes are exercised deterministically offline.

Design, future lab commands and the known emulator count limitation are recorded
in [the independent project plan](../../docs/plans/service-bus-explorer/00-status.md).
No root build/setup or LazyFTP files are required or modified by these commands.

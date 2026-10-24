# Service Bus Explorer — fixture and local emulator workspace

An independent keyboard-first terminal workspace, alongside—but not part of—LazyFTP.
**Two explicit modes:** deterministic offline fixtures, or the independent local
Service Bus emulator. Browsing is read-only; Slice 4 also includes explicitly
reviewed single-message DLQ actions. Automated, PTY, real-emulator and final
rendered-state/interaction checks passed on Linux (2026-10-24).
No cloud connection or MCP access exists.
Layout, theme and dashboard
watchlists can be saved explicitly; credentials and MCP grants cannot.
Fixture counts are sample data. Emulator counts are **unavailable**, never zero
or totals inferred from peek pages.

## Run

Install Go 1.27.3, then from the repository root:

```sh
make -C service-bus-explorer run-fixture
```

Or inside this directory: `go run ./cmd/sbx --fixture`. Select exactly one of
`--fixture` or `--emulator`; the app never silently falls back to sample data.

The workspace includes a 12-card dashboard at 160×48, denser table over the
same watched queues/subscriptions (14 by default), a collapsible namespace/entity tree, bounded
Active/DLQ peek pages, and a read-only metadata/body inspector. Smaller terminals
show fewer cards; move through the list to reach the rest. Under 110 columns the
sidebar hides automatically. The full inspector appears when enough width exists.

| Key | Action |
| --- | --- |
| `j/k`, arrows | Move selected entity or message |
| `Enter` | Open selected entity's Active preview |
| `]` | Open selected entity's DLQ / switch to DLQ |
| `[` | Switch to Active in message view |
| `g` / `t` | Grid / table dashboard |
| `Ctrl+P` | Open fuzzy actions; type to filter, arrows/Ctrl+N/P choose, Enter runs, Esc closes |
| `Tab` / `Shift+Tab` | Cycle visible content, inspector and sidebar panes |
| `Ctrl-H` / `Ctrl-L` | Focus left / right across visible sidebar, content and inspector; clamp at either edge (Ctrl-H still deletes in text inputs) |
| `h` / `l` | Collapse / expand the selected sidebar group |
| `b` | Collapse/expand sidebar |
| `i` | Toggle inspector in message view |
| `n` / `p` / `r` | Next / previous / refresh peek page |
| `r` on Dashboard | Refresh watched counts |
| `j/k`, Page Up/Down in inspector | Scroll without changing selected message |
| `s` in Settings | Save preferences explicitly |
| `Esc` | Return to resource list/dashboard |
| `q`, `Ctrl+C` | Quit |

Settings and Ctrl+P provide Midnight, Gruvbox and Light themes, layout controls,
watchlist editing and Save preferences. Printable keys—including `q`, `j`, and
`k`—belong to the palette query while it is open.

The compact bordered toolbar exposes Active, DLQ, Refresh and Actions shortcuts.
These are keyboard-first controls, not mouse targets. Review uses horizontal
Cancel / release, Resend copy and Delete original buttons with visible focus.
Below 110 columns or 35 rows, the held body moves to a separate read-only page
(`PgDn` / `PgUp`); Enter cannot submit a hidden confirmation from that page.

## Resend the selected DLQ message

Highlight a DLQ message and press `Enter` to open it without acquiring a lock.
For a queue's DLQ, choose **Resend to queue**, optionally enable deletion below,
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
[Slice 4 completion record](../docs/plans/service-bus-explorer/00-status.md#slice-4-completed--2026-10-24).

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
make -C service-bus-explorer lab-seed LAB_FLAGS="--accept-eula --confirm-disposable"
make -C service-bus-explorer run-emulator
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
make -C service-bus-explorer lab-status
make -C service-bus-explorer lab-peek LAB_ARGS="--entity orders --max 20"
make -C service-bus-explorer lab-peek LAB_ARGS="--entity events/shipping --dlq"
make -C service-bus-explorer lab-send LAB_FLAGS="--confirm-disposable" LAB_ARGS="--entity empty --body hello --id manual-1"
make -C service-bus-explorer lab-dead-letter LAB_FLAGS="--confirm-disposable" LAB_ARGS="--entity empty --max 1"
make -C service-bus-explorer lab-drain LAB_FLAGS="--confirm-disposable" LAB_ARGS="--entity empty --dlq --max 10"
make -C service-bus-explorer lab-down LAB_FLAGS="--confirm-disposable"
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
make -C service-bus-explorer test
python3 -m venv service-bus-explorer/.venv
service-bus-explorer/.venv/bin/pip install -r service-bus-explorer/scripts/requirements.txt
make -C service-bus-explorer terminal-test
```

The PTY test runs the real binary and decodes its screen; no Docker, Azure or
credentials are needed. It covers 12-card layout, grid/table, navigation, Active/
DLQ previews, paging, resize, palette query ownership, actual theme colors and
saved-watchlist reload after restarting the binary. Go tests additionally cover
full identities/empty watchlists, stale counts, nonoverlapping bounded reads,
tree grouping, inspector scrolling, Unicode and ASCII frames across themes.
PTY tests currently require POSIX (Linux/macOS).
Native macOS execution is not yet validated; cross-compilation is not that proof.

After `lab-seed`, run the separate real-broker checks **serially**, without other
consumers mutating the lab:

```sh
make -C service-bus-explorer integration-test
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
in [the independent project plan](../docs/plans/service-bus-explorer/00-status.md).
No root build/setup or LazyFTP files are required or modified by these commands.

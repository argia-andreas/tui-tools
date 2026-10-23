# Service Bus Explorer — fixture and local emulator browsing

An independent keyboard-first terminal workspace, alongside—but not part of—LazyFTP.
**Two explicit modes:** deterministic offline fixtures, or read-only browsing
against the independent local Service Bus emulator. No cloud connection,
interactive receive/settlement operations or saved preferences exist yet.
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
same 14 watched queues/subscriptions, expanded/collapsible navigation, bounded
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
| `Tab` / `Shift+Tab` | Switch sidebar/content focus when sidebar visible |
| `b` | Collapse/expand sidebar |
| `i` | Toggle inspector in message view |
| `n` / `p` / `r` | Next / previous / refresh peek page |
| `Esc` | Return to resource list/dashboard |
| `q`, `Ctrl+C` | Quit |

Settings is an explicitly labeled placeholder. Ctrl+P, theme selection, saved
watchlists and reviewed mutations arrive in later approved slices; they are not
working features yet. Three semantic
palettes exist internally, but this slice renders the default Midnight theme.

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
DLQ previews, paging and resize. PTY tests currently require POSIX (Linux/macOS).
Native macOS execution is not yet validated; cross-compilation is not that proof.

After `lab-seed`, run the separate real-broker checks **serially**, without other
consumers mutating the lab:

```sh
make -C service-bus-explorer integration-test
```

This exercises the real Go SDK and real TUI against the emulator: discovery,
Active/DLQ/subscription browsing, explicit page cursors, refresh, message bodies,
empty queues and unavailable counters. It does not prove cloud authentication,
accurate Azure runtime counts, or the future interactive resend/delete flows.

Design, future lab commands and the known emulator count limitation are recorded
in [the independent project plan](../docs/plans/service-bus-explorer/00-status.md).
No root build/setup or LazyFTP files are required or modified by these commands.

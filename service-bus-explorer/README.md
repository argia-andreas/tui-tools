# Service Bus Explorer — first runnable slice

An independent keyboard-first terminal workspace, alongside—but not part of—LazyFTP.
**Currently fixture-only:** deterministic offline data, no Azure connection,
credentials, receive/settlement operations or saved preferences. Sample counts
are not measurements from a broker.

## Run

Install Go 1.27.3, then from the repository root:

```sh
make -C service-bus-explorer run-fixture
```

Or inside this directory: `go run ./cmd/sbx --fixture`. Passing `--fixture` is
required; the app never silently falls back from a cloud connection to mock data.

The first slice includes a 12-card dashboard at 160×48, denser table over the
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
watchlists, emulator integration and reviewed mutations arrive in later approved
slices; they are not working features of this tracer bullet. Three semantic
palettes exist internally, but this slice renders the default Midnight theme.

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

Design, future lab commands and the known emulator count limitation are recorded
in [the independent project plan](../docs/plans/service-bus-explorer/00-status.md).
No root build/setup or LazyFTP files are required or modified by these commands.

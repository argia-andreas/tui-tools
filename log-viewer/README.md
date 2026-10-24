# Log viewer — fixture and local Elasticsearch workbench

An independent keyboard-first terminal logs app in the LazyFTP/Service Bus visual
family. Choose **offline fixtures** or an **authenticated TLS local Elasticsearch
lab**. Neither mode connects to the unidentified company backend. `logview` is a
neutral executable name.

## Run

Requires Go 1.27.3. From the repository root:

```sh
make -C log-viewer run-fixture
```

An explicit `--fixture` or `--config` is required; no flags does not discover
credentials or connect to a default source. The 22 embedded fictional events use a fixed
2026-10-23 14:15–14:30 UTC window, two apps, multiline Go stacks, typed nested
fields, missing severity, and deliberately hostile input. No network, credential
loading, automatic clipboard access, event persistence, or MCP server is started
in fixture mode. Clipboard access requires deliberate inspector `y`/`Y`.

### Local Elasticsearch lab (native Linux x86_64)

```sh
make -C log-viewer lab-install lab-up lab-seed lab-check
make -C log-viewer test-integration test-terminal-lab
make -C log-viewer run-lab
make -C log-viewer lab-down
```

The stdlib Python harness downloads Elasticsearch 9.5.5 with its bundled JDK,
verifies the committed SHA-512 before safe extraction, and requires non-root,
3 GiB installation disk space, at least 1 GiB available RAM and a descriptor hard
limit of 65535. It raises the process soft descriptor limit within that hard limit.
In an Amp orb it uses a supervised private service; it requires the `amp` CLI.
Docker and other host supervisors are not implemented.

The Basic-license node binds HTTP/transport only to loopback, with a private CA,
TLS and authentication. ML and mmap are disabled; heap is 512 MiB. Measured in the
8 GB orb: 18.71 seconds to authenticated readiness, 873 MiB Java RSS at readiness,
roughly 911 MiB warm. These are observations, not hard memory/performance guarantees.

All state lives in ignored, ownership-checked `log-viewer/.lab/`; credentials and
private keys are private files. Provisioning and viewer keys are separate. Seeding
overwrites deterministic IDs in two exact owned indices and explicitly refreshes;
it never resets/deletes indices. `lab-check` proves viewer reads succeed while an
owned-index write and forbidden-target read both return 403. `lab-down` stops only
the named owned service and preserves state for restart. No public portal exists.

`run-lab` starts `logview --config .lab/source.json`. Config decoding rejects
unknown/duplicate fields, trailing values and oversized input. This slice permits
only the exact local lab origin/source/target and fixed ECS mapping; it deliberately
does **not** apply fictional-data policy to production sources. The window is
explicit historical UTC, not “now”. The UI labels the real HTTP source as
`ELASTICSEARCH · LOCAL LAB · TLS` and its records as seeded fictional logs.

For a lab installed before Slice 3, upgrade the disposable test mapping explicitly:
`python3 log-viewer/scripts/lab.py reseed-indexed-fields`. This replaces only the
fixed owned events index and reseeds its 22 baseline events; ordinary startup and
seeding never reset it.

## Search and saved filters

Press `f` from logs to edit exact app selections, time, levels, message phrase and
trace IDs. The facet navigator keeps the summary on the left and edits only one
facet on the right. Tab/Shift+Tab switches focus between the sidebar and editor.
In the sidebar, ↑/↓ or j/k selects a facet; in Time, those keys choose From or To.
The focused pane is highlighted, and draft values survive pane switches.
Vim-style facet input is **on by default**:

- **NORMAL**: ↑/↓ or j/k navigates; Enter or `i` starts typing in the selected
  field. In App, Space toggles the chosen exact app; Enter/`i` edits the keyword.
- **INSERT**: printable keys, including j/k, are literal text. Enter or Esc returns
  to NORMAL without losing the draft; Tab returns to sidebar navigation.
- Ctrl+S applies and returns to logs from either mode or pane. Esc in NORMAL
  returns without applying.

Ctrl+L clears all draft filter criteria except the time window; Ctrl+S applies the
reset. Apps OR together; different facets AND together. App suggestions use a
separate case-insensitive substring keyword, not an applied predicate. Ctrl+X
clears app selections. Exact-field lists support ↑/↓ and j/k; Enter removes a
predicate, without entering INSERT.

Press `,` or choose **Settings** in Ctrl+P to toggle **Vim-style facet input**.
Enter/Space changes the draft preference, Ctrl+S saves it, and Esc cancels.
Settings persist in the platform config directory's `logview/settings.json`;
override with `--settings /private/dir/settings.json`. Files are read-only at
startup and created only on an explicit settings save (directory 0700, file 0600).
With Vim mode off, text fields accept typing immediately, Esc cancels the editor,
and Enter toggles App suggestions as before. Find, save-name, and command-palette
prompts always accept typing directly; the preference affects only facet input.

Times require RFC3339 with `Z` or an explicit offset, and use a half-open window.
`WARN+` means WARN/ERROR/FATAL, not UNKNOWN. Fixture phrase matching is an explicitly
labeled approximation; Elasticsearch uses its analyzed phrase semantics.

From the inspector, press `f` to open the picker on the highlighted field, then
Enter to add an exact AND filter without losing existing facets. Trace ID, app,
instance, level and selected indexed structured fields are supported;
object/null/redacted/unindexed fields
explain why they cannot be used. Lab apps map to ECS `service.name`; production
`app.keyword` mapping is not configured or inferred. In the filter editor's exact
predicate row, Enter removes the selected predicate and Ctrl+R clears all of them.

`S` explicitly saves the whole current query under a name; `R` restores one for the
same source. No automatic writes occur. The default is the platform config
directory's `logview/searches.json`; override with `--searches /private/dir/searches.json`.
Its directory/file must be owned and private (0700/0600). Saves contain absolute
times and filter values, which may be sensitive, but no events, credentials or
cursors. Saving an existing name replaces that search.

On wide terminals, the workspace shows current-source saved searches instead of
planned-provider placeholders. Tab to it, j/k or arrows choose, Enter restores.
Explicit saves update the list immediately; entering the workspace or opening `R`
reloads it. Rendering never reads the file. `R` remains available when the sidebar
is hidden; collapsing the sidebar returns focus to logs.

Inspector keys are cyan, values bright, ERROR red, numbers amber and booleans mint.
Recognizable Go `stack_trace` function/path/line-number spans get restrained syntax
color without rewriting content. Other stack formats remain ordinary text. This
is presentation of policy-safe data, not a parser or a raw-data escape hatch.
`j/k` or arrows select a visible native field, with a mint row and exact JSON
pointer at the inspector foot. Enter collapses/expands objects and arrays, or
opens the full wrapped multiline value instead of its shortened one-line preview.
PgUp/PgDn scroll expanded values; the visible continuation retains field focus.
Selection survives width changes and full-width detail entry; hidden panels never
keep keyboard ownership. `g/G` or Home/End select the first/last visible field.

`y` sends the selected full policy-safe value, even when its preview ends in `…`;
`Y` sends `JSON-pointer=value`. Strings remain text; other types, including objects
and arrays, use stable compact JSON with exact numbers. Redaction is retained.
An event flagged incomplete by its provider refuses copy conservatively, rather
than claiming an available prefix is complete. Current fixture/Elastic adapters
retain complete bounded events or fail closed; page partiality does not imply
individual field truncation.

Native copy detects `pbcopy` (macOS), `wl-copy` (Wayland), or `xclip` (X11), feeds
stdin without a shell, and times out after two seconds. No native command means
copy is unavailable unless you explicitly launch with `--trust-osc52`. That flag
trusts the terminal clipboard path, including any SSH/tmux/terminal limitations.
Native command completion and OSC52 “Copy sent” both report **receipt unverified**;
neither proves the receiving clipboard accepted the value. Failure never claims
success, exposes backend stderr, echoes a value, or falls back to a temporary file.

## Keyboard

| Key | Available now |
| --- | --- |
| `j/k`, arrows | Select a log or inspector field |
| `PgUp/PgDn`, `g/G`, Home/End | Page/scroll, or select first/last visible item |
| Tab / Shift+Tab | Cycle visible panels; only one owns bright focus |
| Enter | Inspect event; expand/collapse inspector field; full-width details when narrow |
| `y`, `Y` | Yank selected field value / JSON-pointer=value (policy-safe only) |
| Esc | Return to logs, retaining selection |
| `f` | Edit filters; from inspector, add an exact field predicate |
| `/`, `n/N` | Literal find in loaded policy-safe values; next/previous match |
| Ctrl+T | Toggle case sensitivity inside find |
| `o`, `r` | Load older snapshot history / reload a fresh snapshot |
| `S`, `R` | Name/save current query / restore a saved query |
| Ctrl+P | Fuzzy command palette; arrows/Tab choose, Enter runs, Esc returns |
| `q`, Ctrl+C | Quit and restore the terminal |

The workspace auto-hides below 140 columns, and the inspector below 120. Enter
still opens details at narrow widths. Minimum size is 60×16. Resizing cannot leave
focus in a hidden panel. The log row shows a one-line summary; the inspector wraps
the full retained message and displays multiline strings and typed structure.

The compact centered palette floats above a dimmed workbench, with a raised
surface, highlighted selection and right-aligned shortcuts. It shrinks with the
match count and scrolls within terminal bounds instead of replacing the screen.
It lists only implemented actions, reusing their existing behavior.
Its filter action is contextual: source filters from logs/workspace, exact field
filtering from inspector. Disabled actions show the current reason (for example,
no selected event, no older history, or no local find yet); Enter cannot run them.
Typing `q`, `y` or Space in the palette is input, not a global shortcut. Ctrl+U
clears the fuzzy query. Esc restores the originating panel, repairing hidden focus
on resize; results and selection are retained. The palette does not save anything
until the separate named-save editor is explicitly submitted.

Keyboard help and follow currently explain that they are unavailable.
No fake “copied,” “connected,” or “following” success is displayed. The provider
names under “PROVIDERS · PLANNED” are not working connections. Local Elasticsearch
is a working source in lab mode, not follow/streaming.

## Boundary and safety

The terminal receives only `SafeEvent` values from the investigation service,
never raw provider records. Source policy explicitly selects one source and output
roots; redaction covers configured JSON-pointer paths (including `*` segments)
and regex patterns in nested arrays/objects, messages, and field names. Invalid
control-bearing keys are dropped rather than renamed into ambiguous paths.
Terminal escape sequences and Unicode format/control characters are removed
before retention; multiline text remains available for inspection. This
conservative sanitation also removes zero-width format characters from text.

The fixture policy redacts `/fields/auth/token` and the deliberate
`fixture-secret-…` canary pattern. **It is not an automatic secret detector.**
Production field policies, production connectivity, and MCP authorization are not
implemented. Local-lab authentication is implemented. Output policy is not an MCP
permission grant.

Reads require a valid half-open time window and explicit event/byte budgets.
Service ceilings are 500 events, 32 MiB of retained field data and 1 MiB per encoded
event. A byte-limited page is labeled partial; an oversized/invalid event fails
closed. Backend error details are not rendered. The Elastic adapter caps response
bytes at 8 MiB, disables automatic decompression, refuses redirects/credential
forwarding, verifies TLS with the explicit CA, and has a 15-second request timeout.
Timeout/shard-failure responses fail closed rather than claiming complete results.
Newest bounded pages display chronologically. Older history uses a frozen PIT
snapshot and timestamp plus native tie-break ordering; appended events appear
only after reload. Opaque one-use cursors expire after two minutes idle, with at
most 16 live sessions. History stops explicitly at 10,000 events / 32 MiB rather
than evicting the selected event. Expired/error cursors require reload, never a
broader fallback query. These are
resource limits, not measured throughput or exact whole-process memory budgets.

## Verify

```sh
uv venv log-viewer/.venv
uv pip install --python log-viewer/.venv/bin/python -r log-viewer/scripts/requirements.txt
make -C log-viewer check test-terminal
# Optional adversarial output-boundary fuzzing:
go -C log-viewer test ./internal/logs -run '^$' -fuzz FuzzSafeOutput -fuzztime 15s -parallel 2
```

`check` runs formatting validation, vet, and race tests. The PTY test runs the
actual executable and checks explicit startup mode, event/inspector contents,
wide/narrow focus recovery, unavailable actions, secret/terminal-control exclusion,
app OR and other facet composition, exact inspected fields, save/restore, literal
find, older paging, editor cancellation, and clean exit. A separate trusted-OSC52
PTY decodes exact y/Y payloads (including full multiline and redacted object
values), exercises selected-field filtering and wide→narrow focus recovery.
This verifies transmission, **not receiving clipboard contents**. Native-copy
tests use disposable stdin capture/failure commands, including a timeout; these
are not real host clipboard receivers. Inspected screenshots and the actual
19.8-second keyboard/failure clip are indexed in the delivery status.
Fuzz seeds include escapes that could reconstruct a secret during
cleanup; redaction applies to both incoming and sanitized text.

Midnight semantic color roles are app-owned values from the existing
`internal/theme/theme.go` palette, not invented styling or imports across Go
`internal` boundaries. This slice does not add theme switching or modify either
existing application. See [design provenance](../docs/plans/log-viewer/design-references.md),
[delivery status](../docs/plans/log-viewer/00-status.md), and
[approved slices](../docs/plans/log-viewer/04-slices.md).

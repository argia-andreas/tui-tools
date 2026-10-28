# SBX click inventory — shared terminal UI slice 6

This inventory covers visible actionable controls, destinations and data selection
in the actual app, not the design mockup. `terminalui.HandleNavigation` interprets
current absolute/clipped workspace regions emitted by the same renderers as View.
Captured Help, palette, review and full-load surfaces retain their shared handlers.
No pointer path synthesizes keyboard messages or calls a broker operation directly.
Stable row IDs encode full resource identity or resource/DLQ/page/sequence/message
identity, not rendered labels or page indexes.

## Controls and exercised scenarios

| Screen/control | Keyboard | Pointer | Enabled / hidden conditions | Exercised scenario |
| --- | --- | --- | --- | --- |
| Workspace pane focus | Ctrl+H/L; Tab/Shift+Tab | Blank pane body | Sidebar requires preference and ≥110 columns (≥122 on messages); inspector requires current layout | Directional-focus width matrix, resize hidden-control repair, PTY sidebar focus |
| Dashboard / Queues / Topics / Settings destinations | Navigation j/k then Enter; Actions | Label click opens destination | Only visible sidebar rows; review captures background | PTY Settings/Dashboard; Go tree and directional tests; Topics keyboard PTY |
| Namespace / Queues / Topics / topic disclosure | Navigation h/l; Enter toggles branch | Disclosure glyph only toggles | Descendants absent when collapsed; no destination side effect | Go independent label/disclosure, tree subscription DLQ; SGR namespace toggle |
| Branch label | Navigation j/k | Label selects only | Current visible tree row | Independent label/disclosure Go + PTY |
| Queue/subscription destination | Navigation Enter Active / ] DLQ | Leaf destination opens Active | Full native resource identity; hidden descendants have no region | Go subscription DLQ identity; general destination frame route |
| Dashboard cards / dashboard, queues, topics table rows | j/k/arrows selects; Enter opens Active | Select only | Current visible page; none while discovery loading | Go + SGR payments card, shipping table, explicit open |
| Open Active / Peek DLQ | Enter / ]; Tab then Enter | Labelled action | Refuses empty resource list; no receive/locks | Go + SGR select-other-entity then explicit action |
| Grid / Table | g / t; Tab then Enter | Labelled action | Whole control must fit | SGR grid/table and PTY keyboard parity |
| Message toolbar Active / DLQ / Refresh / Actions | [ / ] / r / Ctrl+P | Bordered controls | Hidden in compact full reader; captured by review/load; no background action | SGR shipping toolbar; existing paging/refresh PTY |
| Message rows | j/k/arrows selects; Enter inspects | Select only | Current loaded visible page; stale rows not interactive during loading | Go + SGR select second row then j, verify sequence 114 before Inspect |
| Inspect / Open | Enter; Tab then Enter | Explicit button | Requires loaded nonempty page; omitted if clipped | Go + SGR selected shipping message 3 |
| Next / Previous page | n / p; Tab then Enter | Explicit buttons | Availability requires next sequence / history; whole controls omitted if narrow | Existing PTY n/r/p; checked local changePage shared with pointer dispatch |
| Reader text pane | Enter inspect; Ctrl+L; Tab | Body focus | Reader must be visible; no arbitrary mouse range selection | Go exact-copy and pane tests; PTY inspect |
| Reader close / Back | Esc clears selection/pending first, then Back; Tab/Enter explicit Back | Close / Esc Back directly returns to list | Current reader only; full load has its own cancellation owner | Existing reader-pointer tests and SGR Back/reopen |
| Reader Resend / Send copy to | Ctrl+R; Tab/Enter | Explicit footer action | DLQ only, refuses active selection/copy; never sends immediately | Existing Go selected identity and pointer-review tests; SGR configure/review |
| Reader yy line / v char / V line / Y full body | yy / v / V / Y; Tab/Enter | Local hint controls | Current text; copying/full-load gates retained; controls clipped with body | Exact CRLF, combining/CJK pointer-copy Go regression; keyboard OSC52 PTY |
| Reader Copy selection / clear / help | y / Esc / ?; Tab/Enter | Local hint controls | Selection controls only in visual mode; hidden controls removed from traversal | Exact pointer copy Go; Help preserves reader state tests |
| Wheel pane locality | j/k or local reader page/cursor keys | Wheel in pointed pane moves three rows | No focus transfer; captured surfaces do not scroll workspace | Go list-vs-reader locality; captured Help/review tests |
| Settings theme / sidebar / inspector / layout | j/k then Enter or Space; Tab/Enter | Current option row | Current settings page only; sidebar option remains available when sidebar hidden | Go all toggles; SGR sidebar/inspector; PTY theme colors |
| Legacy Settings watch checkboxes (no scope provider) | j/k then Enter or Space; Tab/Enter | Watch row/checkbox | Full profile/namespace/entity identity; Dashboard-only scope; scoped modes use Dashboard inclusion instead | Go full identity/empty watchlists; scoped PTY persistence uses inclusion |
| Settings Save | s; selected Enter; Tab/Enter | Save row | Refuses another save while saving; snapshot revision retained | Go save-snapshot and pointer-save; PTY persistence/restart |
| Help / Actions top toolbar | F1 / Ctrl+P | Explicit shared buttons | Actions unavailable with visible reason during review/full load/environment capture; Help stays available | Existing SGR Help over prepared review; palette capture PTY; environment Help restoration |
| Palette query / results / Help / close | Type/Backspace/Ctrl+U; arrows/Tab; Enter; F1; Esc/Ctrl+P | Results activate available action; Help/close; outside dismiss | Current eligibility; no stale target; unavailable/busy refuses | Existing no-results/busy/query capture Go and actual SGR PTY |
| Help close / scroll | F1/Esc; arrows/Page keys | Close; wheel inside Help | Captures outside presses; retains parent selection/draft/authorization; expiry continues | Existing Help/expiry/full-load tests and PTY |
| Resend destination filter / rows / Choose | Type/Backspace/Ctrl+U; arrows; Enter/Choose | Filter focuses; row selects only; Choose confirms | Queues only; no selected replacement after removal; compact hides Choose | Existing destination pointer select-only, filter ownership and frozen identity Go tests |
| Configure cleanup / destination change / Review resend / Cancel | Tab/arrows/Enter; Esc | Current checkbox/actions/close | Cleanup/destination only configurable before preparation; size and phase checked | Existing review pointer identity and SGR cleanup/review/cancel |
| Prepared review Cancel-release / Resend / Delete / Back / typed delete | Tab/arrows/Enter; exact `delete`; Esc | Current controls; typed authorization field focus | Rechecks expiry, phase, frozen source, busy, dimensions and typed authorization; release reconciliation required | Existing 79×24 / 80×23 refusal vs 80×24, expiry-before-press, late delivery, unknown/partial Go; SGR resend/delete/release |
| Review body and outcome return | PgUp/PgDn; Enter return | Wheel in body; current footer return | Body-only compact review never exposes submit regions; in-flight cannot pretend cancellation rolled back | Existing body-only, busy, outcome and release tests; PTY safe return |
| Full-content Load / Cancel / close | Tab/arrows/Enter; Esc | Current buttons/close | Cancel default; exact read-only lookup; late cancelled results ignored | Existing reader full-load pointer/late-cancel Go tests |

Status text, counters, card shortcut hints, body data, panel titles and footer
keyboard-reference text are not additional action buttons. Actual visible buttons,
fields, checkboxes, disclosure glyphs and destinations retain routes; none are
relabelled unsupported. Drag/drop and arbitrary text-range mouse selection are
explicitly outside the approved scope. Reader word/cursor/grapheme grammar is local.

## Scoped environments

Fixture, emulator and explicit read-only Azure modes use the environment screens.
Settings rows 5/6 are semantic environment-management/inclusion destinations, not
legacy watchlist indexes. Discovery membership and dashboard inclusion remain
separate; Apply changes the session while Save persists applied preferences.

| Surface | Keyboard / pointer contract | Safety boundary |
| --- | --- | --- |
| Switch/manage | Search or list movement; row press selects; explicit Use/New/Edit/Duplicate/Delete controls | Saved environment and connection identities; switching refuses active review/full-load |
| Edit/known namespace | Visible text fields own printable keys; Tab traverses fields/actions; field press focuses | Staged draft; exact connection/namespace validation; no field-click submission |
| Subscriptions/namespaces/selected | Row press selects; separate toggle checkbox or named Open/Preview/Details/Remove | Exact namespace identities; group membership is a reviewed snapshot, never a wildcard |
| Inclusion | Search, select-only entity rows, explicit Toggle/Select matching/All/Selected/Save | Full resource identity; All disables per-entity toggles; excluded saved selections retained |
| Review/group/All/delete/discard/quit | Cancel-first shared Surface; explicit named affirmative controls; delete requires exact name | Only visible enabled controls execute; no default destructive Enter or click-through |
| Contextual Help | F1 and shared close/scroll; closing restores the parent unchanged | Captures input, not lifecycle results; scope/count/save/review messages still reconcile |
| Resize | Scoped workspace minimum 60×18; legacy 60×16; review 80×24 | Hidden forms cannot submit; compact quit choices rendered only at ≥48×3; drafts preserved |

These routes are exercised by the environment Go regressions and
`scripts/environment-smoke.py` in mouse-on and `--mouse-off` fixture modes.
Offline fixtures do not establish Azure permission or network behavior.

## Offline fixture / capture recipes

From this app root: `go build -o bin/sbx ./cmd/sbx`, then
`./bin/sbx --fixture --config /tmp/sbx-slice6-preferences.json` with a fresh path.
No broker, lab or credentials are used. Fixture changes live only in process;
restart restores messages. Only explicit Save writes the chosen preferences path.

- At 160×48: select payments card; Open Active; Esc; Table; select shipping;
  Peek DLQ; select message 2; j; Inspect/Open. Expected selected message is
  `shipping-dlq-0003`, sequence 114. Shipping has 8 DLQ messages; notifications
  intentionally has zero. Default orders has 128 Active / 12 DLQ.
- Sidebar namespace text selects without collapsing; the `▾` toggles its children.
  Settings sidebar toggle hides only navigation, not its own return control.
- Reader `v char`, move right, Copy selection exercises exact local copy;
  `V line` selects logical lines, never terminal gutters or soft wraps. `? help`
  returns without modifying that parent. OSC52 depends on terminal permission.
- Orders DLQ → Inspect → Resend → Review prepares fixture-only state. At 80×24
  confirmation requires its actual safe view; at 79×24 or 80×23 it refuses.
  F1 preserves parent while expiry/completion continues. No live broker mutation.
- Resize while a sidebar destination/control owns focus to 80×24: hidden
  navigation is gone and Enter acts only on current visible content selection.
- Repeat with `--mouse=false`: no application reporting enable sequences;
  j/k, Enter, ], Ctrl+H/L, Ctrl+P, F1 and settings/save remain keyboard-complete.

Checks: `GOWORK=off go test -race ./...`, `GOWORK=off go vet ./...`,
`GOWORK=off go build ./...`, `GOWORK=off make terminal-test`, and
`python3 -m unittest discover -s scripts -p lab_test.py`. PTY sends actual SGR
press/release sequences and uses isolated temporary preferences. Human usability,
native terminal/tmux/SSH and screen-reader acceptance remain unmeasured; visual
captures are collected by the parent slice owner, not claimed by this inventory.

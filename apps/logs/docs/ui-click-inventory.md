# Logs control inventory — slice 6

Current runtime routes, not a promise of native-terminal or screen-reader
acceptance. `internal/ui/navigation.go` aggregates regions produced with each
rendered pane; `terminalui.HandleNavigation` interprets the current frame. IDs
retain event handles, JSON pointers and source/search names, not display rows.
App operations retain query, clipboard, private-file and output-policy checks.

| Screen/control | Keyboard route | Pointer route | Visible/enabled conditions | Offline evidence |
| --- | --- | --- | --- | --- |
| Header Actions / Help / Inspect | Ctrl+P / F1 / Enter in logs; Tab + Enter | Current complete shared buttons | Header fits; Actions blocked by editor; Inspect needs event | Palette/Help PTY; navigation PTY |
| Workspace, logs, inspector panes | Tab/Shift+Tab | Pane background/border press | Workspace ≥140 cols, inspector ≥120 or detail; minimum 60×16 | Navigation model and PTY resize |
| Source / All saved searches | Workspace arrows + Enter; Tab + Enter | Explicit destination press | Workspace visible, destination not clipped | Saved-search model; shared navigation tests |
| Saved query rows | Workspace arrows; Enter/Open restores | Row selects only; Open restores | Current source only, visible row, query validates | Same-source restore and select-only model cases |
| Event rows | j/k/arrows/Home/End/PgUp/PgDn; Enter/Inspect | Row selects only; Inspect opens | Loaded visible row, stable event handle | Navigation model + real SGR PTY at 160/80 cols |
| Log actions: Inspect / Filters / Older | Enter / f / o; Tab + Enter | Complete shared local buttons | Older refuses busy/no cursor/budget stopped; hidden whole buttons have no target | Existing history/budget tests; navigation/fixture PTY |
| Inspector field + wrapped continuation | j/k/arrows; Enter toggles selected field/value | Row/continuation selects native field only | Current event, visible owner mapping | Wrapped-field owner model; copy PTY |
| Container triangle | Enter on field; Tab + Enter on triangle | Separate triangle toggles once | Container first line + intact triangle visible | Disclosure model and real SGR press/release PTY |
| Reader Back / Expand / Filter / Copy value / Copy pair | Esc / Enter / f / y / Y; Tab + Enter | Complete current local buttons | Selected field; copy still refuses incomplete/no trusted transport, exact field support checked | Reader model, exact-field model, unchanged OSC52 payload PTY |
| Wheels | Arrow/page keys as appropriate | Pointed pane scrolls/selects without changing keyboard pane | Pane visible; release/motion ignored | Shared navigation tests; app model + real reader-wheel PTY |
| Palette query/results/Help/close | Type/arrows/Tab/Enter/F1/Esc | Result, Help, close; outside press dismisses consumed | Captures workspace, unavailable actions cannot run | Palette/Help PTY + model tests |
| Help body/close | F1/Esc/arrows/PgUp/PgDn | Wheel/close, no background activation | Parent retained; lifecycle still processed | Help/INSERT PTY + completion model tests |
| Filter facet destinations / body inputs | Sidebar j/k/arrows; Time arrows choose Start/End; i/Enter edits | Destination focuses facet; input focuses field | Visible current pane; NORMAL/INSERT semantics remain local | Facet/Vim PTYs + compact model checks |
| Filter app/predicate rows | j/k/arrows select; Space toggles App / Enter removes predicate | Labels select only; App checkbox toggles | Visible current native app/path/value; predicates preserve typed bytes | Exact-app/field model and search PTY |
| Filter Apply / Cancel / Toggle app / Remove field / Clear | Ctrl+S / Esc NORMAL / Space App / Enter predicate / Ctrl+L; Tab + Enter | Complete visible footer buttons; title close discards directly | Current availability; compact validation excludes obscured regions | Editor pointer PTY; footer traversal model + PTY |
| Filter Tab traversal | Editor → sidebar → visible actions, reverse with Shift+Tab | Click pane/field/action | Leaves INSERT retaining draft; omitted actions skipped | `TestFilterTabReachesVisibleFooterAndReturnsToDraft` and SGR PTY |
| Settings toggle / Apply / Cancel | Enter/Space, Ctrl+S/Esc; Tab + Enter | Toggle row and shared footer/close | Session-only without path; explicit private-file checks otherwise | Settings model and pointer PTY |
| Find text / case / Find / Cancel | Text/Ctrl+T/Enter/Esc; Tab + Enter | Input focuses; case toggles; footer runs checked intent | Only loaded policy-safe data searched | Find model and pointer PTY |
| Save name / Save / Cancel | Text/Enter/Esc; Tab + Enter | Input and explicit footer/close | Explicit searches path; private destination and nonempty name required | Saved-query model and search PTY |
| Restore / exact-field picker | Arrows/j/k/Home/End as shown; Enter applies, Esc cancels; Tab footer | Row selects only; explicit Restore/Apply; wheel body | Same source; redacted/unsupported values refuse; hidden rows cannot activate | Exact typed-field/refusal model + pointer PTY |

Plain status text, column headings and shortcut hint sentences are not buttons.
All implemented commands remain discoverable in Actions/Help when a narrow
terminal omits a whole local button. Drag/drop and arbitrary text-range mouse
selection are excluded from the approved scope, not substituted for missing
control routes. Copy transmission checks do not prove receiving clipboard access.

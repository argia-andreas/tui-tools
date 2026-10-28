# Integration architecture: environments using shared terminal interactions

Integration addendum. Both product designs already exist; this document resolves
their integration rather than reopening environment semantics. Andreas authorized
implementation and remaining design decisions without further gate pauses.

## Fit and evidence

Compared local main `32ca95d` with fetched origin/main `23feda3`. Upstream adds
32 files/changes, including about 4,300 inserted lines. The attempted rebase
reported content conflicts in SBX Makefile, startup, model, view and PTY script,
plus directory-relocation conflicts and a modify/delete conflict for selected.go.
The first attempt was aborted for this integration design; the resumed rebase
preserves both behaviors rather than choosing ours/theirs file-wide.

| Observed implementation | Integration consequence |
| --- | --- |
| Upstream cmd/sbx/main.go selects fixture/emulator/read-only Azure, wires ScopeProvider and migrates local preferences; local startup adds conditional mouse reporting | Retain explicit mode/tenant and provider wiring together with --mouse=false; do not lose either behavior |
| Upstream environments.go environmentUpdate handles async results and captured keys before the model's other handlers | Split lifecycle handling from input ownership; otherwise Help and shared controls lose precedence |
| Upstream environmentKey directly implements page keys and domain actions, with a single integer row/field cursor | Keep local text/navigation grammar, but expose checked semantic operations to key and pointer routes, without synthetic keys |
| Upstream environments_view.go paints full-screen panel lists and textual action hints, without shared regions | Render shared Surface chrome/actions; local body loops emit current rows/fields/controls alongside lines |
| Upstream actions.go has a private action type, fuzzy filtering and palette key handling | Add environment action descriptors to the existing terminalui.Action list; never restore the old palette implementation |
| Upstream view.go pins environment name/discovery summary; local header uses shared Help/Actions on row 1 | Compose both: pinned environment/connection badge, shared controls and concise discovery state, Details for full context |
| Upstream settings indexes 5/6 open environment management/inclusion; local changeSetting/settingID interpret indexes ≥5 as resource watch toggles | Adapt settings rendering, semantic IDs, selection bounds, key and pointer dispatch together; merging settingsKey alone is incorrect |
| Upstream uses 60×18 for environment input; local workspaceFits uses 60×16, review has its own 80×24 gate | One app-owned conditional minimum predicate; scope-enabled mode uses 60×18, legacy/no-provider mode retains 60×16, review retains 80×24 |
| Upstream useEnvironment invalidates scope, peek, count and inspector work; local navigation uses current native-identity regions | Preserve invalidation and resolve every action against current scoped identity; stale rows must not acquire new meaning |

Paths above are relative to upstream service-bus-explorer or local
apps/service-bus-explorer respectively. Existing local integration seams are
internal/ui/{interaction,navigation,review_surface,reader_surface}.go;
shared APIs are lib/terminalui/{action,palette,help,surface,navigation}.go.

## Ownership

- Retain upstream explorer contracts, config validation/deep snapshots/local
  migration, fixture catalogs, Azure providers and read-only restrictions. Move
  newly added source into apps/service-bus-explorer; preserve root plan/history.
- SBX owns environment state, exact namespace membership, group snapshot preview,
  All/Selected inclusion, Apply versus Save, cancellation/generations, connection
  identity, failure reporting and persistence. No domain code moves into lib.
- terminalui owns button appearance, current geometry/clipping, focus traversal,
  semantic input intents, palette and Help. Reuse its current public APIs.
- Add app-local environment surface/body adaptation, not a generic environment
  manager, form engine, modal stack, shared provider adapter or generic list model.
- FTP/Logs runtime behavior and shared public APIs are outside the intended diff.
  If a concrete missing shared contract emerges, stop and review that change.

## Screen adaptation

Keep the approved full-screen stepped environment flow, including compact mode;
do not transplant FTP's chooser layout or force a side-by-side editor.

| Incoming pages | Shared presentation/input | Local behavior retained |
| --- | --- | --- |
| switch/manage | Surface, search FieldRegion where applicable, select-only environment RowRegions; explicit Use/New/Edit/Duplicate/Delete controls | Stable environment ID plus connection, filtering, cross-connection refusal, active marker; delete opens review only |
| edit/known | Surface with name/address fields and explicit browse/selected/add/review/back controls | q/j/k/s remain text in fields, staged draft, unique-name and exact-identity validation, async metadata lookup |
| subscriptions/namespaces | Search field, select-only native rows, explicit Open/Preview; separate namespace checkbox/toggle target | Only explicitly opened Azure subscription metadata, exact group snapshot, retained cross-subscription basket; namespace labels do not toggle on pointer press |
| selected/details | Select-only namespace rows, explicit Details/Remove, body-local scrolling and Back | Full identities, retained parent context, exact-member removal, no wildcard changes |
| inclusion | Select-only resource rows plus distinct checkbox/control targets, search, Details, Select matching, mode and Save controls | One dashboard inclusion system; All disables checkboxes; filtered bulk selection and outside-scope saved picks keep current semantics |
| review/group/all | Surface with explicit Back/Cancel first and named Apply/Add/Enable controls | Exact namespace review, Apply & use versus Apply only, group snapshot and All-mode warning; row Enter must not accidentally confirm |
| delete/discard/quit | Cancel-first Surface with visible refused/enabled actions and reason text | Exact environment-name authorization for local delete, no Azure deletion, dirty draft refusal on Save, explicit Save/Discard/Cancel |

Enter remains contextual: list/input grammar may explicitly Open/Use/Toggle when
its owner is focused, but entering a confirmation starts on safe Cancel/Back.
Tab/Shift+Tab visit current visible fields and controls rather than retaining the
upstream switcher's Tab-to-Manage special case. The Manage action remains visible
and reachable. These deliberate interaction changes require PTY updates, not
weakened outcome assertions.

All dismissal routes (Esc, close and outside press) call the same app-owned
transition: search-clear-first where applicable, dirty-form discard confirmation,
catalog request invalidation and the correct nested return page. Shared Dismiss
never directly clears the environment state. An outside press is consumed even
when dismissal is refused.

Emit geometry in body render loops, using actual appended row positions. Current
native IDs include environment ID/connection, namespace ARM context, subscription
ID and complete resource identity. Group-preview requests also resolve against
the current catalog/snapshot generation. Never derive action identity from row
number or display name. No cached frames across resize or async completion.

## Input and lifecycle ordering

1. Always process resize and domain lifecycle messages, with existing generation
   checks: metadata/discovery/count/peek/clipboard/save/review expiry and releases.
   Help/surface capture must not starve them.
2. Ctrl+C follows the app's safe-quit policy: review cleanup first, otherwise
   unsaved-preference/draft confirmation as required. No unconditional shared quit.
3. Below the relevant minimum, freeze hidden editing/submission; preserve drafts.
   Keep only deliberately rendered compact safe-quit/cancel choices operable.
   The upstream visible undersized dirty-quit menu is not silently removed.
4. Help captures input before its parent, returning to the same draft, query,
   selected native identity and focus. F1 works in environment fields. Printable
   question marks in text fields remain text, not a global Help shortcut.
5. A palette opened from an allowed owner captures input; review/full-load blocks
   Actions as today. An environment form owns input and blocks opening global
   Actions over staged edits; its visible contextual controls/Help remain usable.
   Ctrl+P is consumed/refused there, not also interpreted as upstream's previous-row
   alias. Up remains previous-row; this changed alias is documented in Help.
6. Active broker review/full-load and environment surfaces receive their own
   local grammar plus shared Surface handling. Environment switching cannot bypass
   any existing review/terminal-outcome/full-load ownership restriction.
7. Only without a captured owner does workspace shared navigation/local grammar
   run. A consumed click or Esc never reaches the underlying workspace.

Named app operations recheck phase, busy state, identity, dimensions and current
visible control before side effects. Pointer press may select; release/motion
never repeat activation. Wheel events affect the pointed visible body only.

## Data, endpoints and external effects

No new schema, service endpoint, credential mechanism or provider contract beyond
what upstream already added. Preserve its validated preference format and local
migration. Keep explicit fixture/emulator/Azure mode selection; Azure stays
read-only with pinned connection identity. No cross-tenant expansion, cloud
mutations, MCP changes, deployment or silent credential fallback.

## Validation strategy and release conditions

Before runtime changes, preserve both branch baselines, their binary identities
and representative environment/shared-control flows. Do not treat checks from
either isolated branch as evidence for the combined program.

- Run upstream environment/provider/config tests and actual-binary environment
  PTY before adaptation, then retain their domain assertions afterward. Keep the
  current shared-control/review/reader suites as the other control baseline.
- Adapt PTY input steps only where the approved shared interaction contract
  deliberately changes them: select-first pointer rows, visible explicit
  activation, Tab traversal and safe confirmation defaults. No dropped safety
  assertions or fake synthesized-key mouse paths.
- Cover create across Azure subscriptions, exact group preview, known namespace,
  edit/duplicate/delete active, Apply without Save, failed Save, Save/restart,
  environment-specific All/Selected inclusion and removal/re-add restoration.
- Add paired key/pointer tests for every new control category, including Help
  over dirty drafts, literal q/j/k/s, release ignoring, resize with stale targets,
  busy/late metadata replies after Back, and same-name/different-identity rows.
- Exercise switching during every review/terminal-outcome/full-load ownership
  state, and delayed discovery/count/peek/clipboard/save results. Verify old work
  cannot relabel the new scope or erase an unknown broker outcome.
- Save-and-quit exits only after a successful save of the current revision;
  a failed or older save cannot exit or silently persist the unapplied form.
- Check scoped headers and partial/unavailable/missing states at 160×48, 80×24,
  exactly 60×18 and below minimum. Scope-less compatibility and stricter broker
  review minimum retain their own tests. No hidden Apply/Save/delete operations.
- Root make check, ./build and make test-terminal must include the upstream
  environment PTY at its relocated path, plus both mouse-on and --mouse=false
  environment workflows. Build auxiliary sbx-lab; run all four module checks.
- Inspect actual terminal screenshots and a real-pointer interaction recording;
  inspect persisted private JSON independently after Apply, Save and restart.
  Check truecolor/NO_COLOR and ASCII focus affordances in representative states.
- Azure injected CLI/HTTP tests prove bounded requests and refusal paths, not
  actual RBAC/network/AMQP access. Inspect available tooling before claiming live
  access unavailable. Do not authenticate or mutate Azure to make a claim green.
  Seeded broker integration requires a separately authorized disposable lab.

Success: no lost upstream domain scenarios; every new visible interactive
category has a key route, semantic pointer route and evidence; no duplicate
palette/Help/chrome, no pointer-to-key dispatch, no app-domain effects in lib,
and no known hidden-target or modal-precedence regressions. Coverage counts are
not a substitute for human usability or native-terminal accessibility acceptance.
Only after combined validation should the existing main-push authorization be
used. Fetch again before delivery; never overwrite concurrent work.

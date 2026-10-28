# Architecture: three apps, one small interaction library

Gate 2 APPROVED by Andreas on 2026-10-25. Gate 1 v2 approved on 2026-10-25,
with the added requirement that all three apps have separate directories and
shared code lives under lib. This proposes boundaries, not implementation code.

## Fit

### Repository layout

```text
apps/
  lazyftp/                 # existing root Go app, including lazyftp-mcp
    go.mod, go.sum
    cmd/, internal/, scripts/, lab/, docs/, Makefile, README.md
  service-bus-explorer/    # existing app moved as a unit
    go.mod, go.sum, cmd/, internal/, scripts/, lab/, Makefile, README.md
  log-viewer/              # existing app moved as a unit
    go.mod, go.sum, cmd/, internal/, scripts/, Makefile, README.md
lib/
  terminalui/              # shared Go module; buttons, palette, surfaces, navigation
    go.mod, go.sum
docs/plans/               # repository-wide design/history, unchanged locations
.agents/setup            # repository setup, updated discovery paths
.amp/                    # orb-local portals/artifacts, not app source
go.work                  # four local modules for editing; not required to build
Makefile                 # explicit app/library build/test/check aggregation
README.md                # suite entry points and developer instructions
.go-version, .gitignore  # repository-wide tooling and generated-file exclusions
```

Use `apps/` rather than leaving two apps at root and moving only FTP: all three
then have the same ownership depth. `lib/terminalui` is deliberately concrete,
not a generic `common` package or a home for unrelated backend helpers.
No empty framework subdirectories are needed; Gate 3 defines actual files.

Move tracked FTP-specific cmd/internal, go.mod/sum, scripts/lab and lab assets,
application documentation (alpha/connections/MCP) and its existing README/Makefile
into apps/lazyftp. Keep repository-wide plans, setup and new suite entry points
at root. Both FTP executables remain in that app; creating a fourth app for MCP
is outside scope. Preserve historical plans and screenshots; do not mass-rewrite
old design evidence paths. Update current runnable instructions and links.

### Module and dependency boundary

Retain the three existing module identities: `lazyftp`, `servicebusexplorer`,
`logviewer`. Directory moves then do not require rewriting every internal import.
Add local module `terminalui` at lib/terminalui, using the same toolchain and
compatible Bubble Tea/Lip Gloss/ANSI/fuzzy versions already used by all three.

Each app requires the local library with a relative replacement pointing to
`../../lib/terminalui`. A workspace lists all four modules for editor/local
development convenience. Workspace mode must not conceal missing app requirements:
every module is also checked with `GOWORK=off` from its own root. Full repository
checkout is the supported build unit; app-directory-only exports and remote
`go install` are not newly promised. No registry publication or module merge.

Dependency direction is apps → terminalui → terminal libraries. Shared code
cannot import app packages, storage/Azure/Elastic/MCP clients or app preferences.
App internals stay private; no peer-app imports or root-internal workaround.
The library may expose small reusable test assertions; domain fixtures and async
test drivers remain app-owned. Extract a Python PTY helper only if the integrations
demonstrate useful reuse; do not create another package merely for symmetry.

Why not one root module? It simplifies dependency commands, but would rewrite
existing child imports and combine their independent dependencies/build contexts
without delivering UI value. Why not a public root package? Root should be suite
orchestration, not the former FTP module acting as the other apps' dependency.
The fourth module adds a small amount of explicit build setup in exchange for
enforcing the requested ownership boundary.

### Shared versus app ownership

Shared terminalui owns:
- Action button/bar rendering and keyboard/pointer interpretation, including
  focused/unavailable/busy/danger states and reason text.
- Palette matching, selection, common controls, viewport and capture behavior.
- Surface title/close/actions, inline validation placement, capture and return
  focus mechanics; overlay and docked presentation both supported.
- Visible focus traversal, row select-versus-activate affordances, contextual
  help presentation and semantic component styles/ANSI cell geometry.
- The geometry used by both rendering and hit testing, so they cannot drift.

Each app owns:
- Its Bubble Tea Model/Update, lifecycle messages, domain targets and generations.
- Current authorized/applicable actions and final execution/availability checks.
- Workspace topology, safe dimensions, visible region identities and domain body.
- Theme loading/presets mapped to shared semantic roles, data sanitization,
  clipboard policy, editor/reader submodes, operation status and outcome truth.
- File queues/endpoint/edit lifecycle; broker locks/authorization/send/cleanup;
  log queries/history/redaction/budgets. No shared inspector or operation machine.

Component maintainers own appearance/input contracts and conformance checks;
app maintainers own integration and safety checks. Assign a named maintainer at
implementation planning; no new team or service is implied.

## Endpoints

No new HTTP/API endpoints. Existing FTP MCP tools and all storage/broker/log
provider interfaces retain their behavior. Offline browser previews and design
artifact portals are review tools, not new application architecture.

## Data

No new persistence schema or settings migration. User config locations, CLI
flags, binary names, saved searches and broker preferences remain unchanged.
The repository layout moves source, not users' home-directory data.

Component state is ephemeral: query/selection/focus/scroll/help plus current
presentation and hit regions. Apps supply opaque stable IDs and current safe
display text. Shared components do not persist credentials, retain raw log data,
copy message bytes, acquire locks or dispatch effects.

Ignored runtime state requires care: do not move/delete `.lab`, virtualenvs,
node_modules, binaries or active service state as if they were source. Root and
child ignore rules must cover the new generated locations. Recreate dependencies
at new paths through setup; leave old data/services untouched unless a separately
reviewed action is needed. Changed lab paths/service identities must be reported,
not silently reset/reseeded. User data retention is not a directory-cleanup task.

Concrete relocation hazards: FTP's ownership marker includes its absolute root
(`scripts/lab.py:109–120`); Log Viewer's service name hashes its root and generated
config contains absolute paths (`log-viewer/scripts/lab.py:32–40`); SBX's managed
Compose command embeds its old compose path, while credentials live outside the
checkout. Do not rewrite ownership markers or silently adopt old state. Before
moving source in a live orb, inventory relevant services read-only and arrange an
explicit old-path stop/new-path restart handoff if needed; otherwise stop that
slice rather than leave an orphan process or reset data. Keep existing credentials
and volumes. Source relocation approval alone is not approval to reset/seed labs.

## Flow

1. App processes resize, expiry/completion and other lifecycle messages first.
2. App derives current actions, visible targets, local surface permissions/body
   and semantic styles from its current domain state and app-owned layout.
3. Active captured surface (including local editor/reader grammar) receives input;
   otherwise palette, shared controls, then visible navigation/local keys do.
   A consumed event never reaches the background. App-specific Ctrl+C cleanup
   remains explicit rather than a shared unconditional quit rule.
4. Shared component interprets keyboard/pointer with current geometry and emits
   a semantic select/focus/activate/dismiss/help request, or consumes with none.
   Left press activates once; release/motion do not repeat it.
5. App rechecks target identity/generation, capability, visible operation gates
   and cancellation/authorization, then calls its existing checked domain path.
6. App retains async execution and outcome state; components render the new safe
   presentation. Resize/removal invalidates old regions; no cached-row activation.

Data-row click selects; labelled Open/Inspect or Enter activates. A palette row
is an action, so click requests activation immediately. Destructive palette IDs
open domain review; they are not generic confirm-and-run callbacks. F1 help
preserves drafts/prepared targets. Logs first Esc in INSERT remains local.
Service Bus review remains docked and its compact/expiry refusal still applies.

## Build, relocation and migration boundary

Treat relocation as a behavior-preserving step, verified before interaction
changes. Gate 4 will choose reviewable slices; do not mix mechanical moves with
palette policy changes in the same review unit.

- Root Makefile explicitly visits each module; root `go test ./...` is not a
  substitute for multi-module checks. Root default build should build all three
  app binaries plus FTP MCP; app-specific run/lab targets stay local and explicit.
- Root setup discovers apps/*/go.mod and lib/*/go.mod and app requirements; its
  current root/top-level globs would miss the relocated modules. FTP's current
  setup lab install paths must be updated without starting or seeding labs.
- Keep app scripts with app roots: existing script-relative ROOT calculations,
  binary paths, tests' ../../.lab references and lab resources then move together.
  FTP packaging stays local-only, with its app docs and unchanged binary/archive
  identity. Check each child build without relying on the workspace.
- SBX's sbx-lab module-root detection checks the existing module declaration;
  retaining servicebusexplorer preserves this guard and its refusal test. Include
  this auxiliary command in module build coverage, not just the three TUIs.
- Update maintained README/Makefile/script launch instructions and app-local
  ignore rules. Do not turn historical plan links into a broad documentation
  rewrite or refactor unrelated scripts.
- After relocation is green: visible ActionBar/Palette tracer in FTP+Logs,
  Service Bus+all-app Help, surfaces, then remaining navigation/editor paths.
  Each app integration can be reverted independently of the library boundary.

Validation after implementation approval: all four modules test/vet with
GOWORK=off; build all four executables; run existing offline PTY scenarios from
their app roots with isolated config; validate local packaging and script paths.
Then paired mouse/key scenarios, actual rendered terminal states, compact safety
gates and domain regressions as specified in interaction-design.md. Live labs
are not started/reset/reseeded just to validate source relocation.

## External

No new provider, credentials, environment secrets, cloud resources, publication
or release. Existing setup/lab tooling remains app-specific with pinned versions.
Terminal mouse reporting is enabled in all apps alongside actual routing; native
terminal/tmux/SSH compatibility and mouse-off fallback are validation work, not
a new transport layer. No deployment or production writes are authorized.

## Decisions for Gate 2

Approve apps/{lazyftp,service-bus-explorer,log-viewer}, lib/terminalui, four local
Go modules with explicit app replacements and convenience workspace, and the
ownership/flow above. Gate 3 will define exact files/signatures/test cases;
Gate 4 will define executable slices. Neither gate is pre-approved by this doc.

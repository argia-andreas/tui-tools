# Gate 3 — Program design

Draft for approval. Product and architecture are approved. No implementation code,
lab bootstrap, dependency installation or slice execution is authorized by this
document. The declarations below are proposed contracts, not compiled source.

## Files

All implementation will be confined to the independent `log-viewer/` module.
No FTP/Service Bus edits or root setup rewrites. Add files only as their approved
slices need them, not an empty skeleton for every future provider.

| File(s) | Responsibility |
| --- | --- |
| `go.mod`, `go.sum`, `.gitignore`, `Makefile`, `README.md` | Independent module `logviewer`; local build/test/lab commands; exclude binaries, private state and virtualenv |
| `cmd/logview/main.go` | TUI flags, explicit fixture/live mode, cancellation and private config loading |
| `cmd/logview-mcp/main.go` | Stdio only, explicit policy/config, sanitized stderr; no interactive or HTTP fallback |
| `internal/logs/logs.go` | Event/query/page/provider contracts; no UI, HTTP or MCP dependency |
| `internal/logs/service.go` | Authorized investigation, opaque handles, paging/context/follow session lifecycle and resource limits |
| `internal/logs/output.go` | Redaction, allowlisted output, safe field selection/serialization and text sanitization |
| `internal/config/config.go` | Strict bounded source/preferences JSON, credential references, local paths and atomic explicit saves |
| `internal/access/policy.go` | Strict grant loading, full-grant authorization, expiry, policy revision and field/service constraints |
| `internal/elastic/client.go`, `query.go` | Restricted HTTPS transport, Elastic DSL/PIT/search_after, mapping and normalization |
| `internal/fixture/provider.go`, `events.json` | Deterministic fake and diverse fictional records; clock/delay controls for tests |
| `internal/ui/model.go`, `view.go`, `actions.go`, `inspector.go` | Focus/input ownership, generation-safe commands, log rendering, Ctrl+P and field yank |
| `internal/ui/clipboard.go` | Native clipboard commands with stdin, or explicitly enabled OSC52; receipt semantics and failures |
| `internal/theme/theme.go`, `provenance.go` | Small app-owned semantic Midnight/coding-theme presets using existing provenance |
| `internal/mcp/server.go` | Five bounded read tools delegating only to a policy-configured investigation service |
| `scripts/lab.py`, `scripts/elastic-release.json`, `scripts/compose.yaml` | Pinned native lab lifecycle/preflight, optional equivalent Docker packaging, seeding and ownership-safe cleanup |
| `scripts/terminal-smoke.py`, `scripts/requirements.txt` | Real PTY interaction against fixture or lab using existing pyte workflow |
| Adjacent `*_test.go` files only for public behavior | Config/policy, investigation, Elastic integration, UI transitions, tool outputs and command startup; no test per private helper |

`logview` is a neutral executable name, not approval of SIGNAL as a brand.
Do not create a plugin framework, generic middleware system or fourth shared app
module. The service owns investigation lifecycle; the provider owns native I/O.

## Dependency and local release pins

- Go `1.27.3`, Bubble Tea `v1.3.10`, Lip Gloss `v1.1.0`, `x/ansi v0.10.2`,
  fuzzy `v0.1.1`, MCP Go SDK `v1.8.0`: match current repository dependencies.
- Use Go `net/http`, `encoding/json` (with `UseNumber`), `crypto/rand`, TLS and
  context for the restricted Elastic surface. No full Elastic client dependency
  initially: this app needs a small fixed request set, not administrative APIs.
  Pinning/tests carry the responsibility for API compatibility.
- PTY tooling: `pyte==0.8.2`, already used by adjacent apps. No new browser or
  Testcontainers dependency for the first native-process lab.
- Lab release: **Elasticsearch 9.5.5, Linux x86_64**, official bundled-JDK archive:
  `https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-9.5.5-linux-x86_64.tar.gz`.
  Published SHA-512 retrieved 2026-10-23:
  `36bf5d6897faeda3743841dfaece365330bb41880f907e4c45958964dd2803617d503cba66e99686124899cff944c983b01a2e08eaebd8f54c26db02bf709ced`.
  This verifies the published checksum's availability, NOT downloaded archive
  contents or successful startup. Installation must hash the archive against this
  committed value before extracting; no floating checksum fetched on each run.
- Optional Docker mode uses `docker.elastic.co/elasticsearch/elasticsearch:9.5.5`;
  resolve and commit its image digest before claiming reproducible Docker support.
  Native mode is the first supported lab target; unverified Docker mode must not
  masquerade as a passing required test path.

This pin is a local development target, not a claim of the live deployment's
version, support for all 8.x/9.x deployments, or Serverless compatibility.

## Types and signatures

### Provider contract (`internal/logs`)

Requests contain only validated domain filters. Provider outputs are internal,
untrusted data: neither UI nor MCP gets a provider directly.

```go
type Window struct { Start, End time.Time } // UTC instants; [start, end)
type Limits struct { Events, Bytes int; Timeout time.Duration }
type MessageFilter struct { Mode, Text string; CaseSensitive bool }
type Scope struct {
    Source string
    Targets []string // configured target IDs, NEVER caller-provided URLs/indices
    Services []string
}
type Query struct {
    Scope Scope
    Window Window
    Levels []string // explicit set; Unknown is never INFO
    Instances, TraceIDs []string // exact context/correlation predicates
    Message MessageFilter
    Order string // "newest" or "oldest"; cursor binds ordering too
    Limits Limits
}
type NativeRef struct { Index, ID string }
type Event struct {
    Ref NativeRef // internal only; strip at presentation boundary
    Time time.Time
    IngestedAt *time.Time
    Level, Service, Instance, Message, TraceID, SpanID string
    Fields map[string]json.RawMessage // original object tree, nested types intact
    Incomplete bool
}
type Capabilities struct {
    MessageModes []string
    StableIdentity, History, Context, Suggestions bool
    Follow string // "poll", "stream", "none"; initial real provider: poll
}
type NativePage struct {
    Events []Event
    Cursor string // provider-owned internal continuation, never forwarded
    More, Partial bool
    Warnings []string // codes, not unsanitized backend messages
}
type Values struct { Items []string; More, Partial bool }
type Provider interface {
    Capabilities() Capabilities
    Search(context.Context, Query, string) (NativePage, error)
    Get(context.Context, Scope, NativeRef, Limits) (Event, error)
    Values(context.Context, Query, string, string) (Values, error)
    Close(context.Context, string) error
}
```

`Search` initially pages a fixed-time historical search; follow requests use fresh
bounded searches, not an old PIT. Its cursor stores provider state only inside
the process. Future streaming providers can add an optional subscription contract
when implemented; no fake stream or unused goroutine abstraction now. `Values`
arguments are permitted native field path and keyword, bounded by Query limits.
Configured mapping selects native object paths explicitly (JSON pointers), not
heuristically splitting dots in keys that may themselves contain dots.

### Authorized investigation contract

```go
type AccessRequest struct {
    Operation string
    Query Query
    Fields []string
}
type Permit struct {
    Revision, GrantID string
    Scope Scope
    Fields []string
    Redaction RedactionPolicy
    Limits Limits
    Expires time.Time
}
type RedactionPolicy struct { Paths, Patterns []string }
type Authorizer interface {
    Authorize(context.Context, AccessRequest) (Permit, error)
}
type SafeEvent struct {
    Handle string
    Values map[string]json.RawMessage // permitted normalized fields + native tree
    Incomplete bool
}
type SearchRequest struct { Query Query; Cursor string }
type Page struct {
    Events []SafeEvent
    Next string
    More, Partial bool
    Warnings []string
}
type ContextRequest struct { Event string; Before, After time.Duration; Limits Limits }
type Service struct { /* private source registry, authorizer, clock and sessions */ }
func (s *Service) Sources(context.Context) ([]string, error)
func (s *Service) Search(context.Context, SearchRequest) (Page, error)
func (s *Service) Event(context.Context, string) (SafeEvent, error)
func (s *Service) Context(context.Context, ContextRequest) (Page, error)
func (s *Service) Values(context.Context, Query, string, string) (Values, error)
func (s *Service) Close(context.Context, string) error
func (s *Service) Shutdown(context.Context) error
```

The constructor receives an explicit source registry and an Authorizer; no request
can select a different authorizer. TUI authorizes only explicitly configured
sources, with source output policy. MCP uses the stricter file-backed grant
authorizer. The CLI, not a tool argument, selects the policy file. `Close` releases
only handles owned by this service, not caller-supplied PIT IDs.

SafeEvent's only fields are those permitted after policy. Normalized message,
trace/service IDs and timestamp are also data: a grant denying them must not leak
them through summaries, handle metadata, sorting details or copy feedback. Hidden
fields cannot be used as caller-controlled predicates/suggestions for inference.
Native refs and source endpoints never appear in MCP results or error strings.

Opaque handles: cryptographically random process-local IDs stored with source,
native locator/query, permit revision, grant ID and expiry. No stateless token
cryptography or native locator decoding. Max 16 open cursor sessions, 10,000 event
refs, cursor TTL 2 minutes, event-ref TTL 5 minutes. On expiry/eviction close owned
PIT contexts; expired handles produce an explicit reload-needed response, never
a fresh broader query. Changing policy invalidates affected handles. Service
serializes advancement per cursor; no duplicate page from concurrent use. The
service reuses an event handle for the same source/native identity and permit
revision while retained, so follow deduplication needs no exposed native locator.
TTL refreshes only within bounded capacity; expired/evicted handles require reload.
The UI owns this service for its process; each MCP launch owns its own service.

### Configuration, policy, UI and MCP wiring

```go
// internal/config
func Load(path string) (Config, error)
func SavePreferences(path string, Preferences) error
// internal/access
func NewFileAuthorizer(path string, sources []SourceScope) (*FileAuthorizer, error)
func (a *FileAuthorizer) Authorize(context.Context, logs.AccessRequest) (logs.Permit, error)
// internal/elastic
func New(SourceConfig, *http.Client) (*Client, error)
// internal/ui
func New(Options) Model // service, theme, clock, clipboard; no direct provider
func (m Model) Init() tea.Cmd
func (m Model) Update(tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
// internal/mcp
func New(*logs.Service) *mcp.Server
```

These package-local types' JSON layout is owned by the listed files, not a new
generic schema package. Strict decoding rejects unknown fields, duplicates,
trailing values, invalid references, invalid regexes and oversized configuration.
Source targets map names to operator-configured index/data-stream expressions.
Grants choose target IDs and optional services; tools cannot supply index
expressions. This avoids unsafe ad-hoc wildcard subset calculations. All native
reads still apply the chosen target expressions, service constraints and field
policy; configured alias filters are not a replacement for backend privileges.

One complete grant authorizes an operation; don't union grants. Fields are
allowlisted, redaction only tightens the result, and bounds/expiry can only narrow
source policy. Empty or unreadable policy denies even source listing. Policy is
revalidated before access and immediately before output; observed revocation
cancels/discards output, but cannot recall returned data. Use a content revision,
not just file modification time. No claim of race-free revocation against an
operator modifying policy after the final check.

## Call stack

- **Search:** input/editor → Service.Search → Authorizer → freeze query + scope →
  provider.Search → byte-bounded decode → redaction/allowlist → handle registration
  → final authorization → Page → current-generation UI or bounded MCP result.
- **Inspector/yank:** selected SafeEvent → native JSON-pointer field traversal →
  full available policy-safe value → clipboard command stdin / opted-in OSC52 →
  honest success/sent/failure feedback. Never concatenate log values into a shell.
- **Context:** handle lookup → reauthorize original scope → provider.Get anchor →
  new bounded query, same permitted source/service/instance, message+level relaxed
  → separate safe Page. Missing instance scope produces a warning. Trace action
  uses a distinct bounded exact trace predicate, still within the permit.
- **Follow:** TUI follow state → fresh bounded Search → stable-ID merge with overlap
  → bounded ring → one render batch; pause cancels active poll/tick; resume catches
  up within limits. Timer belongs to UI/session, not the Elastic HTTP client.
- **MCP:** strict SDK tool arguments → service with FileAuthorizer → same safe path.
  Tools: `list_log_sources`, `search_logs`, `get_log_event`, `get_log_context`,
  `suggest_log_values`. Search uses structured filters only; event/context use
  opaque handles only. No follow, clipboard, native query or transport proxy tool.

## Resource, search and UI decisions

- Canonical time ranges are half-open UTC instants; absolute local inputs require
  explicit offset for ambiguous DST times. Missing level becomes Unknown. Unknown
  is selectable, but not part of WARN+. Multiple selected apps use OR; facets AND.
- History display is ascending time within a bounded newest-window fetch; older
  paging loads earlier events without reordering the anchored selection. Provider
  cursor order is explicit and stable (timestamp + PIT tie-breaker), not relevance.
  Context has its own page/session; it never overwrites the original results.
- Backend message mode initially Elastic analyzed phrase, explicitly labeled;
  unsupported literal/case semantics fail visibly. `/` is exact local literal
  find in loaded redacted data with an explicit case toggle. Never silently scan
  the entire source to simulate unsupported search.
- Elastic HTTP body limit 8 MiB; exceedance is a bounded failure, not parsing a
  truncated JSON body as success. Per-event 1 MiB and session 32 MiB/10,000 events
  remain separate ceilings. No automatic decompression of unbounded payloads;
  bound decoded bytes. Partial/timeout/shard failures remain visible.
- MCP defaults: 100 events / 256 KiB / 15-minute span / 10 seconds. Absolute limits
  remain 500 events per call, 1-hour MCP span, 30-second request deadline and bounded
  serialized output even for larger grants. Retrying/paging cannot remove policy
  restrictions; these are per-request limits, not a total exfiltration quota.
- Focus order: sidebar → log list → inspector, visible panels only. Text editors
  own all typing except explicit submit/cancel. Ctrl+P opens fuzzy actions; Enter
  runs the selected supported action, Esc returns to the previous focus. An
  unavailable action explains why. `Space` toggles follow only from the log list.
- `j/k`, arrows and page keys navigate; inspector Enter expands; `y` value and `Y`
  key=value (objects use stable JSON); `f` filters; `/` find; `c` context from list;
  `?` help. `q` quits outside text input and overlays; Ctrl+C cancels/quits cleanly.
- Start three panels at 160×48; collapse inspector/sidebar as needed below 120
  columns, using a full-width detail screen instead. Below 60×16 show a resize
  prompt with help/quit. No focus trap when a panel auto-hides.
- Native clipboard commands are detected explicitly (macOS pbcopy, Wayland
  wl-copy, X11 xclip), with context timeout and stdin. If unavailable and OSC52 is
  not explicitly trusted, copy is unavailable with an explanation—not silently
  written to disk. OSC52 reports transmission, not verified clipboard receipt.

## Local lab mechanics

Proposed Makefile targets: `test`, `check`, `test-terminal`, `lab-install`, `lab-up`,
`lab-seed`, `lab-check`, `lab-down`, `test-integration`, `run-lab`. Normal tests need
no node. No destructive reset by default. All runtime state under ignored
`log-viewer/.lab/`; ownership marker, private permissions and strict loopback
endpoint checks precede any bootstrap/cleanup. Lab tools refuse arbitrary live
URLs. Use unique owned index prefixes; stop only the named lab service/process.

Orb preflight observed 2026-10-23: Linux x86_64, uid 1000, vm.max_map_count 65530,
soft file-descriptor limit 1024. Native lab will set `node.store.allow_mmap=false`,
loopback HTTP/transport, `discovery.type=single-node`, `xpack.license.self_generated.type=basic`,
disable ML, and start with 512 MiB heap for small fixtures. Attempt an appropriate
file-descriptor limit only within the process's allowed hard limit; otherwise
report the constraint. Do not run privileged sysctl or silently ignore startup
failure. These are lab tradeoffs, not production or performance-test defaults.

Use bundled `elasticsearch-certutil` to create a private lab CA and server cert
with loopback SANs. Bootstrap password/keys are generated locally and stored only
in private lab state. The viewer trusts the CA and receives a read-only API key
for the owned log target; provisioning credentials never enter its source config.
No public portal. The service is supervised with `amp orb service` in orbs, not
nohup/tmux; Docker packaging uses an equivalent isolated lifecycle on Docker hosts.

Fixture layout: two apps with asymmetric data, two instances per app, exact
keyword app/level/trace fields, text message and nested structured fields. Fixed
historical fixture time is explicit; live demo seeding may anchor to current time
but tests use a controlled origin. Include >500 equal-time events with unique
IDs to cross a real paging boundary, duplicate messages with distinct IDs, UTF-8,
multiline exception, missing level, huge field and secret canaries. A separate
forbidden target is never granted to the viewer. A timed append/refresh scenario
exercises late visibility; source fixtures alone aren't a live-tail test.

## Test plan

Keep behavior tests that catch plausible regressions; don't freeze private call
order, exact boxes or incidental rendering spacing.

| Test scenario | Observable assertion / competing mistake |
| --- | --- |
| Two apps, same text; two selected apps plus WARN+ | App OR / facet AND; substring in message doesn't select a service; Unknown excluded |
| Event exactly at start/end and Stockholm DST overlap | Start included, end excluded; ambiguous offset-free input rejected |
| 501+ equal-time events in real Elastic | Every unique ID returned once across pages; timestamp-only cursor would fail |
| PIT history while a new event is indexed | History remains stable, follow sees new event; frozen PIT cannot fake live tail |
| Original filtered error surrounded by INFO, another instance and another app | Context relaxes text/level only; foreign instance/app excluded; original filter remains |
| Two identical messages with different IDs; late event after watermark | Both legitimate events survive, refetch duplicates collapse; late/gap status honest |
| Policy absent/invalid/expired; cursor/handle from another source/grant | Denied without log data or unauthorized source discovery; no scope escalation |
| Revoke grant or alter allowed fields between pages | Old handle can't leak old permissions; normalized and native fields obey new policy |
| Secret canaries in nested arrays, message and stack | Absent from UI, clipboard capture, MCP output and diagnostics; raw view doesn't bypass |
| Field denied but requested in suggestions/filter; backend error includes secret | No field inference/unsafe backend details exposed |
| OSC52/ANSI and Unicode controls embedded in log strings | Rendering cannot set clipboard, cursor, hyperlink or reorder labels |
| Long field visually ellipsized vs field actually incomplete | Yank full safe value for the first; refuse completeness claim for the second |
| Huge HTTP reply, event, page and MCP result; 429/partial shards/timeouts | Bounded failure/truncation, no invalid-JSON success, no fake complete count |
| Rapid query changes, poll pause, resize with inspector focus | Stale results never replace current; no background collection after pause/focus trap |
| PTY keyboard sequence, real fixture and real lab | Filter → inspect → y/Y emission → context → Ctrl+P → pause/resume → clean exit |
| Authenticated lab under read-only key | Queries work; forbidden target and attempted index write are rejected |
| Closed/expired cursor and process shutdown | Owned read contexts released or expire; no broader fallback request |

Fuzz strict JSON/query-input and redaction/terminal text boundaries with seeded
adversarial strings and invariants (no added authority, no canary/control leakage,
bounded processing). That supplements, not replaces, asymmetric expected-value
cases and real Elastic API tests. Actual clipboard receipt is checked separately
where supported; terminal escape emission alone is not proof of a system copy.

## Least confident decisions

1. **Live source/version remains unknown.** 9.5.5 is a concrete local target only.
   Confirm the sanitized product/hostname before claiming production compatibility.
2. **512 MiB heap / small native node:** needs an actual startup/resource probe in
   an approved implementation slice. No-mmap reduces host requirements but changes
   performance. Fall back by revising the lab budget, not by faking integration.
3. **Read request/response limits:** proposed numbers prioritize safe terminal/agent
   use; realistic logs may require adjustment with measured evidence.
4. **Provider abstraction:** validate it with a second real backend later; avoid
   speculative signatures for streaming until Kubernetes/Loki is implemented.
5. **Local copy:** OSC52 needs explicit trust and may not work in every SSH/tmux
   setup. Reliable receipt, not just a “Yanked” toast, needs platform validation.

Approve Gate 3, or what should change?

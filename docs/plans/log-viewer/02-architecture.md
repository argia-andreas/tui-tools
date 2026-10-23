# Gate 2 — Log viewer architecture

Draft for Andreas's approval. Gate 1 is approved; Gate 2 is not. No implementation
or lab has been started. This document chooses boundaries and flows, not the
file/type signatures or slice sequence reserved for Gates 3 and 4.

## Fit

An independent Go application/module under `log-viewer/`, using Bubble Tea,
Lip Gloss and the MCP Go SDK already used in this repository. Separate TUI and
stdio MCP entry points share the log investigation layer. The final executable
and product names remain undecided; SIGNAL is only the visual placeholder.

Do not import FTP storage, its transfer-oriented MCP policy, or Service Bus
internals. Preserve Midnight semantic colors and keyboard conventions in a small
app-owned theme, with provenance. Do not extract a shared application framework
or modify the other applications. The existing setup already discovers immediate
child Go modules and terminal-test requirements; no unrelated setup rewrite is
necessary. Local lab lifecycle and test commands belong to this app.

Initial provider: **Elasticsearch plus a deterministic fake**, as the local test
target Andreas requested. This does not confirm the company's live backend.
Azure Monitor, Kubernetes and Loki are future adapters, not simultaneous initial
implementations. The boundary must accommodate them without claiming shared
query syntax, guaranteed event identity, retention or streaming behavior.

## Provider and investigation boundaries

The provider owns native I/O, query translation, paging, capabilities and event
identity. It exposes bounded search, scoped suggestions, event retrieval/context,
and follow where available. A small common request describes source, exact apps,
time, levels, message search mode and limits. Native query editors, plugin loading
and cross-provider result merging are outside the initial architecture.

The investigation layer owns source selection, immutable query scope, budgets,
context expansion, normalization, policy-safe output and session cancellation.
It is the only backend access path for both TUI and MCP. MCP adds explicit grant
authorization before entering it; working TUI credentials are not MCP permission.

Capabilities state whether an operation is server-side, local-only or unavailable:
message phrase/literal search, structured fields, historical paging, context,
stable identity, and streaming versus polling follow. Unsupported is an explicit
result, not an empty successful page. Never implement a fake universal KQL.

## Endpoints

No app HTTP listener, hosted service, browser-session scraping or remote MCP
transport. MCP uses local stdio only; diagnostics must stay off protocol stdout.

| Outbound operation | Elastic surface | Constraint |
| --- | --- | --- |
| Start historical snapshot | `POST /{configured-indices}/_pit` | Short-lived read context, scoped to configured source |
| Search / next page | `POST /_search` with PIT; `search_after` | Fixed query and ordering, include tie-breaker and latest PIT ID |
| Close snapshot | `DELETE /_pit` | Release only this session's read context; not log deletion |
| Follow poll | `POST /{configured-indices}/_search` | Fresh bounded searches; never reuse frozen PIT as a live subscription |
| Details / context / trace | Scoped searches over configured source | Reapply security and source scope; event IDs cannot broaden it |
| App/field suggestions | Bounded search/aggregations | Configured mapped fields only; no cluster-wide discovery |

The provider may need `view_index_metadata` if later field discovery is enabled;
the initial explicit field mapping avoids making that an unconditional requirement.
HTTP method alone does not determine whether an operation writes log data. The
viewer must not expose arbitrary routes, even when a credential is overprivileged.

Proposed MCP tools: list granted sources, search logs, get an event, get context,
and suggest permitted field values. Tool naming/signatures wait for Gate 3. No
native-query tunnel, arbitrary URLs, bulk export, unbounded tail, clipboard, shell,
index creation, ingestion, retention, key management, delete or administration.

## Data

No application database and no durable log cache initially. Persist only explicit
source configuration, UI preferences and user-saved searches. Saved search text
can itself be sensitive: save only on user action, in user-private files, never
automatically as a history. No event bodies or credentials in tracked configuration.

- Source configuration names a provider, fixed endpoint, permitted index/data
  stream scope, field mapping, credential reference and trusted CA as needed.
  No ambient discovery of all cloud accounts, kube contexts or data sources.
- A normalized event preserves source/native locator, timestamps, known or Unknown
  severity, service/instance, message, correlation IDs and typed structured fields.
  Keep absent distinct from null/empty; retain native paths and arrays.
- ECS is the starting Elastic mapping, not a requirement on every deployment.
  Source mappings are validated, and missing critical fields produce an actionable
  configuration error rather than an invented service or timestamp.
- Historical pages carry completeness/warnings and opaque continuation bound to
  source + query + scope. Equal timestamps require a real tie-breaker. A follow
  watermark is separate from a historical cursor.
- Session state holds bounded redacted events, anchor and field focus. Starting
  ceilings: 500 events per fetch, 10,000 events / 32 MiB retained, 1 MiB per event;
  first limit wins and overflow is labeled. Validate numbers later, not benchmarks.
  Apply limits while reading/decoding, not after buffering an unlimited response.
- A size-limited event is explicitly incomplete. `y` copies the entire selected
  policy-safe field only if available; it must never claim that a truncated prefix
  is the complete value. Copy failure is visible, not a false success toast.

MCP has tighter default budgets: 100 events, 256 KiB serialized output, 15-minute
time span and 10-second request deadline unless a complete explicit grant permits
otherwise. Reject an over-budget request before contacting the backend; disclose
response truncation and offer bounded paging. Exact defaults remain reviewable
at Gate 3. A grant cannot remove the app's absolute resource ceilings.

## Flow

### Search and inspect

1. TUI validates source/filter input; MCP first checks one complete operator grant.
2. Freeze absolute time bounds for this request. Typed filters become provider
   queries with correctly encoded values, not concatenated native syntax.
3. Provider returns bounded events plus provenance, completeness and continuation.
4. Apply redaction and field visibility policy before session retention, rendering,
   local find, clipboard or MCP serialization. Sensitive raw response memory is
   transient; no debug logging or disk spill of payloads.
5. TUI accepts only the current request generation. Render visible rows only;
   selection stays tied to event identity, not whichever row occupies its old index.
6. Inspector preserves typed fields and multiline content. Only the active panel
   owns bright selection. `y` / `Y` operate on the selected field after policy;
   transport uses a native clipboard or explicitly trusted OSC52, with honest
   “copy sent” versus confirmed-success feedback.

Escape terminal control sequences at rendering; preserve safe text/newlines for
intentional copy. Raw view means redacted structure, never a policy bypass. Treat
all message text and field values as data, including prompt-injection strings.

### Follow, pause and context

Elastic follows with fresh searches, initially around every two seconds, with
overlap and stable-identity deduplication; back off on throttling. No guaranteed
lossless tail: index delay can exceed overlap. Show last refresh, late arrivals,
partial failures and uncertain gaps. Stop collection on pause; resume performs
bounded catch-up, reporting any incomplete interval. Cancel obsolete requests
and release their read contexts on query/source change or shutdown.

Context is a separate bounded request anchored on an event, normally +/-30s and
same service + instance. It may visibly relax message/level filters, never tenant,
source or mandatory security scope. Missing instance data yields a visible weaker
scope, not an implied same-request guarantee. Follow-trace is a separate action
that can span services only within explicit authorization. Return to the original
results without rewriting their filters or replacing their cursor.

### MCP authorization and secrets

An operator-owned, strict local policy grants operations on specific sources and
indices/tables or future namespace scopes, optional service/field restrictions,
limits and expiry. A request must fit **one complete grant**; do not combine weak
pieces from different grants. Initial policy is empty/default deny, including
source discovery. The local launch configuration selects policy/credential refs;
tool arguments cannot select credentials, endpoints, tenant headers or policies.

Use one stdio process per client configuration as the local trust boundary, not
the model's claimed identity. Read/revalidate policy for each request and page;
invalid/missing policy denies access. Serialize a response only if its grant is
still valid; cancel work if revocation is observed. Revocation cannot recall data
already returned. Expire/bind event locators and cursors to authorized source and
scope, rechecking them rather than trusting IDs supplied by the client.

MCP output fields are explicitly allowlisted by grant; redaction alone is not an
allowlist. Use the restrictive combination of source policy and grant policy.
Sanitize backend errors, bound suggestions as well as logs, and audit only source,
operation, grant and outcome—not payloads, tokens or sensitive query strings.
Explain that permitted results leave the terminal and reach the MCP client/model.

For initial Elastic auth use a source-specific read-only data API key referenced
through an explicitly named environment variable or private credential file.
Never pass key values in command arguments or store them in committed files.
TLS verification and private CA support remain on; refuse credential forwarding
through redirects. An OS keychain integration is optional later, not a new initial
dependency. Startup and diagnostics must not echo key values or response bodies.

## Local development and verification

Two loops exercise the same investigation contract:

- **Fake provider:** deterministic seeded events, controllable clock and deliberate
  failures for instant UI/policy tests. It makes no Elasticsearch compatibility
  claim and needs neither a service nor Docker.
- **Real lab:** official pinned single-node Elasticsearch, no Kibana or Logstash.
  Prefer the standalone Linux archive with bundled JDK in orbs; use the equivalent
  pinned Docker image on developer/CI hosts with a daemon. Same mappings, fixtures
  and tests regardless of packaging. No privileged Docker setup in an orb.

Gate 3 must fix a supported release and verified checksum/image digest, after
checking actual deployment version if available. Until then, the live version
is unresolved. The native process requires non-root execution, filesystem/kernel
and memory checks before it is declared supported. Manage it with the orb's
supervised service lifecycle, private loopback listener and private app-owned data
directory; no public portal. Use a Basic license configuration, not trial-only
features. Validate the proposed ~1 GiB small-fixture budget, don't assume it.

Lab provisioning/seeding is deliberately separate from viewer/MCP capabilities:
the fixture tool alone gets lab-owned write/bootstrap credentials. Viewer/MCP get
read-only keys restricted to fixture indices; a forbidden index proves denial.
Use authenticated TLS for integration/security coverage, not only a security-off
demo. Seed unique event IDs and mappings; wait for search visibility explicitly.
Keep a warm node for interactive iteration and fresh owned indices per test run.
Cleanup touches only owned disposable resources, never configured live sources.

Integration checks must cover real query semantics, equal-timestamp pagination,
context isolation, delayed visibility, redaction and permission failures. UI tests
cover resize/focus/yank/pause with a fake, then an actual PTY proves terminal
interaction against the real lab. MCP tests verify denial before backend access,
scope-bound locators/cursors, grant revocation and byte-limited redacted output.
Clipboard-dependent checks distinguish OSC52 emission from clipboard receipt.
Local success does not prove Azure networking/SSO, Serverless or multi-node behavior.

## External

- Initial runtime: Elasticsearch data API only, over configured HTTPS. No Kibana,
  Elastic Cloud control-plane API, Azure ARM, telemetry service or remote MCP host.
- Libraries follow existing Go/Bubble Tea/Lip Gloss/MCP SDK conventions; exact
  versions and HTTP-client choice belong to Gate 3, not an unreviewed dependency
  addition now. The provider owns the restricted request surface either way.
- Local lab download: official Elastic archive/image pinned by version and digest;
  no floating `curl | sh`. The lab isn't automatically started on ordinary tests.
- Credential variable names are source-config references, never user-supplied MCP
  arguments. Generated lab credentials stay private and separate from real ones.
- Future Azure Monitor uses Entra + Kusto; Loki uses its configured tenant/auth +
  LogQL; Kubernetes uses explicit kube context + pod-log RBAC. Their SDKs, streaming
  machinery and identities are deferred until their adapters are actually scoped.

## Approval and remaining uncertainty

Approve the independent app boundary, initial Elastic/fake providers, shared
policy-safe investigation path, stdio/default-deny MCP and two-loop local lab.
The production provider/version still needs identification; that does not block
designing or testing a clearly labeled local Elastic target. If the actual source
is Azure Monitor, revisit adapter priority before claiming live readiness.

Approve Gate 2, or what should change?

# Provider research and implementation possibilities

Research checked 2026-10-23. These are feasibility notes requested during product
exploration, NOT Gate 2 architecture approval, a program design, or a slice plan.
No private endpoint, credential, sample schema or live source has been inspected.

## Identify the actual data source before choosing an integration

ELK traditionally means Elasticsearch + Logstash + Kibana; logs can reach
Elasticsearch through other ingestion tools too. Azure is a hosting/integration
environment, not a unique log protocol. A hostname resembling `logs.insights…`
could be custom DNS; the word “Insights” also occurs in Azure and AWS products.
Do not infer the backend or authentication from that fragment.

Ask for a sanitized hostname or product heading, then establish the data-plane
endpoint, source scope, schema, deployment/version and required network access.
Browser SSO is not evidence that a terminal can reuse the browser session. Private
Link, corporate VPN or DNS requirements might require a local runner rather than
an orb. Never ask for a token or complete signed URL in chat.

## Actual providers, not interchangeable labels

| Source | Read surface and query | Authentication and authority | Consequences |
| --- | --- | --- | --- |
| Elasticsearch, including Elastic Cloud on Azure / Azure Native Elastic | Elasticsearch deployment/project endpoint; `POST /{allowed-index}/_search`, JSON Query DSL; Elasticsearch search results, not Kibana HTML | Scoped Elasticsearch data API key; Hosted and Serverless key models differ. Entra/SAML browser SSO and Azure ARM roles do not automatically authorize search. `read` on allowed indices/data streams; add metadata privilege only if needed for field discovery | ECS fields are a useful default, not guaranteed. Kibana saved views are not automatically imported. Azure's Elastic management API is not the log-search API |
| Azure Monitor / Log Analytics; workspace Application Insights | `POST https://api.loganalytics.azure.com/v1/workspaces/{workspaceId}/query`, Kusto Query Language, columns + row arrays | Microsoft Entra bearer token for Logs query API, not ARM audience. Appropriate workspace/table/resource data access. Use approved interactive/developer credential; managed identity where actually available | `AppTraces`, `AppExceptions`, `ContainerLogV2`, custom tables differ. Select a known table/schema first, not a subscription-wide union |
| Kubernetes pod/container logs | `GET /api/v1/namespaces/{namespace}/pods/{name}/log`; `follow`, `previous`, `sinceTime`, `tailLines`, `timestamps`, byte limits | Explicit kube context; namespace-scoped `get` on `pods/log`, and pod `get/list` (watch only if used) for discovery. Never secrets/exec/admin | Real stream, finite retained container logs, no historical search engine, no server message/level filter. Container selection, pod restarts, rotation and disconnected catch-up are first-class limits |
| Grafana Loki | `/loki/api/v1/query_range`, LogQL; `/loki/api/v1/tail` WebSocket; label metadata endpoints | Grafana Cloud instance/user + access-policy token with `logs:read`; self-hosted auth at trusted gateway. Tenant selection is constrained configuration, not user-provided impersonation | Grafana itself is normally the frontend to a data source, not the storage provider. Prefer direct Loki API over scraping Explore or assuming a Grafana login grants backend access |

CloudWatch Logs Insights would be another distinct AWS provider if that turns out
to be the source; it is not implied or included just because of “Insights”.

### Two unrelated meanings of KQL

- Azure KQL means **Kusto Query Language**: pipeline expressions over tables.
- Elastic KQL means **Kibana Query Language**: field-oriented filtering; not Kusto.
  ES|QL and Lucene query-string syntax are additional, different choices.
- New Elasticsearch versions can accept a `kql` Query DSL clause (GA from 9.1;
  preview 9.0, available Serverless). Do not make the outdated blanket claim that
  Elastic KQL can never be sent to Elasticsearch; do not assume it on an unknown
  older deployment either. Query DSL is the conservative initial choice.

## Recommended boundary, not an architecture commitment

Use Go + Bubble Tea/Lip Gloss to match the repository's established skills and
terminal behavior. Existing root and Service Bus apps already use these. Build an
independent app with a small provider boundary, not a third mode bolted onto FTP
and not a premature shared application framework. Reuse semantic theme conventions
without changing either existing app. Exact module/files wait for Gate 2/3.

The boundary should normalize events and expose investigation capabilities:
bounded search, field/application suggestions, event context and follow. Every
operation carries an explicit source, security scope, time/budget and cancellation.
Results include completeness, warnings, provider provenance, native locator and
continuation where supported. Keep native structured fields without flattening
away arrays, types or nulls. Useful common fields: event/ingestion timestamps,
severity (including Unknown), service, instance, message, trace/span IDs and raw
redacted payload. Native cursors stay opaque and tied to the query/source.

Capabilities matter: server text modes versus local filtering, history retention,
stable identity, context scope, structured fields, and stream versus polling.
Absent capabilities must be visible, not fake empty successful responses. Begin
with the real backend and a deterministic fake for exercises; add a second real
backend before declaring the abstraction proven. Not a dynamic plugin loader.

### Field mappings are source configuration

| Meaning | Typical Elastic ECS | Azure AppTraces |
| --- | --- | --- |
| Event time | `@timestamp` | `TimeGenerated` |
| App/service | `service.name` | `AppRoleName` |
| Instance | `service.node.name`, container/pod fields as configured | `AppRoleInstance` |
| Message | `message` | `Message` |
| Level | `log.level` | `SeverityLevel` (0 verbose, 1 information, 2 warning, 3 error, 4 critical) |
| Correlation | `trace.id`, `span.id` | `OperationId`, `ParentId`; inspect instrumentation before equating parent to span |
| Exception | `error.*`, source-specific stack fields | Usually `AppExceptions`, not automatically embedded in every AppTraces row |

Kubernetes usually supplies time/line/container identity; parse JSON only when it
really is JSON and label inferred levels. Do not present stderr as ERROR by fiat.
Loki stream labels and parsed fields supply app/level differently per deployment.

## Search semantics and correctness

- App keyword narrows bounded app-name suggestions. Selection adds exact service
  filters; optionally selecting several uses OR inside the app facet, AND with
  other facets. Unknown schemas can use explicitly configured app names instead
  of requiring broad metadata privileges. Suggestions are limited to authorized
  sources, current scope/time and budgets; not a claim of exhaustive discovery.
- Backend message phrase search is labeled as such: Elastic `match_phrase` is
  analyzer-dependent, NOT literal substring search. Azure `contains` is substring,
  whereas `has` is term-based. Loki `|=` is a literal line filter. No false shared
  semantics. A server-side literal mode is available only when the source's field
  mapping/capabilities support it efficiently; never replace it silently with
  phrase search or an unbounded client scan.
- Local literal find over already loaded/redacted text is always predictable;
  clearly show “loaded events only”. Regex is an opt-in later capability with
  explicit scope and resource limits, not the ordinary string-search default.
- Quote/escape typed filters using provider builders/parameters, never raw string
  concatenation. Native query editing can come later as provider-specific expert
  mode; it must not bypass source permissions or expand MCP reach.
- Context anchors on native event identity and bounded time. Preserve source,
  tenant, resource/namespace, app and instance constraints. Only relax message
  and level visibly. If identity/instance is missing, disclose the weaker context
  rather than imply adjacent rows are the same request. Trace search is separate.

## Follow, paging and performance

- Elastic historical browsing: PIT + `search_after` with timestamp and stable
  tie-breaker; keep newest PIT ID, expire/close promptly. Do not use deep `from`
  offsets, a fixed 10,000-hit ceiling disguised as complete data, or scroll for
  interactive browsing. A PIT freezes history; it cannot itself reveal new events.
- Elastic follow: fresh bounded searches with overlap and stable-ID deduplication.
  Delayed indexing/event timestamps can exceed any chosen overlap. Label late
  arrivals and uncertain gaps; no “lossless real-time” claim.
- Azure: bounded table/time/projection and explicit result cap. No assumed generic
  Elasticsearch-like cursor. Time slicing and tie handling need source-specific
  validation; timestamp-only boundaries miss equal-time events. Without a stable
  row identity, disclose deduplication limits and preserve legitimate duplicates.
  Handle partial results and throttling, not just HTTP success. Ingestion-time
  queries may improve delayed-event handling where supported, with a wider event
  time bound; they do not create a guaranteed lossless subscription.
- Kubernetes: cap stream concurrency per selected pods/containers. Reconnect,
  previous-container reads and retained-history boundaries remain visible. Loki
  tail's initial lookback is best-effort, not complete history; historical reads
  use bounded `query_range` with boundary handling, not one tail connection.
- All: virtualize terminal rendering; bounded event/byte ring and bounded field
  decoding. Proposed starting budgets to validate later: 500-event fetch pages,
  10,000 events / 32 MiB session buffer, 1 MiB per displayed event, 2s initial
  polling interval with adaptive backoff. Stop at the first bound, label truncation,
  and do not drop data silently. These are design starting points, not benchmarks
  or hard promises for every provider.
- Cancel obsolete searches, ignore stale generations, project only needed fields,
  and batch terminal updates. Detail payloads load on demand with size limits.
  Missing fields, 401/403, zero results, unavailable metadata, partial results,
  429/retry-after and network loss must remain distinct states. Azure protected
  tables can return no rows rather than 403, so an empty result cannot prove the
  caller has full visibility.

## Redaction, terminal safety and clipboard

Redact configured field paths and recognized sensitive patterns before rendering,
clipboard, export, MCP serialization and diagnostics. Include nested fields,
unstructured messages and stack traces. A raw view is raw structure after policy,
not a bypass; no unredact command proposed. Redaction is defense in depth, not a
guarantee that arbitrary free text contains no secrets. Prefer source-side field
and document access restrictions when available.

Never persist event bodies/search strings by default, including debug logs,
crash traces or telemetry. Credential references point to OS secret storage or
approved environment/identity sources; no secrets in flags, tracked config, URLs
or chat. Verify TLS; scope endpoint configuration and redirects so credentials
cannot be forwarded to arbitrary hosts. Never downgrade transport for convenience.

Log text is untrusted: escape terminal control sequences (including OSC clipboard
and hyperlink controls), misleading control characters and rendering hazards.
Clipboard comes only from deliberate `y`/`Y`, subject to redaction. Prefer native
clipboard where available; trusted/opt-in OSC52 can support SSH/tmux but terminal
support and acknowledgement vary. If only transmission is known, say “copy sent”,
not verified clipboard success. No automatic clipboard fallback to temp files.

## MCP: a read-only product surface, not a general query tunnel

Default deny even if the TUI has working credentials. Prefer a local stdio server
as an initial option; remote transport would need a separately reviewed identity,
authorization and deployment design. No localhost HTTP server with assumed trust.

Proposed grant: named client configuration, allowed source IDs, indices/tables or
namespace/container scope, optional service/field restrictions, permitted read
operations, maximum time span, event/byte limits and expiry. Initial client
identity assumptions need Gate 2 review: stdio ownership is not proof of a model's
identity. User-controlled local policy, not a tool argument, grants permissions.
TUI and MCP use the same source/redaction/capability logic; MCP adds stricter grants.

Candidate tools: list explicitly granted sources, bounded structured log search,
get a permitted event, get scoped context, and bounded field suggestions. No raw
query/HTTP proxy, arbitrary endpoint, arbitrary field/index discovery, saved-query
execution with hidden scope, unbounded tail, shell/kubectl tool, write/delete,
ingest, key creation, retention change or administration. Backend read-only roles
are a second independent control, not a substitute for tool authorization.

Every tool checks scope before contacting a backend, including context, native
locators and continuation tokens. Bind locators/cursors to source, granted scope
and query; do not accept a caller's alternate tenant header. Re-check grants on
every page and after revocation. Negative tests must cover cross-source IDs,
field leakage, rewritten cursors, scope-broadening context and native query escape.

Return redacted, allowlisted fields with provenance, truncation and scope metadata;
cap bytes as well as row count. Summaries/aggregations may also be sensitive.
Never leak credentials, headers, private source lists or raw backend errors.
Treat log strings as untrusted data, never assistant instructions; annotations and
tool descriptions should make prompt-injection risks explicit. A permitted read
still transfers sensitive information to the MCP client/model provider. The grant
UI must state that boundary clearly. Audit client/grant/source/outcome without
event bodies, tokens or sensitive query text. Logs are not automatically AI-safe.

## Checks to require at later gates (not tests claimed today)

Exercise equal timestamps across pages, delayed arrival, legitimate repeated
lines, trace IDs across services, missing fields/levels, multiline and Unicode,
huge events, terminal escape injection, redaction inside free text, copy of a
truncated field, clipboard denial, rapid query changes, pause/resume gaps, backend
partials and permission revocation. Use real read-only provider sandboxes after
authorization. Measure keyboard tasks and rendering latency; do not infer runtime
quality from Painter images.

## Official sources

- [Elastic on Azure Native Service](https://www.elastic.co/docs/deploy-manage/deploy/elastic-cloud/azure-native-isv-service): data endpoints versus management, Azure/Elastic access distinction.
- [Elasticsearch search API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-search): `read` index privilege, Query DSL search.
- [Elasticsearch API keys](https://www.elastic.co/docs/deploy-manage/api-keys/elasticsearch-api-keys): scoped data credentials and Serverless differences.
- [Elastic KQL clause](https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-kql-query): version-dependent direct support.
- [Elasticsearch pagination](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/paginate-search-results): PIT, tie-breakers and search_after.
- [Azure Logs query API](https://learn.microsoft.com/en-us/azure/azure-monitor/logs/api/overview) and [authentication](https://learn.microsoft.com/en-us/azure/azure-monitor/logs/api/access-api): workspace endpoint, Entra tokens, tabular results.
- [Azure log access](https://learn.microsoft.com/en-us/azure/azure-monitor/logs/manage-access): workspace/resource/table/row access; least-privilege roles and protected-table caveats depend on workspace mode.
- [AppTraces schema](https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/apptraces) and [telemetry model](https://learn.microsoft.com/en-us/azure/azure-monitor/app/data-model-complete): tables and field differences.
- [Azure ingestion time](https://learn.microsoft.com/en-us/azure/azure-monitor/logs/data-ingestion-time): variable latency and event/ingestion-time distinction.
- [Kubernetes Pod API](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/) and [RBAC subresources](https://kubernetes.io/docs/reference/access-authn-authz/rbac/): pod-log stream and scoped read permissions.
- [Loki HTTP API](https://grafana.com/docs/loki/latest/reference/loki-http-api/): LogQL reads, best-effort WebSocket tail, authentication and tenant boundaries.

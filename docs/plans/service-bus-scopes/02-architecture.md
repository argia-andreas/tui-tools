# Architecture: Saved Service Bus environments

Authorized by the user's instruction to proceed without further gate pauses, 2026-10-24. Not shipping authorization.

## Fit

Keep the existing Store for counts, peek and reviewed local mutation. Add a ScopeProvider for explicit subscription/namespace metadata queries, known namespace resolution and per-namespace discovery results. Discovery takes a concrete namespace slice, never resource-group rules. Existing Resource.Subscription remains a topic subscription; Namespace adds separately named AzureSubscriptionID metadata. The TUI binds all inventory and refresh to the active environment and invalidates asynchronous results on switching.

## Endpoints

No application server. Azure's management APIs are read-only namespace/subscription metadata operations; Service Bus administration lists entities and runtime counts inside chosen namespaces. Existing emulator access is unchanged.

## Data

Extend nonsecret preferences with named environments: stable ID, connection context, explicit namespace snapshots, All/Selected dashboard mode and retained entity inclusion. Active environment is explicit. Preserve old local watchlists on first local launch and retain their absence-means-All semantics; new environments start Selected/empty. Invalid preferences still fail without overwrite. Existing Save snapshots must deep-copy nested environment selections.

## Flow

Startup selects exactly one explicit connection mode. Local modes migrate into one local environment if not previously configured. Azure mode starts with no inventory until an environment is selected. The editor queries namespace metadata for only explicitly opened Azure subscriptions; group previews add exact listed namespaces. Apply updates the session and initiates scoped discovery; Save persists preferences. Discovery results and count/peek responses carry or are checked against a scope generation. Partial failed namespaces retain stale observations; successful complete listings can identify missing watched resources.

## External

Preserve explicit fixture and fixed emulator modes. To make scope selection usable against Azure without building the separately deferred sign-in product, add an explicit read-only Azure CLI connection using an existing login and explicit tenant. Pin connection identity rather than silently falling through credentials. No login, credential storage, role mutation or cloud send/receive/delete. Cloud operation attempts fail closed. No new MCP access. Authentication UI/vault and cloud message mutations remain outside this feature.

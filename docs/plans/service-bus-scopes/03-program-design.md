# Program design: Saved environments

Authorized to proceed without gate pauses, 2026-10-24.

## Files and ownership

- explorer/explorer.go: Namespace, AzureSubscription, NamespaceDiscovery and ScopeProvider contracts; Store remains compatible with existing broker tests.
- config/config.go and config_test.go: additive validated environment preferences, deep snapshots and explicit local migration preserving watchlists.
- azure/cloud.go and cloud_test.go: explicit read-only Azure CLI connection and namespace metadata/discovery; no cloud mutation implementation.
- fixture/scopes.go: deterministic multi-namespace metadata for the real fixture TUI without altering original fixture contract tests. Local emulator exposes only its one real namespace.
- ui/environments.go and environments_view.go: coherent editor/chooser and Settings inclusion; existing model/actions/view wire these screens and prevent stale replies from crossing environment boundaries.
- cmd/sbx/main.go: explicit mode/connection/provider wiring.
- ui/environment_test.go and scripts/environment-smoke.py: user-visible regression and real PTY behavior.

## Types and signatures

Namespace retains Profile/Address/Name plus AzureSubscriptionID/AzureSubscriptionName/ResourceGroup. ScopeProvider exposes Subscriptions(ctx), Namespaces(ctx, subscriptionID), ResolveNamespace(ctx, input), and DiscoverScope(ctx, exactTargets). NamespaceDiscovery includes one target, its resources, and an error; successful empty and failed listing are distinct. Environment stores ID/Name/Connection, Namespaces, All and Watchlist. No resource-group wildcard exists in preferences or scoped discovery input.

## Call stack

Start → load preferences → explicit connection → local migration or first-run → Model.Init → active environment targets → ScopeProvider.DiscoverScope → generation-checked namespace results → watched count sweep. Editor → subscriptions → explicit subscription → namespaces → group snapshot preview → concrete membership → Apply → same scope-load flow. Save uses a deep preference snapshot. Switching cancels old reads and changes generation; reviews own their lock/outcome and cannot switch.

## Test plan

- Save/reload independent named environments with identical queue names in different namespace contexts; preserve Selected empty vs All, removed namespace watchlist entries, active selection and private atomic save.
- Group preview stores only current namespace members. A later namespace in that group never enters discovery; query only selected Azure subscription metadata.
- Scoped discovery requests never include excluded namespaces; stale discovery/count/peek replies cannot update a different environment even when entity names match.
- Partial discovery keeps observed values stale and unresolved picks unverified; complete successful discovery distinguishes missing picks from an empty inclusion list.
- Keyboard creation/edit/name/duplicate/delete, search ownership for q/j/k/s, Apply versus Save, failed Save and restart; independent grid/table inclusion and global appearance.
- Every review phase blocks environment switching; old safe DLQ workflow tests unchanged.
- Real PTY at 160×48 and 80×24 through creation, group preview, inclusion, save/restart and switching. Render actual terminal in browser, inspect theme/compact screenshots and real keyboard/pointer clip against product references.

## Least confident decisions

Read-only Azure CLI is deliberately narrow: no new browser sign-in/vault product and no cloud mutations. Live Azure credentials may not exist in the orb; mocked management transport and fixture/emulator checks must not be described as live Azure verification. Some editor screens use full-screen lists instead of simultaneous panes at compact sizes; preserve semantics over incidental mockup geometry.

# Research: discovery dimensions and existing behavior

2026-10-24. Read-only code/documentation investigation, no Azure connection or permission experiments.

## Observed implementation

| Source | Observed contract / consequence |
| --- | --- |
| `service-bus-explorer/cmd/sbx/main.go:19–27,48–56` | Exactly one of fixture/emulator; no cloud sign-in or cloud discovery. The reported hundreds-of-Azure-queues problem cannot currently be reproduced against cloud in this app. This is future-facing product design, not a verified existing cloud bug. |
| `internal/azure/store.go:43–78` | Discover pages queues, topics and each topic's subscriptions through a single fixed namespace administration client. It does not enumerate Azure subscriptions, resource groups or namespaces. |
| `internal/explorer/explorer.go:10–11` | Resource identity contains Profile, Namespace, Queue, Topic, Subscription. Subscription here means **topic subscription**, not Azure subscription. No Azure-subscription/RG metadata is currently modeled. |
| `internal/ui/model.go:87–107` | Startup calls Discover without a scope input. An absent profile watchlist means all discovered entities watched; an explicit empty list means none. |
| `internal/ui/model.go:109–148` | Dashboard filters by watched identity; Queues/Topics browse all discovered entities. Count refresh only targets watched entities. A watchlist alone does not limit discovery. |
| `internal/ui/actions.go:150–207` and `internal/ui/view.go:399–440` | Settings combines appearance, Save and entity watchlist checkboxes. Edits apply in-session; Save explicitly persists preferences. Labels already say watchlists affect Dashboard only, not credentials/MCP. |
| `internal/config/config.go:18–31,44–106` | Nonsecret validated versioned preferences; private atomic Save. Malformed/unsupported preferences fail rather than silently reset. Keep that user contract; no storage redesign specified here. |
| `docs/plans/service-bus-explorer/auth-and-mcp-notes.md` | Existing research already separates namespace discovery and messaging authorization and recommends manually adding known namespaces when discovery is unavailable. These remain proposals, not implemented auth. |

Paths above beginning `internal/` are relative to `service-bus-explorer/`. Source line numbers describe the inspected checkout. Existing product preferences establish Settings-only inclusion editing and independent grid/table/sidebar controls; proposed scopes complement those decisions.

## Azure facts and sources

1. **Hierarchy:** Azure subscription → resource group → Service Bus namespace → queue or topic → topic subscription. Microsoft's [resource scope guidance](https://learn.microsoft.com/en-us/azure/service-bus-messaging/authenticate-application#resource-scope) distinguishes subscription, resource group, namespace and entity. The [topic-subscription listing](https://learn.microsoft.com/en-us/rest/api/servicebus/controlplane/subscriptions/list-by-topic?view=rest-servicebus-controlplane-2026-01-01) confirms topic subscription nesting. These subscriptions are not interchangeable.
2. **Azure subscription is a valid namespace-discovery boundary.** [Namespaces List](https://learn.microsoft.com/en-us/rest/api/servicebus/controlplane/namespaces/list?view=rest-servicebus-controlplane-2026-01-01) explicitly lists namespaces within the subscription irrespective of resource groups. Resource-group enumeration is not a required first step. The product can show group organization without forcing a group-only workflow.
3. **Resource group is an optional narrower discovery boundary.** [List By Resource Group](https://learn.microsoft.com/en-us/rest/api/servicebus/controlplane/namespaces/list-by-resource-group?view=rest-servicebus-controlplane-2026-01-01) qualifies the group by Azure subscription. Group name alone is incomplete identity. Both namespace-list operations document paging; the first page is not a complete selection preview.
4. **A namespace can be addressed without broad enumeration.** [Namespaces Get](https://learn.microsoft.com/en-us/rest/api/servicebus/controlplane/namespaces/get?view=rest-servicebus-controlplane-2026-01-01) identifies a known namespace by Azure subscription, group and namespace. Microsoft's [messaging client authentication example](https://learn.microsoft.com/en-us/azure/service-bus-messaging/authenticate-application#authentication-of-the-service-bus-client) uses a fully qualified namespace and credential without subscription/RG enumeration. Knowing the name does not grant access or guarantee entity listing.
5. **Multiple subscriptions require explicit context.** Namespace-list operations address one Azure subscription at a time. [Subscriptions List](https://learn.microsoft.com/en-us/rest/api/resources/subscriptions/list?view=rest-resources-2022-12-01) lists subscriptions for a tenant; [Azure CLI subscription guidance](https://learn.microsoft.com/en-us/cli/azure/manage-azure-subscriptions-azure-cli) explains subscription/tenant selection. Multi-subscription is not automatically multi-tenant.
6. **Namespace discovery and message permissions are not equivalent.** Provider permissions list [`Microsoft.ServiceBus/namespaces/read`](https://learn.microsoft.com/en-us/azure/role-based-access-control/permissions/integration#microsoftservicebus). [Data Receiver](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/integration#azure-service-bus-data-receiver) and [Data Sender](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/integration#azure-service-bus-data-sender) have entity-read management Actions as well as receive/send DataActions, but not namespace-read. [Data Owner](https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/integration#azure-service-bus-data-owner) includes Service Bus wildcards in both categories, bounded by assignment scope. Do not simplify this into “data roles have no management permissions.”
7. **Restricted access is legitimate.** [Resource scope guidance](https://learn.microsoft.com/en-us/azure/service-bus-messaging/authenticate-application#resource-scope) supports namespace/entity role assignments. A usable namespace or entity is not evidence of subscription-wide listing permission, and metadata-read access does not establish message receive access.

## Recommendation justified by the evidence

Use the **combination**: account/tenant context and Azure subscription-qualified namespace identity; resource groups organize and batch-select namespaces. Save an **explicit set of namespaces** as an environment, then reuse entity watchlists within it. A resource-group-only model is too coarse and fails cross-group/cross-subscription environments; a subscription-only model is broader still. Namespace-only labels without Azure context are not a complete management selection.

Group selection should be a snapshot, not a live wildcard. This is a product choice to keep discovery bounded, not an Azure restriction. Existing dashboard watchlists control visibility after discovery, so they cannot by themselves solve the discovery problem.

## Unverified / deferred

- Exact partial-authorization behavior of each collection listing (filtered results versus failed request) was not established by the reference pages. Do not promise child-scoped grants make broad listing work.
- No live roles, network access, counts, paging completeness, cloud authentication or real-user usability measurements were tested.
- Known namespace entry can avoid broad enumeration; inventory/count/peek capabilities still depend on the exact operation, credential and network policy. Entity-only credentials must remain visibly restricted.
- Cross-tenant environments, connection setup delivery, configuration migration mechanics, API selection and implementation belong to later gates, not this product proposal.

## Mockup validation

Static plain-HTML screens are product references with fictional data, not a live TUI. Browser inspection checks legibility and layout only; it cannot validate terminal cell capacity, actual key ownership, discovery boundaries or permissions. The generated comparison board explores placement alternatives; its namespace-overview cards, F1/Ctrl+Q labels and LOCAL Azure-subscription count are illustrative mistakes and not recommended behavior. The hand-authored HTML and product document are authoritative.

- Ran the artifact checker: `share.mjs --check --dir docs/plans/service-bus-scopes/mockups` → `Checked 5 page(s): no errors.`
- Rendered all four screen references with Chromium at 1160×900, DPR 2; inspected captures using view_media. Corrected stepped outer frame alignment and recaptured. Final Dashboard, chooser and Settings captures show legible context/controls and aligned frames.
- State references render at 80 columns; DOM inspection reports widths `[80]` for all lines and no overflow in all four panels. The compact editor frame uses 23 rows within its proposed 80×24 state. This proves only the HTML reference's dimensions, not actual terminal layout behavior.
- Inspected the proposal's comparison at 1160px and 390px, DPR 2. The narrow comparison stacks, and DOM reports no horizontal document overflow. Captures are under `.amp/in/artifacts/service-bus-scopes/` in this thread. Browser session closed; supervised artifact server retained for review.
- Review portal: https://t-03h3zgypthdehx6998kv83klk-p23520.onamp.dev/index.html . The page links all four references and supports the standard floating review button.
- No application source changed, no Go/app tests needed for this research-only deliverable, no cloud access attempted. Documents remain local/uncommitted pending review; no pushes, architecture or implementation performed.

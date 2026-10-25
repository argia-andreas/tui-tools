# Product: Saved environments for Service Bus Explorer

Gate 1 approved — 2026-10-24. Applies only to Service Bus Explorer. The user authorized implementation and remaining gates without further pauses; shipping remains unauthorized.

## Problem

“I have access to lots of Azure resources, but only want the queues for the environment I am working on. I should not have to sift through hundreds of queues or accidentally mix production and staging.”

## Success metric

Target: at least 90% of representative users can switch from Production to Staging and open the intended DLQ in under 15 seconds, with no wrong-environment action. Measure in moderated keyboard-only sessions against a fictional inventory of 500 entities across multiple Azure subscriptions. This is a proposed usability target, not a measured result.

## Announcement — the blog post before the feature

Save a named environment such as Production or Staging and choose the Service Bus namespaces it contains. Browse only that environment, then use the familiar dashboard inclusion settings to show the queues and topic subscriptions that matter. Switch environments from Ctrl+P without losing your saved choices. Resource groups help you select namespaces, but never silently add future namespaces to your environment. Access failures and stale resources stay visible instead of turning into reassuring zeroes.

## Product decisions

### 1. An environment is a name plus an explicit set of namespaces

- Use **Environment** in user-facing navigation and **Discovery scope** as the explanatory Settings label. Production and Staging are user-chosen names, not inferred Azure tags or guarantees about safety.
- The Azure hierarchy is Azure subscription → resource group → Service Bus namespace → queues/topics → topic subscriptions. An Azure subscription and a topic subscription are different things; always qualify the former in mixed contexts.
- Namespace is the practical discovery boundary. Allow one or many namespaces spanning one or many resource groups and Azure subscriptions under the selected account/tenant. Namespace paths retain their Azure subscription and resource-group context; display names alone are never sufficient to distinguish a selection.
- For the initial product, one environment uses one explicit account/tenant connection context. Multiple Azure subscriptions in that tenant are supported; cross-tenant aggregation is deferred. Fixture/local-lab environments stay explicitly local and are not mixed into cloud environments.
- Use subscription and resource-group selection as **batch selection aids**, not live wildcard membership. “Select namespaces in this resource group” selects the currently listed namespaces after a complete successful listing. It previews the exact members, deduplicates overlaps, and says “New namespaces are not added automatically.” For multiple resource groups repeat this action, including across Azure subscriptions. A failed/partial listing cannot be presented as a complete group selection.
- No “all accessible resources” startup default. No queue/topic enumeration outside the active environment. The editor may list namespace metadata only after the user chooses an Azure subscription; it does not need to load queue inventories from those namespaces.

### 2. Discovery and dashboard inclusion are different jobs

- **Discovery scope:** which namespaces are browsable at all in this environment. Applies to Dashboard, Queues, Topics and the sidebar, and to background entity/count refresh. Switching to Queues must not escape the scope.
- **Dashboard inclusion:** existing entity watchlist editing, now contextual to the environment. One list controls both grid and table. Unwatched entities remain browsable, but only inside the active environment.
- Keep Settings as the single home: Appearance (existing theme/sidebar/inspector/grid-table), Environments, and Dashboard inclusion for the active environment. Do not add a competing dashboard editor to the header or scope picker.
- Each environment retains its own inclusion choices. Appearance settings remain global. Switching environments changes the inclusion list, not the theme or layout.
- New cloud environments start with **Selected entities**, none selected; choose specific queues/topic subscriptions using the existing checkboxes plus search and namespace grouping. “Select matching results (N)” is explicit and only affects the current result set. Empty inclusion is valid.
- An explicit alternative, **All entities in saved namespaces, including new entities**, is available in that same inclusion section, with a size warning before enabling it. It is never implicit. Switching back to Selected restores the last explicit entity selection; checkboxes are disabled while All is selected to avoid pretending they affect the result.
- A new entity in an existing selected namespace appears in scoped browsing; it appears on the dashboard only in All mode or after the user adds it in Selected mode. New namespaces never join automatically.
- Removing a namespace immediately removes its entities from live views. Its saved inclusion choices remain under “Outside current discovery scope” so re-adding the exact namespace restores them. They are not counted or treated as accessible while excluded; users can explicitly forget them.

### 3. Create, name, save, edit and delete without a second settings system

1. Settings → Environments → New. Enter a required unique name (trimmed, case-insensitive duplicate detection, up to 40 characters); select a connection context. Duplicate an existing environment to start with a copy, but require a new name.
2. Choose Azure subscriptions to inspect, then expand resource groups and select namespaces. Keep the selected basket visible and searchable across subscriptions. Show readable names plus enough subscription identifier context to distinguish duplicates; Details exposes complete context without requiring horizontal scrolling.
3. If broad listing is unavailable, “Add known namespace” accepts a namespace address or full Azure resource identity under the explicit connection context. Validate namespace access without requiring subscription-wide listing. Show unknown group/subscription metadata honestly. An entity-only connection must state that broader namespace inventory is unavailable; it must not imply access to the whole namespace.
4. Review exact selected namespaces and whether discovery was complete; at least one namespace is required to apply a new environment. Known inaccessible namespaces may be retained deliberately with a warning, never silently dropped. Name-only drafts can be canceled but are not usable environments.
5. **Apply & use** activates edits for this session and marks preferences unsaved. **Save preferences** is the existing explicit persistence action; it saves all pending preferences, including environments, dashboard inclusion and the last active environment. A persistent banner explains that Apply is not Save. No secret or message content is part of scope preferences.
6. Editing the active environment uses the same form with current choices preselected. Cancel discards the form only; Apply updates the session. Editing an inactive environment offers Apply without switching, or Apply & use. Rename does not reset inclusion. Saving failure leaves the session usable, clearly unsaved, with Retry; never claims persistence succeeded.
7. Delete is confirmed with the environment name; explain that only local preferences are removed, not Azure resources. Deleting the active environment returns to the environment chooser with no active inventory. Never silently switch to another environment.

Keep current appearance/watchlist edits immediate in-session, with explicit Save. The scope editor alone stages its multi-step membership changes until Apply so browsing never sees a half-built environment. Leaving the app with unsaved changes offers Save / Discard / Cancel. Switching environments does not silently save or discard unsaved preferences; their unsaved marker follows the session.

### 4. Select and switch with persistent context

- Header always says `Environment: Production` (or a shortened name plus Details in compact mode), with a separate explicit LOCAL/FIXTURE marker where relevant. Do not rely on red/green environment colors.
- Ctrl+P → Switch environment opens a searchable chooser, showing current marker, name, namespace count and Azure-subscription count. Enter selects; Esc cancels. The existing palette remains an action palette, not an overloaded new key binding.
- After selection, return to that environment's Dashboard, not the prior environment's selected message. Show loading/partial/error state for the new environment, never old resources under the new name. Preserve dashboard selection by identity when returning if still present; otherwise select the first visible item.
- Never switch while a mutation is acquiring, holding a lock, submitting or releasing. Show “Finish or cancel the current review before switching”; do not abandon a lock silently. Terminal outcomes must be acknowledged before switching; an unknown outcome is not erased by choosing a different environment.
- Restore the last saved active environment on restart. If unavailable, show the chooser and actionable reason; never broaden discovery or fall back to a different account/environment.

## First-run, empty and exceptional states

| State | User-facing behavior / recovery |
| --- | --- |
| First cloud run, no environments | Dashboard placeholder: “Choose what to discover”, Create environment / Add known namespace / stay offline. No global entity scan and no fake counts. Setup errors keep choices intact. |
| Existing local preferences | Preserve existing fixture/emulator watchlist behavior and choices in an explicitly Local lab / Offline fixture environment; never reinterpret `Profile` or its topic `Subscription` as an Azure subscription. If future legacy cloud configuration exists, require review before discovery rather than guess its namespace boundaries. |
| No watched entities | “Nothing on this dashboard. N entities available in this environment.” Open Dashboard inclusion; do not imply no Azure resources exist. |
| Valid namespace, no entities | “No queues or topic subscriptions found in the selected namespaces.” Show last successful discovery and Refresh / Edit environment. |
| Picker search has no results | “No matching environments/namespaces”; clear search, add known namespace, or create. Do not offer a global inventory fallback. |
| No Azure subscriptions visible | Explain listing access may be absent; allow known namespace entry or another explicit connection context. Do not say the account has no namespaces. |
| Account expired / wrong tenant | Reconnect under the named account/tenant or edit connection; preserve environment and inclusion. Never choose another credential implicitly. |
| One namespace denies listing, others work | Persistent “Partial discovery: 2 of 3 namespaces checked”; show the failing member and Retry failed / Edit. Dashboard/entity totals explicitly partial. Distinguish inventory access from count access and message access. |
| Timeout, throttling or network error | Keep last observed entries marked stale with last-success time and error. With no successful observation show unavailable, not 0. Retry is explicit; no claim of complete discovery. |
| Watched entity/namespace missing | After a complete successful relevant listing, mark “Not found”; after access failure say “Could not verify”, not deleted. Keep saved selection; show separate unresolved entry with reason, Retry / Edit / Forget. Never replace it with a same-named resource elsewhere. |
| Unknown dashboard totals | Preserve existing `~` stale and `—` unavailable conventions, observation time and details; do not compute authoritative totals from peek pages. |
| Many subscriptions / duplicate names | Search within chosen subscription metadata, lazy expansion, retained selection basket and identifier details. Names such as `shared` in two subscriptions remain separate. Only selected subscriptions are inspected by the editor. |
| Resource moved to another group/subscription | Original membership remains unresolved until reviewed. Treat a newly found matching display name as a candidate, not an automatic replacement. |
| All namespace members lost | Keep named environment visible as Needs attention; no fallback/global scan. Edit, retry or choose another environment. |

## Keyboard and compact terminal behavior

- Preserve current j/k or arrows, Tab/Shift+Tab visible-pane focus, h/l collapse/expand, Enter activate, Space checkbox, Esc back and Ctrl+P actions. In text/search fields, printable keys—including q, j, k, s—type text rather than perform global actions. Esc first leaves search, then closes the form with discard confirmation if edited.
- Picker is a vertical list: type to filter, arrows/Ctrl+N/P move, Enter chooses. Ctrl+P already opens the palette; Switch environment is one action. No new unverified single-letter global shortcut.
- Resource editor: Tab cycles name, connection, search, tree, selection basket and buttons; arrows/j/k navigate tree, h/l collapse/expand and Space toggles namespace. Group row selection is a clearly named batch action with a preview, not an ambiguous tri-state wildcard checkbox. Counts say “N selected of M listed”, not assumed totals.
- At 80×24: full-screen chooser/editor instead of an overlay, sidebar hidden as it is today. A single breadcrumb identifies current Azure subscription/group; a step view alternates available namespaces and selected basket. Header, selected row, selection count, critical warning and Apply/Back footer remain visible. Lists scroll vertically; names truncate visually with full identity on a Details page. No horizontal navigation needed to save or cancel.
- At 80×24 the dashboard remains compact, with current environment and partial status in pinned lines; show more detail through an action. At less than 60×18 show a resize notice; no hidden confirmations or mutation submission. Environment switching and quit remain available via a minimal visible menu.
- Focus uses a visible `>` marker/inverse row as well as theme color. Terms do not rely on glyphs; preserve ASCII fallback. Theme behavior remains existing Midnight/Gruvbox/Light, not per-environment appearance settings.

## Acceptance scenarios for later gates

1. Selecting two namespaces named `shared` in different Azure subscriptions never merges them; Dashboard, Queues, Topics, tree and refresh remain inside the selected set.
2. Selecting two resource groups previews their current namespaces. A namespace later created in either group does not appear until explicitly added.
3. In Selected mode a new queue is browsable but not watched. Switching grid/table never changes membership. In All mode a new entity in an already selected namespace is included.
4. Save/restart restores names, exact memberships, per-environment inclusion and last active selection. Apply without Save is not presented as durable; failed Save remains unsaved.
5. A 403 listing one namespace cannot turn its previous DLQ count into 0 or make complete totals appear. Known namespace entry works without a promise of broad management permissions.
6. A missing resource and an unverified resource receive different labels. Removing and re-adding an exact namespace restores its prior explicit inclusion; a same-name resource elsewhere does not inherit it.
7. Keyboard-only 80×24 creation and switching keep Apply, Cancel, status and identity details reachable. A held/unknown mutation review cannot be silently abandoned by switching.

## Screens

- `mockups/dashboard.html`: recommended persistent environment context with unchanged minimal grid and a visible Settings route.
- `mockups/switcher.html`: searchable saved-environment chooser, current/partial markers and explicit local mode.
- `mockups/editor.html`: multi-subscription namespace selection, resource-group snapshot selection and the existing Settings dashboard-inclusion section.
- `mockups/states.html`: first-run, empty dashboard, partial/stale discovery and compact 80×24 editor references.
- Supplementary Painter board compares header picker (recommended), sidebar picker and Settings-only switching. Generated incidental controls/overview cards are not requirements; the HTML screens and this document take precedence.

All inventories, counts and names in the mockups are fictional. Static mockups show intended states, not implemented controls or terminal cell-capacity proof.

## Alternatives and boundaries

- Resource-group-only scope rejected: one group can hold unrelated namespaces, and environments can span groups/subscriptions.
- Azure-subscription-only scope rejected: too broad for the reported problem. A namespace-name-only list hides Azure context and limited-access provenance.
- Live resource-group membership rejected for the first version: quietly admitting future namespaces defeats bounded discovery. Revisit only with explicit opt-in membership semantics.
- A second per-queue scope list rejected: duplicate of dashboard inclusion. Scope is namespace-level; dashboard inclusion remains entity-level.
- Sidebar-only selector rejected because compact mode hides it. Settings-only switching hides operational context. Recommend persistent header plus Ctrl+P action.
- Environment is not an Azure permission boundary or an agent grant. Existing safe peek, explicit send/delete review and separate future MCP grants remain unchanged. Do not expand cross-namespace resend destinations as a side effect.
- This product document leaves architecture, storage format, APIs and authentication delivery to the approved remaining gates. Out of scope: deployment, cross-tenant environments, shared/team scope distribution, dynamic tags/rules, production-role enforcement and any changes to LazyFTP/logs/MCP.

## Approval

Approved by the user through the parent thread on 2026-10-24, followed by explicit authorization to implement, test and verify the real terminal without more gate pauses. Do not ship until asked. Research evidence and baseline-code references live separately in `research.md`; execution evidence and limitations belong in the status/verification notes.

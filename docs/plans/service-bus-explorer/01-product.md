# Product: Service Bus Explorer terminal UI

Working description, not a selected product name. Gate 1 approved 2026-10-23, including the platform, local-feedback and replay decisions below. Architecture and program design approvals are recorded separately; implementation slices await Gate 4 approval.

## Problem

Developers need a keyboard-first way to see the Azure Service Bus queues that matter to them, inspect messages and understand dead-letter failures, without burying the DLQ or making accidental message removal easy. Users new to Azure need clear explanations of access and message-operation consequences.

## Success metric

Acceptance direction: at a representative terminal size, show 12 watched queues simultaneously in grid view with both queued and DLQ counts, and let the user open either message view using the keyboard. Table view must show more rows at that size. Choose the terminal dimensions and broader usability/performance targets during design; do not treat generated image dimensions as tested terminal capacity. No measurements have been made yet.

## Announcement — proposed user experience

A terminal workspace for your Azure Service Bus queues and topic subscriptions. Start with a personal dashboard, switch between clear queue boxes and a compact table, and jump straight into active or dead-letter messages. Inspect safely by default, find actions with Ctrl+P, and review consequences before resending or deleting messages. Choose a dark, warm or light theme and keep navigation visible or collapse it for more space.

## User-requested direction

### Initial delivery and local feedback

- Linux and macOS first, with portable choices allowing Windows later without redesign.
- Begin with connection-string access, dashboard and safe Active/DLQ browsing; add reviewed single-message resend/delete next. Microsoft sign-in follows the connection-string development path. Bulk operations and MCP are later scope.
- Fast repeatable local development must work on the developer's computer and, where feasible, in the orb. Include deterministic seeded active/DLQ messages, empty queues, useful failure reasons and topic subscriptions.
- Provide simple development commands to start, seed/reset, send, drain and dead-letter messages in a disposable lab. Do not allow these destructive lab commands to silently target real Azure.
- Separate instant fixture-backed UI tests from local broker integration and the smaller set of checks requiring real Azure. Mocks are not proof of broker behavior or cloud authentication.

### Dashboard

- Configurable watchlist of important queues and topic subscriptions across namespaces.
- User-selectable grid or table over the same watchlist. Grid targets at least 12 visible queue boxes (currently 3 columns × 4 rows); table fits substantially more rows and scrolls/pages as needed. Twelve is not a watchlist cap.
- Each box shows namespace and queue name, a green queued count and a red dead-letter count, with text labels so color is not the only signal.
- Keep the dashboard focused on counts and navigation. Inclusion editing belongs behind Settings, not in an always-visible editor. No mandatory inspector, charts or detail panels on the dashboard.
- Expanded full-height left navigation by default: Dashboard, Queues, Topics, Preferences/Settings. It is collapsible independently of Grid/Table selection. Exact labels and whether Preferences/Settings is one entry remain open.
- Topics group their subscriptions; subscriptions own message backlogs and DLQs. Do not invent a topic-level DLQ counter or double-count subscription counts.

### Entity explorer

- LazyGit panel structure, Amp CLI workspace and Neovim navigation are inspirations, not a requirement for complete Vim modes.
- Full-height collapsible left entity tree: namespaces, queues, topics and subscriptions. Central message browsing and an optional independently collapsible full-height metadata/body inspector.
- Active and Dead-letter are equally accessible views. Surface DLQ counts while looking at a queue; emphasize reason, description, delivery count and age in DLQ inspection.
- Keyboard-first interaction and Ctrl+P fuzzy actions. Concrete generated shortcuts, icons and focus details are illustrative until reviewed for conflicts and terminal compatibility.
- Dark, warm and light themes across all layouts. Inspired by modern coding palettes; no claim of exact theme imports.

### Safe operations

- Peek is the default inspection path: no lock acquisition or message removal. Distinguish it from Receive, which acquires a message lock.
- Complete removes a received message; Dead-letter settles it into its DLQ. Review risky actions with clear resource/message scope and consequences; safe cancellation is the default.
- DLQ actions include resending to the source queue/topic and permanently deleting the explicitly reviewed, received DLQ message, subject to permissions. User-approved first flow: receive next, then review its actual identity; it may differ from the highlighted peek message. No promise of arbitrary peek-row acquisition. Releasing a received message can affect delivery count, so cancellation is not presented as side-effect-free browsing.
- Resend sends a new copy, not an atomic move. Review destination, duplicate-processing risk and whether to remove the original only after a successful send. Send success and cleanup failure must be distinguishable; the original can remain in the DLQ.
- Replay from a subscription DLQ sends to the original topic; warn that other matching subscriptions may receive another copy. No subscription-only replay is promised. Queue DLQ replay sends to the original queue.

### Authentication and future agents

- Product auth options are Microsoft/Azure sign-in and connection strings, informed by ViewBus. Implement connection strings first for local development; Microsoft sign-in follows. Make account, tenant and permissions understandable. Exact SDK and token storage implementation remain architecture decisions.
- Future MCP access is locked down by default. Explicitly allowlist read access to particular queues/subscriptions and DLQs, with a separate optional allowlist for resubmission. Reading never implies resending, resending never silently implies deletion, and dashboard inclusion never grants MCP access.
- See the separate research notes for auth recommendations, references and security boundaries; these are not architecture approval.

## Screens and saved references

- `mockups/g-dashboard-expanded-sidebar-themes.png`: latest expanded-navigation dashboard, dark/warm/light.
- `mockups/e-dashboard-grid12-themes.png`: space-maximized 12-box grid with navigation collapsed.
- `mockups/f-dashboard-table-themes.png`: denser alternative, 24 visible sample rows from a 48-queue watchlist.
- `mockups/a-harbor-themes.png`: active/DLQ browsing and full-height inspector.
- `mockups/b-orbit-themes.png`: collapsed navigation and Ctrl+P DLQ action palette.
- `mockups/c-forge-themes.png`: scoped resend review, duplicate risk and separate cleanup.
- Earlier references are indexed in `mockups/README.md`; they are history, not additional screens to implement.

Painter-generated images, not running UI. Preserve them rather than creating a new interactive prototype as part of this documentation-only task.

## Open for later design

- Product name, representative terminal size and accessible icon fallback.
- Detailed delivery slices and timing of deferred capabilities.
- Exact shortcuts, preferences navigation and theme selection.
- Watchlist size behavior beyond one grid viewport, refresh/staleness display and partial-access states.
- Session-enabled messages, duplicate detection and replay property handling.
- Architecture and Gate 3's concrete behavior/resource targets are approved separately. Gate 4's implementation slices await approval; the items above are resolved or explicitly deferred in the program design where specified.

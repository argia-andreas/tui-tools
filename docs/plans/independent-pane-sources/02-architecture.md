# Architecture: Independent pane sources

## Fit
Keep the existing Bubble Tea workspace host, picker/wizard and UI model. Give each pane its own detached flag, source-choice cursor and last local folder. `Location.Remote == ""` continues to mean local, never detached. Listing generations invalidate stale data when a pane is detached or rebound.

The connection host receives a stable pane index plus startup/in-workspace context, and returns one target location, not a workspace pair. An optional startup-local folder is the only compatibility exception: initial bookmarks may still restore the local-left companion. Subsequent choices never do so. The UI owns all pane focus, navigation and guards.

## Endpoints
None. Internal Bubble Tea messages carry pane targets. No HTTP API or MCP change.

## Data
No new persistence. Existing bookmarks retain their optional companion-local metadata; new pane-specific bookmarks may omit it. Startup with such a bookmark uses the starting local folder. Old nonempty local paths remain validated as absolute.

## Flow
Pane Detach → guard unfinished work → clear only that attachment/listing/selection/filter and invalidate async results → inline choices. Local choice → targeted folder prompt → validate existing directory → navigate that pane. Saved/New choice → targeted host picker/wizard → existing validation/test/persistence flow → one `ConnectionOpened` → navigate only its pane and focus it. Sidebar locations use the last focused file pane.

## Connection ownership decision
Reuse the existing single backend and append-only session snapshots; do not add per-pane adapters or duplicate configuration contexts. `connections.Prepare` copies the entire active config and rejects existing remote names (case-insensitively). New successful setup therefore preserves every prior remote definition and adds exactly one name. Old candidate configs stay owned by `connectionSessions` until application exit, including in-flight commands. Detach does not erase credentials. Captured retry locations continue resolving to the same named definitions in each later snapshot. Two different credential contexts under the same name remain unsupported rather than ambiguously routed. External edits to user config remain outside this snapshot guarantee, as before.

## Safety
Source changes including sidebar changes are guarded during planning/checking/copy execution. Pending modal confirmations own input. Both panes must be attached before copy; remote/remote remains rejected. Preview cancellation/generation and listing generations prevent detached content returning. Editor completion only refreshes its originating unchanged attachment. No new transfer queue or cancellation semantics.

## External
Existing rclone CLI/config and provider auth only. No new environment variables, protocols or dependencies.

## Approval
User authorized proceeding through remaining gates without pauses on 2026-10-24. This documents the scoped decisions before implementation.

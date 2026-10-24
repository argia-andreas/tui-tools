# Inspection notes — not an approved architecture

Read-only findings to carry into Gate 2 after product approval:

- `internal/storage/storage.go`: Location already distinguishes local from remote independently of pane position; an empty Remote means local, so it cannot also mean detached.
- `internal/ui/model.go`: pane contains location/listing/selection state, but the disconnected flag lives globally on Model. list skips only pane 1; navigate only clears the flag for pane 1. ConnectionOpened can navigate both panes and always focuses left.
- `internal/ui/destination.go`: detached rendering, choice state and folder submission assume right. ConnectRequested has no pane identity; ConnectionOpened carries a left/right pair rather than one target.
- `internal/ui/view.go`: pane 0 is always labelled LOCAL even if its location is remote. Sidebar keyboard and mouse activation route by source kind instead of remembered pane focus.
- `cmd/lazyftp/workspace.go`: picker and wizard success construct left-local/right-remote pairs. Wizard success replaces the shared browser/transfer backend with the candidate's config; opening a second connection must not invalidate the first pane's config.
- `cmd/lazyftp/picker.go`: existing-connection details validate a mandatory companion local folder. That contract must change for pane-specific use without destructively rewriting existing bookmark metadata.
- `cmd/lazyftp/wizard.go`: connectionSessions owns candidate configs until app exit; lifetime and cleanup must account for both panes and captured asynchronous work.
- `internal/ui/transfers.go`: remote-to-remote copying is explicitly rejected. Generic source/destination locations already exist for supported copies; preserve the rejection rather than promise a new transfer mode.
- Existing generation tokens reject stale listings; extend that protection through detach and source-open ownership rather than relying on visibility alone.
- Independent read-only inspection confirmed jobs capture locations but not backend/config identity. Equal remote names in different credential contexts cannot safely be treated as interchangeable. Retry must retain the original usable endpoint context.
- `internal/ui/editor.go` and its completion handler retain a pane index for refresh; later design must distinguish an originating attachment from whatever now occupies that pane. Editing currently suspends normal UI input through Bubble Tea Exec, so concurrent detachment while the editor owns the terminal is not an observed failure.

Gate 2 must settle backend/session ownership, captured retry endpoints, asynchronous cancellation and stale result identity before signatures or implementation are proposed. These are findings and questions, not pre-approved design decisions.

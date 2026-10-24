# Product: Independent pane sources

## Problem
I want to choose what each pane shows, rather than treating the left as my computer and the right as my remote. I need to disconnect either pane, including one showing local files, and choose its next source without moving or disconnecting the other pane.

## Success metric
100% of the eight pane/source transitions pass a terminal acceptance walkthrough: detach and reopen local files or a remote, starting from either local or remote, in either pane. Every transition must leave the opposite pane's source, folder, cursor, filter and selections unchanged. This is a task-completion metric, not a claimed measured improvement or new telemetry requirement.

## Announcement — the blog post before the feature
Both LazyFTP panes now work the same way. Detach either pane to choose local files, a saved remote connection, or a new connection. Keep the other pane exactly where you left it, whether it shows your computer or a remote. Browse two local folders, put a remote on the left, or compare two remote locations. Copy confirmations and existing transfer safeguards remain unchanged; remote-to-remote copying is still unavailable.

## Screens
- [Workspace](mockups/workspace.html): attached sources, detached left, detached right, both detached, and two remote panes. States are shown together for review, not stacked in the application.
- [Local folder](mockups/local-folder.html): a pane-targeted prompt; cancel leaves the pane detached.
- [Saved connection](mockups/saved-connection.html): pane-targeted saved/existing remote chooser and details, without a companion local-folder setting.
- [New connection](mockups/new-connection.html): existing guided provider setup with an explicit target-pane heading; credentials and explicit persistence choices are unchanged.

## Proposed experience
1. Each pane header identifies LEFT or RIGHT and its actual source: LOCAL with a folder, REMOTE with the connection and folder, or No source. Source colors describe the source, not the side. Detaching a local folder uses the same action and empty state as disconnecting a remote.
2. Each attached pane has a clickable `[ Detach ]` control. The command palette exposes `Detach left pane` or `Detach right pane`, targeting the last focused file pane. No new single-letter shortcut is necessary; existing keyboard habits remain intact. The action is unavailable in the read-only demo.
3. Detach clears only that pane's listing, selection and filter, keeps focus there, and reveals three choices: Local files, Saved connection, New connection. It deletes no files, bookmarks or remembered credentials and does not disconnect the other pane even if both use the same remote. A status line names the detached pane. No confirmation is needed because detachment performs no file write or deletion.
4. In a detached pane, Up/Down or j/k chooses an action and Enter opens it; mouse clicks do the same. Tab still moves between panes. A detached pane stays detached when Esc closes its prompt, chooser or wizard. Esc in the inline chooser does nothing; it does not implicitly choose local files or quit.
5. Local files opens an existing-folder prompt for that pane. It defaults to the pane's last local folder, or the app's starting local folder if none was used. Open attaches that folder; invalid input stays editable with a clear error. No local folder is required to open a remote.
6. Saved connection opens the existing bookmark/remote list with `Choose connection · LEFT/RIGHT pane`. Selecting an item reveals remote root and optional bookmark name, with separate Open and Save bookmark actions. Existing bookmark companion-local-folder values are retained when saving existing bookmarks but do not move the opposite pane; new bookmarks do not require a companion local folder. An empty list offers New connection and Back.
7. New connection opens the existing FTP/FTPS, Azure Blob or S3 wizard, labelled with the destination pane. Test failure leaves the setup editable; only explicit successful Open attaches the selected pane. Read-only testing, session-only defaults and explicit credential persistence remain unchanged. Focus returns to the opened pane. Cancel and late results cannot attach a different pane or change the other pane's source.
8. Sidebar locations open in the last focused file pane, regardless of source type. Direct source actions from the palette likewise target that pane, not an inferred local/remote side. A replacement does not require a detachment first; cancel preserves the previously attached source, and successful Open replaces only that pane. Detach remains the explicit way to leave a pane empty.

## Safety and compatibility
- Keep the existing connection-first launch experience. Its initial chooser still opens the right remote beside the starting local folder. Esc still enters the workspace with local files on the left and no source on the right. Existing direct-launch options keep their meaning; symmetric command-line source flags are not part of this feature.
- Both detached panes remain focusable. Preview, edit, refresh, filter, parent navigation and selection do not operate on a detached pane. Copy requires two attached, successfully listed panes and clearly identifies the opposite pane as its destination.
- Local/local, local/remote and remote/local copying retain current confirmations and conflict handling. Two remote panes can browse, preview and edit independently, but Copy explains `Remote-to-remote copies are not supported yet`, including two panes on the same remote.
- Detach, source replacement and sidebar source changes are unavailable while transfer planning or a queue is active: `Finish or cancel the current transfer queue first`. Cancel remains an explicit action and is not rollback. Completed history remains visible; retry uses the originally captured endpoints, never whatever a pane shows later.
- A directory-listing error is an attached-source error, not automatic detachment. Retry and Detach are available. Listing or preview results arriving after detachment must not repopulate the detached pane or inspector.
- Detach releases this pane's use of a connection. It is not a promise of server-side logout or credential erasure. A connection still used by the opposite pane or captured work must remain usable.
- Narrow terminals retain the same two panes and existing minimum-size behavior. Source and pane identity stay legible with truncated paths; Detach remains accessible through the palette if a header cannot fit it.

## Alternatives considered
- Modal-only source selection: rejected as the default detached state because it hides the opposite pane and makes an intentionally empty pane look like a dismissed dialog. Keep the inline chooser, using overlays only for details and guided setup.
- New source-manager sidebar, tabs or automatic pane swapping: unnecessary for this request and would change established navigation.
- Automatically cancel transfers on detach: rejected; hiding a pane must not silently interrupt writes or imply rollback.

## Out of scope
Remote-to-remote copying, new protocols, tabs, remembered workspace layouts, swapping panes, changing startup policy, changing authentication methods, MCP changes, and changes to the other applications in this repository.

## Approval
Approve Gate 1, or what should change?

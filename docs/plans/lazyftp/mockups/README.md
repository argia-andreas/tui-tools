# Mockup reference library

These boards preserve the visual exploration in chronological order. They are generated design references, not implemented screenshots, passing tests, exact theme imports, or promises of every action shown. **Tandem remains an unapproved working name.**

## Current direction

The user now selects **board 07, Connection first (option A)** for default startup, superseding the earlier board 08 approval. Show a centered connection chooser with bookmarks, existing remotes and a prominent New connection action; hide root/bookmark details until an existing connection is selected. Omit empty search. Esc permits local browsing. The file workspace still follows board 08: real local files on the left, a destination on the right, no visible sidebars or empty transfer drawer. Midnight is the default palette. The full-height, independently collapsible sidebars from board 04 remain available on demand. The two central file panes are transfer endpoints; the transfer drawer occupies only the center, never the sidebars. Ctrl+P is the workspace action palette. Confirmation modals are separate from choosing an action. The user also approved implementing the connection wizard in boards 10–11. Board 12 is an MCP settings proposal, not an implemented permission editor. The product and program-design documents take precedence over accidental generated labels, figures or shortcuts.

| Board | Exploration | Status |
| --- | --- | --- |
| [01 — Dual-pane alternatives](01-dual-pane-alternatives.png) | Classic commander, transfer drawer, action preview | Early exploration, superseded by sidebar studies |
| [02 — Sidebar alternatives](02-sidebar-alternatives.png) | LazyGit numbered panels, Amp workspace tree, Neovim workbench | Influences, not three features to implement |
| [03 — Inspector and confirmation](03-inspector-and-confirmation.png) | Both sidebars open, both hidden, centered conflict modal | Confirms collapsibility and modal direction |
| [04 — Command palette](04-command-palette.png) | Full-height sidebars, fuzzy copy search, preview actions | Current layout and palette reference |
| [05 — Dark, warm and light themes](05-dark-warm-light-themes.png) | Night, Ember, Paper | Evidence that layout should work independently of theme |
| [06 — Coding-theme studies](06-coding-theme-studies.png) | Catppuccin Mocha, Tokyo Night, Gruvbox Dark, Dracula | Latest approved visual direction; palette-inspired only |
| [07 — Connection-first startup](07-startup-connection-first.png) | Focused connection chooser | Now approved for startup; empty search omitted, Esc opens local files |
| [08 — Files-first startup](08-startup-files-first.png) | Local browsing and disconnected destination | Retained workspace design; no longer the default first screen |
| [09 — Quick-launch startup](09-startup-quick-launch.png) | Three-action launcher | Alternative, not selected |
| [10 — Connection wizard](10-connection-wizard.png) | Storage type, FTP details, test and save | Approved; FTPS default, read-only test, explicit persistence |
| [11 — Cloud authentication](11-cloud-authentication.png) | Azure identity and S3 profile forms | Approved direction; authentication depends on available provider tooling |
| [12 — MCP permissions](12-mcp-permissions.png) | Default deny and exact folder-scoped grants | Proposed settings view; existing server still reads JSON policy |
| [13 — Polished connection-first](13-connection-first-polished.png) | Spacing, iconography and primary/secondary actions closer to original A | Approved refinement; user also requested moving terminal bits on startup |
| [14 — Type and FTP details](14-polished-type-and-ftp.png) | Storage selection and FTPS details with a read-only test | Approved visual direction |
| [15 — Cloud details](15-polished-cloud-details.png) | Existing Azure CLI identity and AWS profile forms | Approved visual direction; no new authentication implementation implied |
| [16 — Test outcomes](16-polished-test-outcomes.png) | Successful read-only listing, optional persistence and failure recovery | Approved visual direction; write access remains untested |
| [17 — Bookmarks](17-polished-bookmarks.png) | Returning-user chooser and separate folder/name bookmark settings | Approved visual direction; bookmark metadata is separate from credentials |
| [18 — Compact square logo](18-compact-square-logo.png) | Small square-tile logo animation, normalized pixel-icon gutters, no ambient binary digits | Approved correction after user rejected inconsistent glyph sizes/padding and clarified that bits meant squares |

## Known mockup inconsistencies

- Dates, counts, percentages and shortcuts in generated images are illustrative, sometimes inconsistent. File count is not byte progress.
- Board 03 has an inspector timestamp mismatch and conflicting hide shortcuts. Its job status during the conflict is ambiguous; real jobs need individual states.
- Board 04 includes an irrelevant remote-folder preview explanation despite local-file focus. Availability reasons must match the actual action and selection.
- Some backgrounds retain a selection highlight behind a modal. Modal input ownership and active focus must be unambiguous in the real app.
- Sample running transfers do not mean the first demo slice implements transfers. Unsupported actions must explain their unavailability, never simulate success.
- Fonts/icons and exact colors must be checked in an actual terminal, with a portable icon-free fallback.
- Boards 10–11 illustrate authentication choices, not verified provider sign-in capabilities. Azure CLI sign-in requires installed/authenticated Azure CLI; the current implementation must not invent a browser login. Listing success never proves write permissions. FTP password obscuring is reversible, not encryption; persistence must be explicit.
- Board 12 shows remote rows behind a “Not connected” label; that is a generated inconsistency. MCP remains a client-launched stdio process, not a background HTTP service. Policy changes require restarting the MCP client/server; interactive credentials never create grants.
- Boards 13–17 were generated with Painter using board 13 as the shared style reference (board 13 uses original board 07 and the rejected actual preview). They are not actual terminal renders. Preserve spacing, hierarchy and intent, but render icons with terminal-supported glyphs or text fallback. Board 17's `~` is display shorthand, not a promise that typed paths currently expand it. Board 16's combined Back/Edit label denotes one action returning to details, not two independently focusable controls.
- Board 18 uses board 13 and the actual startup capture as references. Animation frames below the screen illustrate the proposed small logo loop, not extra controls. Keep the actual logo's footprint fixed and small; any implementation must verify square proportions and identical action-icon gutters in a real terminal rather than assuming arbitrary Unicode glyphs match the mockup.

The earlier `workspace.html` is an initial static sketch, retained for history but superseded by these boards. Do not treat it as a live preview of the application.

# Mockup reference library

These six boards preserve the visual exploration in chronological order. They are generated design references, not implemented screenshots, passing tests, exact theme imports, or promises of every action shown. **Tandem remains an unapproved working name.**

## Current direction

Use the full-height, independently collapsible sidebar layout from board 04 and familiar coding-theme palettes from board 06. The two central file panes are transfer endpoints; the transfer drawer occupies only the center, never the sidebars. Ctrl+P is the primary action palette. Confirmation modals are separate from choosing an action. The product and program-design documents take precedence over accidental generated labels, figures or shortcuts.

| Board | Exploration | Status |
| --- | --- | --- |
| [01 — Dual-pane alternatives](01-dual-pane-alternatives.png) | Classic commander, transfer drawer, action preview | Early exploration, superseded by sidebar studies |
| [02 — Sidebar alternatives](02-sidebar-alternatives.png) | LazyGit numbered panels, Amp workspace tree, Neovim workbench | Influences, not three features to implement |
| [03 — Inspector and confirmation](03-inspector-and-confirmation.png) | Both sidebars open, both hidden, centered conflict modal | Confirms collapsibility and modal direction |
| [04 — Command palette](04-command-palette.png) | Full-height sidebars, fuzzy copy search, preview actions | Current layout and palette reference |
| [05 — Dark, warm and light themes](05-dark-warm-light-themes.png) | Night, Ember, Paper | Evidence that layout should work independently of theme |
| [06 — Coding-theme studies](06-coding-theme-studies.png) | Catppuccin Mocha, Tokyo Night, Gruvbox Dark, Dracula | Latest approved visual direction; palette-inspired only |

## Known mockup inconsistencies

- Dates, counts, percentages and shortcuts in generated images are illustrative, sometimes inconsistent. File count is not byte progress.
- Board 03 has an inspector timestamp mismatch and conflicting hide shortcuts. Its job status during the conflict is ambiguous; real jobs need individual states.
- Board 04 includes an irrelevant remote-folder preview explanation despite local-file focus. Availability reasons must match the actual action and selection.
- Some backgrounds retain a selection highlight behind a modal. Modal input ownership and active focus must be unambiguous in the real app.
- Sample running transfers do not mean the first demo slice implements transfers. Unsupported actions must explain their unavailability, never simulate success.
- Fonts/icons and exact colors must be checked in an actual terminal, with a portable icon-free fallback.

The earlier `workspace.html` is an initial static sketch, retained for history but superseded by these boards. Do not treat it as a live preview of the application.

# Service Bus Explorer mockup library

All PNGs are Painter-generated design references with fictional offline sample data. They are not working screenshots, Azure connectivity evidence, tested keyboard interactions or exact theme implementations. Images were inspected during the source conversation; generated copy, hierarchy and icons remain illustrative. The product document takes precedence over accidental image details.

## Latest dashboard direction

Use expanded collapsible navigation by default (board G). Keep grid/table selection independent of sidebar state. Grid targets 12 visible boxes, not a 12-queue configuration limit. Table fits more rows. Both show green queued and red DLQ counts with labels; edit watched entities only behind Settings.

| Reference | Purpose |
| --- | --- |
| [G — Expanded sidebar](g-dashboard-expanded-sidebar-themes.png) | Latest default: 12 boxes plus full-height navigation, dark/warm/light |
| [E — Grid, sidebar collapsed](e-dashboard-grid12-themes.png) | 3×4 boxes using more screen width, dark/warm/light |
| [F — Dense table](f-dashboard-table-themes.png) | 24 visible rows of a sample 48-queue watchlist, dark/warm/light |
| [A — Harbor](a-harbor-themes.png) | Entity tree, first-class Active/DLQ tabs, inspector, dark/warm/light |
| [B — Orbit](b-orbit-themes.png) | Ctrl+P resend/delete/peek palette, dark/warm/light |
| [C — Forge](c-forge-themes.png) | Resend review: scope, duplicate risk, separate cleanup, dark/warm/light |

## Preserved exploration history

| Reference | Status |
| --- | --- |
| [Initial Harbor](a-harbor.png) | Original Catppuccin-inspired three-pane browsing |
| [Initial Orbit](b-orbit.png) | Original Tokyo Night receive palette; fuzzy highlights approximate |
| [Initial Forge](c-forge.png) | Original Gruvbox dead-letter confirmation; corrected message-lock wording |
| [Initial dashboard table](d-lookout-themes.png) | Early watchlist editor on dashboard; inline editor superseded |
| [Four-box dashboard](d-lookout-boxes-themes.png) | Early box layout; superseded by 12 visible boxes and Settings-only editing |

## Review caveats

- Theme sheets contain dark, warm and light variants from top to bottom. Colors are inspirations, not imports from coding-theme configuration files.
- Original dashboard variants with inline editors are no longer the intended default. Minimal dashboard content does not mean permanently removing global navigation.
- Board G generates separate Preferences and Settings entries; whether these are combined remains open.
- Forge's light modal contrast and competing Cancel/Resend emphasis need refinement. Subscription indentation in generated images is not an Azure hierarchy specification.
- Some earlier cards have stray shortcut glyphs. Exact shortcuts, icons, focus ownership and layout capacity require real-terminal validation.
- Sample counts, timestamps and defaults do not establish operational guarantees. Resend is a send-copy workflow, not an atomic move.

## Provenance

Generated in https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1 using Painter prompts and targeted image-reference corrections. Saved via `amp files get`, inspected with `view_media`, then copied into this independent project directory without modifying LazyFTP assets. Reuse the saved boards for continuation; no live preview is provided.

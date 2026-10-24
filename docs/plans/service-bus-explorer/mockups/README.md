# Service Bus Explorer mockup library

All PNGs are Painter-generated design references with fictional offline sample data. They are not working screenshots, Azure connectivity evidence, tested keyboard interactions or exact theme implementations. Images were inspected during the source conversation; generated copy, hierarchy and icons remain illustrative. The product document takes precedence over accidental image details.

## Button-like actions — new exploration, 2026-10-24

Andreas requested a nicer UI with actions that almost look like buttons after
Slice 4 completion. These three alternatives explore action placement and visual
hierarchy, not new broker behavior. Andreas selected H (option A in the thread)
for its compact button styling and authorized implementation on 2026-10-24.
All use padded, bordered terminal controls, a distinct focused Cancel state,
outlined resend, separate destructive action and cleanup unchecked by default.
Keyboard focus must remain visible without color; exact cell geometry and all
theme/compact variants require real-terminal validation, not mockup inspection.

| Reference | Direction and trade-off |
| --- | --- |
| [H — Docked action bar](h-actions-docked.png) | Browse toolbar plus full-width bottom review controls; best body width and natural compact-layout fallback. |
| [I — Focused review dialog](i-actions-dialog.png) | Review sits over inactive browsing context; explicit separate typed-delete state, but adds overlay/navigation complexity. |
| [J — Action rail](j-actions-rail.png) | Strongest button affordance and clearest action hierarchy beside the held body; consumes width and needs a stacked/footer fallback at 80 columns. |

Selected direction: H's compact horizontal controls, not J's action rail.
The initial implementation keeps existing shortcut-driven browsing and review
keyboard order; typed delete stays in the action bar rather than adding a modal.
Delete still requires typing `delete`; ellipsis controls propose opening that
confirmation rather than deleting immediately. Receiving next is never a peek
and may acquire a different message from the highlighted preview. Navigation
remains held while acquiring/reviewing/releasing. Generated sidebar labels,
sample counts, dates and incidental preview IDs are not new product contracts.
The layouts deliberately leave space for long message content; large-screen
blank space is not a promise that all content fits without scrolling.

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

H–J were generated and refined in
https://ampcode.com/threads/T-01a16e75-df92-720e-b3c2-4052f34bb7f5
with Painter, using actual Slice 4 terminal screenshots plus earlier boards as
references. Inspected and corrected misleading receive/peek wording, invented
mutation hotkeys, background lock metadata and queue/topic hierarchy. Saved via
`amp files get`. They remain design references, not test evidence.

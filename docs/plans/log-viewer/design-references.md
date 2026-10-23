# Painter reference library

These are inspected static design studies, not implemented terminal screenshots,
interaction recordings, passing tests or measured performance. All events and
counts are fictional. SIGNAL is an unapproved working label. The product proposal
overrides accidental generated wording, icons and shortcuts.

## Boards

| Board | Purpose | Status |
| --- | --- | --- |
| [01 — Layout study](../../../.amp/in/artifacts/log-viewer/01-layout-study.png) | A: stream workbench; B: focus reader; C: context drawer | Gate 1 approved: A default, B/C as modes; not a pixel-level specification |
| [02 — Stream workbench](../../../.amp/in/artifacts/log-viewer/02-stream-workbench.png) | App keyword versus selected app; source search versus local find; event-focused inspector | Default-layout reference; stack content superseded by 04 |
| [03 — Palette and context](../../../.amp/in/artifacts/log-viewer/03-palette-and-context.png) | Ctrl+P owns input; context visibly relaxes text/level while preserving security/source/app/instance | Proposed interaction states, not executable |
| [04 — Inspector yank](../../../.amp/in/artifacts/log-viewer/04-inspector-yank.png) | Inspector focus; y value / Y key=value; provider direction | Latest response to Andreas's field-yank/provider request |

Default state first, followed by inspector focus and action/context states:

![Default stream workbench](../../../.amp/in/artifacts/log-viewer/02-stream-workbench.png)

![Inspector field focus and yank](../../../.amp/in/artifacts/log-viewer/04-inspector-yank.png)

![Command palette and bounded event context](../../../.amp/in/artifacts/log-viewer/03-palette-and-context.png)

## Provenance and production method

Read the existing LazyFTP mockup index and `internal/theme/theme.go`; inspected
LazyFTP board 04 and Service Bus expanded-sidebar board G. Read the historical
design production method from the commits and their Amp threads:

- [LazyFTP design history](https://ampcode.com/threads/T-01a164db-4654-720f-86cf-31f43a7d5cc8): board 04 full-height-sidebar and palette baseline; board 06 familiar coding-theme studies. Painter-generated raster references, no theme-file imports.
- [Service Bus design history](https://ampcode.com/threads/T-01a16a8a-2a84-7615-ae4e-5cd2d7fd57f1): Lookout/Harbor/Orbit language, collapsible navigation, explicit focus and safe actions. Reused Painter image-to-image refinement, not a fabricated runtime screenshot workflow.

All generations used Painter with opaque backgrounds, explicit monospaced
typography, square terminal borders, no browser chrome, and this existing Midnight
semantic palette: background `#0b1423`, surface `#101d2f`, foreground `#dce8f4`,
muted `#91a6bb`, border `#4b6a89`, focus `#65c7f7`, selected row `#8af0ce` on
`#071a24`, warning `#efc46f`, error `#f47d91`. These are prompt inputs, not measured
pixel values or proof of terminal contrast. Actual returned sizes differ from
requested dimensions.

1. Initial three-layout comparison used LazyFTP `04-command-palette.png` and
   `06-coding-theme-studies.png` as image inputs. It accidentally showed INFO/non-
   matching messages behind WARN+/timeout filters. Refined that output with Painter
   to ALL/no text, subdued current source, and sample-data labels. Only the corrected
   1086×1448 image is retained as board 01.
2. Board 02 (1568×1003) used corrected 01 as input, enlarged the workbench, separated
   app-name keyword from exact selection, and required every row to match WARN+
   and timeout. Added redacted raw view and no-grants MCP label.
3. Board 03 (1221×1289) used 02 as input, dimmed its background behind Ctrl+P, and
   contrasted the original matching results with INFO context around the anchor.
4. Board 04 (1568×1003) used 02 as input after Andreas requested field yank and
   provider abstraction. Moved bright focus to the inspector, added y/Y feedback,
   marked other providers planned, and replaced the mixed-language stack content.

Outputs were saved with `amp files get <attachment-url> -o <path>` and inspected
using `view_media`. No manual raster edits, fixture rendering or application
implementation were used. The rejected first comparison was removed.

Original corrected Painter attachments (useful if an ignored artifact directory
is absent in another checkout):

- [01 layout](https://ampcode.com/user-content/attachments/6137b5070f10f7269a783278796ee359b62ba14d8b1abbce84eb00966b731a0b-file.png)
- [02 workbench](https://ampcode.com/user-content/attachments/f9b2c264aa91736277409734408d1204878f8b394f9a35a664148aa9c89a4b6b-file.png)
- [03 palette/context](https://ampcode.com/user-content/attachments/8f9a623c90a532e06092f59a8663cee51dafa7c4f2cb9f1e7d2b650e7e3ccd14-file.png)
- [04 field yank](https://ampcode.com/user-content/attachments/aa8976c3142d9bf74c1283d4b5b94ac3da54e6b0c7891fa9494ea07e7ae492c1-file.png)

## Inspection findings and limits

- Corrected 01 consistently shows ALL/no message filter. A's stack extends beyond
  its visible inspector and needs scrolling in a real UI. B's icon-only rail needs
  keyboard help/text fallback. C is illustrative, not an optimized density spec.
- 02 has readable 12-row filters, one focused event and matching inspector, but
  generated Java exception text mixed with Go frames. Use 04 for stack direction;
  real structured content must retain its true source, not imitate this sample.
- 03 has only palette focus in state A and only context focus in state B. These
  are separate states, so their separate highlights are intentional. Modal
  obscuring underlying results is also intentional. Service/app wording varies;
  the policy is consistently source + app/service + instance + security scope.
- 04 visibly moves focus away from the log anchor to the selected field and shows
  redaction and y/Y hints. Actual field rows and copy feedback must show the exact
  same native path (`trace.id` here); generated typography is not authority for
  renaming or flattening fields. “Yanked” is illustrative, not evidence that any
  clipboard operation has run.
- No claimed clipboard, keyboard, streaming, authentication, provider or MCP
  behavior was executed. Those await implementation gates and real verification.

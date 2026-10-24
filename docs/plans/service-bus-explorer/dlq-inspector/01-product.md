# Product: One DLQ inspector, precise copying

Gate 1 approved · 2026-10-24 · user also authorized remaining gates without pauses.

## Problem

“I already see the dead-letter message on the right. Enter should let me work
there, not open the same body again. I need to copy an ID or a few payload lines
without losing my place or accidentally resending anything.”

## Success metric

Proposed target: at least 9 of 10 representative developer task trials complete
“select a DLQ message, copy its identifier, copy two payload lines, return to the
same list row” within 30 seconds, without an unintended broker operation. Measure
with a fixed sample payload at both 160×48 and 80×24; this is a future usability
target, not a measured result. Zero unintended sends is a release requirement.

## Announcement — the blog post before the feature

Read, select and copy messages in one place. Enter now focuses the message
inspector, with Vim-style movement and selection for copying just the content
you need. Persistent action hints make Back and Resend easy to find without
interrupting your reading. Resend still asks you to review the exact message,
destination and original-retention choice before anything is sent.

## Recommendation

**A: local inspector actions**, rather than a workspace-wide action dock.
The focused inspector owns its body, copy modes, feedback and resend entry point.
Remove the redundant selected-message reading screen, not the final safety review.
Keep the existing visual language: square terminal borders, compact horizontal
controls, theme colors and text-based focus markers.

## Focus and reading

- In the message list, Enter focuses the selected message's inspector. It does
  not acquire a lock, receive, send, or switch to a duplicate reading screen.
  Apply this reading behavior to Active messages too; only DLQ exposes resend.
- The passive inspector follows list selection. On entry, the text cursor starts
  at the first body character; identity/reason remain visible above it. Returning
  to the same message restores its cursor and scroll. No auto-refresh changes
  content under an active selection; show “refresh available” instead.
- While inspector text has focus, the selected list row is pinned. Movement and
  scrolling never choose another message. New row/resource/page selection clears
  the previous cursor selection. Preserve the same message through resize.
- Tab/Shift+Tab traverse list → inspector text → Back → Resend → visible
  navigation → list (reverse in the other direction). Disabled actions are skipped.
  Enter on text does nothing; only a visibly focused action activates.
- Esc clears visual selection first; next Esc returns to the message list; only
  Esc in the list leaves the message browser. “Back” always means return to list
  in normal reading. In resend configuration/review it means return one step.
- Enter temporarily reveals a hidden inspector without changing saved preferences.
  Returning to the list restores the prior visibility preference.
- Side-by-side needs at least 40 list columns and 54 inspector columns, plus
  borders. Hide navigation first if necessary. With less space, Enter makes the
  **same inspector** fill the content area, not a new selected-message screen.
  The header retains queue, DLQ, message identity and position, e.g. 2/12.
- At 80×24, keep a short identity/reason header, scrolling body, mode line and
  two-line action/help strip. At 60×16 reading/copying remain possible; resend
  confirmation requires at least 80×24. Below 60×16 show resize guidance and exit.

## Copy contract and keys

The inspector is read-only. This is a small, documented Vim subset, not an editor.
The selectable document contains metadata, body and application properties;
section headings and terminal decorations are not selectable. Body uses its
original text, not reformatted JSON. Metadata is displayed as labelled text;
line-yank on metadata copies that labelled line; use character selection to omit
the label. Application properties have a clearly labelled readable representation.

| Context/key | Result |
| --- | --- |
| List: j/k or arrows | Select a message, as today |
| Inspector: h/j/k/l or arrows | Move text cursor; keep cursor visible |
| Inspector: w/b/e | Next word / previous word / word end |
| Inspector: 0/$, Home/End | Logical line start/end |
| Inspector: gg/G | Document start/end |
| Inspector: PgUp/PgDn; Ctrl+U/D | Page / half-page movement and scrolling |
| Inspector: yy | Yank entire logical line, including its source newline if present |
| Inspector: v | Character selection anchored at cursor, inclusive endpoints |
| Inspector: V | Visual-line selection of whole logical lines |
| Visual: movement then y | Yank selected characters/lines; return to normal on success |
| Visual: v/V | Same key exits; other key switches selection kind, keeping anchor |
| Inspector: Y | Copy full message body, not only visible rows or selected text |
| Inspector: y alone | Pending operator, show “y… press y for line”; Esc cancels |
| Inspector: ? | Context help; Esc closes help without leaving inspector |
| Inspector normal: Ctrl+R | Open resend configuration, never send immediately |
| Ctrl+P | Actions, including Copy selection / Copy line / Copy full body / Copy message report |

Logical lines, not wrapped screen rows, determine yy and V. j/k move one logical
line, preserving the preferred column; wrapped continuation rows carry a distinct
marker, not a new line number. Selected off-screen characters remain selected.
Show logical line/column, selected character or line count, visible range and
total line count. A block cursor and VISUAL CHAR / VISUAL LINE labels supplement
color. Soft wrap is on; copying omits soft wraps, gutters, borders and cursor.
Tabs, wide characters, emoji and combining characters must remain intact.
Linewise copying preserves source newlines, including whether the last line has
one. Character selection includes source newlines only where selection crosses them.

**Full body** means all original text, with no metadata. **Copy message report**
means a labelled human-readable report containing metadata, full body and
application properties; it is not a lossless broker export or a resend payload.
Explicitly distinguish these commands so users can paste valid JSON with Y.
For binary/non-text bodies, disable text yank with an explanation; binary export
is out of scope. Empty bodies report “Body is empty; clipboard unchanged.”

If the preview is truncated, mark it prominently. Selection/yy can copy the
available preview with feedback that says “preview fragment”. Y and Copy message
report first offer a read-only “Load full message for copy?” action; they must
never report full-copy success from a partial preview. Keep current content on
cancel/failure; message no longer available means no clipboard replacement.
Show size and full/preview state before copying; never silently trim output.

## Keyboard ownership and discoverability

- Preserve Ctrl+P, Ctrl+C safe quit, list pagination n/p, refresh r and Active/DLQ
  switching [/] in the list. In inspector text, b means word-back, not sidebar;
  use the palette to toggle navigation. n/p/r/[ / ] do not navigate or reload
  from text. List and text help always state their current meanings.
- No Vim insert/delete/change commands, counts, registers, macros, search or
  operator-motion grammar in this proposal. Unsupported keys show brief help,
  never mutate content or fall through to message actions. q remains quit,
  except in palette/queue-filter text fields where it is literal input.
- While visually selecting, Ctrl+R does not open resend: say “Esc clears selection
  before resend”. Tab clears selection and moves focus; palette Copy selection
  preserves the selection until success/failure. A palette closes with Esc back
  to the same mode, cursor and viewport. Input fields own their printable keys.
- Persistent footer advertises yy / v / V / Y / ? / Back / Resend. Help expands
  the complete subset and conflicts, accessible through Ctrl+P as well as ?.
  Non-Vim users get arrows, Home/End, Page keys and named Copy actions.
- Use text labels, not icons/color alone. Focus has a named pane plus a leading
  marker and visible cursor. Support all existing themes and ASCII borders.
  Avoid animated/transient-only feedback. Terminal screen-reader usability must
  be checked before claiming accessibility; HTML mockups cannot establish it.

## Clipboard feedback and privacy

- Confirm scope and size, e.g. “Copied 15 characters · body”, “Copied 2 lines ·
  body”, or “Copied full body · 168 bytes”. Feedback remains until the next
  user command; never echo copied sensitive content into status/history/logs.
- Preserve cursor/scroll after yank. A successful yank exits visual mode; a
  failed yank retains its selection so the user can retry or use terminal copy.
- “Clipboard unavailable; selection kept. Use terminal copy; ? for help.” Do
  not falsely show success. If the terminal can only acknowledge a request,
  say “Copy requested; terminal confirmation unavailable”, not “Copied”.
- Terminal clipboard access and remote-session behavior need later feasibility
  work. Do not read the clipboard, send payloads to third parties, persist copied
  content, or trigger clipboard writes from message content/escape sequences.
- Manual terminal selection is an explicit fallback, not guaranteed to exclude
  gutters/wraps. Full-message copy never silently degrades to the visible region.

## Resend stays safe, within the inspector

1. The pinned Resend to queue… action (Ctrl+R or palette) opens configuration
   **in the inspector**. Queue DLQs are fixed to their source queue; subscription
   DLQs say Send copy to… and require explicit same-namespace queue selection.
   Do not reintroduce a queue picker for queue DLQs or silently resend to a topic.
2. Configuration replaces only the inspector's reading area, not the workspace.
   Show exact source/message identity, destination, new-copy/duplicate warning,
   unchecked “Delete original after confirmed resend”, and Cancel / Review resend.
   Cancel is focused initially. Reading/copying never starts cleanup acquisition.
3. Turning cleanup on exposes the existing warning before preparation: selected
   original search up to 100 messages / 5 seconds, other messages may lock,
   releases can increase delivery counts, additional release time may be needed.
4. Review prepares the full exact message and freezes identity, destination and
   cleanup. Show the outgoing body once in this inspector's review state, plus
   new message ID, retained/deleted-original consequence and lock deadline if held.
   Show any material preview-to-full difference before final confirmation.
   Cancel is default again; a distinct Resend copy action requires explicit focus
   and Enter. Repeated Enter on Cancel never sends. No y/yes confirmation.
5. Keep action/identity/consequence visible. If they cannot all fit, use a
   full-width inspector review. A separate read-only body subview cannot submit a
   hidden confirmation. Under 80×24 disable confirmation, retain Esc cancellation.
6. Back to edit releases any held lock first; Cancel releases it and returns to
   reading after the outcome is known. Show releasing/pending states; cancellation
   after submission is not rollback. While sending, block repeated submissions
   and row changes. Do not imply app-level exactly-once or atomic move semantics.
7. Return outcome inside the inspector: confirmed + original retained; confirmed +
   original deleted; confirmed + cleanup unknown; rejected; or send unknown with
   original retained and “Check destination before retrying”. Show lock-release
   uncertainty separately. Outcome cannot send again. Explicit Back returns to
   reading; do not silently select or substitute a neighboring DLQ message.

The separately named Receive next DLQ flow remains separate and clearly says it
may not act on the highlighted peek row. Standalone deletion is not added here.

## Screens

- `mockups/inspector.html`: wide normal focus, character selection, line selection
  and copy success, shown as static states of the same screen.
- `mockups/compact.html`: 80×24 full-width inspector and unavailable-clipboard state.
- `mockups/resend.html`: configuration, final Cancel-default review and unknown-send
  outcome, shown as static states with the list still in place.
- `mockups/index.html`: review summary, before/after flow, binding reference,
  recommendation, alternatives and approval request.

All sample content is synthetic and based on the offline Orders example. HTML
is throwaway review material, not application implementation or terminal evidence.

## Acceptance scenarios for product review

- Enter on row 2 focuses reading without a broker mutation; Esc returns to row 2.
- v then movement can copy an identifier without quotes; V then j then y copies
  two complete source lines with no gutters. yy on a wrapped line copies it once.
- Y copies all body text even while scrolled to the middle; preview truncation
  and unavailable full text cannot silently yield a partial “full” copy.
- Resizing in visual mode preserves the message and logical range. Switching to
  another message clears that range. Returning from help restores it.
- Clipboard failure keeps the selected range; copy feedback contains no payload.
- Queue resend needs no picker; subscription copy needs an explicit queue. Cancel
  remains default at configuration and final review. Visual y never confirms send.
- Cleanup warning/lock release and unknown/partial outcomes remain explicit.

## Original product-gate boundaries

At the product gate, architecture, implementation plan and clipboard transport
selection were deferred to the later gates, now recorded alongside this document.
Bulk resend, an editor, binary export, LazyFTP, log viewer, cloud authentication
and MCP remain out of scope. Terminal screen-reader behavior remains unverified.

## Approval

User approved the shown mockup and explicitly authorized implementation and full
verification through the remaining workflow without further approval pauses.
Shipping still requires a separate user request.

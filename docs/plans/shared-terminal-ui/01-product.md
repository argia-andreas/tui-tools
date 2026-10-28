# Product: One interaction language, three independent workspaces

## Problem

People should be able to click actions, navigate, discover commands and leave
temporary surfaces in the same recognizable way in all three terminal tools.
Today that promise is not met: only the file manager supports mouse clicking,
and action controls, help and palette behavior differ. Sharing invisible utilities
alone would not solve the problem. The tools need common reusable interactions,
not identical layouts: moving files, reviewing messages and investigating logs
must retain the safety rules and language of each job.

## Success metric

Primary proposed target: at least 90% uncoached completion of “select a row,
open its detail, find an action, understand why one is unavailable, cancel, and
resume” in each tool, separately with mouse and keyboard. Use five people × three
tools × two input methods, counterbalance task order, and measure the same tasks
before migration. These are targets, not measured results.

Acceptance requires mouse clicking for every visible action control, navigation
entry, palette result, and applicable dialog/editor control in all three tools;
disabled/hidden controls never run. Every clickable action must have a keyboard
route with the same meaning. Zero duplicate actions from a press/release pair,
zero background actions from a dismissed surface, and zero bypassed safety checks.
Measure behavior and appearance against the shared reference states, not merely
whether common code exists. Developer savings are secondary measures.

## Announcement — the blog post before the feature

All three terminal tools now speak the same mouse-and-keyboard language.
Click a clearly labelled action, search the same familiar command palette, or
use the equivalent keyboard path. Selecting an item is distinct from opening it,
and temporary surfaces clearly show where input goes and how to return. Layouts
still fit each job, and every file or message operation still gets its own safety
checks and review rather than a generic shortcut around them.

## Screens

Three self-contained plain HTML screen studies (fictional data, not running apps):

- `mockups/lazyftp.html`: independent file panes, shared clickable controls,
  palette, cancel-default deletion surface and compact state.
- `mockups/service-bus.html`: resources/messages/reader, same controls/palette,
  shared surface chrome around an app-owned docked review, compact state.
- `mockups/log-viewer.html`: workspace/logs/fields, same controls/palette,
  shared draft-editor chrome and compact state.

Common reference: compact square controls, clear text focus marker, identical
palette structure and cancel-first surface footer. A roomier component treatment
was considered; compact is recommended to preserve terminal content space. These
studies propose interaction coherence, not replacement app layouts. The separate
style-board image is illustrative only; this written contract and HTML studies
override invented labels or actions in generated art.

## Scope for approval

- Mouse clicking in all three tools; keyboard operation remains complete.
- One shared action-button language: default, focused, disabled/reason, busy and
  dangerous states; mouse and keyboard request the same named action.
- One palette: Ctrl+P opens/closes it when permitted; arrows/Tab choose with
  clamped ends; Enter or row click requests the selected action; Esc/outside click
  closes without click-through; same query, shortcut, reason and help placement.
- One surface language for dialogs and editors: title, visible close/cancel,
  inline validation, cancel-first confirmations, clear busy state and return path.
  A blocked cancellation stays blocked for pointer, close button and keyboard.
- One navigation language: click a pane to focus; click a data row to select and
  update its preview; Enter or labelled Open/Inspect activates. Clicking an explicit
  navigation destination opens it. Tab/Shift+Tab move through visible controls and
  panes; hidden content never receives input. No double-click timing dependency.
- Actions and Help controls everywhere; F1 opens context help without losing drafts
  or review state. `?` remains text in inputs and context help outside text entry.
- Preserve layouts, domain actions, confirmations/authorization, safe output and
  clipboard truth. A destructive action in the palette opens review, not execution.

## Not approved by this gate

Gate 1 would approve deliberate interaction changes, including the file manager's
current row-click-open becoming select-first, palette-key normalization and new
mouse/help routes in the other tools. It does not authorize implementation.
No common workspace layout, broad framework, shared inspector model, domain
workflow rewrite, clipboard policy change, drag/drop, arbitrary text mouse
selection, or accessibility certification is proposed. Architecture, API design,
and executable slices still require separate Gate 2–4 approval.

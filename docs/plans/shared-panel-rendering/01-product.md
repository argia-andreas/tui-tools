# Product: Shared panel rendering

## Problem

Maintainers currently have to understand and update several versions of the
same ordinary panel presentation work. Fixing how borders, titles, clipping,
or background padding are drawn can require repeating the fix across tools.
Users should not experience appearance or interaction changes just because
that maintenance work is consolidated.

## Success metric

Zero unintended visual or interaction differences in the agreed comparison
matrix across all four tools. Measure this with before/after rendered captures
and existing interaction checks covering focused and unfocused panels, narrow
and wide layouts, clipped long text, styled/selected content, supported themes,
and supported ASCII modes. Define the exact cases before implementation.
The maintenance outcome is one place to fix common ordinary-panel drawing
behavior instead of four separately maintained versions.

## Announcement — the blog post before the feature

The tools retain their familiar layouts and controls while ordinary panels
become easier to maintain consistently. Borders, titles, focus indicators, and
content continue to look as they did before. Each tool keeps its own workflow,
theme choices, and safety checks. This is a maintenance improvement, not a new
interaction model or a visual redesign.

## Screens

No new UI or redesigned screens. Existing tool screens are the visual contract;
before/after rendered captures will be used for comparison rather than creating
new mockups that could accidentally redefine the desired appearance.

## Scope decisions

- Cover ordinary workspace panel presentation in Transfer, Queue, Logs, and
  Streams.
- Preserve existing title decoration, focus appearance, clipping conventions,
  selected/styled content, background coverage, and supported ASCII appearance.
- Preserve panel placement, dimensions, responsive breakpoints, row visibility,
  pointer targets, keyboard behavior, and domain operations.
- Keep dialogs and editors' capture, close, cancellation, and confirmation
  behavior unchanged; ordinary panels do not acquire those behaviors.
- Exclude clipboard work, theme unification, general text-sanitization changes,
  inspector/workflow consolidation, and the other code quality findings.
- If consolidation requires changing a visible convention or produces more
  complexity than the duplication it removes, revisit scope before proceeding.

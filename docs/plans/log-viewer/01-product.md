# Product proposal: an exceptional terminal log workbench

Gate 1 approved by Andreas on 2026-10-23. SIGNAL is only a working label.

## Problem

“Let me find my application's logs, search the interesting strings, understand
what happened around an error, and copy the exact field I need—without fighting
the interface or switching between several web consoles.”

Andreas wants the best log reader he has used: keyboard-first, fast to orient in,
with the same visual family and Ctrl+P discovery as Tandem and Service Bus Explorer.
He specifically likes the right-hand inspector and requests field-level yank.
More than one log provider must fit the product, without sacrificing honesty.

## Success metric

Proposed, not measured: in five representative incident tasks, find the relevant
event, inspect its context and yank its trace ID in under 60 seconds on at least
four tasks. Measure from opening a configured source, using realistic data and a
keyboard only. Compare with Andreas's current workflow before fixing the target.

## Announcement

Open your logs and get straight to the event that matters. Narrow by application,
time and level, search the message, then read a stack trace without losing your
place. Inspect structured fields and yank exactly the value you need. Pause the
flow, see the events around an error, and return to following with one deliberate
action. Familiar terminal navigation and Ctrl+P make the powerful things easy to
discover, across several log sources without pretending they all work the same.

## Recommended experience

- **Stream first:** source/saved-view sidebar, central chronological log list,
  independent right inspector. Both side panels collapse. On smaller terminals,
  details take over the main area instead of squeezing three unreadable columns.
- **Apps first, not query syntax:** type `checkout` to narrow application names,
  then select `checkout-api` explicitly. Offer multi-select with the selected apps
  visible. A keyword is not silently treated as an exact app, or searched in the
  message. Incomplete app suggestions are labeled incomplete.
- **Two kinds of search:** message search runs against the selected source and
  time range; `/` finds literal strings within loaded, redacted events. Show the
  scope, case behavior and search mode. Never describe a token/phrase search as
  arbitrary substring search. Unsupported modes explain why, not silently change.
- **Useful time and severity:** Last 15m initially, quick presets and explicit
  absolute range; UTC is the unambiguous default, local display is optional and
  labeled (including offset for Stockholm daylight-saving transitions). Missing
  levels are Unknown, never INFO. WARN+ means WARN, ERROR and FATAL, not Unknown.
- **Read beautifully:** compact one-event rows, purposeful severity labels/colors,
  highlighted matches, wrapping on demand, selectable structured fields, redacted
  raw view. A multiline exception remains one inspectable event when the source
  supplies it that way; plain text is not magically reliable structured data.
- **Follow without fighting it:** indicate streaming versus polling, last refresh,
  ingestion-delay caveat and connection state. Pause freezes the view and stops
  collection, rather than retaining an unbounded hidden backlog. Resume attempts
  bounded catch-up and explicitly reports truncation or uncertain continuity.
- **Context without losing scope:** show a bounded window around the anchored
  event, ordinarily same service + instance. Clearly relax only text/level filters;
  preserve source and security scope. A separate “follow trace” action may span
  services only within the user's permitted scope. Context is not a complete trace.
- **Safe agent access:** optional MCP integration is off/no grants by default.
  A person explicitly permits sources and bounded reads; connecting an assistant
  must not automatically share every log source or grant administrative powers.

## Keyboard proposal

These are proposed bindings, not implemented or approved shortcuts. Ordinary
text fields own typing; shortcuts below apply only outside text input.

| Key | Action |
| --- | --- |
| Ctrl+P | Fuzzy, context-aware actions; unavailable actions explain why |
| Tab / Shift+Tab | Cycle visible panels; one bright focus owner |
| j/k or arrows; PgUp/PgDn | Move by event or inspector field; page the reader |
| Enter | Open event details; expand an object/long field when inspector owns focus |
| y | Yank selected field's displayed/redacted value in inspector |
| Y | Yank selected field as `key=value`; objects use deterministic JSON |
| /; n / N | Literal find in loaded events; next/previous match |
| f | Open source-search/filter editor (not a local find) |
| c | Show context for current event from the log list |
| Space | Pause/resume follow from the log list only |
| Esc | Close overlay or return from details; never secretly quit or clear filters |
| ? | Contextual keyboard help |

Yank must copy the full selected field, not its visually truncated prefix. An
object is explicitly selected before copying it; no implicit whole-event yank.
Feedback names the field and size without echoing its sensitive value. No success
toast if the clipboard path fails. Clipboard transport limitations are visible.

## Screens and design references

Painter replaces HTML sketches for this explicitly requested visual exploration.
These are fictional static images, not an interactive prototype. See
[design provenance and inspection](design-references.md).

1. Layout study: workbench, focus reader and context reader alternatives.
2. Workbench: app keyword, source message search, event and multiline stack.
3. Ctrl+P and context: action preview and visibly relaxed context filters.
4. Field-yank state: inspector focus, `y`/`Y`, multi-provider direction.

Recommend workbench as the default, with focus reader and context as modes, not
three different navigation systems. Preserve Midnight semantics, thin square
borders and full-height sidebars. Familiar theme variants can follow; no new
theme engine or promises of arbitrary editor-theme import.

## Initial target and deliberate exclusions

Start with the provider Andreas actually uses, once identified. If it is
Elasticsearch, recommend one configured, read-only ECS-style log source as the
initial integration. If it is Azure Monitor, start with the relevant workspace
table instead. Keep the provider boundary from day one; do not build all backends
simultaneously. Kubernetes and Loki are credible subsequent targets.

First product goal is one-source-at-a-time investigation, with backend search,
local find, inspector/yank, context, follow/pause and permissioned MCP reads.
Provider changes preserve useful preferences, but never silently reinterpret a
native query. No cross-provider global merge, dashboard builder, ingestion,
alerting, log deletion, cluster administration or automatic AI upload. A later
redacted incident bundle could be useful, but export is not an MVP prerequisite.

## Remaining source/provider questions

1. What is the sanitized hostname or product name behind `logs.insights…`? That
   clue does not identify the provider. No token, private query or log dump needed.
2. Which provider should follow the current one: Kubernetes or Loki (or another)?

Andreas approved the product direction and requested a fast local Elastic test
loop. Product approval does not identify the live provider or approve later gates.

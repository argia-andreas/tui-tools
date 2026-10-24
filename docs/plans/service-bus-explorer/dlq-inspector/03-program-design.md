# Program design: unified inspector

Authorized to proceed without further gate pauses on 2026-10-24.

## Files and contracts

- UI inspector.go: read-only selectable document, logical grapheme positions,
  wrapped viewport rendering, modes/commands, clipboard/full-load results.
- UI model.go/view.go/actions.go: focus precedence, responsive pane ownership,
  palette commands, contextual help; no fallthrough to list actions from text.
- UI selected.go/review.go/review_view.go: remove duplicate selected reading
  phase; retain existing mutation lifecycle and embed selected review in inspector.
- explorer.Store.PeekMessage(context.Context, Resource, bool, int64)
  (Message, error): exact full read, never locks/receives; missing means failure.
- azure/fixture: implement full read without changing existing preview contract.
- clipboard.Write(string) (confirmed bool, err error): native success is confirmed;
  OSC52 is unconfirmed request, failures have no payload in diagnostics.
- Existing public UI tests and PTY script: extend behavior checks and update only
  obsolete duplicate-screen/focus expectations. Keep receive-next safety tests.
- README: actual keys, copy scopes, clipboard prerequisites and limitations.

## Call stack

Key routing: active operation → palette/input → inspector modes → global controls
→ navigation/list. Modal help/load prompt consume keys before reader actions.
List Enter focuses. Tab traverses reader, Back, Resend, navigation and list.
Reader movement computes logical grapheme positions; view maps these into wrapped
screen rows. Clipboard requests carry immutable text and identity/generation;
no payloads in status. Full-load completion must match both sequence and ID.

## Test plan

- Public UI: list row pinned; Esc depth; Tab actions; hidden/compact inspector;
  metadata/body/properties copy; yy/v/V/Y exact strings including newline/Unicode;
  failure retains selection; stale completion cannot change newer selection.
- Full load: truncated preview asks first, exact original text or explicit failure,
  no mutation; late/cancelled results ignored; Active and DLQ coverage.
- Resend: same-queue destination, subscription picker, Cancel default, existing
  cleanup/release/unknown/single-flight behavior, compact hidden-confirm guards.
- PTY: real input, OSC52 decoded bytes independently asserted, safe navigation,
  compact resize, full review and existing native binary restart coverage.
- Render: all themes/ASCII at wide/compact/short sizes; inspect screenshot/recording.

## Least confident decisions

Remote clipboard cannot acknowledge receipt: explicitly report requested rather
than copied. Terminal accessibility and native non-Linux platforms require actual
target machines and are not proven by orb screenshots. Preserve unsupported-binary
and terminal-control safety rather than silently converting raw bytes.

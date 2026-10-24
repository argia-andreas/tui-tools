# Architecture: unified inspector

Authorized to proceed without further gate pauses on 2026-10-24.

## Fit

Keep the independent Bubble Tea application and existing broker ownership.
Inspector state owns a grapheme cursor, anchor, copy mode, viewport, help and
temporary visibility. Existing selected-message resend state owns configuration,
preparation, lock lifetime, final review and result; render these states inside
the message workspace rather than adding a second operation state machine.
Receive-next remains the separate full-screen workflow.

## Endpoints / data

No network endpoints or persisted data changes. Add a read-only Store.PeekMessage
operation for exact full-content lookup on Active or DLQ. Existing bounded page
previews and delivery snapshots retain their contract. Late lookups are ignored
after selection/page changes, cancellation or a new operation.

## Flow

List Enter → reveal/focus inspector → cursor/select → copy command → native local
clipboard or terminal OSC52 request → honest confirmed/requested/failed feedback.
Full copy of a truncated preview first asks to load exact full content read-only.
Resend action → existing configuration → exact preparation → Cancel-default
review → existing single-flight mutation/release → inspector outcome.

## External

Use existing uniseg grapheme support and OSC52 library; optional platform clipboard
tools, never clipboard reads. Remote sessions use terminal clipboard requests,
not a remote desktop clipboard. No credentials, production access or deployment.

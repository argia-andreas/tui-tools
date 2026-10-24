# Vertical slices and proof cases

User authorized all slices without intermediate pauses; no shipping.

1. Unified reader/cursor/focus through fixture, including normal/v/V/yy/Y public
   behavior. Baseline: current binary Enter opens separate SELECTED MESSAGE.
2. Native/OSC52 clipboard and exact full-content read; independent decoded text
   assertions, failure and truncation cases. No fake successful clipboard status.
3. Existing resend lifecycle in inspector with same source/destination, Cancel
   default and responsive safety. Keep receive-next as an adjacent control.
4. Full race/vet/Python/PTY suite, real disposable emulator if available, actual
   ttyd rendering and keyboard/pointer recording, compare with approved mockups.

## Proof cases fixed before implementation

- Baseline / after: select DLQ row 2 → Enter. Before: separate duplicate reading
  screen. After: same workspace, focused inspector, list row still 2.
- Copy ID exactly without quotes; two whole lines including original newlines;
  yy on a wrapped Unicode line; Y while scrolled returns full body. Native or
  OSC52 decoded bytes, not just a toast, establish output correctness.
- Selection + resize + help preserves range; failure keeps it; another row
  clears it. r/n/p/[ / ] and b cannot navigate away from inspector text.
- Truncated-preview full copy asks for load; cancellation/missing/stale results
  cannot write a partial or neighboring message. Binary refuses text copy.
- Safe resend exact row 2: original retained by default, optional exact cleanup,
  no neighbor removed. Repeated Enter/visual y cannot submit hidden operations.
- Control: Active browsing/paging, settings persistence, receive-next actual
  identity, lock expiry/release, ambiguous send and partial cleanup remain correct.
- Inspect wide, 80×24, 60×16, short-height, three themes and ASCII. Record actual
  keyboard focus/selection/resend, not simulated HTML state transitions.

No production writes, cloud operations, deployments, pushes or merges.

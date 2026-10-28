# Slice 7: engineering validation and acceptance limits

2026-10-28. **Engineering validated; manual/usability acceptance pending.**
All four design gates and slices 0–7 were explicitly approved. Changes remain
local and uncommitted; no push, deployment, live lab, broker, Elastic or remote
file operation was performed. Fixtures and private temporary files were used.

## Executed checks

From the repository root:

```sh
make check
./build
make test-terminal
go test ./apps/lazyftp/... ./apps/service-bus-explorer/... ./apps/log-viewer/... ./lib/terminalui/...
```

All passed on the final source. `make check` includes independent `GOWORK=off`
race tests and vet for the four modules, formatting, and ten Python lab unit
tests (not a running lab). Terminal suites execute actual binaries against
offline asymmetric fixtures, including mouse-on/off SBX, eight FTP demo/icon
variants plus private-local-file surfaces, and Logs search/Vim/output scenarios.
Full captured output is retained in `.amp/in/artifacts/slice7/verification.txt`.

## Findings fixed by this slice

- **Hidden operations after resize:** before the fix, Logs Enter could save a
  hidden search and SBX Space/`s` could change/save hidden settings below 60×16.
  The same PTY procedure on before/after binaries observed private files written
  before, absent after. Restoring 120×32 retained the draft/settings and explicit
  visible saving worked in both runs. FTP's hidden palette also accepted editing
  and activation below 60×12; its regression failed before and passed after.
  Boundary regressions use both width-one-below and height-one-below. Input gates
  retain safe cancellation and local quit policy; they do not gate async results.
- **Typed-delete prompt:** the SBX inventory promised field focus, but the prompt
  had no region. It now exposes a current fully-visible FieldRegion that selects
  the existing authorization owner. The regression failed before and passed after;
  nonexact text, release events and compact/stale pixels cannot delete. Exact
  `delete` still needs explicit final confirmation; the fixture loses exactly
  one DLQ message and no Active messages. Review expiry, identity, late release,
  UNKNOWN/partial outcomes and cancellation checks remain app-owned.
- **FTP mouse-off fallback:** default reporting remains enabled; the new
  launch-only `--mouse=false` emits none of the checked enable modes through exit.
  Keyboard-only chooser/Open, marks, palette/Help, cancellation and preview work;
  disposable local source bytes remain unchanged, with no remote connection.

The existing suites also cover consumed outside dismissal, no background input
leakage, unavailable action refusal, one activation per press/release, resize
geometry, Help preserving parent drafts/reviews, and lifecycle completion/expiry.
These are scenario checks, not a claim that every possible interleaving was run.

## Fallback, visuals and packaging

- Seven additional real-binary PTY cases: truecolor and NO_COLOR for each app,
  plus SBX ASCII. Keyboard palette captures literal `q/j/?`, Help returns to the
  retained query, Escape closes correctly and Ctrl+C exits cleanly. All cases
  use mouse=false and emit zero mouse enable sequences; NO_COLOR emits zero
  basic/extended color SGR sequences. SBX ASCII+NO_COLOR was additionally rendered
  and inspected. FTP icons-off does not promise a completely ASCII interface.
- Actual apps were rendered through browser-hosted ttyd, not static mockups.
  Inspected monochrome/ASCII workspaces/palettes, selected unavailability reason,
  Help, Logs resize notice and retained Save draft. The selected palette reason
  is complete in its bottom detail row; narrow per-row reason columns truncate
  intentionally. Earlier slices contain color, compact, modal and pointer evidence.
  The final keyboard clip uses `?` then Escape in FTP with mouse reporting off;
  F1 is independently exercised by the real-binary PTY suites.
- Initial screenshots captured ttyd's transient size indicator and an initial
  keyboard recording did not demonstrate the expected Help transitions. Those
  captures were rejected; the replacement clip uses `?` and waits for visible Help
  before continuing. Do not use discarded captures as evidence. Final media lives
  in the `slice7` artifact folder. A recaptured SBX browser terminal needed its
  WebGL addon disabled to remove oversized/cropped browser-host rendering; the
  fallback capture matches the independently checked PTY counts. This changes
  only the capture host, not the application renderer or source.
- FTP packaging cross-built Linux amd64/arm64 and Darwin amd64/arm64, including
  lazyftp-mcp, documentation, BUILD metadata and SHA256SUMS. All four checksums
  verified; extracted Linux amd64 `--help` exposes `-mouse` default true. Initial
  packages made before the flag/final input gates were discarded and rebuilt.
  Darwin/arm64 binaries were not executed natively; cross-build is not certification.
- Calculated RGB contrast across five FTP themes, three SBX themes and Logs:
  foreground/background, foreground/surface and selection pairs exceed 4.5:1.
  Existing FTP Dracula muted/surface is **1.94:1**. This palette limitation is
  recorded, not silently changed in a shared-interaction slice. Terminal palette
  overrides, dim rendering and screen-reader behavior were not certified.

## Measured benefits and costs

| Measure | Before | Final | Interpretation |
| --- | ---: | ---: | --- |
| Palette matcher/render/input implementations | 3 | 1 | All three apps consume the same implementation |
| Apps consuming buttons/palette/Help/Surface/navigation | 0 | 3 | Shared semantics, separate layouts and domain effects |
| Pointer routes synthesizing keyboard events | Present | None found | Semantic checked operations, not keyboard-grammar coupling |
| App-local PTY harnesses | 3 | 3 | Setup extraction deferred; no ≥50% reduction claim |
| FTP UI/theme/main CLI nonblank Go lines | 4,453 | 4,976 | Includes local adapters, not only deleted duplication |
| SBX UI/theme/main CLI nonblank Go lines | 2,948 | 3,696 | Includes checked review/reader/navigation adapters |
| Logs UI/theme/main CLI nonblank Go lines | 2,139 | 2,791 | Includes draft and query-safe editor adapters |
| Shared nonblank runtime Go lines | 0 | 1,028 | Seven source files; tests excluded |
| Total above | 9,540 | 12,491 | **+30.9%**, not net source reduction |

Source counts compare HEAD `9e5816319caa48f7455e20ac6a64020a5e3c3191` with the
final worktree, mapping old paths to relocated apps. Count every nonblank line
(including comments) in production `.go` files directly under `internal/ui`,
`internal/theme` and each main CLI directory, plus all shared production files;
exclude `_test.go`, generated binaries and backend packages. This is a transparent
development-cost proxy, not a complexity or performance benchmark. Most added
code implements previously absent pointer/focus behavior and explicit safety.
No before/after latency, user-effort or defect-rate improvement was measured.

The audit reviewed **33 FTP + 29 SBX + 20 Logs inventory rows**: these are grouped
control categories, not 82 individually executed controls. Each app's
`docs/ui-click-inventory.md` maps keyboard/pointer routes, visibility and evidence.
The reviewed mapping has no known missing in-scope target after the prompt fix;
it is not a measured 100% human completion rate. No peer-app production import or
domain/storage/broker/process effect was found in the shared library. The remaining
Logs KeyMsg construction splits local multi-rune input; it is not pointer dispatch.

## Acceptance still pending

1. Run the approved usability protocol: five people × three tools × both methods,
   counterbalanced with baseline first; target ≥90% uncoached task completion.
   Include selecting, inspecting, discovering an action/reason, cancel and resume.
2. Exercise native terminal, tmux and SSH; mouse-off native text selection,
   actual clipboard receipt/permissions, screen-reader announcements and focus.
3. Decide whether to adjust Dracula muted text in a separately scoped style change.
4. Measure maintenance/test-setup savings before claiming the developer-effort
   target met. A common PTY helper needs its own justified design, not a broad
   framework added to make a percentage look green.

The engineering slice is complete; these acceptance gaps are not authorization
for further implementation or release. App layouts, backend effects, editor/reader
grammars, clipboard policy and checked domain state machines remain local.

## Approved maintenance cleanup — 2026-10-28

Andreas requested a quality review of the added UI code, then authorized cleanup.
This is behavior-preserving maintenance, not a new feature or layout framework.

- FTP file rows/transfer drawer and Logs restore/field-picker loops now emit their
  regions while constructing displayed rows. Separate window/offset calculations
  were removed from their navigation/editor adapters.
- Surface input builds a specification/frame pair once per dispatch. Picker and
  wizard reuse their current pair; shared Surface input checks current geometry
  without repainting the body. No persistent frame cache or new public API.
- SBX reader lines and targets come from one frame; hint ID/label definitions emit
  both text and regions, replacing reverse parsing of styled hint sentences.
  Selection-mode Tab order remains Help → Copy → Clear, with a regression test.
- Each app now owns one minimum-workspace predicate, shared by its render/input/
  Help paths. Different app thresholds and SBX's stricter review minimum remain.
- Deferred: SBX review numeric/string focus unification, common PTY harness, and
  domain grammar/state-machine changes. Their risk is not justified by this cleanup.

Validation against a saved pre-cleanup source/binary baseline:

- `make check`, `./build` and `make test-terminal` passed across all modules/apps.
  These include race tests, vet, formatting, 10 Python lab-unit tests (no live lab),
  and all existing actual-binary offline mouse/keyboard PTY suites.
- A temporary differential sweep compared **34,940 public shared frame/input
  results**, including tiny dimensions, ASCII, focus and blocked cancellation.
  Before/after SHA256 was identical:
  `81226d151d16560d4114bd77c464f5328479fbe2b5353b03a8d2c72395da367d`.
- **31 actual-binary decoded frames** matched before/after (FTP 9, Logs 10, SBX 12),
  checking text, foreground/background, bold and reverse, through workspace/input
  sequences and 160×48 → 80×24 → below-minimum → 160×48 resizing. Live HH:MM:SS
  clock characters were normalized; all geometry and other cells were compared.
  This is representative equivalence evidence, not exhaustive state coverage.
- Rendered final fixtures inspected in ttyd/Chromium at 2×: FTP workspace and
  explicit row-select/Open interaction, Logs filter editor, SBX normal/visual
  reader hints. A real-pointer clip confirms selection does not open a directory
  until Open/Inspect is pressed. Captures/logs are under
  `.amp/in/artifacts/ui-cleanup/`. The first blank browser-session recording and
  transient size-overlay capture were rejected and replaced. Existing Logs
  default-background padding is unchanged; this maintenance does not restyle it.

This cleanup removes independently maintained algorithms, not large amounts of
source. Against the saved pre-cleanup tree: **+355/−272 physical Go lines**,
including 17 test lines. Under the nonblank runtime measure above, FTP 4,976,
SBX 3,710, Logs 2,790 and shared 1,072 total **12,548** (+57 versus slice 7).
Explicit geometry construction accounts for the small increase; there is no
measured speedup or claim that the original 30.9% source growth disappeared.
Changes remain local and uncommitted; manual acceptance gaps above are unchanged.

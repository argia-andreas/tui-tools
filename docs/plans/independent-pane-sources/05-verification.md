# Independent pane sources: verification

Implemented and verified in the feature thread on 2026-10-24 before shipping authorization. The user subsequently authorized committing, rebasing, full verification and pushing to main. Scope is the terminal app and its connection/bookmark support, documentation and acceptance tests. Existing MCP integration tests were run as regression coverage, not changed.

## Executed checks

- `make check`: formatting, vet and all Go tests passed.
- `go test -race ./...`: all packages passed.
- `make test-terminal`: real PTY smoke passed, including mouse/keyboard, palette, selection and resize.
- `make test-terminal-panes`: all eight transitions passed (left/right × initial local/remote × replacement local/remote). Opposite-pane rendered rows, selection and applied filter were retained. Boundaries passed: both detached, file-action guards, target/cancel flows, invalid-folder recovery, keyboard choices, 60×12 layout and explicit remote/remote refusal.
- The same pane acceptance against the unchanged baseline binary failed at the missing `[ Detach ]` control (exit 1).
- Reversed-pane FTP upload/download passed independent byte comparison of 4,370 bytes, with exact owned-file cleanup.
- `make test-terminal-live`: FTP, Azure Blob and S3 passed actual TUI upload/download, independent bytes, default Cancel, overwrite conflict handling and clean exit (33,044-byte fixture per provider).
- `make test-integration`: real local-lab storage, UI and MCP regression suites passed.
- Additional disposable real-PTY validation opened new session-only FTP connections in LEFT then RIGHT through the existing wizard, refreshed the earlier connection after the second snapshot, and confirmed persistent config and bookmarks were unchanged. Pointer Connect passed after waiting for the verified render to settle. The scratch validation created no retained test or credential file in the repository.
- `git diff --check`: passed.

## Inspected actual terminal UI

Screenshots and a moving-pointer interaction recording are in `.amp/in/artifacts/`, not the static proposal mockups:

- `panes-live-default.png`: local-left / FTP-right, both Detach controls.
- `panes-live-left-detached.png`, `panes-live-right-detached.png`, `panes-live-both-detached.png`: independent inline source choices.
- `panes-live-reversed.png`: FTP-left / local-right, correct source labels and listings.
- `panes-live-two-remotes.png`: two FTP panes, explicit remote/remote copy refusal, zero transfers.
- `panes-live-local-folder.png`, `panes-live-folder-error.png`: RIGHT folder prompt and invalid input with recoverable error.
- `panes-live-saved-picker.png`, `panes-live-saved-details.png`: LEFT target retained and no companion-local field.
- `panes-live-new-wizard.png`: LEFT target in the existing provider wizard.
- `panes-live-narrow.png`: actual terminal reports 60 columns × 12 rows; all three choices visible in each pane after the ttyd resize overlay disappeared.
- `panes-interaction.mp4` and `.loop.webp`: real moving pointer detaches each pane, chooses RIGHT local files, and opens LEFT saved selection. Inspected a frame contact sheet; no blank or stretched app frames.

The browser-terminal WebGL renderer initially produced stretched captures despite a correct terminal buffer. Disposing that browser-only renderer fixed the captures; no application workaround was introduced. All screenshots above are final inspected captures. Browser viewport used DPR 2. Static mockups remain design references only.

## Boundaries preserved

Remote/remote browsing is supported; copying remains deliberately unavailable. CLI startup still means local-left / remote-right. Detach is not credential erasure or server logout. Successful session snapshots stay owned until exit, preserving other panes and captured transfer endpoints. Source changes remain blocked during active transfer planning/execution. New-provider creation was exercised against real FTP; Blob/S3 use the shared target routing, unit coverage and existing real-provider transfer suites, not a claim of newly provisioned cloud accounts.

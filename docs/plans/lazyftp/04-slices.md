# Vertical slices

Status: approved 2026-10-23. User explicitly waived further approval gates and asked for an iterative working proof of concept. Build in runnable increments with executed checks and recorded evidence. Commit and push verified work to main under the user's existing authorization; do not deploy.

1. **Interactive tracer bullet:** runnable Go TUI over deterministic in-memory locations; dual-pane navigation/filter/selection, collapsible full-height sidebars, empty transfer drawer, Ctrl+P fuzzy action palette, theme/config switching and optional icons. Unsupported transfers/previews stay explicitly disabled. Exit: fast tests and PTY smoke pass, representative renders and interaction recording inspected, demo instructions work.
2. **Real browsing:** validate rclone structured listings/progress/cancellation, establish seeded local FTP and Azurite services with direct-process and Compose startup paths, replace fixture data with real listings for one happy path on each service. Exit: application navigates both services, integration checks verify known fixture contents, unavailable dependencies fail explicitly. Extend the approved program design before real-storage implementation.
3. **Single-file transfer:** copy either direction, source/destination preview, explicit skip/overwrite/rename conflicts and truthful success/failure. Exit: byte-verified round trips on both services, asymmetric preexisting files preserved without permission, conflict modal exercised in the actual terminal. Document any backend race limitations instead of promising unsupported safety.
4. **Folder jobs and recovery:** recursive work enumeration, queue with captured endpoints, accurate per-file outcomes, cancel and retry unfinished work. Exit: controlled interruption and recovery on both services; completed work remains complete, partial files are disclosed, switching connections never retargets jobs.
5. **Usable alpha:** explicit bounded text previews, broader naming/listing/terminal cases, helpful errors, packaging and contributor documentation. Exit: repeatable acceptance suite, measured feedback-loop timing, inspected representative themes/widths, and clear platform/emulator limitations.

## First slice boundaries

`make demo` must launch a genuinely interactive app without Docker, rclone, a cloud account or production files. It is not yet a file-transfer release. `make test` is the fast deterministic loop; `make test-terminal` exercises the actual executable; `make check` validates formatting, vet and tests. Service setup and transfer APIs belong to later slices, not a horizontal foundation built before anything runs.

Report runnable increments and invite feedback without requiring approval before continuing. Keep `00-status.md` current so a fresh session can resume from these documents.

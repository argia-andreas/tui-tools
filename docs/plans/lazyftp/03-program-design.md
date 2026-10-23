# Program design: first interactive workspace

Status: Gate 3 approved 2026-10-23. This specifies the first runnable slice, not speculative implementations of future transfer logic. The user subsequently waived further approval gates; keep design notes current while building the later slices.

## Files
- `go.mod`, `go.sum`: repository-based module identity and pinned compatible Charm dependencies; the final product name is not required.
- `cmd/lazyftp/main.go`: provisional executable, flags and Bubble Tea startup. Only demo mode is offered in the first slice; no real filesystem writes.
- `internal/ui/model.go`: event loop, panes, focus, asynchronous listing results, and modal ownership.
- `internal/ui/view.go`: terminal-cell-aware layout, clipping, sidebars, drawer and overlay rendering.
- `internal/ui/actions.go`: one catalog for palette results, direct shortcuts, availability and execution.
- `internal/ui/theme.go`: semantic color roles and four palette presets; optional user color overrides loaded by the executable from an explicitly supplied JSON config.
- `internal/ui/model_test.go`: observable interaction and rendered-state tests through the Bubble Tea model entry points.
- `internal/ui/theme_test.go`: config loading and valid role overrides through the config loader.
- `internal/storage/storage.go`: read-only location/entry contract needed for browsing. No premature transfer API.
- `internal/storage/demo.go`: deterministic, nested in-memory FTP/Blob-like fixture trees with metadata, clearly labeled as demo data. This is also the runnable demo's backend, not a separate test-only application.
- `internal/storage/demo_test.go`: listing and path-navigation contract, including missing paths.
- `scripts/terminal-smoke.py`: actual PTY keyboard/resize smoke, using an explicitly pinned terminal-screen decoder if needed. Assert decoded screen state rather than raw ANSI substrings.
- `Makefile`: `run`, `demo`, `test`, `test-terminal`, `check`; do not add pretend lab targets before real lab services exist.
- `.gitignore`: build output, scratch files, disposable lab data, machine-local config; never commit credentials.
- `README.md`: setup, commands, controls, demo limitations and font fallback.
- `.agents/setup`: reproducible toolchain/dependency preparation, following the orb-setup skill when created. Do not start interactive UI during setup.
- Existing planning docs: record approval and slice evidence.

## Types & signatures
Proposed contracts below use no implementation bodies. Bubble Tea method signatures must match the pinned major version selected before coding; use the stable model/update/view convention rather than mixing major APIs.

```go
// internal/storage: read-only first-slice contract
type Location struct {
    Remote string // empty for local; fixture locations are explicitly demo-only
    Path   string
}
type Entry struct {
    Name     string
    IsDir    bool
    Size     int64
    Modified time.Time
}
type Browser interface {
    List(context.Context, Location) ([]Entry, error)
}

// internal/ui: constructor inputs and semantic styling
type Options struct {
    Browser Browser // use storage.Browser, not a duplicate interface
    Left, Right Location // use storage.Location
    Theme Theme
    Icons bool
}
type Theme struct {
    Background, Surface, Foreground, Muted string
    Border, Focus, Selection, SelectionText string
    Local, Remote, Success, Warning, Error string
    Folder, Markdown, Data, Image string
}
func New(Options) Model
func LoadTheme(path string, preset string) (Theme, error)

// internal/ui: the catalog stays internal to the UI
type Action struct {
    ID, Label, Shortcut string
    Keywords []string
    DisabledReason string // nonempty actions cannot execute
}
```

Model owns both pane states, focused region, visibility preferences, palette query/cursor, theme and status. Each pane owns its current location, entries, cursor, selected names, loading/error state and listing generation. Result messages carry pane identity and generation so stale completions are ignored.

No generalized event bus, plugin registration, dependency-injection framework, or separate presentation model. Use ordinary Bubble Tea messages and commands. The fixture Browser permits deterministic injected errors/results in UI tests without mock-interaction assertions.

## Keyboard and action contracts
- `Ctrl+P`: open/focus action palette; arrows choose, text searches, Enter executes an enabled action, Esc dismisses. Use an existing small fuzzy matcher if compatible; avoid writing a ranking engine.
- `Tab` / `Shift+Tab`: traverse visible focusable regions; when a region is hidden, restore focus to its associated file pane. Modal input never leaks to workspace shortcuts.
- Arrows / `j/k`: move cursor; Enter enters directories; Backspace goes to parent; Space toggles selection; `/` edits a pane filter; Ctrl+R refreshes.
- `[` navigation, `]` inspector, `t` transfer drawer, `?` help, `q` quit. Same actions are discoverable through the palette. Input fields consume these characters as text.
- Palette includes theme choices for Catppuccin Mocha, Tokyo Night, Gruvbox Dark and Dracula, toggle icons, toggle panels, refresh, and quit.
- Copy and preview actions may appear disabled with an explicit first-slice explanation. They must not show fake success. Transfer drawer says no real jobs; screenshots' sample running jobs are not product functionality.
- No overwrite dialog is implemented before actual copy/conflict semantics exist. The first slice still proves modal focus with the palette/help.

## Layout and configuration
At 140+ columns, default to a 22-column left navigation, two equal remaining file panes, and an optional 28-column inspector. Borders/padding count inside allocations. Navigation is open by default; inspector and drawer closed. Both sidebars occupy all content rows between a one-row header and contextual footer; drawer uses only central rows.

Below 120 columns automatically suppress inspector; below 90 suppress navigation as well, preserving user visibility preferences for widening. At fewer than 60 columns or 12 rows, show a resize prompt rather than broken panes. Keep both endpoint names visible at supported widths. Test 160×40, 100×30, 80×24 and 50×10.

Render bounded rows with scrolling and display-cell-aware truncation; sanitize control sequences in entry names. Distinguish focused cursor from selected files and inactive pane cursors. Icons are optional; default to portable text glyphs until a user enables Nerd Font icons. No font installation is required to operate the app.

Explicit `--config` JSON supports theme preset, semantic color overrides and icon preference; `--theme` overrides its preset, not explicit role overrides. Unknown presets/roles and malformed colors fail with actionable errors. Do not execute theme code or claim imported editor-theme compatibility. No persistent app-named directory until naming is settled.

## Call stack
1. Startup → parse flags/config → construct fixture Browser and UI → Bubble Tea program.
2. Initial size/list messages → Browser.List commands → pane/generation result messages → sorted listings → render.
3. Navigation → update requested pane location/generation → asynchronous list → matching result → cursor reset and render; errors remain visible and retryable.
4. Ctrl+P → palette captures input → catalog filtered by fuzzy query → enabled result executed through the same action dispatcher → render.
5. Resize/toggle → recompute visible regions and focus → render within terminal cell bounds.

## Test plan
- **NavigateBothEndpoints:** different nested fixture contents on each side detect wrong-pane navigation; parent and refresh preserve correct endpoint.
- **PaletteOwnsTyping:** typing `quit` or brackets in the query does not quit or toggle sidebars; Esc returns focus, Enter on the selected enabled action acts once.
- **PaletteSearchAndAvailability:** a noncontiguous query finds its intended action; disabled copy remains unavailable and explains why. No result is not a panic.
- **SelectionIsPaneLocal:** selecting two asymmetric names in one pane does not select the other pane's cursor entry.
- **StaleListingCannotReplaceNewLocation:** deliver newer and older listing results in reversed order and verify the visible path/content.
- **ListingErrorIsRetryable:** show a controlled listing error, refresh, then render recovery without mutating the opposite pane.
- **LayoutAtRepresentativeSizes:** test sidebar height beside an open drawer, visibility restoration, terminal cell bounds, cursor scrolling, and resize prompt.
- **ThemeConfigAndFallback:** load all presets, override one role without changing others, reject invalid config, render recognizable filenames without special fonts.
- **UntrustedFilenameIsText:** names containing terminal control sequences cannot inject styling or extra terminal rows.
- **PTYSmoke:** start the actual program, navigate, open/query/dismiss palette, change theme, toggle both sidebars and drawer, resize and quit; assert decoded screen states and clean process exit. Capture and inspect representative terminal screenshots and an interaction recording separately from assertions.

`make test` must work without rclone, Docker, network or personal config once dependencies are installed. `make check` runs formatting validation, vet and tests; `make test-terminal` runs the PTY check. Record measured times, not aspirational claims. Later real-service tests are mandatory before reporting any transfer support.

## Least confident decisions
1. Width thresholds and sidebar proportions need real-terminal inspection; tune within this layout contract without adding new UI regions.
2. Bracket shortcuts may be awkward on non-US keyboards; palette access is the universal fallback and user feedback may change bindings.
3. First-slice storage is deliberately read-only. The rclone spike must establish cancellation/conflict guarantees before a transfer contract is designed.
4. Familiar palettes need attribution/licensing review when copied into presets. These design mockups are not exact editor-theme imports.

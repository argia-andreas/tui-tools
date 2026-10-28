package ui_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"logviewer/internal/config"
	"logviewer/internal/fixture"
	"logviewer/internal/logs"
	"logviewer/internal/theme"
	"logviewer/internal/ui"
)

type driver struct {
	t *testing.T
	m ui.Model
}

func (d *driver) apply(msg tea.Msg) tea.Cmd {
	next, cmd := d.m.Update(msg)
	d.m = next.(ui.Model)
	return cmd
}
func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			d.run(child)
		}
		return
	}
	if _, ok := msg.(tea.QuitMsg); ok {
		d.t.Fatal("text input dispatched quit")
	}
	d.run(d.apply(msg))
}
func (d *driver) key(key string) { d.run(d.apply(keyMsg(key))) }
func keyMsg(key string) tea.KeyMsg {
	keys := map[string]tea.KeyType{"enter": tea.KeyEnter, "esc": tea.KeyEsc, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab, "ctrl+l": tea.KeyCtrlL, "ctrl+s": tea.KeyCtrlS, "ctrl+u": tea.KeyCtrlU, "ctrl+t": tea.KeyCtrlT, "ctrl+x": tea.KeyCtrlX, "ctrl+r": tea.KeyCtrlR, "ctrl+p": tea.KeyCtrlP, "down": tea.KeyDown, "up": tea.KeyUp}
	if k, ok := keys[key]; ok {
		return tea.KeyMsg{Type: k}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}
func (d *driver) view() string { return ansi.Strip(d.m.View()) }

func (d *driver) click(label string) {
	d.t.Helper()
	for y, row := range strings.Split(d.view(), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			x = ansi.StringWidth(row[:x])
			d.run(d.apply(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}))
			d.run(d.apply(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}))
			return
		}
	}
	d.t.Fatalf("missing visible target %q\n%s", label, d.view())
}

func TestEditorSurfaceCancelAndApplyAreNotEscAndEnter(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	before := d.view()
	d.key("f")
	d.key("i")
	d.key("payment")
	d.click("[ Cancel esc ]")
	if d.view() != before {
		t.Fatal("explicit Cancel merely exited INSERT or changed results", d.view())
	}
	d.key("f")
	d.key("i")
	d.key("payment")
	d.click("[ Apply ctrl+s ]")
	if strings.Contains(d.view(), "FILTER EDITOR") {
		t.Fatal("Apply ran contextual Enter instead of applying from INSERT")
	}
	d.key("S")
	d.key("keyword is not scope")
	d.click("[ Save search ]")
	saved, err := config.LoadSearches(path)
	if err != nil || len(saved) != 1 || len(saved[0].Query.Scope.Services) != 0 {
		t.Fatalf("Apply incorrectly toggled the app suggestion: %v %+v", err, saved)
	}
	d.key("f")
	d.click("payment-worker") // Row selection is not an exact-app toggle.
	if strings.Contains(d.view(), "Selected: payment-worker") {
		t.Fatal("app row activated instead of selecting")
	}
	d.click("[ Toggle app ]")
	d.click("[ Apply ctrl+s ]")
	d.key("S")
	d.key("explicit exact app")
	d.click("[ Save search ]")
	saved, err = config.LoadSearches(path)
	var apps []string
	for _, entry := range saved {
		if entry.Name == "explicit exact app" {
			apps = entry.Query.Scope.Services
		}
	}
	if err != nil || len(saved) != 2 || !reflect.DeepEqual(apps, []string{"payment-worker"}) {
		t.Fatalf("explicit toggle/apply lost exact app: %v %+v", err, saved)
	}
}

func TestNavigationSelectFirstDisclosureAndWrappedFieldOwner(t *testing.T) {
	d, _ := fixtureDriver(t, 500)
	d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	d.click("POST /orders accepted")
	if !strings.Contains(d.view(), "● LOGS") || strings.Contains(d.view(), "● INSPECTOR") {
		t.Fatal("event row click inspected instead of selecting")
	}
	d.click("Inspect enter")
	if !strings.Contains(d.view(), "● INSPECTOR") || !strings.Contains(d.view(), "POST /orders accepted") {
		t.Fatal("explicit Inspect did not open the selected event")
	}
	d.click("[ Back esc ]")
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.click("Payment gateway timeout after")
	d.click("Inspect enter")
	d.click("message: Payment gateway timeout")
	d.click("[ Expand enter ]")
	d.click("Order 8042 remains pending;")
	if !strings.Contains(d.view(), "/message · safe") {
		t.Fatal("wrapped continuation selected a different native field")
	}
	d.click("fields ▾") // Label selects; only the separate triangle discloses.
	if !strings.Contains(d.view(), "token: [REDACTED]") {
		t.Fatal("container row label collapsed instead of selecting")
	}
	d.click("▾")
	if strings.Contains(d.view(), "token: [REDACTED]") {
		t.Fatal("disclosure press did not collapse exactly once")
	}
	d.click("▸")
	if !strings.Contains(d.view(), "token: [REDACTED]") {
		t.Fatal("disclosure did not expand the same native field")
	}
	d.apply(tea.MouseMsg{X: 40, Y: 12, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if !strings.Contains(d.view(), "● INSPECTOR") {
		t.Fatal("wheel over logs stole reader focus")
	}
	d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(d.view(), "● LOGS") || strings.Contains(d.view(), "● INSPECTOR") {
		t.Fatal("resize kept a hidden reader owner")
	}
}

func TestFilterTabReachesVisibleFooterAndReturnsToDraft(t *testing.T) {
	d, _ := fixtureDriver(t, 500)
	d.key("f")
	d.key("i")
	d.key("pay")
	d.key("tab") // INSERT → sidebar, retaining draft.
	d.key("tab") // Sidebar → Cancel, rather than trapping Tab between panes.
	if !strings.Contains(d.view(), "›[ Cancel esc ]") || strings.Contains(d.view(), "INSERT") {
		t.Fatal("Tab did not reach visible cancel safely", d.view())
	}
	d.key("shift+tab")
	d.key("shift+tab")
	if !strings.Contains(d.view(), "● Edit: App · NORMAL") || !strings.Contains(d.view(), "pay") {
		t.Fatal("reverse traversal lost the draft or body ownership")
	}
	d.key("tab")
	d.key("tab")
	d.key("enter")
	if strings.Contains(d.view(), "FILTER EDITOR") || !strings.Contains(d.view(), "22 fictional events") {
		t.Fatal("focused Cancel did not discard the draft")
	}
}

func TestEditorSurfaceValidationSettingsAndRestore(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	d.key("f")
	d.click("Time")
	d.key("i")
	d.key("ctrl+u")
	d.key("not-a-timestamp")
	d.apply(tea.WindowSizeMsg{Width: 60, Height: 16})
	d.click("[ Apply ctrl+s ]")
	if !strings.Contains(d.view(), "RFC3339 requires") || !strings.Contains(d.view(), "FILTER EDITOR") {
		t.Fatal("compact Apply hid validation or closed invalid draft", d.view())
	}
	d.click("[ Cancel esc ]")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("draft cancellation wrote saved searches")
	}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.key(",")
	d.click("Vim-style facet input:")
	d.click("[ Cancel esc ]")
	settingsPath := filepath.Join(filepath.Dir(path), "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatal("settings cancellation wrote private file")
	}
	d.key(",")
	if !strings.Contains(d.view(), "ON (default)") {
		t.Fatal("cancelled settings mutated session preference")
	}
	d.click("Vim-style facet input:")
	d.click("[ Apply settings ctrl+s ]")
	settings, err := config.LoadSettings(settingsPath)
	if err != nil || settings.VimMode {
		t.Fatalf("explicit settings apply failed: %v %+v", err, settings)
	}
	d.key("S")
	d.key("baseline")
	d.click("[ Save search ]")
	d.key("f")
	d.click("Message")
	d.key("no event has this phrase")
	d.click("[ Apply ctrl+s ]")
	d.key("R")
	d.click("baseline")
	if !strings.Contains(d.view(), "RESTORE SAVED SEARCH") {
		t.Fatal("saved row click restored instead of selecting")
	}
	d.click("[ Restore ]")
	if !strings.Contains(d.view(), "Message/trace/fields: ALL") || !strings.Contains(d.view(), "22 fictional events") {
		t.Fatal("explicit Restore did not replace query", d.view())
	}
}

func fixtureDriver(t *testing.T, size int) (*driver, string) {
	t.Helper()
	p, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	s, err := logs.NewService(p, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	q := fixture.Query()
	q.Limits.Events = size
	path := filepath.Join(t.TempDir(), "searches.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: q, Theme: theme.Midnight(), SearchesPath: path, SettingsPath: filepath.Join(filepath.Dir(path), "settings.json")})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.run(d.m.Init())
	return d, path
}

func TestMinimumSizeRetainsDraftAndBlocksHiddenWrites(t *testing.T) {
	for _, size := range [][2]int{{59, 16}, {60, 15}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			d, path := fixtureDriver(t, 22)
			d.key("S")
			d.key("retained search")
			d.apply(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			d.key("enter")
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("hidden Save wrote a search", err)
			}
			d.key("q")
			d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
			if !strings.Contains(d.view(), "retained search") || strings.Contains(d.view(), "retained searchq") {
				t.Fatal("hidden input lost or edited retained draft", d.view())
			}
			d.key("enter")
			if _, err := os.Stat(path); err != nil {
				t.Fatal("visible Save did not work", err)
			}

			d.key(",")
			d.key("enter") // Deliberately turn Vim mode off before hiding.
			d.apply(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			d.key("enter")
			d.key("ctrl+s")
			settings := filepath.Join(filepath.Dir(path), "settings.json")
			if _, err := os.Stat(settings); !os.IsNotExist(err) {
				t.Fatal("hidden Settings wrote preferences", err)
			}
			d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
			d.key("ctrl+s")
			got, err := config.LoadSettings(settings)
			if err != nil || got.VimMode {
				t.Fatal("hidden toggle changed retained settings draft", got, err)
			}

			d.key("f")
			d.key("payment") // Vim is off: literal app filter input.
			d.apply(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			d.key("ctrl+s")
			d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
			if !strings.Contains(d.view(), "FILTER EDITOR") || !strings.Contains(d.view(), "payment") {
				t.Fatal("hidden Apply submitted or lost filter draft", d.view())
			}
			d.key("esc")
		})
	}
}

func TestPaletteOwnsInputDispatchesExistingActionsAndRestoresFocus(t *testing.T) {
	d, path := fixtureDriver(t, 3)
	d.key("enter")
	before := d.view()
	d.key("ctrl+p")
	if !strings.Contains(d.view(), "WORKSPACE") || !strings.Contains(d.view(), "INSPECTOR") {
		t.Fatal("palette replaced the workbench instead of floating above it")
	}
	d.key("q y ")
	d.key("enter") // No match must not quit, yank or follow.
	if !strings.Contains(d.view(), "No results") {
		t.Fatal(d.view())
	}
	d.key("esc")
	if d.view() != before {
		t.Fatal("palette cancel lost inspector focus, selection or results")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("palette typing wrote a saved search")
	}
	d.key("ctrl+p")
	if !strings.Contains(d.view(), "Filter from selected event field") || strings.Contains(d.view(), "Live follow") {
		t.Fatal("palette does not reflect implemented contextual actions")
	}
	d.key("fltfld") // Non-contiguous fuzzy match, not substring matching.
	d.key("enter")
	if !strings.Contains(d.view(), "FILTER FROM EVENT") {
		t.Fatal(d.view())
	}
	d.key("esc")
	d.key("ctrl+p")
	d.key("older")
	d.key("enter")
	if !strings.Contains(d.view(), "6 fictional events") || !strings.Contains(d.view(), "● INSPECTOR") || !strings.Contains(d.view(), "Settlement batch completed") {
		t.Fatal("palette older action lost history anchor or focus: " + d.view())
	}
	d.key("ctrl+p")
	d.key("literal")
	d.key("enter")
	d.key("q y ")
	if !strings.Contains(d.view(), "LOCAL LITERAL FIND") || !strings.Contains(d.view(), "> q y ") {
		t.Fatal(d.view())
	}
	d.key("esc")
	d.key("ctrl+p")
	d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	d.key("esc")
	if !strings.Contains(d.view(), "● LOGS") || strings.Contains(d.view(), "INSPECTOR") {
		t.Fatal("palette restored hidden focus after resize")
	}
}

func TestPaletteAvailabilityAndNarrowBounds(t *testing.T) {
	d, _ := fixtureDriver(t, 500)
	d.key("ctrl+p")
	d.key("older")
	d.key("enter")
	if !strings.Contains(d.view(), "ACTIONS") || !strings.Contains(d.view(), "No older history available") {
		t.Fatal("unavailable history action ran")
	}
	for _, width := range []int{60, 80, 119, 120, 139, 140, 160} {
		for _, height := range []int{16, 24, 48} {
			d.apply(tea.WindowSizeMsg{Width: width, Height: height})
			d.key("ctrl+u")
			for i := 0; i < 12; i++ {
				d.key("down")
			}
			if !strings.Contains(d.view(), "› Quit log viewer") {
				t.Fatal("last action not reachable: " + d.view())
			}
			lines := strings.Split(d.view(), "\n")
			if len(lines) > height {
				t.Fatalf("%dx%d palette overflows height", width, height)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > width {
					t.Fatalf("%dx%d palette overflows width", width, height)
				}
			}
		}
	}
	// An embedded UI without a persistence path must not infer a writable default.
	d.m = ui.New(ui.Options{Theme: theme.Midnight()})
	d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	d.key("ctrl+p")
	d.key("save named")
	d.key("enter")
	if !strings.Contains(d.view(), "Requires explicit --searches path") || !strings.Contains(d.view(), "ACTIONS") {
		t.Fatal(d.view())
	}
	d.key("ctrl+u")
	d.key("inspect")
	d.key("enter")
	if !strings.Contains(d.view(), "No selected event") || !strings.Contains(d.view(), "ACTIONS") {
		t.Fatal(d.view())
	}
}

func TestPaletteMouseAndKeyboardUseCheckedActions(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	// Select a noninitial event so inspecting the wrong row cannot pass.
	d.key("down")
	d.key("down")
	before := d.view()
	press := func(label string) tea.MouseMsg {
		t.Helper()
		for y, line := range strings.Split(d.view(), "\n") {
			if x := strings.Index(line, label); x >= 0 {
				return tea.MouseMsg{X: ansi.StringWidth(line[:x]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
			}
		}
		t.Fatalf("control %q not rendered:\n%s", label, d.view())
		return tea.MouseMsg{}
	}
	d.run(d.apply(press("Actions ctrl+p")))
	// Dismissing over the background Inspect button consumes the press.
	d.run(d.apply(press("Inspect enter")))
	if d.view() != before {
		t.Fatal("outside press leaked to the workbench")
	}
	d.run(d.apply(press("Actions ctrl+p")))
	d.key("older")
	d.run(d.apply(press("Load older history")))
	if !strings.Contains(d.view(), "No older history available") || !strings.Contains(d.view(), "ACTIONS") {
		t.Fatal("unavailable click closed or executed the palette")
	}
	d.key("ctrl+u")
	d.key("inspect")
	click := press("Inspect selected event")
	d.run(d.apply(click))
	after := d.view()
	if !strings.Contains(after, "● INSPECTOR") {
		t.Fatal("palette click did not inspect")
	}
	click.Action = tea.MouseActionRelease
	d.run(d.apply(click))
	if d.view() != after {
		t.Fatal("release triggered a second action")
	}
	d.key("esc")
	d.key("enter")
	if d.view() != after {
		t.Fatal("click and keyboard inspected different events")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("nonmutating component actions wrote a saved search")
	}
}

func TestEditorsOwnTextAndCancelWithoutWritesOrFocusChanges(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	d.key("enter")
	before := d.view()
	d.key("/")
	d.key("q y ")
	if !strings.Contains(d.view(), "> q y ") {
		t.Fatal("find did not retain printable shortcut keys")
	}
	d.key("esc")
	if d.view() != before {
		t.Fatal("find cancellation changed results, selection or focus")
	}
	d.key("esc")
	before = d.view()
	d.key("f")
	d.key("i")
	d.key("q y ")
	if !strings.Contains(d.view(), "q y ") {
		t.Fatal("filter editor lost text")
	}
	d.key("esc")
	d.key("esc")
	if d.view() != before {
		t.Fatal("filter cancellation changed results")
	}
	d.key("S")
	d.key("q y ")
	d.key("esc")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("editor wrote without explicit submit")
	}
	for _, width := range []int{60, 80, 119, 120, 139, 140, 160} {
		for _, height := range []int{16, 24, 48} {
			d.apply(tea.WindowSizeMsg{Width: width, Height: height})
			d.key("f")
			for i := 0; i < 7; i++ {
				view := d.view()
				if !strings.Contains(view, "FILTER EDITOR") {
					t.Fatal("narrow editor inaccessible")
				}
				lines := strings.Split(view, "\n")
				if len(lines) > height {
					t.Fatalf("facet editor exceeds height at %dx%d", width, height)
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > width {
						t.Fatalf("facet editor exceeds width at %dx%d", width, height)
					}
				}
				d.key("tab")
				d.key("down")
				d.key("shift+tab")
			}
			d.key("esc")
		}
	}
}

func TestWorkspaceRestoresSameSourceSavedSearchAndSurvivesCollapse(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	q := fixture.Query()
	q.Scope.Services = []string{"payment-worker"}
	if err := config.SaveSearch(path, config.SavedSearch{Name: "Payment failures", Query: q}); err != nil {
		t.Fatal(err)
	}
	foreign := q
	foreign.Scope.Source = "local-elastic"
	if err := config.SaveSearch(path, config.SavedSearch{Name: "Foreign source", Query: foreign}); err != nil {
		t.Fatal(err)
	}
	d.key("tab") // Inspector, then workspace.
	d.key("tab")
	if !strings.Contains(d.view(), "Payment failures") || strings.Contains(d.view(), "Foreign source") {
		t.Fatal("workspace must show only current-source saved queries")
	}
	d.key("down")
	d.key("enter")
	if !strings.Contains(d.view(), "App exact: payment-worker") || !strings.Contains(d.view(), "12 fictional events") {
		t.Fatal("sidebar entry did not restore the actual saved predicate")
	}
	d.key("S")
	d.key("Payments again")
	d.key("enter")
	if !strings.Contains(d.view(), "Payments again") {
		t.Fatal("explicit save did not update workspace")
	}
	d.key("tab")
	d.key("tab")
	d.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(d.view(), "● LOGS") || strings.Contains(d.view(), "WORKSPACE") {
		t.Fatal("collapsed sidebar retained hidden focus")
	}
	d.key("R")
	if !strings.Contains(d.view(), "Payment failures") {
		t.Fatal("saved picker unavailable without sidebar")
	}
}

func TestExactAppsInspectedTypedPredicatesSaveAndRestore(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	d.key(",")
	d.key("enter") // Retain coverage of direct-entry mode when Vim is disabled.
	d.key("ctrl+s")
	d.key("f")
	d.key("payment")
	d.key("enter")
	if !strings.Contains(d.view(), "Selected: payment-worker") {
		t.Fatal("exact app selection missing")
	}
	d.key("ctrl+s")
	if !strings.Contains(d.view(), "App exact: payment-worker") || strings.Contains(d.view(), "POST /orders accepted") {
		t.Fatal("app facet not applied")
	}
	d.key("enter")
	d.key("f")
	d.key("g") // The picker starts on the inspector cursor; trace is first.
	if !strings.Contains(d.view(), "› /trace.id") {
		t.Fatal("trace scalar not offered")
	}
	d.key("enter")
	d.key("f")
	d.key("g")
	d.key("j") // supported field after trace: numeric job attempt
	if !strings.Contains(d.view(), "› /fields/job/attempt = 1") {
		t.Fatal("typed event predicate not offered")
	}
	d.click("/fields/job/attempt = 1")
	if !strings.Contains(d.view(), "FILTER FROM EVENT") {
		t.Fatal("field row click applied instead of selecting")
	}
	d.click("[ Apply exact filter ]")
	d.key("f")
	d.click("/fields/auth/token")
	d.click("[ [unavailable] Apply exact filter")
	if !strings.Contains(d.view(), "FILTER FROM EVENT") || !strings.Contains(d.view(), "redacted value · cannot filter") {
		t.Fatal("unavailable redacted field bypassed checked Apply")
	}
	d.click("[ Cancel esc ]")
	d.key("S")
	d.key("payment trace attempt")
	d.key("enter")
	saved, err := config.LoadSearches(path)
	if err != nil || len(saved) != 1 {
		t.Fatalf("explicit save: %v\n%s", err, d.view())
	}
	q := saved[0].Query
	if !reflect.DeepEqual(q.Scope.Services, []string{"payment-worker"}) || len(q.Fields) != 2 || q.Fields[1].Path != "/fields/job/attempt" || string(q.Fields[1].Value) != "1" {
		t.Fatalf("lost exact predicates: %+v", q)
	}
	d.key("esc")
	d.key("f")
	d.key("ctrl+x")
	d.key("ctrl+r")
	d.key("ctrl+s")
	d.key("R")
	d.key("enter")
	if !strings.Contains(d.view(), "App exact: payment-worker") || !strings.Contains(d.view(), "/fields/job/attempt=1") {
		t.Fatal("restore lost exact filters")
	}
	d.key("f")
	d.key("tab")
	for i := 0; i < 5; i++ {
		d.key("j")
	}
	d.key("shift+tab")
	d.key("q")
	d.key("y")
	d.key(" ")
	if !strings.Contains(d.view(), "› Exact field predicates") {
		t.Fatal("predicate row dispatched printable shortcuts")
	}
	d.key("j")
	d.key("enter")
	d.key("ctrl+s")
	if strings.Contains(d.view(), "/fields/job/attempt=1") || !strings.Contains(d.view(), "/trace.id=") || !strings.Contains(d.view(), "App exact: payment-worker") {
		t.Fatal("removing one predicate changed other facets")
	}
}

func TestFacetPaneNavigationApplyAndClear(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	d.key(",")
	d.key("enter")
	d.key("ctrl+s")
	d.key("f")
	d.key("payment")
	d.key("enter")
	d.key("tab")
	if !strings.Contains(d.view(), "● FILTER EDITOR") || strings.Contains(d.view(), "● Edit:") {
		t.Fatal("Tab did not focus the sidebar exclusively")
	}
	d.key("k")
	if !strings.Contains(d.view(), "Edit: Exact fields") {
		t.Fatal("sidebar k did not wrap to last facet")
	}
	d.key("jj") // Fast repeated keys may arrive in a single terminal message.
	d.key("shift+tab")
	if !strings.Contains(d.view(), "● Edit: Time") || !strings.Contains(d.view(), "› Start inclusive") {
		t.Fatal("sidebar did not navigate to Time and return to editing")
	}
	d.key("ctrl+u")
	d.key("2026-10-23T14:26:00Z")
	d.key("j")
	if !strings.Contains(d.view(), "› End exclusive") {
		t.Fatal("j did not choose the second Time input")
	}
	d.key("k")
	d.key("down")
	d.key("up")
	if !strings.Contains(d.view(), "› Start inclusive") {
		t.Fatal("k/up did not return to the first Time input")
	}
	d.key("tab")
	d.key("down")
	d.key("shift+tab")
	d.key("ctrl+u")
	d.key("WARN+")
	d.key("tab")
	d.key("j")
	d.key("shift+tab")
	d.key("jky ")
	if !strings.Contains(d.view(), "jky ") {
		t.Fatal("message text lost navigation characters")
	}
	d.key("ctrl+u")
	d.key("tab")
	d.key("ctrl+s") // Apply works from the sidebar, too.
	if strings.Contains(d.view(), "FILTER EDITOR") || !strings.Contains(d.view(), "App exact: payment-worker") {
		t.Fatal("apply did not return to filtered logs")
	}
	d.key("S")
	d.key("filtered")
	d.key("enter")
	saved, err := config.LoadSearches(path)
	if err != nil || len(saved) != 1 {
		t.Fatalf("save applied query: %v", err)
	}
	q := saved[0].Query
	if q.Window.Start.Format("2006-01-02T15:04:05Z07:00") != "2026-10-23T14:26:00Z" || !reflect.DeepEqual(q.Levels, []string{"WARN", "ERROR", "FATAL"}) {
		t.Fatalf("navigation edited the wrong input: %+v", q)
	}
	// Include every resettable criterion, and verify through save/restore.
	q.Fields = []logs.FieldFilter{{Path: "/fields/job/attempt", Value: []byte("1")}}
	q.TraceIDs = []string{"trace-payment-001"}
	q.Message = logs.MessageFilter{Mode: "phrase", Text: "timeout"}
	if err := config.SaveSearch(path, config.SavedSearch{Name: "filtered", Query: q}); err != nil {
		t.Fatal(err)
	}
	d.key("R")
	d.key("enter")
	d.key("f")
	d.key("ctrl+l")
	d.key("esc")
	d.key("S")
	d.key("cancelled")
	d.key("enter")
	saved, err = config.LoadSearches(path)
	if err != nil || len(saved) != 2 || !reflect.DeepEqual(saved[1].Query, q) {
		t.Fatalf("cancelled clear changed applied query: %v %+v", err, saved)
	}
	d.key("f")
	d.key("tab")
	d.key("ctrl+l")
	d.key("ctrl+s")
	d.key("S")
	d.key("cleared")
	d.key("enter")
	saved, err = config.LoadSearches(path)
	if err != nil || len(saved) != 3 {
		t.Fatalf("save cleared query: %v", err)
	}
	cleared := saved[2].Query
	if len(cleared.Scope.Services)+len(cleared.Fields)+len(cleared.TraceIDs)+len(cleared.Levels) != 0 || cleared.Message.Text != "" || cleared.Window != q.Window || cleared.Scope.Source != q.Scope.Source || cleared.Limits != q.Limits {
		t.Fatalf("clear did not reset criteria while retaining window/source/limits: %+v", cleared)
	}
}

func TestVimFacetModesAndSettings(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	settingsPath := filepath.Join(filepath.Dir(path), "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatal("startup wrote settings")
	}
	d.key("f")
	if !strings.Contains(d.view(), "Edit: App · NORMAL") {
		t.Fatal("Vim mode was not enabled by default")
	}
	d.key("j")
	if !strings.Contains(d.view(), "› [ ] payment-worker") {
		t.Fatal("normal j typed text instead of navigating suggestions")
	}
	d.key(" ")
	d.key("k")
	d.key("enter")
	d.key("jk")
	if !strings.Contains(d.view(), "Edit: App · INSERT") || !strings.Contains(d.view(), "No suggestions") {
		t.Fatal("Enter did not enter literal keyword input")
	}
	d.key("esc")
	if !strings.Contains(d.view(), "Edit: App · NORMAL") || !strings.Contains(d.view(), "jk") {
		t.Fatal("Esc discarded input or closed the editor")
	}
	d.key("i")
	d.key("ctrl+u")
	d.key("payment")
	d.key("enter")
	d.key("tab")
	d.key("jjj")
	d.key("shift+tab")
	d.key("i")
	d.key("jki q y ")
	d.key("esc")
	if !strings.Contains(d.view(), "Edit: Message · NORMAL") || !strings.Contains(d.view(), "jki q y ") {
		t.Fatal("INSERT j/k/i dispatched navigation or global shortcuts")
	}
	d.key("i")
	d.key("tab")
	if !strings.Contains(d.view(), "● FILTER EDITOR") || strings.Contains(d.view(), "INSERT") {
		t.Fatal("Tab did not exit INSERT and switch panes")
	}
	d.key("ctrl+s")
	d.key("S")
	d.key("vim draft")
	d.key("enter")
	saved, err := config.LoadSearches(path)
	if err != nil || len(saved) != 1 || saved[0].Query.Message.Text != "jki q y " || !reflect.DeepEqual(saved[0].Query.Scope.Services, []string{"payment-worker"}) {
		t.Fatalf("Vim navigation corrupted draft or application: %v %+v", err, saved)
	}
	d.key("ctrl+p")
	d.key("settings")
	d.key("enter")
	if !strings.Contains(d.view(), "Vim-style facet input: ON") {
		t.Fatal("settings not reachable from palette")
	}
	d.key("enter")
	d.key("esc")
	d.key(",")
	if !strings.Contains(d.view(), "Vim-style facet input: ON") {
		t.Fatal("cancelled settings draft changed the preference")
	}
	d.key(" ")
	if err := os.WriteFile(settingsPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	d.key("ctrl+s")
	if !strings.Contains(d.view(), "Settings save failed") {
		t.Fatal("unsafe destination was reported as a successful preference save")
	}
	d.key("esc")
	d.key(",")
	if !strings.Contains(d.view(), "Vim-style facet input: ON") {
		t.Fatal("failed settings save changed the active preference")
	}
	if err := os.Chmod(settingsPath, 0600); err != nil {
		t.Fatal(err)
	}
	d.key(" ")
	d.key("ctrl+s")
	settings, err := config.LoadSettings(settingsPath)
	if err != nil || settings.VimMode {
		t.Fatalf("OFF preference was not persisted: %v %+v", err, settings)
	}
	// A fresh UI loads the persisted preference, not a process-only toggle.
	restarted := &driver{t: t, m: ui.New(ui.Options{SettingsPath: settingsPath, Theme: theme.Midnight()})}
	restarted.apply(tea.WindowSizeMsg{Width: 80, Height: 24})
	restarted.key(",")
	if !strings.Contains(restarted.view(), "Vim-style facet input: OFF") {
		t.Fatal("preference did not survive a new model")
	}
	d.key("f")
	d.key("jk")
	if strings.Contains(d.view(), "NORMAL") || !strings.Contains(d.view(), "No suggestions") {
		t.Fatal("OFF mode did not restore direct text entry")
	}
	d.key("esc")
	d.key(",")
	d.key("enter")
	d.key("ctrl+s")
	d.key("f")
	if !strings.Contains(d.view(), "Edit: App · NORMAL") {
		t.Fatal("ON preference did not restore Vim modes")
	}
}

func TestFindSearchesDecodedStructuredFieldsAndCaseToggle(t *testing.T) {
	d, _ := fixtureDriver(t, 500)
	d.key("/")
	d.key("SWEDENCENTRAL")
	d.key("ctrl+t")
	d.key("enter")
	if !strings.Contains(d.view(), "No local literal matches") {
		t.Fatal("case-sensitive find ignored case")
	}
	d.key("/")
	d.key("ctrl+t")
	d.key("enter")
	if !strings.Contains(d.view(), "Local literal match") {
		t.Fatal("full safe structured field not searched")
	}
	d.key("N")
	d.key("n")
	if !strings.Contains(d.view(), "Payment gateway timeout") {
		t.Fatal("find navigation lost matched event")
	}
}

func TestOlderHistoryKeepsSelectedEventAnchored(t *testing.T) {
	d, _ := fixtureDriver(t, 3)
	d.key("enter")
	if !strings.Contains(d.view(), "Settlement batch completed") {
		t.Fatal("unexpected newest page")
	}
	d.key("o")
	if !strings.Contains(d.view(), "6 fictional events") || !strings.Contains(d.view(), "Settlement batch completed") {
		t.Fatal("history lost anchored selection")
	}
}

type cursorProvider struct {
	mu       sync.Mutex
	searches int
	closed   []string
}

func (p *cursorProvider) Search(ctx context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searches++
	n := p.searches
	return logs.NativePage{Events: []logs.Event{{Ref: logs.NativeRef{Index: "fixture", ID: fmt.Sprint(n)}, Time: q.Window.Start, Level: "INFO", Message: fmt.Sprintf("generation-%d", n)}}, More: true, Cursor: fmt.Sprintf("native-%d", n)}, nil
}
func (p *cursorProvider) Close(_ context.Context, cursor string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = append(p.closed, cursor)
	return nil
}
func (p *cursorProvider) Values(context.Context, logs.Query, string, string) (logs.Values, error) {
	return logs.Values{}, nil
}

func TestObsoleteCompletionCannotReplaceResultsAndClosesCursor(t *testing.T) {
	p := &cursorProvider{}
	s, err := logs.NewService(p, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	obsolete := d.m.Init()() // completed before a refresh, delivered out of order
	d.key("r")
	d.run(d.apply(obsolete))
	if !strings.Contains(d.view(), "generation-2") || strings.Contains(d.view(), "generation-1") {
		t.Fatal("stale search replaced current results")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !reflect.DeepEqual(p.closed, []string{"native-1"}) {
		t.Fatalf("stale cursor not closed: %v", p.closed)
	}
}

func TestRefreshDiscardsObsoleteOlderPageAndClosesItsNewCursor(t *testing.T) {
	p := &cursorProvider{}
	s, err := logs.NewService(p, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.run(d.m.Init())
	pageRequest := d.apply(keyMsg("o"))
	obsoletePage := pageRequest()
	d.key("r")
	d.run(d.apply(obsoletePage))
	if !strings.Contains(d.view(), "generation-3") || strings.Contains(d.view(), "generation-2") || strings.Contains(d.view(), "generation-1") {
		t.Fatal("obsolete history merged into refreshed results")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !reflect.DeepEqual(p.closed, []string{"native-2"}) {
		t.Fatalf("stale advanced cursor not closed: %v", p.closed)
	}
}

type tiedProvider struct{}

func (tiedProvider) Search(_ context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	ids := []int{3, 4}
	more := true
	next := "older"
	if cursor != "" {
		ids = []int{1, 2}
		more = false
		next = ""
	}
	page := logs.NativePage{More: more, Cursor: next}
	for _, id := range ids {
		page.Events = append(page.Events, logs.Event{Ref: logs.NativeRef{Index: "ties", ID: fmt.Sprint(id)}, Time: q.Window.Start, Level: "INFO", Message: fmt.Sprintf("tied-event-%d", id)})
	}
	return page, nil
}
func (tiedProvider) Close(context.Context, string) error { return nil }
func (tiedProvider) Values(context.Context, logs.Query, string, string) (logs.Values, error) {
	return logs.Values{}, nil
}

func TestOlderEqualTimePagesPrependWithoutReversingProviderOrder(t *testing.T) {
	s, err := logs.NewService(tiedProvider{}, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.run(d.m.Init())
	d.key("o")
	v := d.view()
	for id := 1; id < 4; id++ {
		if strings.Index(v, fmt.Sprintf("tied-event-%d", id)) >= strings.Index(v, fmt.Sprintf("tied-event-%d", id+1)) {
			t.Fatal("equal-time provider order reversed")
		}
	}
	if !strings.Contains(v, "› "+fixture.Query().Window.Start.Format("15:04:05.000")) {
		t.Fatal("anchored row selection missing")
	}
	d.key("enter")
	if !strings.Contains(d.view(), "MESSAGE") || strings.Count(d.view(), "tied-event-3") != 2 {
		t.Fatal("equal-time prepend lost selected event")
	}
}

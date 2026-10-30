package ui_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

type browserFunc func(context.Context, storage.Location) ([]storage.Entry, error)

func (f browserFunc) List(c context.Context, l storage.Location) ([]storage.Entry, error) {
	return f(c, l)
}

type driver struct {
	model tea.Model
	quit  bool
}

func (d *driver) deliver(msg tea.Msg) {
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, cmd := range batch {
			d.run(cmd)
		}
		return
	}
	if _, ok := msg.(tea.QuitMsg); ok {
		d.quit = true
		return
	}
	next, cmd := d.model.Update(msg)
	d.model = next
	d.run(cmd)
}
func (d *driver) run(cmd tea.Cmd) {
	if cmd != nil {
		d.deliver(cmd())
	}
}
func (d *driver) key(key string) {
	special := map[string]tea.KeyType{"tab": tea.KeyTab, "enter": tea.KeyEnter, "esc": tea.KeyEsc, "ctrl+h": tea.KeyCtrlH, "ctrl+l": tea.KeyCtrlL, "ctrl+p": tea.KeyCtrlP, "ctrl+r": tea.KeyCtrlR, "backspace": tea.KeyBackspace, "shift+tab": tea.KeyShiftTab, "f1": tea.KeyF1}
	if kind, ok := special[key]; ok {
		d.deliver(tea.KeyMsg{Type: kind})
	} else {
		d.deliver(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
}
func (d *driver) screen() string { return ansi.Strip(d.model.View()) }
func start(t *testing.T, b storage.Browser) *driver {
	t.Helper()
	th, err := theme.Get("")
	if err != nil {
		t.Fatal(err)
	}
	m := ui.New(ui.Options{Browser: b, Left: storage.Location{Path: "/project"}, Right: storage.Location{Remote: "demo-ftp", Path: "/incoming"}, Theme: th})
	d := &driver{model: m}
	d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
	d.run(m.Init())
	return d
}
func requireText(t *testing.T, d *driver, want string) {
	t.Helper()
	if !strings.Contains(d.screen(), want) {
		t.Fatalf("missing %q:\n%s", want, d.screen())
	}
}

func (d *driver) click(t *testing.T, label string) {
	t.Helper()
	for y, line := range strings.Split(d.screen(), "\n") {
		if at := strings.Index(line, label); at >= 0 {
			x := ansi.StringWidth(line[:at]) + 1
			d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
			return
		}
	}
	t.Fatalf("missing clickable %q:\n%s", label, d.screen())
}

func TestKeyboardNavigationSelectionAndEndpoints(t *testing.T) {
	d := start(t, storage.NewDemo())
	requireText(t, d, "README.md")
	requireText(t, d, "orders.csv")
	d.key("j")
	d.key("enter")
	requireText(t, d, "/project/src")
	requireText(t, d, "browser.go")
	requireText(t, d, "orders.csv")
	d.key(" ")
	requireText(t, d, "1 selected")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "/incoming/batch 01")
	requireText(t, d, "shipment.json")
	requireText(t, d, "1 selected")
	d.key("]")
	d.key("tab")
	requireText(t, d, "shipment.json")
	// Inspector must retain the remote endpoint after receiving focus.
	parts := strings.Split(d.screen(), "INSPECTOR")
	if len(parts) != 2 || !strings.Contains(parts[1], "/incoming/batch 01") {
		t.Fatal("inspector lost remote endpoint")
	}
}

func TestDirectionalPaneShortcuts(t *testing.T) {
	for _, width := range []int{160, 80} {
		t.Run(fmt.Sprintf("width%d", width), func(t *testing.T) {
			d := start(t, storage.NewDemo())
			d.deliver(tea.WindowSizeMsg{Width: width, Height: 40})
			d.key("]")
			d.key("t")
			// Repeated directional keys stay on the requested file pane,
			// even when Tab can also reach navigation, inspector and jobs.
			d.key("ctrl+l")
			d.key("ctrl+l")
			d.key("enter")
			requireText(t, d, "shipment.json")
			d.key("tab")
			d.key("ctrl+h")
			d.key("ctrl+h")
			d.key("j")
			d.key("enter")
			requireText(t, d, "browser.go")
			requireText(t, d, "shipment.json")
			// Tab and Shift+Tab still move between the two file panes.
			d.key("tab")
			d.key("h")
			requireText(t, d, "orders.csv")
			d.key("shift+tab")
			d.key("h")
			requireText(t, d, "README.md")
		})
	}
}

func TestEnterInspectsFilesWithoutTogglingOrMovingFocus(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote=%t", remote), func(t *testing.T) {
			d := start(t, storage.NewDemo())
			name, nextName, location := "README.md", "project notes.txt", "/project"
			if remote {
				d.key("tab")
				name, nextName, location = "orders.csv", "résumé.pdf", "/incoming"
			}
			d.key("j")
			d.key("j")
			d.key("enter")
			assertInspector := func(want string) {
				t.Helper()
				parts := strings.Split(d.screen(), "INSPECTOR")
				if len(parts) != 2 || !strings.Contains(parts[1], want) || !strings.Contains(parts[1], location) {
					t.Fatalf("inspector does not show %q at %q:\n%s", want, location, d.screen())
				}
			}
			assertInspector(name)
			d.key("enter")
			assertInspector(name)
			d.key("j") // Enter leaves focus in the file pane.
			assertInspector(nextName)
			d.key("enter")
			assertInspector(nextName)
			d.key("]") // The explicit toggle still closes the inspector.
			if strings.Contains(d.screen(), "INSPECTOR") {
				t.Fatal("inspector toggle did not close")
			}
		})
	}
}

func TestEnterDoesNotInspectEmptyPaneOrConsumeOtherContexts(t *testing.T) {
	d := start(t, browserFunc(func(context.Context, storage.Location) ([]storage.Entry, error) {
		return nil, nil
	}))
	d.key("enter")
	if strings.Contains(d.screen(), "INSPECTOR") {
		t.Fatal("Enter opened inspector without a file")
	}
	d = start(t, storage.NewDemo())
	d.key("/")
	d.key("README")
	d.key("enter")
	if strings.Contains(d.screen(), "INSPECTOR") {
		t.Fatal("applying filter opened inspector")
	}
	d.key("?")
	d.key("enter")
	requireText(t, d, " HELP ")
	d.key("esc")
	if strings.Contains(d.screen(), "INSPECTOR") {
		t.Fatal("Enter in help opened inspector")
	}
	d.key("enter")
	requireText(t, d, "INSPECTOR")
	d.key("tab")
	d.key("tab") // Inspector focus: Enter must not close it or navigate.
	d.key("enter")
	requireText(t, d, "INSPECTOR")
	requireText(t, d, "README.md")
}

func TestMinimumSizeRetainsPaletteAndBlocksHiddenActivation(t *testing.T) {
	for _, size := range [][2]int{{59, 12}, {60, 11}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			d := start(t, storage.NewDemo())
			d.key("ctrl+p")
			d.key("drcla")
			d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			d.key("enter")
			d.key("q")
			d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
			requireText(t, d, "Query: drcla")
			if d.quit || strings.Contains(d.screen(), "Query: drclaq") {
				t.Fatal("hidden palette activated or edited", d.screen())
			}
			d.key("enter")
			if strings.Contains(d.screen(), "ACTIONS · Ctrl+P") {
				t.Fatal("visible palette action stopped working")
			}
			requireText(t, d, "dracula")
		})
	}
}

func TestPaletteOwnsInputAndExecutesEnabledActions(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("ctrl+p")
	d.key("quit[]")
	d.key("enter")
	if d.quit {
		t.Fatal("typing quit executed a global action")
	}
	requireText(t, d, "No results")
	d.key("esc")
	requireText(t, d, "WORKSPACE")
	d.key("ctrl+p")
	d.key("drcla")
	requireText(t, d, "Theme: dracula")
	d.key("enter")
	requireText(t, d, "dracula")
	if strings.Contains(d.screen(), "ACTIONS · Ctrl+P") {
		t.Fatal("enabled palette action did not close")
	}
	d.key("ctrl+p")
	d.key("copy")
	d.key("enter")
	requireText(t, d, "demo is read-only")
	requireText(t, d, "ACTIONS · Ctrl+P")
	d.key("esc")
	if d.quit {
		t.Fatal("disabled action quit")
	}
	d.key("tab")
	d.key("c")
	requireText(t, d, "demo is read-only")
	d.key("e")
	requireText(t, d, "Demo is read-only; edit real files in live mode")
	d.key("ctrl+p")
	d.key("Neovim")
	requireText(t, d, "Edit selected file in Neovim")
	d.key("enter")
	requireText(t, d, "ACTIONS · Ctrl+P")
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !d.quit {
		t.Fatal("palette swallowed FTP's cleanup-and-quit interrupt")
	}
}

func TestSharedPaletteCaptureClampsAndInspectorParity(t *testing.T) {
	for _, pointer := range []bool{false, true} {
		t.Run(fmt.Sprintf("pointer=%t", pointer), func(t *testing.T) {
			d := start(t, storage.NewDemo())
			clickText := func(label string) {
				t.Helper()
				for y, row := range strings.Split(d.screen(), "\n") {
					if i := strings.Index(row, label); i >= 0 {
						x := ansi.StringWidth(row[:i])
						d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
						d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
						return
					}
				}
				t.Fatalf("missing clickable text %q:\n%s", label, d.screen())
			}
			d.key("j")
			d.key("j") // README.md: capture must not move this selection.
			if pointer {
				clickText("Actions ctrl+p")
			} else {
				d.key("ctrl+p")
			}
			d.key("qj?")
			d.key("enter")
			requireText(t, d, "No results")
			if d.quit {
				t.Fatal("palette query escaped to workspace")
			}
			d.key("ctrl+p") // Dismiss, not old previous-result alias.
			if strings.Contains(d.screen(), "ACTIONS · Ctrl+P") {
				t.Fatal("Ctrl+P did not dismiss")
			}
			d.key("ctrl+p")
			d.key("theme")
			selectedTheme := func() string {
				t.Helper()
				for _, row := range strings.Split(d.screen(), "\n") {
					if _, label, ok := strings.Cut(row, "› Theme: "); ok {
						name, _, _ := strings.Cut(label, "|")
						return "Theme: " + strings.TrimSpace(name)
					}
				}
				t.Fatalf("missing selected theme:\n%s", d.screen())
				return ""
			}
			first := selectedTheme()
			for range 30 {
				d.key("shift+tab")
			}
			if selectedTheme() != first {
				t.Fatal("backwards navigation wrapped past the first result")
			}
			d.key("enter")
			requireText(t, d, first)
			d.key("ctrl+p")
			d.key("theme")
			for range 30 {
				d.key("tab")
			}
			last := selectedTheme()
			d.key("tab")
			if selectedTheme() != last || first == last {
				t.Fatal("forward navigation did not clamp at the last result")
			}
			d.key("enter")
			requireText(t, d, last)
			d.key("ctrl+p")
			// This top control is outside the captured palette. Its press must
			// only dismiss, never toggle the inspector behind it.
			clickText("[ Info ]")
			if strings.Contains(d.screen(), "INSPECTOR") {
				t.Fatal("outside dismiss activated the background inspector")
			}
			if pointer {
				clickText("Actions ctrl+p")
				d.key("file inspector")
				d.deliver(tea.WindowSizeMsg{Width: 120, Height: 24})
				clickText("Toggle file inspector")
			} else {
				d.key("]")
				d.deliver(tea.WindowSizeMsg{Width: 120, Height: 24})
			}
			requireText(t, d, "INSPECTOR")
			parts := strings.Split(d.screen(), "INSPECTOR")
			if len(parts) != 2 || !strings.Contains(parts[1], "README.md") {
				t.Fatal("palette changed the inspected target or activated twice")
			}
		})
	}
}

func TestFilteringConsumesShortcutsAndUnicode(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("/")
	d.key("配置x")
	d.key("ctrl+h")
	d.key("ctrl+l")
	d.key("enter")
	requireText(t, d, "配置.json")
	if strings.Contains(d.screen(), "README.md") {
		t.Fatal("filter was not applied")
	}
	d.key("/")
	d.key("esc")
	requireText(t, d, "README.md")
	d.key("/")
	d.key("project")
	d.deliver(tea.KeyMsg{Type: tea.KeySpace})
	d.key("notes")
	d.key("enter")
	requireText(t, d, "project notes.txt")
	if strings.Contains(d.screen(), "README.md") {
		t.Fatal("literal space was lost in filename filter")
	}
	d.key("/")
	d.key("esc")
	d.key("/")
	d.key("q[]t")
	if d.quit {
		t.Fatal("filter input quit")
	}
	d.key("esc")
	requireText(t, d, "WORKSPACE")
}

func TestStaleListingCannotReplaceNewLocation(t *testing.T) {
	d := start(t, storage.NewDemo())
	next, old := d.model.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	d.model = next
	d.key("j")
	d.key("enter")
	requireText(t, d, "browser.go")
	d.run(old)
	requireText(t, d, "browser.go")
	if strings.Contains(d.screen(), "README.md") {
		t.Fatal("stale root listing replaced directory contents")
	}
}

func TestListingFailureAndRetry(t *testing.T) {
	fail := true
	demo := storage.NewDemo()
	b := browserFunc(func(ctx context.Context, l storage.Location) ([]storage.Entry, error) {
		if fail && l.Remote == "" {
			return nil, errors.New("fixture unavailable")
		}
		return demo.List(ctx, l)
	})
	d := start(t, b)
	requireText(t, d, "fixture unavailable")
	requireText(t, d, "orders.csv")
	fail = false
	d.key("ctrl+r")
	requireText(t, d, "README.md")
	if strings.Contains(d.screen(), "fixture unavailable") {
		t.Fatal("retry left stale error")
	}
}

func TestLayoutVisibilityAndCellBounds(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("]")
	d.key("t")
	for _, size := range [][2]int{{160, 40}, {100, 30}, {80, 24}, {60, 12}, {50, 10}, {160, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			if size[0] >= 60 && size[0] < 110 {
				lines := strings.Split(d.screen(), "\n")
				if !strings.Contains(lines[len(lines)-1], "? help") {
					t.Fatal("narrow footer lost help")
				}
			}
			for _, modal := range []bool{false, true} {
				if modal {
					d.key("ctrl+p")
					if size[0] >= 60 {
						requireText(t, d, "ACTIONS · Ctrl+P")
						requireText(t, d, "unavailable")
						if size[0] >= 80 {
							requireText(t, d, "Copy to right pane")
						}
					}
				}
				lines := strings.Split(d.model.View(), "\n")
				if len(lines) > size[1] {
					t.Fatalf("height %d > %d", len(lines), size[1])
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > size[0] {
						t.Fatalf("line width %d > %d", ansi.StringWidth(line), size[0])
					}
				}
				if modal {
					d.key("esc")
				}
			}
			if size[0] == 160 {
				requireText(t, d, "INSPECTOR")
				requireText(t, d, "WORKSPACE")
				requireText(t, d, "TRANSFERS")
				lines := strings.Split(d.screen(), "\n")
				bottom := []rune(lines[size[1]-3])
				if bottom[0] != '└' || bottom[21] != '┘' || bottom[132] != '└' {
					t.Fatalf("sidebars do not extend beside drawer: %s", lines[size[1]-3])
				}
			}
		})
	}
}

func TestUntrustedFilenameIsRenderedAsText(t *testing.T) {
	b := browserFunc(func(context.Context, storage.Location) ([]storage.Entry, error) {
		return []storage.Entry{{Name: "hello\x1b[2J\nworld.txt"}, {Name: "配置.json"}}, nil
	})
	d := start(t, b)
	requireText(t, d, "helloworld.txt")
	requireText(t, d, "配置.json")
	if strings.Contains(d.model.View(), "\x1b[2J") {
		t.Fatal("entry injected a terminal clear")
	}
}

func TestMouseBrowsingAndShortcutGuide(t *testing.T) {
	for _, size := range [][2]int{{160, 40}, {80, 24}, {60, 12}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			d := start(t, storage.NewDemo())
			d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			locate := func(label string) (int, int) {
				t.Helper()
				for y, row := range strings.Split(d.screen(), "\n") {
					if i := strings.Index(row, label); i >= 0 {
						return ansi.StringWidth(row[:i]), y
					}
				}
				t.Fatalf("missing clickable text %q:\n%s", label, d.screen())
				return 0, 0
			}
			click := func(x, y int) {
				d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
			}
			clickText := func(label string) {
				x, y := locate(label)
				click(x, y)
			}
			x, y := locate("src/")
			click(x-4, y)
			requireText(t, d, "1 selected")
			if strings.Contains(d.screen(), "LOCAL /project/src") {
				t.Fatal("selection marker opened directory")
			}
			click(x, y)
			requireText(t, d, "src/")
			if strings.Contains(d.screen(), "browser.go") {
				t.Fatal("row press opened directory")
			}
			if size[0] == 160 {
				clickText("Open/Inspect")
			} else {
				d.key("enter")
			}
			requireText(t, d, "browser.go")
			clickText("↑ Up")
			requireText(t, d, "src/")
			clickText("Actions ctrl+p")
			requireText(t, d, "ACTIONS · Ctrl+P")
			d.key("f1")
			requireText(t, d, " HELP ")
			for range 40 {
				d.deliver(tea.MouseMsg{X: size[0] / 2, Y: size[1] / 2, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
			}
			requireText(t, d, "Ctrl+P / Esc")
			clickText("[x]")
			requireText(t, d, "ACTIONS · Ctrl+P")
			clickText("[x]")
			// Filtering keeps result hit regions aligned, including narrow screens.
			clickText("Actions ctrl+p")
			d.key("kybrd")
			clickText("Keyboard shortcuts")
			requireText(t, d, " HELP ")
			clickText("[x]")
			requireText(t, d, "Query: kybrd")
			clickText("[x]")
			x, y = locate("src/")
			clickText("Actions ctrl+p")
			// Dismissal must consume the click, not open the directory below.
			if size[0] < 160 {
				x, y = 0, 0 // the narrow modal covers the whole file row
			}
			click(x, y)
			if strings.Contains(d.screen(), "LOCAL /project/src") || strings.Contains(d.screen(), "ACTIONS · Ctrl+P") {
				t.Fatal("outside click did not dismiss modal safely")
			}
			clickText("src/")
			d.key("enter")
			requireText(t, d, "browser.go")
			if size[0] == 160 {
				clickText("orders.csv") // Sidebar locations target the last focused pane.
				clickText("demo-blob /assets")
				requireText(t, d, "hero image.png")
				clickText("icons/")
				d.key("enter")
				requireText(t, d, "logo.svg")
				requireText(t, d, "browser.go") // remote navigation leaves local untouched
				clickText("Actions ctrl+p")
				clickText("Copy to left pane")
				requireText(t, d, "demo is read-only")
				requireText(t, d, "ACTIONS · Ctrl+P")
				d.key("kybrd")
				clickText("Keyboard shortcuts")
				requireText(t, d, " HELP ")
			}
		})
	}
}

func TestMouseSelectsScrolledRow(t *testing.T) {
	b := browserFunc(func(context.Context, storage.Location) ([]storage.Entry, error) {
		entries := make([]storage.Entry, 40)
		for i := range entries {
			entries[i].Name = fmt.Sprintf("file-%02d", i)
		}
		return entries, nil
	})
	d := start(t, b)
	d.deliver(tea.WindowSizeMsg{Width: 80, Height: 12})
	d.deliver(tea.MouseMsg{X: 10, Y: 4, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	d.deliver(tea.MouseMsg{X: 10, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	requireText(t, d, "› ▤ file-02")
	d.key("/")
	d.key("17")
	d.key("enter")
	d.deliver(tea.MouseMsg{X: 1, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	requireText(t, d, "● ▤ file-17")
	requireText(t, d, "1 selected")
}

func TestFileIcons(t *testing.T) {
	groups := []struct {
		extensions, portable, nerd string
	}{
		{"md markdown", "≡", "\ue73e"},
		{"txt rst pdf", "▤", "\uf15c"},
		{"json csv yaml yml toml xml", "⌘", "\ue60b"},
		{"png jpg jpeg gif svg webp", "◈", "\uf03e"},
		{"bmp ico tif tiff", "▤", "\uf03e"},
		{"go", "λ", "\ue627"},
		{"rs", "λ", "\ue7a8"},
		{"js", "λ", "\ue74e"},
		{"jsx mjs cjs", "▤", "\ue74e"},
		{"ts", "λ", "\ue628"},
		{"tsx", "▤", "\ue628"},
		{"py", "λ", "\ue73c"},
		{"pyw", "▤", "\ue73c"},
		{"sh", "λ", "\uf489"},
		{"bash zsh fish", "▤", "\uf489"},
		{"c h cpp hpp java rb php html htm css scss sql", "▤", "\uf121"},
		{"zip gz tar", "▣", "\uf410"},
		{"tgz bz2 xz 7z rar zst", "▤", "\uf410"},
		{"unknown", "▤", "\uf15b"},
	}
	cases := []struct {
		name, portable, nerd string
		dir                  bool
	}{
		{"folder.json", "▸", "\uf07b", true},
		{"..", "↑", "↑", true},
		{"LICENSE", "▤", "\uf15b", false},
	}
	for _, group := range groups {
		for _, ext := range strings.Fields(group.extensions) {
			for _, suffix := range []string{ext, strings.ToUpper(ext)} {
				cases = append(cases, struct {
					name, portable, nerd string
					dir                  bool
				}{"file." + suffix, group.portable, group.nerd, false})
			}
		}
	}
	for _, tc := range cases {
		for _, icons := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/icons=%t", tc.name, icons), func(t *testing.T) {
				th, err := theme.Get("")
				if err != nil {
					t.Fatal(err)
				}
				b := browserFunc(func(context.Context, storage.Location) ([]storage.Entry, error) {
					return []storage.Entry{{Name: tc.name, IsDir: tc.dir}}, nil
				})
				m := ui.New(ui.Options{Browser: b, Theme: th, Icons: icons, Left: storage.Location{Path: "/project"}})
				d := &driver{model: m}
				d.deliver(tea.WindowSizeMsg{Width: 160, Height: 20})
				d.run(m.Init())
				icon := tc.portable
				if icons {
					icon = tc.nerd
				}
				name := tc.name
				if tc.dir && name != ".." {
					name += "/"
				}
				requireText(t, d, icon+strings.Repeat(" ", 2-ansi.StringWidth(icon))+name)
				for _, row := range strings.Split(d.screen(), "\n") {
					if ansi.StringWidth(row) != 160 {
						t.Fatalf("row width = %d, want 160: %q", ansi.StringWidth(row), row)
					}
					if at := strings.Index(row, icon+strings.Repeat(" ", 2-ansi.StringWidth(icon))+name); at >= 0 {
						if column := ansi.StringWidth(row[:at]) + 2; column != 27 {
							t.Fatalf("filename column = %d, want 27", column)
						}
					}
				}
			})
		}
	}
}

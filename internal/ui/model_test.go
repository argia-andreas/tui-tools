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
	special := map[string]tea.KeyType{"tab": tea.KeyTab, "enter": tea.KeyEnter, "esc": tea.KeyEsc, "ctrl+p": tea.KeyCtrlP, "ctrl+r": tea.KeyCtrlR, "backspace": tea.KeyBackspace, "shift+tab": tea.KeyShiftTab}
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

func TestPaletteOwnsInputAndExecutesEnabledActions(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("ctrl+p")
	d.key("quit[]")
	d.key("enter")
	if d.quit {
		t.Fatal("typing quit executed a global action")
	}
	requireText(t, d, "No matching actions")
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
}

func TestFilteringConsumesShortcutsAndUnicode(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("/")
	d.key("配置")
	d.key("enter")
	requireText(t, d, "配置.json")
	if strings.Contains(d.screen(), "README.md") {
		t.Fatal("filter was not applied")
	}
	d.key("/")
	d.key("esc")
	requireText(t, d, "README.md")
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
						requireText(t, d, "Copy to right pane")
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

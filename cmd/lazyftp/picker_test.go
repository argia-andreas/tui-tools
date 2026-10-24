package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/connections"
	"lazyftp/internal/theme"
)

func key(m picker, s string) picker {
	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	switch s {
	case "tab":
		k.Type = tea.KeyTab
	case "enter":
		k.Type = tea.KeyEnter
	case "down":
		k.Type = tea.KeyDown
	case "ctrl+u":
		k.Type = tea.KeyCtrlU
	case "esc":
		k.Type = tea.KeyEsc
	case " ":
		k = tea.KeyMsg{Type: tea.KeySpace}
	}
	next, _ := m.Update(k)
	return next.(picker)
}

// Find the visible label rather than coupling interactions to card positioning.
func pickerClick(t *testing.T, m picker, label string) picker {
	t.Helper()
	msg := pickerLabelMouse(t, m, label)
	next, _ := m.Update(msg)
	return next.(picker)
}

func pickerLabelMouse(t *testing.T, m picker, label string) tea.MouseMsg {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			return tea.MouseMsg{X: ansi.StringWidth(row[:x]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
		}
	}
	t.Fatalf("label %q not visible:\n%s", label, m.View())
	return tea.MouseMsg{}
}

func TestPickerExplicitSaveThenReuseAndConnect(t *testing.T) {
	theme, _ := theme.Get("")
	local := t.TempDir()
	file := filepath.Join(t.TempDir(), "bookmarks.json")
	m := newPicker([]string{"ftp", "blob", "s3"}, nil, local, "", file, theme)
	m = key(m, "down") // choose Blob, not the first remote
	m = key(m, "enter")
	m = key(m, "ctrl+u")
	m = key(m, "container/space å")
	m = key(m, "tab")
	m = key(m, "ctrl+u")
	m = key(m, local)
	m = key(m, "tab")
	m = key(m, "Research")
	m = key(m, " ")
	m = key(m, "files")
	m = key(m, "tab")
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("saved without explicit action")
	}
	m = key(m, "tab")
	m = key(m, "enter")
	if m.selected != nil || !strings.Contains(m.View(), "Bookmark saved") {
		t.Fatalf("save should not connect: %s", m.View())
	}
	saved, err := connections.Load(file)
	if err != nil || len(saved) != 1 || saved[0].Name != "Research files" || saved[0].Remote != "blob" || saved[0].Root != "/container/space å" {
		t.Fatalf("saved %+v %v", saved, err)
	}
	reused := newPicker(nil, saved, "/different-default", "", file, theme)
	// Clicking Connect uses the saved local directory and root.
	reused = pickerClick(t, reused, "Research files")
	reused = pickerClick(t, reused, "[ Connect ]")
	got := reused.selected
	if got == nil || got.Local != local || got.Root != "/container/space å" || got.Remote != "blob" {
		t.Fatalf("connection %+v", got)
	}
	override := newPicker(nil, saved, local, "/explicit-left", file, theme)
	override = key(override, "enter")
	if !strings.Contains(override.View(), "/explicit-left") {
		t.Fatal("explicit --left lost")
	}
}

func TestPickerMouseScrollEmptyCancelAndSizes(t *testing.T) {
	theme, _ := theme.Get("")
	m := newPicker([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, nil, t.TempDir(), "", filepath.Join(t.TempDir(), "bookmarks"), theme)
	for i := 0; i < 7; i++ {
		next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
		m = next.(picker)
	}
	if !strings.Contains(m.View(), "h:") {
		t.Fatal("last remote not visible")
	}
	m = pickerClick(t, m, "f:")
	for _, s := range []string{"tab", "tab", "tab", "enter"} {
		m = key(m, s)
	}
	if m.selected == nil || m.selected.Remote != "f" {
		t.Fatalf("scrolled row selected wrong remote: %+v", m.selected)
	}
	for _, width := range []int{60, 80, 140} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		for _, row := range strings.Split(next.View(), "\n") {
			if ansi.StringWidth(row) > width {
				t.Fatalf("overflow at %d: %q", width, row)
			}
		}
	}
	empty := newPicker(nil, nil, t.TempDir(), "", "", theme)
	for i := 0; i < 4; i++ {
		empty = key(empty, "tab")
	}
	empty = key(empty, "enter")
	if empty.selected != nil || !empty.newConnection {
		t.Fatal("empty discovery connected")
	}
	empty = key(empty, "esc")
	if empty.selected != nil {
		t.Fatal("cancel connected")
	}
}

func TestGenericRemoteFlagsRetainAllBackends(t *testing.T) {
	var remotes remoteFlags
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.Var(&remotes, "remote", "")
	if err := flags.Parse([]string{"--remote", "ftp:/incoming", "--remote", "blob:/container", "--remote", "s3:/bucket/prefix"}); err != nil {
		t.Fatal(err)
	}
	if len(remotes) != 3 || remotes[0] != "ftp:/incoming" || remotes[1] != "blob:/container" || remotes[2] != "s3:/bucket/prefix" {
		t.Fatalf("roots %v", remotes)
	}
	if err := remotes.Set(":ftp,host=example:/"); err == nil {
		t.Fatal("inline backend accepted")
	}
}

func TestPickerQuietFieldsAndExplicitHelp(t *testing.T) {
	theme, _ := theme.Get("")
	m := newPicker([]string{"ftp"}, nil, t.TempDir(), "", "", theme)
	if strings.Contains(m.View(), "Credentials stay") || strings.Contains(m.View(), "[ Connect ]") || strings.Contains(m.View(), "[ Save bookmark ]") {
		t.Fatalf("unexpected picker guidance: %s", m.View())
	}
	click := pickerLabelMouse(t, m, "New connection")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = next.(picker)
	if !strings.Contains(m.View(), "Credentials stay") {
		t.Fatal("F1 did not expose detailed guidance")
	}
	// Hidden controls must not activate through the help screen.
	next, _ = m.Update(click)
	m = next.(picker)
	if m.selected != nil || m.newConnection {
		t.Fatal("help click connected")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = next.(picker)
	m = key(m, "enter") // root field accepts a literal question mark
	m = key(m, "?")
	if !strings.Contains(m.View(), "Remote root    /?") || strings.Contains(m.View(), "Connection help") {
		t.Fatal("help stole field typing")
	}
}

func TestPickerConnectionFirstCardsAndNewConnection(t *testing.T) {
	theme, _ := theme.Get("")
	for _, size := range [][2]int{{60, 21}, {80, 24}, {140, 45}} {
		for _, remotes := range [][]string{nil, {"ftp", "blob", "s3", "four", "five", "six", "seven"}} {
			m := newPicker(remotes, nil, t.TempDir(), "", "", theme)
			next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m = next.(picker)
			view := ansi.Strip(m.View())
			for _, label := range []string{"LAZYFTP", "Open a connection", "New connection", "Esc local files", "F1 help"} {
				if !strings.Contains(view, label) {
					t.Fatalf("missing %q: %s", label, view)
				}
			}
			for _, label := range []string{"Search", "Remote root", "Local folder", "Bookmark name", "password", "[ Connect ]"} {
				if strings.Contains(view, label) {
					t.Fatalf("initial chooser exposes %q", label)
				}
			}
			rows := strings.Split(view, "\n")
			if len(rows) != size[1] {
				t.Fatalf("height %d, want %d", len(rows), size[1])
			}
			for _, row := range rows {
				if ansi.StringWidth(row) != size[0] {
					t.Fatalf("wrong row width: %q", row)
				}
				if x := strings.Index(row, "┌"); x >= 0 {
					left := ansi.StringWidth(row[:x])
					right := ansi.StringWidth(row[strings.Index(row, "┐")+len("┐"):])
					if left < 1 || left-right > 1 || right-left > 1 {
						t.Fatalf("card not centered: %q", row)
					}
				}
			}
			clicked := pickerClick(t, m, "New connection")
			if !clicked.newConnection || clicked.selected != nil {
				t.Fatal("New connection click did not request wizard")
			}
			if len(remotes) > 0 {
				m = key(m, "tab")
			}
			m = key(m, "enter")
			if !m.newConnection || m.selected != nil {
				t.Fatal("New connection keyboard did not request wizard")
			}
		}
	}
}

func TestPickerDetailsBackAndChooserCancel(t *testing.T) {
	theme, _ := theme.Get("")
	m := newPicker([]string{"ftp"}, nil, t.TempDir(), "", "", theme)
	m = key(m, "enter")
	for _, label := range []string{"Connection details", "Remote root", "Local folder", "Bookmark name", "[ Connect ]", "[ Save bookmark ]"} {
		if !strings.Contains(m.View(), label) {
			t.Fatalf("missing details label %q", label)
		}
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(picker)
	if cmd != nil || !strings.Contains(m.View(), "Open a connection") || strings.Contains(m.View(), "Remote root") {
		t.Fatal("details Escape did not return to chooser")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("chooser Escape did not request Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok || next.(picker).selected != nil {
		t.Fatal("chooser Escape must cancel without selecting")
	}
}

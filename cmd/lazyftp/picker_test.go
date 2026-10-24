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
	next, _ := reused.Update(tea.MouseMsg{X: 5, Y: 14, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	got := next.(picker).selected
	if got == nil || got.Local != local || got.Root != "/container/space å" || got.Remote != "blob" {
		t.Fatalf("connection %+v", got)
	}
	override := newPicker(nil, saved, local, "/explicit-left", file, theme)
	if !strings.Contains(override.View(), "/explicit-left") {
		t.Fatal("explicit --left lost")
	}
}

func TestPickerMouseScrollEmptyCancelAndSizes(t *testing.T) {
	theme, _ := theme.Get("")
	m := newPicker([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, nil, t.TempDir(), "", filepath.Join(t.TempDir(), "bookmarks"), theme)
	for i := 0; i < 7; i++ {
		m = key(m, "down")
	}
	if !strings.Contains(m.View(), "> h:") {
		t.Fatal("last remote not visible")
	}
	next, _ := m.Update(tea.MouseMsg{X: 4, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = next.(picker)
	for _, s := range []string{"enter", "tab", "tab", "tab", "enter"} {
		m = key(m, s)
	}
	if m.selected == nil || m.selected.Remote != "c" {
		t.Fatalf("scrolled row selected wrong remote: %+v", m.selected)
	}
	for _, width := range []int{60, 80, 140} {
		next, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
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
	if empty.selected != nil || !strings.Contains(empty.View(), "No remotes") {
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
	if strings.Contains(m.View(), "Credentials stay") || !strings.Contains(m.View(), "[ Connect ]") || !strings.Contains(m.View(), "[ Save bookmark ]") {
		t.Fatalf("unexpected picker guidance: %s", m.View())
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = next.(picker)
	if !strings.Contains(m.View(), "Credentials stay") {
		t.Fatal("F1 did not expose detailed guidance")
	}
	// Hidden controls must not activate through the help screen.
	next, _ = m.Update(tea.MouseMsg{X: 5, Y: 14, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = next.(picker)
	if m.selected != nil {
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

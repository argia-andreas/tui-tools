package ui_test

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

type memoryStorage struct {
	files map[storage.Location]string
	fail  bool
}

func (s *memoryStorage) List(_ context.Context, loc storage.Location) ([]storage.Entry, error) {
	var result []storage.Entry
	for file, data := range s.files {
		if file.Remote == loc.Remote && path.Dir(file.Path) == loc.Path {
			result = append(result, storage.Entry{Name: path.Base(file.Path), Size: int64(len(data))})
		}
	}
	return result, nil
}
func (s *memoryStorage) Stat(_ context.Context, loc storage.Location) (storage.Entry, error) {
	data, ok := s.files[loc]
	if !ok {
		return storage.Entry{}, fs.ErrNotExist
	}
	return storage.Entry{Name: path.Base(loc.Path), Size: int64(len(data))}, nil
}
func (s *memoryStorage) Copy(_ context.Context, source, destination storage.Location, overwrite bool) error {
	if s.fail {
		return errors.New("connection interrupted")
	}
	if _, exists := s.files[destination]; exists && !overwrite {
		return fs.ErrExist
	}
	s.files[destination] = s.files[source]
	return nil
}

func TestFilesFirstDestinationAndParentNavigation(t *testing.T) {
	th, _ := theme.Get("")
	s := &treeStorage{files: map[storage.Location]string{
		{Path: "/local/child/local.txt"}:                    "local",
		{Remote: "ftp", Path: "/incoming/child/remote.txt"}: "remote",
	}}
	m := ui.New(ui.Options{Browser: s, Transferer: s, Left: storage.Location{Path: "/local"}, Theme: th, Disconnected: true})
	d := &driver{model: m}
	d.deliver(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.run(m.Init())
	requireText(t, d, "child/")
	requireText(t, d, "REMOTE Not connected")
	requireText(t, d, "[ Connect a remote ]")
	for _, size := range [][2]int{{160, 40}, {80, 24}, {60, 12}, {100, 30}} {
		d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		requireText(t, d, "[ Connect a remote ]")
		lines := strings.Split(d.model.View(), "\n")
		if len(lines) != size[1] {
			t.Fatalf("first-screen height %d, want %d", len(lines), size[1])
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("first-screen content overflowed terminal width")
			}
		}
	}
	for _, clutter := range []string{"WORKSPACE", "INSPECTOR", "TRANSFERS", "Loading", "LIVE MODE"} {
		if strings.Contains(d.screen(), clutter) {
			t.Fatalf("unexpected first-screen clutter %q", clutter)
		}
	}
	d.key("c")
	requireText(t, d, "Choose a destination before copying")
	d.key("tab")
	_, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	request, ok := cmd().(ui.ConnectRequested)
	if !ok || request.Local != "/local" {
		t.Fatalf("connection request: %+v", request)
	}
	d.deliver(ui.ConnectionOpened{Right: storage.Location{Remote: "ftp", Path: "/incoming"}})
	for _, endpoint := range []struct{ child, file, parent string }{
		{"LOCAL /local/child", "local.txt", "LOCAL /local"},
		{"REMOTE ftp:/incoming/child", "remote.txt", "REMOTE ftp:/incoming"},
	} {
		d.key("enter") // The first real directory remains the initial cursor.
		requireText(t, d, endpoint.child)
		requireText(t, d, endpoint.file)
		d.key("k") // Parent is immediately above the first actual entry.
		d.key(" ")
		requireText(t, d, "0 selected")
		d.key("c")
		requireText(t, d, "No file to copy")
		d.key("enter")
		requireText(t, d, endpoint.parent)
		if strings.Contains(d.screen(), endpoint.child) {
			t.Fatal("Enter on .. did not leave the child directory")
		}
		d.key("tab")
	}
	// Mouse navigation must use the same synthetic row in either file pane.
	d.key("enter")
	requireText(t, d, "LOCAL /local/child")
	d.deliver(tea.MouseMsg{X: 5, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	requireText(t, d, "LOCAL /local")
	if strings.Contains(d.screen(), "LOCAL /local/child") {
		t.Fatal("clicking .. did not navigate to parent")
	}
	d.deliver(ui.ConnectionOpened{Left: storage.Location{Path: "/"}, Right: storage.Location{Remote: "ftp", Path: "/"}})
	if strings.Contains(d.screen(), "↑ ..") {
		t.Fatal("root directories should not show a parent entry")
	}
}

func TestConfirmedSingleFileCopies(t *testing.T) {
	for _, scenario := range []string{"cancel", "skip", "overwrite", "mouse", "rename", "failure", "new", "download", "frozen"} {
		t.Run(scenario, func(t *testing.T) {
			local := storage.Location{Path: "/local/source.txt"}
			remote := storage.Location{Remote: "ftp", Path: "/incoming/source.txt"}
			s := &memoryStorage{files: map[storage.Location]string{local: "local payload", remote: "remote ORIGINAL"}}
			if scenario == "new" {
				delete(s.files, remote)
			}
			th, _ := theme.Get("")
			m := ui.New(ui.Options{Browser: s, Transferer: s, Left: storage.Location{Path: "/local"}, Right: storage.Location{Remote: "ftp", Path: "/incoming"}, Locations: []storage.Location{{Remote: "ftp", Path: "/incoming"}, {Remote: "blob", Path: "/assets"}, {Path: "/local"}}, Theme: th})
			d := &driver{model: m}
			d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
			d.run(m.Init())
			if scenario == "download" {
				d.key("tab")
			}
			d.key("c")
			if scenario == "new" {
				requireText(t, d, "Destination is new")
			} else {
				requireText(t, d, "DESTINATION EXISTS")
			}
			for _, size := range [][2]int{{60, 12}, {80, 24}, {160, 40}} {
				d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				lines := strings.Split(d.screen(), "\n")
				if len(lines) > size[1] {
					t.Fatal("copy modal exceeds terminal height")
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("copy modal exceeds terminal width")
					}
				}
				requireText(t, d, "[Cancel]")
			}
			switch scenario {
			case "cancel":
				d.key("enter")
			case "skip":
				d.key("tab")
				d.key("enter")
				requireText(t, d, "Skipped")
			case "rename":
				d.key("shift+tab")
				d.key("enter")
				requireText(t, d, "RENAME DESTINATION")
				d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
				d.key("../escape")
				d.key("enter")
				requireText(t, d, "single filename")
				d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
				d.key("renamed")
				d.deliver(tea.KeyMsg{Type: tea.KeySpace})
				d.key("雪.txt")
				d.key("enter")
				requireText(t, d, "Destination is new")
				d.key("tab")
				d.key("enter")
				requireText(t, d, "Completed")
				if s.files[storage.Location{Remote: "ftp", Path: "/incoming/renamed 雪.txt"}] != "local payload" {
					t.Fatal("renamed destination has wrong bytes")
				}
			default:
				d.key("tab")
				if scenario != "new" {
					d.key("tab")
				}
				s.fail = scenario == "failure"
				if scenario == "frozen" {
					next, copyCmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
					d.model = next
					d.key("[") // Live workspaces start with navigation collapsed.
					d.key("shift+tab")
					d.key("j")
					d.key("enter")
					requireText(t, d, "blob:/assets")
					d.run(copyCmd)
					requireText(t, d, "blob:/assets")
					if _, ok := s.files[storage.Location{Remote: "blob", Path: "/assets/source.txt"}]; ok {
						t.Fatal("running copy was retargeted")
					}
				} else if scenario == "mouse" {
					for y, row := range strings.Split(d.screen(), "\n") {
						if x := strings.Index(row, "[Overwrite]"); x >= 0 {
							d.deliver(tea.MouseMsg{X: ansi.StringWidth(row[:x]) + 1, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
							break
						}
					}
				} else {
					d.key("enter")
				}
				if s.fail {
					requireText(t, d, "Failed")
					requireText(t, d, "connection interrupted")
				} else {
					requireText(t, d, "Completed")
				}
			}
			wantRemote, wantLocal := "remote ORIGINAL", "local payload"
			if scenario == "new" || scenario == "overwrite" || scenario == "mouse" || scenario == "frozen" {
				wantRemote = "local payload"
			}
			if scenario == "download" {
				wantLocal = "remote ORIGINAL"
			}
			if s.files[remote] != wantRemote || s.files[local] != wantLocal {
				t.Fatalf("wrong bytes after %s: %+v", scenario, s.files)
			}
			if strings.Contains(d.screen(), "DESTINATION EXISTS") {
				t.Fatal("confirmation did not close")
			}
		})
	}
}

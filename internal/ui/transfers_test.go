package ui_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
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

func TestIndependentPaneDetachAndReattach(t *testing.T) {
	for target := range 2 {
		for _, fromRemote := range []bool{false, true} {
			for _, toRemote := range []bool{false, true} {
				t.Run(fmt.Sprintf("pane%d/remote%t-to-%t", target, fromRemote, toRemote), func(t *testing.T) {
					initial := storage.Location{Path: "/start"}
					if fromRemote {
						initial.Remote = "original"
					}
					opposite := storage.Location{Remote: "opposite", Path: "/keep"}
					replacement := storage.Location{Path: t.TempDir()}
					if toRemote {
						replacement = storage.Location{Remote: "replacement", Path: "/new"}
					}
					file := func(loc storage.Location, name string) storage.Location {
						loc.Path = path.Join(loc.Path, name)
						return loc
					}
					s := &memoryStorage{files: map[storage.Location]string{file(initial, "old-file.txt"): "old", file(opposite, "keep-file.txt"): "keep", file(replacement, "new-file.txt"): "new"}}
					locations := [2]storage.Location{}
					locations[target], locations[1-target] = initial, opposite
					th, _ := theme.Get("")
					m := ui.New(ui.Options{Browser: s, Transferer: s, Left: locations[0], Right: locations[1], Theme: th})
					d := &driver{model: m}
					d.deliver(tea.WindowSizeMsg{Width: 200, Height: 30})
					d.run(m.Init())
					focus := func(i int) {
						d.deliver(tea.MouseMsg{X: 100*i + 3, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
					}
					focus(1 - target)
					d.key("/")
					d.key("keep")
					d.key("enter")
					d.key(" ")
					focus(target)
					paneText := func(i int) string {
						var rows []string
						for _, row := range strings.Split(d.screen(), "\n")[1:29] {
							chars := []rune(row)
							rows = append(rows, string(chars[i*100:(i+1)*100]))
						}
						return strings.Join(rows, "\n")
					}
					before := paneText(1 - target)
					d.key("ctrl+p")
					d.key("detach")
					d.key("enter")
					requireText(t, d, []string{"LEFT", "RIGHT"}[target]+" · No source")
					for _, label := range []string{"Local files", "Saved connection", "New connection"} {
						requireText(t, d, label)
					}
					if strings.Contains(d.screen(), "old-file.txt") || before != paneText(1-target) {
						t.Fatal("detach retained old content or changed the opposite pane")
					}
					d.key("enter") // Local is first; cancel leaves the pane detached.
					d.key("esc")
					requireText(t, d, "No source")
					if toRemote {
						d.key("j")
						next, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
						d.model = next
						request := cmd().(ui.ConnectRequested)
						if request.Pane != target || !request.Destination || request.New {
							t.Fatalf("wrong target: %+v", request)
						}
						d.deliver(ui.ConnectionOpened{Pane: request.Pane, Location: replacement})
					} else {
						d.key("enter")
						d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
						d.key(replacement.Path)
						d.key("enter")
					}
					requireText(t, d, "new-file.txt")
					if before != paneText(1-target) {
						t.Fatal("attachment changed the opposite pane's folder/filter/cursor/selection")
					}
					if toRemote {
						d.key("c")
						requireText(t, d, "Remote-to-remote copies are not supported yet")
					}
				})
			}
		}
	}
}

func TestDetachedPaneRejectsLateListingsAndFileActions(t *testing.T) {
	th, _ := theme.Get("")
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{{Path: "/start/old.txt"}: "old"}}}
	m := ui.New(ui.Options{Browser: s, Transferer: s, Left: storage.Location{Path: "/start"}, Theme: th, Disconnected: true})
	d := &driver{model: m}
	d.deliver(tea.WindowSizeMsg{Width: 160, Height: 30})
	d.run(m.Init())
	next, late := d.model.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	d.model = next
	d.key("ctrl+p")
	d.key("detach")
	d.key("enter")
	s.files[storage.Location{Path: "/start/late.txt"}] = "late"
	d.run(late)
	for _, action := range []struct{ key, reason string }{{"ctrl+r", "Choose a source before refreshing"}, {"c", "Choose a source before copying"}, {"p", "Choose a source before previewing"}, {"e", "Choose a source before editing"}, {"D", "Choose a connected file pane"}, {"R", "Connect a remote or open a local folder first"}} {
		d.key(action.key)
		requireText(t, d, action.reason)
	}
	d.key("backspace")
	d.key("/")
	d.key(" ")
	if strings.Contains(d.screen(), "old.txt") || strings.Contains(d.screen(), "late.txt") || strings.Count(d.screen(), "No source") != 2 {
		t.Fatal("detached pane repopulated or file input reattached it")
	}
	d.deliver(ui.ConnectionOpened{Pane: 0, Location: storage.Location{Path: filepath.Dir(t.TempDir())}})
	d.run(late)
	if strings.Contains(d.screen(), "late.txt") {
		t.Fatal("old listing replaced a new attachment")
	}
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
	requireText(t, d, "RIGHT · No source")
	requireText(t, d, "[ Saved connection ]")
	for _, size := range [][2]int{{160, 40}, {80, 24}, {60, 12}, {100, 30}} {
		d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		requireText(t, d, "[ Saved connection ]")
		requireText(t, d, "[ Local files ]")
		requireText(t, d, "[ New connection ]")
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
	d.key("j") // Local files is the default; saved connections are second.
	_, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	request, ok := cmd().(ui.ConnectRequested)
	if !ok || request.Local != "/local" || request.Pane != 1 || !request.Destination {
		t.Fatalf("connection request: %+v", request)
	}
	d.deliver(ui.ConnectionOpened{Pane: 1, Location: storage.Location{Remote: "ftp", Path: "/incoming"}})
	d.key("tab") // Opening now returns focus to the chosen pane, not always left.
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
	d.deliver(ui.ConnectionOpened{Pane: 0, Location: storage.Location{Path: "/"}})
	d.deliver(ui.ConnectionOpened{Pane: 1, Location: storage.Location{Remote: "ftp", Path: "/"}})
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
					d.key("ctrl+p")
					d.key("detach")
					d.key("enter")
					requireText(t, d, "Finish or cancel the current transfer queue first")
					if strings.Contains(d.screen(), "No source") {
						t.Fatal("running transfer allowed detachment")
					}
					d.key("esc")
					d.key("[") // Live workspaces start with navigation collapsed.
					d.key("shift+tab")
					d.key("j")
					d.key("enter")
					requireText(t, d, "Finish or cancel the current transfer queue first")
					d.key("tab") // Return from sidebar to left, then right.
					d.key("tab")
					d.key("backspace") // Folder navigation still cannot retarget captured jobs.
					if strings.Contains(d.screen(), "RIGHT · REMOTE ftp:/incoming") {
						t.Fatal("parent navigation did not leave the original destination")
					}
					d.run(copyCmd)
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

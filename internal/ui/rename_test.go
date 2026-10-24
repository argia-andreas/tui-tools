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

type renameStorage struct{ memoryStorage }

func (s *renameStorage) Delete(_ context.Context, loc storage.Location) error {
	delete(s.files, loc)
	return nil
}

func (s *renameStorage) Rename(_ context.Context, source, destination storage.Location) error {
	if s.fail {
		return errors.New("connection interrupted; outcome uncertain")
	}
	if _, exists := s.files[destination]; exists {
		return fs.ErrExist
	}
	s.files[destination] = s.files[source]
	delete(s.files, source)
	return nil
}

func renameDriver(t *testing.T, s *renameStorage) *driver {
	t.Helper()
	th, _ := theme.Get("")
	m := ui.New(ui.Options{Browser: s, Transferer: s, Theme: th,
		Left: storage.Location{Path: "/local"}, Right: storage.Location{Remote: "ftp", Path: "/incoming"}})
	d := &driver{model: m}
	d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
	d.run(m.Init())
	return d
}

func TestRenameBothPanesRequiresExplicitConfirmation(t *testing.T) {
	for _, right := range []bool{false, true} {
		t.Run(map[bool]string{false: "left", true: "right"}[right], func(t *testing.T) {
			source := storage.Location{Path: "/local/original.txt"}
			if right {
				source = storage.Location{Remote: "ftp", Path: "/incoming/original.txt"}
			}
			s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "unaltered payload\x00"}}}
			d := renameDriver(t, s)
			if right {
				d.key("tab")
			}
			d.key(" ") // marked selection must follow the rename, not create a batch
			d.deliver(tea.KeyMsg{Type: tea.KeyUp})
			d.key("R")
			requireText(t, d, "Choose a file, not a directory")
			d.deliver(tea.KeyMsg{Type: tea.KeyDown})
			d.key("r")
			requireText(t, d, "No failed or canceled transfers to retry")
			d.key("R")
			requireText(t, d, "Enter review")
			if strings.Contains(d.screen(), "RENAME FILE?") {
				t.Fatal("rename started with a modal instead of inline editing")
			}
			d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
			d.key("räkning qrc.txt") // shortcut letters must be editable text
			d.key("enter")
			requireText(t, d, "RENAME FILE?")
			requireText(t, d, `From: "original.txt"`)
			requireText(t, d, `To:   "räkning qrc.txt"`)
			requireText(t, d, "[Cancel]")
			if _, ok := s.files[source]; !ok || len(s.files) != 1 || d.quit {
				t.Fatal("editing or review changed files or triggered a shortcut")
			}
			for _, size := range [][2]int{{60, 12}, {80, 24}, {160, 40}} {
				d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				requireText(t, d, "[Cancel]")
				for _, line := range strings.Split(d.screen(), "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("rename confirmation overflows width")
					}
				}
			}
			d.key("tab")
			d.key("enter")
			destination := source
			destination.Path = path.Join(path.Dir(source.Path), "räkning qrc.txt")
			if s.files[destination] != "unaltered payload\x00" || len(s.files) != 1 {
				t.Fatalf("rename did not preserve bytes/remove old name: %+v", s.files)
			}
			requireText(t, d, "Renamed")
			// The cursor follows the new file, rather than accidentally selecting '..'.
			d.key("R")
			d.key("enter")
			requireText(t, d, "Name unchanged")
		})
	}
}

func TestRenameCancellationValidationAndRecovery(t *testing.T) {
	source := storage.Location{Path: "/local/original.txt"}
	for _, cancel := range []string{"inline-escape", "confirm-escape", "default-cancel", "outside-click", "close-click", "cancel-click"} {
		t.Run(cancel, func(t *testing.T) {
			s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "original"}}}
			d := renameDriver(t, s)
			d.key("R")
			d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
			d.key("new.txt")
			if cancel != "inline-escape" {
				d.key("enter")
			}
			switch cancel {
			case "default-cancel":
				d.key("enter")
			case "outside-click", "close-click", "cancel-click":
				x, y := 0, 0
				if cancel == "close-click" {
					x, y = 117, 14
				}
				if cancel == "cancel-click" {
					x, y = 43, 24
				}
				d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			default:
				d.key("esc")
			}
			requireText(t, d, "Rename canceled")
			if len(s.files) != 1 || s.files[source] != "original" {
				t.Fatal("cancellation changed storage")
			}
		})
	}
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "original"}}}
	d := renameDriver(t, s)
	d.key("R")
	for _, invalid := range []string{"", "..", "../escape", "folder/name", "bad\\name", "bad\nname"} {
		d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
		d.key(invalid)
		d.key("enter")
		requireText(t, d, "single filename")
		if len(s.files) != 1 {
			t.Fatal("invalid input changed storage")
		}
	}
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("åx.txt")
	d.deliver(tea.KeyMsg{Type: tea.KeyHome})
	d.deliver(tea.KeyMsg{Type: tea.KeyRight})
	d.deliver(tea.KeyMsg{Type: tea.KeyDelete})
	d.key("b")
	d.deliver(tea.KeyMsg{Type: tea.KeyEnd})
	d.deliver(tea.KeyMsg{Type: tea.KeyLeft})
	d.key("Z")
	d.key("backspace")
	d.key("enter")
	requireText(t, d, `To:   "åb.txt"`)
	s.fail = true
	d.key("tab")
	d.key("enter")
	requireText(t, d, "outcome uncertain")
	if s.files[source] != "original" {
		t.Fatal("failed operation changed source")
	}
	s.fail = false
	d.key("R")
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("ok.txt")
	d.key("enter")
	d.key("tab")
	d.key("enter")
	if s.files[storage.Location{Path: "/local/ok.txt"}] != "original" {
		t.Fatal("fresh confirmed retry failed")
	}
}

func TestRenameCollisionAndFilteredSuccess(t *testing.T) {
	source := storage.Location{Path: "/local/original.txt"}
	existing := storage.Location{Path: "/local/existing.txt"}
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "source", existing: "preserve"}}}
	d := renameDriver(t, s)
	// Mark a different file: rename must target the highlighted row, not
	// use the copy operation's marked-file selection rules.
	d.key("/")
	d.key("existing")
	d.key("enter")
	d.key(" ")
	d.key("/")
	d.key("esc")
	d.key("/")
	d.key("original")
	d.key("enter")
	d.key("R")
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("existing.txt")
	d.key("enter")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "Name already exists")
	if s.files[source] != "source" || s.files[existing] != "preserve" {
		t.Fatal("collision changed either file")
	}
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("filtered.txt")
	d.key("enter")
	d.key("tab")
	d.key("enter")
	requireText(t, d, `Renamed "original.txt" → "filtered.txt"`)
	requireText(t, d, "No matching files")
	if s.files[storage.Location{Path: "/local/filtered.txt"}] != "source" {
		t.Fatal("filtered rename failed")
	}
	d = start(t, storage.NewDemo())
	d.key("R")
	requireText(t, d, "demo is read-only")
}

func TestRenameLongNameReviewAndMouseConfirmation(t *testing.T) {
	source := storage.Location{Path: "/local/original.txt"}
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "bytes"}}}
	d := renameDriver(t, s)
	d.deliver(tea.WindowSizeMsg{Width: 60, Height: 12})
	d.key("R")
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	name := strings.Repeat("long", 55) + " END.txt"
	d.key(name)
	requireText(t, d, "END.txt▏")
	d.key("enter")
	// Small screens must let the user read the entire proposed name, not
	// silently truncate away its differentiating suffix.
	for range 12 {
		d.deliver(tea.KeyMsg{Type: tea.KeyDown})
	}
	requireText(t, d, `END.txt"`)
	requireText(t, d, "[Cancel]")
	d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
	// Confirm with the displayed Rename button rather than a private helper.
	x, y := -1, -1
	for row, line := range strings.Split(d.screen(), "\n") {
		if i := strings.Index(line, "[Rename]"); i >= 0 {
			x, y = ansi.StringWidth(line[:i])+2, row
		}
	}
	if x < 0 {
		t.Fatal("confirmation button not visible")
	}
	d.deliver(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	destination := storage.Location{Path: "/local/" + name}
	if len(s.files) != 1 || s.files[destination] != "bytes" {
		t.Fatal("mouse confirmation did not rename the file")
	}
}

func TestStartedRenameCannotBeRetargeted(t *testing.T) {
	source := storage.Location{Path: "/local/original.txt"}
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "bytes"}}}
	d := renameDriver(t, s)
	d.key("R")
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("finished.txt")
	d.key("enter")
	d.key("tab")
	next, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d.model = next // hold the actual async command while input arrives
	for _, key := range []string{"q", "tab", "esc", "R", "D"} {
		d.key(key)
	}
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlC})
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlX})
	d.deliver(tea.MouseMsg{X: 100, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	requireText(t, d, "wait for the result")
	if d.quit || s.files[source] != "bytes" {
		t.Fatal("input quit or changed a pending rename")
	}
	d.run(cmd)
	if len(s.files) != 1 || s.files[storage.Location{Path: "/local/finished.txt"}] != "bytes" {
		t.Fatal("pending rename was retargeted")
	}
}

func TestRenameWaitsForStartedDeletion(t *testing.T) {
	source := storage.Location{Path: "/local/original.txt"}
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "bytes"}}}
	d := renameDriver(t, s)
	d.key("D")
	d.key("tab")
	next, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d.model = next
	d.key("R")
	requireText(t, d, "Wait for the current deletion to finish")
	d.run(cmd)
	if len(s.files) != 0 {
		t.Fatal("confirmed deletion did not finish")
	}
}

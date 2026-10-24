package ui_test

import (
	"context"
	"errors"
	"path"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func (s *memoryStorage) Delete(_ context.Context, loc storage.Location) error {
	if s.fail {
		return errors.New("permission denied")
	}
	delete(s.files, loc)
	return nil
}

func (s *treeStorage) Delete(_ context.Context, loc storage.Location) error {
	delete(s.files, loc)
	return nil
}

func TestConfirmedFileDeletion(t *testing.T) {
	for _, remote := range []string{"", "ftp", "s3", "blob", "other-backend"} {
		for _, scenario := range []string{"cancel", "escape", "outside", "confirm", "mouse", "failure", "palette"} {
			t.Run(remote+"/"+scenario, func(t *testing.T) {
				target := storage.Location{Remote: remote, Path: "/files/space 雪 ; $x.txt"}
				neighbor := storage.Location{Remote: remote, Path: "/files/neighbor.keep"}
				s := &memoryStorage{files: map[storage.Location]string{target: "target", neighbor: "preserve"}, fail: scenario == "failure"}
				th, _ := theme.Get("")
				loc := storage.Location{Remote: remote, Path: "/files"}
				m := ui.New(ui.Options{Browser: s, Transferer: s, Left: loc, Right: loc, Theme: th})
				d := &driver{model: m}
				d.deliver(tea.WindowSizeMsg{Width: 100, Height: 30})
				d.run(m.Init())
				// Filter identifies the highlighted file despite map listing order.
				d.key("/")
				d.key(".keep")
				d.key("enter")
				d.key(" ") // A different marked file must not be deleted.
				d.key("/")
				d.key("esc")
				d.key("/")
				d.key(path.Base(target.Path))
				d.key("enter")
				d.key("D")
				if scenario == "palette" {
					d.key("esc")
					d.key("ctrl+p")
					d.key("Delete selected file")
					d.key("enter")
				}
				requireText(t, d, "CONFIRM FILE DELETION")
				requireText(t, d, "File: "+path.Base(target.Path))
				requireText(t, d, "In:   "+loc.String())
				if len(s.files) != 2 {
					t.Fatal("opening confirmation mutated storage")
				}
				for _, size := range [][2]int{{60, 12}, {100, 30}} {
					d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
					requireText(t, d, "[Cancel]")
					requireText(t, d, "[Delete]")
					for _, line := range strings.Split(d.model.View(), "\n") {
						if ansi.StringWidth(line) > size[0] {
							t.Fatal("confirmation overflowed terminal")
						}
					}
				}
				switch scenario {
				case "cancel":
					d.key("enter")
				case "escape":
					d.key("tab")
					d.key("esc")
				case "outside":
					d.deliver(tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				case "mouse":
					d.deliver(tea.MouseMsg{X: 24, Y: 17, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
				default:
					d.key("tab")
					d.key("enter")
				}
				deleted := scenario == "confirm" || scenario == "mouse" || scenario == "palette"
				_, exists := s.files[target]
				if exists == deleted || s.files[neighbor] != "preserve" {
					t.Fatalf("wrong deletion result: %+v", s.files)
				}
				if deleted {
					requireText(t, d, "Deleted: "+target.String())
					d.key("/")
					d.key("esc")
					if strings.Count(d.screen(), path.Base(target.Path)) != 1 {
						t.Fatal("deleted file remains listed")
					}
				}
				if scenario == "failure" {
					requireText(t, d, "Delete failed: permission denied")
				}
			})
		}
	}
}

func TestDeletionRejectsDemoAndDirectories(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("D")
	requireText(t, d, "Demo is read-only")
	th, _ := theme.Get("")
	s := &treeStorage{files: map[storage.Location]string{{Path: "/files/folder/keep.txt"}: "keep"}}
	m := ui.New(ui.Options{Browser: s, Transferer: s, Left: storage.Location{Path: "/files"}, Theme: th, Disconnected: true})
	d = &driver{model: m}
	d.deliver(tea.WindowSizeMsg{Width: 100, Height: 30})
	d.run(m.Init())
	for _, key := range []string{"D", "k", "D"} {
		d.key(key)
	}
	requireText(t, d, "Choose a file, not a directory, to delete")
}

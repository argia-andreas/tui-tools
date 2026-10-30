package ui_test

import (
	"fmt"
	"path"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func TestRetainedMarksExplainCopyAndExplicitClear(t *testing.T) {
	for _, clear := range []string{"shortcut", "palette"} {
		t.Run(clear, func(t *testing.T) {
			s := &treeStorage{files: map[storage.Location]string{
				{Path: "/local/a.txt"}: "A", {Path: "/local/b.txt"}: "B",
				{Path: "/local/release-notes.txt"}: "release",
			}}
			d := queueDriver(s)
			d.key(" ")
			d.key("j")
			d.key(" ")
			d.key("c")
			requireText(t, d, "Copy: 2 marked; highlight included")
			d.key("tab")
			d.key("enter")
			requireText(t, d, "2 completed")
			requireText(t, d, "2 selected (marked)")
			requireText(t, d, "c Copy 2 marked · Ctrl+G clear marks")
			d.key("j") // Highlight a third file, without changing retained marks.
			d.key("ctrl+p")
			d.key("copy")
			requireText(t, d, "2 marked; highlight excluded")
			d.key("enter")
			requireText(t, d, "Copy: 2 marked; highlight excluded")
			requireText(t, d, "2 captured file jobs")
			d.key("enter") // Cancel remains the default.
			if len(s.files) != 5 {
				t.Fatal("review or default Cancel wrote files")
			}
			// Opposite-pane marks must survive a clear in this pane.
			d.key("ctrl+l")
			d.key("j")
			d.key(" ")
			d.key("ctrl+h")
			// Clearing also removes marks hidden by a filter.
			d.key("/")
			d.key("release-notes")
			d.key("enter")
			if clear == "palette" {
				d.key("ctrl+p")
				d.key("clear marks")
				requireText(t, d, "Clear marks in focused pane")
				d.key("enter")
			} else {
				d.deliver(tea.KeyMsg{Type: tea.KeyCtrlG})
			}
			requireText(t, d, "0 selected (marked)")
			requireText(t, d, "1 selected (marked)")
			requireText(t, d, "Copy uses the highlighted entry")
			d.key("c")
			requireText(t, d, "Copy: highlighted entry")
			requireText(t, d, "From: /local/release-notes.txt")
			d.key("tab")
			d.key("enter")
			if got := s.files[storage.Location{Remote: "ftp", Path: "/incoming/release-notes.txt"}]; got != "release" {
				t.Fatalf("cleared marks did not copy highlight: %q", got)
			}
		})
	}
}

func TestCopyConfirmationKeepsLongPathFilenamesAndCapturedLocations(t *testing.T) {
	for _, width := range []int{60, 80, 160} {
		for _, batch := range []bool{false, true} {
			for _, longName := range []bool{false, true} {
				t.Run(fmt.Sprintf("width%d/batch%t/longName%t", width, batch, longName), func(t *testing.T) {
					name := "release-notes-雪.txt"
					if longName {
						name = "release-" + strings.Repeat("雪", 80) + "x-final.txt"
					}
					left := storage.Location{Path: "/local/" + strings.Repeat("long-source-directory/", 12)}
					right := storage.Location{Remote: "ftp", Path: "/incoming/" + strings.Repeat("long-destination-directory/", 12)}
					left.Path, right.Path = path.Clean(left.Path), path.Clean(right.Path)
					src, dst := left, right
					src.Path, dst.Path = path.Join(left.Path, name), path.Join(right.Path, name)
					s := &treeStorage{files: map[storage.Location]string{src: "exact source bytes"}}
					if batch {
						s.files[storage.Location{Path: path.Join(left.Path, "z.txt")}] = "Z"
					}
					th, _ := theme.Get("")
					m := ui.New(ui.Options{Browser: s, Transferer: s, Left: left, Right: right, Theme: th})
					d := &driver{model: m}
					d.deliver(tea.WindowSizeMsg{Width: width, Height: 30})
					d.run(m.Init())
					if batch {
						d.key(" ")
						d.key("j")
						d.key(" ")
					}
					d.key("c")
					for _, label := range []string{"From:", "To:"} {
						found := false
						for _, row := range strings.Split(d.screen(), "\n") {
							if !strings.Contains(row, label) {
								continue
							}
							found = true
							if longName {
								if !strings.Contains(row, "release-") || !strings.Contains(row, "-final.txt") {
									t.Fatalf("long basename lost its identifying ends: %s", row)
								}
							} else if !strings.Contains(row, name) {
								t.Fatalf("filename hidden by directory prefix: %s", row)
							}
						}
						if !found {
							t.Fatalf("missing %s in confirmation:\n%s", label, d.screen())
						}
					}
					for _, row := range strings.Split(d.screen(), "\n") {
						if ansi.StringWidth(row) != width {
							t.Fatalf("rendered width %d, want %d", ansi.StringWidth(row), width)
						}
					}
					if _, exists := s.files[dst]; exists {
						t.Fatal("confirmation wrote destination")
					}
					d.key("tab")
					d.key("enter")
					if got := s.files[dst]; got != "exact source bytes" {
						t.Fatalf("display elision changed captured location: %q", got)
					}
				})
			}
		}
	}
}

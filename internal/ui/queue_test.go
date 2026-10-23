package ui_test

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

type treeStorage struct {
	files    map[storage.Location]string
	failName string
}

func (s *treeStorage) List(ctx context.Context, loc storage.Location) ([]storage.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items := map[string]storage.Entry{}
	for file, data := range s.files {
		if file.Remote != loc.Remote {
			continue
		}
		rest, ok := strings.CutPrefix(file.Path, path.Clean(loc.Path)+"/")
		if !ok {
			continue
		}
		name, _, dir := strings.Cut(rest, "/")
		items[name] = storage.Entry{Name: name, IsDir: dir, Size: int64(len(data))}
	}
	var result []storage.Entry
	for _, e := range items {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
func (s *treeStorage) Stat(ctx context.Context, loc storage.Location) (storage.Entry, error) {
	if err := ctx.Err(); err != nil {
		return storage.Entry{}, err
	}
	if data, ok := s.files[loc]; ok {
		return storage.Entry{Name: path.Base(loc.Path), Size: int64(len(data))}, nil
	}
	children, _ := s.List(ctx, loc)
	if len(children) > 0 {
		return storage.Entry{Name: path.Base(loc.Path), IsDir: true}, nil
	}
	return storage.Entry{}, fs.ErrNotExist
}
func (s *treeStorage) Copy(ctx context.Context, src, dst storage.Location, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if path.Base(src.Path) == s.failName {
		return errors.New("connection interrupted")
	}
	if _, ok := s.files[dst]; ok && !overwrite {
		return fs.ErrExist
	}
	s.files[dst] = s.files[src]
	return nil
}
func queueDriver(s *treeStorage) *driver {
	th, _ := theme.Get("")
	m := ui.New(ui.Options{Browser: s, Transferer: s, Left: storage.Location{Path: "/local"}, Right: storage.Location{Remote: "ftp", Path: "/incoming"}, Theme: th})
	d := &driver{model: m}
	d.deliver(tea.WindowSizeMsg{Width: 160, Height: 40})
	d.run(m.Init())
	return d
}

func TestFolderQueueConflictFailureAndRetry(t *testing.T) {
	s := &treeStorage{files: map[storage.Location]string{
		{Path: "/local/folder/a.txt"}:                   "source A",
		{Path: "/local/folder/deep/b 雪.txt"}:            "source B unequal",
		{Path: "/local/folder/z.txt"}:                   "source Z",
		{Remote: "ftp", Path: "/incoming/folder/a.txt"}: "PRESERVE",
	}, failName: "z.txt"}
	d := queueDriver(s)
	d.key("c")
	requireText(t, d, "3 captured file jobs")
	if len(s.files) != 4 {
		t.Fatal("planning wrote files")
	}
	d.key("enter") // default Cancel
	if len(s.files) != 4 {
		t.Fatal("Cancel wrote files")
	}
	d.key("r")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "DESTINATION EXISTS")
	d.key("tab")
	d.key("enter") // Skip a, copy nested b, fail z
	requireText(t, d, "1 completed · 1 skipped · 1 failed")
	if s.files[storage.Location{Remote: "ftp", Path: "/incoming/folder/deep/b 雪.txt"}] != "source B unequal" {
		t.Fatal("nested output wrong")
	}
	if s.files[storage.Location{Remote: "ftp", Path: "/incoming/folder/a.txt"}] != "PRESERVE" {
		t.Fatal("skip overwritten")
	}
	// If retry reruns completed/skipped files these changed sources expose it.
	s.files[storage.Location{Path: "/local/folder/deep/b 雪.txt"}] = "CHANGED"
	s.failName = ""
	d.key("r")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "2 completed · 1 skipped · 0 failed")
	if s.files[storage.Location{Remote: "ftp", Path: "/incoming/folder/deep/b 雪.txt"}] != "source B unequal" {
		t.Fatal("retry repeated completed file")
	}
	if s.files[storage.Location{Remote: "ftp", Path: "/incoming/folder/z.txt"}] != "source Z" {
		t.Fatal("retry failed")
	}
}

func TestCancelRunningBatchAndRecheckPartialDestination(t *testing.T) {
	s := &treeStorage{files: map[storage.Location]string{{Path: "/local/a"}: "aaa", {Path: "/local/b"}: "BBBBB"}}
	d := queueDriver(s)
	d.key(" ")
	d.key("j")
	d.key(" ")
	d.key("c")
	d.key("tab")
	// Hold the Copy command so cancellation happens before backend completion.
	next, statCmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d.model = next
	next, copyCmd := d.model.Update(statCmd())
	d.model = next
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlX})
	d.run(copyCmd)
	requireText(t, d, "2 canceled")
	if len(s.files) != 2 {
		t.Fatal("canceled copy wrote files")
	}
	// A real interrupted backend may leave partial output. Retry must pause.
	s.files[storage.Location{Remote: "ftp", Path: "/incoming/a"}] = "PARTIAL"
	d.key("r")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "DESTINATION EXISTS")
	if s.files[storage.Location{Remote: "ftp", Path: "/incoming/a"}] != "PARTIAL" {
		t.Fatal("retry silently overwrote partial output")
	}
	d.key("tab")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "2 completed")
	if s.files[storage.Location{Remote: "ftp", Path: "/incoming/b"}] != "BBBBB" {
		t.Fatal("unfinished copy not retried")
	}
}

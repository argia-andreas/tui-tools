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
)

type previewBrowser struct {
	read func(context.Context, storage.Location, int64) ([]byte, error)
}

func (b previewBrowser) List(context.Context, storage.Location) ([]storage.Entry, error) {
	return []storage.Entry{{Name: "first.txt"}, {Name: "second.txt"}}, nil
}
func (b previewBrowser) Read(ctx context.Context, loc storage.Location, n int64) ([]byte, error) {
	return b.read(ctx, loc, n)
}

func TestPreviewExplicitRequestAndStaleResponse(t *testing.T) {
	reads := 0
	b := previewBrowser{read: func(ctx context.Context, loc storage.Location, n int64) ([]byte, error) {
		reads++
		if n > 64*1024+1 {
			t.Fatalf("unbounded read: %d", n)
		}
		return []byte(loc.Path), nil
	}}
	d := start(t, b)
	_ = d.screen()
	if reads != 0 {
		t.Fatal("browsing read content without permission")
	}
	next, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	d.model = next
	requireText(t, d, "Loading…")
	old := cmd()
	d.key("esc")
	d.key("j")
	d.key("p")
	requireText(t, d, "/project/second.txt")
	d.deliver(old)
	requireText(t, d, "/project/second.txt")
	if strings.Contains(d.screen(), "/project/first.txt") {
		t.Fatal("stale response replaced latest preview")
	}
	d.key("esc")
	d.deliver(old)
	if strings.Contains(d.screen(), "CONTENT PREVIEW") {
		t.Fatal("late response reopened preview")
	}
}

func TestPreviewCancelStopsRead(t *testing.T) {
	started, finished := make(chan context.Context, 1), make(chan tea.Msg, 1)
	d := start(t, previewBrowser{read: func(ctx context.Context, _ storage.Location, _ int64) ([]byte, error) {
		started <- ctx
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	next, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	d.model = next
	go func() { finished <- cmd() }()
	ctx := <-started
	d.key("esc")
	if ctx.Err() != context.Canceled {
		t.Fatal("closing preview did not cancel backend")
	}
	d.deliver(<-finished)
	if strings.Contains(d.screen(), "CONTENT PREVIEW") {
		t.Fatal("canceled response reopened preview")
	}
}

func TestPreviewContentStatesAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name, input, notice string
		err                 error
		truncated           bool
	}{
		{name: "empty", notice: "Empty file"},
		{name: "text", input: "hello\tworld\nsecond", notice: "hello world"},
		{name: "nul", input: "hi\x00world", notice: "Binary or invalid UTF-8"},
		{name: "invalid", input: "\xff", notice: "Binary or invalid UTF-8"},
		{name: "error", err: errors.New("denied\x1b[2J\nretry"), notice: "Preview failed: denied retry"},
		{name: "exact limit", input: strings.Repeat("x", 64*1024), notice: "Text preview"},
		{name: "truncated", input: strings.Repeat("x", 64*1024+1), notice: "Truncated at 64 KiB", truncated: true},
		{name: "split rune", input: strings.Repeat("x", 64*1024-1) + "界", notice: "Truncated at 64 KiB", truncated: true},
		{name: "escape", input: "hello\x1b[2J\x1b]52;c;c2VjcmV0\a\x1bPdanger\x1b\\\r\b\tworld\nnext\u202e", notice: "hello world"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := start(t, previewBrowser{read: func(context.Context, storage.Location, int64) ([]byte, error) { return []byte(tc.input), tc.err }})
			d.key("p")
			requireText(t, d, tc.notice)
			if strings.Contains(d.screen(), "Truncated at") != tc.truncated {
				t.Fatal("wrong truncation notice")
			}
			if strings.Contains(d.screen(), "c2VjcmV0") || strings.Contains(d.screen(), "danger") || strings.Contains(d.model.View(), "\x1b[2J") || strings.Contains(d.screen(), "\u202e") {
				t.Fatal("unsafe preview output")
			}
			for _, size := range [][2]int{{160, 40}, {80, 24}, {60, 12}} {
				d.deliver(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				lines := strings.Split(d.model.View(), "\n")
				if len(lines) > size[1] {
					t.Fatal("preview exceeds terminal height")
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("preview exceeds terminal width")
					}
				}
			}
		})
	}
}

func TestPreviewScrolling(t *testing.T) {
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf("LINE_%03d", i))
	}
	d := start(t, previewBrowser{read: func(context.Context, storage.Location, int64) ([]byte, error) {
		return []byte(strings.Join(lines, "\n")), nil
	}})
	d.key("p")
	requireText(t, d, "LINE_000")
	d.deliver(tea.KeyMsg{Type: tea.KeyEnd})
	requireText(t, d, "LINE_039")
	if strings.Contains(d.screen(), "LINE_000") {
		t.Fatal("End did not scroll")
	}
	d.deliver(tea.KeyMsg{Type: tea.KeyHome})
	requireText(t, d, "LINE_000")
}

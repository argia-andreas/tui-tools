package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/connections"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func updateWorkspace(m workspace, msg tea.Msg) (workspace, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(workspace), cmd
}

func runWorkspace(m workspace, cmd tea.Cmd) workspace {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			m = runWorkspace(m, child)
		}
		return m
	}
	m, cmd = updateWorkspace(m, msg)
	return runWorkspace(m, cmd)
}

func TestDefaultStartupListsRealLocalFilesWithoutRcloneOrBookmarks(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	theme, _ := theme.Get("")
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "real-local.txt"), []byte("real bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := workspaceOptions(context.Background(), false, local, "", []string{"s3:/bucket/prefix"}, "missing-config", theme, false)
	if err != nil || !o.Disconnected || o.Right != (storage.Location{}) || o.Left.Path != local || o.Transferer == nil {
		t.Fatalf("startup options %+v: %v", o, err)
	}
	if len(o.Locations) != 2 || o.Locations[0] != (storage.Location{Remote: "s3", Path: "/bucket/prefix"}) {
		t.Fatalf("extra roots lost: %+v", o.Locations)
	}
	// A malformed bookmarks file must not be read by construction or Init.
	bookmarks := filepath.Join(t.TempDir(), "connections.json")
	if err := os.WriteFile(bookmarks, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	m := workspace{model: ui.New(o), ctx: context.Background(), theme: theme, bookmarkPath: bookmarks}
	m, _ = updateWorkspace(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = runWorkspace(m, m.Init())
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "real-local.txt") || strings.Contains(view, "Open a connection") || strings.Contains(view, "Demo workspace") {
		t.Fatalf("default did not retain local workspace:\n%s", view)
	}
	// Failure is delayed until the explicit connection request, and is actionable.
	m, cmd := updateWorkspace(m, ui.ConnectRequested{Local: local})
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "Install rclone") {
		t.Fatalf("missing dependency guidance: %s", m.View())
	}
	m, cmd = updateWorkspace(m, tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || !strings.Contains(m.View(), "real-local.txt") {
		t.Fatal("cancel lost local workspace or quit")
	}
}

func TestStartupAbsoluteCwdExplicitRightAndReadOnlyDemo(t *testing.T) {
	theme, _ := theme.Get("")
	o, err := workspaceOptions(context.Background(), false, ".", "ftp:/incoming", []string{"blob:/container"}, "", theme, false)
	if err != nil || o.Disconnected || !filepath.IsAbs(o.Left.Path) || o.Right != (storage.Location{Remote: "ftp", Path: "/incoming"}) {
		t.Fatalf("explicit right %+v: %v", o, err)
	}
	if len(o.Locations) != 3 {
		t.Fatalf("locations %+v", o.Locations)
	}
	demo, err := workspaceOptions(context.Background(), true, ".", "", nil, "", theme, false)
	if err != nil || demo.Transferer != nil || demo.Disconnected || demo.Right.Remote != "demo-ftp" {
		t.Fatalf("demo options %+v: %v", demo, err)
	}
}

// This public tea.Model stand-in renders every delivered message. Assertions
// exercise wrapper routing through Update/View, not the wrapper's private state.
type workspaceProbe struct{ events string }

func (m workspaceProbe) Init() tea.Cmd { return nil }
func (m workspaceProbe) View() string  { return m.events }
func (m workspaceProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.events += fmt.Sprintf("size=%dx%d;", msg.Width, msg.Height)
	case ui.ConnectionOpened:
		m.events += fmt.Sprintf("connected=%s|%s|%v;", msg.Left, msg.Right, msg.Locations)
		m.events += fmt.Sprintf("backend=%t/%t;", msg.Browser != nil, msg.Transferer != nil)
		return m, func() tea.Msg { return "connection-result" }
	case tea.KeyMsg:
		m.events += "key=" + msg.String() + ";"
	case tea.MouseMsg:
		m.events += "mouse;"
	case string:
		m.events += msg + ";"
	}
	return m, nil
}

func fakeRclone(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "rclone"), []byte("#!/bin/sh\nprintf 'ftp:\\nblob:\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestWorkspacePickerCancelRetainsAsyncResultsAndResize(t *testing.T) {
	fakeRclone(t)
	theme, _ := theme.Get("")
	m := workspace{model: workspaceProbe{}, ctx: context.Background(), theme: theme, bookmarkPath: filepath.Join(t.TempDir(), "bookmarks")}
	m, _ = updateWorkspace(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, discovery := updateWorkspace(m, ui.ConnectRequested{Local: t.TempDir()})
	if discovery == nil || !strings.Contains(m.View(), "Loading connections") {
		t.Fatal("request did not immediately show async loading picker")
	}
	m, _ = updateWorkspace(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m, _ = updateWorkspace(m, tea.MouseMsg{X: 5, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m, _ = updateWorkspace(m, "background-result")
	m, _ = updateWorkspace(m, tea.WindowSizeMsg{Width: 100, Height: 28})
	m = runWorkspace(m, discovery)
	for _, row := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(row) != 100 {
			t.Fatalf("picker missed resize: width %d", ansi.StringWidth(row))
		}
	}
	m, cmd := updateWorkspace(m, tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || m.View() != "size=80x24;background-result;size=100x28;" {
		t.Fatalf("cancel routing/retention failed: %q", m.View())
	}
}

func TestWorkspaceConnectRestoresBookmarkAndPreservesExtraRoots(t *testing.T) {
	fakeRclone(t)
	theme, _ := theme.Get("")
	local := t.TempDir()
	path := filepath.Join(t.TempDir(), "bookmarks")
	if err := connections.Save(path, connections.Bookmark{Name: "Saved", Remote: "ftp", Root: "/incoming", Local: local}); err != nil {
		t.Fatal(err)
	}
	for _, override := range []string{"", t.TempDir()} {
		m := workspace{model: workspaceProbe{}, ctx: context.Background(), theme: theme, bookmarkPath: path, leftOverride: override,
			extra: []storage.Location{{Remote: "s3", Path: "/bucket"}, {Path: "/old-local"}}}
		m, cmd := updateWorkspace(m, ui.ConnectRequested{Local: "/different-default"})
		m = runWorkspace(m, cmd)
		if !strings.Contains(m.View(), "Saved") {
			t.Fatal("bookmark not displayed")
		}
		m, cmd = updateWorkspace(m, tea.MouseMsg{X: 5, Y: 14, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		if cmd == nil {
			t.Fatal("workspace connection command discarded")
		}
		result := cmd()
		if _, quit := result.(tea.QuitMsg); quit {
			t.Fatal("picker success quit program")
		}
		m, cmd = updateWorkspace(m, result)
		m = runWorkspace(m, cmd)
		wantLocal := local
		if override != "" {
			wantLocal = override
		}
		if !strings.Contains(m.View(), "connected="+wantLocal+"|ftp:/incoming|") || !strings.Contains(m.View(), "s3:/bucket") || !strings.Contains(m.View(), "connection-result") || strings.Contains(m.View(), "/old-local") {
			t.Fatalf("wrong connection payload/result: %s", m.View())
		}
	}
}

func TestWorkspaceCanceledDiscoveryCannotReopenPickerAndCtrlCQuits(t *testing.T) {
	fakeRclone(t)
	theme, _ := theme.Get("")
	m := workspace{model: workspaceProbe{events: "workspace"}, ctx: context.Background(), theme: theme, bookmarkPath: filepath.Join(t.TempDir(), "bookmarks")}
	m, discovery := updateWorkspace(m, ui.ConnectRequested{Local: t.TempDir()})
	m, _ = updateWorkspace(m, tea.KeyMsg{Type: tea.KeyEsc})
	result := discovery().(connectionsLoaded)
	if !strings.Contains(result.picker.message, "could not discover") {
		t.Fatalf("discovery did not respect cancellation: %s", result.picker.message)
	}
	m, _ = updateWorkspace(m, result)
	if m.View() != "workspace" {
		t.Fatal("stale discovery reopened picker")
	}
	m, _ = updateWorkspace(m, ui.ConnectRequested{Local: t.TempDir()})
	_, cmd := updateWorkspace(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl+C returned non-quit command")
	}
}

func TestDiscoveryIsDeferredAndUsesExplicitRcloneConfig(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "called")
	// Only the exact config-aware discovery command succeeds. No real remotes
	// or authentication are involved in this process-boundary test.
	script := fmt.Sprintf("#!/bin/sh\nprintf called > '%s'\n[ \"$1\" = listremotes ] && [ \"$2\" = --config ] && [ \"$3\" = 'chosen config' ] || exit 1\nprintf 'configured:\\n'\n", marker)
	if err := os.WriteFile(filepath.Join(bin, "rclone"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	theme, _ := theme.Get("")
	m := workspace{model: workspaceProbe{}, ctx: context.Background(), theme: theme, config: "chosen config", bookmarkPath: filepath.Join(t.TempDir(), "bookmarks")}
	m = runWorkspace(m, m.Init())
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("startup invoked rclone")
	}
	m, cmd := updateWorkspace(m, ui.ConnectRequested{Local: t.TempDir()})
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("Update blocked on discovery rather than returning a command")
	}
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "configured:") {
		t.Fatalf("config was not passed to async discovery: %s", m.View())
	}
}

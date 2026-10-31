package ui_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"streams/internal/fixture"
	"streams/internal/stream"
	"streams/internal/ui"
)

func send(m ui.Model, msg tea.Msg) (ui.Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(ui.Model), cmd
}
func key(m ui.Model, s string) (ui.Model, tea.Cmd) {
	if len(s) > 1 && strings.Contains(s, " ") {
		var cmd tea.Cmd
		for _, r := range s {
			m, cmd = key(m, string(r))
		}
		return m, cmd
	}
	t := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	switch s {
	case " ":
		t = tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		t = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		t = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+u":
		t = tea.KeyMsg{Type: tea.KeyCtrlU}
	case "f1":
		t = tea.KeyMsg{Type: tea.KeyF1}
	case "tab":
		t = tea.KeyMsg{Type: tea.KeyTab}
	}
	return send(m, t)
}
func boot(t *testing.T, b stream.Backend) ui.Model {
	t.Helper()
	m := ui.New(ui.Options{Backend: b, Fixture: true})
	m, _ = send(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = send(m, m.Init()())
	return m
}
func text(m ui.Model) string { return ansi.Strip(m.View()) }
func TestVisibleInvestigation(t *testing.T) {
	m := boot(t, fixture.New())
	defer m.Close()
	if !strings.Contains(text(m), "orders.created") {
		t.Fatal(text(m))
	}
	// Clicking a topic selects without opening; release cannot run it.
	m, cmd := send(m, tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if cmd != nil || strings.Contains(text(m), "EVENTS /") {
		t.Fatal("row click opened")
	}
	m, cmd = key(m, "enter")
	if cmd == nil {
		t.Fatal("Open missing")
	}
	m, _ = send(m, cmd())
	if !strings.Contains(text(m), "18 loaded") {
		t.Fatal(text(m))
	}
	m, _ = key(m, "enter")
	if !strings.Contains(text(m), "PAYLOAD / orders.created") {
		t.Fatal(text(m))
	}
	m, _ = key(m, "v")
	if !strings.Contains(text(m), "raw · v") {
		t.Fatal("reader mode did not change")
	}
	m, _ = key(m, "esc")
	m, _ = key(m, "s")
	m, _ = key(m, "ctrl+u")
	m, _ = key(m, "offset 2 303")
	m, _ = key(m, "f1")
	if !strings.Contains(text(m), "HELP") {
		t.Fatal("help missing")
	}
	m, _ = key(m, "esc")
	if !strings.Contains(text(m), "offset 2 303") {
		t.Fatal("Help lost seek draft")
	}
	m, cmd = key(m, "enter")
	if cmd == nil {
		t.Fatal("seek missing")
	}
	m, _ = send(m, cmd())
	if !strings.Contains(text(m), "p2:306") {
		t.Fatal("seek did not keep explicit partition cursor", text(m))
	}
	m, _ = key(m, "s")
	m, _ = key(m, "ctrl+u")
	m, _ = key(m, "offset 2 -1")
	m, cmd = key(m, "enter")
	if cmd != nil || !strings.Contains(text(m), "Offset must be a nonnegative integer") {
		t.Fatal("invalid offset ran")
	}
	m, _ = key(m, "esc")
	m, _ = key(m, "g") // list loaded separately
	m, cmd = key(m, "r")
	m, _ = send(m, cmd())
	m, cmd = key(m, "enter")
	m, _ = send(m, cmd())
	if !strings.Contains(text(m), "lag 84 (partial: false)") {
		t.Fatal(text(m))
	}
	// Resize freezes mutation and retains a draft.
	m, _ = key(m, "esc")
	m, _ = key(m, "t")
	m, cmd = key(m, "r")
	m, _ = send(m, cmd())
	m, _ = key(m, "/")
	m, _ = key(m, "orders")
	m, _ = send(m, tea.WindowSizeMsg{Width: 59, Height: 15})
	m, cmd = key(m, "enter")
	if cmd != nil {
		t.Fatal("hidden draft applied")
	}
	m, _ = send(m, tea.WindowSizeMsg{Width: 60, Height: 16})
	if !strings.Contains(text(m), "orders") {
		t.Fatal("resize lost draft")
	}
}

func TestDensitySelectionAndPayloadModal(t *testing.T) {
	m := boot(t, fixture.New())
	defer m.Close()
	m, cmd := key(m, "enter")
	m, _ = send(m, cmd())
	m, _ = key(m, "j")
	if !strings.Contains(text(m), "p0 / offset 101") || !strings.Contains(text(m), "Compact") {
		t.Fatal("keyboard selection did not update inspector", text(m))
	}
	m, _ = send(m, tea.MouseMsg{X: 40, Y: 12, Action: tea.MouseActionMotion})
	if !strings.Contains(text(m), "p0 / offset 101") {
		t.Fatal("mouse motion changed selection")
	}
	m, _ = key(m, "enter")
	if !strings.Contains(text(m), "PAYLOAD / orders.created · p0 / offset 101") {
		t.Fatal("payload modal missing selected record", text(m))
	}
	m, _ = key(m, "g")
	m, _ = key(m, "j")
	if !strings.Contains(text(m), "PAYLOAD / orders.created · p0 / offset 101") {
		t.Fatal("modal input changed workspace selection")
	}
	m, _ = key(m, "tab") // key
	m, _ = key(m, "tab") // headers
	if !strings.Contains(text(m), "[1] source") || !strings.Contains(text(m), "[2] source") {
		t.Fatal("ordered duplicate headers missing", text(m))
	}
	m, _ = key(m, "esc")
	if strings.Contains(text(m), "PAYLOAD /") || !strings.Contains(text(m), "p0 / offset 101") {
		t.Fatal("modal close lost selection", text(m))
	}
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = key(m, "Comfortable")
	m, _ = key(m, "enter")
	if !strings.Contains(text(m), "Comfortable") || !strings.Contains(text(m), "p0 / offset 101") {
		t.Fatal("density change lost selection", text(m))
	}
	if strings.Contains(text(m), "TOPICS / 3") || !strings.Contains(text(m), "[ • orders.created ]") || !strings.Contains(text(m), "┃ P0  101") {
		t.Fatal("Comfortable must use bordered cards and a topic switcher, not a taller table", text(m))
	}
	// The payload line of the fifth Comfortable card selects without opening.
	m, cmd = send(m, tea.MouseMsg{X: 50, Y: 23, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if cmd != nil || !strings.Contains(text(m), "p0 / offset 104") {
		t.Fatal("Comfortable card hit target incorrect", text(m))
	}
	m, _ = key(m, "enter")
	m, _ = send(m, tea.WindowSizeMsg{Width: 60, Height: 16})
	if !strings.Contains(text(m), "PAYLOAD /") || !strings.Contains(text(m), "Close") {
		t.Fatal("compact payload modal inaccessible", text(m))
	}
	m, _ = key(m, "esc")
	m, _ = send(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	if !strings.Contains(text(m), "p0 / offset 104") {
		t.Fatal("resize/close lost selection", text(m))
	}
	m, cmd = send(m, tea.MouseMsg{X: 30, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if cmd == nil {
		t.Fatal("topic switcher did not open topic", text(m))
	}
	m, _ = send(m, cmd())
	if !strings.Contains(text(m), "EVENTS / orders.failed") {
		t.Fatal("topic switcher opened wrong topic", text(m))
	}
}

type changingCatalog struct {
	stream.Backend
	name string
}

func (b *changingCatalog) Topics(context.Context) ([]stream.Topic, error) {
	return []stream.Topic{{Ref: stream.TopicRef{Name: b.name}}}, nil
}

func TestLatestCatalogWins(t *testing.T) {
	b := &changingCatalog{Backend: fixture.New(), name: "initial"}
	m := boot(t, b)
	defer m.Close()
	m, old := key(m, "r")
	b.name = "outdated"
	oldResult := old()
	m, newer := key(m, "r")
	b.name = "current"
	m, _ = send(m, newer())
	m, _ = send(m, oldResult)
	if !strings.Contains(text(m), "current") || strings.Contains(text(m), "outdated") {
		t.Fatal("older catalog replaced newer observation", text(m))
	}
}

func TestConnectionSwitchBlocksOldClusterOperations(t *testing.T) {
	m := ui.New(ui.Options{Backend: fixture.New(), Connection: "first", Connections: []string{"first", "second"}, OpenConnection: func(context.Context, string) (stream.Backend, error) { return fixture.New(), nil }})
	defer m.Close()
	m, _ = send(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = send(m, m.Init()())
	// Use the public action palette, leaving the asynchronous switch held.
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = key(m, "Switch connection")
	m, _ = key(m, "enter")
	m, _ = key(m, "ctrl+u")
	m, _ = key(m, "second")
	m, connect := key(m, "enter")
	if connect == nil {
		t.Fatal("switch command missing", text(m))
	}
	for _, action := range []string{"enter", "r", "g", "s", "F"} {
		var cmd tea.Cmd
		m, cmd = key(m, action)
		if cmd != nil || !strings.Contains(text(m), "Connection switch in progress") {
			t.Fatal("operation escaped pending connection switch", action, text(m))
		}
	}
	m, catalog := send(m, connect())
	if catalog == nil {
		t.Fatal("new connection catalog missing")
	}
	m, _ = send(m, catalog())
	if !strings.Contains(text(m), "second") || !strings.Contains(text(m), "orders.created") {
		t.Fatal("new connection not installed", text(m))
	}
}

type heldBackend struct {
	stream.Backend
	started, release, closed chan struct{}
}

func (b *heldBackend) Open(ctx context.Context, q stream.ReadRequest) (stream.Reader, error) {
	close(b.started)
	<-b.release
	r, err := b.Backend.Open(ctx, q)
	return &observedReader{Reader: r, closed: b.closed}, err
}

type observedReader struct {
	stream.Reader
	once   sync.Once
	closed chan struct{}
}

func (r *observedReader) Close() { r.once.Do(func() { r.Reader.Close(); close(r.closed) }) }
func TestPauseRejectsLateOpen(t *testing.T) {
	b := &heldBackend{Backend: fixture.New(), started: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
	m := boot(t, b)
	defer m.Close()
	m, cmd := key(m, "enter")
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	<-b.started
	m, _ = key(m, " ")
	close(b.release)
	old := <-result
	m, _ = send(m, old)
	<-b.closed
	if strings.Contains(text(m), "18 loaded") || !strings.Contains(text(m), "PAUSED") {
		t.Fatal("late Open changed paused view", text(m))
	}
}

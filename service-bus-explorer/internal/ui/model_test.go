package ui_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
	"servicebusexplorer/internal/ui"
)

func workspace(t *testing.T) tea.Model {
	t.Helper()
	s, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	return start(t, s, config.Default(), "fixture", false)
}

func start(t *testing.T, store explorer.Store, prefs config.Preferences, mode string, ascii bool) tea.Model {
	t.Helper()
	m := ui.New(store, ui.Options{Mode: mode, Preferences: prefs, ASCII: ascii, ConfigPath: filepath.Join(t.TempDir(), "preferences.json")})
	ready, counts := m.Update(m.Init()())
	ready, _ = ready.Update(counts())
	t.Cleanup(func() { _, _ = ready.Update(tea.KeyMsg{Type: tea.KeyCtrlC}) })
	return ready
}

func key(m tea.Model, text string) (tea.Model, tea.Cmd) {
	typeOf := tea.KeyRunes
	var runes []rune
	switch text {
	case "enter":
		typeOf = tea.KeyEnter
	case "esc":
		typeOf = tea.KeyEsc
	case "tab":
		typeOf = tea.KeyTab
	case "shift+tab":
		typeOf = tea.KeyShiftTab
	case "ctrl+h":
		typeOf = tea.KeyCtrlH
	case "ctrl+l":
		typeOf = tea.KeyCtrlL
	case "ctrl+p":
		typeOf = tea.KeyCtrlP
	case "ctrl+u":
		typeOf = tea.KeyCtrlU
	case "down":
		typeOf = tea.KeyDown
	case "up":
		typeOf = tea.KeyUp
	case "pgdown":
		typeOf = tea.KeyPgDown
	default:
		runes = []rune(text)
	}
	return m.Update(tea.KeyMsg{Type: typeOf, Runes: runes})
}

func press(m tea.Model, text string) tea.Model {
	m, cmd := key(m, text)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	return m
}

func assertFrame(t *testing.T, m tea.Model, width, height int) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != height {
		t.Fatalf("height %d != %d", len(lines), height)
	}
	for n, line := range lines {
		if ansi.StringWidth(line) != width {
			t.Fatalf("row %d width %d != %d: %q", n, ansi.StringWidth(line), width, ansi.Strip(line))
		}
	}
}

func TestFixtureWorkspaceNavigationAndResponsiveFrames(t *testing.T) {
	m := workspace(t)
	view := ansi.Strip(m.View())
	for _, want := range []string{"WORKSPACE", "FIXTURE · OFFLINE", "orders", "payments", "notifications", "inventory", "shipping", "invoices", "audit-log", "email-outbox", "webhook-dispatch", "reports", "customer-sync", "empty", "128 queued", "12 dead-letter"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
	assertFrame(t, m, 160, 48)
	m = press(m, "t")
	if !strings.Contains(ansi.Strip(m.View()), "events/billing") {
		t.Fatal("table lost subscriptions beyond the first 12 cards")
	}
	m = press(m, "g")
	m = press(m, "]")
	view = ansi.Strip(m.View())
	for _, want := range []string{"orders / DEAD-LETTER", "orders-dlq-0001", "MaxDeliveryCountExceeded", "BODY / read-only", `"currency": "EUR"`} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
	assertFrame(t, m, 160, 48)
	m = press(m, "[")
	if !strings.Contains(ansi.Strip(m.View()), "orders-active-0001") {
		t.Fatal("Active switch failed")
	}
	m = press(m, "n")
	if !strings.Contains(ansi.Strip(m.View()), "orders-active-0021") {
		t.Fatal("next page failed")
	}
	m = press(m, "r")
	if !strings.Contains(ansi.Strip(m.View()), "orders-active-0021") {
		t.Fatal("refresh advanced cursor")
	}
	m = press(m, "p")
	if !strings.Contains(ansi.Strip(m.View()), "orders-active-0001") {
		t.Fatal("previous page failed")
	}
	m = press(m, "esc")
	m = press(m, "tab")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assertFrame(t, m, 80, 24)
	if !strings.Contains(ansi.Strip(m.View()), "q quit") {
		t.Fatal("compact layout hid the exit shortcut")
	}
	m = press(m, "enter")
	if !strings.Contains(ansi.Strip(m.View()), "orders / ACTIVE") {
		t.Fatal("hidden sidebar retained focus after resize")
	}
	assertFrame(t, m, 80, 24)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	assertFrame(t, m, 160, 48)
}

func TestLateActiveReplyCannotReplaceDLQ(t *testing.T) {
	m := workspace(t)
	m, active := key(m, "enter")
	activeReply := active()
	m, dlq := key(m, "]")
	m, _ = m.Update(dlq())
	m, _ = m.Update(activeReply)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "orders-dlq-0001") || strings.Contains(view, "orders-active-0001") {
		t.Fatalf("late reply replaced DLQ selection\n%s", view)
	}
}

type unavailableCounts struct{ explorer.Store }

func (unavailableCounts) Counts(context.Context, explorer.Resource) (explorer.Counts, error) {
	return explorer.Counts{State: "unavailable", Detail: "emulator limitation"}, nil
}

func TestEmulatorLabelAndUnavailableCounts(t *testing.T) {
	store, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	ready := start(t, unavailableCounts{store}, config.Default(), "emulator", false)
	view := ansi.Strip(ready.View())
	for _, want := range []string{"EMULATOR · LOCAL", "runtime counts unavailable", "— queued", "— dead-letter"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "FIXTURE") || strings.Contains(view, "0 queued") || strings.Contains(view, "0 dead-letter") {
		t.Fatal("emulator was mislabeled or unknown counts rendered as zero")
	}
	assertFrame(t, ready, 160, 48)
}

func choose(m tea.Model, query string) tea.Model {
	m = press(m, "ctrl+p")
	m = press(m, query)
	return press(m, "enter")
}

func TestPaletteOwnsPrintableKeysAndRestoresFocus(t *testing.T) {
	m := workspace(t)
	m = press(m, "ctrl+p")
	for _, text := range []string{"q", "j", "k"} {
		var cmd tea.Cmd
		m, cmd = key(m, text)
		if cmd != nil {
			t.Fatal("printable query key escaped palette")
		}
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "> qjk_") || !strings.Contains(view, "No matching actions") {
		t.Fatalf("query was not owned by palette\n%s", view)
	}
	m = press(press(m, "ctrl+h"), "ctrl+l")
	if !strings.Contains(m.View(), "> qj_") {
		t.Fatal("directional keys escaped palette text input")
	}
	m = press(m, "enter")
	m = press(m, "esc")
	m = press(m, "enter")
	if !strings.Contains(ansi.Strip(m.View()), "orders / ACTIVE") {
		t.Fatal("palette moved underlying selection")
	}
	m = press(m, "tab") // inspector focus
	m = press(m, "ctrl+p")
	m = press(m, "esc")
	m = press(m, "j")
	if !strings.Contains(ansi.Strip(m.View()), "› 100") {
		t.Fatal("closing palette lost inspector focus")
	}
	m = choose(m, "clthgrv") // deliberately non-contiguous fuzzy match
	m = choose(m, "settings")
	if !strings.Contains(ansi.Strip(m.View()), "Color theme: gruvbox") {
		t.Fatal("fuzzy theme action failed")
	}
}

func TestDirectionalFocusAcrossVisibleRegions(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		width                        int
		sidebar, inspector, messages bool
	}{
		{"all panes", 160, true, true, true},
		{"sidebar disabled", 160, false, true, true},
		{"sidebar auto hidden", 100, true, true, true},
		{"inspector disabled", 160, true, false, true},
		{"inspector auto hidden", 110, true, true, true},
		{"only content", 80, true, true, true},
		{"dashboard", 160, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := fixture.New()
			prefs := config.Default()
			prefs.Sidebar, prefs.Inspector = tc.sidebar, tc.inspector
			base := start(t, s, prefs, "fixture", false)
			base, _ = base.Update(tea.WindowSizeMsg{Width: tc.width, Height: 48})
			if tc.messages {
				base = press(base, "enter")
			}
			// Tab establishes the existing rendered focus states without inspecting internals.
			left, right := base, base
			if tc.sidebar && tc.width >= 110 {
				left = press(base, "shift+tab")
			}
			contentWidth := tc.width
			if tc.sidebar && tc.width >= 110 {
				contentWidth -= 24
			}
			if tc.messages && tc.inspector && contentWidth >= 95 {
				right = press(base, "tab")
			}
			m := press(press(base, "ctrl+h"), "ctrl+h")
			if press(m, "j").View() != press(left, "j").View() {
				t.Fatal("left navigation did not clamp at the first visible region")
			}
			if tc.sidebar && tc.width >= 110 {
				m = press(m, "ctrl+l")
				if press(m, "j").View() != press(base, "j").View() {
					t.Fatal("right skipped content")
				}
			}
			m = press(press(m, "ctrl+l"), "ctrl+l")
			if press(m, "j").View() != press(right, "j").View() {
				t.Fatal("right navigation did not clamp at the last visible region")
			}
			if tc.messages && tc.inspector && contentWidth >= 95 {
				m = press(m, "ctrl+h")
				if press(m, "j").View() != press(base, "j").View() {
					t.Fatal("left skipped content")
				}
			}
			if press(press(press(base, "tab"), "shift+tab"), "j").View() != press(base, "j").View() {
				t.Fatal("Tab reverse cycle changed")
			}
		})
	}
}

func TestSaveSnapshotAndReloadWatchlist(t *testing.T) {
	store, _ := fixture.New()
	path := filepath.Join(t.TempDir(), "preferences.json")
	initial := ui.New(store, ui.Options{Mode: "fixture", Preferences: config.Default(), ConfigPath: path})
	m, counts := initial.Update(initial.Init()())
	m, _ = m.Update(counts())
	m = choose(m, "settings")
	m = press(m, "enter") // gruvbox
	for range 5 {
		m = press(m, "j")
	}
	m = press(m, "enter") // remove orders, keep every other full identity
	m, save := key(m, "s")
	m = press(m, "k") // save row
	for range 4 {
		m = press(m, "k")
	}
	m = press(m, "enter") // newer, not-yet-saved light theme
	m, _ = m.Update(save())
	if !strings.Contains(ansi.Strip(m.View()), "unsaved changes") {
		t.Fatal("old save completion claimed new edits were saved")
	}
	p, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Theme != "gruvbox" {
		t.Fatal("save did not own a snapshot")
	}
	resources, _ := store.Discover(context.Background())
	if len(p.Watchlists[resources[0].Profile]) != 13 {
		t.Fatal("watchlist not persisted")
	}
	reloaded := start(t, store, p, "fixture", false)
	view := ansi.Strip(reloaded.View())
	if !strings.Contains(view, "13 watched entities") || strings.Contains(view, "128 queued") {
		t.Fatal("reload ignored watchlist")
	}
	reloaded = choose(reloaded, "queues")
	if !strings.Contains(ansi.Strip(reloaded.View()), "orders") {
		t.Fatal("watchlist removed discovery access")
	}
}

type scenario struct {
	explorer.Store
	resources           []explorer.Resource
	mu                  sync.Mutex
	active, peak, calls int
	fail                bool
	gate                chan struct{}
	started             chan struct{}
}

func (s *scenario) Discover(context.Context) ([]explorer.Resource, error) { return s.resources, nil }
func (s *scenario) Counts(ctx context.Context, r explorer.Resource) (explorer.Counts, error) {
	s.mu.Lock()
	s.active++
	s.calls++
	s.peak = max(s.peak, s.active)
	fail, gate := s.fail, s.gate
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	if gate != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return explorer.Counts{}, ctx.Err()
		}
	}
	if fail {
		return explorer.Counts{}, errors.New("access denied")
	}
	return explorer.Counts{Active: 37, DeadLetter: 6, State: "available", ObservedAt: time.Date(2026, 10, 23, 12, 34, 56, 0, time.UTC)}, nil
}

func TestWatchlistsUseFullIdentityAndEmptyMeansNone(t *testing.T) {
	base, _ := fixture.New()
	a := explorer.Resource{Profile: "p", Namespace: "east", Queue: "same"}
	b := explorer.Resource{Profile: "p", Namespace: "west", Queue: "same"}
	s := &scenario{Store: base, resources: []explorer.Resource{a, b}}
	p := config.Default()
	p.Watchlists = map[string][]explorer.Resource{"p": {b}}
	m := start(t, s, p, "fixture", false)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "1 watched entities") || strings.Count(view, "37 queued") != 1 || s.calls != 1 {
		t.Fatal("watchlist matched only display names")
	}
	m = choose(m, "settings")
	for range 6 {
		m = press(m, "j")
	}
	m = press(m, "enter")
	m = press(m, "esc")
	if !strings.Contains(ansi.Strip(m.View()), "Nothing watched") {
		t.Fatal("empty watchlist fell back to all")
	}
	m = choose(m, "queues")
	if !strings.Contains(ansi.Strip(m.View()), "2 discovered entities") {
		t.Fatal("watch-none hid resources from browser")
	}
}

func TestBoundedRefreshPreservesStaleValuesAndMessageSelection(t *testing.T) {
	base, _ := fixture.New()
	resources, _ := base.Discover(context.Background())
	s := &scenario{Store: base, resources: resources}
	m := start(t, s, config.Default(), "fixture", false)
	s.fail, s.gate, s.started = true, make(chan struct{}), make(chan struct{}, 4)
	m, refresh := key(m, "r")
	result := make(chan tea.Msg, 1)
	go func() { result <- refresh() }()
	for range 4 {
		select {
		case <-s.started:
		case <-time.After(time.Second):
			t.Fatal("refresh did not run four bounded reads")
		}
	}
	var duplicate tea.Cmd
	m, duplicate = key(m, "r")
	if duplicate != nil {
		t.Fatal("overlapping count sweep started")
	}
	m = press(m, "]")
	m = press(m, "j")
	close(s.gate)
	select {
	case msg := <-result:
		m, _ = m.Update(msg)
	case <-time.After(time.Second):
		t.Fatal("sweep did not finish")
	}
	if !strings.Contains(ansi.Strip(m.View()), "orders / DEAD-LETTER") || !strings.Contains(ansi.Strip(m.View()), "› 107") {
		t.Fatal("count result replaced message selection")
	}
	m = press(m, "esc")
	view := ansi.Strip(m.View())
	for _, want := range []string{"37~ queued", "6~ dead-letter", "Counts: stale", "12:34:56 UTC", "access denied"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing stale evidence %q\n%s", want, view)
		}
	}
	if s.peak > 4 {
		t.Fatalf("%d concurrent reads", s.peak)
	}
}

func TestAllThemesResponsivePaletteAndLargeTable(t *testing.T) {
	base, _ := fixture.New()
	s := &scenario{Store: base}
	for n := range 29 {
		s.resources = append(s.resources, explorer.Resource{Profile: "large", Namespace: "sample", Queue: fmt.Sprintf("queue-%02d", n)})
	}
	for _, name := range []string{"midnight", "gruvbox", "light"} {
		for _, ascii := range []bool{false, true} {
			p := config.Default()
			p.Theme = name
			m := start(t, s, p, "fixture", ascii)
			m = press(m, "t")
			view := ansi.Strip(m.View())
			if !strings.Contains(view, "queue-20") {
				t.Fatal("table does not show at least 20 entities")
			}
			for range 28 {
				m = press(m, "j")
			}
			if !strings.Contains(ansi.Strip(m.View()), "Selected 29/29") {
				t.Fatal("large watchlist unreachable")
			}
			for _, size := range [][2]int{{160, 48}, {110, 24}, {80, 24}, {60, 16}} {
				m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				assertFrame(t, m, size[0], size[1])
				m = press(m, "ctrl+p")
				assertFrame(t, m, size[0], size[1])
				m = press(m, "esc")
			}
			if ascii && strings.ContainsAny(ansi.Strip(m.View()), "┌┐└┘─│›▾▸·—…") {
				t.Fatal("ASCII fallback left UI glyphs")
			}
		}
	}
}

func TestEntityTreeCollapsesAndOpensSubscriptionDLQ(t *testing.T) {
	m := press(workspace(t), "tab")
	for range 4 {
		m = press(m, "j")
	}
	m = press(m, "h")
	if strings.Contains(ansi.Strip(m.View()), "billing") {
		t.Fatal("collapsed namespace retained children")
	}
	m = press(m, "l")
	m = press(m, "j") // queues group
	m = press(m, "h")
	m = press(m, "j") // topics group, not a hidden queue
	m = press(m, "j") // events topic
	m = press(m, "h")
	if strings.Contains(ansi.Strip(m.View()), "billing") {
		t.Fatal("collapsed topic retained subscriptions")
	}
	m = press(m, "l")
	m = press(m, "j") // billing subscription
	m = press(m, "]")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "events/billing / DEAD-LETTER") || !strings.Contains(view, "events/billing-dlq-0001") {
		t.Fatalf("tree did not open exact subscription DLQ\n%s", view)
	}
}

type longPreview struct{ explorer.Store }

func (s longPreview) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	p, err := s.Store.Peek(ctx, r, dlq, from, limit)
	if len(p.Messages) > 0 {
		p.Messages[0].Body = []byte("\x1b]52;c;clipboard\a測試\n" + strings.Repeat("body line\n", 80) + "END OF BODY")
	}
	return p, err
}

func TestInspectorScrollIsIndependentAndBrokerControlsAreData(t *testing.T) {
	s, _ := fixture.New()
	m := start(t, longPreview{s}, config.Default(), "fixture", false)
	m = press(m, "enter")
	if strings.ContainsAny(m.View(), "\a") || strings.Contains(m.View(), "\x1b]") {
		t.Fatal("broker controls reached terminal")
	}
	assertFrame(t, m, 160, 48)
	m = press(m, "tab")
	m = press(m, "pgdown")
	m = press(m, "pgdown")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "END OF BODY") || !strings.Contains(view, "› 100") {
		t.Fatal("inspector scroll changed message or lost body tail")
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = press(m, "j")
	if !strings.Contains(ansi.Strip(m.View()), "› 107") {
		t.Fatal("hidden inspector trapped keyboard focus")
	}
}

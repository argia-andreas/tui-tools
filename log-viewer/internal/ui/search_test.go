package ui_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"logviewer/internal/config"
	"logviewer/internal/fixture"
	"logviewer/internal/logs"
	"logviewer/internal/theme"
	"logviewer/internal/ui"
)

type driver struct {
	t *testing.T
	m ui.Model
}

func (d *driver) apply(msg tea.Msg) tea.Cmd {
	next, cmd := d.m.Update(msg)
	d.m = next.(ui.Model)
	return cmd
}
func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			d.run(child)
		}
		return
	}
	if _, ok := msg.(tea.QuitMsg); ok {
		d.t.Fatal("text input dispatched quit")
	}
	d.run(d.apply(msg))
}
func (d *driver) key(key string) { d.run(d.apply(keyMsg(key))) }
func keyMsg(key string) tea.KeyMsg {
	keys := map[string]tea.KeyType{"enter": tea.KeyEnter, "esc": tea.KeyEsc, "tab": tea.KeyTab, "ctrl+s": tea.KeyCtrlS, "ctrl+u": tea.KeyCtrlU, "ctrl+t": tea.KeyCtrlT, "ctrl+x": tea.KeyCtrlX, "ctrl+r": tea.KeyCtrlR, "down": tea.KeyDown}
	if k, ok := keys[key]; ok {
		return tea.KeyMsg{Type: k}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}
func (d *driver) view() string { return ansi.Strip(d.m.View()) }
func fixtureDriver(t *testing.T, size int) (*driver, string) {
	t.Helper()
	p, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	s, err := logs.NewService(p, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	q := fixture.Query()
	q.Limits.Events = size
	path := filepath.Join(t.TempDir(), "searches.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: q, Theme: theme.Midnight(), SearchesPath: path})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.run(d.m.Init())
	return d, path
}

func TestEditorsOwnTextAndCancelWithoutWritesOrFocusChanges(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	d.key("enter")
	before := d.view()
	d.key("/")
	d.key("q y ")
	if !strings.Contains(d.view(), "> q y ") {
		t.Fatal("find did not retain printable shortcut keys")
	}
	d.key("esc")
	if d.view() != before {
		t.Fatal("find cancellation changed results, selection or focus")
	}
	d.key("esc")
	before = d.view()
	d.key("f")
	d.key("q y ")
	if !strings.Contains(d.view(), "q y ") {
		t.Fatal("filter editor lost text")
	}
	d.key("esc")
	if d.view() != before {
		t.Fatal("filter cancellation changed results")
	}
	d.key("S")
	d.key("q y ")
	d.key("esc")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("editor wrote without explicit submit")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 60, Height: 16}} {
		d.apply(size)
		d.key("f")
		for i := 0; i < 6; i++ {
			d.key("tab")
			if !strings.Contains(d.view(), "FILTER EDITOR") {
				t.Fatal("narrow editor inaccessible")
			}
		}
		d.key("esc")
	}
}

func TestExactAppsInspectedTypedPredicatesSaveAndRestore(t *testing.T) {
	d, path := fixtureDriver(t, 500)
	d.key("f")
	d.key("payment")
	d.key("enter")
	if !strings.Contains(d.view(), "Selected: payment-worker") {
		t.Fatal("exact app selection missing")
	}
	d.key("ctrl+s")
	if !strings.Contains(d.view(), "App exact: payment-worker") || strings.Contains(d.view(), "POST /orders accepted") {
		t.Fatal("app facet not applied")
	}
	d.key("enter")
	d.key("f")
	if !strings.Contains(d.view(), "› /trace.id") {
		t.Fatal("trace scalar not offered")
	}
	d.key("enter")
	d.key("f")
	d.key("j") // supported field after trace: numeric job attempt
	if !strings.Contains(d.view(), "› /fields/job/attempt = 1") {
		t.Fatal("typed event predicate not offered")
	}
	d.key("enter")
	d.key("S")
	d.key("payment trace attempt")
	d.key("enter")
	saved, err := config.LoadSearches(path)
	if err != nil || len(saved) != 1 {
		t.Fatalf("explicit save: %v\n%s", err, d.view())
	}
	q := saved[0].Query
	if !reflect.DeepEqual(q.Scope.Services, []string{"payment-worker"}) || len(q.Fields) != 2 || q.Fields[1].Path != "/fields/job/attempt" || string(q.Fields[1].Value) != "1" {
		t.Fatalf("lost exact predicates: %+v", q)
	}
	d.key("esc")
	d.key("f")
	d.key("ctrl+x")
	d.key("ctrl+r")
	d.key("ctrl+s")
	d.key("R")
	d.key("enter")
	if !strings.Contains(d.view(), "App exact: payment-worker") || !strings.Contains(d.view(), "/fields/job/attempt=1") {
		t.Fatal("restore lost exact filters")
	}
	d.key("f")
	for i := 0; i < 6; i++ {
		d.key("tab")
	}
	d.key("q")
	d.key("y")
	d.key(" ")
	if !strings.Contains(d.view(), "› Exact field predicates") {
		t.Fatal("predicate row dispatched printable shortcuts")
	}
	d.key("down")
	d.key("enter")
	d.key("ctrl+s")
	if strings.Contains(d.view(), "/fields/job/attempt=1") || !strings.Contains(d.view(), "/trace.id=") || !strings.Contains(d.view(), "App exact: payment-worker") {
		t.Fatal("removing one predicate changed other facets")
	}
}

func TestFindSearchesDecodedStructuredFieldsAndCaseToggle(t *testing.T) {
	d, _ := fixtureDriver(t, 500)
	d.key("/")
	d.key("SWEDENCENTRAL")
	d.key("ctrl+t")
	d.key("enter")
	if !strings.Contains(d.view(), "No local literal matches") {
		t.Fatal("case-sensitive find ignored case")
	}
	d.key("/")
	d.key("ctrl+t")
	d.key("enter")
	if !strings.Contains(d.view(), "Local literal match") {
		t.Fatal("full safe structured field not searched")
	}
	d.key("N")
	d.key("n")
	if !strings.Contains(d.view(), "Payment gateway timeout") {
		t.Fatal("find navigation lost matched event")
	}
}

func TestOlderHistoryKeepsSelectedEventAnchored(t *testing.T) {
	d, _ := fixtureDriver(t, 3)
	d.key("enter")
	if !strings.Contains(d.view(), "Settlement batch completed") {
		t.Fatal("unexpected newest page")
	}
	d.key("o")
	if !strings.Contains(d.view(), "6 fictional events") || !strings.Contains(d.view(), "Settlement batch completed") {
		t.Fatal("history lost anchored selection")
	}
}

type cursorProvider struct {
	mu       sync.Mutex
	searches int
	closed   []string
}

func (p *cursorProvider) Search(ctx context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searches++
	n := p.searches
	return logs.NativePage{Events: []logs.Event{{Ref: logs.NativeRef{Index: "fixture", ID: fmt.Sprint(n)}, Time: q.Window.Start, Level: "INFO", Message: fmt.Sprintf("generation-%d", n)}}, More: true, Cursor: fmt.Sprintf("native-%d", n)}, nil
}
func (p *cursorProvider) Close(_ context.Context, cursor string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = append(p.closed, cursor)
	return nil
}
func (p *cursorProvider) Values(context.Context, logs.Query, string, string) (logs.Values, error) {
	return logs.Values{}, nil
}

func TestObsoleteCompletionCannotReplaceResultsAndClosesCursor(t *testing.T) {
	p := &cursorProvider{}
	s, err := logs.NewService(p, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	obsolete := d.m.Init()() // completed before a refresh, delivered out of order
	d.key("r")
	d.run(d.apply(obsolete))
	if !strings.Contains(d.view(), "generation-2") || strings.Contains(d.view(), "generation-1") {
		t.Fatal("stale search replaced current results")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !reflect.DeepEqual(p.closed, []string{"native-1"}) {
		t.Fatalf("stale cursor not closed: %v", p.closed)
	}
}

func TestRefreshDiscardsObsoleteOlderPageAndClosesItsNewCursor(t *testing.T) {
	p := &cursorProvider{}
	s, err := logs.NewService(p, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.run(d.m.Init())
	pageRequest := d.apply(keyMsg("o"))
	obsoletePage := pageRequest()
	d.key("r")
	d.run(d.apply(obsoletePage))
	if !strings.Contains(d.view(), "generation-3") || strings.Contains(d.view(), "generation-2") || strings.Contains(d.view(), "generation-1") {
		t.Fatal("obsolete history merged into refreshed results")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !reflect.DeepEqual(p.closed, []string{"native-2"}) {
		t.Fatalf("stale advanced cursor not closed: %v", p.closed)
	}
}

type tiedProvider struct{}

func (tiedProvider) Search(_ context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	ids := []int{3, 4}
	more := true
	next := "older"
	if cursor != "" {
		ids = []int{1, 2}
		more = false
		next = ""
	}
	page := logs.NativePage{More: more, Cursor: next}
	for _, id := range ids {
		page.Events = append(page.Events, logs.Event{Ref: logs.NativeRef{Index: "ties", ID: fmt.Sprint(id)}, Time: q.Window.Start, Level: "INFO", Message: fmt.Sprintf("tied-event-%d", id)})
	}
	return page, nil
}
func (tiedProvider) Close(context.Context, string) error { return nil }
func (tiedProvider) Values(context.Context, logs.Query, string, string) (logs.Values, error) {
	return logs.Values{}, nil
}

func TestOlderEqualTimePagesPrependWithoutReversingProviderOrder(t *testing.T) {
	s, err := logs.NewService(tiedProvider{}, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	d := &driver{t: t, m: ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight()})}
	d.apply(tea.WindowSizeMsg{Width: 160, Height: 48})
	d.run(d.m.Init())
	d.key("o")
	v := d.view()
	for id := 1; id < 4; id++ {
		if strings.Index(v, fmt.Sprintf("tied-event-%d", id)) >= strings.Index(v, fmt.Sprintf("tied-event-%d", id+1)) {
			t.Fatal("equal-time provider order reversed")
		}
	}
	if !strings.Contains(v, "› "+fixture.Query().Window.Start.Format("15:04:05.000")) {
		t.Fatal("anchored row selection missing")
	}
	d.key("enter")
	if !strings.Contains(d.view(), "MESSAGE") || strings.Count(d.view(), "tied-event-3") != 2 {
		t.Fatal("equal-time prepend lost selected event")
	}
}

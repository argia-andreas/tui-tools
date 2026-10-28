package ui_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
	"servicebusexplorer/internal/ui"
)

// Keep the fixture's identities, pagination and counts, but make body bytes
// deliberately unlike the rendered gutters, wraps, properties and metadata.
type inspectorFixture struct {
	explorer.Store
	body, full string
	truncated  bool
	fullError  error
	fullID     string
	mu         sync.Mutex
	lookups    []inspectorLookup
	mutations  int
}

type inspectorLookup struct {
	resource explorer.Resource
	dlq      bool
	sequence int64
}

func (s *inspectorFixture) Peek(ctx context.Context, r explorer.Resource, dlq bool, from int64, limit int) (explorer.Page, error) {
	p, err := s.Store.Peek(ctx, r, dlq, from, limit)
	for i := range p.Messages {
		p.Messages[i].Body = []byte(s.body)
		p.Messages[i].ContentType = "text/plain; charset=utf-8"
		p.Messages[i].PreviewTruncated = s.truncated
	}
	return p, err
}

func (s *inspectorFixture) PeekMessage(ctx context.Context, r explorer.Resource, dlq bool, sequence int64) (explorer.Message, error) {
	s.mu.Lock()
	s.lookups = append(s.lookups, inspectorLookup{r, dlq, sequence})
	s.mu.Unlock()
	m, err := s.Store.PeekMessage(ctx, r, dlq, sequence)
	m.Body = []byte(s.full)
	m.ContentType = "text/plain; charset=utf-8"
	if s.fullID != "" {
		m.ID = s.fullID
	}
	if s.fullError != nil {
		err = s.fullError
	}
	return m, err
}

func (s *inspectorFixture) rejectMutation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mutations++
	return errors.New("read-only inspector attempted mutation")
}

func (s *inspectorFixture) ReceiveNextDLQ(context.Context, explorer.Resource) (explorer.Delivery, error) {
	return nil, s.rejectMutation()
}

func (s *inspectorFixture) ReceiveSelectedDLQ(context.Context, explorer.Resource, int64) (explorer.Delivery, error) {
	return nil, s.rejectMutation()
}

func (s *inspectorFixture) PrepareDLQCopy(context.Context, explorer.Resource, int64) (explorer.Replay, error) {
	return nil, s.rejectMutation()
}

type inspectorClipboard struct {
	mu        sync.Mutex
	writes    []string
	confirmed bool
	err       error
}

func (c *inspectorClipboard) write(text string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, text)
	return c.confirmed, c.err
}

func (c *inspectorClipboard) assertWrites(t *testing.T, want ...string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if !reflect.DeepEqual(c.writes, want) {
		t.Fatalf("clipboard writes = %q, want %q", c.writes, want)
	}
}

func inspectorWorkspace(t *testing.T, body string, clipboard *inspectorClipboard) (tea.Model, *inspectorFixture) {
	t.Helper()
	base, err := fixture.New()
	if err != nil {
		t.Fatal(err)
	}
	s := &inspectorFixture{Store: base, body: body, full: body}
	m := ui.New(s, ui.Options{Mode: "fixture", Preferences: config.Default(), Clipboard: clipboard.write, ConfigPath: filepath.Join(t.TempDir(), "preferences.json")})
	ready, counts := m.Update(m.Init()())
	ready, _ = ready.Update(counts())
	t.Cleanup(func() {
		_, _ = ready.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.mutations != 0 {
			t.Errorf("inspector attempted %d mutating broker operations", s.mutations)
		}
	})
	return ready, s
}

func inspectorKeys(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		m = press(m, k)
	}
	return m
}

func inspectorContains(t *testing.T, m tea.Model, wants ...string) {
	t.Helper()
	view := ansi.Strip(m.View())
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
}

func TestSharedHelpRetainsReaderSelectionAndProcessesFullLoad(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, s := inspectorWorkspace(t, "line one\nline two", c)
	m = inspectorKeys(m, "enter", "enter", "v", "l")
	before := m.View()
	m = inspectorKeys(m, "?", "q", "j", "Y", "ctrl+p")
	var help string
	for range 8 {
		help += ansi.Strip(m.View())
		m = press(m, "pgdown")
	}
	for _, want := range []string{"yy - Copy whole logical line", "y / Y - Copy selection / full body", "Ctrl+P - Open Actions, not copy report", "terminal permission", "gutters/soft wraps"} {
		if !strings.Contains(help, want) {
			t.Fatalf("grouped reader Help lost %q", want)
		}
	}
	m = press(m, "esc")
	if m.View() != before {
		t.Fatal("Help altered reader cursor/selection")
	}
	c.assertWrites(t)
	m = inspectorKeys(m, "esc", "esc", "esc")
	s.truncated, s.full = true, "exact full body"
	m = inspectorKeys(m, "enter", "enter", "Y")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	inspectorContains(t, m, "Full load active")
	m = press(m, "ctrl+p")
	if strings.Contains(m.View(), "Query:") {
		t.Fatal("full-load capture allowed palette")
	}
	m = inspectorKeys(m, "tab")
	m, load := key(m, "enter")
	if load == nil {
		t.Fatal("exact load missing")
	}
	m = press(m, "f1")
	m, _ = m.Update(load())
	inspectorContains(t, m, " HELP ")
	m = press(m, "esc")
	inspectorContains(t, m, "Full message loaded")
	c.assertWrites(t)
}

func TestReaderLocalControlsRetainExactCopyPolicy(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, _ := inspectorWorkspace(t, "first asymmetric\r\n界é\nthird", c)
	m = inspectorKeys(m, "]", "enter")
	m = clickText(t, m, "yy line", false)
	c.assertWrites(t, "first asymmetric\r\n")
	m = clickText(t, m, "v char", false)
	m = press(m, "l")
	m = clickText(t, m, "y Copy selection", false)
	c.assertWrites(t, "first asymmetric\r\n", "fi")
	m = clickText(t, m, "V line", false)
	m = press(m, "j")
	m = clickText(t, m, "y Copy selection", false)
	c.assertWrites(t, "first asymmetric\r\n", "fi", "first asymmetric\r\n界é\n")
	m = clickText(t, m, "Y full body", false)
	c.assertWrites(t, "first asymmetric\r\n", "fi", "first asymmetric\r\n界é\n", "first asymmetric\r\n界é\nthird")
	m = clickText(t, m, "? help", false)
	inspectorContains(t, m, " HELP ")
	m = press(m, "esc")
	inspectorContains(t, m, "ID: orders-dlq-0001")
}

func TestInspectorEnterFocusesSelectedPeekWithoutMutation(t *testing.T) {
	for _, dlq := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "dlq"}[dlq], func(t *testing.T) {
			c := &inspectorClipboard{confirmed: true}
			m, s := inspectorWorkspace(t, "first\r\nsecond\r\nthird", c)
			if dlq {
				m = press(m, "]")
			} else {
				m = press(m, "enter")
			}
			m = press(m, "j") // Select message 0002 before entering the reader.
			var cmd tea.Cmd
			m, cmd = key(m, "enter")
			if cmd != nil {
				t.Fatal("Enter issued an operation rather than focusing the existing peek")
			}
			id := "orders-active-0002"
			if dlq {
				id = "orders-dlq-0002"
			}
			inspectorContains(t, m, "ID: "+id, "NORMAL · body 1:1")
			m = inspectorKeys(m, "enter", "j", "y", "y")
			inspectorContains(t, m, "ID: "+id, "NORMAL · body 2:1")
			c.assertWrites(t, "second\r\n")
			s.mu.Lock()
			defer s.mu.Unlock()
			if len(s.lookups) != 0 {
				t.Fatal("focusing performed a full-message lookup")
			}
		})
	}
}

func TestInspectorCopyExactLogicalBytes(t *testing.T) {
	long := "α\t" + strings.Repeat("界", 100) + "tail\r\n"
	for _, tc := range []struct {
		name, body, want string
		keys             []string
	}{
		{"yy ignores soft wraps and gutters", long + "other", long, []string{"l", "y", "y"}},
		{"character UTF8 graphemes", "Aé界e\u0301🙂Z\r\nnext", "é界e\u0301🙂", []string{"l", "v", "l", "l", "l", "y"}},
		{"reversed characters", "Aé界e\u0301🙂Z\r\nnext", "é界e\u0301🙂", []string{"$", "h", "v", "h", "h", "h", "y"}},
		{"characters across CRLF", "Aé界\r\nβZ!\nlast", "é界\r\nβZ", []string{"l", "v", "j", "y"}},
		{"lines preserve mixed endings", "Aé界\r\nβZ!\nlast", "Aé界\r\nβZ!\n", []string{"V", "j", "y"}},
		{"reversed lines", "Aé界\r\nβZ!\nlast", "Aé界\r\nβZ!\n", []string{"j", "V", "k", "y"}},
		{"Y excludes properties and metadata", "Aé\r\nβ\nlast\t🙂", "Aé\r\nβ\nlast\t🙂", []string{"j", "l", "Y"}},
		{"word end advances from word end", "alpha béta tail", "alpha béta", []string{"v", "e", "e", "y"}},
		{"word end starts in whitespace", "  béta tail", "  béta", []string{"v", "e", "y"}},
		{"word end crosses punctuation", "ab, cd", "ab,", []string{"v", "e", "e", "y"}},
		{"vertical motion restores preferred column", "abcdef\r\nx\r\nαβγδεζ", "ε", []string{"l", "l", "l", "l", "j", "j", "v", "y"}},
		{"reverse vertical motion restores preferred column", "abcdef\r\nx\r\nαβγδεζ", "e", []string{"l", "l", "l", "l", "j", "j", "k", "k", "v", "y"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &inspectorClipboard{confirmed: true}
			m, _ := inspectorWorkspace(t, tc.body, c)
			m = inspectorKeys(m, "enter", "enter")
			m = inspectorKeys(m, tc.keys...)
			c.assertWrites(t, tc.want)
			inspectorContains(t, m, "Copied", "INSPECTOR / NORMAL", "ID: orders-active-0001")
		})
	}
}

func TestInspectorClipboardFeedbackAndSelectionRetention(t *testing.T) {
	for _, tc := range []struct {
		name      string
		confirmed bool
		err       error
		feedback  string
	}{
		{"native", true, nil, "Copied 2 characters · body"},
		{"unconfirmed", false, nil, "Copy requested · 2 characters · body · terminal confirmation unavailable"},
		{"failure", false, errors.New("test clipboard denied"), "Clipboard unavailable; selection kept."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &inspectorClipboard{confirmed: tc.confirmed, err: tc.err}
			m, _ := inspectorWorkspace(t, "é界Z", c)
			m = inspectorKeys(m, "enter", "enter", "v", "l", "y")
			c.assertWrites(t, "é界")
			inspectorContains(t, m, tc.feedback)
			if tc.err != nil {
				inspectorContains(t, m, "VISUAL CHAR · 2 characters selected", "test clipboard denied")
				// A retry must copy precisely the retained selection, not yy.
				m = press(m, "y")
				c.assertWrites(t, "é界", "é界")
			} else {
				inspectorContains(t, m, "INSPECTOR / NORMAL")
				if strings.Contains(ansi.Strip(m.View()), "VISUAL CHAR") {
					t.Fatal("successful copy retained selection")
				}
			}
		})
	}
}

func TestInspectorTruncatedYRequiresExplicitExactLoadAndSecondY(t *testing.T) {
	for _, dlq := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "dlq"}[dlq], func(t *testing.T) {
			c := &inspectorClipboard{confirmed: true}
			m, s := inspectorWorkspace(t, "short preview", c)
			s.truncated, s.full = true, "exact full é界\r\nnot the preview\t🙂"
			resources, err := s.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if dlq {
				m = press(m, "]")
			} else {
				m = press(m, "enter")
			}
			m = inspectorKeys(m, "j", "enter", "Y")
			inspectorContains(t, m, "Load full message for copy?", "clipboard unchanged")
			c.assertWrites(t)
			// Cancel is the default, so Enter alone must not read or copy.
			m = press(m, "enter")
			s.mu.Lock()
			calls := len(s.lookups)
			s.mu.Unlock()
			if calls != 0 {
				t.Fatal("default Cancel performed full lookup")
			}
			m = inspectorKeys(m, "Y", "tab", "enter")
			inspectorContains(t, m, "Full message loaded", "Y copies full body")
			c.assertWrites(t)
			s.mu.Lock()
			got := append([]inspectorLookup(nil), s.lookups...)
			s.mu.Unlock()
			want := []inspectorLookup{{resources[0], dlq, 107}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("full lookups = %+v, want %+v", got, want)
			}
			m = press(m, "Y")
			c.assertWrites(t, s.full)
			inspectorContains(t, m, "Copied full body")
		})
	}
}

func TestInspectorActiveHasNoResendAndEscapeFocusStages(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, _ := inspectorWorkspace(t, "one\r\ntwo\r\nthree", c)
	m = inspectorKeys(m, "enter", "enter")
	if strings.Contains(ansi.Strip(m.View()), "^R Resend") || strings.Contains(ansi.Strip(m.View()), "^R Send copy") {
		t.Fatal("Active inspector exposes a resend button")
	}
	m = press(m, "ctrl+r")
	inspectorContains(t, m, "ID: orders-active-0001", "INSPECTOR / NORMAL")
	m = inspectorKeys(m, "v", "l", "esc") // First Esc clears only selection.
	inspectorContains(t, m, "NORMAL · body 1:2", "ID: orders-active-0001")
	m = press(m, "j")
	inspectorContains(t, m, "NORMAL · body 2:2", "ID: orders-active-0001")
	m = inspectorKeys(m, "esc", "j") // Second Esc gives keys back to list.
	inspectorContains(t, m, "ID: orders-active-0002")
	m = press(m, "esc") // List Esc returns to workspace.
	inspectorContains(t, m, "WORKSPACE", "128 queued", "12 dead-letter")
	c.assertWrites(t)
}

func TestInspectorFullLoadNeverCopiesUnavailableNeighborOrCancelledResult(t *testing.T) {
	for _, reason := range []string{"missing", "neighbor", "cancelled"} {
		t.Run(reason, func(t *testing.T) {
			c := &inspectorClipboard{confirmed: true}
			m, s := inspectorWorkspace(t, "preview only", c)
			s.truncated, s.full = true, "full content"
			if reason == "missing" {
				s.fullError = errors.New("gone")
			}
			if reason == "neighbor" {
				s.fullID = "not-the-selected-message"
			}
			m = inspectorKeys(m, "]", "enter", "Y", "tab")
			var load tea.Cmd
			m, load = key(m, "enter")
			if load == nil {
				t.Fatal("explicit load did not start")
			}
			if reason == "cancelled" {
				m = press(m, "esc")
			}
			m, _ = m.Update(load())
			c.assertWrites(t)
			inspectorContains(t, m, "preview only", "clipboard unchanged", "PREVIEW TRUNCATED")
			if strings.Contains(ansi.Strip(m.View()), "full content") {
				t.Fatal("unavailable or stale result replaced preview")
			}
		})
	}
}

func TestInspectorResizeHelpAndKeyboardOwnershipPreserveSelection(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, _ := inspectorWorkspace(t, "abcd界\r\nsecond", c)
	m = inspectorKeys(m, "]", "enter", "l", "v", "l")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	assertFrame(t, m, 60, 16)
	m = inspectorKeys(m, "?", "j", "j", "esc", "r", "n", "p", "[", "]", "ctrl+r")
	inspectorContains(t, m, "ID: orders-dlq-0001", "VISUAL CHAR", "Esc clears selection before resend")
	m = press(m, "y")
	c.assertWrites(t, "bc")
	m = press(m, "ctrl+r") // Too small to confirm/configure; visible Cancel and Esc remain safe.
	inspectorContains(t, m, "Resize to at least 80×24")
	m = press(m, "esc")
	inspectorContains(t, m, "INSPECTOR / NORMAL")
}

func TestInspectorPagesThroughWrappedPayloadWithoutSkippingOrCopyingWraps(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	line := strings.Repeat("界", 400) + "\r\n"
	m, _ := inspectorWorkspace(t, line+"different next line", c)
	m = inspectorKeys(m, "]", "enter")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	m = inspectorKeys(m, "pgdown", "v", "y", "y", "y")
	c.assertWrites(t, "界", line)
	inspectorContains(t, m, "ID: orders-dlq-0001", "NORMAL · body 1:")
	m = press(m, "pgup")
	inspectorContains(t, m, "NORMAL · body 1:1")
}

func TestInspectorDirectionalPanesCoexistWithActionTabCycle(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, _ := inspectorWorkspace(t, "first\nsecond", c)
	m = inspectorKeys(m, "]", "enter", "tab", "tab", "tab", "tab", "tab", "tab") // Local reader controls precede Back.
	inspectorContains(t, m, "›[ Esc Back ]")
	m = press(m, "ctrl+l") // Right edge clamps without activating or leaving Back.
	inspectorContains(t, m, "›[ Esc Back ]")
	m = inspectorKeys(m, "shift+tab", "tab", "tab")
	inspectorContains(t, m, "›[ ^R Resend to queue… ]")
	m = press(m, "ctrl+l") // Action buttons belong to the inspector pane.
	inspectorContains(t, m, "›[ ^R Resend to queue… ]")
	m = inspectorKeys(m, "ctrl+h", "j")
	inspectorContains(t, m, "ID: orders-dlq-0002")
	m = inspectorKeys(m, "ctrl+l", "j")
	inspectorContains(t, m, "ID: orders-dlq-0002", "NORMAL · body 2:1")
	m = inspectorKeys(m, "ctrl+h", "j")
	inspectorContains(t, m, "ID: orders-dlq-0003") // Pane shortcut skips local controls.
	c.assertWrites(t)
}

func TestInspectorSurfacePointerPreservesSelectionAndUsesExplicitBack(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, _ := inspectorWorkspace(t, "first asymmetric\nsecond", c)
	m = inspectorKeys(m, "]", "enter", "v", "l")
	m = clickText(t, m, "^R Resend to queue…", false)
	inspectorContains(t, m, "VISUAL CHAR")
	if strings.Contains(ansi.Strip(m.View()), "Destination queue:") {
		t.Fatal("disabled Resend bypassed reader selection")
	}
	m = press(m, "esc") // Keyboard Esc clears selection, without leaving the reader.
	inspectorContains(t, m, "INSPECTOR / NORMAL")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = clickText(t, m, "Esc Back", true)
	inspectorContains(t, m, "INSPECTOR / NORMAL")
	m = clickText(t, m, "Esc Back", false)
	if strings.Contains(ansi.Strip(m.View()), "INSPECTOR / NORMAL") {
		t.Fatal("explicit Back failed to return to compact message list")
	}
	m = inspectorKeys(m, "enter", "v", "l")
	m = clickText(t, m, "Esc Back", false) // Explicit Back is not synthetic first-Esc.
	if strings.Contains(ansi.Strip(m.View()), "VISUAL CHAR") {
		t.Fatal("explicit Back merely cleared selection")
	}
	c.assertWrites(t)
}

func TestInspectorFullLoadPointerCancellationRejectsLateCompletion(t *testing.T) {
	c := &inspectorClipboard{confirmed: true}
	m, s := inspectorWorkspace(t, "preview-only", c)
	s.truncated, s.full = true, "exact full asymmetric\r\n界"
	m = inspectorKeys(m, "]", "j", "enter", "Y")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = clickText(t, m, "Load full message ]", true)
	inspectorContains(t, m, "Load full message for copy?")
	// Capture the real command without delivering its result until after Cancel.
	var load tea.Cmd
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if x := strings.Index(row, "Load full message ]"); x >= 0 {
			m, load = m.Update(tea.MouseMsg{X: ansi.StringWidth(row[:x]), Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			break
		}
	}
	if load == nil {
		t.Fatal("visible full-load pointer action did not start lookup")
	}
	inspectorContains(t, m, "Loading exact full message")
	m = clickText(t, m, "Cancel", false)
	m, _ = m.Update(load())
	inspectorContains(t, m, "preview-only", "clipboard unchanged")
	if strings.Contains(ansi.Strip(m.View()), "exact full asymmetric") {
		t.Fatal("cancelled late full load replaced preview")
	}
	c.assertWrites(t)
}

func TestReaderSelectionHintsKeepPointerAndTabOrderAcrossLayouts(t *testing.T) {
	for _, size := range [][2]int{{160, 48}, {80, 24}, {60, 16}} {
		c := &inspectorClipboard{confirmed: true}
		m, _ := inspectorWorkspace(t, "first asymmetric\nsecond", c)
		m = inspectorKeys(m, "]", "enter")
		m, _ = m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = press(m, "v")
		m = clickText(t, m, "Esc clear", false)
		inspectorContains(t, m, "INSPECTOR / NORMAL")
		m = inspectorKeys(m, "v", "tab", "enter") // Selection hints keep Help first.
		inspectorContains(t, m, " HELP ")
		m = press(m, "esc")
		inspectorContains(t, m, "VISUAL CHAR")
		c.assertWrites(t)
	}
}

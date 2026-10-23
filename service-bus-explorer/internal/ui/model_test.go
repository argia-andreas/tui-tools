package ui_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
	m := ui.New(s, "fixture")
	ready, _ := m.Update(m.Init()())
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
	m := ui.New(unavailableCounts{store}, "emulator")
	ready, _ := m.Update(m.Init()())
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

package ui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func bodyPointer(t *testing.T, m tea.Model, label string, button tea.MouseButton, action tea.MouseAction) tea.Model {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if len(row) < 24 {
			continue
		}
		if x := strings.Index(row[24:], label); x >= 0 {
			next, cmd := m.Update(tea.MouseMsg{X: ansi.StringWidth(row[:24+x]), Y: y, Button: button, Action: action})
			if cmd != nil {
				next, _ = next.Update(cmd())
			}
			return next
		}
	}
	t.Fatalf("body pointer target %q absent\n%s", label, ansi.Strip(m.View()))
	return m
}

func TestWorkspacePointerRowsSelectBeforeExplicitOpen(t *testing.T) {
	m := bodyPointer(t, workspace(t), "payments", tea.MouseButtonLeft, tea.MouseActionPress)
	inspectorContains(t, m, "Selected 2/14", "GRID")
	if strings.Contains(m.View(), "payments / ACTIVE") {
		t.Fatal("grid selection opened the entity")
	}
	m = clickText(t, m, "Open Active", false)
	inspectorContains(t, m, "payments / ACTIVE", "payments-active-0001")
	m = press(m, "esc")
	m = clickText(t, m, "Table t", false)
	m = bodyPointer(t, m, "shipping", tea.MouseButtonLeft, tea.MouseActionPress)
	inspectorContains(t, m, "Selected 5/14", "TABLE")
	m = clickText(t, m, "Peek DLQ", false)
	inspectorContains(t, m, "shipping / DEAD-LETTER")
	m = bodyPointer(t, m, "shipping-dlq-0002", tea.MouseButtonLeft, tea.MouseActionPress)
	m = press(m, "j") // A mistaken inspect would move the reader, not select 114.
	inspectorContains(t, m, "› 114")
	m = clickText(t, m, "Inspect / Open", false)
	inspectorContains(t, m, "ID: shipping-dlq-0003")
	m = clickText(t, m, "Esc Back", false)
	m = clickText(t, m, "Active [", false)
	inspectorContains(t, m, "shipping-active-0001", "PEEK ONLY")
}

func TestDisclosureLabelSelectionAndSettingsPointerParity(t *testing.T) {
	m := workspace(t)
	m = clickText(t, m, "fixture / local-lab", false) // Label is not disclosure.
	inspectorContains(t, m, "billing")
	m = clickText(t, m, "▾ fixture", false)
	if strings.Contains(m.View(), "billing") {
		t.Fatal("disclosure did not hide descendants")
	}
	m = clickText(t, m, "▸ fixture", false)
	inspectorContains(t, m, "billing")
	m = clickText(t, m, "Settings", false)
	m = clickText(t, m, "Navigation sidebar: Expanded", false)
	if strings.Contains(m.View(), "WORKSPACE") {
		t.Fatal("sidebar toggle did not hide pane")
	}
	m = clickText(t, m, "Navigation sidebar: Collapsed", false)
	inspectorContains(t, m, "WORKSPACE")
	m = clickText(t, m, "Message inspector: Visible", false)
	inspectorContains(t, m, "Message inspector: Hidden")
	m = clickText(t, m, "Dashboard layout: Grid", false)
	inspectorContains(t, m, "Dashboard layout: Table")
	m = clickText(t, m, "[x] fixture / local-lab / orders", false)
	inspectorContains(t, m, "[ ] fixture / local-lab / orders")
	m = clickText(t, m, "Save preferences", false)
	inspectorContains(t, m, "Preferences saved")
	m = clickText(t, m, "Dashboard", false)
	inspectorContains(t, m, "13 watched entities")
}

func TestWorkspaceWheelLocalityAndRelease(t *testing.T) {
	m := press(workspace(t), "enter")
	before := m.View()
	m = bodyPointer(t, m, "orders-active-0002", tea.MouseButtonLeft, tea.MouseActionRelease)
	if before != m.View() {
		t.Fatal("release selected or activated a row")
	}
	m = bodyPointer(t, m, "orders-active-0002", tea.MouseButtonWheelDown, tea.MouseActionPress)
	inspectorContains(t, m, "› 121") // Exactly three list rows, not reader scrolling.
	m = bodyPointer(t, m, "FULL BODY / read-only", tea.MouseButtonWheelDown, tea.MouseActionPress)
	inspectorContains(t, m, "› 121", "ID: orders-active-0004")
	// Hidden sidebar control focus is repaired after resize; Enter opens the
	// selected content entity, never its stale navigation destination.
	m = press(m, "esc")
	m = press(m, "ctrl+h")
	m = press(m, "tab") // Dashboard destination, not the content pane.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = press(m, "enter")
	inspectorContains(t, m, "orders / ACTIVE")
}

func TestVisibleControlTabTraversalAndEnter(t *testing.T) {
	m := workspace(t)
	m = press(m, "tab")
	inspectorContains(t, m, "›[ Open Active ]")
	m = press(m, "tab")
	inspectorContains(t, m, "›[ Peek DLQ ]")
	m = press(m, "enter")
	inspectorContains(t, m, "orders / DEAD-LETTER")
}

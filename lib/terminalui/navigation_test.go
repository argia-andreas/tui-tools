package terminalui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func TestNavigationSelectActivateAndCurrentVisibility(t *testing.T) {
	f := terminalui.Frame{Bounds: terminalui.Rect{X: 3, Y: 2, W: 23, H: 10}, Regions: []terminalui.Region{
		{ID: "files", Kind: terminalui.PaneRegion, Bounds: terminalui.Rect{X: 3, Y: 2, W: 12, H: 10}},
		{ID: "file:/a", Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: 4, Y: 4, W: 10, H: 1}},
		{ID: "mark:/a", Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: 4, Y: 4, W: 2, H: 1}},
		{ID: "reader", Kind: terminalui.PaneRegion, Bounds: terminalui.Rect{X: 15, Y: 2, W: 11, H: 10}},
		{ID: "open", Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: 21, Y: 10, W: 5, H: 1}},
	}}
	check := func(msg tea.Msg, focus, selected string, kind terminalui.IntentKind, id string) {
		t.Helper()
		r := terminalui.HandleNavigation(msg, f, focus, selected)
		if !r.Consumed || r.Intent == nil || r.Intent.Kind != kind || r.Intent.ID != id {
			t.Fatalf("wanted %v %q, got %+v", kind, id, r)
		}
	}
	check(tea.MouseMsg{X: 9, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}, "reader", "", terminalui.Select, "file:/a")
	check(tea.MouseMsg{X: 4, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}, "reader", "", terminalui.Activate, "mark:/a")
	check(tea.KeyMsg{Type: tea.KeyEnter}, "files", "file:/a", terminalui.Activate, "file:/a")
	check(tea.KeyMsg{Type: tea.KeyTab}, "files", "file:/a", terminalui.Focus, "mark:/a")
	check(tea.KeyMsg{Type: tea.KeyShiftTab}, "files", "file:/a", terminalui.Focus, "open")
	check(tea.MouseMsg{X: 18, Y: 6, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}, "files", "file:/a", terminalui.Scroll, "reader")
	for _, msg := range []tea.Msg{
		tea.MouseMsg{X: 9, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		tea.MouseMsg{X: 9, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
		tea.KeyMsg{Type: tea.KeyEnter},
	} {
		if r := terminalui.HandleNavigation(msg, f, "reader", "file:/a"); r.Intent != nil {
			t.Fatalf("release/motion or other pane's selection activated: %+v", r)
		}
	}
	// Resize clips a control but leaves the pane visible. Neither Enter nor a
	// press may activate the old control, and traversal skips it entirely.
	f.Bounds.W = 21
	check(tea.KeyMsg{Type: tea.KeyShiftTab}, "files", "file:/a", terminalui.Focus, "reader")
	if r := terminalui.HandleNavigation(tea.KeyMsg{Type: tea.KeyEnter}, f, "open", "file:/a"); r.Intent != nil {
		t.Fatal("hidden former control activated")
	}
	if r := terminalui.HandleNavigation(tea.MouseMsg{X: 22, Y: 10, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}, f, "files", "file:/a"); r.Intent != nil && r.Intent.Kind == terminalui.Activate {
		t.Fatal("partly clipped control retained a hitbox")
	}
}

package terminalui_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	ui "terminalui"
)

func key(kind tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: kind} }
func text(s string) tea.KeyMsg        { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func press(r ui.Region) tea.MouseMsg {
	return tea.MouseMsg{X: r.Bounds.X, Y: r.Bounds.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}
func wantIntent(t *testing.T, r ui.Result, kind ui.IntentKind, id string) {
	t.Helper()
	if !r.Consumed || r.Intent == nil || r.Intent.Kind != kind || r.Intent.ID != id {
		t.Fatalf("got %+v, intent %+v; want consumed %v %q", r, r.Intent, kind, id)
	}
}
func wantCaptured(t *testing.T, r ui.Result) {
	t.Helper()
	if !r.Consumed || r.Intent != nil {
		t.Fatalf("want captured without intent, got %+v, intent %+v", r, r.Intent)
	}
}

func TestActionActivationParity(t *testing.T) {
	actions := []ui.Action{
		{ID: "inspect", Label: "Inspect", Keys: []string{"i"}},
		{ID: "delete", Label: "Delete", Keys: []string{"d"}, Dangerous: true, Unavailable: "read only"},
		{ID: "load", Label: "Load", Keys: []string{"l"}, Busy: true},
	}
	f := ui.RenderActionBar(actions, ui.Rect{X: 3, Y: 4, W: 150, H: 1}, "inspect", ui.Style{})
	for i, a := range actions {
		for _, msg := range []tea.Msg{press(f.Regions[i]), text(a.Keys[0])} {
			r := ui.HandleActionBar(msg, actions, f, "inspect")
			if i == 0 {
				wantIntent(t, r, ui.Activate, a.ID)
			} else {
				wantCaptured(t, r)
			}
		}
	}
	wantIntent(t, ui.HandleActionBar(key(tea.KeyEnter), actions, f, "inspect"), ui.Activate, "inspect")
	for _, action := range []tea.MouseAction{tea.MouseActionRelease, tea.MouseActionMotion} {
		m := press(f.Regions[0])
		m.Action = action
		if r := ui.HandleActionBar(m, actions, f, "inspect"); r.Intent != nil {
			t.Fatalf("release/motion activated: %+v", r)
		}
	}
	changed := append([]ui.Action(nil), actions...)
	changed[0].Busy = true
	wantCaptured(t, ui.HandleActionBar(press(f.Regions[0]), changed, f, "inspect"))
	small := ui.RenderActionBar(actions, ui.Rect{W: 1, H: 1}, "inspect", ui.Style{})
	if r := ui.HandleActionBar(text("i"), actions, small, "inspect"); r.Intent != nil {
		t.Fatal("hidden control activated")
	}
}

func TestPaletteCaptureAndNavigation(t *testing.T) {
	actions := []ui.Action{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}, {ID: "c", Label: "Gamma"}}
	p := ui.Palette{}
	bounds := ui.Rect{X: 8, Y: 3, W: 72, H: 8} // two visible rows
	handle := func(msg tea.Msg) ui.Result { return p.Handle(msg, actions, p.View(actions, bounds, ui.Style{})) }
	for range 5 {
		wantCaptured(t, handle(key(tea.KeyUp)))
	}
	if p.SelectedID != "a" {
		t.Fatal("first result did not clamp")
	}
	for range 5 {
		wantCaptured(t, handle(key(tea.KeyTab)))
	}
	if p.SelectedID != "c" || p.Offset != 1 {
		t.Fatalf("last result/viewport did not clamp: %+v", p)
	}
	for range 5 {
		handle(key(tea.KeyShiftTab))
	}
	if p.SelectedID != "a" || p.Offset != 0 {
		t.Fatalf("reverse clamp failed: %+v", p)
	}
	for _, s := range []string{"q", "j", "?"} {
		wantCaptured(t, handle(text(s)))
	}
	if p.Query != "qj?" || p.SelectedID != "" {
		t.Fatalf("query leaked or no-results selection survived: %+v", p)
	}
	wantCaptured(t, handle(key(tea.KeyEnter)))
	wantCaptured(t, handle(key(tea.KeyCtrlU)))
	wantCaptured(t, handle(text("\x00\x1b\n\t\u200d\u202e")))
	if p.Query != "" {
		t.Fatalf("control/format accepted: %q", p.Query)
	}
	handle(text(strings.Repeat("界", 140)))
	if utf8.RuneCountInString(p.Query) != 128 {
		t.Fatalf("query limit: %d", utf8.RuneCountInString(p.Query))
	}
	handle(key(tea.KeyCtrlH))
	handle(key(tea.KeyBackspace))
	if utf8.RuneCountInString(p.Query) != 126 {
		t.Fatal("backspace did not remove Unicode runes")
	}
	handle(key(tea.KeyCtrlU))
	handle(key(tea.KeySpace))
	if p.Query != " " {
		t.Fatal("space not captured")
	}
	handle(key(tea.KeyCtrlU))
	handle(key(tea.KeyDown))
	handle(text("alp"))
	if p.SelectedID != "a" || p.Offset != 0 {
		t.Fatalf("new query did not reset selection: %+v", p)
	}
	wantIntent(t, handle(key(tea.KeyCtrlP)), ui.Dismiss, "")
	wantIntent(t, handle(key(tea.KeyEsc)), ui.Dismiss, "")
	wantIntent(t, handle(key(tea.KeyF1)), ui.Help, "")
}

func TestPaletteIdentityAndEligibility(t *testing.T) {
	a := ui.Action{ID: "a", Label: "Alpha", Keywords: "reader"}
	b := ui.Action{ID: "b", Label: "Beta", Keys: []string{"b"}}
	p := ui.Palette{SelectedID: "b"}
	bounds := ui.Rect{X: 10, Y: 2, W: 80, H: 10}
	old := p.View([]ui.Action{a, b}, bounds, ui.Style{})
	wantIntent(t, p.Handle(key(tea.KeyEnter), []ui.Action{b, a}, old), ui.Activate, "b")
	wantIntent(t, p.Handle(press(old.Regions[1]), []ui.Action{b, a}, old), ui.Activate, "b")
	// A removed row must not activate the replacement now at its old index.
	wantCaptured(t, p.Handle(press(old.Regions[1]), []ui.Action{a}, old))
	if p.SelectedID != "a" {
		t.Fatalf("removed selection was not repaired: %+v", p)
	}
	for _, unavailable := range []ui.Action{
		{ID: "b", Label: "Beta", Unavailable: "read only"},
		{ID: "b", Label: "Beta", Busy: true},
	} {
		p.SelectedID = "b"
		wantCaptured(t, p.Handle(key(tea.KeyEnter), []ui.Action{unavailable, a}, old))
		wantCaptured(t, p.Handle(press(old.Regions[1]), []ui.Action{a, unavailable}, old))
	}
	p.Query = "reader"
	wantCaptured(t, p.Handle(press(old.Regions[1]), []ui.Action{a, b}, old))
	wantIntent(t, p.Handle(key(tea.KeyEnter), []ui.Action{a, b}, old), ui.Activate, "a")
	// Label matches win over keywords, even when the keyword action comes first.
	c := ui.Action{ID: "c", Label: "Reader"}
	f := p.View([]ui.Action{a, c}, bounds, ui.Style{})
	if len(f.Regions) != 3 || f.Regions[0].ID != "c" || f.Regions[1].Kind != ui.ControlRegion {
		t.Fatalf("label priority: %+v", f.Regions)
	}
}

func TestPaletteDismissConsumesPointerAndWheel(t *testing.T) {
	actions := []ui.Action{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}
	p := ui.Palette{}
	f := p.View(actions, ui.Rect{X: 10, Y: 4, W: 60, H: 9}, ui.Style{})
	wantIntent(t, p.Handle(press(f.Regions[len(f.Regions)-1]), actions, f), ui.Dismiss, "")
	outside := tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	wantIntent(t, p.Handle(outside, actions, f), ui.Dismiss, "")
	outside.Action = tea.MouseActionRelease
	wantCaptured(t, p.Handle(outside, actions, f))
	for _, action := range []tea.MouseAction{tea.MouseActionRelease, tea.MouseActionMotion} {
		m := press(f.Regions[1])
		m.Action = action
		wantCaptured(t, p.Handle(m, actions, f))
	}
	wheel := tea.MouseMsg{X: 11, Y: 7, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	wantCaptured(t, p.Handle(wheel, actions, f))
	if p.SelectedID != "b" {
		t.Fatal("wheel did not select locally")
	}
	wheel.X, wheel.Y, wheel.Button = 0, 0, tea.MouseButtonWheelUp
	wantCaptured(t, p.Handle(wheel, actions, f))
	if p.SelectedID != "b" {
		t.Fatal("background wheel changed selection")
	}
	wantIntent(t, p.Handle(press(f.Regions[1]), actions, f), ui.Activate, "b")
	wantIntent(t, p.Handle(key(tea.KeyEnter), actions, f), ui.Activate, "b")
	right := press(f.Regions[0])
	right.Button = tea.MouseButtonRight
	wantCaptured(t, p.Handle(right, actions, f))
}

package terminalui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	ui "terminalui"
)

func surfaceFixture() (ui.SurfaceSpec, ui.Rect) {
	bounds := ui.Rect{X: 4, Y: 2, W: 100, H: 9}
	body := ui.SurfaceBodyBounds(bounds)
	return ui.SurfaceSpec{
		ID: "edit", Title: "Edit", CancelID: "cancel", FocusID: "field",
		Actions: []ui.Action{
			{ID: "apply", Label: "Apply", Keys: []string{"ctrl+s", "enter", " "}},
			{ID: "cancel", Label: "Cancel", Keys: []string{"ctrl+x"}},
		},
		FocusOrder: []string{"field", "row", "missing", "cancel", "apply", "dest", "check"},
		Body: ui.Frame{Bounds: body, Lines: []string{"field", "data row [x]", "destination"}, Regions: []ui.Region{
			{ID: "pane", Kind: ui.PaneRegion, Bounds: body},
			{ID: "field", Kind: ui.FieldRegion, Bounds: ui.Rect{X: body.X, Y: body.Y, W: 12, H: 1}},
			{ID: "row", Kind: ui.RowRegion, Bounds: ui.Rect{X: body.X, Y: body.Y + 1, W: 20, H: 1}},
			{ID: "check", Kind: ui.ControlRegion, Bounds: ui.Rect{X: body.X + 9, Y: body.Y + 1, W: 3, H: 1}},
			{ID: "dest", Kind: ui.DestinationRegion, Bounds: ui.Rect{X: body.X, Y: body.Y + 2, W: 12, H: 1}},
		}},
	}, bounds
}

func surfaceRegion(t *testing.T, f ui.Frame, id string) ui.Region {
	t.Helper()
	for _, r := range f.Regions {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("missing region %q: %+v", id, f.Regions)
	return ui.Region{}
}

func TestSurfaceFieldAndButtonOwnership(t *testing.T) {
	spec, bounds := surfaceFixture()
	f := ui.RenderSurface(spec, bounds, ui.Style{})
	for _, id := range []string{"field", "row", "missing"} {
		spec.FocusID = id
		for _, msg := range []tea.Msg{key(tea.KeyEnter), key(tea.KeySpace)} {
			wantCaptured(t, ui.HandleSurface(msg, spec, f))
		}
	}
	for _, id := range []string{"cancel", "apply", "dest", "check"} {
		spec.FocusID = id
		for _, msg := range []tea.Msg{key(tea.KeyEnter), key(tea.KeySpace), press(surfaceRegion(t, f, id))} {
			wantIntent(t, ui.HandleSurface(msg, spec, f), ui.Activate, id)
		}
	}
	wantIntent(t, ui.HandleSurface(press(surfaceRegion(t, f, "field")), spec, f), ui.Focus, "field")
	wantIntent(t, ui.HandleSurface(press(surfaceRegion(t, f, "row")), spec, f), ui.Select, "row")
	spec.FocusID = "field"
	wantIntent(t, ui.HandleSurface(key(tea.KeyCtrlS), spec, f), ui.Activate, "apply")
	for _, action := range []tea.MouseAction{tea.MouseActionRelease, tea.MouseActionMotion} {
		m := press(surfaceRegion(t, f, "apply"))
		m.Action = action
		wantCaptured(t, ui.HandleSurface(m, spec, f))
	}
	for _, msg := range []tea.Msg{text("qj?"), key(tea.KeyF1), tea.MouseMsg{Button: tea.MouseButtonRight}} {
		wantCaptured(t, ui.HandleSurface(msg, spec, f))
	}
	for _, msg := range []tea.Msg{tea.WindowSizeMsg{Width: 5, Height: 3}, struct{ Done bool }{true}} {
		if r := ui.HandleSurface(msg, spec, f); r.Consumed || r.Intent != nil {
			t.Fatalf("lifecycle captured: %+v", r)
		}
	}
}

func TestSurfaceCancellationRoutesAndRefusal(t *testing.T) {
	for _, blocked := range []string{"", "operation running"} {
		spec, bounds := surfaceFixture()
		spec.CancelBlocked, spec.FocusID = blocked, "cancel"
		f := ui.RenderSurface(spec, bounds, ui.Style{})
		outside := tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
		for _, msg := range []tea.Msg{key(tea.KeyEsc), outside, press(surfaceRegion(t, f, "terminalui:surface:close"))} {
			r := ui.HandleSurface(msg, spec, f)
			if blocked == "" {
				wantIntent(t, r, ui.Dismiss, spec.ID)
			} else {
				wantCaptured(t, r)
			}
		}
		for _, msg := range []tea.Msg{key(tea.KeyEnter), key(tea.KeySpace), key(tea.KeyCtrlX), press(surfaceRegion(t, f, "cancel"))} {
			r := ui.HandleSurface(msg, spec, f)
			if blocked == "" {
				wantIntent(t, r, ui.Activate, "cancel")
			} else {
				wantCaptured(t, r)
			}
		}
		outside.Action = tea.MouseActionRelease
		wantCaptured(t, ui.HandleSurface(outside, spec, f))
		if blocked != "" && !strings.Contains(ansi.Strip(f.Lines[0]), "[x] "+blocked) {
			t.Fatal("visible close hides cancellation refusal")
		}
		spec.FocusID = "terminalui:surface:close"
		for _, msg := range []tea.Msg{key(tea.KeyEnter), key(tea.KeySpace)} {
			if blocked == "" {
				wantIntent(t, ui.HandleSurface(msg, spec, f), ui.Dismiss, spec.ID)
			} else {
				wantCaptured(t, ui.HandleSurface(msg, spec, f))
			}
		}
		// A body-owned cancellation control must obey the same gate even when
		// it is not one of the footer Actions.
		spec.Actions = nil
		spec.CancelID, spec.FocusID = "check", "check"
		f = ui.RenderSurface(spec, bounds, ui.Style{})
		for _, msg := range []tea.Msg{key(tea.KeyEnter), key(tea.KeySpace), press(surfaceRegion(t, f, "check"))} {
			if blocked == "" {
				wantIntent(t, ui.HandleSurface(msg, spec, f), ui.Activate, "check")
			} else {
				wantCaptured(t, ui.HandleSurface(msg, spec, f))
			}
		}
	}
	// Cancellation permission changed asynchronously after the last paint.
	spec, bounds := surfaceFixture()
	f := ui.RenderSurface(spec, bounds, ui.Style{})
	spec.CancelBlocked, spec.FocusID = "now running", "cancel"
	for _, msg := range []tea.Msg{key(tea.KeyEsc), key(tea.KeyEnter), key(tea.KeyCtrlX), press(surfaceRegion(t, f, "cancel")), press(surfaceRegion(t, f, "terminalui:surface:close"))} {
		wantCaptured(t, ui.HandleSurface(msg, spec, f))
	}
}

func TestSurfaceCurrentEligibilityAndClipping(t *testing.T) {
	spec, bounds := surfaceFixture()
	old := ui.RenderSurface(spec, bounds, ui.Style{})
	for _, a := range []ui.Action{
		{ID: "apply", Label: "Apply", Keys: []string{"ctrl+s"}, Unavailable: "read only"},
		{ID: "apply", Label: "Apply", Keys: []string{"ctrl+s"}, Busy: true},
		{ID: "replacement", Label: "Apply", Keys: []string{"ctrl+s"}},
	} {
		spec.Actions[0] = a
		spec.FocusID = "apply"
		for _, msg := range []tea.Msg{press(surfaceRegion(t, old, "apply")), key(tea.KeyEnter), key(tea.KeyCtrlS)} {
			wantCaptured(t, ui.HandleSurface(msg, spec, old))
		}
	}
	spec, bounds = surfaceFixture()
	bounds.W = 20 // The complete Cancel shortcut fits; secondary Apply does not.
	f := ui.RenderSurface(spec, bounds, ui.Style{})
	checkFrame(t, f)
	for _, r := range f.Regions {
		if r.ID == "apply" {
			t.Fatal("secondary action survived compact footer")
		}
	}
	surfaceRegion(t, f, "cancel")
	spec.FocusID = "apply"
	wantCaptured(t, ui.HandleSurface(key(tea.KeyCtrlS), spec, f))
	wantCaptured(t, ui.HandleSurface(key(tea.KeyEnter), spec, f))
	bounds.W = 12 // Body checkbox and destinations are now partial.
	f = ui.RenderSurface(spec, bounds, ui.Style{})
	for _, r := range f.Regions {
		if r.ID == "check" || r.ID == "dest" || r.ID == "cancel" {
			t.Fatalf("partial control exposed: %+v", r)
		}
	}
	row := surfaceRegion(t, f, "row")
	if row.Bounds.W != 10 {
		t.Fatalf("row not clipped: %+v", row)
	}
	wantIntent(t, ui.HandleSurface(press(row), spec, f), ui.Select, "row")
	// Even a caller-supplied partially composed button must not activate.
	f = old
	f.Regions = append([]ui.Region(nil), old.Regions...)
	for i := range f.Regions {
		if f.Regions[i].ID == "apply" {
			f.Regions[i].Bounds.W--
		}
	}
	wantCaptured(t, ui.HandleSurface(press(surfaceRegion(t, f, "apply")), spec, f))
}

func TestSurfaceVisibleFocusAndWheelLocality(t *testing.T) {
	spec, bounds := surfaceFixture()
	f := ui.RenderSurface(spec, bounds, ui.Style{})
	for _, pair := range [][2]string{{"field", "cancel"}, {"cancel", "apply"}, {"check", "field"}, {"gone", "field"}} {
		spec.FocusID = pair[0]
		wantIntent(t, ui.HandleSurface(key(tea.KeyTab), spec, f), ui.Focus, pair[1])
	}
	spec.FocusID = "field"
	wantIntent(t, ui.HandleSurface(key(tea.KeyShiftTab), spec, f), ui.Focus, "check")
	bounds.W = 12
	f = ui.RenderSurface(spec, bounds, ui.Style{})
	wantIntent(t, ui.HandleSurface(key(tea.KeyTab), spec, f), ui.Focus, "field")
	spec.FocusOrder = []string{"row", "missing"}
	wantCaptured(t, ui.HandleSurface(key(tea.KeyTab), spec, f))
	for _, backwards := range []bool{false, true} {
		if ui.NextFocus(nil, "x", backwards) != "" || ui.NextFocus([]string{"x"}, "x", backwards) != "x" {
			t.Fatal("empty/single focus traversal")
		}
	}
	if ui.NextFocus([]string{"a", "b"}, "gone", true) != "b" || ui.NextFocus([]string{"a", "b"}, "b", false) != "a" {
		t.Fatal("missing/wrapped focus traversal")
	}
	body := ui.SurfaceBodyBounds(bounds)
	for _, button := range []tea.MouseButton{tea.MouseButtonWheelUp, tea.MouseButtonWheelDown} {
		for _, p := range [][2]int{{body.X, body.Y}, {0, 0}, {bounds.X, bounds.Y + 1}, {body.X, bounds.Y + bounds.H - 2}, {body.X, bounds.Y}} {
			m := tea.MouseMsg{X: p[0], Y: p[1], Button: button, Action: tea.MouseActionPress}
			r := ui.HandleSurface(m, spec, f)
			if p == [2]int{body.X, body.Y} {
				wantIntent(t, r, ui.Scroll, spec.ID)
				want := 1
				if button == tea.MouseButtonWheelUp {
					want = -1
				}
				if r.Intent.Delta != want {
					t.Fatalf("wheel delta: %+v", r.Intent)
				}
			} else {
				wantCaptured(t, r)
			}
		}
	}
}

func TestSurfaceGeometryAndUnicode(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for _, w := range []int{0, 1, 2, 3, 4, 5, 10, 20, 40, 100} {
			for _, h := range []int{0, 1, 2, 3, 4, 9} {
				t.Run(fmt.Sprintf("%t/%d/%d", ascii, w, h), func(t *testing.T) {
					spec, bounds := surfaceFixture()
					bounds.W, bounds.H = w, h
					spec.Title = "界 e\u0301 👩‍💻"
					spec.Body.Lines[0] = "\x1b[31m界 e\u0301 👩‍💻\x1b[0m" + strings.Repeat("body", 40)
					spec.CancelBlocked = "running 界"
					f := ui.RenderSurface(spec, bounds, ui.Style{ASCII: ascii})
					checkFrame(t, f)
					body := ui.SurfaceBodyBounds(bounds)
					if w >= 3 && h >= 3 {
						if body != (ui.Rect{X: bounds.X + 1, Y: bounds.Y + 1, W: w - 2, H: h - 3}) {
							t.Fatalf("body geometry %+v", body)
						}
						for _, r := range f.Regions {
							if r.ID == "cancel" && r.Bounds.Y != bounds.Y+h-2 {
								t.Fatal("footer geometry")
							}
						}
					} else if body.W != 0 || body.H != 0 {
						t.Fatal("compact surface retained body")
					}
					// Caller text may be Unicode in ASCII mode; chrome may not.
					if ascii && w >= 3 && h >= 2 && strings.Trim(ansi.Strip(f.Lines[h-1]), "+-") != "" {
						t.Fatal("non-ASCII border")
					}
				})
			}
		}
	}
}

func TestSurfaceControlArrowsAndPaneFocus(t *testing.T) {
	spec, bounds := surfaceFixture()
	f := ui.RenderSurface(spec, bounds, ui.Style{})
	for _, id := range []string{"field", "row", "pane", "dest", "missing"} {
		spec.FocusID = id
		for _, k := range []tea.KeyType{tea.KeyLeft, tea.KeyRight, tea.KeyUp, tea.KeyDown} {
			wantCaptured(t, ui.HandleSurface(key(k), spec, f))
		}
	}
	spec.FocusID = "cancel"
	for _, k := range []tea.KeyType{tea.KeyLeft, tea.KeyUp} {
		wantIntent(t, ui.HandleSurface(key(k), spec, f), ui.Focus, "field")
	}
	for _, k := range []tea.KeyType{tea.KeyRight, tea.KeyDown} {
		wantIntent(t, ui.HandleSurface(key(k), spec, f), ui.Focus, "apply")
	}
	// Body controls also participate, wrapping through visible FocusOrder.
	spec.FocusID = "check"
	wantIntent(t, ui.HandleSurface(key(tea.KeyRight), spec, f), ui.Focus, "field")
	// A hidden former button must not take arrow ownership after resize.
	bounds.W = 12
	f = ui.RenderSurface(spec, bounds, ui.Style{})
	wantCaptured(t, ui.HandleSurface(key(tea.KeyRight), spec, f))
	// Click empty body space to focus the pane; specific fields/rows/controls
	// still win over the containing pane (covered by ownership tests above).
	spec, bounds = surfaceFixture()
	f = ui.RenderSurface(spec, bounds, ui.Style{})
	pane := surfaceRegion(t, f, "pane")
	pane.Bounds.Y += 4
	wantIntent(t, ui.HandleSurface(press(pane), spec, f), ui.Focus, "pane")
}

func TestSurfacePreviewPaneCloseTraversal(t *testing.T) {
	bounds := ui.Rect{X: 3, Y: 2, W: 50, H: 8}
	body := ui.SurfaceBodyBounds(bounds)
	spec := ui.SurfaceSpec{
		ID: "preview", Title: "Preview", CancelID: "close", FocusID: "reader",
		FocusOrder: []string{"reader", "close"},
		Actions:    []ui.Action{{ID: "close", Label: "Close"}},
		Body: ui.Frame{Bounds: body, Lines: []string{"preview content"}, Regions: []ui.Region{
			{ID: "reader", Kind: ui.PaneRegion, Bounds: body},
		}},
	}
	for _, k := range []tea.KeyType{tea.KeyTab, tea.KeyShiftTab} {
		spec.FocusID = "reader"
		f := ui.RenderSurface(spec, bounds, ui.Style{})
		wantIntent(t, ui.HandleSurface(key(k), spec, f), ui.Focus, "close")
		spec.FocusID = "close"
		f = ui.RenderSurface(spec, bounds, ui.Style{})
		wantIntent(t, ui.HandleSurface(key(k), spec, f), ui.Focus, "reader")
	}
	spec.FocusID = "reader"
	f := ui.RenderSurface(spec, bounds, ui.Style{})
	for _, k := range []tea.KeyType{tea.KeyEnter, tea.KeySpace, tea.KeyUp, tea.KeyDown} {
		wantCaptured(t, ui.HandleSurface(key(k), spec, f))
	}
	// A pane clipped away by resize is not a focus stop.
	bounds.H = 3
	spec.FocusID = "close"
	f = ui.RenderSurface(spec, bounds, ui.Style{})
	wantIntent(t, ui.HandleSurface(key(tea.KeyTab), spec, f), ui.Focus, "close")
}

func TestSurfaceBodyCannotCoverChrome(t *testing.T) {
	bounds := ui.Rect{X: 10, Y: 5, W: 30, H: 6}
	// Body deliberately extends through all chrome and outside the surface.
	bodyBounds := ui.Rect{X: 8, Y: 3, W: 36, H: 12}
	spec := ui.SurfaceSpec{ID: "scope", Title: "Title", CancelID: "cancel", Actions: []ui.Action{{ID: "cancel", Label: "Cancel"}}, Body: ui.Frame{Bounds: bodyBounds}}
	for range bodyBounds.H {
		spec.Body.Lines = append(spec.Body.Lines, strings.Repeat("z", bodyBounds.W))
	}
	spec.Body.Regions = []ui.Region{
		{ID: "field", Kind: ui.FieldRegion, Bounds: bodyBounds},
		{ID: "partial", Kind: ui.ControlRegion, Bounds: bodyBounds},
	}
	f := ui.RenderSurface(spec, bounds, ui.Style{ASCII: true})
	checkFrame(t, f)
	if surfaceRegion(t, f, "field").Bounds != ui.SurfaceBodyBounds(bounds) {
		t.Fatal("body field escaped its reserved bounds")
	}
	for _, r := range f.Regions {
		if r.ID == "partial" {
			t.Fatal("partially clipped control retained hitbox")
		}
	}
	if !strings.Contains(ansi.Strip(f.Lines[0]), "Title") || !strings.Contains(ansi.Strip(f.Lines[4]), "Cancel") || ansi.Strip(f.Lines[5]) != "+"+strings.Repeat("-", 28)+"+" {
		t.Fatal("body overwrote shared chrome")
	}
	for _, y := range []int{1, 2, 3} {
		if ansi.Strip(f.Lines[y]) != "|"+strings.Repeat("z", 28)+"|" {
			t.Fatalf("body cell clipping failed: %q", f.Lines[y])
		}
	}
	bounds.H = 2
	f = ui.RenderSurface(spec, bounds, ui.Style{})
	for _, r := range f.Regions {
		if r.ID != "terminalui:surface:close" {
			t.Fatalf("hidden compact region: %+v", r)
		}
	}
}

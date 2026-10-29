package terminalui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// SurfaceSpec describes one current capture scope. IDs must be unique; the
// terminalui:surface:close ID is reserved. Body lines are local to Body.Bounds,
// while all regions are absolute. Supply safe styled content, not raw provider
// text. Recompute both spec and frame after resize/async changes and recheck
// domain permissions when executing any returned intent.
//
// Body FieldRegion presses request Focus; Enter/Space remain app-local grammar.
// RowRegion presses request Select and rows are never Tab stops or keyboard
// activation targets. DestinationRegion and ControlRegion presses or focused
// Enter/Space request Activate. If their ID appears in Actions, its current
// availability is enforced; otherwise they are explicit app-owned controls whose
// permissions the app must check. PaneRegion presses request Focus.
// Partially clipped fields/rows retain their visible area; partially clipped
// controls/destinations have no hit target. Body cannot overlap shared chrome.
// At overlapping body targets, controls/destinations win over fields, which win
// over rows, which win over panes; equal-priority targets use caller order.
type SurfaceSpec struct {
	ID, Title, CancelID, CancelBlocked string
	Actions                            []Action
	FocusID                            string
	FocusOrder                         []string
	Body                               Frame
}

const surfaceCloseID = "terminalui:surface:close"

// SurfaceBodyBounds reserves title/top border, a one-row footer and bottom
// border. For W>=3,H>=3 it is {X+1,Y+1,W-2,H-3}; smaller surfaces have no body.
// The footer is {X+1,Y+H-2,W-2,1}, only when W>=3,H>=3.
func SurfaceBodyBounds(bounds Rect) Rect {
	r := Rect{X: bounds.X + 1, Y: bounds.Y + 1}
	if bounds.W >= 3 && bounds.H >= 3 {
		r.W, r.H = bounds.W-2, bounds.H-3
	}
	return r
}

func surfaceActions(spec SurfaceSpec) []Action {
	actions := make([]Action, 0, len(spec.Actions))
	for _, a := range spec.Actions {
		if a.ID == spec.CancelID {
			if spec.CancelBlocked != "" {
				a.Unavailable = spec.CancelBlocked
			}
			actions = append(actions, a)
		}
	}
	for _, a := range spec.Actions {
		if a.ID != spec.CancelID {
			actions = append(actions, a)
		}
	}
	return actions
}

func surfaceCloseLabel(spec SurfaceSpec) string {
	label := "[x]"
	if spec.CancelBlocked != "" {
		label += " " + spec.CancelBlocked
	}
	return label
}

// The same current geometry drives painting and stale-frame input checks.
// No body text or ANSI styling is rendered to discover hit targets.
func surfaceRegions(spec SurfaceSpec, bounds Rect) []Region {
	if bounds.W < 3 || bounds.H <= 0 {
		return nil
	}
	var regions []Region
	inner := bounds.W - 2
	if inner >= 3 {
		closeWidth := min(inner, ansi.StringWidth(surfaceCloseLabel(spec)))
		regions = append(regions, Region{surfaceCloseID, ControlRegion, Rect{bounds.X + bounds.W - 1 - closeWidth, bounds.Y, 3, 1}})
	}
	visible := intersection(SurfaceBodyBounds(bounds), spec.Body.Bounds)
	for _, r := range spec.Body.Regions {
		clipped := intersection(r.Bounds, visible)
		if r.ID == "" || r.ID == surfaceCloseID || clipped.W == 0 || clipped.H == 0 {
			continue
		}
		if (r.Kind == ControlRegion || r.Kind == DestinationRegion) && clipped != r.Bounds {
			continue
		}
		r.Bounds = clipped
		regions = append(regions, r)
	}
	if bounds.H >= 3 {
		for _, control := range actionBarControls(surfaceActions(spec), Rect{bounds.X + 1, bounds.Y + bounds.H - 2, inner, 1}, spec.FocusID) {
			regions = append(regions, Region{control.action.ID, ControlRegion, control.bounds})
		}
	}
	return regions
}

// RenderSurface owns title, close, footer and border. Cancel is laid out before
// secondary actions; RenderActionBar omits whole controls that cannot fit.
// CancelBlocked is also shown beside the always-visible close when space allows,
// so a long unavailable footer label cannot hide every cancellation refusal.
// Body padding uses Background; chrome and action bars use Surface. Explicit
// body styles (including selected-row backgrounds) are retained.
// Use Overlay for floating surfaces; pass the original surface frame to input.
func RenderSurface(spec SurfaceSpec, bounds Rect, style Style) Frame {
	f := blank(bounds, style)
	w, h := f.Bounds.W, f.Bounds.H
	if w == 0 || h == 0 {
		return f
	}
	vertical, horizontal, tl, tr, bl, br := "│", "─", "┌", "┐", "└", "┘"
	if style.ASCII {
		vertical, horizontal, tl, tr, bl, br = "|", "-", "+", "+", "+", "+"
	}
	if w < 3 {
		f.Lines[0] = paint(Fit(spec.Title, w), style.Border, style.Surface)
		return f
	}
	inner := w - 2
	title := Fit(" "+spec.Title, inner)
	if inner >= 3 {
		close := surfaceCloseLabel(spec)
		closeWidth := min(inner, ansi.StringWidth(close))
		title = Fit(" "+spec.Title, inner-closeWidth) + Fit(close, closeWidth)
	}
	f.Lines[0] = paint(tl+title+tr, style.Border, style.Surface)
	for y := 1; y < h-1; y++ {
		f.Lines[y] = paint(vertical, style.Border, style.Surface) + paint(strings.Repeat(" ", inner), style.Foreground, style.Background) + paint(vertical, style.Border, style.Surface)
	}
	if h >= 2 {
		f.Lines[h-1] = paint(bl+strings.Repeat(horizontal, inner)+br, style.Border, style.Surface)
	}
	body := SurfaceBodyBounds(f.Bounds)
	visible := intersection(body, spec.Body.Bounds)
	for y := visible.Y; y < visible.Y+visible.H; y++ {
		i := y - spec.Body.Bounds.Y
		if i < 0 || i >= len(spec.Body.Lines) {
			continue
		}
		line := ansi.Cut(spec.Body.Lines[i], visible.X-spec.Body.Bounds.X, visible.X-spec.Body.Bounds.X+visible.W)
		text := paint(strings.Repeat(" ", visible.X-body.X), style.Foreground, style.Background) + paint(line, style.Foreground, style.Background)
		text += paint(strings.Repeat(" ", max(0, inner-ansi.StringWidth(text))), style.Foreground, style.Background)
		f.Lines[y-f.Bounds.Y] = paint(vertical, style.Border, style.Surface) + text + paint(vertical, style.Border, style.Surface)
	}
	if h >= 3 {
		bar := RenderActionBar(surfaceActions(spec), Rect{f.Bounds.X + 1, f.Bounds.Y + h - 2, inner, 1}, spec.FocusID, style)
		f.Lines[h-2] = paint(vertical, style.Border, style.Surface) + bar.Lines[0] + paint(vertical, style.Border, style.Surface)
	}
	f.Regions = surfaceRegions(spec, f.Bounds)
	return f
}

// HandleSurface captures all key/mouse input, never lifecycle messages. Local
// field grammar must run first where appropriate (notably INSERT Esc). Escape,
// outside left press and close request Dismiss with spec.ID; footer Cancel is
// Activate with CancelID. CancelBlocked refuses all these routes, including any
// matching body control or action shortcut. Focus traversal includes only current
// visible fields/panes/controls/destinations in FocusOrder, not rows or hidden
// actions. Panes never activate on Enter/Space.
// Left/Up traverse backwards and Right/Down forwards only when FocusID is a
// rendered ControlRegion; body arrows otherwise remain app-owned grammar.
// Route local text/Enter and any body scrolling (such as rename Up/Down) first.
// Wheel inside the body requests Scroll with spec.ID and Delta +/-1; elsewhere
// it is captured without a request. Release/motion never activate anything.
func HandleSurface(msg tea.Msg, spec SurfaceSpec, frame Frame) Result {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
	default:
		return Result{}
	}
	captured := Result{Consumed: true}
	// Recheck current eligibility/geometry, and require an intact region in the
	// supplied frame too. This rejects stale IDs and partially composed controls.
	var regions []Region
	for _, r := range surfaceRegions(spec, frame.Bounds) {
		for _, old := range frame.Regions {
			if r == old {
				regions = append(regions, r)
				break
			}
		}
	}
	dismiss := func() Result {
		if spec.CancelBlocked != "" {
			return captured
		}
		return request(Dismiss, spec.ID)
	}
	activate := func(id string) Result {
		if id == spec.CancelID && spec.CancelBlocked != "" {
			return captured
		}
		for _, a := range spec.Actions {
			if a.ID == id && !allowed(a) {
				return captured
			}
		}
		return request(Activate, id)
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		k := m.String()
		switch k {
		case "esc":
			return dismiss()
		case "tab", "shift+tab", "left", "right", "up", "down":
			if k != "tab" && k != "shift+tab" {
				controlFocused := false
				for _, r := range regions {
					if r.ID == spec.FocusID && r.Kind == ControlRegion {
						controlFocused = true
						break
					}
				}
				if !controlFocused {
					return captured
				}
			}
			var visible []string
			for _, id := range spec.FocusOrder {
				for _, r := range regions {
					if r.ID == id && (r.Kind == FieldRegion || r.Kind == PaneRegion || r.Kind == ControlRegion || r.Kind == DestinationRegion) {
						visible = append(visible, id)
						break
					}
				}
			}
			if id := NextFocus(visible, spec.FocusID, k == "shift+tab" || k == "left" || k == "up"); id != "" {
				return request(Focus, id)
			}
			return captured
		}
		// A default action or even an Enter/Space shortcut cannot steal field
		// grammar. Explicit non-text action shortcuts remain available.
		if k == "enter" || k == " " {
			for _, r := range regions {
				if r.ID == spec.FocusID {
					if r.Kind == FieldRegion || r.Kind == RowRegion {
						return captured
					}
					if r.ID == surfaceCloseID {
						return dismiss()
					}
					if r.Kind == ControlRegion || r.Kind == DestinationRegion {
						return activate(r.ID)
					}
				}
			}
			return captured
		}
		for _, a := range spec.Actions {
			for _, shortcut := range a.Keys {
				if k != shortcut {
					continue
				}
				for _, r := range regions {
					if r.ID == a.ID && r.Kind == ControlRegion {
						return activate(a.ID)
					}
				}
			}
		}
	case tea.MouseMsg:
		if m.Action != tea.MouseActionPress {
			return captured
		}
		if m.Button == tea.MouseButtonWheelUp || m.Button == tea.MouseButtonWheelDown {
			if contains(SurfaceBodyBounds(frame.Bounds), m.X, m.Y) {
				delta := 1
				if m.Button == tea.MouseButtonWheelUp {
					delta = -1
				}
				return Result{Consumed: true, Intent: &Intent{Kind: Scroll, ID: spec.ID, Delta: delta}}
			}
			return captured
		}
		if m.Button != tea.MouseButtonLeft {
			return captured
		}
		if !contains(frame.Bounds, m.X, m.Y) {
			return dismiss()
		}
		for _, kind := range []RegionKind{ControlRegion, DestinationRegion, FieldRegion, RowRegion, PaneRegion} {
			for _, r := range regions {
				if r.Kind != kind || !contains(r.Bounds, m.X, m.Y) {
					continue
				}
				if r.ID == surfaceCloseID {
					return dismiss()
				}
				switch r.Kind {
				case ControlRegion, DestinationRegion:
					return activate(r.ID)
				case FieldRegion, PaneRegion:
					return request(Focus, r.ID)
				case RowRegion:
					return request(Select, r.ID)
				}
			}
		}
	}
	return captured
}

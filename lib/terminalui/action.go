package terminalui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type IntentKind uint8

const (
	Activate IntentKind = iota + 1
	Select
	Focus
	Dismiss
	Help
	Scroll
)

type Intent struct {
	Kind  IntentKind
	ID    string
	Delta int
}

type Result struct {
	Consumed bool
	Intent   *Intent
}

// Action IDs must be unique in the current authorized scope. Keys are
// single-event tea.KeyMsg.String values, not multi-key reader grammars.
type Action struct {
	ID, Label, Keywords, Unavailable string
	Keys                             []string
	Busy, Dangerous                  bool
}

func allowed(a Action) bool { return a.ID != "" && a.Unavailable == "" && !a.Busy }

func request(kind IntentKind, id string) Result {
	return Result{Consumed: true, Intent: &Intent{Kind: kind, ID: id}}
}

func actionLabel(a Action) string {
	s := a.Label
	if a.Dangerous {
		s = "! " + s
	}
	if a.Busy {
		s = "[busy] " + s
	}
	if a.Unavailable != "" {
		s = "[unavailable] " + s
	}
	return s
}

// Layout is independent of theme/painting so input can recheck current controls
// without rendering their text. A focus scope reserves one marker cell.
type actionControl struct {
	action Action
	text   string
	bounds Rect
}

func actionBarControls(actions []Action, bounds Rect, focusID string) []actionControl {
	if bounds.H <= 0 {
		return nil
	}
	var controls []actionControl
	x, markerWidth := 0, 0
	if focusID != "" {
		markerWidth = 1
	}
	for _, a := range actions {
		text := "[ " + actionLabel(a)
		if len(a.Keys) > 0 {
			text += " " + strings.Join(a.Keys, "/")
		}
		if a.Unavailable != "" {
			text += " - " + a.Unavailable
		}
		text += " ]"
		w, gap := ansi.StringWidth(text), 0
		if x > 0 {
			gap = 1
		}
		if x+gap+markerWidth+w > bounds.W {
			break
		}
		x += gap + markerWidth
		controls = append(controls, actionControl{a, text, Rect{bounds.X + x, bounds.Y, w, 1}})
		x += w
	}
	return controls
}

// RenderActionBar renders complete compact controls only. Controls that do not
// fit have neither pixels nor hit regions (including keyboard activation).
// A nonempty focusID reserves one marker cell before every button (even if the
// focused ID is not an action): selected uses ›, or > in ASCII mode. Moving
// focus never changes layout. Marker cells are outside the button hit regions.
// An empty focusID preserves the unscoped toolbar layout without marker cells.
func RenderActionBar(actions []Action, bounds Rect, focusID string, style Style) Frame {
	f := blank(bounds, style)
	if f.Bounds.H == 0 {
		return f
	}
	var line string
	for i, control := range actionBarControls(actions, f.Bounds, focusID) {
		a := control.action
		marker := ""
		if focusID != "" {
			marker = " "
			if a.ID == focusID {
				marker = "›"
				if style.ASCII {
					marker = ">"
				}
			}
		}
		if i > 0 {
			line += " "
		}
		line += paint(marker+control.text, actionColor(a, a.ID == focusID, style), style.Surface)
		f.Regions = append(f.Regions, Region{a.ID, ControlRegion, control.bounds})
	}
	f.Lines[0] = Fit(line, f.Bounds.W)
	return f
}

// HandleActionBar must be routed only after the owner wins input. Pass a frame
// recomputed for current dimensions/actions, never a cached pre-resize frame.
func HandleActionBar(msg tea.Msg, actions []Action, frame Frame, focusID string) Result {
	for _, a := range actions {
		for _, r := range frame.Regions {
			if r.ID != a.ID || r.Kind != ControlRegion || r.Bounds.W <= 0 || r.Bounds.H <= 0 || intersection(r.Bounds, frame.Bounds) != r.Bounds {
				continue
			}
			hit := false
			switch m := msg.(type) {
			case tea.MouseMsg:
				hit = m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft && contains(r.Bounds, m.X, m.Y)
			case tea.KeyMsg:
				key := m.String()
				hit = a.ID == focusID && (key == "enter" || key == " ")
				for _, k := range a.Keys {
					hit = hit || key == k
				}
			}
			if hit {
				if allowed(a) {
					return request(Activate, a.ID)
				}
				return Result{Consumed: true}
			}
		}
	}
	return Result{}
}

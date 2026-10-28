package terminalui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// HelpTopic documents app-owned local grammar, including multi-key reader and
// editor commands that must not be registered as Action.Keys.
type HelpTopic struct{ Group, Keys, Description string }

const helpCloseID = "terminalui:help:close"

type helpRow struct {
	text    string
	heading bool
	warning bool
}

// Both rendering and input use the same width-dependent wrapped content rows.
func helpRows(actions []Action, topics []HelpTopic, width int) []helpRow {
	var rows []helpRow
	var groups []string
	groupTopics := make(map[string][]HelpTopic)
	for _, topic := range topics {
		group := topic.Group
		if group == "" {
			group = "Local commands"
		}
		if _, exists := groupTopics[group]; !exists {
			groups = append(groups, group)
		}
		groupTopics[group] = append(groupTopics[group], topic)
	}
	for _, group := range groups {
		rows = append(rows, helpRow{text: group, heading: true})
		for _, topic := range groupTopics[group] {
			rows = append(rows, helpRow{text: topic.Keys + " - " + topic.Description})
		}
	}
	if len(actions) > 0 {
		rows = append(rows, helpRow{text: "Actions", heading: true})
		for _, a := range actions {
			keys := strings.Join(a.Keys, "/")
			if keys == "" {
				keys = "(no shortcut)"
			}
			text := keys + " - " + actionLabel(a)
			if allowed(a) {
				text += " - available"
			}
			rows = append(rows, helpRow{text: text, warning: !allowed(a)})
			if a.Unavailable != "" {
				rows = append(rows, helpRow{text: "Reason: " + a.Unavailable, warning: true})
			}
		}
	}
	if len(rows) == 0 {
		rows = append(rows, helpRow{text: "No commands available"})
	}
	var wrapped []helpRow
	for _, row := range rows {
		for _, line := range strings.Split(ansi.Wrap(row.text, max(1, width), ""), "\n") {
			wrapped = append(wrapped, helpRow{text: line, heading: row.heading, warning: row.warning})
		}
	}
	return wrapped
}

func helpVisibleRows(bounds Rect) int {
	if bounds.W < 3 || bounds.H < 4 {
		return 0
	}
	return bounds.H - 3
}

func helpOffset(offset, count, visible int) int {
	if visible == 0 {
		return 0
	}
	return max(0, min(offset, max(0, count-visible)))
}

// RenderHelp is view-only: actions supply documentation and availability, never
// action hit targets. Apps retain the parent state and the help-open flag/offset.
// Content is safe single-line terminal text, as with the other renderers.
func RenderHelp(actions []Action, topics []HelpTopic, offset int, bounds Rect, style Style) Frame {
	f := blank(bounds, style)
	w, h := f.Bounds.W, f.Bounds.H
	if w == 0 || h == 0 {
		return f
	}
	visible := helpVisibleRows(f.Bounds)
	if visible == 0 {
		f.Lines[0] = paint(Fit("HELP F1/Esc close", w), style.Focus, style.Surface)
		return f
	}
	vertical, horizontal, tl, tr, bl, br := "│", "─", "┌", "┐", "└", "┘"
	if style.ASCII {
		vertical, horizontal, tl, tr, bl, br = "|", "-", "+", "+", "+", "+"
	}
	inner := w - 2
	title := Fit(" HELP ", inner)
	if inner >= 8 {
		title = Fit(" HELP ", inner-3) + "[x]"
		f.Regions = append(f.Regions, Region{helpCloseID, ControlRegion, Rect{f.Bounds.X + w - 4, f.Bounds.Y, 3, 1}})
	}
	f.Lines[0] = paint(tl+title+tr, style.Border, style.Surface)
	put := func(y int, text, fg string) {
		f.Lines[y] = paint(vertical, style.Border, style.Surface) + paint(Fit(text, inner), fg, style.Surface) + paint(vertical, style.Border, style.Surface)
	}
	rows := helpRows(actions, topics, inner)
	offset = helpOffset(offset, len(rows), visible)
	for y := 0; y < visible; y++ {
		text, fg := "", style.Foreground
		if i := offset + y; i < len(rows) {
			row := rows[i]
			text = row.text
			if row.heading {
				fg = style.Focus
			} else if row.warning {
				fg = style.Warning
			}
		}
		put(y+1, text, fg)
	}
	put(h-2, fmt.Sprintf("F1/Esc close  Up/Down/PgUp/PgDn scroll  %d-%d/%d", offset+1, min(offset+visible, len(rows)), len(rows)), style.Muted)
	f.Lines[h-1] = paint(bl+strings.Repeat(horizontal, inner)+br, style.Border, style.Surface)
	return f
}

// HandleHelp captures every key/mouse message without effects or workspace
// intents. Route lifecycle/expiry updates before this handler and pass a current
// help frame (not its composed Overlay). Only Dismiss is emitted, closing Help
// rather than its parent. Non-input messages are left to the owner.
func HandleHelp(msg tea.Msg, actions []Action, topics []HelpTopic, offset *int, frame Frame) Result {
	visible := helpVisibleRows(frame.Bounds)
	count := len(helpRows(actions, topics, frame.Bounds.W-2))
	if offset != nil {
		*offset = helpOffset(*offset, count, visible)
	}
	delta := 0
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "f1", "esc":
			return request(Dismiss, "")
		case "up":
			delta = -1
		case "down":
			delta = 1
		case "pgup":
			delta = -visible
		case "pgdown":
			delta = visible
		}
	case tea.MouseMsg:
		if m.Action == tea.MouseActionPress && contains(frame.Bounds, m.X, m.Y) {
			if m.Button == tea.MouseButtonLeft {
				for _, r := range frame.Regions {
					if r.Kind == ControlRegion && r.ID == helpCloseID && intersection(r.Bounds, frame.Bounds) == r.Bounds && contains(r.Bounds, m.X, m.Y) {
						return request(Dismiss, "")
					}
				}
			} else if m.Button == tea.MouseButtonWheelUp {
				delta = -1
			} else if m.Button == tea.MouseButtonWheelDown {
				delta = 1
			}
		}
	default:
		return Result{}
	}
	if offset != nil {
		*offset = helpOffset(*offset+delta, count, visible)
	}
	return Result{Consumed: true}
}

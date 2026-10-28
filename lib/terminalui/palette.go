package terminalui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// Palette owns only query/navigation state. Its owner controls open/closed
// state, return focus, current authorization, and dispatch of returned intents.
type Palette struct {
	Query, SelectedID string
	Offset            int
}

const paletteCloseID = "terminalui:palette:close"
const paletteHelpID = "terminalui:palette:help"

func matching(actions []Action, query string) []Action {
	if query == "" {
		return actions
	}
	labels, keywords := make([]string, len(actions)), make([]string, len(actions))
	for i, a := range actions {
		labels[i], keywords[i] = a.Label, a.Keywords
	}
	matches := fuzzy.Find(query, labels)
	if len(matches) == 0 {
		matches = fuzzy.Find(query, keywords)
	}
	result := make([]Action, 0, len(matches))
	for _, m := range matches {
		result = append(result, actions[m.Index])
	}
	return result
}

func paletteRows(bounds Rect) int {
	if bounds.W < 3 {
		return 0
	}
	return max(0, bounds.H-6)
}

func (p *Palette) refresh(actions []Action, rows int) int {
	index := 0
	for i, a := range actions {
		if a.ID == p.SelectedID {
			index = i
			break
		}
	}
	p.SelectedID = ""
	if len(actions) > 0 {
		p.SelectedID = actions[index].ID
	}
	p.Offset = max(0, min(p.Offset, max(0, len(actions)-rows)))
	if rows > 0 {
		if index < p.Offset {
			p.Offset = index
		} else if index >= p.Offset+rows {
			p.Offset = index - rows + 1
		}
	}
	return index
}

// View uses local exact-size lines and absolute row regions. Selection is
// resolved by current ID even when an asynchronous refresh reorders results.
func (p Palette) View(actions []Action, bounds Rect, style Style) Frame {
	f := blank(bounds, style)
	w, h := f.Bounds.W, f.Bounds.H
	if w == 0 || h == 0 {
		return f
	}
	vertical, horizontal, tl, tr, bl, br, dot, marker := "│", "─", "┌", "┐", "└", "┘", "·", "›"
	if style.ASCII {
		vertical, horizontal, tl, tr, bl, br, dot, marker = "|", "-", "+", "+", "+", "+", ".", ">"
	}
	if w < 3 || h < 6 {
		f.Lines[0] = paint(Fit("ACTIONS "+dot+" Ctrl+P", w), style.Focus, style.Surface)
		return f
	}
	inner := w - 2
	title := Fit(" ACTIONS "+dot+" Ctrl+P ", inner)
	if inner >= 21 {
		title = Fit(" ACTIONS "+dot+" Ctrl+P ", inner-13) + "[Help F1] [x]"
	} else if inner >= 3 {
		title = Fit(" ACTIONS "+dot+" Ctrl+P ", inner-3) + "[x]"
	}
	f.Lines[0] = paint(tl+title+tr, style.Border, style.Surface)
	put := func(y int, text, fg, bg string) {
		f.Lines[y] = paint(vertical, style.Border, style.Surface) + paint(Fit(text, inner), fg, bg) + paint(vertical, style.Border, style.Surface)
	}
	put(1, "Query: "+p.Query, style.Foreground, style.Surface)
	// Preserve recognizable action names at the standard width; the selected
	// reason has its own full-width line, so it need not dominate this column.
	labelWidth := max(0, inner-3-min(24, inner/3))
	put(2, Fit("  Action", labelWidth)+" | Keys / reason", style.Muted, style.Surface)
	matches := matching(actions, p.Query)
	rows := paletteRows(f.Bounds)
	selected := p.refresh(matches, rows)
	for row := 0; row < rows; row++ {
		i := p.Offset + row
		if i >= len(matches) {
			text := ""
			if row == 0 && len(matches) == 0 {
				text = "No results"
			}
			put(3+row, text, style.Muted, style.Surface)
			continue
		}
		a := matches[i]
		prefix := "  "
		fg, bg := actionColor(a, false, style), style.Surface
		if a.ID == p.SelectedID {
			prefix = marker + " "
			fg, bg = style.SelectionText, style.Selection
		}
		text := Fit(prefix+actionLabel(a), labelWidth) + " | " + strings.Join(a.Keys, "/")
		if a.Unavailable != "" {
			text += " - " + a.Unavailable
		}
		put(3+row, text, fg, bg)
		f.Regions = append(f.Regions, Region{a.ID, RowRegion, Rect{f.Bounds.X + 1, f.Bounds.Y + 3 + row, inner, 1}})
	}
	detail, detailColor := "", style.Muted
	if len(matches) > 0 {
		a := matches[selected]
		if a.Unavailable != "" {
			detail, detailColor = a.Unavailable, style.Warning
		} else if a.Busy {
			detail = "Busy - please wait"
		}
	}
	put(h-3, detail, detailColor, style.Surface)
	put(h-2, "Enter run  Tab/arrows select  Esc close  F1 Help", style.Muted, style.Surface)
	f.Lines[h-1] = paint(bl+strings.Repeat(horizontal, inner)+br, style.Border, style.Surface)
	if inner >= 21 {
		f.Regions = append(f.Regions, Region{paletteHelpID, ControlRegion, Rect{f.Bounds.X + w - 14, f.Bounds.Y, 9, 1}})
	}
	if inner >= 3 {
		f.Regions = append(f.Regions, Region{paletteCloseID, ControlRegion, Rect{f.Bounds.X + w - 4, f.Bounds.Y, 3, 1}})
	}
	return f
}

// Handle captures all messages while open. Lifecycle processing belongs before
// this handler. Pointer IDs are intersected with current matches/eligibility;
// stale row indices can never target a replacement action.
func (p *Palette) Handle(msg tea.Msg, actions []Action, frame Frame) Result {
	matches := matching(actions, p.Query)
	rows := paletteRows(frame.Bounds)
	index := p.refresh(matches, rows)
	move := func(delta int) {
		if len(matches) > 0 {
			index = max(0, min(index+delta, len(matches)-1))
			p.SelectedID = matches[index].ID
			p.refresh(matches, rows)
		}
	}
	edit := func(query string) {
		p.Query, p.SelectedID, p.Offset = query, "", 0
		p.refresh(matching(actions, query), rows)
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "esc", "ctrl+p":
			return request(Dismiss, "")
		case "f1":
			return request(Help, "")
		case "up", "shift+tab":
			move(-1)
		case "down", "tab":
			move(1)
		case "enter":
			if len(matches) > 0 && rows > 0 && allowed(matches[index]) {
				return request(Activate, matches[index].ID)
			}
		case "backspace", "ctrl+h":
			runes := []rune(p.Query)
			if len(runes) > 0 {
				edit(string(runes[:len(runes)-1]))
			}
		case "ctrl+u":
			edit("")
		default:
			if !m.Alt && (m.Type == tea.KeyRunes || m.Type == tea.KeySpace) {
				runes := []rune(p.Query)
				input := m.Runes
				if m.Type == tea.KeySpace {
					input = []rune{' '}
				}
				for _, r := range input {
					if len(runes) < 128 && !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r) && unicode.IsPrint(r) {
						runes = append(runes, r)
					}
				}
				if string(runes) != p.Query {
					edit(string(runes))
				}
			}
		}
	case tea.MouseMsg:
		inside := contains(frame.Bounds, m.X, m.Y)
		if m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft {
			if !inside {
				return request(Dismiss, "")
			}
			for _, r := range frame.Regions {
				if !contains(intersection(r.Bounds, frame.Bounds), m.X, m.Y) {
					continue
				}
				if r.Kind == ControlRegion && r.ID == paletteCloseID {
					return request(Dismiss, "")
				}
				if r.Kind == ControlRegion && r.ID == paletteHelpID {
					return request(Help, "")
				}
				if r.Kind != RowRegion {
					continue
				}
				for _, a := range matches {
					if a.ID == r.ID {
						p.SelectedID = a.ID
						p.refresh(matches, rows)
						if allowed(a) {
							return request(Activate, a.ID)
						}
						return Result{Consumed: true}
					}
				}
			}
		}
		if inside && m.Action == tea.MouseActionPress {
			switch m.Button {
			case tea.MouseButtonWheelUp:
				move(-1)
			case tea.MouseButtonWheelDown:
				move(1)
			}
		}
	}
	return Result{Consumed: true}
}

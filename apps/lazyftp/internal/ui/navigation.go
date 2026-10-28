package ui

import (
	"fmt"
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
	"terminalui"
)

var paneIDs = []string{"pane:nav", "pane:left", "pane:right", "pane:inspector", "pane:jobs"}

var browseControls = []struct{ label, action string }{{"↑ Up", "parent"}, {"/ Filter", "filter"}, {"↻ Refresh", "refresh"}, {"Open/Inspect", "open"}}

func paneControlText(w int) string {
	result := ""
	for _, c := range browseControls {
		prefix := strings.Split(paneControls, c.label)[0] + c.label
		if ansi.StringWidth(prefix) <= w-2 {
			result = prefix
		}
	}
	return result
}

func (m Model) highlightNavigation(lines []string) []string {
	f := m.interactionFrame()
	if m.rename != nil && !m.rename.confirming {
		f = m.inlineRenameFrame()
	}
	for _, r := range f.Regions {
		if r.ID != m.controlFocus || r.Kind == terminalui.PaneRegion {
			continue
		}
		b := r.Bounds
		if b.Y < 0 || b.Y >= len(lines) || b.X < 0 || b.X+b.W > m.width {
			continue
		}
		line := lines[b.Y]
		label := ansi.Strip(ansi.Cut(line, b.X, b.X+b.W))
		label = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(label)
		lines[b.Y] = ansi.Cut(line, 0, b.X) + label + ansi.Cut(line, b.X+b.W, m.width)
	}
	return lines
}

func locationID(loc storage.Location) string { return fmt.Sprintf("%q:%q", loc.Remote, loc.Path) }
func (m Model) rowID(i int, e storage.Entry) string {
	return fmt.Sprintf("entry:%s:%s:%q", paneIDs[i+1], locationID(m.panes[i].location), e.Name)
}
func (m Model) paneControl(i int, action string) string {
	return "control:" + paneIDs[i+1] + ":" + locationID(m.panes[i].location) + ":" + action
}

// Derive regions from the same layout, row window and labels used by View.
// Fully clipped controls are omitted rather than retaining invisible hot spots.
func (m Model) interactionFrame() terminalui.Frame {
	f := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}}
	if !m.workspaceFits() {
		return f
	}
	l := m.layout()
	add := func(id string, kind terminalui.RegionKind, b terminalui.Rect) {
		if b.W > 0 && b.H > 0 {
			f.Regions = append(f.Regions, terminalui.Region{ID: id, Kind: kind, Bounds: b})
		}
	}
	if l.nav > 0 {
		add(paneIDs[0], terminalui.PaneRegion, terminalui.Rect{Y: 1, W: l.nav, H: l.height})
	}
	h := l.height - l.drawer
	for i := range m.panes {
		x, w := l.nav, l.left
		if i == 1 {
			x, w = l.nav+l.left, l.center-l.left
		}
		add(paneIDs[i+1], terminalui.PaneRegion, terminalui.Rect{X: x, Y: 1, W: w, H: h})
	}
	if l.inspector > 0 {
		add(paneIDs[3], terminalui.PaneRegion, terminalui.Rect{X: l.nav + l.center, Y: 1, W: l.inspector, H: l.height})
	}
	if l.drawer > 0 {
		add(paneIDs[4], terminalui.PaneRegion, terminalui.Rect{X: l.nav, Y: 1 + h, W: l.center, H: l.drawer})
	}
	if l.inspector > 0 && len(m.panes[m.active()].visible()) > 0 && l.height > 13 {
		add("preview", terminalui.ControlRegion, terminalui.Rect{X: l.nav + l.center + 3, Y: 13, W: 22, H: 1})
	}
	for _, r := range m.actionBarFrame().Regions {
		f.Regions = append(f.Regions, r)
	}
	if l.nav > 0 {
		for i, loc := range m.locations {
			if y := m.navY(i); y < l.height {
				add("destination:"+locationID(loc), terminalui.DestinationRegion, terminalui.Rect{X: 1, Y: y, W: l.nav - 2, H: 1})
			}
		}
	}
	appendRegions := func(frame terminalui.Frame, x, y int) {
		for _, r := range frame.Regions {
			r.Bounds.X += x
			r.Bounds.Y += y
			add(r.ID, r.Kind, r.Bounds)
		}
	}
	for i := range m.panes {
		x, w := l.nav, l.left
		if i == 1 {
			x, w = l.nav+l.left, l.center-l.left
		}
		appendRegions(m.filePane(i, w, h), x, 1)
	}
	if l.drawer > 0 {
		appendRegions(m.transferPane(l.center, l.drawer), l.nav, 1+h)
	}
	if m.filtering {
		prefix := " Filter: " + text(m.panes[m.active()].filter) + "▏   "
		limit := m.width
		if m.transferer != nil {
			limit -= len(fmt.Sprintf(" %d transfers ", len(m.queue)))
		}
		for _, c := range []struct{ label, id string }{{"Enter apply", "filter-apply"}, {"Esc clear", "filter-clear"}} {
			x := ansi.StringWidth(prefix)
			if x+len(c.label) < limit {
				add(c.id, terminalui.ControlRegion, terminalui.Rect{X: x, Y: m.height - 1, W: len(c.label), H: 1})
			}
			prefix += c.label + " · "
		}
	}
	if m.transferer != nil && m.status == "" && !m.filtering {
		for _, c := range []struct{ label, id string }{{"Space Select", "mark"}, {"c Copy", "copy"}, {"D Delete", "delete"}, {"? help", "help"}} {
			x := strings.Index(workspaceFooter, c.label)
			if x+len(c.label) <= m.width-13 {
				add("footer:"+c.id, terminalui.ControlRegion, terminalui.Rect{X: x, Y: m.height - 1, W: len(c.label), H: 1})
			}
		}
	}
	return f
}

func (m Model) navigationFocus() string {
	if m.controlFocus != "" {
		for _, r := range m.interactionFrame().Regions {
			if r.ID == m.controlFocus {
				return r.ID
			}
		}
	}
	return paneIDs[m.focus]
}
func (m Model) navigationSelected() string {
	if m.focus == 0 && m.navCursor >= 0 && m.navCursor < len(m.locations) {
		return "destination:" + locationID(m.locations[m.navCursor])
	}
	if m.focus == 1 || m.focus == 2 {
		p := m.panes[m.active()]
		if p.detached {
			return ""
		}
		e := p.visible()
		if p.cursor >= 0 && p.cursor < len(e) {
			return m.rowID(m.active(), e[p.cursor])
		}
	}
	return ""
}

func (m Model) navigationIntent(in terminalui.Intent, f terminalui.Frame) (tea.Model, tea.Cmd) {
	// Resolve against current native identities, never against a previous row index.
	visible := false
	for _, r := range f.Regions {
		if r.ID == in.ID {
			visible = true
			break
		}
	}
	if !visible {
		return m, nil
	}
	if in.Kind == terminalui.Focus {
		m.controlFocus = ""
		for i, id := range paneIDs {
			if id == in.ID {
				m.focus = i
				m.filtering = false
				if i == 1 || i == 2 {
					m.endpoint = i - 1
				}
				return m, nil
			}
		}
		m.controlFocus = in.ID
	}
	for i, p := range m.panes {
		for j, e := range p.visible() {
			id := m.rowID(i, e)
			if in.ID == id || in.ID == "mark:"+id {
				m.focus, m.endpoint = i+1, i
				m.panes[i].cursor = j
				if in.Kind == terminalui.Focus {
					return m, nil
				}
				m.controlFocus, m.filtering = "", false
				if in.ID == "mark:"+id {
					return m.toggleMark()
				}
				if in.Kind == terminalui.Activate {
					return m.openSelection()
				}
				return m, nil
			}
		}
		for _, action := range []string{"folder", "connect", "new", "detach", "parent", "filter", "refresh", "open"} {
			if in.ID == m.paneControl(i, action) {
				m.focus, m.endpoint = i+1, i
				if p.detached {
					for choice, b := range sourceButtons {
						if b.action == action {
							m.panes[i].sourceChoice = choice
							break
						}
					}
				}
				if in.Kind == terminalui.Activate {
					m.controlFocus, m.filtering = "", false
					return m.execute(action)
				}
				return m, nil
			}
		}
	}
	for i, loc := range m.locations {
		if in.ID == "destination:"+locationID(loc) {
			m.focus, m.navCursor = 0, i
			if in.Kind == terminalui.Activate {
				m.controlFocus = ""
				return m.openSelection()
			}
			return m, nil
		}
	}
	for i, j := range m.queue {
		if in.ID == "job:"+locationID(j.source)+":"+locationID(j.destination) {
			m.focus, m.jobCursor = 4, i
			return m, nil
		}
	}
	if in.Kind == terminalui.Scroll {
		switch in.ID {
		case "pane:nav":
			m.navCursor = max(0, min(len(m.locations)-1, m.navCursor+in.Delta))
		case "pane:jobs":
			m.jobCursor = max(0, min(len(m.queue)-1, m.jobCursor+in.Delta))
		case "pane:left", "pane:right":
			i := 0
			if in.ID == "pane:right" {
				i = 1
			}
			p := &m.panes[i]
			if p.detached {
				p.sourceChoice = max(0, min(len(sourceButtons)-1, p.sourceChoice+in.Delta))
			} else {
				p.cursor = max(0, min(len(p.visible())-1, p.cursor+in.Delta))
			}
		}
	}
	if in.Kind == terminalui.Activate {
		return m.execute(strings.TrimPrefix(in.ID, "footer:"))
	}
	return m, nil
}

func (m Model) openSelection() (tea.Model, tea.Cmd) {
	if m.focus == 0 && m.navCursor >= 0 && m.navCursor < len(m.locations) {
		if reason := m.connectionDisabled(); m.transferer != nil && reason != "" {
			m.status = reason
			return m, nil
		}
		i := m.active()
		m.focus, m.endpoint = i+1, i
		return m, m.navigate(i, m.locations[m.navCursor])
	}
	if m.focus != 1 && m.focus != 2 {
		return m, nil
	}
	i := m.active()
	p := m.panes[i]
	if p.detached {
		return m.execute(sourceButtons[p.sourceChoice].action)
	}
	e := p.visible()
	if p.cursor < 0 || p.cursor >= len(e) {
		return m, nil
	}
	if e[p.cursor].IsDir {
		loc := p.location
		loc.Path = path.Join(loc.Path, e[p.cursor].Name)
		return m, m.navigate(i, loc)
	}
	m.inspector = true
	return m, nil
}
func (m Model) toggleMark() (tea.Model, tea.Cmd) {
	if m.focus == 1 || m.focus == 2 {
		p := &m.panes[m.active()]
		entries := p.visible()
		if !p.loading && p.err == nil && p.cursor >= 0 && p.cursor < len(entries) && entries[p.cursor].Name != ".." {
			name := entries[p.cursor].Name
			p.selected[name] = !p.selected[name]
		}
	}
	return m, nil
}

package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func (m Model) workspaceFrames() []terminalui.Frame {
	h := m.height - 7
	if m.detail {
		return []terminalui.Frame{m.inspectorFrame(terminalui.Rect{Y: 3, W: m.width, H: h})}
	}
	left, right := 0, 0
	if m.width >= 140 {
		left = 24
	}
	if m.width >= 120 {
		right = min(50, m.width/3)
	}
	var frames []terminalui.Frame
	if left > 0 {
		frames = append(frames, m.workspaceFrame(terminalui.Rect{Y: 3, W: left, H: h}))
	}
	frames = append(frames, m.logListFrame(terminalui.Rect{X: left, Y: 3, W: m.width - left - right, H: h}))
	if right > 0 {
		frames = append(frames, m.inspectorFrame(terminalui.Rect{X: m.width - right, Y: 3, W: right, H: h}))
	}
	return frames
}

func (m Model) navigationFocus() string {
	if m.controlFocus != "" {
		return m.controlFocus
	}
	switch m.focus {
	case 0:
		return "workspace"
	case 2:
		return "inspector"
	}
	return "logs"
}

func (m Model) fieldID(path string) string {
	if len(m.page.Events) == 0 {
		return ""
	}
	return "field:" + m.page.Events[m.selected].Handle + ":" + path
}

func (m Model) workspaceActions() []terminalui.Action {
	if m.focus == 0 {
		return []terminalui.Action{{ID: "workspace-open", Label: "Open", Keys: []string{"enter"}}, {ID: "save", Label: "Save", Keys: []string{"S"}}}
	}
	if m.focus == 2 {
		available := ""
		if len(m.inspectorFields()) == 0 {
			available = "No selected field"
		}
		return []terminalui.Action{
			{ID: "return", Label: "Back", Keys: []string{"esc"}},
			{ID: "field-toggle", Label: "Expand", Keys: []string{"enter"}, Unavailable: available},
			{ID: "field-filter", Label: "Filter", Keys: []string{"f"}, Unavailable: available},
			{ID: "yank-value", Label: "Copy value", Keys: []string{"y"}, Unavailable: available},
			{ID: "yank-pair", Label: "Copy pair", Keys: []string{"Y"}, Unavailable: available},
		}
	}
	var actions []terminalui.Action
	for _, id := range []string{"inspect", "filters", "older"} {
		for _, action := range m.paletteActions() {
			if action.ID == id {
				action.Label = map[string]string{"inspect": "Inspect", "filters": "Filters", "older": "Older"}[id]
				actions = append(actions, action)
			}
		}
	}
	return actions
}

func (m Model) workspaceBar() terminalui.Frame {
	return terminalui.RenderActionBar(m.workspaceActions(), terminalui.Rect{Y: m.height - 4, W: m.width, H: 1}, m.navigationFocus(), m.options.Theme.Interactions())
}

func (m Model) navigationFrame() terminalui.Frame {
	f := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}}
	var bodyRegions []terminalui.Region
	for _, pane := range m.workspaceFrames() {
		// Regions may only describe content inside the current rendered pane,
		// excluding clipped body lines and the bottom status border.
		for _, region := range pane.Regions {
			if region.Kind != terminalui.PaneRegion && (region.Bounds.Y < pane.Bounds.Y+1 || region.Bounds.Y+region.Bounds.H > pane.Bounds.Y+pane.Bounds.H-1 || region.Bounds.X+region.Bounds.W > pane.Bounds.X+pane.Bounds.W-1) {
				continue
			}
			if region.Kind == terminalui.PaneRegion {
				f.Regions = append(f.Regions, region)
			} else {
				bodyRegions = append(bodyRegions, region)
			}
		}
	}
	f.Regions = append(f.Regions, bodyRegions...)
	f.Regions = append(f.Regions, m.actionBar().Regions...)
	f.Regions = append(f.Regions, m.workspaceBar().Regions...)
	return f
}

func (m Model) navigationInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	frame := m.navigationFrame()
	// Bars retain current eligibility checks, including controls unavailable
	// while a request is busy. Shared navigation handles focus, not permission.
	for _, bar := range []struct {
		actions []terminalui.Action
		frame   terminalui.Frame
	}{{m.headerActions(), m.actionBar()}, {m.workspaceActions(), m.workspaceBar()}} {
		if _, key := msg.(tea.KeyMsg); key {
			var focused []terminalui.Action
			for _, action := range bar.actions {
				if action.ID == m.controlFocus {
					action.Keys = nil
					focused = append(focused, action)
				}
			}
			bar.actions = focused
		}
		r := terminalui.HandleActionBar(msg, bar.actions, bar.frame, m.navigationFocus())
		if r.Consumed {
			if r.Intent != nil {
				return m.navigationActivate(r.Intent.ID)
			}
			return m, nil
		}
	}
	selected := ""
	if m.focus == 1 && len(m.page.Events) > 0 {
		selected = "event:" + m.page.Events[m.selected].Handle
	} else if m.focus == 2 {
		fields := m.inspectorFields()
		if len(fields) > 0 {
			selected = m.fieldID(fields[min(m.fieldIndex, len(fields)-1)].path)
		}
	} else if m.workspaceRow == 0 {
		selected = "source"
	} else if m.workspaceRow <= len(m.saved) {
		s := m.saved[m.workspaceRow-1]
		selected = "saved:" + s.Query.Scope.Source + ":" + s.Name
	} else {
		selected = "restore"
	}
	r := terminalui.HandleNavigation(msg, frame, m.navigationFocus(), selected)
	if r.Intent == nil {
		return m, nil
	}
	i := *r.Intent
	switch i.Kind {
	case terminalui.Focus:
		m.controlFocus = i.ID
		if i.ID == "source" || i.ID == "restore" {
			m.reloadSaved()
			m.focus, m.workspaceRow = 0, 0
			if i.ID == "restore" {
				m.workspaceRow = len(m.saved) + 1
			}
		}
		if strings.HasPrefix(i.ID, "toggle:") {
			for n, field := range m.inspectorFields() {
				if i.ID == "toggle:"+m.fieldID(field.path) {
					m.focus, m.fieldIndex = 2, n
				}
			}
		}
		for n, id := range []string{"workspace", "logs", "inspector"} {
			if i.ID == id {
				m.focus, m.controlFocus = n, ""
				if n == 0 {
					m.reloadSaved()
				}
			}
		}
	case terminalui.Select, terminalui.Activate:
		return m.navigationActivateSelection(i.ID, i.Kind == terminalui.Activate)
	case terminalui.Scroll:
		switch i.ID {
		case "workspace":
			m.workspaceRow = min(len(m.saved)+1, max(0, m.workspaceRow+i.Delta))
		case "logs":
			m.selected = min(max(0, len(m.page.Events)-1), max(0, m.selected+i.Delta))
			m.fieldIndex, m.inspectorOffset = 0, 0
			m.keepVisible()
		case "inspector":
			m.inspectorOffset += i.Delta
			m.clampInspector()
		}
	}
	return m, nil
}

func (m Model) navigationActivate(id string) (tea.Model, tea.Cmd) {
	if id == "workspace-open" {
		m.controlFocus = ""
		return m.openWorkspaceRow()
	}
	if id == "field-toggle" {
		m.toggleField()
		return m, nil
	}
	m.controlFocus = ""
	return m.execute(id)
}

func (m Model) navigationActivateSelection(id string, activate bool) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(id, "toggle:") {
		id = strings.TrimPrefix(id, "toggle:")
		activate = true
	}
	for n, event := range m.page.Events {
		if id == "event:"+event.Handle {
			if n != m.selected {
				m.fieldIndex, m.inspectorOffset = 0, 0
			}
			m.selected, m.focus, m.controlFocus = n, 1, ""
			m.keepVisible()
			if activate {
				return m.execute("inspect")
			}
			return m, nil
		}
	}
	for n, field := range m.inspectorFields() {
		if id == m.fieldID(field.path) {
			m.fieldIndex, m.focus, m.controlFocus = n, 2, ""
			if activate {
				m.toggleField()
			}
			return m, nil
		}
	}
	for n, saved := range m.saved {
		if id == "saved:"+saved.Query.Scope.Source+":"+saved.Name {
			m.workspaceRow, m.focus, m.controlFocus = n+1, 0, ""
			if activate {
				return m.openWorkspaceRow()
			}
			return m, nil
		}
	}
	if id == "source" {
		m.workspaceRow, m.focus, m.controlFocus = 0, 1, ""
		return m, nil
	}
	return m.navigationActivate(id)
}

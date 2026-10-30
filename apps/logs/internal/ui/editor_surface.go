package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"terminalui"
)

func (m Model) editorBounds() terminalui.Rect {
	return terminalui.Rect{Y: 1, W: m.width, H: m.height - 1}
}

func (m Model) editorSurface() terminalui.SurfaceSpec {
	e, b := m.editor, m.editorBounds()
	body := terminalui.SurfaceBodyBounds(b)
	s := terminalui.SurfaceSpec{ID: "editor", CancelID: "cancel", Actions: []terminalui.Action{{ID: "cancel", Label: "Cancel", Keys: []string{"esc"}}}}
	if e.kind == "filter" {
		s.Title, s.Body = "FILTER EDITOR", m.filterBody(body)
		s.FocusID = "facet-input"
		if e.sidebar {
			s.FocusID = "facet-nav"
		}
		s.FocusOrder = []string{"facet-input", "facet-nav"}
		s.Actions = append(s.Actions, terminalui.Action{ID: "apply", Label: "Apply", Keys: []string{"ctrl+s"}})
		if e.field == 0 {
			a := terminalui.Action{ID: "toggle-app", Label: "Toggle app"}
			if len(e.suggestions.Items) == 0 {
				a.Unavailable = "No suggestion"
			}
			s.Actions = append(s.Actions, a)
		} else if e.field == 6 {
			a := terminalui.Action{ID: "remove-predicate", Label: "Remove field"}
			if len(e.draft.Fields) == 0 {
				a.Unavailable = "No predicate"
			}
			s.Actions = append(s.Actions, a)
		}
		s.Actions = append(s.Actions, terminalui.Action{ID: "clear-filters", Label: "Clear filters", Keys: []string{"ctrl+l"}})
		for _, action := range s.Actions {
			s.FocusOrder = append(s.FocusOrder, action.ID)
		}
	} else {
		s.Body.Bounds = body
		s.Body.Regions = []terminalui.Region{{ID: "editor-body", Kind: terminalui.PaneRegion, Bounds: body}}
		row := func(id string, y int, kind terminalui.RegionKind) {
			s.Body.Regions = append(s.Body.Regions, terminalui.Region{ID: id, Kind: kind, Bounds: terminalui.Rect{X: body.X, Y: body.Y + y, W: body.W, H: 1}})
		}
		title, lines := m.editorLines(row)
		s.Title = title
		for _, line := range lines {
			s.Body.Lines = append(s.Body.Lines, oneLine(line))
		}
		s.FocusID, s.FocusOrder = "editor-body", []string{"editor-body"}
		primary := terminalui.Action{}
		switch e.kind {
		case "settings":
			primary = terminalui.Action{ID: "save-settings", Label: "Apply settings", Keys: []string{"ctrl+s"}}
			row("toggle-vim", 1, terminalui.ControlRegion)
			s.FocusID, s.FocusOrder = "toggle-vim", []string{"toggle-vim"}
		case "find":
			primary = terminalui.Action{ID: "find", Label: "Find"}
			row("text-input", 3, terminalui.FieldRegion)
			row("toggle-case", 2, terminalui.ControlRegion)
			s.FocusID, s.FocusOrder = "text-input", []string{"text-input", "toggle-case"}
		case "save":
			primary = terminalui.Action{ID: "save-search", Label: "Save search"}
			if m.options.SearchesPath == "" {
				primary.Unavailable = "Requires explicit --searches path"
			}
			row("text-input", 3, terminalui.FieldRegion)
			s.FocusID, s.FocusOrder = "text-input", []string{"text-input"}
		case "restore":
			primary = terminalui.Action{ID: "restore-search", Label: "Restore"}
			if len(e.saved) == 0 {
				primary.Unavailable = "No saved searches"
			}
		case "field":
			primary = terminalui.Action{ID: "field-apply", Label: "Apply exact filter"}
			if len(e.choices) == 0 {
				primary.Unavailable = "No selected field"
			} else {
				primary.Unavailable = e.choices[e.suggestion].unavailable
			}
		}
		s.FocusOrder = append(s.FocusOrder, "cancel", primary.ID)
		s.Actions = append(s.Actions, primary)
		// Reserve validation at the bottom rather than clipping it off-screen.
		if e.error != "" {
			if n := len(s.Body.Lines); n > 0 && s.Body.Lines[n-1] == oneLine(e.error) {
				s.Body.Lines = s.Body.Lines[:n-1]
			}
			errors := strings.Split(ansi.Hardwrap(oneLine(e.error), max(1, body.W), true), "\n")
			s.Body.Lines = s.Body.Lines[:min(len(s.Body.Lines), max(0, body.H-len(errors)))]
			visible := body
			visible.H = len(s.Body.Lines)
			regions := s.Body.Regions[:0]
			for _, region := range s.Body.Regions {
				region.Bounds.H = min(region.Bounds.H, visible.Y+visible.H-region.Bounds.Y)
				if region.Bounds.W > 0 && region.Bounds.H > 0 {
					regions = append(regions, region)
				}
			}
			s.Body.Regions = regions
			for _, line := range errors {
				s.Body.Lines = append(s.Body.Lines, m.style(m.options.Theme.Error).Render(line))
			}
		}
	}
	if e.surfaceFocus != "" {
		s.FocusID = e.surfaceFocus
	}
	return s
}

func (m Model) editorFrame() terminalui.Frame {
	return terminalui.RenderSurface(m.editorSurface(), m.editorBounds(), m.options.Theme.Interactions())
}

func (m Model) editorInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	e := m.editor
	if k, ok := msg.(tea.KeyMsg); ok {
		key := k.String()
		// Text and contextual Enter/Esc stay local. Tab traverses the current
		// visible body/footer scope; leaving INSERT retains its draft.
		if (e.surfaceFocus == "" && key != "tab" && key != "shift+tab") || key == "ctrl+s" || key == "esc" || key == "ctrl+t" || key == "ctrl+l" || key == "ctrl+x" || key == "ctrl+r" {
			return m.edit(k)
		}
		if e.kind == "filter" && (key == "tab" || key == "shift+tab") {
			e.insert = false
		}
	}
	s := m.editorSurface()
	f := terminalui.RenderSurface(s, m.editorBounds(), m.options.Theme.Interactions())
	r := terminalui.HandleSurface(msg, s, f)
	if r.Intent == nil {
		return m, nil
	}
	i := *r.Intent
	switch i.Kind {
	case terminalui.Dismiss:
		return m.editorAction("cancel")
	case terminalui.Focus:
		e.surfaceFocus = i.ID
		if n, err := strconv.Atoi(strings.TrimPrefix(i.ID, "input-")); strings.HasPrefix(i.ID, "input-") && err == nil {
			e.field, e.sidebar, e.insert, e.surfaceFocus = n, false, false, ""
		}
		if i.ID == "editor-body" || i.ID == "text-input" || i.ID == "toggle-vim" {
			e.surfaceFocus = ""
		}
		if i.ID == "facet-nav" || i.ID == "facet-input" {
			e.sidebar, e.insert, e.surfaceFocus = i.ID == "facet-nav", false, ""
		}
	case terminalui.Activate:
		for n, app := range e.suggestions.Items {
			if i.ID == "app-toggle:"+app {
				e.suggestion, e.sidebar, e.insert = n, false, false
				return m.editorAction("toggle-app")
			}
		}
		if field, err := strconv.Atoi(strings.TrimPrefix(i.ID, "facet-")); strings.HasPrefix(i.ID, "facet-") && err == nil {
			e.field, e.sidebar, e.insert, e.surfaceFocus = field, false, false, ""
			if field == 0 {
				return m.suggest()
			}
			return m, nil
		}
		return m.editorAction(i.ID)
	case terminalui.Select:
		e.surfaceFocus = ""
		for n, saved := range e.saved {
			if i.ID == "restore-row:"+saved.Query.Scope.Source+":"+saved.Name {
				e.suggestion = n
			}
		}
		for n, choice := range e.choices {
			if i.ID == "field-row:"+choice.path {
				e.suggestion = n
			}
		}
		for n, app := range e.suggestions.Items {
			if i.ID == "app-row:"+app {
				e.suggestion = n
			}
		}
		for n, field := range e.draft.Fields {
			if i.ID == "predicate-row:"+field.Path+":"+string(field.Value) {
				e.predicate = n
			}
		}
		if e.kind == "filter" {
			e.sidebar, e.insert = false, false
		}
	case terminalui.Scroll:
		if e.kind == "field" {
			e.suggestion = min(max(0, len(e.choices)-1), max(0, e.suggestion+i.Delta))
		} else if e.kind == "restore" {
			e.suggestion = min(max(0, len(e.saved)-1), max(0, e.suggestion+i.Delta))
		} else if e.kind == "filter" {
			mouse := msg.(tea.MouseMsg)
			for _, pane := range s.Body.Regions {
				b := pane.Bounds
				if pane.ID != "facet-input" || mouse.X < b.X || mouse.X >= b.X+b.W || mouse.Y < b.Y || mouse.Y >= b.Y+b.H {
					continue
				}
				if e.field == 0 {
					e.suggestion = min(max(0, len(e.suggestions.Items)-1), max(0, e.suggestion+i.Delta))
				} else if e.field == 6 {
					e.predicate = min(max(0, len(e.draft.Fields)-1), max(0, e.predicate+i.Delta))
				}
			}
		}
	}
	return m, nil
}

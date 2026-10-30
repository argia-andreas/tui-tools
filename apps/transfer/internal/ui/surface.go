package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func (m Model) surfaceBounds() terminalui.Rect {
	if !m.workspaceFits() {
		return terminalui.Rect{}
	}
	w, h := m.modalSize()
	return terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}
}

// The body and operations stay FTP-owned. Shared chrome derives every visible
// control and hit target from this same current-state specification.
func (m Model) surfaceSpec() terminalui.SurfaceSpec {
	bounds := m.surfaceBounds()
	body := terminalui.SurfaceBodyBounds(bounds)
	s := terminalui.SurfaceSpec{ID: m.modal, CancelID: "cancel", Body: terminalui.Frame{Bounds: body}}
	cancel := terminalui.Action{ID: "cancel", Label: "Cancel"}
	s.Actions = []terminalui.Action{cancel}
	var fields []string
	switch m.modal {
	case "delete":
		s.Title, s.Body.Lines = m.deleteLines()
		s.Actions = append(s.Actions, terminalui.Action{ID: "delete", Label: "Delete", Dangerous: true})
	case "folder":
		s.Title = "Open local folder · " + paneName(m.folderPane) + " pane"
		s.Body.Lines = []string{"", "  Folder", "  > " + text(m.folderPath) + "▏", m.paint("  "+text(m.status), m.theme.Error), "  Tab focus · Enter opens · Ctrl+U clear · Esc cancel"}
		fields = []string{"folder-path"}
		s.Body.Regions = []terminalui.Region{{ID: fields[0], Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: body.X, Y: body.Y + 2, W: body.W, H: 1}}}
		s.Actions = append(s.Actions, terminalui.Action{ID: "open-folder", Label: "Open"})
	case "file-rename":
		s.Title, s.Body.Lines = "RENAME FILE?", m.renameLines(bounds.W, bounds.H)
		s.Actions = append(s.Actions, terminalui.Action{ID: "rename", Label: "Rename"})
	case "copy":
		s.Title, s.Body.Lines = m.copyLines(bounds.W, bounds.H)
		s.Actions = nil
		for _, label := range m.copyButtons() {
			s.Actions = append(s.Actions, terminalui.Action{ID: strings.ToLower(label), Label: label, Dangerous: label == "Overwrite"})
		}
	case "rename":
		s.Title = "RENAME DESTINATION"
		s.Body.Lines = []string{"  New filename (Ctrl+U clears):", "  " + text(m.query) + "▏", m.paint("  "+text(m.status), m.theme.Error), "  Enter checks name · Esc cancels copy"}
		fields = []string{"copy-name"}
		s.Body.Regions = []terminalui.Region{{ID: fields[0], Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: body.X, Y: body.Y + 1, W: body.W, H: 1}}}
		s.Actions = append(s.Actions, terminalui.Action{ID: "check-name", Label: "Check name"})
	case "batch":
		s.Title, s.Body.Lines = m.batchLines()
		if !m.queuePlanning {
			s.Actions = append(s.Actions, terminalui.Action{ID: "start-queue", Label: "Start queue"})
		}
	case "preview":
		s.Title, s.Body.Lines = m.previewLines(bounds.W, bounds.H)
		s.Actions[0].Label = "Close"
		fields = []string{"preview-body"}
		s.Body.Regions = []terminalui.Region{{ID: fields[0], Kind: terminalui.PaneRegion, Bounds: body}}
	}
	s.FocusOrder = append(s.FocusOrder, fields...)
	for _, action := range s.Actions {
		s.FocusOrder = append(s.FocusOrder, action.ID)
	}
	if m.modal == "preview" && m.preview != nil {
		s.FocusID = m.preview.focus
	} else if len(s.FocusOrder) > 0 {
		s.FocusID = s.FocusOrder[max(0, min(m.choice, len(s.FocusOrder)-1))]
	}
	return s
}

func (m Model) surfaceFrame() terminalui.Frame {
	return terminalui.RenderSurface(m.surfaceSpec(), m.surfaceBounds(), m.theme.TerminalStyle(false))
}

func (m Model) surfaceInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	spec := m.surfaceSpec()
	frame := terminalui.RenderSurface(spec, m.surfaceBounds(), m.theme.TerminalStyle(false))
	bodyVisible := terminalui.SurfaceBodyBounds(frame.Bounds).H > 0
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+p" && (m.modal == "delete" || m.modal == "folder" || m.modal == "preview") {
			return m.surfaceAction("cancel")
		}
		// Field grammar is local, and only a currently rendered field owns it.
		for _, region := range frame.Regions {
			if region.ID == spec.FocusID && region.Kind == terminalui.FieldRegion && key.String() != "tab" && key.String() != "shift+tab" && key.String() != "esc" {
				if m.modal == "folder" {
					return m.folderKey(key)
				}
				if m.modal == "rename" {
					return m.copyKey(key)
				}
			}
		}
		if bodyVisible && m.modal == "file-rename" && (key.String() == "up" || key.String() == "down") {
			delta := 1
			if key.String() == "up" {
				delta = -1
			}
			return m.scrollRename(delta), nil
		}
		if bodyVisible && m.modal == "preview" && spec.FocusID == "preview-body" && key.String() != "tab" && key.String() != "shift+tab" && key.String() != "esc" {
			return m.previewKey(key)
		}
	}
	r := terminalui.HandleSurface(msg, spec, frame)
	if r.Intent == nil {
		return m, nil
	}
	switch r.Intent.Kind {
	case terminalui.Dismiss:
		return m.surfaceAction("cancel")
	case terminalui.Activate:
		return m.surfaceAction(r.Intent.ID)
	case terminalui.Focus:
		if m.modal == "preview" && m.preview != nil {
			p := *m.preview
			p.focus = r.Intent.ID
			m.preview = &p
		} else {
			for i, id := range spec.FocusOrder {
				if id == r.Intent.ID {
					m.choice = i
				}
			}
		}
	case terminalui.Scroll:
		if m.modal == "preview" {
			m = m.scrollPreview(r.Intent.Delta)
		} else if m.modal == "file-rename" {
			m = m.scrollRename(r.Intent.Delta)
		}
	}
	return m, nil
}

func (m Model) scrollRename(delta int) Model {
	r := *m.rename
	w, h := m.modalSize()
	r.scroll = max(0, min(r.scroll+delta, max(0, len(m.renameDetails(w))-(h-4))))
	m.rename = &r
	return m
}

func (m Model) surfaceAction(id string) (tea.Model, tea.Cmd) {
	// Re-enter domain checks from both keyboard and pointer intents. The shared
	// component never receives a storage callback or owns the captured target.
	switch m.modal {
	case "delete":
		return m.deleteAction(id)
	case "file-rename":
		return m.renameAction(id)
	case "copy", "rename":
		return m.copyAction(id)
	case "batch":
		return m.batchAction(id)
	case "preview":
		if id == "cancel" {
			return m.closePreview()
		}
	case "folder":
		if id == "cancel" {
			m.modal = ""
		} else if id == "open-folder" {
			return m.openFolder()
		}
	}
	return m, nil
}

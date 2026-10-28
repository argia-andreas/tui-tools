package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func (m Model) actionsBlocked() string {
	if m.review.phase != "" {
		return "Review active"
	}
	if m.reader.load != "" {
		return "Full load active"
	}
	if m.env.page != "" {
		return "Environment active"
	}
	return ""
}

func (m Model) toolbarActions() []terminalui.Action {
	actions := []terminalui.Action{
		{ID: "help", Label: "Help", Keys: []string{"f1"}},
		{ID: "actions", Label: "Actions", Keys: []string{"ctrl+p"}, Unavailable: m.actionsBlocked()},
	}
	// Keep the current refusal visible even in the 60-column reader. The
	// blocked shortcut is captured explicitly below, never passed to a draft.
	if actions[1].Unavailable != "" {
		actions[1].Keys = nil
	}
	return actions
}

func (m Model) toolbarFrame() terminalui.Frame {
	y := 1
	if m.compactReading() {
		y = 0
	}
	if !m.workspaceFits() {
		return terminalui.Frame{}
	}
	return terminalui.RenderActionBar(m.toolbarActions(), terminalui.Rect{X: 0, Y: y, W: m.width, H: 1}, m.focus, m.theme.TerminalStyle(m.ascii))
}

func (m Model) overlayBounds() terminalui.Rect {
	if !m.workspaceFits() {
		return terminalui.Rect{}
	}
	w, h := max(0, min(82, m.width-6)), max(0, min(18, m.height-4))
	return terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}
}

func (m Model) helpActions() []terminalui.Action {
	if m.palette || m.actionsBlocked() != "" {
		return nil
	}
	return m.actions()
}

func (m Model) helpTopics() []terminalui.HelpTopic {
	topics := []terminalui.HelpTopic{
		{Group: "Help", Keys: "F1 / Esc / Close", Description: "Return without changing parent"},
		{Group: "Help", Keys: "arrows / PgUp/PgDn", Description: "Scroll Help"},
	}
	if m.palette {
		return append(topics,
			terminalui.HelpTopic{Group: "Palette", Keys: "Type / Backspace / Ctrl+U", Description: "Edit fuzzy query; workspace shortcuts stay captured"},
			terminalui.HelpTopic{Group: "Palette", Keys: "Arrows / Tab / Shift+Tab", Description: "Choose result without wrapping"},
			terminalui.HelpTopic{Group: "Palette", Keys: "Enter / result click", Description: "Run an available action"},
			terminalui.HelpTopic{Group: "Palette", Keys: "Ctrl+P / Esc / outside click", Description: "Dismiss palette without click-through"})
	}
	if m.env.page != "" {
		return append(topics, m.environmentHelpTopics()...)
	}
	if m.review.phase != "" {
		if m.review.phase == "destination" {
			return append(topics,
				terminalui.HelpTopic{Group: "Destination", Keys: "Type / Backspace / Ctrl+U", Description: "Filter destination queues; printable keys stay in this field"},
				terminalui.HelpTopic{Group: "Destination", Keys: "Up/Down / Enter", Description: "Choose queue / return to configuration; never sends"},
				terminalui.HelpTopic{Group: "Destination", Keys: "Esc", Description: "Cancel destination selection without acquiring a lock"})
		}
		topics = append(topics,
			terminalui.HelpTopic{Group: "Review", Keys: "Tab / arrows / Enter", Description: "Choose and request a permitted action; confirmations start on Cancel"},
			terminalui.HelpTopic{Group: "Review", Keys: "Esc / Ctrl+C", Description: "Request safe cancellation / safe quit; in-flight work must reconcile"},
			terminalui.HelpTopic{Group: "Review", Keys: "PgUp/PgDn", Description: "Scroll body; hidden confirmation controls cannot execute"},
			terminalui.HelpTopic{Group: "Review", Description: "Actions blocked. Lock expiry and completion continue during Help"})
		if m.review.selected {
			topics = append(topics, terminalui.HelpTopic{Group: "Review", Description: "Destination and cleanup freeze after preparation; resend never implies cleanup"})
		} else if m.review.phase == "review" {
			topics = append(topics, terminalui.HelpTopic{Group: "Review", Description: "Delete original requires exact typed delete; resend never implies cleanup"})
		}
		return topics
	}
	if m.reader.load != "" {
		return append(topics,
			terminalui.HelpTopic{Group: "Full content", Keys: "Tab / arrows / Enter", Description: "Choose Cancel or authorize bounded full-content loading"},
			terminalui.HelpTopic{Group: "Full content", Keys: "Esc", Description: "Cancel pending load; clipboard unchanged"},
			terminalui.HelpTopic{Group: "Full content", Description: "Actions blocked. Completion continues while Help is showing"})
	}
	topics = append(topics, []terminalui.HelpTopic{
		{Group: "Workspace", Keys: "Ctrl+P", Description: "Open/close Actions when permitted"},
		{Group: "Workspace", Description: "Actions blocked during review/full-content load"},
		{Group: "Workspace", Keys: "j/k, arrows; Enter", Description: "Select; open resource/message"},
		{Group: "Workspace", Keys: "Tab / Ctrl+H/L", Description: "Change pane"},
		{Group: "Workspace", Keys: "q", Description: "Quit outside captured Help/palette"},
		{Group: "Workspace", Keys: "Ctrl+C", Description: "Quit safely, including from Help/palette"},
	}...)
	if m.screen == "messages" {
		topics = append(topics,
			terminalui.HelpTopic{Group: "Reader", Keys: "h/j/k/l, arrows; w/b/e", Description: "Cursor; words"},
			terminalui.HelpTopic{Group: "Reader", Keys: "b", Description: "Word-back here, not sidebar"},
			terminalui.HelpTopic{Group: "Reader", Keys: "0/$, Home/End; gg/G", Description: "Line start/end; document start/end"},
			terminalui.HelpTopic{Group: "Reader", Keys: "PgUp/PgDn; Ctrl+U/D", Description: "Page; half page"},
			terminalui.HelpTopic{Group: "Reader", Keys: "yy", Description: "Copy whole logical line"},
			terminalui.HelpTopic{Group: "Reader", Keys: "v / V", Description: "Select characters / whole lines"},
			terminalui.HelpTopic{Group: "Reader", Keys: "y / Y", Description: "Copy selection / full body"},
			terminalui.HelpTopic{Group: "Reader", Keys: "Esc", Description: "Clear selection, then Back to list"},
			terminalui.HelpTopic{Group: "Reader", Keys: "Tab", Description: "Text / Back / Resend / other panes"},
			terminalui.HelpTopic{Group: "Reader", Keys: "Ctrl+R", Description: "Configure resend (DLQ only); never sends"},
			terminalui.HelpTopic{Group: "Clipboard", Keys: "Ctrl+P", Description: "Open Actions, not copy report"},
			terminalui.HelpTopic{Group: "Clipboard", Description: "Copy message report: metadata, body, properties"},
			terminalui.HelpTopic{Group: "Clipboard", Description: "Local tools or OSC52 request"},
			terminalui.HelpTopic{Group: "Clipboard", Description: "Remote requests may need terminal permission"},
			terminalui.HelpTopic{Group: "Clipboard", Description: "Failure: use terminal selection/copy"},
			terminalui.HelpTopic{Group: "Clipboard", Description: "Fallback may include gutters/soft wraps"},
		)
	}
	return topics
}

func (m Model) sharedInput(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if m.help {
		f := terminalui.RenderHelp(m.helpActions(), m.helpTopics(), m.helpOffset, m.overlayBounds(), m.theme.TerminalStyle(m.ascii))
		r := terminalui.HandleHelp(msg, m.helpActions(), m.helpTopics(), &m.helpOffset, f)
		if r.Intent != nil && r.Intent.Kind == terminalui.Dismiss {
			m.help = false
		}
		return m, nil, true
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "f1" {
		m.help, m.helpOffset = true, 0
		return m, nil, true
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+p" && m.actionsBlocked() != "" {
		return m, nil, true
	}
	if m.palette {
		// Eligibility is recomputed on every input, never retained from open time.
		if m.actionsBlocked() != "" {
			m.palette = false
			return m, nil, true
		}
		r := m.paletteState.Handle(msg, m.actions(), m.paletteState.View(m.actions(), m.overlayBounds(), m.theme.TerminalStyle(m.ascii)))
		if r.Intent != nil {
			switch r.Intent.Kind {
			case terminalui.Dismiss:
				m.palette = false
			case terminalui.Help:
				m.help, m.helpOffset = true, 0
			case terminalui.Activate:
				for _, a := range m.actions() {
					if a.ID == r.Intent.ID && a.Unavailable == "" && !a.Busy {
						m.palette = false
						next, cmd := m.execute(a.ID)
						return next, cmd, true
					}
				}
			}
		}
		return m, nil, true
	}
	r := terminalui.HandleActionBar(msg, m.toolbarActions(), m.toolbarFrame(), m.focus)
	if r.Intent != nil {
		if r.Intent.ID == "help" {
			m.help, m.helpOffset = true, 0
		}
		if r.Intent.ID == "actions" && m.actionsBlocked() == "" {
			m.palette, m.paletteState = true, terminalui.Palette{}
		}
	}
	return m, nil, r.Consumed
}

package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

// Help documents only the owner beneath it, never an always-global keymap.
func (m Model) helpContent() ([]terminalui.Action, []terminalui.HelpTopic) {
	topics := []terminalui.HelpTopic{{Group: "Help", Keys: "F1 / Esc / Close", Description: "Return without changing the parent draft, query or focus"}}
	if m.editor != nil {
		switch m.editor.kind {
		case "palette":
			return nil, append(topics,
				terminalui.HelpTopic{Group: "Palette", Keys: "Type / Backspace / Ctrl+U", Description: "Edit fuzzy query; workspace shortcuts are not active"},
				terminalui.HelpTopic{Group: "Palette", Keys: "Up/Down / Tab/Shift+Tab", Description: "Choose result (clamped); Enter or click runs enabled action"},
				terminalui.HelpTopic{Group: "Palette", Keys: "Esc / Ctrl+P / Close / outside click", Description: "Dismiss palette; query is retained while viewing Help"})
		case "filter":
			topics = append(topics,
				terminalui.HelpTopic{Group: "Filters", Keys: "Ctrl+S", Description: "Apply draft filters and refresh; does not save settings"},
				terminalui.HelpTopic{Group: "Filters", Keys: "Tab / Shift+Tab", Description: "Cycle editor, sidebar and visible footer controls; leave INSERT retaining draft"},
				terminalui.HelpTopic{Group: "Filters", Keys: "Ctrl+L / Ctrl+X / Ctrl+R", Description: "Clear draft except time / clear apps / clear exact predicates"},
				terminalui.HelpTopic{Group: "Filters", Keys: "App selection", Description: "Exact selected apps OR together; keyword alone is not a filter"})
			if m.settings.VimMode {
				topics = append(topics,
					terminalui.HelpTopic{Group: "NORMAL", Keys: "j/k / arrows", Description: "Navigate facets, suggestions or Time Start/End"},
					terminalui.HelpTopic{Group: "NORMAL", Keys: "Enter / i", Description: "Start INSERT in text fields; Enter on exact predicate removes it"},
					terminalui.HelpTopic{Group: "NORMAL", Keys: "Space / Esc", Description: "Toggle chosen App / cancel draft without applying"},
					terminalui.HelpTopic{Group: "INSERT", Keys: "Text / Enter / first Esc", Description: "j/k are literal; Enter or first Esc returns to NORMAL, retaining draft"})
			} else {
				topics = append(topics, terminalui.HelpTopic{Group: "Direct entry", Keys: "Type / Enter / Esc", Description: "Edit text; Enter toggles App/removes predicate or applies; Esc cancels"})
			}
		case "settings":
			topics = append(topics,
				terminalui.HelpTopic{Group: "Settings", Keys: "Enter / Space", Description: "Toggle draft Vim-style facet input preference"},
				terminalui.HelpTopic{Group: "Settings", Keys: "Ctrl+S / Esc", Description: "Save settings (session only without path) / cancel; not filter Apply"})
		case "field":
			topics = append(topics,
				terminalui.HelpTopic{Group: "Exact field picker", Keys: "j/k / arrows / g/G / Home/End", Description: "Choose field; no workspace shortcuts are active"},
				terminalui.HelpTopic{Group: "Exact field picker", Keys: "Enter / Esc", Description: "Add full safe typed value as AND filter / return without applying"},
				terminalui.HelpTopic{Group: "Exact field picker", Keys: "Unavailable", Description: "Objects, arrays, null, redacted or unsupported fields cannot filter"})
		case "find":
			topics = append(topics, terminalui.HelpTopic{Group: "Literal find", Keys: "Text / Ctrl+T / Enter / Esc", Description: "Edit / toggle case / find in loaded safe data / cancel"})
		case "save":
			topics = append(topics, terminalui.HelpTopic{Group: "Named search", Keys: "Text / Enter / Esc", Description: "Name / explicitly save or replace current query / cancel"})
		case "restore":
			topics = append(topics, terminalui.HelpTopic{Group: "Restore", Keys: "Up/Down / Enter / Esc", Description: "Choose same-source saved query / restore / cancel"})
		}
		return nil, topics
	}
	actions := m.paletteActions()
	for i := range actions {
		if actions[i].ID == "inspect" && m.focus != 1 {
			actions[i].Keys = nil // Enter belongs to the workspace/reader here.
		}
	}
	topics = append(topics,
		terminalui.HelpTopic{Group: "Navigation", Keys: "Tab / Shift+Tab", Description: "Cycle visible panes, destinations, disclosures and action controls"},
		terminalui.HelpTopic{Group: "Navigation", Keys: "Row / triangle / action click", Description: "Select only / disclose / activate; wheel scrolls the pointed body"},
		terminalui.HelpTopic{Group: "Navigation", Keys: "j/k / arrows / g/G / Home/End", Description: "Choose current pane item; PgUp/PgDn page or scroll reader"},
		terminalui.HelpTopic{Group: "Navigation", Keys: "Ctrl+P / F1 / ?", Description: "Actions / contextual Help (only outside text owners)"})
	if m.focus == 2 {
		topics = append(topics,
			terminalui.HelpTopic{Group: "Reader", Keys: "Enter", Description: "Expand/collapse selected field or full wrapped value"},
			terminalui.HelpTopic{Group: "Reader", Keys: "y / Y", Description: "Copy full safe value / JSON-pointer=value; incomplete events refuse copy"})
	} else if m.focus == 0 {
		topics = append(topics, terminalui.HelpTopic{Group: "Workspace", Keys: "Enter", Description: "Open selected workspace destination or saved search"})
	}
	return actions, topics
}

func (m Model) helpFrame() terminalui.Frame {
	if !m.workspaceFits() {
		return terminalui.Frame{}
	}
	actions, topics := m.helpContent()
	w, h := min(104, m.width), min(30, m.height)
	return terminalui.RenderHelp(actions, topics, m.helpOffset, terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}, m.options.Theme.Interactions())
}

func (m Model) handleHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	actions, topics := m.helpContent()
	result := terminalui.HandleHelp(msg, actions, topics, &m.helpOffset, m.helpFrame())
	if result.Intent != nil && result.Intent.Kind == terminalui.Dismiss {
		m.help = false
	}
	return m, nil
}

func (m Model) helpView() string {
	parent := m
	parent.help = false
	base := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: strings.Split(parent.View(), "\n")}
	return strings.Join(terminalui.Overlay(base, m.helpFrame(), m.options.Theme.Interactions()).Lines, "\n")
}

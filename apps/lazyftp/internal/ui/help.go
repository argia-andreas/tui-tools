package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

// Help owns presentation only; modal, palette, filter and operation state stay
// in place so lifecycle results can keep updating their actual owner.
func (m Model) helpActions() []terminalui.Action {
	if m.modal != "" || m.rename != nil || m.filtering {
		return nil
	}
	return m.actions()
}

func (m Model) helpTopics() []terminalui.HelpTopic {
	topics := []terminalui.HelpTopic{
		{Group: "Help", Keys: "F1 / Esc / Close", Description: "Return to the unchanged parent view"},
	}
	switch {
	case m.rename != nil:
		if m.rename.running {
			topics = append(topics, terminalui.HelpTopic{Group: "Rename", Keys: "Wait", Description: "A started rename must finish; cancellation cannot roll it back"})
		} else if m.rename.confirming {
			topics = append(topics, terminalui.HelpTopic{Group: "Rename", Keys: "Tab / Left / Right / Enter", Description: "Choose Cancel or Rename / request the focused action; Up/Down scroll the review"})
		} else {
			topics = append(topics, terminalui.HelpTopic{Group: "Rename", Keys: "Text / Enter", Description: "Edit the filename / review the draft with Cancel initially selected"})
		}
		topics = append(topics, terminalui.HelpTopic{Group: "Rename", Keys: "Esc", Description: "Cancel before execution; a started rename cannot be rolled back"})
	case m.modal == "palette":
		topics = append(topics,
			terminalui.HelpTopic{Group: "Palette", Keys: "Type / Ctrl+U", Description: "Search actions / clear query"},
			terminalui.HelpTopic{Group: "Palette", Keys: "Arrows / Tab", Description: "Choose a result without wrapping"},
			terminalui.HelpTopic{Group: "Palette", Keys: "Enter / click", Description: "Run an available result; action shortcuts apply in the workspace"},
			terminalui.HelpTopic{Group: "Palette", Keys: "Ctrl+P / Esc", Description: "Close palette; outside press closes without click-through"})
	case m.filtering:
		topics = append(topics,
			terminalui.HelpTopic{Group: "Filter", Keys: "Type / Backspace", Description: "Edit the current pane filter; printable keys are text"},
			terminalui.HelpTopic{Group: "Filter", Keys: "Enter / Esc", Description: "Keep filter / clear filter and return"})
	case m.modal == "folder":
		topics = append(topics, terminalui.HelpTopic{Group: "Folder", Keys: "Tab / Enter / Esc", Description: "Choose field or action / open checked folder / cancel"})
	case m.modal == "preview":
		topics = append(topics,
			terminalui.HelpTopic{Group: "Preview", Keys: "Arrows / PgUp/PgDown", Description: "Scroll bounded content while its body is focused; Enter there does not activate"},
			terminalui.HelpTopic{Group: "Preview", Keys: "Tab / Shift+Tab / Esc", Description: "Move between body and Close / close and cancel pending read"})
	case m.modal == "rename":
		topics = append(topics, terminalui.HelpTopic{Group: "Destination name", Keys: "Text / Enter / Esc", Description: "Edit destination / check name / cancel the pending copy"})
	case m.modal != "":
		topics = append(topics,
			terminalui.HelpTopic{Group: "Review", Keys: "Tab / arrows / Enter", Description: "Choose and request an available action; Cancel is initially selected"},
			terminalui.HelpTopic{Group: "Review", Keys: "Esc", Description: "Cancel this review when permitted; started work may still finish"})
	default:
		topics = append(topics,
			terminalui.HelpTopic{Group: "Browse & select", Keys: "Tab / Shift+Tab", Description: "Focus next / previous visible pane or control"},
			terminalui.HelpTopic{Group: "Browse & select", Keys: "Ctrl+H/L", Description: "Focus left/right file pane"},
			terminalui.HelpTopic{Group: "Browse & select", Keys: "Arrows / j/k", Description: "Move the cursor in the focused pane"},
			terminalui.HelpTopic{Group: "Browse & select", Keys: "Enter / Open/Inspect", Description: "Open directory / inspect file; row click highlights only"},
			terminalui.HelpTopic{Group: "Browse & select", Keys: "Space / marker click", Description: "Toggle file selection without opening"},
			terminalui.HelpTopic{Group: "Browse & select", Keys: "Backspace / h", Description: "Parent directory"},
			terminalui.HelpTopic{Group: "Browse & select", Keys: "/", Description: "Filter current pane"})
	}
	return topics
}

func (m Model) helpFrame() terminalui.Frame {
	if !m.workspaceFits() {
		return terminalui.Frame{}
	}
	w, h := min(82, m.width-6), min(20, m.height-4)
	return terminalui.RenderHelp(m.helpActions(), m.helpTopics(), m.helpOffset,
		terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}, m.theme.TerminalStyle(false))
}

func (m Model) helpInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	r := terminalui.HandleHelp(msg, m.helpActions(), m.helpTopics(), &m.helpOffset, m.helpFrame())
	if r.Intent != nil && r.Intent.Kind == terminalui.Dismiss {
		m.help = false
	}
	return m, nil
}

func (m Model) helpView() string {
	parent := m
	parent.help = false
	base := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: strings.Split(parent.View(), "\n")}
	return strings.Join(terminalui.Overlay(base, m.helpFrame(), m.theme.TerminalStyle(false)).Lines, "\n")
}

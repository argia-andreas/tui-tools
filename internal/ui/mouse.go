package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.width < 60 || m.height < 12 || msg.X < 0 || msg.Y < 0 || msg.X >= m.width || msg.Y >= m.height {
		return m, nil
	}
	click := msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress
	wheel := msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
	if !click && !wheel {
		return m, nil
	}
	key := func(kind tea.KeyType) (tea.Model, tea.Cmd) {
		return m.Update(tea.KeyMsg{Type: kind})
	}
	if m.modal != "" {
		w, h := m.modalSize()
		x, y := (m.width-w)/2, (m.height-h)/2
		outside := msg.X < x || msg.X >= x+w || msg.Y < y || msg.Y >= y+h
		if click && (outside || (msg.Y == y && msg.X >= x+w-6)) {
			if m.modal == "copy" || m.modal == "rename" || m.modal == "batch" || m.modal == "preview" || m.modal == "folder" {
				return key(tea.KeyEsc)
			}
			m.modal = ""
			m.pending = nil
			return m, nil
		}
		if outside {
			return m, nil
		}
		if m.modal == "folder" {
			if click && msg.Y == y+h-2 && msg.X >= x+2 && msg.X < x+12 {
				return key(tea.KeyEnter)
			}
			return m, nil
		}
		if m.modal == "batch" {
			if click && msg.Y == y+h-2 {
				if msg.X >= x+2 && msg.X < x+12 {
					m.choice = 0
					return key(tea.KeyEnter)
				}
				if !m.queuePlanning && msg.X >= x+12 && msg.X < x+27 {
					m.choice = 1
					return key(tea.KeyEnter)
				}
			}
			return m, nil
		}
		if m.modal == "copy" || m.modal == "rename" {
			if click && m.modal == "copy" && msg.Y == y+h-2 {
				start := x + 2
				for i, label := range m.copyButtons() {
					if msg.X >= start && msg.X < start+len(label)+4 {
						m.choice = i
						return key(tea.KeyEnter)
					}
					start += len(label) + 4
				}
			}
			return m, nil
		}
		if wheel {
			if msg.Button == tea.MouseButtonWheelUp {
				return key(tea.KeyUp)
			}
			return key(tea.KeyDown)
		}
		if msg.Y == y+h-2 {
			footer := paletteFooter
			if m.modal == "help" {
				footer = helpFooter
			}
			for _, label := range []string{"Enter run", "F1 shortcuts", "Esc close"} {
				i := strings.Index(footer, label)
				if i < 0 {
					continue
				}
				start := x + 1 + ansi.StringWidth(footer[:i])
				if msg.X >= start && msg.X < start+len(label) {
					switch label {
					case "Enter run":
						return key(tea.KeyEnter)
					case "F1 shortcuts":
						return m.execute("help")
					case "Esc close":
						return key(tea.KeyEsc)
					}
				}
			}
		}
		if m.modal == "palette" {
			n := max(1, min(9, h-8))
			start := max(0, m.choice-n+1)
			row := msg.Y - y - 3
			if row >= 0 && row < n && start+row < len(m.matches()) {
				m.choice = start + row
				return key(tea.KeyEnter)
			}
		}
		return m, nil
	}
	if click && msg.Y == 0 {
		controls := m.toolbar()
		x := msg.X - (m.width - ansi.StringWidth(controls))
		for _, button := range []struct{ label, action string }{
			{"[Nav]", "navigation"}, {"[Info]", "inspector"}, {"[Jobs]", "jobs"}, {"[?]", "help"}, {"[Copy]", "copy"}, {"[Preview]", "preview"}, {"[Stop]", "cancel"}, {"[Retry]", "retry"}, {"[Actions]", "palette"},
		} {
			start := strings.Index(controls, button.label)
			if start >= 0 && x >= start && x < start+len(button.label) {
				if button.action == "palette" {
					return key(tea.KeyCtrlP)
				}
				return m.execute(button.action)
			}
		}
		return m, nil
	}
	if click && msg.Y == m.height-1 && m.transferer != nil && m.status == "" && !m.filtering {
		start := strings.Index(workspaceFooter, "? help")
		if msg.X >= start && msg.X < start+len("? help") {
			return m.execute("help")
		}
	}
	l := m.layout()
	if msg.Y < 1 || msg.Y > l.height {
		return m, nil
	}
	m.filtering = false
	if msg.X < l.nav {
		m.focus = 0
		if wheel {
			if msg.Button == tea.MouseButtonWheelUp {
				return key(tea.KeyUp)
			}
			return key(tea.KeyDown)
		}
		for i := range m.locations {
			if msg.Y == m.navY(i) {
				m.navCursor = i
				return key(tea.KeyEnter)
			}
		}
		return m, nil
	}
	if msg.X >= l.nav+l.center {
		m.focus = 3
		return m, nil
	}
	if msg.Y >= 1+l.height-l.drawer {
		m.focus = 4
		if wheel {
			if msg.Button == tea.MouseButtonWheelUp {
				return key(tea.KeyUp)
			}
			return key(tea.KeyDown)
		}
		row := msg.Y - (1 + l.height - l.drawer) - 2
		start := max(0, min(m.jobCursor, len(m.queue)-1)-max(0, l.drawer-5))
		if row >= 0 && row < l.drawer-4 && start+row < len(m.queue) {
			m.jobCursor = start + row
		}
		return m, nil
	}
	i, origin := 0, l.nav
	if msg.X >= l.nav+l.left {
		i, origin = 1, l.nav+l.left
	}
	m.focus, m.endpoint = i+1, i
	if i == 1 && m.disconnected {
		if click {
			connect, folder := destinationRows(l.height - l.drawer)
			w := l.center - l.left
			for choice, button := range []struct {
				row           int
				label, action string
			}{{connect, connectButton, "connect"}, {folder, folderButton, "folder"}} {
				x := origin + 1 + max(0, (w-2-len(button.label))/2)
				if msg.Y == 2+button.row && msg.X >= x && msg.X < x+len(button.label) {
					m.destinationChoice = choice
					return m.execute(button.action)
				}
			}
		}
		return m, nil
	}
	if wheel {
		p := &m.panes[i]
		delta := 3
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -3
		}
		p.cursor = max(0, min(len(p.visible())-1, p.cursor+delta))
		return m, nil
	}
	x := msg.X - origin - 1
	if msg.Y == 2 {
		switch {
		case x >= 1 && x < 5:
			return key(tea.KeyBackspace)
		case x >= 7 && x < 15:
			m.filtering = true
		case x >= 16 && x < 25:
			return m.execute("refresh")
		}
		return m, nil
	}
	p := &m.panes[i]
	available := max(1, l.height-l.drawer-7)
	start := max(0, p.cursor-available+1)
	row := msg.Y - 4
	entries := p.visible()
	paneWidth := l.left
	if i == 1 {
		paneWidth = l.center - l.left
	}
	if x >= 0 && x < paneWidth-2 && row >= 0 && row < available && start+row < len(entries) {
		p.cursor = start + row
		if x < 2 {
			return key(tea.KeySpace)
		}
		return key(tea.KeyEnter)
	}
	return m, nil
}

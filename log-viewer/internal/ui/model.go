package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"logviewer/internal/logs"
	"logviewer/internal/theme"
)

type Options struct {
	Service *logs.Service
	Query   logs.Query
	Theme   theme.Theme
	Context context.Context
}

type Model struct {
	options                               Options
	width, height                         int
	page                                  logs.Page
	loaded                                bool
	loadError                             bool
	selected, top, focus, inspectorOffset int
	detail                                bool
	status                                string
}

type loadedMsg struct {
	page logs.Page
	err  error
}

func New(options Options) Model {
	return Model{options: options, focus: 1, status: "Offline fictional data · no network · no MCP server"}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		page, err := m.options.Service.Search(m.options.Context, m.options.Query)
		return loadedMsg{page, err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if !m.detail && ((m.focus == 0 && m.width < 140) || (m.focus == 2 && m.width < 120)) {
			m.focus = 1
		}
		m.keepVisible()
		m.clampInspector()
	case loadedMsg:
		m.loaded = true
		m.loadError = msg.err != nil
		m.page = msg.page
		if m.loadError {
			m.status = "Source read failed · no event data retained"
		} else {
			for i, e := range m.page.Events {
				if e.Text("level") == "ERROR" {
					m.selected = i
					break
				}
			}
			m.keepVisible()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.detail, m.focus = false, 1
		case "tab", "shift+tab":
			if m.detail {
				break
			}
			panels := []int{1}
			if m.width >= 140 {
				panels = []int{0, 1}
			}
			if m.width >= 120 {
				panels = append(panels, 2)
			}
			for i, panel := range panels {
				if panel == m.focus {
					delta := 1
					if msg.String() == "shift+tab" {
						delta = len(panels) - 1
					}
					m.focus = panels[(i+delta)%len(panels)]
					break
				}
			}
		case "enter":
			if m.focus == 0 {
				m.focus = 1
			} else if len(m.page.Events) > 0 {
				m.focus = 2
				m.detail = m.width < 120
			}
		case "j", "down", "k", "up", "pgdown", "pgup", "g", "G", "home", "end":
			step := 1
			if msg.String() == "k" || msg.String() == "up" {
				step = -1
			}
			if msg.String() == "pgdown" {
				step = max(1, m.height-12)
			}
			if msg.String() == "pgup" {
				step = -max(1, m.height-12)
			}
			if m.focus == 1 {
				m.selected += step
				if msg.String() == "g" || msg.String() == "home" {
					m.selected = 0
				}
				if msg.String() == "G" || msg.String() == "end" {
					m.selected = len(m.page.Events) - 1
				}
				m.selected = max(0, min(m.selected, len(m.page.Events)-1))
				m.inspectorOffset = 0
				m.keepVisible()
			} else if m.focus == 2 {
				m.inspectorOffset += step
				if msg.String() == "g" || msg.String() == "home" {
					m.inspectorOffset = 0
				}
				if msg.String() == "G" || msg.String() == "end" {
					m.inspectorOffset = 1 << 30
				}
				m.clampInspector()
			}
		case "ctrl+p", "y", "Y", " ", "/", "f", "?":
			feature := map[string]string{"ctrl+p": "Command palette", "y": "Field yank", "Y": "Field yank", " ": "Live follow", "/": "Local find", "f": "Filters", "?": "Command help"}[msg.String()]
			m.status = fmt.Sprintf("%s is not available in this fixture slice · navigation only", feature)
		}
	}
	return m, nil
}

func (m *Model) keepVisible() {
	visible := max(1, m.height-13)
	if m.selected < m.top {
		m.top = m.selected
	}
	if m.selected >= m.top+visible {
		m.top = m.selected - visible + 1
	}
	m.top = min(m.top, max(0, len(m.page.Events)-visible))
}

func (m Model) inspectorWidth() int {
	if m.detail {
		return max(1, m.width-6)
	}
	return min(50, m.width/3) - 6
}

func (m *Model) clampInspector() {
	m.inspectorOffset = max(0, min(m.inspectorOffset, max(0, len(m.inspectorLines(m.inspectorWidth()))-max(1, m.height-12))))
}

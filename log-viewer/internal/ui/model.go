package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"logviewer/internal/logs"
	"logviewer/internal/theme"
)

type Options struct {
	Service      *logs.Service
	Query        logs.Query
	Theme        theme.Theme
	Context      context.Context
	LocalLab     bool
	SearchesPath string
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
	generation                            uint64
	cancel                                context.CancelFunc
	requestContext                        context.Context
	loading                               bool
	budgetStopped                         bool
	editor                                *editor
	suggestionGeneration                  uint64
	suggestionCancel                      context.CancelFunc
	find                                  string
	findCase                              bool
}

type loadedMsg struct {
	page       logs.Page
	err        error
	generation uint64
	older      bool
}

func New(options Options) Model {
	status := "Offline fictional data · no network · no MCP server"
	if options.LocalLab {
		status = "Local Elasticsearch · TLS · read-only key · seeded fictional data · no follow"
	}
	if options.Context == nil {
		options.Context = context.Background()
	}
	if options.Query.Limits.Events == 0 {
		options.Query.Limits.Events = 500
	}
	ctx, cancel := context.WithCancel(options.Context)
	return Model{options: options, focus: 1, status: status, generation: 1, requestContext: ctx, cancel: cancel, loading: true}
}

func (m Model) Init() tea.Cmd {
	return m.searchCommand(false)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if !m.detail && ((m.focus == 0 && m.width < 140) || (m.focus == 2 && m.width < 120)) {
			m.focus = 1
		}
		if m.editor != nil && !m.detail && ((m.editor.returnFocus == 0 && m.width < 140) || (m.editor.returnFocus == 2 && m.width < 120)) {
			m.editor.returnFocus = 1
		}
		m.keepVisible()
		m.clampInspector()
	case loadedMsg:
		return m.receivePage(msg)
	case suggestionsMsg:
		if m.editor != nil && m.editor.kind == "filter" && msg.generation == m.suggestionGeneration {
			m.editor.suggestions = msg.values
			m.editor.suggestion = 0
			m.editor.suggestionError = msg.err != nil
			m.editor.suggestionLoading = false
		}
	case tea.KeyMsg:
		if m.editor != nil {
			return m.edit(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Sequence(m.closeCursor(m.page.Next), tea.Quit)
		case "f":
			if m.focus == 2 {
				return m.openFieldPicker()
			}
			return m.openFilters()
		case "/":
			m.editor = &editor{kind: "find", text: m.find, caseSensitive: m.findCase, returnFocus: m.focus}
		case "n", "N":
			m.findNext(msg.String() == "N")
		case "o":
			if m.loading {
				m.status = "Read already in progress"
				break
			}
			if m.budgetStopped {
				m.status = "History stopped: session budget 10000 events / 32 MiB"
				break
			}
			if m.page.Next == "" {
				m.status = "No older history available"
				break
			}
			m.loading = true
			return m, m.searchCommand(true)
		case "r":
			return m.refresh()
		case "S":
			if m.options.SearchesPath == "" {
				m.status = "Saved searches unavailable: no explicit --searches path"
				break
			}
			m.editor = &editor{kind: "save", returnFocus: m.focus}
		case "R":
			return m.openRestore()
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
		case "ctrl+p", "y", "Y", " ", "?":
			feature := map[string]string{"ctrl+p": "Command palette", "y": "Field yank", "Y": "Field yank", " ": "Live follow", "?": "Command help"}[msg.String()]
			m.status = fmt.Sprintf("%s is not available in this slice · navigation only", feature)
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

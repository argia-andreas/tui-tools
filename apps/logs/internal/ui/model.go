package ui

import (
	"context"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"

	"logviewer/internal/config"
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
	TrustOSC52   bool
	SettingsPath string
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
	palette                               terminalui.Palette
	help                                  bool
	helpOffset                            int
	suggestionGeneration                  uint64
	suggestionCancel                      context.CancelFunc
	find                                  string
	findCase                              bool
	saved                                 []config.SavedSearch
	savedError                            bool
	workspaceRow                          int
	controlFocus                          string
	fieldIndex                            int
	collapsed, expanded                   map[string]bool
	settings                              config.Settings
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
	m := Model{options: options, focus: 1, status: status, generation: 1, requestContext: ctx, cancel: cancel, loading: true}
	m.settings = config.DefaultSettings()
	if options.SettingsPath != "" {
		settings, err := config.LoadSettings(options.SettingsPath)
		if err != nil {
			m.status = "Settings unavailable · using defaults · check private settings file"
		} else {
			m.settings = settings
		}
	}
	m.reloadSaved()
	return m
}

func (m Model) Init() tea.Cmd {
	return m.searchCommand(false)
}

func (m Model) workspaceFits() bool {
	return m.width >= 60 && m.height >= 16
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
		m.revealField()
		m.controlFocus = ""
	case loadedMsg:
		return m.receivePage(msg)
	case copiedMsg:
		m.status = msg.status
	case suggestionsMsg:
		if m.editor != nil && m.editor.kind == "filter" && msg.generation == m.suggestionGeneration {
			m.editor.suggestions = msg.values
			m.editor.suggestion = 0
			m.editor.suggestionError = msg.err != nil
			m.editor.suggestionLoading = false
		}
	case tea.MouseMsg:
		if m.help {
			return m.handleHelp(msg)
		}
		if !m.workspaceFits() {
			return m, nil
		}
		result := terminalui.HandleActionBar(msg, m.headerActions(), m.actionBar(), "")
		if result.Intent != nil && (m.editor == nil || m.editor.kind != "palette") {
			return m.execute(result.Intent.ID)
		}
		if result.Consumed && (m.editor == nil || m.editor.kind != "palette") {
			return m, nil
		}
		if m.editor != nil {
			if m.editor.kind == "palette" {
				return m.editPalette(msg)
			}
			return m.editorInput(msg)
		}
		return m.navigationInput(msg)
	case tea.KeyMsg:
		if m.help {
			return m.handleHelp(msg)
		}
		if !m.workspaceFits() {
			// The resize notice replaces editors and controls. Keep their drafts,
			// but allow only safe cancellation or the existing workspace quit.
			if msg.String() == "esc" {
				if m.editor != nil {
					if m.editor.kind == "palette" {
						return m.editPalette(msg)
					}
					return m.edit(msg)
				}
				return m.execute("return")
			}
			if m.editor == nil && (msg.String() == "q" || msg.String() == "ctrl+c") {
				return m.execute("quit")
			}
			return m, nil
		}
		if msg.String() == "f1" || (msg.String() == "?" && m.editor == nil) {
			return m.execute("help")
		}
		if m.editor != nil {
			if m.editor.kind == "palette" {
				return m.editPalette(msg)
			}
			return m.editorInput(msg)
		}
		if msg.String() == "tab" || msg.String() == "shift+tab" || msg.String() == "enter" || (msg.String() == " " && m.controlFocus != "") {
			return m.navigationInput(msg)
		}
		m.controlFocus = ""
		// Enter and Escape retain their contextual navigation meaning; palette
		// and button requests use explicit inspect/return IDs instead.
		if key := msg.String(); key != "enter" && key != "esc" {
			for _, action := range m.paletteActions() {
				if slices.Contains(action.Keys, key) {
					return m.execute(action.ID)
				}
			}
		}
		switch msg.String() {
		case "ctrl+p":
			return m.execute("palette")
		case "esc":
			return m.execute("return")
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
			if m.focus == 0 {
				m.workspaceRow += step
				if msg.String() == "g" || msg.String() == "home" {
					m.workspaceRow = 0
				}
				if msg.String() == "G" || msg.String() == "end" {
					m.workspaceRow = len(m.saved) + 1
				}
				m.workspaceRow = max(0, min(m.workspaceRow, len(m.saved)+1))
			} else if m.focus == 1 {
				m.selected += step
				if msg.String() == "g" || msg.String() == "home" {
					m.selected = 0
				}
				if msg.String() == "G" || msg.String() == "end" {
					m.selected = len(m.page.Events) - 1
				}
				m.selected = max(0, min(m.selected, len(m.page.Events)-1))
				m.inspectorOffset = 0
				m.fieldIndex = 0
				m.keepVisible()
			} else if m.focus == 2 {
				if msg.String() == "pgdown" || msg.String() == "pgup" {
					m.inspectorOffset += step
					m.clampInspector()
					_, owners := m.inspectorRows(m.inspectorWidth())
					visible := owners[m.inspectorOffset:min(len(owners), m.inspectorOffset+max(1, m.height-12))]
					if !slices.Contains(visible, m.fieldIndex) {
						m.fieldIndex = max(0, owners[m.inspectorOffset])
					}
					break
				}
				m.fieldIndex += step
				if msg.String() == "g" || msg.String() == "home" {
					m.fieldIndex = 0
				}
				if msg.String() == "G" || msg.String() == "end" {
					m.fieldIndex = len(m.inspectorFields()) - 1
				}
				m.revealField()
			}
		case "y", "Y":
			return m.yank(msg.String() == "Y")
		case " ":
			m.status = "Live follow is not available in this slice · navigation only"
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

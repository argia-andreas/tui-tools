package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/connections"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

// workspace owns a single program: the file model continues receiving async
// results under the picker, but only the visible model receives user input.
type workspace struct {
	model                tea.Model
	ctx                  context.Context
	theme                theme.Theme
	bookmarkPath, config string
	persistentConfig     string
	sessions             *connectionSessions
	wizard               *connectionWizard
	leftOverride         string
	extra                []storage.Location
	picker               *picker
	size                 tea.WindowSizeMsg
	request              int
	connectOnStart       string
	pickerDestination    bool
	discoveryCancel      context.CancelFunc
}

type connectionsLoaded struct {
	request int
	picker  picker
}

type connectionTick int

func tickConnection(request int) tea.Cmd {
	return func() tea.Msg { return connectionTick(request) }
}

func (m workspace) Init() tea.Cmd {
	if m.connectOnStart != "" {
		return tea.Batch(m.model.Init(), func() tea.Msg {
			return ui.ConnectRequested{Local: m.connectOnStart}
		})
	}
	return m.model.Init()
}

func (m workspace) String() string   { return m.View() }
func (m workspace) GoString() string { return m.View() }

func (m workspace) View() string {
	if m.wizard != nil {
		return m.wizardOverlay()
	}
	if m.picker != nil {
		return m.picker.View()
	}
	return m.model.View()
}

func (m *workspace) closePicker() {
	if m.wizard != nil {
		m.wizard.invalidate()
		m.wizard = nil
	}
	if m.discoveryCancel != nil {
		m.discoveryCancel()
		m.discoveryCancel = nil
	}
	m.request++ // Ignore results from canceled or superseded discovery.
	m.picker = nil
}

func (m workspace) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case connectionTick:
		if m.picker == nil || m.wizard != nil || int(msg) != m.request {
			return m, nil
		}
		m.picker.frame++
		request := m.request
		return m, tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return connectionTick(request) })
	case wizardTested:
		if m.wizard == nil || msg.owner != m.wizard || msg.request != m.wizard.request {
			if msg.candidate != nil {
				return m, func() tea.Msg { m.sessions.release(msg.candidate); return nil }
			}
			return m, nil
		}
		w := m.wizard
		w.busy, w.cancel = false, nil
		if msg.err != nil {
			w.message = msg.err.Error()
			return m, nil
		}
		w.candidate, w.verified = msg.candidate, true
		w.message = "Listing verified; write access not tested."
		w.focus = 3
		return m, nil
	case wizardConnected:
		if m.wizard == nil || msg.owner != m.wizard || msg.request != m.wizard.request {
			return m, nil
		}
		w := m.wizard
		w.busy, w.cancel = false, nil
		if msg.err != nil {
			w.message = msg.err.Error()
			return m, nil
		}
		left := storage.Location{Path: w.local}
		right := storage.Location{Remote: msg.candidate.Remote, Path: w.root()}
		m.config = msg.candidate.ConfigPath
		backend := storage.NewRclone(m.config)
		locations := []storage.Location{right}
		for _, loc := range m.extra {
			if loc.Remote != "" && loc != right {
				locations = append(locations, loc)
			}
		}
		locations = append(locations, left)
		m.extra = locations
		w.candidate = nil // Now owned by the active workspace until program exit.
		m.closePicker()
		var cmd tea.Cmd
		m.model, cmd = m.model.Update(ui.ConnectionOpened{Left: left, Right: right, Locations: locations, Browser: backend, Transferer: backend})
		return m, cmd
	case ui.ConnectRequested:
		m.closePicker()
		m.pickerDestination = msg.Destination
		p := newPicker(nil, nil, msg.Local, m.leftOverride, m.bookmarkPath, m.theme)
		p.loading = true
		if m.size.Width > 0 {
			p.width, p.height = m.size.Width, m.size.Height
		}
		visible := p // The async command must not mutate the visible picker.
		m.picker = &visible
		ctx, cancel := context.WithCancel(m.ctx)
		m.discoveryCancel = cancel
		request, config := m.request, m.config
		discover := func() tea.Msg {
			var messages []string
			if p.bookmarkPath == "" {
				var err error
				p.bookmarkPath, err = connections.DefaultPath()
				if err != nil {
					messages = append(messages, "Cannot locate bookmarks: "+err.Error())
				}
			}
			saved, err := connections.Load(p.bookmarkPath)
			if err != nil {
				messages = append(messages, err.Error())
			}
			var names []string
			if _, err := exec.LookPath("rclone"); err != nil {
				p.connectError = "Install rclone (or run ./.agents/setup), then connect again."
				messages = append(messages, p.connectError)
			} else {
				names, err = connections.Discover(ctx, config)
				if err != nil {
					messages = append(messages, err.Error())
				}
			}
			loaded := newPicker(names, saved, p.local, p.leftOverride, p.bookmarkPath, p.theme)
			loaded.message, loaded.connectError = strings.Join(messages, "; "), p.connectError
			return connectionsLoaded{request: request, picker: loaded}
		}
		return m, tea.Batch(discover, tickConnection(request))
	case connectionsLoaded:
		if m.picker != nil && msg.request == m.request {
			p := msg.picker
			p.width, p.height = m.picker.width, m.picker.height
			p.frame = m.picker.frame
			p.help = m.picker.help
			if m.picker.navigated {
				// Discovery must not steal an action the user already chose.
				p.focus, p.navigated = m.picker.focus, true
			}
			m.picker = &p
			if m.discoveryCancel != nil {
				m.discoveryCancel()
				m.discoveryCancel = nil
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.size = msg
		if m.wizard != nil {
			m.wizard.width, m.wizard.height = msg.Width, msg.Height
		}
		if m.picker != nil {
			m.picker.width, m.picker.height = msg.Width, msg.Height
		}
	case tea.KeyMsg:
		if m.wizard != nil {
			if msg.String() == "ctrl+c" {
				m.closePicker()
				return m, tea.Quit
			}
			if msg.String() == "esc" {
				m.wizard.back()
				if m.wizard.dismissed {
					m.wizard = nil
					return m, tickConnection(m.request)
				}
				return m, nil
			}
			return m.updateWizard(msg)
		}
		if m.picker != nil {
			if msg.String() == "ctrl+c" {
				m.closePicker()
				return m, tea.Quit
			}
			return m.updatePicker(msg)
		}
	case tea.MouseMsg:
		if m.wizard != nil {
			return m.updateWizard(msg)
		}
		if m.picker != nil {
			return m.updatePicker(msg)
		}
	}
	var cmd tea.Cmd
	m.model, cmd = m.model.Update(msg)
	return m, cmd
}

func (m workspace) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.wizard.input(msg)
	if m.wizard.dismissed {
		m.wizard = nil
		return m, tea.Batch(cmd, tickConnection(m.request))
	}
	return m, cmd
}

func (m workspace) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.picker.Update(msg)
	p := next.(picker)
	m.picker = &p
	if p.localFiles {
		m.closePicker()
		if m.pickerDestination {
			m.model, cmd = m.model.Update(ui.OpenLocalFolderRequested{})
			return m, cmd
		}
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" && cmd != nil {
		// The chooser's standalone Quit means return to files, not exit the app.
		// Details and help handle Back themselves without returning a command.
		m.closePicker()
		return m, nil
	}
	if p.newConnection {
		if m.discoveryCancel != nil {
			m.discoveryCancel()
			m.discoveryCancel = nil
		}
		m.request++
		p.newConnection = false
		if m.sessions == nil {
			m.sessions = &connectionSessions{}
		}
		m.wizard = newWizard(m.ctx, m.theme, p.local, m.config, m.persistentConfig, m.sessions, m.request)
		m.wizard.width, m.wizard.height = p.width, p.height
		return m, nil
	}
	if p.selected == nil {
		return m, cmd
	}
	choice := p.selected
	left := storage.Location{Path: choice.Local}
	right := storage.Location{Remote: choice.Remote, Path: choice.Root}
	locations := []storage.Location{right}
	for _, loc := range m.extra {
		if loc.Remote != "" && loc != right {
			locations = append(locations, loc)
		}
	}
	locations = append(locations, left)
	m.closePicker()
	// Discard the standalone picker's Quit command on successful selection.
	m.model, cmd = m.model.Update(ui.ConnectionOpened{Left: left, Right: right, Locations: locations})
	return m, cmd
}

func (m workspace) wizardOverlay() string {
	w := m.wizard
	panel := w.View()
	rows := strings.Split(panel, "\n")
	x := max(0, (w.width-lipgloss.Width(panel))/2)
	y := max(0, (w.height-len(rows))/2)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Muted)).Background(lipgloss.Color(m.theme.Background))
	lines := make([]string, w.height)
	for i := range lines {
		base := strings.Repeat(" ", w.width)
		if i == 0 {
			base = ansi.Truncate("  LAZYFTP"+base, w.width, "")
		} else if i == 1 || i == w.height-2 {
			base = strings.Repeat("─", w.width)
		}
		if i >= y && i-y < len(rows) {
			row := rows[i-y]
			lines[i] = dim.Render(ansi.Cut(base, 0, x)) + row + dim.Render(ansi.Cut(base, x+lipgloss.Width(row), w.width))
		} else {
			lines[i] = dim.Render(base)
		}
	}
	return strings.Join(lines, "\n")
}

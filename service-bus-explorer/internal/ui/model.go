package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/theme"
)

type item struct {
	resource explorer.Resource
	counts   explorer.Counts
}

type loaded struct {
	items []item
	err   error
}
type peeked struct {
	generation uint64
	page       explorer.Page
	err        error
}

// Model is an offline-first workspace. Network/fixture reads run as commands.
type Model struct {
	store                                    explorer.Store
	mode                                     string
	life                                     context.Context
	stop                                     context.CancelFunc
	theme                                    theme.Theme
	width, height                            int
	items                                    []item
	screen, returnScreen                     string
	selected, nav, message                   int
	sidebar, navFocus, table, dlq, inspector bool
	page                                     explorer.Page
	resource                                 explorer.Resource
	from                                     int64
	history                                  []int64
	generation                               uint64
	cancel                                   context.CancelFunc
	loading                                  bool
	status                                   string
}

func New(store explorer.Store, mode string) Model {
	t, _ := theme.Get("")
	ctx, cancel := context.WithCancel(context.Background())
	return Model{store: store, mode: mode, life: ctx, stop: cancel, theme: t, width: 160, height: 48, screen: "dashboard", sidebar: true, inspector: true, loading: true, status: "Discovering resources…"}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
		defer cancel()
		resources, err := m.store.Discover(ctx)
		if err != nil {
			return loaded{err: err}
		}
		var items []item
		for _, r := range resources {
			if r.Queue == "" && r.Subscription == "" {
				continue
			}
			c, err := m.store.Counts(ctx, r)
			if err != nil {
				c.State = "unavailable"
				c.Detail = err.Error()
			}
			items = append(items, item{r, c})
		}
		return loaded{items: items}
	}
}

func (m Model) visible() []item {
	var result []item
	for _, i := range m.items {
		if m.screen == "queues" && i.resource.Queue == "" {
			continue
		}
		if m.screen == "topics" && i.resource.Subscription == "" {
			continue
		}
		result = append(result, i)
	}
	return result
}

func (m Model) peek(from int64) (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	m.generation++
	m.from, m.message, m.loading = from, 0, true
	m.page = explorer.Page{}
	ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
	m.cancel = cancel
	store, r, dlq, generation := m.store, m.resource, m.dlq, m.generation
	return m, func() tea.Msg {
		defer cancel()
		p, err := store.Peek(ctx, r, dlq, from, 20)
		return peeked{generation, p, err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.width < 110 {
			m.navFocus = false
		}
	case loaded:
		m.loading = false
		if msg.err != nil {
			m.status = "Load failed: " + msg.err.Error()
			break
		}
		m.items = msg.items
		m.status = "Offline sample data · browsing never receives or removes a message"
		if m.mode == "emulator" {
			m.status = "Emulator connected · runtime counts unavailable (emulator limitation) · peek only"
		}
	case peeked:
		if msg.generation != m.generation {
			break
		}
		m.loading = false
		if msg.err != nil {
			m.status = "Peek failed: " + msg.err.Error()
			break
		}
		m.page = msg.page
		m.status = "PEEK ONLY · no locks acquired · no messages removed"
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || key == "q" {
			m.stop()
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if key == "b" {
			m.sidebar = !m.sidebar
			m.navFocus = false
			return m, nil
		}
		if key == "tab" || key == "shift+tab" {
			if m.sidebar && m.width >= 110 {
				m.navFocus = !m.navFocus
			}
			return m, nil
		}
		if key == "esc" && m.screen != "dashboard" {
			if m.cancel != nil {
				m.cancel()
			}
			m.generation++
			if m.screen == "messages" {
				m.screen = m.returnScreen
			} else {
				m.screen = "dashboard"
				m.selected = 0
				m.nav = 0
			}
			m.navFocus = false
			m.loading = false
			return m, nil
		}
		if m.navFocus {
			switch key {
			case "j", "down":
				m.nav = min(3, m.nav+1)
			case "k", "up":
				m.nav = max(0, m.nav-1)
			case "enter":
				if m.cancel != nil {
					m.cancel()
				}
				m.generation++
				m.screen = []string{"dashboard", "queues", "topics", "settings"}[m.nav]
				m.selected, m.message, m.navFocus, m.loading = 0, 0, false, false
			}
			return m, nil
		}
		if m.screen == "messages" {
			switch key {
			case "j", "down":
				m.message = min(max(0, len(m.page.Messages)-1), m.message+1)
			case "k", "up":
				m.message = max(0, m.message-1)
			case "i":
				m.inspector = !m.inspector
			case "[", "]":
				dlq := key == "]"
				if dlq != m.dlq {
					m.dlq = dlq
					m.history = nil
					return m.peek(0)
				}
			case "r":
				return m.peek(m.from)
			case "n":
				if m.page.NextSequence != nil {
					m.history = append(m.history, m.from)
					return m.peek(*m.page.NextSequence)
				}
			case "p":
				if len(m.history) > 0 {
					from := m.history[len(m.history)-1]
					m.history = m.history[:len(m.history)-1]
					return m.peek(from)
				}
			}
			return m, nil
		}
		if m.screen == "settings" {
			return m, nil
		}
		items := m.visible()
		switch key {
		case "g":
			m.table = false
		case "t":
			m.table = true
		case "j", "down":
			m.selected = min(max(0, len(items)-1), m.selected+1)
		case "k", "up":
			m.selected = max(0, m.selected-1)
		case "enter", "]":
			if len(items) > 0 {
				m.resource = items[m.selected].resource
				m.returnScreen, m.screen, m.dlq = m.screen, "messages", key == "]"
				m.history = nil
				return m.peek(0)
			}
		}
	}
	return m, nil
}

package ui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/theme"
)

type item struct {
	resource explorer.Resource
	counts   explorer.Counts
}

type loaded struct {
	resources []explorer.Resource
	err       error
}
type peeked struct {
	generation uint64
	page       explorer.Page
	err        error
}
type counted map[explorer.Resource]explorer.Counts
type countTick uint64
type saved struct {
	revision uint64
	err      error
}

type Options struct {
	Mode, ConfigPath string
	Preferences      config.Preferences
	ASCII            bool
}

// Model is an offline-first workspace. Broker reads and disk writes are commands.
type Model struct {
	store                explorer.Store
	mode, configPath     string
	life                 context.Context
	stop                 context.CancelFunc
	theme                theme.Theme
	prefs                config.Preferences
	ascii                bool
	width, height        int
	items                []item
	screen, returnScreen string
	focus                string
	selected, nav        int
	message, offset      int
	setting              int
	dlq                  bool
	page                 explorer.Page
	resource             explorer.Resource
	from                 int64
	history              []int64
	generation           uint64
	cancel               context.CancelFunc
	loading              bool
	refreshing, again    bool
	tick                 uint64
	revision, persisted  uint64
	saving               bool
	palette              bool
	query                string
	action               int
	collapsed            map[branch]bool
	status               string
}

func New(store explorer.Store, options Options) Model {
	t, _ := theme.Get(options.Preferences.Theme)
	ctx, cancel := context.WithCancel(context.Background())
	return Model{store: store, mode: options.Mode, configPath: options.ConfigPath,
		prefs: options.Preferences, ascii: options.ASCII, life: ctx, stop: cancel,
		theme: t, width: 160, height: 48, screen: "dashboard", focus: "content",
		collapsed: make(map[branch]bool), loading: true, status: "Discovering resources…"}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
		defer cancel()
		resources, err := m.store.Discover(ctx)
		return loaded{resources, err}
	}
}

func (m Model) watched(r explorer.Resource) bool {
	resources, explicit := m.prefs.Watchlists[r.Profile]
	if !explicit {
		return true
	}
	for _, candidate := range resources {
		if candidate == r {
			return true
		}
	}
	return false
}

func (m Model) visible() []item {
	var result []item
	for _, i := range m.items {
		if m.screen == "dashboard" && !m.watched(i.resource) ||
			m.screen == "queues" && i.resource.Queue == "" ||
			m.screen == "topics" && i.resource.Subscription == "" {
			continue
		}
		result = append(result, i)
	}
	return result
}

// One bounded sweep, at most four reads in flight. No selection/page state is
// captured, so late count results cannot navigate or replace a message page.
func (m Model) refresh() (tea.Model, tea.Cmd) {
	if m.refreshing {
		return m, nil
	}
	m.refreshing = true
	var targets []explorer.Resource
	for _, i := range m.items {
		if m.watched(i.resource) {
			targets = append(targets, i.resource)
		}
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
		defer cancel()
		jobs := make(chan explorer.Resource, len(targets))
		results := make(chan item, len(targets))
		for _, r := range targets {
			jobs <- r
		}
		close(jobs)
		var workers sync.WaitGroup
		for range min(4, len(targets)) {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for r := range jobs {
					c, err := m.store.Counts(ctx, r)
					if err != nil {
						c.State, c.Detail = "unavailable", err.Error()
					}
					results <- item{r, c}
				}
			}()
		}
		workers.Wait()
		close(results)
		counts := counted{}
		for i := range results {
			counts[i.resource] = i.counts
		}
		return counts
	}
}

func (m Model) peek(from int64) (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	m.generation++
	m.from, m.message, m.offset, m.loading = from, 0, 0, true
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

func (m Model) open(r explorer.Resource, dlq bool) (tea.Model, tea.Cmd) {
	if m.screen != "messages" {
		m.returnScreen = m.screen
	}
	m.resource, m.screen, m.dlq, m.focus = r, "messages", dlq, "content"
	m.history = nil
	return m.peek(0)
}

func (m Model) contentWidth() int {
	if m.prefs.Sidebar && m.width >= 110 {
		return m.width - 24
	}
	return m.width
}

func (m Model) hasInspector() bool {
	return m.screen == "messages" && m.prefs.Inspector && m.contentWidth() >= 95
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.focus == "navigation" && m.width < 110 || m.focus == "inspector" && !m.hasInspector() {
			m.focus = "content"
		}
	case loaded:
		m.loading = false
		if msg.err != nil {
			m.status = "Load failed: " + msg.err.Error()
			break
		}
		m.items = nil
		for _, r := range msg.resources {
			if r.Queue != "" || r.Subscription != "" {
				m.items = append(m.items, item{r, explorer.Counts{State: "unavailable", Detail: "not observed"}})
			}
		}
		m.status = "Offline sample data · browsing never receives or removes a message"
		if m.mode == "emulator" {
			m.status = "Emulator connected · runtime counts unavailable (emulator limitation) · peek only"
		}
		return m.refresh()
	case counted:
		m.refreshing = false
		for index := range m.items {
			i := &m.items[index]
			if c, ok := msg[i.resource]; ok {
				if c.State != "available" && (i.counts.State == "available" || i.counts.State == "stale") {
					i.counts.State, i.counts.Detail = "stale", c.Detail
				} else {
					i.counts = c
				}
			}
		}
		if m.again {
			m.again = false
			return m.refresh()
		}
		m.tick++
		token := m.tick
		return m, tea.Tick(30*time.Second, func(time.Time) tea.Msg { return countTick(token) })
	case countTick:
		if uint64(msg) == m.tick {
			return m.refresh()
		}
	case saved:
		m.saving = false
		if msg.err != nil {
			m.status = "Save failed: " + msg.err.Error()
		} else {
			m.persisted = msg.revision
			m.status = "Preferences saved · no credentials, message bodies or MCP grants stored"
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
		if key == "ctrl+c" {
			m.stop()
			return m, tea.Quit
		}
		if m.palette {
			return m.paletteKey(msg)
		}
		if key == "ctrl+p" {
			m.palette, m.query, m.action = true, "", 0
			return m, nil
		}
		if key == "q" {
			m.stop()
			return m, tea.Quit
		}
		if key == "b" {
			return m.execute("sidebar")
		}
		if key == "tab" || key == "shift+tab" {
			panes := []string{"content"}
			if m.hasInspector() {
				panes = append(panes, "inspector")
			}
			if m.prefs.Sidebar && m.width >= 110 {
				panes = append(panes, "navigation")
			}
			for index, pane := range panes {
				if m.focus == pane {
					step := 1
					if key == "shift+tab" {
						step = len(panes) - 1
					}
					m.focus = panes[(index+step)%len(panes)]
					break
				}
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
				m.screen, m.selected, m.nav = "dashboard", 0, 0
			}
			m.focus, m.loading = "content", false
			return m, nil
		}
		if m.focus == "navigation" {
			return m.navigationKey(key)
		}
		if m.screen == "settings" {
			return m.settingsKey(key)
		}
		if m.screen == "messages" {
			if m.focus == "inspector" {
				step := 0
				switch key {
				case "j", "down":
					step = 1
				case "k", "up":
					step = -1
				case "pgdown":
					step = max(1, m.height-10)
				case "pgup":
					step = -max(1, m.height-10)
				}
				m.offset = min(max(0, len(m.inspectorLines())-(m.height-8)), max(0, m.offset+step))
				if key != "i" && key != "[" && key != "]" && key != "r" {
					return m, nil
				}
			}
			switch key {
			case "j", "down":
				m.message, m.offset = min(max(0, len(m.page.Messages)-1), m.message+1), 0
			case "k", "up":
				m.message, m.offset = max(0, m.message-1), 0
			case "i":
				return m.execute("inspector")
			case "[", "]":
				if dlq := key == "]"; dlq != m.dlq {
					m.dlq, m.history = dlq, nil
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
		items := m.visible()
		switch key {
		case "g":
			return m.execute("grid")
		case "t":
			return m.execute("table")
		case "r":
			if len(m.items) == 0 && !m.loading {
				m.loading = true
				return m, m.Init()
			}
			return m.refresh()
		case "j", "down":
			m.selected = min(max(0, len(items)-1), m.selected+1)
		case "k", "up":
			m.selected = max(0, m.selected-1)
		case "enter", "]":
			if len(items) > 0 {
				return m.open(items[m.selected].resource, key == "]")
			}
		}
	}
	return m, nil
}

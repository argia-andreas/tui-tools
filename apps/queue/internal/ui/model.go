package ui

import (
	"context"
	"slices"
	"sync"
	"terminalui"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/clipboard"
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
	Mode, ConfigPath            string
	Preferences                 config.Preferences
	ASCII                       bool
	Clipboard                   func(string) (bool, error)
	ScopeProvider               explorer.ScopeProvider
	Connection, ConnectionLabel string
}

// Model is an offline-first workspace. Broker reads and disk writes are commands.
type Model struct {
	store                       explorer.Store
	mode, configPath            string
	provider                    explorer.ScopeProvider
	connection, connectionLabel string
	env                         environmentState
	scopeID                     uint64
	scopeLife                   context.Context
	scopeCancel                 context.CancelFunc
	scopeResults                []explorer.NamespaceDiscovery
	environmentItems            map[string][]item
	environmentSelection        map[string]explorer.Resource
	life                        context.Context
	stop                        context.CancelFunc
	theme                       theme.Theme
	prefs                       config.Preferences
	ascii                       bool
	width, height               int
	items                       []item
	screen, returnScreen        string
	focus                       string
	selected, nav               int
	message                     int
	setting                     int
	dlq                         bool
	page                        explorer.Page
	resource                    explorer.Resource
	from                        int64
	history                     []int64
	generation                  uint64
	cancel                      context.CancelFunc
	loading                     bool
	refreshing, again           bool
	tick                        uint64
	revision, persisted         uint64
	saving                      bool
	palette                     bool
	paletteState                terminalui.Palette
	help                        bool
	helpOffset                  int
	collapsed                   map[branch]bool
	status                      string
	review                      reviewState
	reviewID                    uint64
	reader                      inspectorState
	clipboard                   func(string) (bool, error)
}

func New(store explorer.Store, options Options) Model {
	t, _ := theme.Get(options.Preferences.Theme)
	ctx, cancel := context.WithCancel(context.Background())
	if options.Clipboard == nil {
		options.Clipboard = clipboard.Write
	}
	return Model{store: store, mode: options.Mode, configPath: options.ConfigPath,
		clipboard: options.Clipboard,
		provider:  options.ScopeProvider, connection: options.Connection, connectionLabel: options.ConnectionLabel,
		environmentItems: make(map[string][]item), environmentSelection: make(map[string]explorer.Resource),
		scopeLife: ctx,
		prefs:     options.Preferences, ascii: options.ASCII, life: ctx, stop: cancel,
		theme: t, width: 160, height: 48, screen: "dashboard", focus: "content",
		collapsed: make(map[branch]bool), loading: true, status: "Discovering resources…"}
}

func (m Model) Init() tea.Cmd {
	if m.provider != nil {
		return m.discoverEnvironment()
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
		defer cancel()
		resources, err := m.store.Discover(ctx)
		return loaded{resources, err}
	}
}

func (m Model) watched(r explorer.Resource) bool {
	if m.provider != nil {
		e, ok := m.environment()
		return ok && (e.All || slices.Contains(e.Watchlist, r)) && inNamespaces(e.Namespaces, r)
	}
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
		if m.provider != nil && slices.ContainsFunc(m.scopeResults, func(result explorer.NamespaceDiscovery) bool {
			return result.Err != nil && inNamespaces([]explorer.Namespace{result.Namespace}, i.resource)
		}) {
			continue
		}
		if m.watched(i.resource) {
			targets = append(targets, i.resource)
		}
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.scopeLife, 10*time.Second)
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
		if m.provider != nil {
			return environmentCounts{m.scopeID, counts}
		}
		return counts
	}
}

func (m Model) peek(from int64) (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
	}
	m.generation++
	m.from, m.message, m.loading = from, 0, true
	m.page = explorer.Page{}
	m.resetInspector()
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
	if m.showSidebar() {
		return m.width - 24
	}
	return m.width
}

func (m Model) hasInspector() bool {
	return m.screen == "messages" && ((m.prefs.Inspector && m.splitInspector()) || m.reader.revealed || m.inspectorFocused() || m.review.selected)
}

func (m Model) showSidebar() bool {
	return m.env.page == "" && m.prefs.Sidebar && m.width >= 110 && (m.screen != "messages" || m.width >= 122) && (m.review.phase == "" || m.review.selected)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.environmentUpdate(msg); handled {
		return next, cmd
	}
	if next, cmd, handled := m.inspectorMessage(msg); handled {
		return next, cmd
	}
	// Only input is captured here: async results and lock expiry still reach
	// their domain handlers while view-only Help covers the parent.
	switch input := msg.(type) {
	case tea.KeyMsg:
		if input.String() == "ctrl+c" {
			return m.quit()
		}
		if !m.workspaceFits() {
			if m.compactQuitVisible() {
				return m.environmentInput(msg)
			}
			if input.String() == "q" && m.env.page == "" && m.provider != nil {
				return m.quit()
			}
			// Nothing but the resize notice is rendered. Preserve hidden owners
			// and their drafts; only safe cancellation may act until they return.
			if input.String() == "esc" {
				switch {
				case m.help:
					m.help = false
				case m.palette:
					m.palette = false
				case m.review.phase != "":
					return m.reviewSurfaceInput(input)
				case m.reader.load != "":
					return m.fullLoadInput(input)
				}
			}
			return m, nil
		}
		if next, cmd, handled := m.sharedInput(msg); handled {
			return next, cmd
		}
	case tea.MouseMsg:
		if !m.workspaceFits() {
			return m, nil
		}
		if next, cmd, handled := m.sharedInput(msg); handled {
			return next, cmd
		}
	}
	if next, cmd, handled := m.reviewUpdate(msg); handled {
		return next, cmd
	}
	if m.env.page != "" {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			return m.environmentInput(msg)
		}
	}
	if m.reader.load == "" {
		m.repairWorkspaceFocus()
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			if next, cmd, handled := m.workspaceInput(msg); handled {
				return next, cmd
			}
		}
	}
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.reader.load != "" {
			return m.fullLoadInput(msg)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.focus == "navigation" && !m.showSidebar() {
			m.focus = "content"
		}
		m.repairWorkspaceFocus()
		m.keepCursorVisible()
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
			m.status = "Local emulator · discovery complete · runtime counts unavailable (emulator limitation) · peek only"
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
			if m.env.quitAfterSave && m.revision == msg.revision {
				m.stop()
				return m, tea.Quit
			}
		}
		m.env.quitAfterSave = false
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
		m.resetInspector()
		m.status = "PEEK ONLY · no locks acquired · no messages removed"
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m.quit()
		}
		if m.inspectorFocused() && m.reader.load != "" {
			if key == "q" {
				return m.quit()
			}
			return m.fullLoadInput(msg)
		}
		if key == "q" {
			return m.quit()
		}
		if key == "b" {
			if !m.inspectorFocused() {
				return m.execute("sidebar")
			}
		}
		if key == "ctrl+h" || key == "ctrl+l" {
			panes := []string{}
			if m.showSidebar() {
				panes = append(panes, "navigation")
			}
			panes = append(panes, "content")
			if m.hasInspector() {
				panes = append(panes, "inspector")
			}
			focus := m.workspacePane()
			for index, pane := range panes {
				if pane == focus {
					step := 1
					if key == "ctrl+h" {
						step = -1
					}
					next := panes[min(len(panes)-1, max(0, index+step))]
					if next != focus {
						m.focus = next
						m.reader.mode, m.reader.pending = "", ""
						if next != "inspector" {
							m.reader.revealed = false
						}
					}
					break
				}
			}
			return m, nil
		}
		if m.inspectorFocused() {
			return m.inspectorKey(key)
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
		if m.workspacePane() == "navigation" {
			m.focusWorkspace("navigation")
			return m.navigationKey(key)
		}
		if m.screen == "settings" {
			m.focusWorkspace("content")
			return m.settingsKey(key)
		}
		if m.screen == "messages" {
			switch key {
			case "j", "down":
				if next := min(max(0, len(m.page.Messages)-1), m.message+1); next != m.message {
					m.message = next
					m.resetInspector()
				}
			case "k", "up":
				if next := max(0, m.message-1); next != m.message {
					m.message = next
					m.resetInspector()
				}
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
				return m.changePage(true)
			case "p":
				return m.changePage(false)
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
		case "]":
			if len(items) > 0 {
				return m.open(items[m.selected].resource, true)
			}
		}
	}
	m.repairWorkspaceFocus()
	return m, nil
}

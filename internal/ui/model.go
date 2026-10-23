package ui

import (
	"context"
	"fmt"
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
)

type Options struct {
	Browser     storage.Browser
	Left, Right storage.Location
	Theme       theme.Theme
	Icons       bool
}

type pane struct {
	location   storage.Location
	entries    []storage.Entry
	cursor     int
	selected   map[string]bool
	filter     string
	generation int
	loading    bool
	err        error
}

type listed struct {
	pane, generation int
	entries          []storage.Entry
	err              error
}

type Model struct {
	browser                     storage.Browser
	panes                       [2]pane
	theme                       theme.Theme
	icons                       bool
	width, height               int
	focus                       int // 0 navigation, 1 local, 2 remote, 3 inspector, 4 jobs
	endpoint                    int // last focused file pane, retained while inspecting
	navigation, inspector, jobs bool
	modal                       string
	query                       string
	choice                      int
	filtering                   bool
	status                      string
	navCursor                   int
}

func New(o Options) Model {
	m := Model{browser: o.Browser, theme: o.Theme, icons: o.Icons, focus: 1, navigation: true,
		status: "Demo workspace — fixture data only; no network or disk writes"}
	m.panes[0] = pane{location: o.Left, selected: map[string]bool{}, loading: true}
	m.panes[1] = pane{location: o.Right, selected: map[string]bool{}, loading: true}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.list(0), m.list(1))
}

func (m Model) list(i int) tea.Cmd {
	p := m.panes[i]
	return func() tea.Msg {
		entries, err := m.browser.List(context.Background(), p.location)
		return listed{pane: i, generation: p.generation, entries: entries, err: err}
	}
}

func (m *Model) navigate(i int, loc storage.Location) tea.Cmd {
	p := &m.panes[i]
	p.location, p.cursor, p.filter = loc, 0, ""
	p.entries = nil
	p.selected = map[string]bool{}
	p.generation++
	p.loading, p.err = true, nil
	return m.list(i)
}

func (m Model) active() int {
	if m.focus == 2 {
		return 1
	}
	if m.focus == 1 {
		return 0
	}
	return m.endpoint
}

func (p pane) visible() []storage.Entry {
	var entries []storage.Entry
	for _, e := range p.entries {
		if strings.Contains(strings.ToLower(e.Name), strings.ToLower(p.filter)) {
			entries = append(entries, e)
		}
	}
	return entries
}

func (m Model) regions() []int {
	r := []int{}
	if m.navigation && m.width >= 90 {
		r = append(r, 0)
	}
	r = append(r, 1, 2)
	if m.inspector && m.width >= 120 {
		r = append(r, 3)
	}
	if m.jobs {
		r = append(r, 4)
	}
	return r
}

func (m *Model) fixFocus() {
	for _, r := range m.regions() {
		if r == m.focus {
			return
		}
	}
	m.focus = m.endpoint + 1
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.fixFocus()
	case listed:
		p := &m.panes[msg.pane]
		if p.generation == msg.generation {
			p.entries, p.err, p.loading = msg.entries, msg.err, false
			if p.cursor >= len(p.visible()) {
				p.cursor = 0
			}
		}
	case tea.MouseMsg:
		return m.mouse(msg)
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "f1" {
			return m.execute("help")
		}
		if m.modal != "" {
			return m.modalKey(msg)
		}
		if m.filtering {
			p := &m.panes[m.active()]
			switch key {
			case "esc":
				p.filter = ""
				m.filtering = false
			case "enter":
				m.filtering = false
			case "backspace", "ctrl+h":
				p.filter = dropRune(p.filter)
			default:
				if msg.Type == tea.KeyRunes {
					p.filter += string(msg.Runes)
				}
			}
			p.cursor = 0
			return m, nil
		}
		if key == "ctrl+p" {
			m.modal = "palette"
			m.query = ""
			m.choice = 0
			return m, nil
		}
		for _, a := range m.actions() {
			if a.shortcut == key && a.shortcut != "" {
				return m.execute(a.id)
			}
		}
		switch key {
		case "tab", "shift+tab":
			r := m.regions()
			idx := 0
			for i, v := range r {
				if v == m.focus {
					idx = i
				}
			}
			step := 1
			if key == "shift+tab" {
				step = len(r) - 1
			}
			m.focus = r[(idx+step)%len(r)]
			if m.focus == 1 || m.focus == 2 {
				m.endpoint = m.focus - 1
			}
		case "j", "down", "k", "up":
			delta := 1
			if key == "k" || key == "up" {
				delta = -1
			}
			if m.focus == 0 {
				m.navCursor = (m.navCursor + delta + len(navLocations)) % len(navLocations)
			}
			if m.focus == 1 || m.focus == 2 {
				p := &m.panes[m.active()]
				n := len(p.visible())
				if n > 0 {
					p.cursor = max(0, min(n-1, p.cursor+delta))
				}
			}
		case "enter":
			if m.focus == 0 {
				loc := navLocations[m.navCursor]
				i := 1
				if loc.Remote == "" {
					i = 0
				}
				m.focus = i + 1
				m.endpoint = i
				cmd := m.navigate(i, loc)
				return m, cmd
			}
			if m.focus == 1 || m.focus == 2 {
				i := m.active()
				p := m.panes[i]
				entries := p.visible()
				if len(entries) > 0 && entries[p.cursor].IsDir {
					loc := p.location
					loc.Path = path.Join(loc.Path, entries[p.cursor].Name)
					cmd := m.navigate(i, loc)
					return m, cmd
				}
			}
		case "backspace", "h":
			if m.focus == 1 || m.focus == 2 {
				i := m.active()
				loc := m.panes[i].location
				loc.Path = path.Dir(loc.Path)
				cmd := m.navigate(i, loc)
				return m, cmd
			}
		case " ":
			if m.focus == 1 || m.focus == 2 {
				p := &m.panes[m.active()]
				entries := p.visible()
				if len(entries) > 0 {
					name := entries[p.cursor].Name
					p.selected[name] = !p.selected[name]
				}
			}
		case "/":
			if m.focus == 1 || m.focus == 2 {
				m.filtering = true
			}
		}
	}
	return m, nil
}

func dropRune(s string) string {
	r := []rune(s)
	if len(r) > 0 {
		return string(r[:len(r)-1])
	}
	return ""
}

var navLocations = []storage.Location{
	{Remote: "demo-ftp", Path: "/incoming"}, {Remote: "demo-ftp", Path: "/archive"},
	{Remote: "demo-blob", Path: "/assets"}, {Path: "/project"}, {Path: "/downloads"},
}

type action struct{ id, label, shortcut, keywords, disabled string }

func (m Model) actions() []action {
	direction := "right"
	if m.active() == 1 {
		direction = "left"
	}
	a := []action{
		{"copy", "Copy to " + direction + " pane", "c", "upload download transfer", "Transfers arrive in the next slice; demo is read-only"},
		{"preview", "Preview selected file", "p", "inspect text", "Content previews arrive after real storage integration"},
		{"navigation", "Toggle navigation sidebar", "[", "connections places", ""},
		{"inspector", "Toggle file inspector", "]", "metadata details sidebar", ""},
		{"jobs", "Toggle transfer drawer", "t", "queue jobs", ""},
		{"refresh", "Refresh focused filesystem", "ctrl+r", "reload browse", ""},
		{"icons", "Toggle filetype icons", "", "glyphs nerd font", ""},
		{"help", "Keyboard shortcuts", "?", "help keys", ""},
	}
	for _, name := range theme.Names() {
		a = append(a, action{"theme:" + name, "Theme: " + name, "", "colors style appearance", ""})
	}
	a = append(a, action{"quit", "Quit workspace", "q", "exit close", ""})
	return a
}

func (m Model) matches() []action {
	a := m.actions()
	if m.query == "" {
		return a
	}
	labels := make([]string, len(a))
	for i, v := range a {
		labels[i] = v.label
	}
	var found []action
	for _, match := range fuzzy.Find(m.query, labels) {
		found = append(found, a[match.Index])
	}
	if len(found) == 0 {
		for i, v := range a {
			labels[i] = v.keywords
		}
		for _, match := range fuzzy.Find(m.query, labels) {
			found = append(found, a[match.Index])
		}
	}
	return found
}

func (m Model) modalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" || key == "ctrl+p" {
		m.modal = ""
		return m, nil
	}
	if m.modal == "help" {
		switch key {
		case "up", "k":
			m.choice = max(0, m.choice-1)
		case "down", "j":
			m.choice = min(max(0, len(shortcutLines())-(min(20, m.height-4)-3)), m.choice+1)
		}
		return m, nil
	}
	switch key {
	case "down", "ctrl+n":
		if n := len(m.matches()); n > 0 {
			m.choice = (m.choice + 1) % n
		}
	case "up", "ctrl+k":
		if n := len(m.matches()); n > 0 {
			m.choice = (m.choice + n - 1) % n
		}
	case "enter":
		a := m.matches()
		if len(a) > 0 {
			chosen := a[min(m.choice, len(a)-1)]
			if chosen.disabled != "" {
				m.status = chosen.disabled
				return m, nil
			}
			m.modal = ""
			return m.execute(chosen.id)
		}
	case "backspace", "ctrl+h":
		m.query = dropRune(m.query)
		m.choice = 0
	default:
		if msg.Type == tea.KeyRunes {
			m.query += string(msg.Runes)
			m.choice = 0
		}
	}
	return m, nil
}

func (m Model) execute(id string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(id, "theme:") {
		t, err := theme.Get(strings.TrimPrefix(id, "theme:"))
		if err == nil {
			m.theme = t
			m.status = "Theme: " + t.Name
		}
		return m, nil
	}
	switch id {
	case "navigation":
		m.navigation = !m.navigation
		m.fixFocus()
	case "inspector":
		m.inspector = !m.inspector
		m.fixFocus()
	case "jobs":
		m.jobs = !m.jobs
		m.fixFocus()
	case "icons":
		m.icons = !m.icons
		m.status = fmt.Sprintf("Filetype icons: %t — enable with a Nerd Font", m.icons)
	case "refresh":
		i := m.active()
		m.panes[i].generation++
		m.panes[i].loading = true
		return m, m.list(i)
	case "help":
		m.modal = "help"
		m.choice = 0
	case "quit":
		return m, tea.Quit
	case "copy", "preview":
		for _, a := range m.actions() {
			if a.id == id {
				m.status = a.disabled
			}
		}
	}
	return m, nil
}

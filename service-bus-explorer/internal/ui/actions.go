package ui

import (
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/theme"
)

type action struct{ id, title string }

func (m Model) actions() []action {
	choices := []action{
		{"dashboard", "Go to Dashboard"}, {"queues", "Browse all Queues"},
		{"topics", "Browse Topics and subscriptions"}, {"settings", "Open Settings and watchlists"},
		{"save", "Save preferences"}, {"sidebar", "Toggle navigation sidebar"},
		{"grid", "Dashboard layout: Grid"}, {"table", "Dashboard layout: Table"},
		{"counts", "Refresh watched counts (read-only)"},
	}
	for _, name := range theme.Names() {
		choices = append(choices, action{"theme:" + name, "Color theme: " + name})
	}
	if m.screen == "messages" {
		choices = append(choices, action{"inspector", "Toggle message inspector"}, action{"refresh", "Refresh this peek page (no receive)"})
		if m.dlq {
			choices = append(choices, action{"receive", "Receive next DLQ message… (acquire lock, then review)"})
		}
	}
	if m.screen == "messages" || (m.screen != "settings" && len(m.visible()) > 0) {
		choices = append(choices, action{"active", "Peek Active messages (no locks)"}, action{"dlq", "Peek DLQ messages (no locks)"})
	}
	if m.query == "" {
		return choices
	}
	titles := make([]string, len(choices))
	for i, c := range choices {
		titles[i] = c.title
	}
	var matches []action
	for _, match := range fuzzy.Find(m.query, titles) {
		matches = append(matches, choices[match.Index])
	}
	return matches
}

func (m Model) paletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.palette = false
	case "down", "ctrl+n":
		m.action = min(max(0, len(m.actions())-1), m.action+1)
	case "up", "ctrl+p":
		m.action = max(0, m.action-1)
	case "backspace", "ctrl+h":
		runes := []rune(m.query)
		m.query = string(runes[:max(0, len(runes)-1)])
		m.action = 0
	case "ctrl+u":
		m.query, m.action = "", 0
	case "enter":
		if choices := m.actions(); len(choices) > 0 {
			m.palette = false
			return m.execute(choices[m.action].id)
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			// Limit pasted queries, and never let printable q/j/k escape this field.
			m.query += clean(string(msg.Runes), false)
			if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
				m.query += " "
			}
			m.query = string([]rune(m.query)[:min(128, len([]rune(m.query)))])
			m.action = 0
		}
	}
	return m, nil
}

func (m Model) execute(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "receive":
		return m.acquire()
	case "dashboard", "queues", "topics", "settings":
		if m.cancel != nil {
			m.cancel()
		}
		m.generation++
		m.screen, m.focus, m.selected, m.loading = id, "content", 0, false
	case "sidebar":
		m.prefs.Sidebar = !m.prefs.Sidebar
		m.focus = "content"
		m.revision++
	case "inspector":
		m.prefs.Inspector = !m.prefs.Inspector
		m.focus = "content"
		m.revision++
	case "grid", "table":
		m.prefs.Table = id == "table"
		m.revision++
	case "counts":
		return m.refresh()
	case "refresh":
		return m.peek(m.from)
	case "active", "dlq":
		r := m.resource
		if m.screen != "messages" {
			items := m.visible()
			if len(items) == 0 {
				return m, nil
			}
			r = items[m.selected].resource
		}
		return m.open(r, id == "dlq")
	case "save":
		if m.saving {
			return m, nil
		}
		m.saving, m.status = true, "Saving preferences…"
		p, path, revision := m.prefs, m.configPath, m.revision
		// The save command owns a snapshot, even if the user keeps editing.
		p.Watchlists = maps.Clone(p.Watchlists)
		for profile, list := range p.Watchlists {
			p.Watchlists[profile] = slices.Clone(list)
		}
		return m, func() tea.Msg { return saved{revision, config.Save(path, p)} }
	default:
		if name, ok := strings.CutPrefix(id, "theme:"); ok {
			m.theme, _ = theme.Get(name)
			m.prefs.Theme = name
			m.revision++
		}
	}
	return m, nil
}

func (m Model) settingsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		m.setting = min(4+len(m.items), m.setting+1)
	case "k", "up":
		m.setting = max(0, m.setting-1)
	case "s":
		return m.execute("save")
	case "enter", " ":
		switch m.setting {
		case 0:
			names := theme.Names()
			return m.execute("theme:" + names[(slices.Index(names, m.prefs.Theme)+1)%len(names)])
		case 1:
			return m.execute("sidebar")
		case 2:
			return m.execute("inspector")
		case 3:
			if m.prefs.Table {
				return m.execute("grid")
			}
			return m.execute("table")
		case 4:
			return m.execute("save")
		default:
			r := m.items[m.setting-5].resource
			m.prefs.Watchlists = maps.Clone(m.prefs.Watchlists)
			if m.prefs.Watchlists == nil {
				m.prefs.Watchlists = make(map[string][]explorer.Resource)
			}
			list, explicit := m.prefs.Watchlists[r.Profile]
			list = slices.Clone(list)
			if !explicit {
				for _, i := range m.items {
					if i.resource.Profile == r.Profile {
						list = append(list, i.resource)
					}
				}
			}
			if index := slices.Index(list, r); index >= 0 {
				list = slices.Delete(list, index, index+1)
			} else {
				list = append(list, r)
			}
			if list == nil {
				list = []explorer.Resource{}
			}
			m.prefs.Watchlists[r.Profile] = list
			m.selected = 0
			m.revision++
			if m.refreshing {
				m.again = true
			} else {
				return m.refresh()
			}
		}
	}
	return m, nil
}

// Branch keys retain profile and namespace identity; display names are not keys.
type branch struct{ profile, namespace, kind, topic string }
type treeRow struct {
	label    string
	depth    int
	branch   branch
	resource explorer.Resource
	leaf     bool
}

func (m Model) tree() []treeRow {
	var rows []treeRow
	seen := map[branch]bool{}
	for _, i := range m.items {
		r := i.resource
		ns := branch{profile: r.Profile, namespace: r.Namespace, kind: "namespace"}
		if seen[ns] {
			continue
		}
		seen[ns] = true
		rows = append(rows, treeRow{label: r.Profile + " / " + r.Namespace, branch: ns})
		if m.collapsed[ns] {
			continue
		}
		for _, kind := range []string{"Queues", "Topics"} {
			group := ns
			group.kind = kind
			rows = append(rows, treeRow{label: kind, branch: group, depth: 1})
			if m.collapsed[group] {
				continue
			}
			for _, candidate := range m.items {
				c := candidate.resource
				if c.Profile != r.Profile || c.Namespace != r.Namespace {
					continue
				}
				if kind == "Queues" && c.Queue != "" {
					rows = append(rows, treeRow{label: c.Queue, resource: c, leaf: true, depth: 2})
				} else if kind == "Topics" && c.Subscription != "" {
					topic := group
					topic.topic = c.Topic
					if !seen[topic] {
						seen[topic] = true
						rows = append(rows, treeRow{label: c.Topic, branch: topic, depth: 2})
						if !m.collapsed[topic] {
							for _, sub := range m.items {
								if sub.resource.Profile == c.Profile && sub.resource.Namespace == c.Namespace && sub.resource.Topic == c.Topic && sub.resource.Subscription != "" {
									rows = append(rows, treeRow{label: sub.resource.Subscription, resource: sub.resource, leaf: true, depth: 3})
								}
							}
						}
					}
				}
			}
		}
	}
	return rows
}

func (m Model) navigationKey(key string) (tea.Model, tea.Cmd) {
	rows := m.tree()
	m.nav = min(m.nav, len(rows)+3)
	switch key {
	case "j", "down":
		m.nav = min(len(rows)+3, m.nav+1)
	case "k", "up":
		m.nav = max(0, m.nav-1)
	case "enter", "h", "left", "l", "right", "]":
		if m.nav < 4 {
			if key == "enter" {
				return m.execute([]string{"dashboard", "queues", "topics", "settings"}[m.nav])
			}
		} else {
			row := rows[m.nav-4]
			if row.leaf {
				if key == "enter" || key == "]" {
					return m.open(row.resource, key == "]")
				}
			} else {
				m.collapsed = maps.Clone(m.collapsed)
				switch key {
				case "enter":
					m.collapsed[row.branch] = !m.collapsed[row.branch]
				case "h", "left":
					m.collapsed[row.branch] = true
				case "l", "right":
					m.collapsed[row.branch] = false
				}
			}
		}
	}
	return m, nil
}

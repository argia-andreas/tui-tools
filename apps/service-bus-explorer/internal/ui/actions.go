package ui

import (
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/theme"
	"terminalui"
)

func (m Model) actions() []terminalui.Action {
	var choices []terminalui.Action
	add := func(id, label string) {
		choices = append(choices, terminalui.Action{ID: id, Label: label, Unavailable: m.actionsBlocked()})
	}
	for _, pair := range [][2]string{
		{"dashboard", "Go to Dashboard"}, {"queues", "Browse all Queues"},
		{"topics", "Browse Topics and subscriptions"}, {"settings", "Open Settings and watchlists"},
		{"save", "Save preferences"}, {"sidebar", "Toggle navigation sidebar"},
		{"grid", "Dashboard layout: Grid"}, {"table", "Dashboard layout: Table"},
		{"counts", "Refresh watched counts (read-only)"},
	} {
		add(pair[0], pair[1])
	}
	if m.provider != nil {
		for _, pair := range [][2]string{
			{"env:switch", "Switch environment"}, {"env:manage", "Manage environments (Settings)"},
			{"env:new", "New environment…"}, {"env:edit-current", "Edit current environment…"},
			{"env:inclusion", "Dashboard inclusion (Settings)"}, {"env:refresh", "Rediscover selected namespaces / Retry failed"},
			{"env:details", "Discovery details / namespace errors"},
		} {
			add(pair[0], pair[1])
		}
	}
	for _, name := range theme.Names() {
		add("theme:"+name, "Color theme: "+name)
	}
	if m.screen == "messages" {
		add("inspector", "Toggle message inspector")
		add("refresh", "Refresh this peek page (no receive)")
		if !m.loading && len(m.page.Messages) > 0 {
			add("copy-body", "Copy full body (Y)")
			add("copy-report", "Copy message report (metadata, body, properties)")
			add("copy-line", "Copy line (yy)")
			add("copy-help", "Message navigation and clipboard help (?)")
			if m.reader.mode != "" {
				add("copy-selection", "Copy selection (y)")
			}
		}
		if m.dlq {
			add("receive", "Receive next DLQ message… (acquire lock, then review)")
			if !m.loading && len(m.page.Messages) > 0 {
				label := "Send selected DLQ message to queue…"
				if m.resource.Queue != "" {
					label = "Resend selected DLQ message to this queue…"
				}
				add("selected-copy", label)
			}
		}
	}
	if m.screen == "messages" || (m.screen != "settings" && len(m.visible()) > 0) {
		add("active", "Peek Active messages (no locks)")
		add("dlq", "Peek DLQ messages (no locks)")
	}
	for i := range choices {
		if choices[i].ID == "save" {
			choices[i].Busy = m.saving
		}
		if choices[i].ID == "counts" {
			choices[i].Busy = m.refreshing
		}
		if m.mode == "azure" && (choices[i].ID == "receive" || choices[i].ID == "selected-copy") {
			choices[i].Unavailable = "Azure connection is read-only"
		}
		// Document only shortcuts that the current local owner actually handles.
		// Reader grammar (yy, selections) remains in contextual Help topics.
		if choices[i].ID == "sidebar" && !m.inspectorFocused() {
			choices[i].Keys = []string{"b"}
		}
		if m.focus == "content" {
			if m.screen == "messages" {
				if key := map[string]string{"inspector": "i", "refresh": "r", "active": "[", "dlq": "]"}[choices[i].ID]; key != "" {
					choices[i].Keys = []string{key}
				}
			} else if m.screen == "settings" {
				if choices[i].ID == "save" {
					choices[i].Keys = []string{"s"}
				}
			} else if key := map[string]string{"grid": "g", "table": "t", "counts": "r", "active": "enter", "dlq": "]"}[choices[i].ID]; key != "" {
				choices[i].Keys = []string{key}
			}
		}
		if m.focus == "inspector" && choices[i].ID == "copy-body" {
			choices[i].Keys = []string{"Y"}
		}
	}
	return choices
}

func (m Model) execute(id string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(id, "env:") && m.provider != nil {
		return m.environmentAction(id)
	}
	switch id {
	case "copy-body", "copy-report", "copy-line", "copy-selection":
		m.focus, m.reader.revealed = "inspector", true
		return m.copyText(strings.TrimPrefix(id, "copy-"))
	case "copy-help":
		m.help, m.helpOffset = true, 0
		return m, nil
	case "selected-copy":
		return m.openSelected()
	case "receive":
		if m.mode == "azure" {
			m.status = "Azure connection is read-only; message mutations are disabled"
			return m, nil
		}
		return m.acquire()
	case "dashboard", "queues", "topics", "settings":
		if m.cancel != nil {
			m.cancel()
		}
		m.generation++
		m.screen, m.focus, m.selected, m.loading = id, "content", 0, false
		m.resetInspector()
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
		p, path, revision := m.prefs.Clone(), m.configPath, m.revision
		// The save command owns a snapshot, even if the user keeps editing.
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

func (m Model) settingsCount() int {
	if m.provider != nil {
		return 7
	}
	return 5 + len(m.items)
}

func (m Model) settingsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		m.setting = min(m.settingsCount()-1, m.setting+1)
	case "k", "up":
		m.setting = max(0, m.setting-1)
	case "s":
		return m.execute("save")
	case " ":
		return m.changeSetting()
	}
	return m, nil
}

func (m Model) changeSetting() (tea.Model, tea.Cmd) {
	if m.setting < 0 || m.setting >= m.settingsCount() {
		return m, nil
	}
	if m.provider != nil && m.setting >= 5 {
		return m.environmentAction(m.settingID(m.setting))
	}
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
	case "h", "left", "l", "right", "]":
		if m.nav >= 4 {
			row := rows[m.nav-4]
			if row.leaf {
				if key == "]" {
					return m.open(row.resource, true)
				}
			} else {
				m.collapsed = maps.Clone(m.collapsed)
				switch key {
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

func (m Model) toggleBranch(b branch) (tea.Model, tea.Cmd) {
	m.collapsed = maps.Clone(m.collapsed)
	m.collapsed[b] = !m.collapsed[b]
	return m, nil
}

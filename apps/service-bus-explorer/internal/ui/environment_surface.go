package ui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"terminalui"
)

func environmentID(kind string, value any) string {
	b, _ := json.Marshal(value)
	return "environment:" + kind + ":" + string(b)
}

// These IDs are recomputed from current native identities, not labels or row
// offsets. Catalog request identity also fences group snapshot controls.
func (m Model) environmentRows() []string {
	var ids []string
	switch m.env.page {
	case "switch", "manage":
		for _, e := range m.filteredEnvironments() {
			ids = append(ids, environmentID("saved", []string{e.ID, e.Connection}))
		}
	case "subscriptions":
		for _, s := range m.filteredSubscriptions() {
			ids = append(ids, environmentID("subscription", s.ID))
		}
	case "namespaces":
		for _, c := range m.namespaceChoices() {
			if c.group != nil {
				ids = append(ids, environmentID("group", []any{m.env.request, c.group}))
			} else {
				ids = append(ids, environmentID("namespace", c.namespace))
			}
		}
	case "selected", "review", "group":
		list := m.env.draft.Namespaces
		if m.env.page == "group" {
			list = m.env.group
		}
		for _, n := range list {
			ids = append(ids, environmentID("namespace", n))
		}
	case "inclusion":
		for _, r := range m.inclusionResources() {
			ids = append(ids, resourceID(r))
		}
	}
	return ids
}

func (m Model) environmentControls() []terminalui.Action {
	var actions []terminalui.Action
	add := func(id, label string) { actions = append(actions, terminalui.Action{ID: id, Label: label}) }
	switch m.env.page {
	case "switch":
		add("use", "Use")
		add("manage", "Manage")
	case "manage":
		add("use", "Use")
		add("new", "New")
		add("edit", "Edit")
		add("duplicate", "Duplicate")
		add("delete-review", "Delete…")
	case "subscriptions":
		add("open", "Open")
	case "namespaces":
		choices := m.namespaceChoices()
		if m.env.index >= 0 && m.env.index < len(choices) && choices[m.env.index].group != nil {
			add("preview", "Preview group")
		} else {
			add("toggle", "Toggle namespace")
		}
	case "selected":
		add("identity", "Details")
		add("remove", "Remove")
	case "known":
		add("verify", "Verify/add")
	case "review":
		add("apply-use", "Apply & use")
		add("apply", "Apply only")
	case "group":
		add("add-group", "Add listed")
	case "all":
		add("enable-all", "Enable All")
	case "inclusion":
		add("toggle", "Toggle")
		add("matching", "Select matching")
		add("mode", "All/Selected")
		add("identity", "Details")
		add("save", "Save")
		add("quit", "Quit")
	case "delete":
		add("delete", "Delete local")
	case "discard":
		add("discard", "Discard form")
	case "quit":
		add("save-quit", "Save & quit")
		add("discard-quit", "Discard & quit")
	}
	rows := m.environmentRows()
	for i := range actions {
		a := &actions[i]
		switch a.ID {
		case "use", "edit", "duplicate", "delete-review", "open", "preview", "toggle", "identity", "remove":
			if m.env.index < 0 || m.env.index >= len(rows) {
				a.Unavailable = "No selection"
			}
		}
		if a.ID == "delete" && m.env.query != m.env.draft.Name {
			a.Unavailable = "Exact name required"
		}
		if a.ID == "save-quit" && m.env.dirty {
			a.Unavailable = "Unapplied draft"
		}
		if a.ID == "toggle" || a.ID == "matching" {
			if e, ok := m.environment(); m.env.page == "inclusion" && ok && e.All {
				a.Unavailable = "All mode"
			}
		}
		a.Busy = m.env.busy
	}
	return actions
}

func environmentConfirmation(page string) bool {
	return page == "review" || page == "group" || page == "all" || page == "discard" || page == "quit" || page == "delete"
}

func (m Model) environmentFocus() string {
	if m.env.focusPage == m.env.page && m.env.focus != "" {
		return m.env.focus
	}
	if environmentConfirmation(m.env.page) {
		return "back"
	}
	if m.env.page == "edit" {
		return []string{"name", "browse", "selected", "known", "review", "back"}[min(5, max(0, m.env.field))]
	}
	if m.env.page == "known" || m.env.page == "switch" || m.env.page == "subscriptions" || m.env.page == "namespaces" {
		return "search"
	}
	return "list"
}

func (m Model) environmentTextFocused() bool {
	f := m.environmentFocus()
	return f == "name" || f == "search" || f == "authorization"
}

func (m Model) environmentBounds() terminalui.Rect {
	return terminalui.Rect{Y: 3, W: m.width, H: max(0, m.height-6)}
}

// environmentInput captures only input, after Help/shared toolbar routing. The
// caller must route lifecycle messages through environmentUpdate independently.
func (m Model) environmentInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
	default:
		return m, nil
	}
	if m.env.page == "" || m.review.phase != "" || m.reader.load != "" {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+p" {
		return m, nil
	}
	// Compact View renders exactly these local quit choices. No hidden editing
	// or surface activation is permitted below the environment minimum.
	if m.width < 60 || m.height < 18 {
		if k, ok := msg.(tea.KeyMsg); ok && m.env.page == "quit" && m.width >= 48 && m.height >= 3 {
			switch k.String() {
			case "esc", "c", "s", "d":
				return m.environmentCommand(k.String(), nil)
			}
		}
		return m, nil
	}
	bounds := m.environmentBounds()
	spec := m.environmentSurface(bounds.W, bounds.H)
	frame := terminalui.RenderSurface(spec, bounds, m.theme.TerminalStyle(m.ascii))
	if k, ok := msg.(tea.KeyMsg); ok {
		key := k.String()
		text := func(value string, limit int) string { return editText(value, k, limit) }
		if key == "esc" {
			return m.environmentDismiss()
		}
		if key != "tab" && key != "shift+tab" {
			if key == " " && m.env.page == "namespaces" && (m.environmentTextFocused() || m.environmentFocus() == "list") {
				return m.environmentPrimary()
			}
			if m.environmentTextFocused() && key != "enter" {
				if m.env.page == "edit" && (key == "up" || key == "down") {
					m.env.focus, m.env.focusPage = "", ""
				}
				return m.environmentCommand(key, text)
			}
			// Local list grammar remains available; confirmations have no default
			// destructive Enter. Body arrows do not traverse unrelated controls.
			if m.environmentFocus() == "list" || m.environmentTextFocused() {
				if key == "enter" {
					if m.env.page == "delete" && m.environmentFocus() == "authorization" {
						m.env.focus, m.env.focusPage = "delete", m.env.page
						return m, nil
					}
					if environmentConfirmation(m.env.page) {
						return m, nil
					}
					return m.environmentPrimary()
				}
				if key == " " && (m.env.page == "inclusion" || m.env.page == "namespaces" || m.env.page == "selected") {
					return m.environmentDo(map[string]string{"selected": "remove", "inclusion": "toggle", "namespaces": "toggle"}[m.env.page])
				}
				if key == "down" || key == "up" || key == "j" || key == "k" || key == "pgdown" || key == "pgup" || key == "ctrl+n" {
					return m.environmentCommand(key, nil)
				}
			}
			shortcuts := map[string]string{"n": "new", "e": "edit", "d": "duplicate", "x": "delete-review"}
			if m.env.page == "inclusion" {
				shortcuts = map[string]string{"a": "matching", "m": "mode", "v": "identity", "s": "save", "q": "quit"}
				if key == "/" {
					m.env.focus, m.env.focusPage, m.env.field = "search", m.env.page, 1
					return m, nil
				}
			} else if m.env.page == "selected" {
				shortcuts = map[string]string{"x": "remove"}
			} else if m.env.page == "review" {
				shortcuts = map[string]string{"a": "apply"}
			} else if m.env.page == "quit" {
				shortcuts = map[string]string{"s": "save-quit", "d": "discard-quit", "c": "back"}
			} else if m.env.page != "manage" {
				shortcuts = nil
			}
			if id := shortcuts[key]; id != "" {
				return m.environmentDo(id)
			}
		}
	}
	r := terminalui.HandleSurface(msg, spec, frame)
	if r.Intent == nil {
		return m, nil
	}
	switch r.Intent.Kind {
	case terminalui.Dismiss:
		return m.environmentDismiss()
	case terminalui.Focus:
		m.env.focus, m.env.focusPage = r.Intent.ID, m.env.page
		if m.env.page == "edit" {
			for i, id := range []string{"name", "browse", "selected", "known", "review", "back"} {
				if id == r.Intent.ID {
					m.env.field = i
				}
			}
		}
		if m.env.page == "inclusion" {
			m.env.field = 0
			if r.Intent.ID == "search" {
				m.env.field = 1
			}
		}
	case terminalui.Select:
		for i, id := range m.environmentRows() {
			if id == r.Intent.ID {
				m.env.index, m.env.focus, m.env.focusPage = i, "list", m.env.page
				break
			}
		}
	case terminalui.Activate:
		if strings.HasPrefix(r.Intent.ID, "check:") {
			for i, id := range m.environmentRows() {
				if r.Intent.ID == "check:"+id {
					m.env.index, m.env.focus, m.env.focusPage = i, "list", m.env.page
					return m.environmentDo("toggle")
				}
			}
		}
		return m.environmentDo(r.Intent.ID)
	case terminalui.Scroll:
		if m.env.page == "inclusion" {
			m.env.field, m.env.focus, m.env.focusPage = 0, "list", m.env.page
		}
		key := "down"
		if r.Intent.Delta < 0 {
			key = "up"
		}
		return m.environmentCommand(key, nil)
	}
	return m, nil
}

func (m Model) environmentPrimary() (tea.Model, tea.Cmd) {
	id := map[string]string{"switch": "use", "manage": "use", "subscriptions": "open", "namespaces": "toggle", "selected": "identity", "known": "verify", "details": "back"}[m.env.page]
	if m.env.page == "namespaces" {
		choices := m.namespaceChoices()
		if m.env.index >= 0 && m.env.index < len(choices) && choices[m.env.index].group != nil {
			id = "preview"
		}
	}
	if m.env.page == "edit" {
		m.env.field = min(5, m.env.field+1)
		m.env.focus, m.env.focusPage = "", ""
		return m, nil
	}
	if m.env.page == "inclusion" && m.env.field == 1 {
		m.env.field, m.env.focus, m.env.focusPage = 0, "list", m.env.page
		return m, nil
	}
	if m.env.page == "inclusion" {
		id = "toggle"
	}
	return m.environmentDo(id)
}

func (m Model) environmentDismiss() (tea.Model, tea.Cmd) {
	m.env.focus, m.env.focusPage = "", ""
	return m.environmentCommand("esc", nil)
}

// Rebuild visible controls immediately before every semantic operation. Named
// operations call domain methods directly; pointer input never becomes keys.
func (m Model) environmentDo(id string) (tea.Model, tea.Cmd) {
	if m.env.page == "" || m.review.phase != "" || m.reader.load != "" {
		return m, nil
	}
	if id == "back" {
		return m.environmentDismiss()
	}
	spec := m.environmentSurface(m.width, max(0, m.height-6))
	frame := terminalui.RenderSurface(spec, m.environmentBounds(), m.theme.TerminalStyle(m.ascii))
	found := false
	for _, r := range frame.Regions {
		if r.ID == id && r.Kind == terminalui.ControlRegion {
			found = true
		}
	}
	if !found || m.env.busy || m.width < 60 || m.height < 18 {
		return m, nil
	}
	for _, a := range m.environmentControls() {
		if a.ID == id && (a.Unavailable != "" || a.Busy) {
			return m, nil
		}
	}
	m.env.focus, m.env.focusPage = "", ""
	if m.env.page == "inclusion" {
		m.env.field = 0
	}
	if m.env.page == "edit" {
		for i, control := range []string{"name", "browse", "selected", "known", "review", "back"} {
			if id == control {
				m.env.field = i
			}
		}
	}
	if id == "manage" {
		m.env.page, m.env.query, m.env.index = "manage", "", 0
		return m, nil
	}
	switch id {
	case "browse":
		return m.catalog("subscriptions", "")
	case "selected":
		m.env.page, m.env.index = "selected", 0
	case "known":
		m.env.page, m.env.query = "known", ""
	case "review":
		m.env.page, m.env.index = "review", 0
	case "new":
		return m.environmentAction("env:new")
	case "use", "edit", "duplicate", "delete-review":
		e := m.filteredEnvironments()[m.env.index]
		if id == "use" {
			return m.useEnvironment(e.ID)
		}
		if e.Connection != m.connection {
			m.env.err = "Restart with this environment's explicit connection to edit it"
			return m, nil
		}
		if id == "delete-review" {
			m.env.draft, m.env.page, m.env.query = e, "delete", ""
		} else if id == "duplicate" {
			e.ID, e.Name = "env-"+rand.Text(), ""
			return m.editEnvironment(e, true)
		} else {
			return m.editEnvironment(e, false)
		}
	case "open":
		return m.catalog("namespaces", m.filteredSubscriptions()[m.env.index].ID)
	case "preview":
		m.env.group, m.env.page = slices.Clone(m.namespaceChoices()[m.env.index].group), "group"
	case "toggle":
		if m.env.page == "inclusion" {
			return m.changeInclusion(false, false)
		}
		choice := m.namespaceChoices()[m.env.index]
		if i := slices.IndexFunc(m.env.draft.Namespaces, func(n explorer.Namespace) bool { return sameNamespace(n, choice.namespace) }); i >= 0 {
			m.env.draft.Namespaces = slices.Delete(slices.Clone(m.env.draft.Namespaces), i, i+1)
			m.env.dirty = true
		} else {
			m = m.addNamespaces([]explorer.Namespace{choice.namespace})
		}
	case "identity":
		if m.env.page == "inclusion" {
			r := m.inclusionResources()[m.env.index]
			m.env.details = []string{"Namespace: " + r.Namespace, "Entity: " + r.Name(), "Connection: " + r.Profile,
				"Azure subscription ID: " + r.AzureSubscriptionID, "Resource group: " + r.ResourceGroup, "Status: " + m.inclusionState(r)}
		} else {
			m.env.details = namespaceDetails(m.env.draft.Namespaces[m.env.index])
		}
		m.env.back, m.env.page, m.env.index = m.env.page, "details", 0
	case "remove":
		m.env.draft.Namespaces = slices.Delete(slices.Clone(m.env.draft.Namespaces), m.env.index, m.env.index+1)
		m.env.index, m.env.dirty = max(0, m.env.index-1), true
	case "verify":
		m.env.request++
		request, input, provider, life := m.env.request, m.env.query, m.provider, m.life
		m.env.busy = true
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(life, 30*time.Second)
			defer cancel()
			ns, err := provider.ResolveNamespace(ctx, input)
			return catalogLoaded{id: request, known: &ns, err: err}
		}
	case "apply-use", "apply":
		return m.applyEnvironment(id == "apply-use")
	case "add-group":
		m = m.addNamespaces(m.env.group)
		m.env.page, m.env.index = "namespaces", 0
	case "enable-all":
		m.env.page = "inclusion"
		return m.changeInclusion(true, true)
	case "mode":
		if e, ok := m.environment(); ok && e.All {
			return m.changeInclusion(false, true)
		}
		m.env.page = "all"
	case "matching":
		if e, ok := m.environment(); ok && !e.All {
			m.prefs = m.prefs.Clone()
			i := m.environmentIndex()
			for _, r := range m.inclusionResources() {
				if inNamespaces(e.Namespaces, r) && !slices.Contains(m.prefs.Environments[i].Watchlist, r) {
					m.prefs.Environments[i].Watchlist = append(m.prefs.Environments[i].Watchlist, r)
				}
			}
			m.revision++
			return m.refresh()
		}
	case "save":
		return m.execute("save")
	case "quit":
		return m.quit()
	case "delete":
		m.prefs = m.prefs.Clone()
		m.prefs.Environments = slices.DeleteFunc(m.prefs.Environments, func(e config.Environment) bool { return e.ID == m.env.draft.ID })
		m.revision++
		if m.prefs.ActiveEnvironment == m.env.draft.ID {
			next, cmd := m.useEnvironment("")
			model := next.(Model)
			model.env.page = "switch"
			return model, cmd
		}
		m.env.page, m.env.index, m.env.query = "manage", 0, ""
	case "discard":
		m.env.dirty, m.env.page = false, "manage"
	case "save-quit":
		m.env.quitAfterSave = true
		return m.execute("save")
	case "discard-quit":
		m.stop()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) environmentHelpTopics() []terminalui.HelpTopic {
	return []terminalui.HelpTopic{
		{Group: "Environment / " + m.env.page, Keys: "Tab / Shift+Tab", Description: "Traverse visible fields and controls; Enter activates focused control"},
		{Group: "Environment", Keys: "Up / Down", Description: "Choose row; pointer row presses select only"},
		{Group: "Environment", Keys: "Esc / Close / outside press", Description: "Clear search first, then Back or staged-form cancellation"},
		{Group: "Environment", Keys: "Ctrl+P", Description: "Actions blocked while environment owns input; Up selects previous row"},
		{Group: "Environment", Description: "q/j/k/s and ? are literal text in fields; Apply changes this session, Save persists applied preferences"},
	}
}

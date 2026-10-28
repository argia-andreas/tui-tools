package ui

import (
	"context"
	"crypto/rand"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
)

type environmentState struct {
	page, back, editID, query, err string
	focus, focusPage               string
	draft                          config.Environment
	index, field                   int
	request                        uint64
	busy, dirty, quitAfterSave     bool
	subscriptions                  []explorer.AzureSubscription
	available, group               []explorer.Namespace
	details                        []string
}

type environmentLoaded struct {
	id      uint64
	results []explorer.NamespaceDiscovery
}
type environmentCounts struct {
	id     uint64
	counts counted
}
type catalogLoaded struct {
	id            uint64
	subscriptions []explorer.AzureSubscription
	namespaces    []explorer.Namespace
	known         *explorer.Namespace
	err           error
}

func inNamespaces(list []explorer.Namespace, r explorer.Resource) bool {
	return slices.ContainsFunc(list, func(n explorer.Namespace) bool {
		return n.Profile == r.Profile && strings.EqualFold(n.Address, r.Namespace) &&
			strings.EqualFold(n.AzureSubscriptionID, r.AzureSubscriptionID) && strings.EqualFold(n.ResourceGroup, r.ResourceGroup)
	})
}

func sameNamespace(a, b explorer.Namespace) bool {
	return a.Profile == b.Profile && strings.EqualFold(a.Address, b.Address) && strings.EqualFold(a.AzureSubscriptionID, b.AzureSubscriptionID) && strings.EqualFold(a.ResourceGroup, b.ResourceGroup)
}

func (m Model) environmentIndex() int {
	return slices.IndexFunc(m.prefs.Environments, func(e config.Environment) bool {
		return e.ID == m.prefs.ActiveEnvironment && e.Connection == m.connection
	})
}

func (m Model) environment() (config.Environment, bool) {
	if i := m.environmentIndex(); i >= 0 {
		return m.prefs.Environments[i], true
	}
	return config.Environment{}, false
}

func (m Model) discoverEnvironment() tea.Cmd {
	e, ok := m.environment()
	return func() tea.Msg {
		if !ok {
			return environmentLoaded{id: m.scopeID}
		}
		ctx, cancel := context.WithTimeout(m.scopeLife, 30*time.Second)
		defer cancel()
		return environmentLoaded{m.scopeID, m.provider.DiscoverScope(ctx, slices.Clone(e.Namespaces))}
	}
}

func (m Model) useEnvironment(id string) (tea.Model, tea.Cmd) {
	if m.review.phase != "" || m.reader.load != "" {
		m.status = "Finish or cancel the current review and acknowledge its outcome before switching"
		return m, nil
	}
	index := slices.IndexFunc(m.prefs.Environments, func(e config.Environment) bool { return e.ID == id && e.Connection == m.connection })
	if id != "" && index < 0 {
		m.env.err = "This environment uses another connection; restart with its explicit connection mode"
		return m, nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.scopeCancel != nil {
		m.scopeCancel()
	}
	m.resetInspector()
	if e, ok := m.environment(); ok {
		m.environmentItems[e.ID] = slices.Clone(m.items)
		if visible := m.visible(); m.selected < len(visible) {
			m.environmentSelection[e.ID] = visible[m.selected].resource
		}
	}
	m.scopeLife, m.scopeCancel = context.WithCancel(m.life)
	m.scopeID++
	m.tick++
	m.generation++
	m.refreshing, m.again, m.loading = false, false, id != ""
	m.items, m.scopeResults, m.page = slices.Clone(m.environmentItems[id]), nil, explorer.Page{}
	m.prefs.ActiveEnvironment = id
	if e, ok := m.environment(); ok {
		m.items = slices.DeleteFunc(m.items, func(i item) bool { return !inNamespaces(e.Namespaces, i.resource) })
	}
	m.revision++
	m.screen, m.focus, m.selected, m.nav = "dashboard", "content", 0, 0
	m.env.page, m.env.err, m.env.dirty = "", "", false
	m.env.query, m.env.index, m.env.field = "", 0, 0
	m.status = "Discovering only the selected environment's namespaces…"
	return m, m.discoverEnvironment()
}

func (m Model) environmentUpdate(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if m.provider == nil {
		return m, nil, false
	}
	switch msg := msg.(type) {
	case environmentLoaded:
		if msg.id != m.scopeID {
			return m, nil, true
		}
		m.loading = false
		e, active := m.environment()
		if !active {
			m.items = nil
			m.status = "Choose what to discover · Ctrl+P → New environment or Switch environment"
			return m, nil, true
		}
		old := m.items
		m.items, m.scopeResults = nil, nil
		// Treat omitted results as unverified, not successful empty listings.
		for _, ns := range e.Namespaces {
			result := explorer.NamespaceDiscovery{Namespace: ns, Err: fmt.Errorf("namespace was not verified")}
			for _, candidate := range msg.results {
				if sameNamespace(candidate.Namespace, ns) {
					result = candidate
					break
				}
			}
			m.scopeResults = append(m.scopeResults, result)
			if result.Err != nil {
				for _, i := range old {
					if inNamespaces([]explorer.Namespace{ns}, i.resource) {
						if i.counts.State == "available" || i.counts.State == "stale" {
							i.counts.State = "stale"
						}
						i.counts.Detail = "Discovery unverified: " + result.Err.Error()
						m.items = append(m.items, i)
					}
				}
				continue
			}
			for _, r := range result.Resources {
				if (r.Queue == "" && r.Subscription == "") || !inNamespaces([]explorer.Namespace{ns}, r) {
					continue
				}
				c := explorer.Counts{State: "unavailable", Detail: "not observed"}
				for _, previous := range old {
					if previous.resource == r {
						c = previous.counts
						break
					}
				}
				m.items = append(m.items, item{r, c})
			}
		}
		m.selected = min(m.selected, max(0, len(m.visible())-1))
		for index, i := range m.visible() {
			if i.resource == m.environmentSelection[e.ID] {
				m.selected = index
				break
			}
		}
		m.status = "Offline sample data · scoped discovery · browsing never receives or removes a message"
		if m.mode == "emulator" {
			m.status = "Emulator connected · runtime counts unavailable (emulator limitation) · scoped discovery"
		}
		if m.mode == "azure" {
			m.status = "Azure read-only · scoped discovery · message mutations disabled"
		}
		returnModel, cmd := m.refresh()
		return returnModel, cmd, true
	case environmentCounts:
		if msg.id != m.scopeID {
			return m, nil, true
		}
		next, cmd := m.Update(msg.counts)
		return next, cmd, true
	case catalogLoaded:
		if msg.id != m.env.request {
			return m, nil, true
		}
		m.env.busy = false
		if msg.err != nil {
			m.env.err = "Could not list/verify: " + msg.err.Error() + ". Retry or add a known namespace."
			return m, nil, true
		}
		m.env.err = ""
		if msg.known != nil {
			m = m.addNamespaces([]explorer.Namespace{*msg.known})
			m.env.page, m.env.query = "edit", ""
		} else if m.env.page == "subscriptions" {
			m.env.subscriptions = msg.subscriptions
		} else if m.env.page == "namespaces" {
			m.env.available = msg.namespaces
		}
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) environmentAction(id string) (tea.Model, tea.Cmd) {
	if m.review.phase != "" || m.reader.load != "" {
		m.status = "Finish or cancel the current review before changing environment"
		return m, nil
	}
	m.env.focus, m.env.focusPage = "", ""
	m.env.page, m.env.query, m.env.index, m.env.field, m.env.err = strings.TrimPrefix(id, "env:"), "", 0, 0, ""
	switch m.env.page {
	case "new":
		return m.editEnvironment(config.Environment{ID: "env-" + rand.Text(), Connection: m.connection}, true)
	case "edit-current":
		if e, ok := m.environment(); ok {
			return m.editEnvironment(e, false)
		}
		return m.editEnvironment(config.Environment{ID: "env-" + rand.Text(), Connection: m.connection}, true)
	case "refresh":
		m.env.page = ""
		if m.scopeCancel != nil {
			m.scopeCancel()
		}
		m.scopeLife, m.scopeCancel = context.WithCancel(m.life)
		m.scopeID++
		m.tick++
		m.refreshing, m.again = false, false
		m.loading = true
		return m, m.discoverEnvironment()
	case "details":
		m.env.details = []string{"Connection: " + m.connectionLabel}
		for _, result := range m.scopeResults {
			line := result.Namespace.Address + " · checked successfully"
			if result.Err != nil {
				line = result.Namespace.Address + " · Could not verify: " + result.Err.Error()
			}
			m.env.details = append(m.env.details, line)
		}
		m.env.back = ""
	}
	return m, nil
}

func (m Model) editEnvironment(e config.Environment, isNew bool) (tea.Model, tea.Cmd) {
	m.env.draft, m.env.editID = e, e.ID
	m.env.draft.Namespaces = slices.Clone(e.Namespaces)
	m.env.draft.Watchlist = slices.Clone(e.Watchlist)
	if isNew {
		m.env.editID = ""
	}
	m.env.page, m.env.field, m.env.index, m.env.query, m.env.err, m.env.dirty = "edit", 0, 0, "", "", false
	return m, nil
}

func (m Model) catalog(page, subscription string) (tea.Model, tea.Cmd) {
	m.env.page, m.env.query, m.env.index, m.env.busy, m.env.err = page, "", 0, true, ""
	m.env.request++
	m.env.available, m.env.subscriptions = nil, nil
	id, provider := m.env.request, m.provider
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.life, 30*time.Second)
		defer cancel()
		r := catalogLoaded{id: id}
		if page == "subscriptions" {
			r.subscriptions, r.err = provider.Subscriptions(ctx)
		} else {
			r.namespaces, r.err = provider.Namespaces(ctx, subscription)
		}
		return r
	}
}

func (m Model) addNamespaces(list []explorer.Namespace) Model {
	for _, ns := range list {
		if ns.Profile != m.connection {
			continue
		}
		if slices.ContainsFunc(m.env.draft.Namespaces, func(n explorer.Namespace) bool {
			return strings.EqualFold(n.Address, ns.Address) && !sameNamespace(n, ns)
		}) {
			m.env.err = "Address already selected with a different identity; review/remove it before adding again"
			continue
		}
		if !slices.ContainsFunc(m.env.draft.Namespaces, func(n explorer.Namespace) bool { return sameNamespace(n, ns) }) {
			m.env.draft.Namespaces = append(slices.Clone(m.env.draft.Namespaces), ns)
			m.env.dirty = true
		}
	}
	return m
}

func (m Model) applyEnvironment(use bool) (tea.Model, tea.Cmd) {
	e := m.env.draft
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" || len([]rune(e.Name)) > 40 {
		m.env.err = "Use a unique name between 1 and 40 characters"
		return m, nil
	}
	for _, other := range m.prefs.Environments {
		if other.ID != m.env.editID && strings.EqualFold(other.Name, e.Name) {
			m.env.err = "An environment with that name already exists"
			return m, nil
		}
	}
	if len(e.Namespaces) == 0 {
		m.env.err = "Choose at least one namespace before applying"
		return m, nil
	}
	m.prefs = m.prefs.Clone()
	i := slices.IndexFunc(m.prefs.Environments, func(old config.Environment) bool { return old.ID == m.env.editID })
	if i < 0 {
		m.prefs.Environments = append(m.prefs.Environments, e)
	} else {
		m.prefs.Environments[i] = e
	}
	m.env.dirty, m.env.page, m.env.err = false, "", ""
	m.revision++
	if use || m.prefs.ActiveEnvironment == e.ID {
		return m.useEnvironment(e.ID)
	}
	m.env.page = "manage"
	m.status = "Environment applied to this session · Save preferences to keep it"
	return m, nil
}

func editText(value string, msg tea.KeyMsg, limit int) string {
	switch msg.String() {
	case "backspace", "ctrl+h":
		r := []rune(value)
		return string(r[:max(0, len(r)-1)])
	case "ctrl+u":
		return ""
	default:
		if msg.Type == tea.KeyRunes {
			value += clean(string(msg.Runes), false)
		}
		if msg.Type == tea.KeySpace {
			value += " "
		}
		r := []rune(value)
		return string(r[:min(limit, len(r))])
	}
}

type namespaceChoice struct {
	namespace explorer.Namespace
	group     []explorer.Namespace
	label     string
}

func (m Model) namespaceChoices() []namespaceChoice {
	var rows []namespaceChoice
	for _, ns := range m.env.available {
		if !strings.Contains(strings.ToLower(ns.Name+" "+ns.ResourceGroup+" "+ns.Address), strings.ToLower(m.env.query)) {
			continue
		}
		rows = append(rows, namespaceChoice{namespace: ns, label: ns.ResourceGroup + " / " + ns.Name})
	}
	// Batch actions only exist over a successfully completed metadata listing.
	groups := map[string][]explorer.Namespace{}
	var order []string
	for _, choice := range rows {
		key := choice.namespace.ResourceGroup
		if _, found := groups[key]; !found {
			order = append(order, key)
		}
		groups[key] = append(groups[key], choice.namespace)
	}
	for _, group := range order {
		label := group
		if label == "" {
			label = "local namespaces"
		}
		rows = append(rows, namespaceChoice{group: groups[group], label: fmt.Sprintf("Preview %d listed namespaces in %s…", len(groups[group]), label)})
	}
	return rows
}

func (m Model) inclusionResources() []explorer.Resource {
	var list []explorer.Resource
	for _, i := range m.items {
		list = append(list, i.resource)
	}
	if e, ok := m.environment(); ok {
		for _, r := range e.Watchlist {
			if !slices.Contains(list, r) {
				list = append(list, r)
			}
		}
	}
	return slices.DeleteFunc(list, func(r explorer.Resource) bool {
		return !strings.Contains(strings.ToLower(r.Namespace+" / "+r.Name()), strings.ToLower(m.env.query))
	})
}

func (m Model) inclusionState(r explorer.Resource) string {
	e, _ := m.environment()
	if !inNamespaces(e.Namespaces, r) {
		return "Outside current discovery scope"
	}
	for _, result := range m.scopeResults {
		if inNamespaces([]explorer.Namespace{result.Namespace}, r) && result.Err != nil {
			return "Could not verify (last observation retained)"
		}
	}
	if slices.ContainsFunc(m.items, func(i item) bool { return i.resource == r }) {
		return ""
	}
	for _, result := range m.scopeResults {
		if inNamespaces([]explorer.Namespace{result.Namespace}, r) && result.Err == nil {
			return "Not found in successful listing"
		}
	}
	return "Could not verify"
}

func (m Model) changeInclusion(all, toggle bool) (tea.Model, tea.Cmd) {
	index := m.environmentIndex()
	if index < 0 {
		m.env.err = "Choose an environment first"
		return m, nil
	}
	m.prefs = m.prefs.Clone()
	e := &m.prefs.Environments[index]
	if toggle {
		e.All = all
	} else if !e.All {
		list := m.inclusionResources()
		if m.env.index < len(list) {
			r := list[m.env.index]
			if i := slices.Index(e.Watchlist, r); i >= 0 {
				e.Watchlist = slices.Delete(e.Watchlist, i, i+1)
			} else if inNamespaces(e.Namespaces, r) {
				e.Watchlist = append(e.Watchlist, r)
			}
		}
	}
	m.revision++
	m.selected = 0
	if m.refreshing {
		m.again = true
		return m, nil
	}
	return m.refresh()
}

// environmentCommand owns text, row movement and domain Back transitions only.
// Activation belongs to environmentDo's checked semantic operations.
func (m Model) environmentCommand(key string, text func(string, int) string) (tea.Model, tea.Cmd) {
	if text == nil {
		text = func(value string, _ int) string { return value }
	}
	page := m.env.page
	if page == "quit" {
		switch key {
		case "esc", "c":
			m.env.page = m.env.back
		case "d":
			m.stop()
			return m, tea.Quit
		case "s":
			if m.env.dirty {
				m.env.err = "Apply or cancel the environment form first; its draft is not saved"
				return m, nil
			}
			m.env.quitAfterSave = true
			return m.execute("save")
		}
		return m, nil
	}
	if m.width < 60 || m.height < 18 {
		return m, nil
	}
	if page == "discard" {
		if key == "esc" {
			m.env.page = m.env.back
		}
		return m, nil
	}
	if page == "details" {
		if key == "esc" || key == "enter" {
			m.env.page, m.env.index = m.env.back, 0
		}
		if key == "down" || key == "j" {
			m.env.index++
		}
		if key == "up" || key == "k" {
			m.env.index = max(0, m.env.index-1)
		}
		return m, nil
	}
	if key == "esc" {
		m.env.request++
		m.env.busy = false
		m.env.err = ""
		if m.env.query != "" {
			m.env.query, m.env.index = "", 0
			return m, nil
		}
		switch page {
		case "edit":
			if m.env.dirty {
				m.env.back, m.env.page = "edit", "discard"
			} else {
				m.env.page = "manage"
			}
		case "subscriptions", "namespaces", "selected", "known", "review":
			m.env.page, m.env.index, m.env.field = "edit", 0, 1
		case "group":
			m.env.page, m.env.index = "namespaces", 0
		case "all":
			m.env.page = "inclusion"
		case "delete":
			m.env.page = "manage"
		default:
			m.env.page = ""
		}
		return m, nil
	}
	if page == "edit" {
		if key == "tab" || key == "down" || key == "shift+tab" || key == "up" {
			step := 1
			if key == "shift+tab" || key == "up" {
				step = 5
			}
			m.env.field = (m.env.field + step) % 6
			return m, nil
		}
		if m.env.field == 0 && key != "enter" {
			old := m.env.draft.Name
			m.env.draft.Name = text(old, 40)
			m.env.dirty = m.env.dirty || old != m.env.draft.Name
			return m, nil
		}
		return m, nil
	}
	if page == "known" {
		m.env.query = text(m.env.query, 300)
		return m, nil
	}
	if page == "delete" {
		m.env.query = text(m.env.query, 40)
		return m, nil
	}
	if page == "review" {
		if key == "down" || key == "j" || key == "pgdown" {
			m.env.index = min(max(0, len(m.env.draft.Namespaces)-1), m.env.index+1)
		}
		if key == "up" || key == "k" || key == "pgup" {
			m.env.index = max(0, m.env.index-1)
		}
		return m, nil
	}
	if page == "group" {
		if key == "down" || key == "j" || key == "pgdown" {
			m.env.index = min(max(0, len(m.env.group)-1), m.env.index+1)
		}
		if key == "up" || key == "k" || key == "pgup" {
			m.env.index = max(0, m.env.index-1)
		}
		return m, nil
	}
	if page == "all" {
		return m, nil
	}
	if page == "inclusion" && m.env.field == 1 {
		if key == "tab" || key == "enter" {
			m.env.field = 0
			return m, nil
		}
		m.env.query = text(m.env.query, 128)
		m.env.index = 0
		return m, nil
	}
	count := 0
	switch page {
	case "switch", "manage":
		count = len(m.filteredEnvironments())
	case "subscriptions":
		count = len(m.filteredSubscriptions())
	case "namespaces":
		count = len(m.namespaceChoices())
	case "selected":
		count = len(m.env.draft.Namespaces)
	case "inclusion":
		count = len(m.inclusionResources())
	}
	if key == "down" || key == "ctrl+n" || ((page == "manage" || page == "selected" || page == "inclusion") && key == "j") {
		m.env.index = min(max(0, count-1), m.env.index+1)
		return m, nil
	}
	if key == "up" || ((page == "manage" || page == "selected" || page == "inclusion") && key == "k") {
		m.env.index = max(0, m.env.index-1)
		return m, nil
	}
	m.env.index = min(m.env.index, max(0, count-1))
	switch page {
	case "switch", "manage":
		if page == "switch" {
			m.env.query = text(m.env.query, 128)
			m.env.index = 0
		}
	case "subscriptions":
		m.env.query = text(m.env.query, 128)
		m.env.index = 0
	case "namespaces":
		m.env.query = text(m.env.query, 128)
		m.env.index = 0
	}
	return m, nil
}

func namespaceDetails(ns explorer.Namespace) []string {
	return []string{"Name: " + ns.Name, "Address: " + ns.Address, "Connection: " + ns.Profile, "Azure subscription: " + ns.AzureSubscriptionName, "Azure subscription ID: " + ns.AzureSubscriptionID, "Resource group: " + ns.ResourceGroup}
}

func (m Model) filteredEnvironments() []config.Environment {
	var list []config.Environment
	for _, e := range m.prefs.Environments {
		if strings.Contains(strings.ToLower(e.Name), strings.ToLower(m.env.query)) {
			list = append(list, e)
		}
	}
	return list
}

func (m Model) filteredSubscriptions() []explorer.AzureSubscription {
	var list []explorer.AzureSubscription
	for _, s := range m.env.subscriptions {
		if strings.Contains(strings.ToLower(s.Name+" "+s.ID), strings.ToLower(m.env.query)) {
			list = append(list, s)
		}
	}
	return list
}

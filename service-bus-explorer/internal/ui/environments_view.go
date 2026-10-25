package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
)

func environmentSize(e config.Environment) string {
	subscriptions := map[string]bool{}
	for _, n := range e.Namespaces {
		if n.AzureSubscriptionID != "" {
			subscriptions[n.AzureSubscriptionID] = true
		}
	}
	if strings.HasPrefix(e.Connection, "azure-cli:") {
		return fmt.Sprintf("%d namespaces · %d Azure subscriptions", len(e.Namespaces), len(subscriptions))
	}
	if e.Connection == "fixture" {
		return fmt.Sprintf("%d namespaces · OFFLINE fixture", len(e.Namespaces))
	}
	return fmt.Sprintf("%d namespaces · LOCAL emulator", len(e.Namespaces))
}

func (m Model) scopeSummary() string {
	e, ok := m.environment()
	if !ok {
		return "No active environment · Ctrl+P to create or switch"
	}
	good := 0
	for _, n := range e.Namespaces {
		if slices.ContainsFunc(m.scopeResults, func(r explorer.NamespaceDiscovery) bool { return sameNamespace(n, r.Namespace) && r.Err == nil }) {
			good++
		}
	}
	if m.loading {
		return environmentSize(e) + " · Discovering…"
	}
	if good != len(e.Namespaces) {
		return fmt.Sprintf("Partial discovery: %d of %d namespaces checked · Ctrl+P → Discovery details", good, len(e.Namespaces))
	}
	return environmentSize(e) + " · Ctrl+P → Switch environment"
}

func (m Model) environmentView(width, height int) []string {
	page := m.env.page
	title, intro, footer := "Environments", []string{}, []string{}
	var rows []string
	index := m.env.index
	switch page {
	case "switch", "manage":
		title = "Switch environment"
		if page == "manage" {
			title = "Settings / Environments"
		}
		intro = []string{"  Connection: " + m.connectionLabel}
		if page == "switch" {
			intro = append(intro, "  Search: "+m.env.query+"_")
		}
		for _, e := range m.filteredEnvironments() {
			mark := ""
			if e.ID == m.prefs.ActiveEnvironment {
				mark = " · CURRENT"
			}
			if e.Connection != m.connection {
				mark += " · other connection"
			}
			rows = append(rows, e.Name+mark+" / "+environmentSize(e))
		}
		if len(rows) == 0 {
			rows = []string{"No matching environments. Use Settings to create one."}
		}
		if page == "switch" {
			footer = []string{"  Type to filter · ↑/↓ choose · Enter Use · Tab Manage · Esc Cancel"}
		} else {
			footer = []string{"  n New · e Edit · d Duplicate · x Delete local preferences only", "  j/k Move · Enter Use environment · Esc Back"}
		}
	case "edit":
		title = "Settings / Environments / Edit"
		intro = []string{"  Staged form · applies only after review", "  Connection: " + m.connectionLabel, ""}
		name := m.env.draft.Name
		if m.env.field == 0 {
			name += "_"
		}
		rows = []string{"Name: [ " + name + " ]", "[ Browse Azure subscriptions / local catalog… ]", fmt.Sprintf("[ Review %d selected namespaces / remove / details… ]", len(m.env.draft.Namespaces)), "[ Add known namespace… ]", "[ Review & apply… ]", "[ Cancel ]"}
		index = m.env.field
		footer = []string{"  Group selection is a snapshot. New namespaces are not added automatically.", "  Tab/↑/↓ choose · Enter action · Esc cancel · Apply is not Save"}
	case "subscriptions":
		title = "Discovery scope / Choose Azure subscription"
		intro = []string{"  Lists namespace metadata only in the subscription you open.", "  Search: " + m.env.query + "_"}
		for _, s := range m.filteredSubscriptions() {
			rows = append(rows, s.Name+" / "+s.ID)
		}
		footer = []string{"  No listing access? Esc → Add known namespace instead.", "  Type search · ↑/↓ choose · Enter Open · Esc Back"}
	case "namespaces":
		title = "Discovery scope / Choose namespaces"
		intro = []string{fmt.Sprintf("  %d selected across opened subscriptions · Search: %s_", len(m.env.draft.Namespaces), m.env.query)}
		for _, choice := range m.namespaceChoices() {
			if choice.group != nil {
				rows = append(rows, "[ "+choice.label+" ]")
				continue
			}
			mark := "[ ] "
			if slices.ContainsFunc(m.env.draft.Namespaces, func(n explorer.Namespace) bool { return sameNamespace(n, choice.namespace) }) {
				mark = "[x] "
			}
			rows = append(rows, mark+choice.label)
		}
		footer = []string{"  New namespaces are not added automatically. Esc returns to the form.", "  Type search · ↑/↓ choose · Space/Enter Toggle or Preview · Esc Back"}
	case "selected", "review", "group":
		title = "Selected namespaces / Details and removal"
		list := m.env.draft.Namespaces
		footer = []string{"  Enter Full identity · Space/x Remove · ↑/↓ Move · Esc Back"}
		if page == "review" {
			title = "Review environment / " + m.env.draft.Name
			intro = []string{"  Exact namespace membership below. New namespaces are never added automatically."}
			footer = []string{"  [ Enter: Apply & use ]   [ a: Apply without switching ]   [ Esc: Back ]", "  Apply changes this session. Save preferences makes it durable."}
		}
		if page == "group" {
			title, list = "Preview resource-group selection / snapshot", m.env.group
			intro = []string{fmt.Sprintf("  Add only these %d listed namespaces. No live resource-group rule is saved.", len(list))}
			footer = []string{"  [ Enter: Add listed namespaces ]   [ Esc: Cancel ]", "  Future namespaces in this group will NOT be included."}
		}
		intro = append(intro, "  Namespace / resource group / Azure subscription ID")
		for _, n := range list {
			label := n.Address
			if n.AzureSubscriptionID != "" {
				label += " / " + n.ResourceGroup + " / " + n.AzureSubscriptionID
			}
			rows = append(rows, label)
		}
	case "known":
		title = "Add known namespace / no broad enumeration"
		intro = []string{"  Namespace address or full Azure resource ID; never a connection string.", "  This connection's operation permissions and network access still apply.", "  Unknown subscription/group metadata remains unknown until verified."}
		rows = []string{"[ " + m.env.query + "_ ]"}
		footer = []string{"  Type the namespace · Enter Verify/add · Esc Back"}
	case "inclusion":
		title = "Settings / Dashboard inclusion"
		e, ok := m.environment()
		if !ok {
			intro = []string{"  Choose an environment first · Ctrl+P → Switch environment"}
			break
		}
		mode := "Selected entities"
		if e.All {
			mode = "All entities in saved namespaces, including new entities"
		}
		intro = []string{"  Environment: " + e.Name + " · Mode: " + mode, "  Same inclusion for grid/table. Unwatched entities remain browsable in scope.", fmt.Sprintf("  Search: %s · %d matching results · / Edit search", m.env.query, len(m.inclusionResources()))}
		if m.env.field == 1 {
			intro[2] = "  Search: " + m.env.query + "_ · Enter returns to the list"
		}
		for _, r := range m.inclusionResources() {
			mark := "[ ] "
			if m.watched(r) {
				mark = "[x] "
			}
			if e.All && inNamespaces(e.Namespaces, r) {
				mark = "[All] "
			}
			if !inNamespaces(e.Namespaces, r) && slices.Contains(e.Watchlist, r) {
				mark = "[saved] "
			}
			line := mark + r.Namespace + " / " + r.Name()
			if status := m.inclusionState(r); status != "" {
				line += " · " + status
			}
			rows = append(rows, line)
		}
		footer = []string{"  Space/Enter Toggle · a Select matching · m All/Selected · / Search · v Full identity", "  s Save preferences (all pending edits) · Esc Back · q Quit"}
	case "all":
		title = "Dashboard inclusion / Enable All mode?"
		intro = []string{fmt.Sprintf("  %d discovered entities may appear, plus future entities in saved namespaces.", len(m.items)), "  This can produce a large dashboard. Namespace membership stays explicit.", "  Your last Selected list is retained; checkboxes are disabled in All mode."}
		footer = []string{"  [ Enter: Enable All entities ]   [ Esc: Keep Selected mode ]"}
	case "details":
		title = "Environment / Full identity and discovery status"
		for _, line := range m.env.details {
			rows = append(rows, strings.Split(ansi.Wrap(clean(line, false), max(1, width-8), ""), "\n")...)
		}
		footer = []string{"  ↑/↓ or j/k Scroll · Enter/Esc Back"}
	case "delete":
		title = "Delete saved environment / local preferences only"
		intro = []string{"  No Azure resources will be deleted. This removes the saved environment.", "  Type its exact name to confirm: " + m.env.draft.Name}
		rows = []string{m.env.query + "_"}
		footer = []string{"  Enter Delete local preferences · Esc Cancel"}
	case "discard":
		title = "Discard staged environment changes?"
		intro = []string{"  This form has unapplied changes. Other session preferences are unaffected."}
		footer = []string{"  [ Esc: Keep editing ]   [ Enter: Discard form ]"}
	case "quit":
		title = "Unsaved changes / Quit?"
		intro = []string{"  Changes have not been saved. Save persists all applied preferences.", "  A staged environment form must be applied before it can be saved."}
		footer = []string{"  [ Esc/c: Cancel ]   [ s: Save preferences & quit ]   [ d: Discard & quit ]"}
	}
	if width < 90 {
		switch page {
		case "switch":
			footer = []string{"  Type search · ↑/↓ · Enter Use · Tab Manage · Esc"}
		case "manage":
			footer = []string{"  n New · e Edit · d Copy · x Delete saved only", "  j/k Move · Enter Use · Esc Back"}
		case "edit":
			footer = []string{"  Snapshot only: future namespaces excluded.", "  Tab/↑/↓ choose · Enter action · Esc cancel"}
		case "review":
			footer = []string{"  Enter Apply & use · a Apply only · Esc Back", "  Apply ≠ Save · Save preferences in Settings"}
		case "group":
			footer = []string{"  Enter Add listed · Esc Cancel", "  Future namespaces NOT included"}
		case "inclusion":
			footer = []string{"  Space toggle · a matching · m mode · / search", "  v Details · s Save all edits · Esc Back · q Quit"}
		case "quit":
			footer = []string{"  Esc/c Cancel · s Save & quit · d Discard & quit"}
		}
	}
	if m.env.busy {
		rows = []string{"Loading metadata… (no queues are scanned here)"}
	}
	if len(rows) == 0 && (page == "namespaces" || page == "subscriptions") && !m.env.busy {
		rows = []string{"No matching results; clear search or add a known namespace."}
	}
	lines := []string{}
	for _, line := range intro {
		lines = append(lines, m.paint(clean(line, false), m.theme.Muted))
	}
	lines = append(lines, "")
	capacity := max(1, height-len(lines)-len(footer)-5)
	index = min(index, max(0, len(rows)-1))
	start := 0
	if page == "details" {
		start = min(m.env.index, max(0, len(rows)-capacity))
	} else {
		start = index / capacity * capacity
	}
	for i := start; i < min(len(rows), start+capacity); i++ {
		line := "  " + clean(rows[i], false)
		if i == index && page != "details" {
			lines = append(lines, m.selectedLine("› "+clean(rows[i], false), width-2))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	for len(lines) < height-len(footer)-4 {
		lines = append(lines, "")
	}
	if m.env.err != "" {
		lines = append(lines, m.paint("  "+clean(m.env.err, false), m.theme.Warning))
	} else {
		lines = append(lines, "")
	}
	for _, line := range footer {
		lines = append(lines, m.paint(line, m.theme.Focus))
	}
	return m.panel(title, width, height, lines, true)
}

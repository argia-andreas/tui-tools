package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"terminalui"
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
	bounds := terminalui.Rect{Y: 3, W: width, H: height}
	return terminalui.RenderSurface(m.environmentSurface(width, height), bounds, m.theme.TerminalStyle(m.ascii)).Lines
}

func (m Model) environmentSurface(width, height int) terminalui.SurfaceSpec {
	bounds := terminalui.Rect{Y: 3, W: width, H: height}
	body := terminalui.SurfaceBodyBounds(bounds)
	focus := m.environmentFocus()
	spec := terminalui.SurfaceSpec{ID: "environment", CancelID: "back", FocusID: focus,
		Actions: []terminalui.Action{{ID: "back", Label: "Back / Cancel"}}, Body: terminalui.Frame{Bounds: body}}
	page := m.env.page
	title, intro := "Environments", []string{}
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
	case "edit":
		title = "Settings / Environments / Edit"
		intro = []string{"  Staged form · applies only after review", "  Connection: " + m.connectionLabel}
		name := m.env.draft.Name
		if m.env.field == 0 {
			name += "_"
		}
		rows = []string{"Name: [ " + name + " ]", "[ Browse Azure subscriptions / local catalog… ]", fmt.Sprintf("[ Review %d selected namespaces / remove / details… ]", len(m.env.draft.Namespaces)), "[ Add known namespace… ]", "[ Review & apply… ]", "[ Cancel ]"}
		index = m.env.field
	case "subscriptions":
		title = "Discovery scope / Choose Azure subscription"
		intro = []string{"  Lists namespace metadata only in the subscription you open.", "  Search: " + m.env.query + "_"}
		for _, s := range m.filteredSubscriptions() {
			rows = append(rows, s.Name+" / "+s.ID)
		}
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
	case "selected", "review", "group":
		title = "Selected namespaces / Details and removal"
		list := m.env.draft.Namespaces
		if page == "review" {
			title = "Review environment / " + m.env.draft.Name
			intro = []string{"  Exact namespace membership below. New namespaces are never added automatically."}
		}
		if page == "group" {
			title, list = "Preview resource-group selection / snapshot", m.env.group
			intro = []string{fmt.Sprintf("  Add only these %d listed namespaces. No live resource-group rule is saved.", len(list))}
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
	case "all":
		title = "Dashboard inclusion / Enable All mode?"
		intro = []string{fmt.Sprintf("  %d discovered entities may appear, plus future entities in saved namespaces.", len(m.items)), "  This can produce a large dashboard. Namespace membership stays explicit.", "  Your last Selected list is retained; checkboxes are disabled in All mode."}
	case "details":
		title = "Environment / Full identity and discovery status"
		for _, line := range m.env.details {
			rows = append(rows, strings.Split(ansi.Wrap(clean(line, false), max(1, width-8), ""), "\n")...)
		}
	case "delete":
		title = "Delete saved environment / local preferences only"
		intro = []string{"  No Azure resources will be deleted. This removes the saved environment.", "  Type its exact name to confirm: " + m.env.draft.Name}
		rows = []string{m.env.query + "_"}
	case "discard":
		title = "Discard staged environment changes?"
		intro = []string{"  This form has unapplied changes. Other session preferences are unaffected."}
	case "quit":
		title = "Unsaved changes / Quit?"
		intro = []string{"  Changes have not been saved. Save persists all applied preferences.", "  A staged environment form must be applied before it can be saved."}
	}
	if m.env.busy {
		rows = []string{"Loading metadata… (no queues are scanned here)"}
	}
	if len(rows) == 0 && (page == "namespaces" || page == "subscriptions") && !m.env.busy {
		rows = []string{"No matching results; clear search or add a known namespace."}
	}
	lines := []string{}
	addRegion := func(id string, kind terminalui.RegionKind, x, w int) {
		spec.Body.Regions = append(spec.Body.Regions, terminalui.Region{ID: id, Kind: kind,
			Bounds: terminalui.Rect{X: body.X + x, Y: body.Y + len(lines), W: w, H: 1}})
	}
	for i, line := range intro {
		if (page == "switch" && i == 1) || (page == "subscriptions" && i == 1) || (page == "namespaces" && i == 0) || (page == "inclusion" && i == 2) {
			addRegion("search", terminalui.FieldRegion, 0, body.W)
			spec.FocusOrder = append(spec.FocusOrder, "search")
		}
		lines = append(lines, m.paint(clean(line, false), m.theme.Muted))
	}
	// Explicit controls wrap as whole buttons. The geometry comes from the same
	// RenderActionBar call used to append the visible line, never label parsing.
	controls := m.environmentControls()
	for len(controls) > 0 {
		bar := terminalui.RenderActionBar(controls, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1}, focus, m.theme.TerminalStyle(m.ascii))
		if len(bar.Regions) == 0 {
			break
		}
		lines = append(lines, bar.Lines[0])
		spec.Body.Regions = append(spec.Body.Regions, bar.Regions...)
		for _, r := range bar.Regions {
			spec.FocusOrder = append(spec.FocusOrder, r.ID)
		}
		controls = controls[len(bar.Regions):]
	}
	capacity := max(1, body.H-len(lines)-1)
	index = min(index, max(0, len(rows)-1))
	start := 0
	if page == "details" {
		start = min(m.env.index, max(0, len(rows)-capacity))
	} else {
		start = index / capacity * capacity
	}
	for i := start; i < min(len(rows), start+capacity); i++ {
		ids := m.environmentRows()
		if !m.env.busy && i < len(ids) {
			addRegion(ids[i], terminalui.RowRegion, 0, body.W)
			if page == "namespaces" && m.namespaceChoices()[i].group == nil || page == "inclusion" {
				enabled := true
				if e, ok := m.environment(); page == "inclusion" && ok && e.All {
					enabled = false
				}
				if enabled {
					addRegion("check:"+ids[i], terminalui.ControlRegion, 2, 3)
				}
			}
		}
		if !m.env.busy && page == "edit" {
			id := []string{"name", "browse", "selected", "known", "review", "back"}[i]
			kind := terminalui.ControlRegion
			if i == 0 {
				kind = terminalui.FieldRegion
			}
			addRegion(id, kind, 0, body.W)
			spec.FocusOrder = append(spec.FocusOrder, id)
		}
		if !m.env.busy && (page == "known" || page == "delete") {
			id := "search"
			if page == "delete" {
				id = "authorization"
			}
			addRegion(id, terminalui.FieldRegion, 0, body.W)
			spec.FocusOrder = append(spec.FocusOrder, id)
		}
		line := "  " + clean(rows[i], false)
		if i == index && page != "details" {
			lines = append(lines, m.selectedLine("› "+clean(rows[i], false), width-2))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	for len(lines) < body.H-1 {
		lines = append(lines, "")
	}
	if m.env.err != "" {
		lines = append(lines, m.paint("  "+clean(m.env.err, false), m.theme.Warning))
	} else {
		lines = append(lines, m.paint("Tab controls · Enter focused action · Esc Back", m.theme.Focus))
	}
	if page != "edit" && page != "known" && page != "delete" {
		spec.Body.Regions = append(spec.Body.Regions, terminalui.Region{ID: "list", Kind: terminalui.PaneRegion, Bounds: body})
		if !environmentConfirmation(page) {
			spec.FocusOrder = append([]string{"list"}, spec.FocusOrder...)
		}
	}
	order := spec.FocusOrder
	spec.FocusOrder = nil
	if environmentConfirmation(page) {
		spec.FocusOrder = append(spec.FocusOrder, "back")
		if page == "delete" {
			spec.FocusOrder = append(spec.FocusOrder, "authorization")
		}
	}
	for _, id := range append(order, "back") {
		if !slices.Contains(spec.FocusOrder, id) {
			spec.FocusOrder = append(spec.FocusOrder, id)
		}
	}
	spec.Title, spec.Body.Lines = title, lines
	return spec
}

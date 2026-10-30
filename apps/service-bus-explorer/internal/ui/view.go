package ui

import (
	"fmt"
	"slices"
	"strings"
	"terminalui"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/explorer"
)

// Broker text is data, not terminal instructions. Preserve only printable text
// and explicitly allowed body newlines; do not pass OSC/CSI through to the TTY.
func clean(s string, multiline bool) string {
	return strings.Map(func(r rune) rune {
		if multiline && r == '\n' {
			return r
		}
		if unicode.IsControl(r) || r == '\u202e' || r == '\u202d' || r == '\u2066' || r == '\u2067' || r == '\u2068' || r == '\u2069' {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "…")
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}

func (m Model) paint(s, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(lipgloss.Color(m.theme.Background)).Render(s)
}

func (m Model) selectedLine(s string, width int) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(fit(s, width))
}

func (m Model) button(label string, focused bool, color string) string {
	prefix, foreground, background := "  ", color, m.theme.Background
	if focused {
		prefix, foreground, background = "› ", m.theme.SelectionText, m.theme.Selection
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(color)).BorderBackground(lipgloss.Color(m.theme.Background)).
		Foreground(lipgloss.Color(foreground)).Background(lipgloss.Color(background)).
		Padding(0, 1).Render(prefix + label)
}

func (m Model) panel(title string, width, height int, lines []string, focused bool) []string {
	color := m.theme.Border
	if focused {
		color = m.theme.Focus
	}
	bar := func(s string) string { return m.paint(s, color) }
	inner := max(0, width-2)
	top := ansi.Truncate("─ "+clean(title, false)+" ", inner, "…")
	top += strings.Repeat("─", max(0, inner-lipgloss.Width(top)))
	result := []string{bar("┌" + top + "┐")}
	for row := 0; row < height-2; row++ {
		line := ""
		if row < len(lines) {
			line = lines[row]
		}
		line = ansi.Truncate(line, inner, "…")
		result = append(result, bar("│")+line+m.paint(strings.Repeat(" ", max(0, inner-lipgloss.Width(line))), m.theme.Foreground)+bar("│"))
	}
	return append(result, bar("└"+strings.Repeat("─", inner)+"┘"))
}

func (m Model) sidebarView(height int, frames ...*terminalui.Frame) []string {
	body := panelBody(terminalui.Rect{Y: 3, W: 24, H: height})
	addHit(frames, body, "navigation", terminalui.PaneRegion, body)
	var lines []string
	for i, label := range []string{"Dashboard", "Queues", "Topics", "Settings"} {
		addHit(frames, body, "nav:"+strings.ToLower(label), terminalui.DestinationRegion, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1})
		prefix := "  "
		active := m.screen == strings.ToLower(label) || (m.screen == "messages" && m.returnScreen == strings.ToLower(label))
		if active {
			prefix = "› "
		}
		line := prefix + label
		if (m.focus == "navigation" && i == m.nav) || m.focus == "nav:"+strings.ToLower(label) {
			lines = append(lines, m.selectedLine("› "+label, 22))
		} else if active {
			lines = append(lines, m.paint(line, m.theme.Focus))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	lines = append(lines, "", m.paint("  RESOURCES / peek", m.theme.Muted))
	rows := m.tree()
	capacity := max(1, height-8)
	start := max(0, m.nav-4-capacity+1)
	for index := start; index < min(len(rows), start+capacity); index++ {
		row := rows[index]
		prefix := "  "
		if !row.leaf {
			prefix = "▾ "
			if m.collapsed[row.branch] {
				prefix = "▸ "
			}
		}
		line := strings.Repeat(" ", row.depth*2) + prefix + clean(row.label, false)
		id, kind := "branch-row:"+branchID(row.branch), terminalui.RowRegion
		if row.leaf {
			id, kind = "tree:"+resourceID(row.resource), terminalui.DestinationRegion
		} else {
			addHit(frames, body, branchID(row.branch), terminalui.ControlRegion, terminalui.Rect{X: body.X + row.depth*2, Y: body.Y + len(lines), W: 2, H: 1})
		}
		addHit(frames, body, id, kind, terminalui.Rect{X: body.X + row.depth*2, Y: body.Y + len(lines), W: body.W - row.depth*2, H: 1})
		if (m.focus == "navigation" && m.nav == index+4) || m.focus == id || m.focus == branchID(row.branch) {
			lines = append(lines, m.selectedLine(line, 22))
		} else {
			color := m.theme.Foreground
			if !row.leaf {
				color = m.theme.Focus
			}
			lines = append(lines, m.paint(line, color))
		}
	}
	return m.panel("WORKSPACE", 24, height, lines, m.workspacePane() == "navigation")
}

func (m Model) counter(value int64, state, color string, width int) string {
	text := fmt.Sprint(value)
	if state == "stale" {
		text += "~"
		color = m.theme.Warning
	} else if state != "available" {
		text, color = "—", m.theme.Warning
	}
	if width > 0 {
		text = fit(text, width)
	}
	return m.paint(text, color)
}

func (m Model) dashboardView(width, height int, frames ...*terminalui.Frame) []string {
	body := panelBody(m.contentBounds())
	addHit(frames, body, "content", terminalui.PaneRegion, body)
	items := m.visible()
	title := "Dashboard"
	if m.screen == "queues" {
		title = "Queues"
	}
	if m.screen == "topics" {
		title = "Topic subscriptions"
	}
	if m.screen == "settings" {
		return m.settingsView(width, height, frames...)
	}
	if m.loading {
		return m.panel(title, width, height, []string{"", m.paint("  Discovering resources…", m.theme.Muted)}, true)
	}
	if m.provider != nil {
		if _, ok := m.environment(); !ok {
			return m.panel(title, width, height, []string{"", m.paint("  Choose what to discover", m.theme.Focus), "",
				m.paint("  Create a named environment and choose its namespaces.", m.theme.Foreground),
				m.paint("  No queues or topics have been scanned for this connection.", m.theme.Muted), "",
				m.paint("  Ctrl+P → New environment / Switch environment", m.theme.Focus)}, true)
		}
	}
	var lines []string
	mode := "GRID"
	if m.prefs.Table || m.screen != "dashboard" {
		mode = "TABLE"
	}
	scope := "watched entities"
	if m.screen != "dashboard" {
		scope = "discovered entities"
	}
	lines = append(lines, m.paint(fmt.Sprintf("  %s  /  %d %s", mode, len(items), scope), m.theme.Foreground))
	bar := terminalui.RenderActionBar([]terminalui.Action{
		{ID: "active", Label: "Open Active", Unavailable: emptyReason(len(items))},
		{ID: "dlq", Label: "Peek DLQ", Unavailable: emptyReason(len(items))},
		{ID: "grid", Label: "Grid g"}, {ID: "table", Label: "Table t"},
	}, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1}, m.focus, m.theme.TerminalStyle(m.ascii))
	lines = append(lines, bar.Lines[0], "")
	if len(frames) > 0 {
		frames[0].Regions = append(frames[0].Regions, bar.Regions...)
	}
	if len(items) == 0 {
		message := "  Nothing watched. Open Settings with Ctrl+P to choose entities."
		if m.provider != nil {
			if m.screen == "dashboard" && len(m.items) > 0 {
				message = fmt.Sprintf("  Nothing on this dashboard. %d scoped entities available.", len(m.items))
			} else if slices.ContainsFunc(m.scopeResults, func(r explorer.NamespaceDiscovery) bool { return r.Err != nil }) {
				message = "  No matching entities verified. Some namespaces could not be checked."
			} else {
				message = "  No matching entities found in successfully checked namespaces."
			}
		}
		lines = append(lines, m.paint(message, m.theme.Warning))
	}
	if mode == "TABLE" {
		nameWidth := max(16, width-52)
		lines = append(lines, m.paint("  "+fit("ENTITY", nameWidth)+fit("QUEUED", 10)+fit("DEAD-LETTER", 14)+"NAMESPACE", m.theme.Muted), "")
		capacity := max(1, height-9)
		start := m.selected / capacity * capacity
		for index := start; index < min(len(items), start+capacity); index++ {
			i := items[index]
			addHit(frames, body, resourceID(i.resource), terminalui.RowRegion, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1})
			prefix := "  "
			if index == m.selected {
				prefix = "› "
			}
			line := m.paint(prefix+fit(clean(i.resource.Name(), false), nameWidth), m.theme.Foreground) + m.counter(i.counts.Active, i.counts.State, m.theme.Queued, 10) + m.counter(i.counts.DeadLetter, i.counts.State, m.theme.DeadLetter, 14) + m.paint(clean(i.resource.Namespace, false), m.theme.Muted)
			if index == m.selected && m.focus == "content" {
				line = m.selectedLine(ansi.Strip(line), width-2)
			}
			lines = append(lines, line)
		}
	} else {
		columns := min(3, max(1, (width-4)/35))
		rows := min(4, max(1, (height-7)/8))
		capacity := columns * rows
		start := m.selected / capacity * capacity
		cardWidth := (width - 4 - (columns-1)*2) / columns
		for row := 0; row < rows; row++ {
			var cards [][]string
			for col := 0; col < columns; col++ {
				index := start + row*columns + col
				if index >= len(items) {
					break
				}
				i := items[index]
				addHit(frames, body, resourceID(i.resource), terminalui.RowRegion, terminalui.Rect{X: body.X + 1 + col*(cardWidth+2), Y: body.Y + len(lines), W: cardWidth, H: 7})
				label := clean(i.resource.Name(), false)
				selected := index == m.selected && m.focus == "content"
				if selected {
					label = "› " + label
				}
				content := []string{m.paint(" "+clean(i.resource.Namespace, false), m.theme.Muted), "", m.paint(" ", m.theme.Muted) + m.counter(i.counts.Active, i.counts.State, m.theme.Queued, 0) + m.paint(" queued   ", m.theme.Muted) + m.counter(i.counts.DeadLetter, i.counts.State, m.theme.DeadLetter, 0) + m.paint(" dead-letter", m.theme.Muted), "", m.paint(" Enter active    ] DLQ", m.theme.Muted)}
				cards = append(cards, m.panel(label, cardWidth, 7, content, selected))
			}
			for line := 0; line < 7 && len(cards) > 0; line++ {
				parts := []string{m.paint(" ", m.theme.Foreground)}
				for col, card := range cards {
					if col > 0 {
						parts = append(parts, m.paint("  ", m.theme.Foreground))
					}
					parts = append(parts, card[line])
				}
				lines = append(lines, strings.Join(parts, ""))
			}
			lines = append(lines, "")
		}
	}
	lines = append(lines, m.paint(fmt.Sprintf("  Selected %d/%d · j/k move · more entities scroll into view", min(m.selected+1, len(items)), len(items)), m.theme.Muted))
	if len(items) > 0 {
		c := items[m.selected].counts
		state := "  Counts: " + c.State
		if !c.ObservedAt.IsZero() {
			state += " · observed " + c.ObservedAt.UTC().Format("15:04:05 UTC")
		}
		if c.Detail != "" {
			state += " · " + clean(c.Detail, false)
		}
		if m.refreshing {
			state += " · refreshing…"
		}
		lines = append(lines, m.paint(state, m.theme.Muted))
	}
	return m.panel(title, width, height, lines, m.focus == "content")
}

func (m Model) messageView(width, height int, frames ...*terminalui.Frame) []string {
	view := "ACTIVE"
	if m.dlq {
		view = "DEAD-LETTER"
	}
	actions := "Actions ^P"
	if width < 70 {
		actions = "More ^P"
	}
	gap := m.paint(" \n \n ", m.theme.Foreground)
	toolbar := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top,
		m.button("Active [", m.focus == "active", m.theme.Focus), gap,
		m.button("DLQ ]", m.focus == "dlq", m.theme.Focus), gap,
		m.button("Refresh r", m.focus == "refresh", m.theme.Focus), gap,
		m.button(actions, m.focus == "actions", m.theme.Focus)), "\n")
	for i, line := range toolbar {
		toolbar[i] = line + m.paint(strings.Repeat(" ", max(0, width-lipgloss.Width(line))), m.theme.Foreground)
	}
	if m.review.selected && (m.height < 35 || m.inspectorWidth() < 80) || m.compactReading() {
		toolbar = nil
	}
	b := m.contentBounds()
	if len(toolbar) > 0 {
		x := b.X
		for _, control := range []struct{ id, label string }{{"active", "Active ["}, {"dlq", "DLQ ]"}, {"refresh", "Refresh r"}, {"actions", actions}} {
			w := lipgloss.Width(m.button(control.label, false, m.theme.Focus))
			addHit(frames, b, control.id, terminalui.ControlRegion, terminalui.Rect{X: x, Y: b.Y, W: w, H: 3})
			x += w + 1
		}
	}
	height -= len(toolbar)
	listWidth := width
	showInspector := m.hasInspector()
	if showInspector && m.splitInspector() {
		listWidth = width * 40 / 100
	}
	if showInspector && (!m.splitInspector() || m.review.selected && (width-listWidth < 80 || m.height < 35)) {
		if m.review.selected {
			return append(toolbar, m.reviewView(width, height)...)
		}
		return append(toolbar, m.readerView(width, height, frames...)...)
	}
	body := panelBody(terminalui.Rect{X: b.X, Y: b.Y + len(toolbar), W: listWidth, H: height})
	addHit(frames, body, "content", terminalui.PaneRegion, body)
	lines := []string{m.paint("  PEEK ONLY · no locks or settlement", m.theme.Queued), m.paint("  "+clean(m.resource.Profile+" / "+m.resource.Namespace, false), m.theme.Muted)}
	if m.loading {
		lines = append(lines, m.paint("  Loading messages…", m.theme.Muted))
	} else if len(m.page.Messages) == 0 {
		lines = append(lines, m.paint("  No messages in this peek page", m.theme.Muted))
	}
	capacity := max(1, height-9)
	start := m.message / capacity * capacity
	for index := start; index < min(len(m.page.Messages), start+capacity); index++ {
		msg := m.page.Messages[index]
		if !m.loading {
			addHit(frames, body, m.messageID(index), terminalui.RowRegion, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1})
		}
		prefix := "  "
		if index == m.message {
			prefix = "› "
		}
		line := fmt.Sprintf("%s%-6d %s", prefix, msg.Sequence, clean(msg.ID, false))
		if index == m.message && m.focus == "content" && m.review.phase == "" {
			lines = append(lines, m.selectedLine(line, listWidth-2))
		} else if index == m.message {
			lines = append(lines, m.paint(line, m.theme.Focus))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	lines = append(lines, "")
	bar := terminalui.RenderActionBar([]terminalui.Action{
		{ID: "inspect", Label: "Inspect / Open", Unavailable: emptyReason(len(m.page.Messages)), Busy: m.loading},
		{ID: "next", Label: "Next n", Unavailable: unavailable(m.page.NextSequence == nil)},
		{ID: "previous", Label: "Previous p", Unavailable: unavailable(len(m.history) == 0)},
	}, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1}, m.focus, m.theme.TerminalStyle(m.ascii))
	if bar.Bounds.Y < body.Y+body.H {
		lines = append(lines, bar.Lines[0])
		if len(frames) > 0 {
			frames[0].Regions = append(frames[0].Regions, bar.Regions...)
		}
	}
	if !showInspector && len(m.page.Messages) > 0 {
		msg := m.page.Messages[m.message]
		lines = append(lines, "", m.paint("  "+clean(msg.Subject, false), m.theme.Focus))
		if m.dlq {
			lines = append(lines, m.paint("  "+clean(msg.DeadLetterReason, false), m.theme.DeadLetter))
		}
	}
	left := m.panel(clean(m.resource.Name(), false)+" / "+view, listWidth, height, lines, m.focus == "content")
	if !showInspector {
		return append(toolbar, left...)
	}
	var right []string
	if m.review.selected {
		right = m.reviewView(width-listWidth, height)
	} else {
		right = m.readerView(width-listWidth, height, frames...)
	}
	for i := range left {
		left[i] += right[i]
	}
	return append(toolbar, left...)
}

func (m Model) compactReading() bool {
	return m.env.page == "" && m.review.phase == "" && m.inspectorFocused() && (!m.splitInspector() || m.height < 25)
}

// Dock bounds mirror messageView's app-owned layout. Components render and
// handle the same current rectangle, including full-width compact readers.
func (m Model) readerBounds() terminalui.Rect {
	b := terminalui.Rect{Y: 3, W: m.contentWidth(), H: m.height - 6}
	if m.showSidebar() {
		b.X = 24
	}
	if m.compactReading() {
		b.Y, b.H = 1, m.height-2
	}
	if !(m.review.selected && (m.height < 35 || m.inspectorWidth() < 80)) && !m.compactReading() {
		b.Y, b.H = b.Y+3, b.H-3
	}
	listWidth := b.W * 40 / 100
	if m.splitInspector() && !(m.review.selected && (b.W-listWidth < 80 || m.height < 35)) {
		b.X, b.W = b.X+listWidth, b.W-listWidth
	}
	if !m.workspaceFits() || !m.hasInspector() {
		return terminalui.Rect{}
	}
	return b
}

func (m Model) reviewBounds() terminalui.Rect {
	if m.review.selected {
		return m.readerBounds()
	}
	if !m.workspaceFits() {
		return terminalui.Rect{}
	}
	return terminalui.Rect{Y: 3, W: m.width, H: m.height - 6}
}

func (m Model) View() string {
	if !m.workspaceFits() {
		lines := []string{fmt.Sprintf("Resize terminal to at least 60×%d. Ctrl+C quits safely.", m.minimumHeight())}
		if m.compactQuitVisible() {
			lines = append(lines, "Unsaved changes: Esc cancel / s save / d discard")
			if m.env.err != "" {
				lines = append(lines, clean(m.env.err, false))
			}
		}
		lines = lines[:min(len(lines), max(0, m.height))]
		for i := range lines {
			lines[i] = m.paint(fit(lines[i], max(0, m.width)), m.theme.Warning)
		}
		return strings.Join(lines, "\n")
	}
	header := "  SBX / Service Bus Explorer"
	badge := "FIXTURE · OFFLINE  "
	subtitle := "  Safe browsing workspace · sample data, no Azure connection"
	if m.mode == "emulator" {
		badge = "EMULATOR · LOCAL  "
		subtitle = "  Real local broker · safe peek by default · explicit DLQ review · — counts unavailable"
	}
	if m.mode == "azure" {
		badge = "AZURE · READ ONLY  "
		subtitle = "  Explicit Azure CLI connection · no cloud message mutations"
	}
	if m.provider != nil {
		name := "None"
		if e, ok := m.environment(); ok {
			name = e.Name
		}
		header = "  SBX / Environment: " + clean(name, false)
		subtitle = "  " + m.scopeSummary()
	}
	lines := []string{m.paint(fit(header, m.width-lipgloss.Width(badge)), m.theme.Focus) + m.paint(badge, m.theme.Warning), m.paint(fit(subtitle, m.width), m.theme.Muted), m.paint(strings.Repeat("─", m.width), m.theme.Border)}
	height, width := m.height-6, m.width
	if m.compactReading() {
		context := "  SBX / "
		if e, ok := m.environment(); m.provider != nil && ok {
			context += clean(e.Name, false) + " / "
		}
		lines = []string{m.paint(fit(context+clean(m.resource.Name(), false)+" / "+map[bool]string{true: "DEAD-LETTER", false: "ACTIVE"}[m.dlq]+" · PEEK ONLY", m.width), m.theme.Focus)}
		height = m.height - 2
	}
	showSidebar := m.showSidebar()
	if showSidebar {
		width -= 24
	}
	var content []string
	if m.env.page != "" && m.review.phase == "" {
		content = m.environmentView(width, height)
	} else if m.review.phase != "" && !m.review.selected {
		content = m.reviewView(width, height)
	} else if m.review.phase != "" {
		content = m.messageView(width, height)
	} else {
		content = m.workspaceFrame().Lines
	}
	if showSidebar && m.review.phase != "" {
		nav := m.sidebarView(height)
		for i := range content {
			content[i] = nav[i] + content[i]
		}
	}
	lines = append(lines, content...)
	keys := "  j/k move   Enter open   ] DLQ   g/t layout   Tab/Ctrl+H/L focus   b sidebar   Ctrl+P actions   q quit"
	if m.screen == "messages" {
		keys = "  j/k move   Enter inspect   [/] view   i inspector   n/p pages   Tab/Ctrl+H/L focus   Ctrl+P actions   Esc back   q quit"
	} else if m.screen == "settings" {
		keys = "  j/k move   Enter change   s save   Tab/Ctrl+H/L focus   Ctrl+P actions   Esc back   q quit"
	}
	if m.focus == "navigation" {
		keys = "  j/k move   h/l collapse/expand   Enter open   ] DLQ   Tab/Ctrl+H/L focus   Ctrl+P actions   q quit"
	}
	if m.width < 110 {
		keys = "  j/k move  Enter open  ] DLQ  Ctrl+P actions  q quit"
		if m.screen == "messages" {
			keys = "  j/k move  Enter inspect  [/] view  n/p pages  Ctrl+P  Esc back  q quit"
		} else if m.screen == "settings" {
			keys = "  j/k move  Enter change  s save  Ctrl+P  Esc back  q quit"
		}
	}
	if m.inspectorFocused() {
		keys = "  h/j/k/l cursor · yy line · v/V select · Y full body · Tab actions · Esc Back · Ctrl+P · ? help · q quit"
	}
	status := m.status
	if m.env.page != "" {
		keys = "  Follow the focused panel's keys · input fields own printable keys · Ctrl+C quit safely"
		if m.width < 110 {
			keys = "  Panel keys above · input owns text · Ctrl+C quit safely"
		}
	}
	if m.review.phase != "" {
		keys = "  Tab/arrows choose   Enter confirm   Esc cancel   PgUp/PgDn body   Ctrl+C quit safely"
		if m.width < 110 {
			keys = "  Tab choose  Enter confirm  Esc cancel  PgUp/PgDn body  Ctrl+C quit"
		}
		if m.review.phase == "review" && !m.splitReview() && m.review.scroll > 0 {
			keys = "  PgUp/PgDn scroll   PgUp return to review   Esc cancel   Ctrl+C quit safely"
		}
		if m.review.phase == "destination" {
			keys = "  Type to filter queues   arrows choose   Enter select   Esc back   Ctrl+C quit"
		}
	}
	if m.revision != m.persisted {
		if strings.HasPrefix(status, "Preferences saved") {
			status = "changes made since last save"
		}
		status = "unsaved preferences · " + status
	}
	if !m.compactReading() {
		lines = append(lines, m.paint(strings.Repeat("─", m.width), m.theme.Border), m.paint(fit(keys, m.width), m.theme.Focus))
	}
	lines = append(lines, m.paint(fit("  "+clean(status, false), m.width), m.theme.Muted))
	if m.palette {
		lines = terminalui.Overlay(terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: lines}, m.paletteState.View(m.actions(), m.overlayBounds(), m.theme.TerminalStyle(m.ascii)), m.theme.TerminalStyle(m.ascii)).Lines
	}
	if !m.palette && !m.help {
		bar := m.toolbarFrame()
		if len(bar.Lines) > 0 {
			lines[bar.Bounds.Y] = bar.Lines[0]
			if m.provider != nil {
				context := m.scopeSummary()
				if m.compactReading() {
					if e, ok := m.environment(); ok {
						context = "Env: " + clean(e.Name, false)
					}
				}
				x := 0
				for _, r := range bar.Regions {
					x = max(x, r.Bounds.X+r.Bounds.W+2)
				}
				if x < m.width {
					lines[bar.Bounds.Y] = ansi.Cut(bar.Lines[0], 0, x) + m.paint(fit(context, m.width-x), m.theme.Muted)
				}
			}
		}
	}
	if m.help {
		lines = terminalui.Overlay(terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: lines}, terminalui.RenderHelp(m.helpActions(), m.helpTopics(), m.helpOffset, m.overlayBounds(), m.theme.TerminalStyle(m.ascii)), m.theme.TerminalStyle(m.ascii)).Lines
	}
	if m.ascii {
		replacer := strings.NewReplacer("┌", "+", "┐", "+", "└", "+", "┘", "+", "─", "-", "│", "|", "›", ">", "▾", "v", "▸", ">", "·", ".", "—", "-", "…", "~", "×", "x")
		for i := range lines {
			lines[i] = replacer.Replace(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) settingsView(width, height int, frames ...*terminalui.Frame) []string {
	body := panelBody(m.contentBounds())
	state := "saved"
	if m.revision != m.persisted {
		state = "unsaved changes"
	}
	if m.saving {
		state = "saving…"
	}
	sidebar, inspector, layout := "Collapsed", "Hidden", "Grid"
	if m.prefs.Sidebar {
		sidebar = "Expanded"
	}
	if m.prefs.Inspector {
		inspector = "Visible"
	}
	if m.prefs.Table {
		layout = "Table"
	}
	labels := []string{"Color theme: " + m.prefs.Theme,
		"Navigation sidebar: " + sidebar,
		"Message inspector: " + inspector,
		"Dashboard layout: " + layout, "Save preferences (s)"}
	for _, i := range m.items {
		mark := "[ ]"
		if m.watched(i.resource) {
			mark = "[x]"
		}
		labels = append(labels, mark+" "+clean(i.resource.Profile+" / "+i.resource.Namespace+" / "+i.resource.Name(), false))
	}
	if m.provider != nil {
		labels = append(labels[:5], "Environments / discovery scope…", "Dashboard inclusion / active environment…")
	}
	lines := []string{m.paint("  Preferences / "+state, m.theme.Focus), m.paint("  Watchlists affect Dashboard only; never credentials or MCP access.", m.theme.Muted), ""}
	capacity := max(1, height-7)
	start := m.setting / capacity * capacity
	for index := start; index < min(len(labels), start+capacity); index++ {
		addHit(frames, body, m.settingID(index), terminalui.ControlRegion, terminalui.Rect{X: body.X, Y: body.Y + len(lines), W: body.W, H: 1})
		line := "  " + labels[index]
		if (index == m.setting && m.focus == "content") || m.focus == m.settingID(index) {
			lines = append(lines, m.selectedLine("› "+labels[index], width-2))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	lines = append(lines, "", m.paint("  Enter changes the selected option · s saves · Esc returns", m.theme.Muted))
	return m.panel("Settings", width, height, lines, m.focus == "content")
}

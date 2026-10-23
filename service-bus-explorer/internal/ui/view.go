package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func (m Model) sidebarView(height int) []string {
	var lines []string
	for i, label := range []string{"Dashboard", "Queues", "Topics", "Settings"} {
		prefix := "  "
		active := m.screen == strings.ToLower(label) || (m.screen == "messages" && m.returnScreen == strings.ToLower(label))
		if active {
			prefix = "› "
		}
		line := prefix + label
		if m.focus == "navigation" && i == m.nav {
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
		if m.focus == "navigation" && m.nav == index+4 {
			lines = append(lines, m.selectedLine(line, 22))
		} else {
			color := m.theme.Foreground
			if !row.leaf {
				color = m.theme.Focus
			}
			lines = append(lines, m.paint(line, color))
		}
	}
	return m.panel("WORKSPACE", 24, height, lines, m.focus == "navigation")
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

func (m Model) dashboardView(width, height int) []string {
	items := m.visible()
	title := "Dashboard"
	if m.screen == "queues" {
		title = "Queues"
	}
	if m.screen == "topics" {
		title = "Topic subscriptions"
	}
	if m.screen == "settings" {
		return m.settingsView(width, height)
	}
	if m.loading {
		return m.panel(title, width, height, []string{"", m.paint("  Discovering resources…", m.theme.Muted)}, true)
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
	lines = append(lines, m.paint(fmt.Sprintf("  %s  /  %d %s", mode, len(items), scope), m.theme.Foreground), m.paint("  Enter peek active   ] peek DLQ   g/t layout   Ctrl+P actions", m.theme.Muted), "")
	if len(items) == 0 {
		lines = append(lines, m.paint("  Nothing watched. Open Settings with Ctrl+P to choose entities.", m.theme.Warning))
	}
	if mode == "TABLE" {
		nameWidth := max(16, width-52)
		lines = append(lines, m.paint("  "+fit("ENTITY", nameWidth)+fit("QUEUED", 10)+fit("DEAD-LETTER", 14)+"NAMESPACE", m.theme.Muted), "")
		capacity := max(1, height-9)
		start := m.selected / capacity * capacity
		for index := start; index < min(len(items), start+capacity); index++ {
			i := items[index]
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

func (m Model) messageView(width, height int) []string {
	view := "ACTIVE"
	if m.dlq {
		view = "DEAD-LETTER"
	}
	listWidth := width
	showInspector := m.hasInspector()
	if showInspector {
		listWidth = width * 48 / 100
	}
	lines := []string{m.paint("  [ Active     ] Dead-letter", m.theme.Focus), m.paint("  PEEK ONLY · no locks or settlement", m.theme.Queued), m.paint("  "+clean(m.resource.Profile+" / "+m.resource.Namespace, false), m.theme.Muted)}
	if m.loading {
		lines = append(lines, m.paint("  Loading messages…", m.theme.Muted))
	} else if len(m.page.Messages) == 0 {
		lines = append(lines, m.paint("  No messages in this peek page", m.theme.Muted))
	}
	capacity := max(1, height-9)
	start := m.message / capacity * capacity
	for index := start; index < min(len(m.page.Messages), start+capacity); index++ {
		msg := m.page.Messages[index]
		prefix := "  "
		if index == m.message {
			prefix = "› "
		}
		line := fmt.Sprintf("%s%-6d %s", prefix, msg.Sequence, clean(msg.ID, false))
		if index == m.message {
			lines = append(lines, m.selectedLine(line, listWidth-2))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	lines = append(lines, "", m.paint("  n next page  p previous  r refresh", m.theme.Muted))
	if !showInspector && len(m.page.Messages) > 0 {
		msg := m.page.Messages[m.message]
		lines = append(lines, "", m.paint("  "+clean(msg.Subject, false), m.theme.Focus))
		if m.dlq {
			lines = append(lines, m.paint("  "+clean(msg.DeadLetterReason, false), m.theme.DeadLetter))
		}
	}
	left := m.panel(clean(m.resource.Name(), false)+" / "+view, listWidth, height, lines, m.focus == "content")
	if !showInspector {
		return left
	}
	details := m.inspectorLines()
	offset := min(m.offset, max(0, len(details)-(height-2)))
	right := m.panel("INSPECTOR / Tab focus · j/k scroll", width-listWidth, height, details[offset:], m.focus == "inspector")
	for i := range left {
		left[i] += right[i]
	}
	return left
}

func (m Model) inspectorLines() []string {
	var details []string
	if len(m.page.Messages) > 0 {
		msg := m.page.Messages[m.message]
		details = []string{"", m.paint("  MESSAGE", m.theme.Muted), m.paint("  "+clean(msg.ID, false), m.theme.Foreground), "", m.paint("  "+clean(msg.ContentType, false), m.theme.Focus), m.paint(fmt.Sprintf("  Sequence %d · deliveries %d", msg.Sequence, msg.DeliveryCount), m.theme.Muted), m.paint("  "+msg.EnqueuedAt.Format("2006-01-02 15:04 UTC"), m.theme.Muted)}
		if m.dlq {
			details = append(details, "", m.paint("  "+clean(msg.DeadLetterReason, false), m.theme.DeadLetter), m.paint("  "+clean(msg.DeadLetterDescription, false), m.theme.Muted))
		}
		details = append(details, "", m.paint("  BODY / read-only", m.theme.Muted), "")
		body := msg.Body
		truncated := len(body) > 64*1024 || msg.PreviewTruncated
		if len(body) > 64*1024 {
			body = body[:64*1024]
		}
		for _, line := range strings.Split(clean(string(body), true), "\n") {
			details = append(details, m.paint("  "+line, m.theme.Foreground))
		}
		if truncated {
			details = append(details, m.paint("  [body preview truncated]", m.theme.Warning))
		}
		details = append(details, "", m.paint("  No receive / resend / delete in this slice.", m.theme.Muted))
	}
	return details
}

func (m Model) View() string {
	if m.width < 60 || m.height < 16 {
		return m.paint("Resize terminal to at least 60×16. Ctrl+C quits.", m.theme.Warning)
	}
	header := "  SBX / Service Bus Explorer"
	badge := "FIXTURE · OFFLINE  "
	subtitle := "  Safe browsing workspace · sample data, no Azure connection"
	if m.mode == "emulator" {
		badge = "EMULATOR · LOCAL  "
		subtitle = "  Real local broker · peek only · — means runtime counts unavailable"
	}
	lines := []string{m.paint(fit(header, m.width-lipgloss.Width(badge)), m.theme.Focus) + m.paint(badge, m.theme.Warning), m.paint(fit(subtitle, m.width), m.theme.Muted), m.paint(strings.Repeat("─", m.width), m.theme.Border)}
	height, width := m.height-6, m.width
	showSidebar := m.prefs.Sidebar && width >= 110
	if showSidebar {
		width -= 24
	}
	var content []string
	if m.screen == "messages" {
		content = m.messageView(width, height)
	} else {
		content = m.dashboardView(width, height)
	}
	if showSidebar {
		nav := m.sidebarView(height)
		for i := range content {
			content[i] = nav[i] + content[i]
		}
	}
	lines = append(lines, content...)
	keys := "  j/k move   Enter open   ] DLQ   g/t layout   Tab focus   b sidebar   Ctrl+P actions   q quit"
	if m.screen == "messages" {
		keys = "  j/k move/scroll   [ Active   ] DLQ   i inspector   n/p pages   Tab focus   Ctrl+P actions   Esc back   q quit"
	} else if m.screen == "settings" {
		keys = "  j/k move   Enter change   s save preferences   Ctrl+P actions   Esc back   q quit"
	}
	if m.focus == "navigation" {
		keys = "  j/k move   h/l collapse/expand   Enter open   ] DLQ   Tab focus   Ctrl+P actions   q quit"
	}
	if m.width < 110 {
		keys = "  j/k move  Enter open  ] DLQ  Ctrl+P actions  q quit"
		if m.screen == "messages" {
			keys = "  j/k move  [/] view  n/p pages  Ctrl+P  Esc back  q quit"
		} else if m.screen == "settings" {
			keys = "  j/k move  Enter change  s save  Ctrl+P  Esc back  q quit"
		}
	}
	status := m.status
	if m.revision != m.persisted {
		status += " · unsaved preferences"
	}
	lines = append(lines, m.paint(strings.Repeat("─", m.width), m.theme.Border), m.paint(fit(keys, m.width), m.theme.Focus), m.paint(fit("  "+clean(status, false), m.width), m.theme.Muted))
	if m.palette {
		m.overlayPalette(lines)
	}
	if m.ascii {
		replacer := strings.NewReplacer("┌", "+", "┐", "+", "└", "+", "┘", "+", "─", "-", "│", "|", "›", ">", "▾", "v", "▸", ">", "·", ".", "—", "-", "…", "~", "×", "x")
		for i := range lines {
			lines[i] = replacer.Replace(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) settingsView(width, height int) []string {
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
	lines := []string{m.paint("  Preferences / "+state, m.theme.Focus), m.paint("  Watchlists affect Dashboard only; never credentials or MCP access.", m.theme.Muted), ""}
	capacity := max(1, height-7)
	start := m.setting / capacity * capacity
	for index := start; index < min(len(labels), start+capacity); index++ {
		line := "  " + labels[index]
		if index == m.setting && m.focus == "content" {
			lines = append(lines, m.selectedLine("› "+labels[index], width-2))
		} else {
			lines = append(lines, m.paint(line, m.theme.Foreground))
		}
	}
	lines = append(lines, "", m.paint("  Enter changes the selected option · s saves · Esc returns", m.theme.Muted))
	return m.panel("Settings", width, height, lines, m.focus == "content")
}

func (m Model) overlayPalette(lines []string) {
	width := min(76, m.width-6)
	height := min(18, m.height-6)
	choices := m.actions()
	rows := []string{m.paint("  > "+clean(m.query, false)+"_", m.theme.Foreground), ""}
	capacity := max(1, height-6)
	start := m.action / capacity * capacity
	for index := start; index < min(len(choices), start+capacity); index++ {
		line := "  " + choices[index].title
		if index == m.action {
			rows = append(rows, m.selectedLine("› "+choices[index].title, width-2))
		} else {
			rows = append(rows, m.paint(line, m.theme.Foreground))
		}
	}
	if len(choices) == 0 {
		rows = append(rows, m.paint("  No matching actions", m.theme.Muted))
	}
	for len(rows) < height-4 {
		rows = append(rows, "")
	}
	rows = append(rows, m.paint(fmt.Sprintf("  %d results · arrows choose · Enter run · Esc close", len(choices)), m.theme.Muted), m.paint("  Read-only workspace / no receive, resend or delete", m.theme.Queued))
	panel := m.panel("Ctrl+P / ACTIONS", width, height, rows, true)
	x, y := (m.width-width)/2, max(3, (m.height-height)/3)
	for index, row := range panel {
		under := lines[y+index]
		lines[y+index] = ansi.Cut(under, 0, x) + row + ansi.Cut(under, x+width, m.width)
	}
}

package ui

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
)

func text(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
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

func (m Model) paint(s, c string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(lipgloss.Color(m.theme.Background)).Render(s)
}

func (m Model) block(title string, w, h int, lines []string, focused bool, accent string) []string {
	if w < 2 || h < 2 {
		return make([]string, max(0, h))
	}
	border := m.theme.Border
	if focused && m.modal == "" {
		border = m.theme.Focus
	}
	bar := func(s string) string { return m.paint(s, border) }
	top := "─ " + text(title) + " "
	top = ansi.Truncate(top, w-2, "…") + strings.Repeat("─", max(0, w-2-lipgloss.Width(top)))
	result := []string{bar("┌") + m.paint(top, accent) + bar("┐")}
	for i := 0; i < h-2; i++ {
		s := ""
		if i < len(lines) {
			s = lines[i]
		}
		s = ansi.Truncate(s, w-2, "…")
		padding := m.paint(strings.Repeat(" ", max(0, w-2-lipgloss.Width(s))), m.theme.Foreground)
		if ansi.Strip(s) == s {
			s = m.paint(s, m.theme.Foreground)
		}
		result = append(result, bar("│")+s+padding+bar("│"))
	}
	return append(result, bar("└"+strings.Repeat("─", w-2)+"┘"))
}

func join(a, b []string) []string {
	result := make([]string, max(len(a), len(b)))
	for i := range result {
		if i < len(a) {
			result[i] += a[i]
		}
		if i < len(b) {
			result[i] += b[i]
		}
	}
	return result
}

func (m Model) View() string {
	if m.width == 0 {
		return "Starting workspace…"
	}
	if m.width < 60 || m.height < 12 {
		return m.paint(fit("Resize terminal to at least 60 × 12", m.width), m.theme.Warning)
	}
	h := m.height - 3
	navWidth, inspectorWidth := 0, 0
	if m.navigation && m.width >= 90 {
		navWidth = 22
	}
	if m.inspector && m.width >= 120 {
		inspectorWidth = 28
	}
	centerWidth := m.width - navWidth - inspectorWidth
	leftWidth := centerWidth / 2
	drawerHeight := 0
	if m.jobs {
		drawerHeight = min(7, h-6)
	}
	center := join(m.filePane(0, leftWidth, h-drawerHeight), m.filePane(1, centerWidth-leftWidth, h-drawerHeight))
	if m.jobs {
		center = append(center, m.block("TRANSFERS · demo", centerWidth, drawerHeight, []string{
			"", m.paint("  No real transfers yet", m.theme.Muted), "  Browse, select, and try Ctrl+P.",
			"  FTP + Blob integration is next."}, m.focus == 4, m.theme.Success)...)
	}
	body := center
	if navWidth > 0 {
		body = join(m.navigationPane(navWidth, h), body)
	}
	if inspectorWidth > 0 {
		body = join(body, m.inspectorPane(inspectorWidth, h))
	}
	headerLeft := m.paint(" LAZYFTP", m.theme.Focus) + m.paint(" / proof of concept", m.theme.Muted)
	headerRight := m.paint(m.theme.Name+"  ·  Ctrl+P Actions ", m.theme.Foreground)
	header := headerLeft + m.paint(strings.Repeat(" ", max(0, m.width-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight))), m.theme.Foreground) + headerRight
	footer := " Tab focus · ↑↓/jk move · Enter open · Space select · / filter · [ nav · ] info · t jobs · ? help"
	if m.width < 110 {
		footer = " Tab pane · j/k move · Enter open · Ctrl+P actions · ? help"
	}
	if m.filtering {
		footer = " Filter: " + text(m.panes[m.active()].filter) + "▏   Enter apply · Esc clear"
	}
	status := m.status
	lines := append([]string{fit(header, m.width)}, body...)
	lines = append(lines, m.paint(fit(" "+text(status), m.width), m.theme.Muted), m.paint(fit(footer, m.width), m.theme.Foreground))
	if m.modal != "" {
		lines = m.overlay(lines)
	}
	return strings.Join(lines, "\n")
}

func locationLabel(loc storage.Location) string {
	if loc.Remote != "" {
		return loc.Remote + ":" + loc.Path
	}
	return loc.Path
}

func (m Model) filePane(i, w, h int) []string {
	p := m.panes[i]
	accent := m.theme.Local
	label := "LOCAL"
	if i == 1 {
		accent = m.theme.Remote
		label = "REMOTE"
	}
	lines := []string{m.paint(fit("  Name", max(1, w-13))+"Size", m.theme.Muted), m.paint(strings.Repeat("─", max(0, w-2)), m.theme.Border)}
	entries := p.visible()
	available := max(1, h-7)
	start := max(0, p.cursor-available+1)
	for j := start; j < min(len(entries), start+available); j++ {
		e := entries[j]
		color := m.theme.Foreground
		icon := "·"
		name := text(e.Name)
		size := fmt.Sprintf("%d B", e.Size)
		if e.Size >= 1024 {
			size = fmt.Sprintf("%.1f KB", float64(e.Size)/1024)
		}
		if e.IsDir {
			name += "/"
			color = m.theme.Folder
			icon = "▸"
			size = "—"
		}
		if m.icons {
			icon = "\uf15b"
			switch {
			case e.IsDir:
				icon = "\uf07b"
			case path.Ext(e.Name) == ".md":
				icon = "\ue73e"
				color = m.theme.Markdown
			case path.Ext(e.Name) == ".json":
				icon = "\ue60b"
				color = m.theme.Data
			case path.Ext(e.Name) == ".png":
				icon = "\uf03e"
				color = m.theme.Image
			case path.Ext(e.Name) == ".go":
				icon = "\ue627"
				color = m.theme.Local
			}
		}
		marker := " "
		if p.selected[e.Name] {
			marker = "●"
		}
		if j == p.cursor && m.focus == i+1 {
			marker = "›"
		}
		row := marker + " " + icon + " " + fit(name, max(1, w-17)) + fit(size, 10)
		row = fit(row, w-2)
		if j == p.cursor && m.focus == i+1 && m.modal == "" {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(row)
		} else {
			row = m.paint(row, color)
		}
		lines = append(lines, row)
	}
	if p.loading {
		lines = append(lines, m.paint("  Loading…", m.theme.Muted))
	}
	if p.err != nil {
		lines = append(lines, m.paint("  "+text(p.err.Error()), m.theme.Error), "  Ctrl+R retry")
	}
	if len(entries) == 0 && !p.loading && p.err == nil {
		lines = append(lines, m.paint("  No matching files", m.theme.Muted))
	}
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	count := 0
	for _, v := range p.selected {
		if v {
			count++
		}
	}
	info := fmt.Sprintf(" %d entries · %d selected", len(entries), count)
	if p.filter != "" {
		info += " · /" + text(p.filter)
	}
	lines = append(lines, m.paint(info, m.theme.Muted))
	return m.block(label+" "+locationLabel(p.location), w, h, lines, m.focus == i+1, accent)
}

func (m Model) navigationPane(w, h int) []string {
	m.theme.Background = m.theme.Surface
	lines := []string{m.paint("  REMOTES", m.theme.Muted)}
	labels := []string{"demo-ftp /incoming", "demo-ftp /archive", "demo-blob /assets", "Project", "Downloads"}
	for i, label := range labels {
		if i == 3 {
			lines = append(lines, "", m.paint("  PLACES", m.theme.Muted))
		}
		row := fit("  "+label, w-2)
		if m.focus == 0 && m.navCursor == i && m.modal == "" {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(fit("› "+label, w-2))
		}
		lines = append(lines, row)
	}
	lines = append(lines, "", m.paint("  FTP · unencrypted", m.theme.Warning), m.paint("  Blob · prefixes", m.theme.Muted))
	for len(lines) < h-5 {
		lines = append(lines, "")
	}
	lines = append(lines, m.paint("  FIXTURE MODE", m.theme.Warning), "  Read-only demo", "")
	return m.block("WORKSPACE", w, h, lines, m.focus == 0, m.theme.Local)
}

func (m Model) inspectorPane(w, h int) []string {
	m.theme.Background = m.theme.Surface
	p := m.panes[m.active()]
	entries := p.visible()
	source := p.location.Remote
	if source == "" {
		source = "LOCAL"
	}
	lines := []string{m.paint("  "+source, m.theme.Muted), m.paint("  "+p.location.Path, m.theme.Muted), ""}
	if len(entries) > 0 {
		e := entries[min(p.cursor, len(entries)-1)]
		kind := "File"
		if e.IsDir {
			kind = "Directory"
		}
		lines = append(lines, "  "+text(e.Name), "", fmt.Sprintf("  Size     %d B", e.Size), "  Type     "+kind, "  Modified", "  "+e.Modified.Format("02 Jan 2006 15:04"), "", m.paint("  CONTENT PREVIEW", m.theme.Muted), "  Not loaded", "  Coming after storage", "  integration.")
	}
	return m.block("INSPECTOR", w, h, lines, m.focus == 3, m.theme.Remote)
}

func (m Model) overlay(background []string) []string {
	w := min(82, m.width-6)
	h := min(20, m.height-4)
	lines := []string{}
	title := "ACTIONS · Ctrl+P"
	if m.modal == "help" {
		title = "KEYBOARD SHORTCUTS"
		lines = []string{"", "  Ctrl+P        Search every action", "  Tab/Shift+Tab Focus next/previous panel", "  ↑↓ or j/k     Move cursor", "  Enter         Open directory / action", "  Backspace/h   Parent directory", "  Space         Select file", "  /             Filter current pane", "  [  /  ]       Navigation / inspector", "  t             Transfer drawer", "  Ctrl+R        Refresh", "  q             Quit", "", "  Esc returns to your workspace"}
	} else {
		lines = append(lines, "  > "+text(m.query)+"▏", m.paint(strings.Repeat("─", w-2), m.theme.Border))
		matches := m.matches()
		n := max(1, min(9, h-8))
		start := max(0, m.choice-n+1)
		for i := start; i < min(len(matches), start+n); i++ {
			a := matches[i]
			suffix := a.shortcut
			if a.disabled != "" {
				suffix = "unavailable"
			}
			row := fit("  "+a.label, w-19) + fit(suffix, 15)
			row = fit(row, w-2)
			if i == m.choice {
				row = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection)).Render(row)
			} else if a.disabled != "" {
				row = m.paint(row, m.theme.Muted)
			}
			lines = append(lines, row)
		}
		if len(matches) == 0 {
			lines = append(lines, "  No matching actions")
		}
		for len(lines) < h-6 {
			lines = append(lines, "")
		}
		detail := "Actions follow the focused pane."
		if len(matches) > 0 {
			a := matches[min(m.choice, len(matches)-1)]
			if a.disabled != "" {
				detail = a.disabled
			}
		}
		lines = append(lines, m.paint("  "+detail, m.theme.Muted), "", "  ↑↓ choose · Enter run · Esc close")
	}
	box := m.block(title, w, h, lines, true, m.theme.Focus)
	x, y := (m.width-w)/2, (m.height-h)/2
	for i, row := range box {
		idx := y + i
		background[idx] = ansi.Cut(background[idx], 0, x) + row + ansi.Cut(background[idx], x+w, m.width)
	}
	return background
}

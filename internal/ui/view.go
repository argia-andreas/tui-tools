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
	if focused {
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

const toolbar = " [Nav] [Info] [Jobs] [?] [Actions] "
const paneControls = " ↑ Up  / Filter  ↻ Refresh"
const workspaceFooter = " Space Select   c Copy   ? help"
const paletteFooter = "  ↑↓ choose · Enter run · F1 shortcuts · Esc close"
const helpFooter = "  ↑↓ / wheel scroll · Esc close"

func (m Model) toolbar() string {
	if m.transferer != nil {
		return " Ctrl+P [Actions] "
	}
	return toolbar
}

func (m Model) navY(i int) int {
	y := 3 + i
	for j := 0; j <= i; j++ {
		if m.locations[j].Remote == "" {
			return y + 2
		}
	}
	return y
}

// Rendering and mouse input share these dimensions, including responsive panels.
type workspaceLayout struct{ nav, inspector, center, left, height, drawer int }

func (m Model) layout() workspaceLayout {
	l := workspaceLayout{height: m.height - 3}
	if m.transferer != nil {
		l.height++ // Real workspaces need only one contextual footer row.
	}
	if m.navigation && m.width >= 90 {
		l.nav = 22
	}
	if m.inspector && m.width >= 120 {
		l.inspector = 28
	}
	l.center = m.width - l.nav - l.inspector
	l.left = l.center / 2
	if m.jobs {
		l.drawer = min(5, l.height-6)
		if m.job != nil && m.job.err != nil {
			l.drawer = min(6, l.height-6)
		}
		if len(m.queue) > 0 {
			l.drawer = min(9, l.height-6)
		}
	}
	return l
}

// Use the same content-sized modal bounds for rendering and mouse hit testing.
func (m Model) modalSize() (int, int) {
	w, h := min(82, m.width-6), min(20, m.height-4)
	switch m.modal {
	case "folder":
		h = min(h, 8)
	case "copy", "batch":
		h = min(h, 12)
	case "rename":
		h = min(h, 8)
	case "palette":
		h = min(h, max(9, min(9, len(m.matches()))+8))
	}
	return w, h
}

func (m Model) View() string {
	if m.width == 0 {
		return "Starting workspace…"
	}
	if m.width < 60 || m.height < 12 {
		return m.paint(fit("Resize terminal to at least 60 × 12", m.width), m.theme.Warning)
	}
	l := m.layout()
	h, navWidth, inspectorWidth := l.height, l.nav, l.inspector
	centerWidth, leftWidth, drawerHeight := l.center, l.left, l.drawer
	center := join(m.filePane(0, leftWidth, h-drawerHeight), m.filePane(1, centerWidth-leftWidth, h-drawerHeight))
	if m.jobs {
		jobLines := []string{
			m.paint("  No real transfers in demo mode", m.theme.Muted), "  Browse, select, and try Ctrl+P.",
			"  Use --live to connect your filesystems."}
		if m.transferer != nil {
			jobLines = []string{"  No transfers yet — select a file and press c."}
		}
		if m.job != nil {
			color := m.theme.Focus
			switch m.job.state {
			case "Completed":
				color = m.theme.Success
			case "Failed":
				color = m.theme.Error
			case "Skipped":
				color = m.theme.Warning
			}
			jobLines = []string{m.paint("  ● "+m.job.state, color), "  From: " + text(m.job.source.String()), "  To:   " + text(m.job.destination.String())}
			if m.job.err != nil {
				jobLines = append(jobLines, m.paint("  "+text(m.job.err.Error()), m.theme.Error))
			}
		}
		if len(m.queue) > 0 {
			jobLines = []string{m.paint("  "+m.queueSummary(), m.theme.Focus)}
			start := max(0, min(m.jobCursor, len(m.queue)-1)-max(0, drawerHeight-5))
			for i := start; i < min(len(m.queue), start+drawerHeight-4); i++ {
				j := m.queue[i]
				row := "  " + j.state + " · " + text(j.destination.String())
				if j.err != nil {
					row += " — " + text(j.err.Error())
				}
				color := m.theme.Foreground
				if j.state == "Failed" {
					color = m.theme.Error
				}
				if j.state == "Completed" {
					color = m.theme.Success
				}
				if j.state == "Canceled" || j.state == "Skipped" {
					color = m.theme.Warning
				}
				if m.focus == 4 && i == m.jobCursor {
					row = "› " + row[2:]
				}
				jobLines = append(jobLines, m.paint(row, color))
			}
			jobLines = append(jobLines, m.paint("  ↑↓ / wheel review · Ctrl+X stop · r retry · partial files may remain", m.theme.Muted))
		}
		center = append(center, m.block("TRANSFERS", centerWidth, drawerHeight, jobLines, m.focus == 4, m.theme.Success)...)
	}
	body := center
	if navWidth > 0 {
		body = join(m.navigationPane(navWidth, h), body)
	}
	if inspectorWidth > 0 {
		body = join(body, m.inspectorPane(inspectorWidth, h))
	}
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Focus)).Background(lipgloss.Color(m.theme.Background)).Render(" LAZYFTP")
	headerRight := m.toolbar()
	if m.width >= 110 && m.transferer == nil {
		headerRight = m.theme.Name + " ·" + headerRight
	}
	headerRight = m.paint(headerRight, m.theme.Foreground)
	header := headerLeft + m.paint(strings.Repeat(" ", max(0, m.width-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight))), m.theme.Foreground) + headerRight
	footer := " Click row to open · Click marker to select · Wheel scroll · Ctrl+P actions · F1 / ? shortcuts"
	if m.width < 110 {
		footer = " Tab pane · j/k move · Enter open · Ctrl+P actions · ? help"
	}
	if m.filtering {
		footer = " Filter: " + text(m.panes[m.active()].filter) + "▏   Enter apply · Esc clear"
	}
	status := m.status
	lines := append([]string{fit(header, m.width)}, body...)
	if m.transferer != nil {
		footer = workspaceFooter
		if m.filtering {
			footer = " Filter: " + text(m.panes[m.active()].filter) + "▏   Enter apply · Esc clear"
		} else if status != "" {
			footer = " " + text(status)
		}
		count := fmt.Sprintf(" %d transfers ", len(m.queue))
		if len(m.queue) == 0 && m.job != nil {
			count = " 1 transfer "
		}
		footer = fit(footer, m.width-lipgloss.Width(count)) + count
		lines = append(lines, m.paint(fit(footer, m.width), m.theme.Muted))
	} else {
		lines = append(lines, m.paint(fit(" "+text(status), m.width), m.theme.Muted), m.paint(fit(footer, m.width), m.theme.Foreground))
	}
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
		if m.disconnected {
			return m.destinationPane(w, h)
		}
		if p.location.Remote == "" {
			label, accent = "LOCAL", m.theme.Local
		}
	}
	columns := fit("    Name", max(1, w-13)) + fit("Size", 10)
	lines := []string{m.paint(paneControls, m.theme.Muted), m.paint(fit(columns, w-2), m.theme.Muted)}
	entries := p.visible()
	available := max(1, h-7)
	start := max(0, p.cursor-available+1)
	for j := start; j < min(len(entries), start+available); j++ {
		e := entries[j]
		color := m.theme.Foreground
		icon := "▤"
		name := text(e.Name)
		ext := strings.ToLower(path.Ext(e.Name))
		size := fmt.Sprintf("%d B", e.Size)
		if e.Size >= 1024 {
			size = fmt.Sprintf("%.1f KB", float64(e.Size)/1024)
		}
		if e.IsDir {
			name += "/"
			color = m.theme.Folder
			icon = "▸"
			size = "—"
		} else {
			switch ext {
			case ".md", ".markdown":
				icon, color = "≡", m.theme.Markdown
			case ".json", ".csv", ".yaml", ".yml", ".toml", ".xml":
				icon, color = "⌘", m.theme.Data
			case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
				icon, color = "◈", m.theme.Image
			case ".go", ".rs", ".js", ".ts", ".py", ".sh":
				icon, color = "λ", m.theme.Local
			case ".zip", ".gz", ".tar":
				icon, color = "▣", m.theme.Warning
			}
		}
		if m.icons {
			icon = "\uf15b"
			if e.IsDir {
				icon = "\uf07b"
			} else {
				switch ext {
				case ".md", ".markdown":
					icon, color = "\ue73e", m.theme.Markdown
				case ".txt", ".rst", ".pdf":
					icon, color = "\uf15c", m.theme.Markdown
				case ".json", ".csv", ".yaml", ".yml", ".toml", ".xml":
					icon, color = "\ue60b", m.theme.Data
				case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico", ".tif", ".tiff":
					icon, color = "\uf03e", m.theme.Image
				case ".go":
					icon, color = "\ue627", m.theme.Local
				case ".rs":
					icon, color = "\ue7a8", m.theme.Local
				case ".js", ".jsx", ".mjs", ".cjs":
					icon, color = "\ue74e", m.theme.Local
				case ".ts", ".tsx":
					icon, color = "\ue628", m.theme.Local
				case ".py", ".pyw":
					icon, color = "\ue73c", m.theme.Local
				case ".sh", ".bash", ".zsh", ".fish":
					icon, color = "\uf489", m.theme.Local
				case ".c", ".h", ".cpp", ".hpp", ".java", ".rb", ".php", ".html", ".htm", ".css", ".scss", ".sql":
					icon, color = "\uf121", m.theme.Local
				case ".zip", ".gz", ".tar", ".tgz", ".bz2", ".xz", ".7z", ".rar", ".zst":
					icon, color = "\uf410", m.theme.Warning
				}
			}
		}
		if e.Name == ".." {
			name, icon, color = "..", "↑", m.theme.Muted
		}
		marker := "○"
		if e.Name == ".." {
			marker = " "
		}
		if p.selected[e.Name] {
			marker = "●"
		}
		if j == p.cursor && m.focus == i+1 && !p.selected[e.Name] {
			marker = "›"
		}
		nameColor := m.theme.Foreground
		if e.IsDir {
			nameColor = m.theme.Folder
		}
		row := m.paint(marker+" ", m.theme.Focus) + m.paint(fit(icon, 2), color) + m.paint(fit(name, max(1, w-17)), nameColor) + m.paint(fit(size, 10), m.theme.Muted)
		row += m.paint(strings.Repeat(" ", max(0, w-2-lipgloss.Width(row))), m.theme.Foreground)
		if j == p.cursor && m.focus == i+1 && m.modal == "" {
			foreground, background := m.theme.SelectionText, m.theme.Selection
			if m.disconnected {
				foreground, background = m.theme.Foreground, m.theme.Surface
			}
			row = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(foreground)).Background(lipgloss.Color(background)).Render(ansi.Strip(row))
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
	entryCount := len(entries)
	if p.hasParent() && p.filter == "" {
		entryCount--
	}
	info := fmt.Sprintf(" %d entries · %d selected", entryCount, count)
	if p.filter != "" {
		info += " · /" + text(p.filter)
	}
	lines = append(lines, m.paint(info, m.theme.Muted))
	return m.block(label+" "+locationLabel(p.location), w, h, lines, m.focus == i+1, accent)
}

func (m Model) navigationPane(w, h int) []string {
	m.theme.Background = m.theme.Surface
	lines := []string{m.paint("  REMOTES", m.theme.Muted)}
	for i, loc := range m.locations {
		label := loc.Remote + " " + loc.Path
		if loc.Remote == "" {
			label = path.Base(loc.Path)
			if label == "project" {
				label = "Project"
			}
			if label == "downloads" {
				label = "Downloads"
			}
		}
		if loc.Remote == "" && (i == 0 || m.locations[i-1].Remote != "") {
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
	if m.transferer == nil {
		lines = append(lines, m.paint("  FIXTURE MODE", m.theme.Warning), "  Read-only demo", "")
	} else {
		lines = append(lines, m.paint("  LIVE MODE", m.theme.Warning), "  Confirm each copy", "")
	}
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
		lines = append(lines, "  "+text(e.Name), "", fmt.Sprintf("  Size     %d B", e.Size), "  Type     "+kind, "  Modified", "  "+e.Modified.Format("02 Jan 2006 15:04"), "", m.paint("  CONTENT PREVIEW", m.theme.Muted), "  p / Actions → Preview", "  Explicit bounded read", "  (up to 64 KiB)")
	}
	return m.block("INSPECTOR", w, h, lines, m.focus == 3, m.theme.Remote)
}

func (m Model) overlay(background []string) []string {
	w, h := m.modalSize()
	// Suppress competing colors without altering workspace state or hit regions.
	for i, row := range background {
		background[i] = m.paint(ansi.Strip(row), m.theme.Border)
	}
	m.theme.Background = m.theme.Surface
	accent := m.theme.Focus
	if m.modal == "copy" && m.pending != nil && (m.pending.conflict || m.pending.err != nil) {
		accent = m.theme.Warning
	}
	m.theme.Focus = accent
	lines := []string{}
	title := "ACTIONS · Ctrl+P"
	if m.modal == "folder" {
		title = "OPEN LOCAL DESTINATION"
		lines = []string{"", "  Folder", "  > " + text(m.folderPath) + "▏", m.paint("  "+m.status, m.theme.Error), "  Enter Open · Ctrl+U Clear · Esc Cancel"}
	} else if m.modal == "preview" {
		title, lines = m.previewLines(w, h)
	} else if m.modal == "batch" {
		title, lines = m.batchLines(h)
	} else if m.modal == "copy" || m.modal == "rename" {
		title, lines = m.copyLines(w, h)
	} else if m.modal == "help" {
		title = "KEYBOARD SHORTCUTS · F1"
		guide := shortcutLines()
		start := min(m.choice, max(0, len(guide)-(h-3)))
		lines = append(lines, guide[start:min(len(guide), start+h-3)]...)
		lines = append(lines, m.paint(helpFooter, m.theme.Muted))
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
		for len(lines) < h-5 {
			lines = append(lines, "")
		}
		detail := "Actions follow the focused pane."
		if len(matches) > 0 {
			a := matches[min(m.choice, len(matches)-1)]
			if a.disabled != "" {
				detail = a.disabled
			} else if a.id == "copy" {
				entry, _ := m.copyEntry()
				destination := m.panes[1-m.active()].location
				destination.Path = path.Join(destination.Path, entry.Name)
				detail = entry.Name + " → " + destination.String()
			}
		}
		lines = append(lines, m.paint("  "+detail, m.theme.Muted), "", paletteFooter)
	}
	box := m.block(title, w, h, lines, true, accent)
	box[0] = ansi.Cut(box[0], 0, w-6) + m.paint(" [×] ", accent) + ansi.Cut(box[0], w-1, w)
	x, y := (m.width-w)/2, (m.height-h)/2
	for i, row := range box {
		idx := y + i
		background[idx] = ansi.Cut(background[idx], 0, x) + row + ansi.Cut(background[idx], x+w, m.width)
	}
	return background
}

func shortcutLines() []string {
	return []string{
		"  DISCOVER", "  Ctrl+P        Search every action", "  F1 / ?        Keyboard shortcuts", "  Esc           Close modal / clear filter", "",
		"  BROWSE & SELECT", "  Tab/Shift+Tab Focus next/previous panel", "  ↑↓ or j/k     Move cursor", "  Enter         Open directory / run action", "  Backspace/h   Parent directory", "  Space         Toggle file selection", "  c             Copy files / folders (live)", "  Ctrl+X        Cancel unfinished transfers", "  r             Retry failed / canceled files", "  p             Bounded text preview", "  /             Filter current pane", "  Ctrl+R        Refresh listing", "",
		"  WORKSPACE", "  [ / ]         Toggle navigation / inspector", "  t             Toggle transfer drawer", "  q / Ctrl+C    Quit workspace", "",
		"  COMMAND PALETTE", "  Type          Fuzzy-search actions", "  ↑↓ / Ctrl+N/K Choose action", "  Enter         Run selected action", "  F1            Show this guide", "",
		"  MOUSE", "  Click row     Open directory / inspect file", "  Click marker  Toggle selection without opening", "  Wheel         Scroll list / guide / palette", "  Top buttons   Toggle panels / help / actions",
	}
}

package ui

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
	"terminalui"
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

// Confirmation paths favor the identifying tail, not a long directory prefix.
// A basename wider than the field keeps both its beginning and extension/tail.
// This is display-only: captured storage locations remain untouched.
func displayPath(loc storage.Location, width int) string {
	s := text(loc.String())
	if ansi.StringWidth(s) <= width {
		return s
	}
	base := text(path.Base(loc.Path))
	prefix := "…"
	if ansi.StringWidth(base) >= width {
		head := max(0, (width-1)/2)
		prefix, s = ansi.Truncate(base, head, "")+"…", base
	}
	remaining := width - ansi.StringWidth(prefix)
	tail := ansi.TruncateLeft(s, ansi.StringWidth(s)-remaining, "")
	// TruncateLeft retains a wide grapheme crossing its cut boundary.
	if ansi.StringWidth(tail) > remaining {
		tail = ansi.TruncateLeft(tail, 2, "")
	}
	return prefix + tail
}

func (m Model) paint(s, c string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(lipgloss.Color(m.theme.Background)).Render(s)
}

func (m Model) block(title string, w, h int, lines []string, focused bool, accent string) []string {
	style := m.theme.TerminalStyle(false)
	if focused {
		style.Border = m.theme.Focus
	}
	body := make([]string, min(len(lines), max(0, h-2)))
	for i := range body {
		body[i] = lines[i]
		if ansi.Strip(body[i]) == body[i] {
			body[i] = m.paint(body[i], m.theme.Foreground)
		}
	}
	return terminalui.RenderPanel(terminalui.PanelSpec{
		Title: "─ " + text(title) + " ", Lines: body,
		TitleColor: accent, TitleRule: true, Ellipsis: "…",
	}, w, h, style)
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

const paneControls = " ↑ Up  / Filter  ↻ Refresh  Open/Inspect"
const workspaceFooter = " Space Mark   c Copy highlight   D Delete   ? help"

func (m Model) workspaceFits() bool { return m.width >= 60 && m.height >= 12 }

func (m Model) barActions() []terminalui.Action {
	palette := terminalui.Action{ID: "palette", Label: "Actions", Keys: []string{"ctrl+p"}}
	if m.filtering {
		palette.Unavailable = "Filter owns input"
	}
	if (m.modal != "" && m.modal != "palette") || m.rename != nil {
		palette.Unavailable = "Review owns input"
	}
	actions := []terminalui.Action{palette, {ID: "help", Label: "Help", Keys: []string{"f1"}}}
	if palette.Unavailable != "" {
		// Keep Help visible when the blocked Actions explanation cannot fit.
		actions[0], actions[1] = actions[1], actions[0]
	}
	if m.transferer == nil && (m.modal == "" || m.modal == "palette") && m.rename == nil {
		actions = append(actions,
			terminalui.Action{ID: "navigation", Label: "Nav"}, terminalui.Action{ID: "inspector", Label: "Info"},
			terminalui.Action{ID: "jobs", Label: "Jobs"})
	}
	return actions
}

func (m Model) actionBarFrame() terminalui.Frame {
	if !m.workspaceFits() {
		return terminalui.Frame{}
	}
	// Preserve the app's right-aligned top toolbar geometry.
	w := min(78, m.width-10)
	focus := m.controlFocus
	if focus == "" {
		focus = "workspace"
	} // Reserve the marker cell before focus moves.
	return terminalui.RenderActionBar(m.barActions(), terminalui.Rect{X: m.width - w, W: w, H: 1}, focus, m.theme.TerminalStyle(false))
}

func (m Model) paletteFrame() terminalui.Frame {
	if !m.workspaceFits() {
		return terminalui.Frame{}
	}
	w, h := min(82, m.width-6), min(18, m.height-4)
	return m.palette.View(m.actions(), terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}, m.theme.TerminalStyle(false))
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
	case "folder", "delete":
		h = min(h, 8)
	case "copy", "batch", "file-rename":
		h = min(h, 12)
	case "rename":
		h = min(h, 8)
	}
	return w, h
}

func (m Model) View() string {
	if m.width == 0 {
		return "Starting workspace…"
	}
	if !m.workspaceFits() {
		return m.paint(fit("Resize terminal to at least 60 × 12", m.width), m.theme.Warning)
	}
	if m.help {
		return m.helpView()
	}
	l := m.layout()
	h, navWidth, inspectorWidth := l.height, l.nav, l.inspector
	centerWidth, leftWidth, drawerHeight := l.center, l.left, l.drawer
	center := join(m.filePane(0, leftWidth, h-drawerHeight).Lines, m.filePane(1, centerWidth-leftWidth, h-drawerHeight).Lines)
	if m.jobs {
		center = append(center, m.transferPane(centerWidth, drawerHeight).Lines...)
	}
	body := center
	if navWidth > 0 {
		body = join(m.navigationPane(navWidth, h), body)
	}
	if inspectorWidth > 0 {
		body = join(body, m.inspectorPane(inspectorWidth, h))
	}
	headerLeft := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.Focus)).Background(lipgloss.Color(m.theme.Background)).Render(" TRANSFER")
	bar := m.actionBarFrame()
	headerRight := strings.Join(bar.Lines, "")
	if m.width >= 110 && m.transferer == nil {
		headerRight = m.theme.Name + " ·" + headerRight
	}
	headerRight = m.paint(headerRight, m.theme.Foreground)
	header := headerLeft + m.paint(strings.Repeat(" ", max(0, m.width-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight))), m.theme.Foreground) + headerRight
	footer := " Click row to highlight · Marker/Space marks · Enter / Open/Inspect opens · Wheel scroll · Ctrl+P actions"
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
		if !m.filtering && m.markedCount() > 0 && (status == "" || strings.HasPrefix(status, "Queue finished") || strings.HasPrefix(status, "Completed:")) {
			footer = fmt.Sprintf(" c Copy %d marked · Ctrl+G clear marks", m.markedCount())
		}
		if m.rename != nil && !m.rename.confirming {
			footer = " Rename " + text(path.Base(m.rename.source.Path)) + " · Enter review · Esc cancel"
			if status != "" {
				footer = " " + text(status)
			}
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
	lines = m.highlightNavigation(lines)
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

func (m Model) filePane(i, w, h int) terminalui.Frame {
	f := terminalui.Frame{Bounds: terminalui.Rect{W: w, H: h}}
	control := func(action string, x, y, width int) {
		if x >= 1 && x+width <= w-1 && y >= 0 && y < h-1 {
			f.Regions = append(f.Regions, terminalui.Region{ID: m.paneControl(i, action), Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: x, Y: y, W: width, H: 1}})
		}
	}
	p := m.panes[i]
	if p.detached {
		f.Lines = m.destinationPane(i, w, h)
		for choice, b := range sourceButtons {
			control(b.action, 1+max(0, (w-2-len(b.label))/2), 1+destinationRows(h)+2*choice, len(b.label))
		}
		return f
	}
	if m.transferer != nil {
		control("detach", w-len(detachButton)-2, 0, len(detachButton))
	}
	for _, c := range browseControls {
		start := ansi.StringWidth(strings.Split(paneControls, c.label)[0])
		control(c.action, 1+start, 1, ansi.StringWidth(c.label))
	}
	accent := m.theme.Local
	label := "LOCAL"
	if p.location.Remote != "" {
		accent = m.theme.Remote
		label = "REMOTE"
	}
	columns := fit("    Name", max(1, w-13)) + fit("Size", 10)
	controls := paneControlText(w)
	if m.rename != nil && m.rename.pane == i && !m.rename.confirming {
		controls = " [ Review ] [ Cancel ]"
	}
	lines := []string{m.paint(controls, m.theme.Muted), m.paint(fit(columns, w-2), m.theme.Muted)}
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
		if m.rename != nil && m.rename.pane == i && e.Name == path.Base(m.rename.source.Path) && !m.rename.confirming {
			row = m.paint("› "+fit(icon, 2)+m.renameInput(max(1, w-17)), m.theme.Focus) + m.paint(fit(size, 10), m.theme.Muted)
		}
		row += m.paint(strings.Repeat(" ", max(0, w-2-lipgloss.Width(row))), m.theme.Foreground)
		if j == p.cursor && m.focus == i+1 && m.modal == "" {
			foreground, background := m.theme.SelectionText, m.theme.Selection
			row = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(foreground)).Background(lipgloss.Color(background)).Render(ansi.Strip(row))
		}
		if y := 1 + len(lines); y < h-1 {
			id := m.rowID(i, e)
			f.Regions = append(f.Regions, terminalui.Region{ID: id, Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: 1, Y: y, W: w - 2, H: 1}})
			if e.Name != ".." {
				f.Regions = append(f.Regions, terminalui.Region{ID: "mark:" + id, Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: 1, Y: y, W: 2, H: 1}})
			}
		}
		lines = append(lines, row)
	}
	if p.loading {
		lines = append(lines, m.paint("  Loading…", m.theme.Muted))
	}
	if p.err != nil {
		control("refresh", 3, len(lines)+2, len("Ctrl+R retry"))
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
	info := fmt.Sprintf(" %d entries · %d selected (marked)", entryCount, count)
	if p.filter != "" {
		info += " · /" + text(p.filter)
	}
	lines = append(lines, m.paint(info, m.theme.Muted))
	title := paneName(i) + " · " + label + " " + locationLabel(p.location)
	if m.transferer != nil {
		title = ansi.Truncate(text(title), max(0, w-len(detachButton)-6), "…")
	}
	result := m.block(title, w, h, lines, m.focus == i+1, accent)
	if m.transferer != nil {
		border := m.theme.Border
		if m.focus == i+1 {
			border = m.theme.Focus
		}
		result[0] = ansi.Cut(result[0], 0, w-len(detachButton)-2) + m.paint(detachButton, m.theme.Focus) + m.paint("─┐", border)
	}
	f.Lines = result
	return f
}

func (m Model) transferPane(w, h int) terminalui.Frame {
	f := terminalui.Frame{Bounds: terminalui.Rect{W: w, H: h}}
	lines := []string{m.paint("  No real transfers in demo mode", m.theme.Muted), "  Browse, select, and try Ctrl+P.", "  Use --live to connect your filesystems."}
	if m.transferer != nil {
		lines = []string{"  No transfers yet — select a file and press c."}
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
		lines = []string{m.paint("  ● "+m.job.state, color), "  From: " + text(m.job.source.String()), "  To:   " + text(m.job.destination.String())}
		if m.job.err != nil {
			lines = append(lines, m.paint("  "+text(m.job.err.Error()), m.theme.Error))
		}
	}
	if len(m.queue) > 0 {
		lines = []string{m.paint("  "+m.queueSummary(), m.theme.Focus)}
		start := max(0, min(m.jobCursor, len(m.queue)-1)-max(0, h-5))
		for i := start; i < min(len(m.queue), start+h-4); i++ {
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
			f.Regions = append(f.Regions, terminalui.Region{ID: "job:" + locationID(j.source) + ":" + locationID(j.destination), Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: 1, Y: 1 + len(lines), W: w - 2, H: 1}})
			lines = append(lines, m.paint(row, color))
		}
		hint := "  ↑↓ / wheel review · Ctrl+X stop · r retry · partial files may remain"
		if y := 1 + len(lines); y < h-1 {
			for _, c := range []struct{ label, id string }{{"Ctrl+X stop", "cancel"}, {"r retry", "retry"}} {
				x := ansi.StringWidth(strings.Split(hint, c.label)[0])
				if x+len(c.label) <= w-2 {
					f.Regions = append(f.Regions, terminalui.Region{ID: c.id, Kind: terminalui.ControlRegion, Bounds: terminalui.Rect{X: 1 + x, Y: y, W: len(c.label), H: 1}})
				}
			}
		}
		lines = append(lines, m.paint(hint, m.theme.Muted))
	}
	f.Lines = m.block("TRANSFERS", w, h, lines, m.focus == 4, m.theme.Success)
	return f
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
	if p.detached {
		source = paneName(m.active()) + " · No source"
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
	foreground := m.surfaceFrame()
	if m.modal == "palette" {
		foreground = m.paletteFrame()
	}
	return terminalui.Overlay(terminalui.Frame{
		Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: background,
	}, foreground, m.theme.TerminalStyle(false)).Lines
}

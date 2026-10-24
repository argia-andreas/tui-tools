package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"logviewer/internal/logs"
)

func (m Model) style(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(lipgloss.Color(m.options.Theme.Background))
}

func fit(s string, width int) string {
	s = ansi.Truncate(s, max(0, width), "…")
	return s + strings.Repeat(" ", max(0, width-ansi.StringWidth(s)))
}

func (m Model) panel(title string, lines []string, width, height, owner int) string {
	t := m.options.Theme
	color := t.Border
	if m.focus == owner {
		color = t.Focus
		title = "● " + title
	}
	inner := max(1, width-2)
	border := m.style(color)
	label := ansi.Truncate(" "+title+" ", inner, "…")
	rows := []string{border.Render("┌" + label + strings.Repeat("─", inner-ansi.StringWidth(label)) + "┐")}
	for i := 0; i < height-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		rows = append(rows, border.Render("│")+m.style(t.Foreground).Render(fit(line, inner))+border.Render("│"))
	}
	rows = append(rows, border.Render("└"+strings.Repeat("─", inner)+"┘"))
	return strings.Join(rows, "\n")
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	t := m.options.Theme
	base := m.style(t.Foreground)
	if m.width < 60 || m.height < 16 {
		return base.Render(fit("Resize to at least 60 × 16 · q quit", m.width))
	}
	mode := " LOGVIEW  /  FIXTURE · OFFLINE"
	if m.options.LocalLab {
		mode = " LOGVIEW  /  ELASTICSEARCH · LOCAL LAB · TLS"
	}
	header := m.style(t.Focus).Bold(true).Render(fit(mode, m.width))
	if m.editor != nil && m.editor.kind == "palette" {
		return m.paletteView()
	}
	if m.editor != nil {
		title, lines := m.editorLines()
		for i := range lines {
			lines[i] = strings.ReplaceAll(logs.Clean(lines[i]), "\n", " ↵ ")
		}
		return header + "\n" + m.panel(title, lines, m.width, m.height-2, m.focus) + "\n" + m.style(t.Focus).Render(fit(" Text owns keys · Esc cancels without changing results/focus", m.width))
	}
	q := m.options.Query
	apps := strings.Join(q.Scope.Services, " OR ")
	if apps == "" {
		apps = "ALL apps"
	}
	levels := strings.Join(q.Levels, ",")
	if levels == "" {
		levels = "ALL"
	}
	window := fmt.Sprintf(" App exact: %s / levels: %s / [%s, %s) UTC", apps, levels, q.Window.Start.UTC().Format("2006-01-02T15:04:05"), q.Window.End.UTC().Format("2006-01-02T15:04:05"))
	predicates := []string{}
	if q.Message.Text != "" {
		label := "phrase"
		if !m.options.LocalLab {
			label = "phrase (fixture approximation)"
		}
		predicates = append(predicates, label+": "+strings.ReplaceAll(logs.Clean(q.Message.Text), "\n", " ↵ "))
	}
	if len(q.TraceIDs) > 0 {
		predicates = append(predicates, "trace="+strings.Join(q.TraceIDs, " OR "))
	}
	for _, f := range q.Fields {
		predicates = append(predicates, f.Path+"="+string(f.Value))
	}
	summary := " Filters: " + strings.Join(predicates, " AND ")
	if len(predicates) == 0 {
		summary = " Message/trace/fields: ALL"
	}
	header += "\n" + m.style(t.Muted).Render(fit(window, m.width)) + "\n" + m.style(t.Muted).Render(fit(summary, m.width))
	height := m.height - 7
	var body string
	if m.detail {
		body = m.inspector(m.width, height)
	} else {
		left, right := 0, 0
		if m.width >= 140 {
			left = 24
		}
		if m.width >= 120 {
			right = min(50, m.width/3)
		}
		var panels []string
		if left > 0 {
			source := "  fixture / local"
			if m.options.LocalLab {
				source = "  local-elastic"
			}
			if m.focus == 0 {
				source = m.style(t.SelectionText).Background(lipgloss.Color(t.Selection)).Render(fit("›"+source[1:], left-4))
			}
			lines := []string{"", " SOURCES", "", source, "", " Fictional records", " No network access", "", " PROVIDERS · PLANNED", "", "  Elasticsearch", "  Azure Monitor", "  Kubernetes", "  Loki", "", " MCP · NOT RUNNING", " No grants or tools", "", " Fixed sample window", " No live collection"}
			if m.options.LocalLab {
				lines = []string{"", " SOURCES", "", source, "", " Seeded fictional logs", " HTTPS / read-only key", "", " PROVIDERS · PLANNED", "", "  Azure Monitor", "  Kubernetes", "  Loki", "", " MCP · NOT RUNNING", " No grants or tools", "", " Fixed lab window", " No live collection"}
			}
			panels = append(panels, m.panel("WORKSPACE", lines, left, height, 0))
		}
		panels = append(panels, m.logList(m.width-left-right, height))
		if right > 0 {
			panels = append(panels, m.inspector(right, height))
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, panels...)
	}
	footer := " Ctrl+P actions  Tab focus  j/k move  Enter inspect  f filters  / find  n/N match  o older  r reload  S save  R restore  q quit"
	if m.focus == 2 {
		footer = " Ctrl+P actions  j/k scroll  g/G start/end  f exact field filter  / find  S save  R restore  Esc logs  q quit"
	}
	if m.focus == 0 {
		footer = " Ctrl+P actions  Tab next panel  Enter logs  q quit"
	}
	status := m.status
	if m.loading {
		status = "Reading… · " + status
	}
	if m.page.More {
		if m.page.Next != "" {
			status = "More: o older · " + status
		} else {
			status = "More: reload · " + status
		}
	}
	if m.page.Partial {
		status = "PARTIAL · " + status
	}
	if m.budgetStopped {
		status = "STOPPED: 10000 events / 32 MiB session budget · r reload"
	}
	return header + "\n" + body + "\n" + base.Render(fit(" ", m.width)) + "\n" + m.style(t.Focus).Render(fit(footer, m.width)) + "\n" + m.style(t.Muted).Render(fit(" "+status, m.width))
}

func (m Model) logList(width, height int) string {
	t := m.options.Theme
	inner := width - 4
	serviceWidth := min(18, max(10, inner/4))
	columns := "  " + fit("TIME (UTC)", 13) + fit("LEVEL", 8) + fit("APP / SERVICE", serviceWidth) + "MESSAGE"
	lines := []string{" " + m.style(t.Muted).Render(fit(columns, inner)), " " + m.style(t.Border).Render(strings.Repeat("─", inner))}
	visible := max(1, height-6)
	for i := m.top; i < len(m.page.Events) && i < m.top+visible; i++ {
		e := m.page.Events[i]
		stamp := e.Text("time")
		if instant, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			stamp = instant.UTC().Format("15:04:05.000")
		}
		message := strings.SplitN(e.Text("message"), "\n", 2)[0]
		if strings.Contains(e.Text("message"), "\n") {
			message += " ↵"
		}
		level := strings.ReplaceAll(e.Text("level"), "\n", " ↵ ")
		color := t.Muted
		if level == "ERROR" {
			color = t.Error
		}
		if level == "WARN" {
			color = t.Warning
		}
		prefix := "  "
		if i == m.selected {
			prefix = "› "
		}
		service := strings.ReplaceAll(e.Text("service"), "\n", " ↵ ")
		row := prefix + fit(stamp, 13) + m.style(color).Render(fit(level, 8)) + fit(service, serviceWidth) + message
		row = fit(row, inner)
		if i == m.selected {
			bg, fg := t.Surface, t.Foreground
			if m.focus == 1 {
				bg, fg = t.Selection, t.SelectionText
			}
			row = lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg)).Render(fit(ansi.Strip(row), inner))
		}
		lines = append(lines, " "+row)
	}
	if !m.loaded {
		label := " Loading offline sample…"
		if m.options.LocalLab {
			label = " Querying local Elasticsearch…"
		}
		lines = append(lines, label)
	}
	if m.loaded && len(m.page.Events) == 0 && !m.loadError {
		lines = append(lines, " No events in this window")
	}
	if m.loadError {
		lines = append(lines, " Source read failed")
	}
	for len(lines) < height-3 {
		lines = append(lines, "")
	}
	count := fmt.Sprintf(" %d fictional events · focus: logs", len(m.page.Events))
	if m.focus != 1 {
		count = fmt.Sprintf(" %d fictional events · anchor %d", len(m.page.Events), m.selected+1)
	}
	if m.options.LocalLab {
		count = strings.Replace(count, "fictional events", "lab events", 1)
	}
	lines = append(lines, m.style(t.Muted).Render(count))
	return m.panel("LOGS / chronological", lines, width, height, 1)
}

func (m Model) inspector(width, height int) string {
	lines := m.inspectorLines(width - 6)
	visible := max(1, height-5)
	offset := max(0, min(m.inspectorOffset, max(0, len(lines)-visible)))
	shown := []string{}
	for _, line := range lines[offset:min(len(lines), offset+visible)] {
		shown = append(shown, "  "+line)
	}
	for len(shown) < height-3 {
		shown = append(shown, "")
	}
	shown = append(shown, m.style(m.options.Theme.Muted).Render(fmt.Sprintf("  Lines %d–%d / %d · redacted", offset+1, min(len(lines), offset+visible), len(lines))))
	return m.panel("INSPECTOR", shown, width, height, 2)
}

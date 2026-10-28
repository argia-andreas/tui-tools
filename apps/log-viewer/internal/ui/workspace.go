package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"logviewer/internal/config"
	"logviewer/internal/logs"
	"terminalui"
)

// Cache only query metadata, never event payloads. Reload on explicit save,
// restore, or entry into the workspace; rendering never touches the filesystem.
func (m *Model) reloadSaved() {
	m.saved = nil
	m.savedError = false
	if m.options.SearchesPath != "" {
		saved, err := config.LoadSearches(m.options.SearchesPath)
		m.savedError = err != nil
		for _, entry := range saved {
			if entry.Query.Scope.Source == m.options.Query.Scope.Source {
				m.saved = append(m.saved, entry)
			}
		}
	}
	m.workspaceRow = min(m.workspaceRow, len(m.saved)+1)
}

func (m Model) workspaceFrame(bounds terminalui.Rect) terminalui.Frame {
	width, height := bounds.W, bounds.H
	f := terminalui.Frame{Bounds: bounds, Regions: []terminalui.Region{{ID: "workspace", Kind: terminalui.PaneRegion, Bounds: bounds}}}
	t := m.options.Theme
	row := func(label string, index int) string {
		if m.focus == 0 && m.workspaceRow == index {
			return m.style(t.SelectionText).Background(lipgloss.Color(t.Selection)).Render(fit("› "+oneLine(label), width-2))
		}
		return "  " + oneLine(label)
	}
	source := "fixture / local"
	if m.options.LocalLab {
		source = "local-elastic"
	}
	lines := []string{"", m.style(t.Muted).Render(" SOURCES"), row(source, 0), "", m.style(t.Muted).Render(" SAVED SEARCHES")}
	f.Regions = append(f.Regions, terminalui.Region{ID: "source", Kind: terminalui.DestinationRegion, Bounds: terminalui.Rect{X: bounds.X + 1, Y: bounds.Y + 3, W: width - 2, H: 1}})
	visible := max(1, height-13)
	start := max(0, min(m.workspaceRow-1-visible+1, len(m.saved)-visible))
	for i := start; i < min(len(m.saved), start+visible); i++ {
		f.Regions = append(f.Regions, terminalui.Region{ID: "saved:" + m.saved[i].Query.Scope.Source + ":" + m.saved[i].Name, Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: bounds.X + 1, Y: bounds.Y + 1 + len(lines), W: width - 2, H: 1}})
		lines = append(lines, row(m.saved[i].Name, i+1))
	}
	if len(m.saved) == 0 {
		label := " S to save a search"
		if m.savedError {
			label = " Saved file unavailable"
		}
		lines = append(lines, m.style(t.Muted).Render(label))
	}
	f.Regions = append(f.Regions, terminalui.Region{ID: "restore", Kind: terminalui.DestinationRegion, Bounds: terminalui.Rect{X: bounds.X + 1, Y: bounds.Y + 2 + len(lines), W: width - 2, H: 1}})
	lines = append(lines, "", row("R All saved searches", len(m.saved)+1), "", m.style(t.Muted).Render(" Fictional records"))
	if m.options.LocalLab {
		lines = append(lines, m.style(t.Muted).Render(" TLS / read-only key"))
	} else {
		lines = append(lines, m.style(t.Muted).Render(" No network access"))
	}
	f.Lines = strings.Split(m.panel("WORKSPACE", lines, width, height, 0), "\n")
	return f
}

func (m Model) openWorkspaceRow() (tea.Model, tea.Cmd) {
	if m.workspaceRow == 0 {
		m.focus = 1
		return m, nil
	}
	if m.workspaceRow > len(m.saved) {
		return m.openRestore()
	}
	q := m.saved[m.workspaceRow-1].Query
	if q.Scope.Source != m.options.Query.Scope.Source || logs.ValidateQuery(q) != nil {
		m.status = "Saved query unavailable for this source"
		return m, nil
	}
	m.options.Query, m.focus = q, 1
	return m.refresh()
}

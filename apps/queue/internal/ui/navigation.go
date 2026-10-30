package ui

import (
	"encoding/json"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/explorer"
	"terminalui"
)

// IDs carry native identities, never display labels or positions in a page.
func resourceID(r explorer.Resource) string {
	b, _ := json.Marshal(r)
	return "resource:" + string(b)
}

func branchID(b branch) string {
	p, _ := json.Marshal([]string{b.profile, b.namespace, b.kind, b.topic})
	return "branch:" + string(p)
}

func (m Model) messageID(index int) string {
	msg := m.page.Messages[index]
	b, _ := json.Marshal([]any{m.resource, m.dlq, m.from, msg.Sequence, msg.ID})
	return "message:" + string(b)
}

func (m Model) contentBounds() terminalui.Rect {
	b := terminalui.Rect{Y: 3, W: m.contentWidth(), H: m.height - 6}
	if m.showSidebar() {
		b.X = 24
	}
	if m.compactReading() {
		b.Y, b.H = 1, m.height-2
	}
	return b
}

func (m Model) minimumHeight() int {
	if m.provider != nil {
		return 18
	}
	return 16
}

func (m Model) workspaceFits() bool { return m.width >= 60 && m.height >= m.minimumHeight() }

func (m Model) compactQuitVisible() bool {
	return m.env.page == "quit" && m.width >= 48 && m.height >= 3
}

// Renderers emit their hit regions as they append the corresponding visible
// lines. Both View and input rebuild this frame from current state.
func (m Model) workspaceFrame() terminalui.Frame {
	f := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}}
	if !m.workspaceFits() || m.review.phase != "" || m.env.page != "" {
		return f
	}
	b := m.contentBounds()
	if m.screen == "messages" {
		f.Lines = m.messageView(b.W, b.H, &f)
	} else {
		f.Lines = m.dashboardView(b.W, b.H, &f)
	}
	if m.showSidebar() {
		nav := terminalui.Frame{Bounds: f.Bounds}
		lines := m.sidebarView(b.H, &nav)
		for i := range f.Lines {
			f.Lines[i] = lines[i] + f.Lines[i]
		}
		f.Regions = append(nav.Regions, f.Regions...)
	}
	f.Regions = append(f.Regions, m.toolbarFrame().Regions...)
	return f
}

// addHit clips rows/panes to their rendered body; controls must fit in full.
func addHit(frames []*terminalui.Frame, clip terminalui.Rect, id string, kind terminalui.RegionKind, b terminalui.Rect) {
	if len(frames) == 0 {
		return
	}
	x, y := max(b.X, clip.X), max(b.Y, clip.Y)
	r := terminalui.Rect{X: x, Y: y, W: max(0, min(b.X+b.W, clip.X+clip.W)-x), H: max(0, min(b.Y+b.H, clip.Y+clip.H)-y)}
	if r.W == 0 || r.H == 0 || ((kind == terminalui.ControlRegion || kind == terminalui.DestinationRegion) && r != b) {
		return
	}
	frames[0].Regions = append(frames[0].Regions, terminalui.Region{ID: id, Kind: kind, Bounds: r})
}

func panelBody(b terminalui.Rect) terminalui.Rect {
	return terminalui.Rect{X: b.X + 1, Y: b.Y + 1, W: max(0, b.W-2), H: max(0, b.H-2)}
}

func emptyReason(n int) string { return unavailable(n == 0) }
func unavailable(blocked bool) string {
	if blocked {
		return "Unavailable"
	}
	return ""
}

func (m *Model) focusWorkspace(id string) {
	readerFocus := func(id string) bool {
		return id == "inspector" || strings.HasPrefix(id, "inspector-") || strings.HasPrefix(id, "reader-") || id == "terminalui:surface:close"
	}
	if m.focus != id && !(readerFocus(m.focus) && readerFocus(id)) {
		m.reader.mode, m.reader.pending = "", ""
	}
	m.focus = id
	for index, row := range m.tree() {
		if id == branchID(row.branch) || id == "tree:"+resourceID(row.resource) {
			m.nav = index + 4
		}
	}
	for index, name := range []string{"dashboard", "queues", "topics", "settings"} {
		if id == "nav:"+name {
			m.nav = index
		}
	}
	if m.screen == "settings" {
		for index := 0; index < m.settingsCount(); index++ {
			if id == m.settingID(index) {
				m.setting = index
			}
		}
	}
}

func (m Model) workspacePane() string {
	if m.inspectorFocused() {
		return "inspector"
	}
	if m.focus == "navigation" || strings.HasPrefix(m.focus, "nav:") || strings.HasPrefix(m.focus, "tree:") || strings.HasPrefix(m.focus, "branch:") {
		return "navigation"
	}
	return "content"
}

func (m Model) selectedNavigationID() string {
	if m.focus == "navigation" {
		if m.nav < 4 {
			return "nav:" + []string{"dashboard", "queues", "topics", "settings"}[m.nav]
		}
		rows := m.tree()
		if m.nav-4 < len(rows) {
			r := rows[m.nav-4]
			if r.leaf {
				return "tree:" + resourceID(r.resource)
			}
			return "branch-row:" + branchID(r.branch)
		}
	}
	if m.focus == "content" {
		if m.screen == "messages" && m.message < len(m.page.Messages) && !m.loading {
			return m.messageID(m.message)
		}
		if items := m.visible(); m.screen != "settings" && m.selected < len(items) {
			return resourceID(items[m.selected].resource)
		}
		if m.screen == "settings" {
			return m.settingID(m.setting)
		}
	}
	return ""
}

func (m Model) settingID(index int) string {
	if index < 5 {
		return "setting:" + strconv.Itoa(index)
	}
	if m.provider != nil {
		if index == 5 {
			return "env:manage"
		}
		if index == 6 {
			return "env:inclusion"
		}
		return ""
	}
	if index-5 < len(m.items) {
		return "watch:" + resourceID(m.items[index-5].resource)
	}
	return ""
}

func (m Model) inspectMessage() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.page.Messages) > 0 {
		m.focusWorkspace("inspector")
		m.reader.revealed = true
		m.keepCursorVisible()
	}
	return m, nil
}

func (m Model) changePage(next bool) (tea.Model, tea.Cmd) {
	if next && m.page.NextSequence != nil {
		m.history = append(m.history, m.from)
		return m.peek(*m.page.NextSequence)
	}
	if !next && len(m.history) > 0 {
		from := m.history[len(m.history)-1]
		m.history = m.history[:len(m.history)-1]
		return m.peek(from)
	}
	return m, nil
}

func (m Model) workspaceInput(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if m.reader.load != "" {
		return m, nil, false
	}
	f := m.workspaceFrame()
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
		// Settings retain their local selected-option grammar. The selected
		// control must still be present in the current rendered body.
		if m.focus == "content" && m.screen == "settings" {
			for _, region := range f.Regions {
				if region.ID == m.settingID(m.setting) {
					next, cmd := m.changeSetting()
					return next, cmd, true
				}
			}
			return m, nil, true
		}
	}
	r := terminalui.HandleNavigation(msg, f, m.focus, m.selectedNavigationID())
	if r.Intent == nil {
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "enter" {
			return m, nil, true // No hidden/stale fallback activation.
		}
		return m, nil, r.Consumed
	}
	i := *r.Intent
	if i.Kind == terminalui.Focus {
		m.focusWorkspace(i.ID)
		return m, nil, true
	}
	if i.Kind == terminalui.Scroll {
		switch i.ID {
		case "navigation":
			m.nav = min(max(0, m.nav+i.Delta), len(m.tree())+3)
		case "inspector":
			if !m.reader.copying {
				m.moveReaderRows(i.Delta)
				m.keepCursorVisible()
			}
		case "content":
			if m.screen == "messages" {
				m.selectMessage(min(max(0, m.message+i.Delta), max(0, len(m.page.Messages)-1)))
			} else if m.screen == "settings" {
				m.setting = min(max(0, m.setting+i.Delta), m.settingsCount()-1)
			} else {
				m.selected = min(max(0, m.selected+i.Delta), max(0, len(m.visible())-1))
			}
		}
		return m, nil, true
	}
	next, cmd := m.workspaceIntent(i)
	return next, cmd, true
}

func (m *Model) selectMessage(index int) {
	if index != m.message && index >= 0 && index < len(m.page.Messages) {
		m.message = index
		m.resetInspector()
	}
}

func (m Model) workspaceIntent(i terminalui.Intent) (tea.Model, tea.Cmd) {
	id := i.ID
	if strings.HasPrefix(id, "nav:") {
		for index, name := range []string{"dashboard", "queues", "topics", "settings"} {
			if id == "nav:"+name {
				m.nav = index
			}
		}
		return m.execute(strings.TrimPrefix(id, "nav:"))
	}
	for index, row := range m.tree() {
		if !row.leaf && id == "branch-row:"+branchID(row.branch) {
			m.nav = index + 4
			m.focusWorkspace("navigation")
			if i.Kind == terminalui.Activate {
				return m.toggleBranch(row.branch)
			}
			return m, nil
		}
		if !row.leaf && id == branchID(row.branch) {
			m.nav = index + 4
			m.focusWorkspace("navigation")
			return m.toggleBranch(row.branch)
		}
		if row.leaf && id == "tree:"+resourceID(row.resource) {
			m.nav = index + 4
			return m.open(row.resource, false)
		}
	}
	for index := 0; index < m.settingsCount(); index++ {
		if m.screen == "settings" && id == m.settingID(index) {
			m.setting = index
			return m.changeSetting()
		}
	}
	for index, item := range m.visible() {
		if id == resourceID(item.resource) {
			m.selected = index
			m.focusWorkspace("content")
			if i.Kind == terminalui.Activate {
				return m.open(item.resource, false)
			}
			return m, nil
		}
	}
	if m.screen == "messages" {
		for index := range m.page.Messages {
			if id == m.messageID(index) {
				m.selectMessage(index)
				m.focusWorkspace("content")
				if i.Kind == terminalui.Activate {
					return m.inspectMessage()
				}
				return m, nil
			}
		}
	}
	if i.Kind != terminalui.Activate {
		return m, nil
	}
	switch id {
	case "inspect":
		return m.inspectMessage()
	case "next", "previous":
		return m.changePage(id == "next")
	case "actions":
		m.palette, m.paletteState = true, terminalui.Palette{}
	case "terminalui:surface:close":
		return m.readerAction("inspector-back")
	case "inspector-back", "inspector-resend":
		return m.readerAction(id)
	case "reader-line", "reader-char", "reader-visual-line", "reader-body", "reader-selection", "reader-clear", "reader-help":
		return m.readerLocalAction(id)
	default:
		return m.execute(id)
	}
	return m, nil
}

func (m *Model) repairWorkspaceFocus() {
	if m.review.phase != "" || m.reader.load != "" {
		return
	}
	frame := m.workspaceFrame()
	for _, r := range frame.Regions {
		if r.ID == m.focus && r.Kind != terminalui.RowRegion {
			return
		}
	}
	// If compact reading covers the list, return to its visible text pane;
	// otherwise content is the safe return owner. Empty readers expose no actions.
	for _, r := range frame.Regions {
		if r.ID == "inspector" && r.Kind == terminalui.PaneRegion && m.inspectorFocused() {
			m.focusWorkspace("inspector")
			return
		}
	}
	m.focusWorkspace("content")
	if len(m.reader.lines) == 0 {
		m.reader.revealed = false
	}
}

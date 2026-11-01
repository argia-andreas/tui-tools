package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"streams/internal/stream"
	"terminalui"
)

func (m Model) paint(s, color string) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(lipgloss.Color(m.style().Background))
	// Nested token/selection resets must restore the surrounding pane, not
	// expose the user's terminal background between tokens.
	base := strings.TrimSuffix(style.Render(""), "\x1b[0m")
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+base)
	return style.Render(s)
}
func safe(s string) string { return stream.SafeText([]byte(s)) }
func (m Model) panel(title, id string, b terminalui.Rect, lines []string, regions []terminalui.Region) terminalui.Frame {
	s := m.style()
	if m.focus == id {
		s.Border = s.Focus
		title = "> " + title
	}
	body := make([]string, min(len(lines), max(0, b.H-2)))
	for i := range body {
		body[i] = m.paint(lines[i], s.Foreground)
	}
	return terminalui.Frame{
		Bounds: b, Regions: append([]terminalui.Region{{ID: id, Kind: terminalui.PaneRegion, Bounds: b}}, regions...),
		Lines: terminalui.RenderPanel(terminalui.PanelSpec{Title: " " + title + " ", Lines: body}, b.W, b.H, s),
	}
}
func (m Model) selectedLine(text string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.style().SelectionText)).Background(lipgloss.Color(m.style().Selection)).Render(ansi.Strip(text))
	}
	return m.paint(text, m.style().Foreground)
}
func (m Model) headerActions() []terminalui.Action {
	return []terminalui.Action{{ID: "palette", Label: "Actions", Keys: []string{"ctrl+p"}}, {ID: "help", Label: "Help", Keys: []string{"f1"}}, {ID: "topics", Label: "Topics", Keys: []string{"t"}}, {ID: "groups", Label: "Groups", Keys: []string{"g"}}, {ID: "density", Label: m.densityLabel()}}
}
func (m Model) footerActions() []terminalui.Action {
	ids := []string{"open", "find", "refresh"}
	if m.view == "events" {
		ids = []string{"open", "seek", "follow"}
		if m.loading || m.following {
			ids = append(ids, "pause")
		} else if len(m.cursor.Positions) > 0 {
			ids = append(ids, "resume")
		}
	}
	var out []terminalui.Action
	for _, id := range ids {
		for _, a := range m.actions() {
			if a.ID == id {
				a.Label = map[string]string{"open": "Open", "seek": "Seek", "follow": "Follow latest", "find": "Find", "refresh": "Refresh", "pause": "Pause", "resume": "Continue", "back": "Back", "copy": "Copy", "mode": "Mode"}[id]
				out = append(out, a)
			}
		}
	}
	return out
}
func (m Model) overlayBounds(w, h int) terminalui.Rect {
	w = min(w, m.width-4)
	h = min(h, m.height-4)
	return terminalui.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}
}
func (m Model) paletteFrame() terminalui.Frame {
	return m.palette.View(m.actions(), m.overlayBounds(88, 23), m.style())
}
func (m Model) helpTopics() []terminalui.HelpTopic {
	return []terminalui.HelpTopic{
		{Group: "Navigate", Keys: "Tab / Shift+Tab", Description: "Visible controls and panes; click row selects, Enter or Inspect opens"},
		{Group: "Navigate", Keys: "j/k, arrows, PgUp/PgDn", Description: "Moving the selected event updates the inspector; Enter opens payload, Esc closes"},
		{Group: "Appearance", Keys: "Actions / density", Description: "Compact default or Comfortable two-line events; Save appearance remembers the choice"},
		{Group: "Payload", Keys: "Tab / j/k / PgUp/PgDn", Description: "Tab selects Value, Key, Headers; j/k scrolls payload or moves through ordered headers"},
		{Group: "Reader", Keys: "h/l, [ / ]", Description: "Scroll horizontally; previous/next 256 KiB raw/hex window"},
		{Group: "Reader", Keys: "v / y", Description: "JSON, raw, hex mode / copy full selected field (hex mode copies hex)"},
		{Group: "Safety", Keys: "Read only", Description: "No producer, group membership, acknowledgements or offset commits; broker ACLs recommended"},
		{Group: "Seek", Keys: "s", Description: seekHint},
		{Group: "Find", Keys: "/", Description: "Topic-name filter in Topics; case-sensitive literal key/value find in loaded events"},
		{Group: "Follow", Keys: "Space / c / F", Description: "Pause fetching / resume unread positions / deliberately jump to latest"},
		{Group: "Limits", Keys: "500 / 4 MiB / 5s", Description: "Historical bounds; gaps may leave scan incomplete. No global partition ordering"},
	}
}
func (m Model) helpFrame() terminalui.Frame {
	return terminalui.RenderHelp(m.actions(), m.helpTopics(), m.helpOffset, m.overlayBounds(94, 28), m.style())
}

func (m Model) frames() []terminalui.Frame {
	b := terminalui.Rect{Y: 3, W: m.width, H: m.height - 7}
	left, right := 0, 0
	if m.view == "events" && !m.settings.Comfortable && m.width >= 140 && m.settings.Sidebar {
		left = 22
	}
	if (m.view == "events" || m.view == "topics") && m.width >= 110 && m.settings.Inspector {
		right = min(m.width-left-50, max(44, (m.width-left)*2/5))
	}
	var fs []terminalui.Frame
	if m.view == "events" && m.settings.Comfortable {
		var topics []terminalui.Action
		for i, topic := range m.visibleTopics() {
			label := safe(topic.Ref.Name)
			if topic.Ref.Name == m.request.Topic.Name {
				label = "• " + label
				if m.style().ASCII {
					label = "* " + safe(topic.Ref.Name)
				}
			}
			topics = append(topics, terminalui.Action{ID: fmt.Sprintf("topic:%d", i), Label: label})
		}
		fs = append(fs, terminalui.RenderActionBar(topics, terminalui.Rect{Y: 2, W: m.width, H: 1}, m.focus, m.style()))
	}
	if left > 0 {
		fs = append(fs, m.topicsFrame(terminalui.Rect{Y: 3, W: left, H: b.H}, "sidebar"))
	}
	fs = append(fs, m.listFrame(terminalui.Rect{X: left, Y: 3, W: m.width - left - right, H: b.H}))
	if right > 0 {
		bounds := terminalui.Rect{X: m.width - right, Y: 3, W: right, H: b.H}
		if m.view == "topics" {
			fs = append(fs, m.topicMetadataFrame(bounds))
		} else {
			fs = append(fs, m.inspectorFrame(bounds))
		}
	}
	return fs
}
func (m Model) topicMetadataFrame(b terminalui.Rect) terminalui.Frame {
	var lines []string
	if t, ok := m.selectedTopic(); ok {
		lines = append(lines, " "+safe(t.Ref.Name), " Partition / leader / replicas / ISR")
		start := min(m.scroll, max(0, len(t.Partitions)-1))
		for _, p := range t.Partitions[start:] {
			line := fmt.Sprintf(" p%d / %d / %v / %v", p.ID, p.Leader, p.Replicas, p.InSync)
			if p.Err != nil {
				line = fmt.Sprintf(" p%d / %s", p.ID, safe(p.Err.Error()))
			}
			lines = append(lines, line)
		}
		lines = append(lines, " s chooses partitions · PgUp/PgDn scrolls", " Metadata is a current observation")
	}
	return m.panel("PARTITIONS", "metadata", b, lines, nil)
}
func (m Model) topicsFrame(b terminalui.Rect, id string) terminalui.Frame {
	ts := m.visibleTopics()
	lines := []string{m.paint(" / filter: "+safe(m.filter), m.style().Muted)}
	var rs []terminalui.Region
	start := max(0, m.topicIndex-max(0, b.H-5))
	for i := start; i < len(ts) && len(lines) < b.H-2; i++ {
		t := ts[i]
		label := " " + safe(t.Ref.Name)
		if id == "list" {
			label = fmt.Sprintf(" %-42s %4d partitions", safe(t.Ref.Name), len(t.Partitions))
		}
		lines = append(lines, m.selectedLine(terminalui.Fit(label, b.W-2), i == m.topicIndex))
		rs = append(rs, terminalui.Region{ID: fmt.Sprintf("topic:%d", i), Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: b.X + 1, Y: b.Y + len(lines), W: b.W - 2, H: 1}})
	}
	if len(ts) == 0 {
		lines = append(lines, " No matching topics · Refresh / clear filter")
	}
	return m.panel(fmt.Sprintf("TOPICS / %d", len(ts)), id, b, lines, rs)
}
func (m Model) listFrame(b terminalui.Rect) terminalui.Frame {
	if m.view == "topics" {
		return m.topicsFrame(b, "list")
	}
	if m.view == "events" {
		return m.eventsFrame(b)
	}
	var lines []string
	var rs []terminalui.Region
	title := strings.ToUpper(m.view)
	rows := m.rowCount()
	start := max(0, min(m.row-max(0, b.H-6), max(0, rows-(b.H-4))))
	if m.view == "group" {
		title = "GROUP / " + safe(m.group.Name)
		lag := int64(0)
		partial := m.group.Partial
		for _, p := range m.group.Partitions {
			n, s := stream.Lag(p)
			lag += n
			partial = partial || s != ""
		}
		lines = append(lines, fmt.Sprintf(" %s · observed %s UTC · lag %d (partial: %t)", safe(m.group.State), m.group.Finished.UTC().Format("15:04:05"), lag, partial), " TOPIC / P                  COMMIT       HWM       LAG / STATE")
	} else {
		lines = append(lines, m.paint(" Consumer groups · select then Open", m.style().Muted))
	}
	for i := start; i < rows && len(lines) < b.H-2; i++ {
		text := ""
		switch m.view {
		case "groups":
			text = " " + safe(m.groups[i])
		case "group":
			p := m.group.Partitions[i]
			n, s := stream.Lag(p)
			lag := fmt.Sprint(n)
			if s != "" {
				lag = s
			}
			text = fmt.Sprintf(" %-25s %10s %9s %s", safe(p.Topic)+fmt.Sprintf(" / %d", p.Partition), offsetLabel(p.Committed), offsetLabel(p.HighWatermark), lag)
		}
		lines = append(lines, m.selectedLine(terminalui.Fit(text, b.W-2), i == m.row))
		rs = append(rs, terminalui.Region{ID: fmt.Sprintf("row:%d", i), Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: b.X + 1, Y: b.Y + len(lines), W: b.W - 2, H: 1}})
	}
	if rows == 0 {
		line := " No records in this view"
		if m.loading {
			line = " Loading…"
		}
		lines = append(lines, line)
	}
	return m.panel(title, "list", b, lines, rs)
}

func (m Model) densityLabel() string {
	if m.settings.Comfortable {
		return "Comfortable"
	}
	return "Compact"
}

func (m Model) preview(data []byte, value bool) string {
	if data == nil {
		label := "<null>"
		if value {
			label = "<tombstone>"
		}
		return m.paint(label, m.style().Error)
	}
	if len(data) == 0 {
		return m.paint("<empty>", m.style().Muted)
	}
	end := min(len(data), 256)
	if end < len(data) {
		for end > 253 && !utf8.RuneStart(data[end]) {
			end--
		}
	}
	window := data[:end]
	if !utf8.Valid(window) || bytes.IndexFunc(window, func(r rune) bool { return r < 32 && r != '\n' && r != '\r' && r != '\t' }) >= 0 {
		return m.paint(fmt.Sprintf("<binary · %d bytes>", len(data)), m.style().Muted)
	}
	if len(data) <= 4096 && json.Valid(data) {
		var compact bytes.Buffer
		_ = json.Compact(&compact, data)
		return highlightJSON(stream.SafeText(compact.Bytes()))
	}
	text := stream.SafeText(window)
	if len(data) > len(window) {
		text += "…"
	}
	return m.paint(text, m.style().Foreground)
}

func (m Model) eventsFrame(b terminalui.Rect) terminalui.Frame {
	lines := []string{m.paint(" Committed only · arrival order · "+m.densityLabel(), m.style().Muted)}
	stride := 1
	keyWidth := max(8, min(18, (b.W-32)/3))
	if m.settings.Comfortable {
		stride = 4
	} else {
		lines = append(lines, m.paint("   P     OFFSET  UTC           "+terminalui.Fit("KEY", keyWidth)+"  VALUE", m.style().Muted))
	}
	capacity := max(1, (b.H-2-len(lines))/stride)
	start := max(0, m.row-capacity+1)
	var regions []terminalui.Region
	for i := start; i < len(m.records) && len(lines)+min(stride, 4) <= b.H-2; i++ {
		r := m.records[i]
		marker := "  "
		if i == m.row {
			marker = "> "
		}
		meta := m.paint(fmt.Sprintf("%2d ", r.ID.Partition), m.style().Muted) + m.paint(fmt.Sprintf("%10d", r.ID.Offset), m.style().Warning) + m.paint("  "+r.Time.UTC().Format("15:04:05.000")+"  ", m.style().Muted)
		key := m.preview(r.Key, false)
		if r.Key != nil && len(r.Key) > 0 {
			key = m.paint(ansi.Strip(key), m.style().Focus)
		}
		y := b.Y + 1 + len(lines)
		if m.settings.Comfortable {
			inner := max(1, b.W-6)
			metadataWidth := min(24, inner/2)
			payloadWidth := max(1, inner-metadataWidth-1)
			vertical, horizontal, tl, tr, bl, br := "│", "─", "┌", "┐", "└", "┘"
			border := m.style().Border
			if i == m.row {
				vertical, border = "┃", m.style().Focus
			}
			if m.style().ASCII {
				vertical, horizontal, tl, tr, bl, br = "|", "-", "+", "+", "+", "+"
			}
			divider := "│"
			if m.style().ASCII {
				divider = "|"
			}
			cell := func(text string, width int) string {
				return terminalui.Fit(ansi.Truncate(text, width, "…"), width)
			}
			row := func(metadata, content string) string {
				return " " + m.paint(vertical, border) + cell(metadata, metadataWidth) + m.paint(divider, m.style().Border) + cell(content, payloadWidth) + m.paint(vertical, border)
			}
			lines = append(lines,
				" "+m.paint(tl+strings.Repeat(horizontal, inner)+tr, border),
				row(m.paint(fmt.Sprintf(" P%d  %d", r.ID.Partition, r.ID.Offset), m.style().Warning), " key: "+key),
				row(m.paint(" "+r.Time.UTC().Format("2006-01-02 15:04:05"), m.style().Muted), " "+m.preview(r.Value, true)),
				" "+m.paint(bl+strings.Repeat(horizontal, inner)+br, border))
		} else {
			line := marker + meta + terminalui.Fit(key, keyWidth) + "  " + m.preview(r.Value, true)
			lines = append(lines, m.selectedLine(terminalui.Fit(ansi.Truncate(line, b.W-2, "…"), b.W-2), i == m.row))
		}
		regions = append(regions, terminalui.Region{ID: fmt.Sprintf("row:%d", i), Kind: terminalui.RowRegion, Bounds: terminalui.Rect{X: b.X + 1, Y: y, W: b.W - 2, H: min(stride, 4)}})
	}
	if len(m.records) == 0 {
		label := " No records in this view"
		if m.loading {
			label = " Loading…"
		}
		lines = append(lines, m.paint(label, m.style().Muted))
	}
	return m.panel("EVENTS / "+safe(m.request.Topic.Name), "list", b, lines, regions)
}

func offsetLabel(v stream.OffsetValue) string {
	if !v.Known {
		return "?"
	}
	return fmt.Sprint(v.Value)
}
func (m Model) stateLabel() string {
	if m.following {
		return "FOLLOWING"
	}
	if m.loading {
		return "READING"
	}
	if m.paused {
		return "PAUSED"
	}
	return "BOUNDED"
}
func (m Model) rowCount() int {
	switch m.view {
	case "topics":
		return len(m.visibleTopics())
	case "events":
		return len(m.records)
	case "groups":
		return len(m.groups)
	case "group":
		return len(m.group.Partitions)
	}
	return 0
}

func (m Model) View() string {
	if !m.fits() {
		return fmt.Sprintf("streams · resize to at least 60×16 (now %d×%d)\nEsc cancels overlays · q / Ctrl+C quit", m.width, m.height)
	}
	base := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: make([]string, m.height)}
	for i := range base.Lines {
		base.Lines[i] = m.paint(strings.Repeat(" ", m.width), m.style().Foreground)
	}
	mode := "READ ONLY"
	if m.options.Fixture {
		mode = "OFFLINE FIXTURE / FICTIONAL"
	}
	base.Lines[0] = m.paint(terminalui.Fit(" streams  /  "+safe(m.options.Connection)+"  /  "+mode, m.width), m.style().Muted)
	put := func(f terminalui.Frame) {
		for y, line := range f.Lines {
			dy := f.Bounds.Y + y
			if dy >= 0 && dy < len(base.Lines) {
				old := base.Lines[dy]
				base.Lines[dy] = ansi.Cut(old, 0, f.Bounds.X) + terminalui.Fit(line, f.Bounds.W) + ansi.Cut(old, f.Bounds.X+f.Bounds.W, m.width)
			}
		}
	}
	put(terminalui.RenderActionBar(m.headerActions(), terminalui.Rect{Y: 1, W: m.width, H: 1}, m.focus, m.style()))
	for _, f := range m.frames() {
		put(f)
	}
	put(terminalui.RenderActionBar(m.footerActions(), terminalui.Rect{Y: m.height - 4, W: m.width, H: 1}, m.focus, m.style()))
	base.Lines[m.height-3] = m.paint(terminalui.Fit(ansi.Truncate(" "+safe(m.status), m.width, "…"), m.width), m.style().Warning)
	positions := ""
	for i, p := range m.cursor.Positions {
		if i >= 8 {
			positions += " …"
			break
		}
		positions += fmt.Sprintf(" p%d:%d", p.Partition, p.Next)
	}
	base.Lines[m.height-2] = m.paint(terminalui.Fit(" "+m.stateLabel()+" · Position"+positions+" · find loaded: "+safe(m.find), m.width), m.style().Muted)
	base.Lines[m.height-1] = m.paint(terminalui.Fit(" ↑↓ select + preview · Enter payload · / find · s seek · Ctrl+P actions", m.width), m.style().Muted)
	if m.detail {
		base = terminalui.Overlay(base, m.payloadFrame(), m.style())
	}
	if m.paletteOpen {
		base = terminalui.Overlay(base, m.paletteFrame(), m.style())
	}
	if m.editor != nil {
		base = terminalui.Overlay(base, terminalui.RenderSurface(m.editorSpec(), m.overlayBounds(96, 12), m.style()), m.style())
	}
	if m.help {
		base = terminalui.Overlay(base, m.helpFrame(), m.style())
	}
	return strings.Join(base.Lines, "\n")
}

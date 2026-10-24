package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
	"servicebusexplorer/internal/explorer"
)

type textPoint struct{ line, column int }
type textLine struct {
	text, ending, section string
	characters            []string
	number                int
}
type inspectorState struct {
	cursor, anchor textPoint
	mode, pending  string
	lines          []textLine
	top            int
	preferred      int
	helpTop        int
	revealed, help bool
	load, loading  string
	choice         int
	cancel         context.CancelFunc
	epoch          uint64
	copying        bool
}
type copied struct {
	epoch     uint64
	confirmed bool
	scope     string
	err       error
}
type fullMessage struct {
	epoch   uint64
	message explorer.Message
	err     error
}

func characters(s string) []string {
	var result []string
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		result = append(result, g.Str())
	}
	return result
}

func (m Model) inspected() explorer.Message {
	if m.message < len(m.page.Messages) {
		return m.page.Messages[m.message]
	}
	return explorer.Message{}
}

func (m *Model) resetInspector() {
	if m.reader.cancel != nil {
		m.reader.cancel()
	}
	m.reader = inspectorState{epoch: m.reader.epoch + 1}
	m.buildDocument()
}

func (m *Model) buildDocument() {
	m.reader.lines = nil
	if len(m.page.Messages) == 0 {
		return
	}
	msg := m.inspected()
	add := func(text, section string) {
		parts := strings.SplitAfter(text, "\n")
		if len(parts) > 1 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		for n, part := range parts {
			line := textLine{section: section, number: n + 1, text: part}
			if strings.HasSuffix(part, "\n") {
				line.text, line.ending = strings.TrimSuffix(part, "\n"), "\n"
				if strings.HasSuffix(line.text, "\r") {
					line.text, line.ending = strings.TrimSuffix(line.text, "\r"), "\r\n"
				}
			}
			line.characters = characters(line.text)
			m.reader.lines = append(m.reader.lines, line)
		}
	}
	add(string(msg.Body), "body")
	properties, err := json.MarshalIndent(msg.Properties, "", "  ")
	if err != nil {
		properties = []byte("[properties cannot be represented as JSON]")
	}
	add(string(properties), "properties")
	add(fmt.Sprintf("ID: %s\nSubject: %s\nContent-Type: %s\nSequence: %d\nDeliveries: %d\nEnqueued: %s\nReason: %s\nDescription: %s\nSource: %s / %s / %s",
		msg.ID, msg.Subject, msg.ContentType, msg.Sequence, msg.DeliveryCount,
		msg.EnqueuedAt.UTC().Format(time.RFC3339), msg.DeadLetterReason, msg.DeadLetterDescription,
		m.resource.Profile, m.resource.Namespace, m.resource.Name()), "metadata")
}

func (m Model) inspectorFocused() bool {
	return m.screen == "messages" && (m.focus == "inspector" || m.focus == "inspector-back" || m.focus == "inspector-resend")
}

func (m Model) splitInspector() bool {
	return m.contentWidth() >= 98
}

func (m Model) inspectorWidth() int {
	width := m.contentWidth()
	if m.splitInspector() {
		return width - width*40/100
	}
	return width
}

func (m Model) inspectorCapacity() int {
	height := m.height - 9
	if m.compactReading() {
		height = m.height - 2
	}
	return max(1, height-10)
}

// Display controls as data; clipboard commands use the original grapheme bytes.
func visibleCharacter(s string, column int) string {
	if s == "\t" {
		return strings.Repeat(" ", 8-column%8)
	}
	if clean(s, false) != s {
		return "�"
	}
	return s
}

type inspectorRow struct {
	text  string
	point textPoint
}

func (m Model) inspectorRows(width int) ([]inspectorRow, int) {
	var rows []inspectorRow
	cursorRow := 0
	available := max(1, width-10)
	for n, line := range m.reader.lines {
		if n == 0 || line.section != m.reader.lines[n-1].section {
			label := strings.ToUpper(line.section) + " / read-only"
			if line.section == "body" {
				state := "FULL BODY / read-only"
				if m.inspected().PreviewTruncated {
					state = "BODY / PREVIEW TRUNCATED"
				}
				label = fmt.Sprintf("%s · %d bytes · %s", state, len(m.inspected().Body), clean(m.inspected().ContentType, false))
			}
			rows = append(rows, inspectorRow{m.paint("  "+label, m.theme.Muted), textPoint{-1, 0}})
		}
		first, cell := 0, 0
		var content, span strings.Builder
		spanKind := -1
		flushSpan := func() {
			if span.Len() == 0 {
				return
			}
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Foreground)).Background(lipgloss.Color(m.theme.Background))
			if spanKind&1 != 0 {
				style = style.Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection))
			}
			if spanKind&2 != 0 {
				style = style.Reverse(true).Underline(true)
			}
			content.WriteString(style.Render(span.String()))
			span.Reset()
		}
		flush := func(last int) {
			flushSpan()
			prefix := fmt.Sprintf("%4d │ ", line.number)
			if first > 0 {
				prefix = "   ↪ │ "
			}
			if n == m.reader.cursor.line && m.reader.cursor.column >= first && m.reader.cursor.column <= last {
				cursorRow = len(rows)
			}
			rows = append(rows, inspectorRow{m.paint(prefix, m.theme.Muted) + content.String(), textPoint{n, first}})
			content.Reset()
			first, cell = last+1, 0
		}
		for c, character := range line.characters {
			text := visibleCharacter(character, cell)
			w := lipgloss.Width(text)
			if cell+w > available && cell > 0 {
				flush(c - 1)
				text = visibleCharacter(character, 0)
				w = lipgloss.Width(text)
			}
			kind := 0
			if m.textSelected(n, c) {
				kind |= 1
			}
			if m.focus == "inspector" && n == m.reader.cursor.line && c == m.reader.cursor.column {
				kind |= 2
			}
			if kind != spanKind {
				flushSpan()
				spanKind = kind
			}
			span.WriteString(text)
			cell += w
		}
		if len(line.characters) == 0 {
			text := " "
			if m.focus == "inspector" && n == m.reader.cursor.line {
				text = lipgloss.NewStyle().Reverse(true).Render(text)
			}
			content.WriteString(text)
		}
		flush(max(0, len(line.characters)-1))
	}
	return rows, cursorRow
}

func lessPoint(a, b textPoint) bool {
	return a.line < b.line || a.line == b.line && a.column < b.column
}

func (m Model) selection() (textPoint, textPoint) {
	a, b := m.reader.anchor, m.reader.cursor
	if lessPoint(b, a) {
		a, b = b, a
	}
	return a, b
}

func (m Model) textSelected(line, column int) bool {
	if m.reader.mode == "" {
		return false
	}
	a, b := m.selection()
	if m.reader.mode == "line" {
		return line >= a.line && line <= b.line
	}
	p := textPoint{line, column}
	return !lessPoint(p, a) && !lessPoint(b, p)
}

func (m *Model) keepCursorVisible() {
	if len(m.reader.lines) == 0 {
		return
	}
	m.reader.cursor.line = min(max(0, m.reader.cursor.line), len(m.reader.lines)-1)
	m.reader.cursor.column = min(max(0, m.reader.cursor.column), max(0, len(m.reader.lines[m.reader.cursor.line].characters)-1))
	rows, cursor := m.inspectorRows(m.inspectorWidth())
	capacity := m.inspectorCapacity()
	if cursor < m.reader.top {
		m.reader.top = cursor
	} else if cursor >= m.reader.top+capacity {
		m.reader.top = cursor - capacity + 1
	}
	m.reader.top = min(max(0, m.reader.top), max(0, len(rows)-capacity))
}

func (m Model) readerMode() string {
	switch m.reader.mode {
	case "char":
		return "VISUAL CHAR"
	case "line":
		return "VISUAL LINE"
	default:
		return "NORMAL"
	}
}

func (m Model) readerView(width, height int) []string {
	msg := m.inspected()
	if len(m.reader.lines) == 0 {
		return m.panel("INSPECTOR", width, height, []string{m.paint("  No message selected", m.theme.Muted)}, false)
	}
	if m.reader.help {
		rows := []string{"  READ-ONLY / COPY HELP", "  h/j/k/l or arrows: cursor · w/b/e: words", "  0/$ Home/End: line · gg/G: document", "  PgUp/PgDn: page · Ctrl+U/D: half page", "  yy: whole logical line · v: characters", "  V: whole lines · y: copy selection", "  Y: full body · Ctrl+P: copy report", "  Esc: clear selection, then Back to list", "  Tab: text / Back / Resend / other panes", "  Ctrl+R: configure resend (DLQ only)", "  b is word-back here, not sidebar", "  Clipboard: local tools or OSC52 request.", "  Remote requests may need terminal permission.", "  Failure: use terminal selection/copy; this", "  fallback may include gutters/soft wraps.", "  q / Ctrl+C quits · Esc closes help"}
		for i := range rows {
			rows[i] = m.paint(rows[i], m.theme.Foreground)
		}
		top := min(m.reader.helpTop, max(0, len(rows)-max(1, height-3)))
		rows = append(rows[top:min(len(rows), top+max(1, height-3))], m.paint("  j/k scroll · Esc close", m.theme.Focus))
		return m.panel("INSPECTOR / HELP", width, height, rows, true)
	}
	if m.reader.load != "" {
		rows := []string{"", "  Load full message for copy?", "  Preview is truncated; clipboard unchanged.", "  Read-only exact lookup; no receive or lock.", "  Full byte size is shown after loading.", "", "  Tab choose · Enter select · Esc cancel"}
		if m.reader.loading != "" {
			rows = []string{"", "  Loading exact full message…", "  Read-only; no lock or send.", "  Esc cancels; clipboard unchanged."}
		} else {
			rows = append(rows, "", "  "+m.buttonLabel("Cancel", m.reader.choice == 0)+"  "+m.buttonLabel("Load full message", m.reader.choice == 1))
		}
		return m.panel("INSPECTOR / FULL CONTENT", width, height, rows, true)
	}
	header := []string{
		m.paint(fmt.Sprintf("  ID: %s · Seq %d · %d/%d", clean(msg.ID, false), msg.Sequence, m.message+1, len(m.page.Messages)), m.theme.Foreground),
	}
	if m.dlq {
		header = append(header, m.paint("  "+clean(msg.DeadLetterReason, false), m.theme.Warning), m.paint("  "+clean(msg.DeadLetterDescription, false), m.theme.Muted))
	} else {
		header = append(header, m.paint("  "+clean(msg.Subject, false), m.theme.Focus), m.paint("  PEEK ONLY · no locks or settlement", m.theme.Muted))
	}
	rows, _ := m.inspectorRows(width)
	capacity := max(1, height-len(header)-7)
	top := min(m.reader.top, max(0, len(rows)-capacity))
	lines := append(header, "")
	for i := top; i < min(len(rows), top+capacity); i++ {
		lines = append(lines, rows[i].text)
	}
	for len(lines) < height-6 {
		lines = append(lines, "")
	}
	p := m.reader.cursor
	line := m.reader.lines[p.line]
	position := fmt.Sprintf("  %s · %s %d:%d · rows %d–%d/%d", m.readerMode(), line.section, line.number, p.column+1, top+1, min(len(rows), top+capacity), len(rows))
	if m.reader.mode != "" {
		a, b := m.selection()
		if m.reader.mode == "line" {
			position = fmt.Sprintf("  VISUAL LINE · %d lines selected", b.line-a.line+1)
		} else {
			position = fmt.Sprintf("  VISUAL CHAR · %d characters selected", uniseg.GraphemeClusterCount(m.selectedText()))
		}
	}
	if m.reader.pending != "" {
		position = "  " + m.reader.pending + "… press " + m.reader.pending + " again · Esc cancel"
	}
	lines = append(lines, m.paint(position, m.theme.Focus))
	back := m.buttonLabel("Esc Back", m.focus == "inspector-back")
	resend := ""
	if m.dlq {
		resend = "  " + m.buttonLabel("^R Resend to queue…", m.focus == "inspector-resend")
		if m.resource.Queue == "" {
			resend = "  " + m.buttonLabel("^R Send copy to…", m.focus == "inspector-resend")
		}
	}
	if m.reader.mode != "" {
		lines = append(lines, m.paint("  y Copy selection · Esc clear · ? help", m.theme.Focus), m.paint("  Resend blocked until selection cleared", m.theme.Muted))
	} else {
		lines = append(lines, m.paint("  "+back+resend, m.theme.Focus), m.paint("  yy line · v/V select · Y full body · ? help", m.theme.Muted))
	}
	return m.panel("INSPECTOR / "+m.readerMode(), width, height, lines, m.inspectorFocused())
}

func (m Model) buttonLabel(label string, focused bool) string {
	if focused {
		return "[› " + label + "]"
	}
	return "[ " + label + " ]"
}

func textBody(msg explorer.Message) bool {
	if !utf8.Valid(msg.Body) {
		return false
	}
	for _, r := range string(msg.Body) {
		if r == 0 {
			return false
		}
	}
	t := strings.ToLower(msg.ContentType)
	return t == "" || strings.HasPrefix(t, "text/") || strings.Contains(t, "json") || strings.Contains(t, "xml") || strings.Contains(t, "javascript") || strings.Contains(t, "yaml")
}

func (m Model) selectedText() string {
	if len(m.reader.lines) == 0 {
		return ""
	}
	a, b := m.selection()
	if m.reader.mode == "" {
		a, b = m.reader.cursor, m.reader.cursor
	}
	var result strings.Builder
	for n := a.line; n <= b.line; n++ {
		line := m.reader.lines[n]
		first, last := 0, len(line.characters)
		if m.reader.mode == "char" {
			if n == a.line {
				first = a.column
			}
			if n == b.line {
				last = min(last, b.column+1)
			}
		}
		result.WriteString(strings.Join(line.characters[first:last], ""))
		if m.reader.mode != "char" || n < b.line {
			result.WriteString(line.ending)
			if n < b.line && line.ending == "" {
				result.WriteString("\n") // Explicit separator between document sections.
			}
		}
	}
	return result.String()
}

func (m Model) copyText(kind string) (tea.Model, tea.Cmd) {
	if len(m.reader.lines) == 0 || m.reader.copying {
		return m, nil
	}
	msg := m.inspected()
	if !textBody(msg) {
		m.status = "Text copy unavailable for binary/non-text body; clipboard unchanged"
		return m, nil
	}
	if (kind == "body" || kind == "report") && msg.PreviewTruncated {
		m.reader.load, m.reader.choice = kind, 0
		return m, nil
	}
	var text, scope string
	switch kind {
	case "body":
		text, scope = string(msg.Body), fmt.Sprintf("full body · %d bytes", len(msg.Body))
	case "report":
		props, err := json.MarshalIndent(msg.Properties, "", "  ")
		if err != nil {
			m.status = "Cannot copy message report: properties cannot be represented as JSON"
			return m, nil
		}
		text = fmt.Sprintf("MESSAGE\nID: %s\nSource: %s / %s / %s\nSequence: %d\nSubject: %s\nContent-Type: %s\nDeliveries: %d\nEnqueued: %s\nReason: %s\nDescription: %s\n\nBODY\n%s\n\nAPPLICATION PROPERTIES\n%s", msg.ID, m.resource.Profile, m.resource.Namespace, m.resource.Name(), msg.Sequence, msg.Subject, msg.ContentType, msg.DeliveryCount, msg.EnqueuedAt.UTC().Format(time.RFC3339), msg.DeadLetterReason, msg.DeadLetterDescription, msg.Body, props)
		scope = fmt.Sprintf("message report · %d bytes", len(text))
	default:
		if kind == "line" {
			line := m.reader.lines[m.reader.cursor.line]
			text = strings.Join(line.characters, "") + line.ending
		} else {
			text = m.selectedText()
		}
		scope = fmt.Sprintf("%d characters · %s", uniseg.GraphemeClusterCount(text), m.reader.lines[m.reader.cursor.line].section)
		if kind == "line" || m.reader.mode == "line" {
			a, b := m.selection()
			count := b.line - a.line + 1
			if kind == "line" {
				count = 1
			}
			scope = fmt.Sprintf("%d lines · %s", count, m.reader.lines[m.reader.cursor.line].section)
		}
		if msg.PreviewTruncated {
			scope += " · preview fragment"
		}
	}
	if text == "" {
		m.status = "Body is empty; clipboard unchanged"
		return m, nil
	}
	m.reader.copying = true
	m.status = "Copying " + scope + "…"
	write, epoch := m.clipboard, m.reader.epoch
	return m, func() tea.Msg { confirmed, err := write(text); return copied{epoch, confirmed, scope, err} }
}

func (m Model) inspectorMessage(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case copied:
		if msg.epoch != m.reader.epoch {
			return m, nil, true
		}
		m.reader.copying = false
		if msg.err != nil {
			m.status = "Clipboard unavailable; selection kept. " + msg.err.Error() + " · ? help"
		} else {
			m.status = "Copied " + msg.scope
			if !msg.confirmed {
				m.status = "Copy requested · " + msg.scope + " · terminal confirmation unavailable"
			}
			m.reader.mode, m.reader.pending = "", ""
		}
		return m, nil, true
	case fullMessage:
		if msg.epoch != m.reader.epoch || m.reader.loading == "" {
			return m, nil, true
		}
		if m.reader.cancel != nil {
			m.reader.cancel()
		}
		selected := m.inspected()
		if msg.err != nil || msg.message.Sequence != selected.Sequence || msg.message.ID != selected.ID || msg.message.PreviewTruncated {
			m.reader.load, m.reader.loading = "", ""
			m.status = "Full message unavailable; clipboard unchanged; no replacement selected"
			return m, nil, true
		}
		m.page.Messages[m.message] = msg.message
		m.buildDocument()
		m.reader.load, m.reader.loading = "", ""
		m.keepCursorVisible()
		m.status = fmt.Sprintf("Full message loaded · %d bytes · Y copies full body; Ctrl+P copies report", len(msg.message.Body))
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) inspectorKey(key string) (tea.Model, tea.Cmd) {
	if key == "?" && m.focus == "inspector" {
		m.reader.help = !m.reader.help
		m.reader.helpTop = 0
		return m, nil
	}
	if m.reader.help {
		switch key {
		case "esc":
			m.reader.help = false
		case "j", "down", "pgdown":
			m.reader.helpTop++
		case "k", "up", "pgup":
			m.reader.helpTop = max(0, m.reader.helpTop-1)
		}
		return m, nil
	}
	if m.reader.load != "" {
		if key == "esc" {
			if m.reader.cancel != nil {
				m.reader.cancel()
			}
			m.reader.load, m.reader.loading = "", ""
			m.reader.epoch++
			m.status = "Full-content load cancelled; clipboard unchanged"
		} else if m.reader.loading == "" {
			switch key {
			case "tab", "shift+tab", "left", "right":
				m.reader.choice = 1 - m.reader.choice
			case "enter":
				if m.reader.choice == 0 {
					m.reader.load = ""
					return m, nil
				}
				m.reader.loading = m.reader.load
				ctx, cancel := context.WithTimeout(m.life, 10*time.Second)
				m.reader.cancel = cancel
				epoch, selected, store, resource, dlq := m.reader.epoch, m.inspected(), m.store, m.resource, m.dlq
				return m, func() tea.Msg {
					defer cancel()
					message, err := store.PeekMessage(ctx, resource, dlq, selected.Sequence)
					return fullMessage{epoch, message, err}
				}
			}
		}
		return m, nil
	}
	if key == "esc" {
		if m.reader.mode != "" || m.reader.pending != "" {
			m.reader.mode, m.reader.pending = "", ""
		} else {
			m.focus = "content"
			m.reader.revealed = false
		}
		return m, nil
	}
	if m.reader.copying {
		return m, nil
	}
	if key == "ctrl+r" {
		if m.reader.mode != "" {
			m.status = "Esc clears selection before resend"
			return m, nil
		}
		return m.openSelected()
	}
	if m.focus != "inspector" {
		if key == "enter" {
			if m.focus == "inspector-back" {
				m.focus = "content"
				m.reader.revealed = false
				return m, nil
			}
			return m.openSelected()
		}
		return m, nil
	}
	if len(m.reader.lines) == 0 {
		return m, nil
	}
	if key == "Y" {
		return m.copyText("body")
	}
	if key == "y" {
		if m.reader.mode != "" {
			return m.copyText("selection")
		}
		if m.reader.pending == "y" {
			m.reader.pending = ""
			return m.copyText("line")
		}
		m.reader.pending = "y"
		return m, nil
	}
	if key == "g" && m.reader.pending != "g" {
		m.reader.pending = "g"
		return m, nil
	}
	previous := m.reader.pending
	m.reader.pending = ""
	if key == "v" || key == "V" {
		mode := "char"
		if key == "V" {
			mode = "line"
		}
		if m.reader.mode == mode {
			m.reader.mode = ""
		} else {
			if m.reader.mode == "" {
				m.reader.anchor = m.reader.cursor
			}
			m.reader.mode = mode
		}
		return m, nil
	}
	p := &m.reader.cursor
	vertical := false
	switch key {
	case "h", "left":
		p.column = max(0, p.column-1)
	case "l", "right":
		p.column++
	case "j", "down":
		p.line++
		vertical = true
	case "k", "up":
		p.line--
		vertical = true
	case "0", "home":
		p.column = 0
	case "$", "end":
		p.column = len(m.reader.lines[p.line].characters) - 1
	case "g":
		if previous == "g" {
			p.line, p.column = 0, 0
		}
	case "G":
		p.line, p.column = len(m.reader.lines)-1, 0
	case "pgdown", "ctrl+d", "pgup", "ctrl+u":
		step := m.inspectorCapacity()
		if key == "ctrl+u" || key == "ctrl+d" {
			step = max(1, step/2)
		}
		direction := 1
		if key == "pgup" || key == "ctrl+u" {
			direction = -1
		}
		rows, current := m.inspectorRows(m.inspectorWidth())
		target := min(max(0, current+direction*step), len(rows)-1)
		for rows[target].point.line < 0 {
			next := target + direction
			if next < 0 || next >= len(rows) {
				target -= direction
				break
			}
			target = next
		}
		if rows[target].point.line >= 0 {
			offset := p.column - rows[current].point.column
			*p = rows[target].point
			p.column += offset
		}
	case "w", "b", "e":
		m.moveWord(key)
	case "enter":
	default:
		m.status = "Read-only inspector · yy line · v/V select · Y full body · ? help"
	}
	if vertical {
		p.column = m.reader.preferred
	}
	m.keepCursorVisible()
	if !vertical {
		m.reader.preferred = p.column
	}
	return m, nil
}

func (m *Model) moveWord(key string) {
	p := &m.reader.cursor
	step := 1
	if key == "b" {
		step = -1
	}
	class := func(p textPoint) int {
		chars := m.reader.lines[p.line].characters
		if len(chars) == 0 {
			return 0
		}
		r, _ := utf8.DecodeRuneInString(chars[p.column])
		if unicode.IsSpace(r) {
			return 0
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return 1
		}
		return 2
	}
	next := func(p textPoint) (textPoint, bool) {
		p.column += step
		if p.column < 0 {
			p.line--
			if p.line < 0 {
				return textPoint{}, false
			}
			p.column = max(0, len(m.reader.lines[p.line].characters)-1)
		}
		if p.column >= max(1, len(m.reader.lines[p.line].characters)) {
			p.line++
			if p.line >= len(m.reader.lines) {
				return textPoint{}, false
			}
			p.column = 0
		}
		return p, true
	}
	if key == "e" {
		// Always advance first: e at a word end finds the next word end.
		n, ok := next(*p)
		if !ok {
			return
		}
		*p = n
		for class(*p) == 0 {
			n, ok = next(*p)
			if !ok {
				return
			}
			*p = n
		}
		for {
			n, ok = next(*p)
			if !ok || n.line != p.line || class(n) != class(*p) {
				return
			}
			*p = n
		}
	}
	old := class(*p)
	for {
		n, ok := next(*p)
		if !ok {
			return
		}
		c := class(n)
		*p = n
		if key == "w" && c != 0 && (c != old || n.column == 0) {
			return
		}
		if key == "b" && c != 0 {
			before, ok := next(n)
			if !ok || n.column == 0 || class(before) != c {
				return
			}
		}
		old = c
	}
}

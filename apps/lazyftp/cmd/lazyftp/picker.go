package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/connections"
	"lazyftp/internal/theme"
	"terminalui"
)

// The connection picker runs over the retained file workspace. Saving always
// requires its own explicit action and never starts a connection.
type picker struct {
	choices                      []connections.Bookmark
	cursor, focus, width, height int
	root, local, name, message   string
	bookmarkPath                 string
	leftOverride                 string
	startLocal                   string
	destination                  bool
	pane                         int
	connectError                 string
	newConnection                bool
	localFiles                   bool
	frame                        int
	scroll                       int
	loading, navigated           bool
	details                      bool
	help                         bool
	helpOffset                   int
	dismissed                    bool
	theme                        theme.Theme
	selected                     *connections.Bookmark
}

func newPicker(remotes []string, bookmarks []connections.Bookmark, local, leftOverride, path string, t theme.Theme) picker {
	m := picker{choices: append([]connections.Bookmark(nil), bookmarks...), local: local, startLocal: local, root: "/", bookmarkPath: path, leftOverride: leftOverride, theme: t, width: 80, height: 24}
	for _, remote := range remotes {
		m.choices = append(m.choices, connections.Bookmark{Remote: remote, Root: "/", Local: local})
	}
	if len(m.choices) == 0 {
		m.focus = 6
	}
	m.choose()
	return m
}

func (m *picker) choose() {
	if len(m.choices) == 0 {
		return
	}
	b := m.choices[m.cursor]
	m.root, m.local, m.name = b.Root, b.Local, b.Name
	if m.local == "" {
		m.local = m.startLocal
	}
	if m.leftOverride != "" {
		m.local = m.leftOverride
	}
}

func (m picker) Init() tea.Cmd { return nil }

func (m picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.help {
		r := terminalui.HandleHelp(msg, m.surfaceSpec().Actions, m.helpTopics(), &m.helpOffset, m.helpFrame())
		if r.Intent != nil && r.Intent.Kind == terminalui.Dismiss {
			m.help = false
		}
		if r.Consumed {
			return m, nil
		}
	}
	if k, ok := msg.(tea.KeyMsg); ok && (k.String() == "f1" || (k.String() == "?" && (m.focus == 0 || m.focus >= 4))) {
		m.help = true
		return m, nil
	}
	spec, frame := m.surface()
	if r := m.handleSurface(msg, spec, frame); r.Consumed {
		if r.Intent != nil {
			switch r.Intent.Kind {
			case terminalui.Dismiss:
				return m.executePicker("back")
			case terminalui.Help:
				m.help = true
			case terminalui.Activate:
				return m.executePicker(r.Intent.ID)
			case terminalui.Focus:
				m.setFocus(r.Intent.ID)
				if !m.details {
					m.navigated = true
				}
			case terminalui.Scroll:
				if !m.details && m.width >= 60 && m.height >= 21 {
					m.move(r.Intent.Delta)
				}
			case terminalui.Select:
				for i := range m.choices {
					if m.choiceID(i) == r.Intent.ID {
						m.cursor, m.focus = i, 0
						m.choose()
						m.navigated = true
					}
				}
			}
		}
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll = m.offset()
	case tea.KeyMsg:
		if m.width < 60 || m.height < 21 {
			return m, nil
		}
		key := msg.String()
		if !m.details {
			switch key {
			case "j":
				key = "down"
			case "k":
				key = "up"
			}
		}
		switch key {
		case "up":
			if !m.details {
				m.move(-1)
			}
		case "down":
			if !m.details {
				m.move(1)
			}
		case "enter":
			if m.width < 60 || m.height < 21 {
				return m, nil
			}
			if m.focus == 7 {
				return m.executePicker("local-files")
			}
			if m.focus == 6 || (m.focus == 0 && len(m.choices) == 0) {
				return m.executePicker("new")
			}
			if m.focus == 0 {
				return m.executePicker("open")
			} else if m.focus < 4 {
				m.focus++
				if m.destination && m.focus == 2 {
					m.focus = 3
				}
			} else {
				return m.executePicker(m.focusID())
			}
		default:
			var field *string
			switch m.focus {
			case 1:
				field = &m.root
			case 2:
				field = &m.local
			case 3:
				field = &m.name
			}
			if field != nil {
				runes := []rune(*field)
				if msg.String() == "ctrl+u" {
					*field = ""
				} else if msg.String() == "backspace" && len(runes) > 0 {
					*field = string(runes[:len(runes)-1])
				} else if msg.Type == tea.KeySpace {
					*field += " "
				} else if msg.Type == tea.KeyRunes {
					for _, r := range msg.Runes {
						if !unicode.IsControl(r) {
							*field += string(r)
						}
					}
				}
			}
		}
	}
	return m, nil
}

func (m *picker) move(delta int) {
	m.navigated = true
	position := m.cursor
	if m.focus == 6 || m.focus == 7 {
		position = len(m.choices) + m.focus - 6
	}
	position = max(0, min(len(m.choices)+1, position+delta))
	if position < len(m.choices) {
		m.cursor, m.focus = position, 0
		m.choose()
	} else {
		m.focus = 6 + position - len(m.choices)
	}
	m.scroll = m.offset()
}

func (m picker) visibleChoices() int {
	return min(4, max(1, m.height-18))
}

func (m picker) offset() int {
	offset := min(m.scroll, max(0, len(m.choices)-m.visibleChoices()))
	if m.focus == 0 {
		offset = max(0, min(m.cursor, max(offset, m.cursor-m.visibleChoices()+1)))
	}
	return offset
}

func (m picker) activate(id string) (tea.Model, tea.Cmd) {
	if m.connectError != "" && id != "save" {
		m.message = m.connectError
		return m, nil
	}
	if len(m.choices) == 0 {
		m.message = "No remotes. Choose New connection to create one."
		return m, nil
	}
	loc, err := connections.ParseRemote(m.choices[m.cursor].Remote + ":" + m.root)
	if err != nil {
		m.message = err.Error()
		return m, nil
	}
	local := m.choices[m.cursor].Local
	if !m.destination {
		local, err = filepath.Abs(m.local)
		if strings.TrimSpace(m.local) == "" {
			err = fmt.Errorf("choose a local folder")
		}
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(local)
			if err == nil && !info.IsDir() {
				err = fmt.Errorf("not a directory")
			}
		}
		if err != nil {
			m.message = "Local folder must be an existing directory"
			return m, nil
		}
	} else if m.choices[m.cursor].Name == "" {
		local = ""
	}
	b := connections.Bookmark{Name: m.name, Remote: loc.Remote, Root: loc.Path, Local: local}
	if id == "save" {
		if err := connections.Save(m.bookmarkPath, b); err != nil {
			m.message = "Save failed: " + err.Error()
			return m, nil
		}
		m.message = "Bookmark saved (same name replaces). Choose Connect when ready."
		return m, nil
	}
	m.selected = &b
	return m, tea.Quit
}

func (m picker) focusID() string {
	return map[int]string{0: "choices", 1: "root", 2: "local", 3: "name", 4: "connect", 5: "save", 6: "new", 7: "local-files", 8: "back", 9: "open", 10: "help"}[m.focus]
}

func (m *picker) setFocus(id string) {
	for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		old := m.focus
		m.focus = n
		if m.focusID() == id {
			return
		}
		m.focus = old
	}
}

func (m picker) choiceID(i int) string {
	b := m.choices[i]
	return "connection:" + b.Remote + ":" + b.Name + ":" + b.Root + ":" + b.Local
}

func (m picker) executePicker(id string) (tea.Model, tea.Cmd) {
	if id == "back" {
		if m.details {
			m.details, m.focus, m.message = false, 0, ""
		} else {
			m.dismissed = true
		}
		return m, nil
	}
	if m.width < 60 || m.height < 21 {
		return m, nil
	}
	visible := false
	for _, r := range m.surfaceFrame().Regions {
		if r.ID == id {
			visible = true
		}
	}
	if !visible {
		return m, nil
	}
	switch id {
	case "open":
		if len(m.choices) > 0 {
			m.details, m.focus = true, 1
		}
	case "new":
		m.newConnection = true
	case "local-files":
		m.localFiles = true
	case "help":
		m.help = true
	case "connect", "save":
		if m.details {
			return m.activate(id)
		}
	}
	return m, nil
}

func (m picker) surface() (terminalui.SurfaceSpec, terminalui.Frame) {
	// Help is a view over this same draft, not another picker layout.
	m.help = false
	l := m.layout()
	h := min(m.height-4, len(l.rows)+3)
	bounds := terminalui.Rect{X: (m.width - l.width) / 2, Y: max(2, (m.height-h)/2), W: l.width, H: h}
	b := terminalui.SurfaceBodyBounds(bounds)
	s := terminalui.SurfaceSpec{ID: "connections", Title: "Connections", CancelID: "back", FocusID: m.focusID(), Body: terminalui.Frame{Bounds: b}}
	if m.width < 60 || m.height < 21 {
		return s, terminalui.Frame{}
	}
	for y, row := range l.rows {
		if y >= b.H {
			break
		}
		text := safeConnectionText(row.text)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Foreground)).Background(lipgloss.Color(m.theme.Background))
		if row.focus == m.focus && (row.choice < 0 || row.choice == m.cursor) {
			style = style.Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection))
		}
		s.Body.Lines = append(s.Body.Lines, style.Render(terminalui.Fit(text, b.W)))
		id, kind := "", terminalui.FieldRegion
		if row.choice >= 0 {
			id, kind = m.choiceID(row.choice), terminalui.RowRegion
		} else if row.focus > 0 {
			n := m
			n.focus = row.focus
			id = n.focusID()
			if row.focus >= 4 {
				kind = terminalui.ControlRegion
			}
		}
		if id != "" {
			s.Body.Regions = append(s.Body.Regions, terminalui.Region{ID: id, Kind: kind, Bounds: terminalui.Rect{X: b.X, Y: b.Y + y, W: b.W, H: 1}})
		}
	}
	if m.details {
		label := "Connect"
		if m.destination {
			label = "Open in " + strings.ToLower(paneHeading(m.pane)) + " pane"
		}
		s.Actions = []terminalui.Action{{ID: "back", Label: "Back"}, {ID: "connect", Label: label}, {ID: "save", Label: "Save bookmark"}}
		s.FocusOrder = []string{"root"}
		if !m.destination {
			s.FocusOrder = append(s.FocusOrder, "local")
		}
		s.FocusOrder = append(s.FocusOrder, "name", "connect", "save", "back", "help")
	} else {
		s.Actions = []terminalui.Action{{ID: "back", Label: "Close"}, {ID: "open", Label: "Open"}}
		s.FocusOrder = []string{"open", "new", "local-files", "back", "help"}
		if len(m.choices) == 0 {
			s.Actions[1].Unavailable = "No connection selected"
		}
	}
	s.Actions = append(s.Actions, terminalui.Action{ID: "help", Label: "Help"})
	return s, terminalui.RenderSurface(s, bounds, m.theme.TerminalStyle(false))
}

func (m picker) surfaceSpec() terminalui.SurfaceSpec {
	spec, _ := m.surface()
	return spec
}

func (m picker) surfaceFrame() terminalui.Frame {
	_, frame := m.surface()
	return frame
}

func (m picker) handleSurface(msg tea.Msg, s terminalui.SurfaceSpec, f terminalui.Frame) terminalui.Result {
	switch k := msg.(type) {
	case tea.MouseMsg:
	case tea.KeyMsg:
		if k.String() != "esc" && k.String() != "tab" && k.String() != "shift+tab" && !(k.String() == "enter" && m.focus >= 4) {
			return terminalui.Result{}
		}
	default:
		return terminalui.Result{}
	}
	return terminalui.HandleSurface(msg, s, f)
}

func safeConnectionText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func (m picker) helpTopics() []terminalui.HelpTopic {
	return []terminalui.HelpTopic{{Group: "Connection help", Keys: "↑↓ / j k", Description: "Select a connection; row clicks select only. Enter / Open opens details."}, {Group: "Connection help", Keys: "Tab / Ctrl+U", Description: "Move fields / clear field. Save bookmark is explicit; it does not connect. Credentials stay in rclone."}, {Group: "Connection help", Keys: "Status", Description: safeConnectionText(m.message)}}
}

func (m picker) helpFrame() terminalui.Frame {
	return terminalui.RenderHelp(m.surfaceSpec().Actions, m.helpTopics(), m.helpOffset, terminalui.Rect{X: 2, Y: 2, W: max(0, m.width-4), H: max(0, m.height-4)}, m.theme.TerminalStyle(false))
}

// Rendering and pointer hit testing use the same card rows and centered origin.
type pickerRow struct {
	text          string
	focus, choice int
}

type pickerLayout struct {
	width int
	rows  []pickerRow
}

func (m picker) layout() pickerLayout {
	l := pickerLayout{width: min(78, m.width-4)}
	if !m.details && !m.help {
		l.width = min(64, m.width-8)
	}
	spacious := m.height >= 32
	add := func(s string, focus, choice int) {
		l.rows = append(l.rows, pickerRow{text: s, focus: focus, choice: choice})
	}
	center := func(s string) { add(lipgloss.PlaceHorizontal(l.width-6, lipgloss.Center, s), -1, -1) }
	divider := func() { add(strings.Repeat("─", l.width-6), -1, -1) }
	add("", -1, -1)
	if m.details {
		heading := "Connection details"
		if m.destination {
			heading += " · " + paneHeading(m.pane) + " pane"
		}
		add(heading, -1, -1)
		add("", -1, -1)
		center("▤  " + m.choices[m.cursor].Remote + ":")
		add("", -1, -1)
		add("Remote root    "+m.root, 1, -1)
		if spacious {
			add("", -1, -1)
		}
		if !m.destination {
			add("Local folder   "+m.local, 2, -1)
			if spacious {
				add("", -1, -1)
			}
		}
		add("Bookmark name  "+m.name, 3, -1)
		add("", -1, -1)
		divider()
		add("Saves folders and name only. Does not connect.", -1, -1)
	} else {
		// A fixed 4×3 square logo; only its accent tile changes, never its size.
		for row := 0; row < 3; row++ {
			var tiles []string
			for col := 0; col < 4; col++ {
				tile := "□"
				if row*4+col == (m.frame/2)%12 {
					tile = "■"
				}
				tiles = append(tiles, tile)
			}
			label := []string{"Open a connection", "Choose where to work", ""}[row]
			if m.destination && row == 0 {
				label = "Choose connection · " + paneHeading(m.pane) + " pane"
			}
			add("  "+strings.Join(tiles, " ")+"   "+label, -1, -1)
		}
		add("", -1, -1)
		heading := "  Saved & configured"
		if len(m.choices) > m.visibleChoices() {
			heading += fmt.Sprintf("  %d–%d of %d", m.offset()+1, min(m.offset()+m.visibleChoices(), len(m.choices)), len(m.choices))
		}
		add(heading, -1, -1)
		if len(m.choices) == 0 {
			switch {
			case m.loading:
				add("  Loading connections…", -1, -1)
				add("  You can create a connection or browse locally.", -1, -1)
			case m.message != "":
				add("  Could not load connections.", -1, -1)
				add("  F1 for details. Local files remain available.", -1, -1)
			default:
				add("  No connections yet", -1, -1)
				add("  Create one, or browse files on this computer.", -1, -1)
			}
		} else {
			for i := m.offset(); i < min(m.offset()+m.visibleChoices(), len(m.choices)); i++ {
				b := m.choices[i]
				s := b.Remote + ":"
				if b.Name != "" {
					s = b.Name
				}
				prefix := "     "
				if m.focus == 0 && i == m.cursor {
					prefix = " ›   "
				}
				if m.width >= 80 && b.Name != "" {
					s = ansi.Truncate(s, 22, "…")
					s += strings.Repeat(" ", 22-ansi.StringWidth(s)) + " " + b.Remote + ":" + b.Root
				}
				add(prefix+s, 0, i)
			}
			if m.message != "" {
				add("  Could not load all connections. F1 for details.", -1, -1)
			}
		}
		add("  "+strings.Repeat("─", l.width-6), -1, -1)
		for _, a := range []struct {
			label string
			focus int
		}{{"+ New connection", 6}, {"  Browse local files", 7}} {
			prefix := "   "
			if m.focus == a.focus {
				prefix = " › "
			}
			add(prefix+a.label, a.focus, -1)
		}
		add("", -1, -1)
	}
	if m.message != "" && m.details && !m.help {
		add(m.message, -1, -1)
	}
	if spacious && (m.details || m.help) {
		add("", -1, -1)
	}
	for len(l.rows) > max(0, m.height-7) {
		removed := false
		for i := len(l.rows) - 1; i >= 0; i-- {
			if l.rows[i].text == "" {
				l.rows = append(l.rows[:i], l.rows[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			break
		}
	}
	return l
}

func (m picker) View() string {
	if m.width < 60 || m.height < 21 {
		return "Resize to at least 60 columns × 21 rows. Esc cancels.\n"
	}
	base := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Foreground)).Background(lipgloss.Color(m.theme.Background))
	muted := base.Foreground(lipgloss.Color(m.theme.Muted))
	accent := base.Foreground(lipgloss.Color(m.theme.Focus)).Bold(true)
	rows := make([]string, m.height)
	for i := range rows {
		rows[i] = base.Width(m.width).Render("")
	}
	rows[0] = accent.Width(m.width).Render("  LAZYFTP")
	rows[1] = muted.Render(strings.Repeat("─", m.width))
	footer := "↑↓ / j k move   Enter open   Esc local files   F1 help"
	if m.width < 80 {
		footer = "↑↓ move  Enter open  Esc local files  F1 help"
	}
	if m.destination {
		footer = strings.Replace(footer, "Esc local files", "Esc back", 1)
	}
	if m.details {
		footer = "Tab focus · Enter select · Esc connections · F1 help"
	}
	rows[m.height-1] = muted.Width(m.width).Render(" " + ansi.Truncate(footer, m.width-2, "…"))
	rows[m.height-2] = muted.Render(strings.Repeat("─", m.width))
	frame := m.surfaceFrame()
	if m.help {
		frame = m.helpFrame()
	}
	return strings.Join(terminalui.Overlay(terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}, Lines: rows}, frame, m.theme.TerminalStyle(false)).Lines, "\n")
}

func paneHeading(pane int) string {
	if pane == 0 {
		return "LEFT"
	}
	return "RIGHT"
}

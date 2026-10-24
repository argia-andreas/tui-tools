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
)

// The connection picker runs over the retained file workspace. Saving always
// requires its own explicit action and never starts a connection.
type picker struct {
	choices                      []connections.Bookmark
	cursor, focus, width, height int
	root, local, name, message   string
	bookmarkPath                 string
	leftOverride                 string
	connectError                 string
	newConnection                bool
	localFiles                   bool
	frame                        int
	scroll                       int
	loading, navigated           bool
	details                      bool
	help                         bool
	theme                        theme.Theme
	selected                     *connections.Bookmark
}

func newPicker(remotes []string, bookmarks []connections.Bookmark, local, leftOverride, path string, t theme.Theme) picker {
	m := picker{choices: append([]connections.Bookmark(nil), bookmarks...), local: local, root: "/", bookmarkPath: path, leftOverride: leftOverride, theme: t, width: 80, height: 24}
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
	if m.leftOverride != "" {
		m.local = m.leftOverride
	}
}

func (m picker) Init() tea.Cmd { return nil }

func (m picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll = m.offset()
	case tea.MouseMsg:
		if m.help || m.width < 60 || m.height < 21 {
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			if m.details {
				return m, nil
			}
			delta := 1
			if msg.Button == tea.MouseButtonWheelUp {
				delta = -1
			}
			m.move(delta)
		} else if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			layout := m.layout()
			y := msg.Y - layout.y - 1
			if msg.X > layout.x && msg.X < layout.x+layout.width-1 && y >= 0 && y < len(layout.rows) {
				row := layout.rows[y]
				if row.choice >= 0 {
					m.cursor = row.choice
					m.choose()
					m.navigated = true
					m.details, m.focus = true, 1
				} else if row.focus > 0 {
					m.focus = row.focus
					if m.focus == 6 {
						m.newConnection = true
					} else if m.focus == 7 {
						m.localFiles = true
					} else if m.focus == 8 {
						m.details, m.focus, m.message = false, 0, ""
					} else if m.focus >= 4 {
						return m.activate()
					}
				}
			}
		}
	case tea.KeyMsg:
		if msg.String() == "f1" || (msg.String() == "?" && (m.help || m.focus == 0 || m.focus >= 4)) {
			m.help = !m.help
			return m, nil
		}
		if m.help && msg.String() != "ctrl+c" && msg.String() != "esc" {
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
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.help {
				m.help = false
			} else if m.details {
				m.details, m.focus, m.message = false, 0, ""
			} else {
				return m, tea.Quit
			}
		case "tab":
			if m.details {
				m.focus = m.focus%5 + 1
			} else {
				m.move(1)
			}
		case "shift+tab":
			if m.details {
				m.focus = (m.focus+3)%5 + 1
			} else {
				m.move(-1)
			}
		case "up":
			if !m.details {
				m.move(-1)
			}
		case "down":
			if !m.details {
				m.move(1)
			}
		case "enter":
			if m.focus == 7 {
				m.localFiles = true
				return m, nil
			}
			if m.focus == 6 || (m.focus == 0 && len(m.choices) == 0) {
				m.newConnection = true
				return m, nil
			}
			if m.focus == 0 {
				m.details, m.focus = true, 1
			} else if m.focus < 4 {
				m.focus++
			} else {
				return m.activate()
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
	if m.focus >= 6 {
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

func (m picker) activate() (tea.Model, tea.Cmd) {
	if m.connectError != "" && m.focus != 5 {
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
	local, err := filepath.Abs(m.local)
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
	b := connections.Bookmark{Name: m.name, Remote: loc.Remote, Root: loc.Path, Local: local}
	if m.focus == 5 {
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

// Rendering and pointer hit testing use the same card rows and centered origin.
type pickerRow struct {
	text          string
	focus, choice int
	logo          bool
	metadata      int
}

type pickerLayout struct {
	x, y, width int
	rows        []pickerRow
}

func (m picker) layout() pickerLayout {
	l := pickerLayout{width: min(78, m.width-10)}
	if !m.details && !m.help {
		l.width = min(64, m.width-8)
	}
	spacious := m.height >= 32
	add := func(s string, focus, choice int) {
		l.rows = append(l.rows, pickerRow{text: s, focus: focus, choice: choice})
	}
	center := func(s string) { add(lipgloss.PlaceHorizontal(l.width-6, lipgloss.Center, s), -1, -1) }
	divider := func() { add(strings.Repeat("─", l.width-6), -1, -1) }
	action := func(s string, focus int) {
		// Preserve the existing details-screen Connect action.
		icon := [3]string{"  ■  ", "■ ■ ■", "  ■  "}
		if spacious {
			add(" "+icon[0], focus, -1)
		}
		label := " ■  " + s
		if spacious {
			label = " " + icon[1] + "   " + s
		}
		add(label+strings.Repeat(" ", max(1, l.width-8-ansi.StringWidth(label)))+"›", focus, -1)
		if spacious {
			add(" "+icon[2], focus, -1)
		}
	}
	add("", -1, -1)
	if m.help {
		for _, s := range []string{
			"Connection help",
			"↑↓ / j k move through connections and actions.",
			"Tab / Shift+Tab move; Enter opens; details: Tab.",
			"Type to append; Backspace delete; Ctrl+U clear.",
			"Connect opens files. Copies need confirmation.",
			"Save bookmark is explicit; same name replaces.",
			"Saves name, remote, root and local folder only.",
			"Credentials stay in rclone; no passwords here.",
			"Discovery lists names, not connection health.",
			"New connection opens setup; Esc browses locally.",
			"F1 / ? / Esc back · Ctrl+C quit",
		} {
			add(s, -1, -1)
		}
		if m.message != "" {
			add(m.message, -1, -1)
		}
	} else if m.details {
		add("Connection details", -1, -1)
		add("", -1, -1)
		center("▤  " + m.choices[m.cursor].Remote + ":")
		add("", -1, -1)
		add("Remote root    "+m.root, 1, -1)
		if spacious {
			add("", -1, -1)
		}
		add("Local folder   "+m.local, 2, -1)
		if spacious {
			add("", -1, -1)
		}
		add("Bookmark name  "+m.name, 3, -1)
		add("", -1, -1)
		divider()
		action("[ Connect ]", 4)
		add("■  [ Save bookmark ]", 5, -1)
		add("Saves folders and name only. Does not connect.", -1, -1)
		add("", -1, -1)
		add("‹  Back", 8, -1)
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
			add("  "+strings.Join(tiles, " ")+"   "+label, -1, -1)
			l.rows[len(l.rows)-1].logo = true
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
				metadata := 0
				if m.width >= 80 && b.Name != "" {
					s = ansi.Truncate(s, 22, "…")
					s += strings.Repeat(" ", 22-ansi.StringWidth(s)) + " " + b.Remote + ":" + b.Root
					metadata = 28
				}
				add(prefix+s, 0, i)
				l.rows[len(l.rows)-1].metadata = metadata
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
	l.x, l.y = (m.width-l.width)/2, (m.height-len(l.rows)-2)/2
	return l
}

func (m picker) View() string {
	if m.width < 60 || m.height < 21 {
		return "Resize to at least 60 columns × 21 rows. Esc cancels.\n"
	}
	l := m.layout()
	base := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Foreground)).Background(lipgloss.Color(m.theme.Background))
	muted := base.Foreground(lipgloss.Color(m.theme.Muted))
	accent := base.Foreground(lipgloss.Color(m.theme.Focus)).Bold(true)
	var card []string
	for i, row := range l.rows {
		style := base
		if i == 1 && (m.details || m.help) {
			style = style.Foreground(lipgloss.Color(m.theme.Focus)).Bold(true)
		}
		if row.focus < 0 && i > 1 {
			style = muted
		}
		if row.focus >= 0 && m.focus == row.focus && (row.choice < 0 || row.choice == m.cursor) {
			style = style.Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection))
		}
		if row.logo && strings.Contains(row.text, "Open a connection") {
			style = accent
		}
		s := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, row.text)
		padding := 2
		if !m.details && !m.help {
			padding = 0
		}
		contentWidth := l.width - 2 - 2*padding
		s = ansi.Truncate(s, contentWidth, "…")
		content := style.Width(contentWidth).Render(s)
		if row.logo {
			content = style.Width(contentWidth).Render(strings.ReplaceAll(s, "■", accent.Render("■")))
		}
		if row.metadata > 0 && (m.focus != row.focus || m.cursor != row.choice) {
			content = base.Render(ansi.Cut(s, 0, row.metadata)) + muted.Width(contentWidth-row.metadata).Render(ansi.Cut(s, row.metadata, contentWidth))
		}
		card = append(card, base.Render(strings.Repeat(" ", padding))+content+base.Render(strings.Repeat(" ", padding)))
	}
	box := base.Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(m.theme.Border)).BorderBackground(lipgloss.Color(m.theme.Background)).Render(strings.Join(card, "\n"))
	rows := make([]string, m.height)
	for i := range rows {
		rows[i] = base.Width(m.width).Render("")
	}
	rows[0] = accent.Width(m.width).Render("  LAZYFTP")
	rows[1] = muted.Render(strings.Repeat("─", m.width))
	for i, row := range strings.Split(box, "\n") {
		background := rows[l.y+i]
		rows[l.y+i] = ansi.Cut(background, 0, l.x) + row + ansi.Cut(background, l.x+l.width, m.width)
	}
	footer := "↑↓ / j k move   Enter open   Esc local files   F1 help"
	if m.width < 80 {
		footer = "↑↓ move  Enter open  Esc local files  F1 help"
	}
	if m.details {
		footer = "Tab focus · Enter select · Esc connections · F1 help"
	}
	rows[m.height-1] = muted.Width(m.width).Render(" " + ansi.Truncate(footer, m.width-2, "…"))
	rows[m.height-2] = muted.Render(strings.Repeat("─", m.width))
	return strings.Join(rows, "\n")
}

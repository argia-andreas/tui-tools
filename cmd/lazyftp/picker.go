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
		switch msg.String() {
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
			} else if m.focus == 0 || (m.focus == 7 && len(m.choices) == 0) {
				m.focus = 6
			} else if m.focus == 6 {
				m.focus = 7
			} else {
				m.focus = 0
			}
		case "shift+tab":
			if m.details {
				m.focus = (m.focus+3)%5 + 1
			} else if m.focus == 0 || (m.focus == 6 && len(m.choices) == 0) {
				m.focus = 7
			} else if m.focus == 7 {
				m.focus = 6
			} else {
				m.focus = 0
			}
		case "up":
			if !m.details && m.focus == 7 {
				m.focus = 6
			} else if m.focus == 0 {
				m.move(-1)
			}
		case "down":
			if !m.details && m.focus == 6 {
				m.focus = 7
			} else if m.focus == 0 {
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
	if len(m.choices) == 0 {
		return
	}
	m.cursor = max(0, min(len(m.choices)-1, m.cursor+delta))
	m.choose()
}

func (m picker) visibleChoices() int {
	if m.height >= 32 {
		return min(3, max(1, (m.height-22)/2))
	}
	return min(4, max(1, m.height-17))
}

func (m picker) offset() int { return max(0, m.cursor-m.visibleChoices()+1) }

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
}

type pickerLayout struct {
	x, y, width int
	rows        []pickerRow
}

func (m picker) layout() pickerLayout {
	l := pickerLayout{width: min(78, m.width-10)}
	spacious := m.height >= 32
	add := func(s string, focus, choice int) { l.rows = append(l.rows, pickerRow{s, focus, choice}) }
	center := func(s string) { add(lipgloss.PlaceHorizontal(l.width-6, lipgloss.Center, s), -1, -1) }
	divider := func() { add(strings.Repeat("─", l.width-6), -1, -1) }
	action := func(s string, focus int) {
		if spacious {
			add("", focus, -1)
		}
		add(s+strings.Repeat(" ", max(1, l.width-8-ansi.StringWidth(s)))+"›", focus, -1)
		if spacious {
			add("", focus, -1)
		}
	}
	add("", -1, -1)
	if m.help {
		for _, s := range []string{
			"Connection help", "", "Choose a bookmark or an existing rclone remote.",
			"Tab / Shift+Tab focus; Enter selects or opens.",
			"Type to append; Backspace delete; Ctrl+U clear.",
			"Connect opens files. Copies need confirmation.",
			"Save bookmark is explicit; same name replaces.",
			"Saves name, remote name, root and local folder.",
			"Credentials stay in rclone.",
			"Never enter passwords or tokens here.",
			"Discovery lists names, not authentication status.",
			"New connection opens guided setup.",
			"", "F1 / ? / Esc back · Ctrl+C quit",
		} {
			add(s, -1, -1)
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
		action("▷  [ Connect ]", 4)
		add("◇  [ Save bookmark ]", 5, -1)
		add("Saves folders and name only. Does not connect.", -1, -1)
		add("", -1, -1)
		add("‹  Back", 8, -1)
	} else {
		add("Open a connection", -1, -1)
		add("", -1, -1)
		if len(m.choices) == 0 {
			if spacious {
				center("┌────┐")
				center("├─ · ┤")
				center("└────┘")
				add("", -1, -1)
			}
			center("No connections yet")
			center("Connect your first remote to get started.")
		} else {
			add(fmt.Sprintf("Bookmarks & remotes  ·  %d/%d", m.cursor+1, len(m.choices)), -1, -1)
			for i := m.offset(); i < min(m.offset()+m.visibleChoices(), len(m.choices)); i++ {
				b := m.choices[i]
				s := b.Remote + ":"
				if b.Name != "" {
					s = b.Name
				}
				prefix := "  ▤  "
				if i == m.cursor {
					prefix = "› ▤  "
				}
				add(prefix+s, 0, i)
				if spacious {
					add("     "+b.Remote+":"+b.Root, 0, i)
				}
			}
		}
		add("", -1, -1)
		action("⊕  New connection", 6)
		action("▱  Browse local files", 7)
		add("", -1, -1)
		divider()
		add("FTP / FTPS · Azure Blob · S3", -1, -1)
	}
	if m.message != "" {
		add(m.message, -1, -1)
	}
	if spacious {
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
		if i == 1 || row.focus == 6 {
			style = style.Foreground(lipgloss.Color(m.theme.Focus)).Bold(true)
		}
		if row.focus < 0 && i > 1 {
			style = muted
		}
		if row.focus >= 0 && m.focus == row.focus && (row.choice < 0 || row.choice == m.cursor) {
			style = style.Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection))
		}
		s := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, row.text)
		card = append(card, base.Render("  ")+style.Width(l.width-6).Render(ansi.Truncate(s, l.width-6, "…"))+base.Render("  "))
	}
	box := base.Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(m.theme.Border)).BorderBackground(lipgloss.Color(m.theme.Background)).Render(strings.Join(card, "\n"))
	rows := make([]string, m.height)
	for i := range rows {
		cells := []rune(strings.Repeat(" ", m.width))
		if !m.details && !m.help && i > 2 && i < m.height-3 {
			// Sparse packets drift across fixed lanes, never through the card.
			for packet := 0; packet < 12; packet++ {
				if 3+(packet*7)%(m.height-6) != i {
					continue
				}
				x := (packet*31+m.frame)%(m.width+8) - 4
				for bit, r := range "0101" {
					col := x + bit
					if col >= 0 && col < m.width && (i < l.y-1 || i > l.y+len(l.rows)+2 || col < l.x-2 || col >= l.x+l.width+2) {
						cells[col] = r
					}
				}
			}
		}
		rows[i] = base.Foreground(lipgloss.Color(m.theme.Border)).Faint(true).Render(string(cells))
	}
	rows[0] = accent.Width(m.width).Render("  LAZYFTP")
	rows[1] = muted.Render(strings.Repeat("─", m.width))
	for i, row := range strings.Split(box, "\n") {
		background := rows[l.y+i]
		rows[l.y+i] = ansi.Cut(background, 0, l.x) + row + ansi.Cut(background, l.x+l.width, m.width)
	}
	footer := "↑↓ choose · Tab · Enter open · Esc local files · F1 help"
	if m.details {
		footer = "Tab focus · Enter select · Esc connections · F1 help"
	}
	rows[m.height-1] = muted.Width(m.width).Render(" " + ansi.Truncate(footer, m.width-2, "…"))
	rows[m.height-2] = muted.Render(strings.Repeat("─", m.width))
	return strings.Join(rows, "\n")
}

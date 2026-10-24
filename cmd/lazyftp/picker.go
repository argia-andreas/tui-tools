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
	help                         bool
	theme                        theme.Theme
	selected                     *connections.Bookmark
}

func newPicker(remotes []string, bookmarks []connections.Bookmark, local, leftOverride, path string, t theme.Theme) picker {
	m := picker{choices: append([]connections.Bookmark(nil), bookmarks...), local: local, root: "/", bookmarkPath: path, leftOverride: leftOverride, theme: t, width: 80, height: 24}
	for _, remote := range remotes {
		m.choices = append(m.choices, connections.Bookmark{Remote: remote, Root: "/", Local: local})
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
			delta := 1
			if msg.Button == tea.MouseButtonWheelUp {
				delta = -1
			}
			m.move(delta)
		} else if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.X >= 2 && msg.X < m.width-2 {
			y := msg.Y
			if y >= 4 && y < 10 && m.offset()+y-4 < len(m.choices) {
				m.cursor, m.focus = m.offset()+y-4, 0
				m.choose()
			}
			if y >= 11 && y <= 14 {
				m.focus = y - 10
			}
			if y == 15 {
				m.focus = 5
			}
			if y == 16 {
				m.newConnection = true
				return m, nil
			}
			if y == 14 || y == 15 {
				return m.activate()
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
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 7
		case "shift+tab":
			m.focus = (m.focus + 6) % 7
		case "up":
			if m.focus == 0 {
				m.move(-1)
			}
		case "down":
			if m.focus == 0 {
				m.move(1)
			}
		case "enter":
			if m.focus == 6 || (m.focus == 0 && len(m.choices) == 0) {
				m.newConnection = true
				return m, nil
			}
			if m.focus == 0 {
				m.focus = 1
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

func (m picker) offset() int { return max(0, m.cursor-5) }

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

func (m picker) View() string {
	if m.width < 60 || m.height < 21 {
		return "Resize to at least 60 columns × 21 rows. Esc cancels.\n"
	}
	var rows []string
	line := func(s string, active bool) {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Foreground)).Background(lipgloss.Color(m.theme.Background))
		if active {
			style = style.Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection))
		}
		s = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, s)
		rows = append(rows, style.Width(m.width).Render("  "+ansi.Truncate(s, m.width-4, "…")))
	}
	if m.help {
		for _, s := range []string{
			"Connection help", "", "Choose a bookmark or an existing rclone remote.",
			"Tab / Shift+Tab changes focus; Enter selects or activates.",
			"Type to append; Backspace deletes; Ctrl+U clears a field.",
			"Connect opens browsing. Copies still require confirmation.",
			"Save bookmark is explicit; saving the same name replaces it.",
			"Only name, remote name, root and local folder are saved.",
			"Credentials stay in rclone. Never enter passwords or tokens here.",
			"Discovery lists names; it does not test authentication.",
			"No remotes? Run rclone config, then reopen connections.",
			"", "F1 / ? back · Esc cancel connection · Ctrl+C quit",
		} {
			line(s, false)
		}
		for len(rows) < m.height {
			line("", false)
		}
		return strings.Join(rows, "\n")
	}
	line("lazyftp  /  Open a connection", false)
	line("", false)
	line("", false)
	line(fmt.Sprintf("Connections  (%d/%d)  ↑↓ or click; wheel scrolls", min(m.cursor+1, len(m.choices)), len(m.choices)), m.focus == 0)
	for i := 0; i < 6; i++ {
		idx := m.offset() + i
		s := ""
		if idx < len(m.choices) {
			b := m.choices[idx]
			s = "  " + b.Remote + ":"
			if b.Name != "" {
				s = "  " + b.Name + "  ·  " + b.Remote + ":" + b.Root
			}
			if idx == m.cursor {
				s = ">" + s[1:]
			}
		} else if i == 0 && len(m.choices) == 0 {
			s = "No connections yet — choose New connection."
		}
		line(s, m.focus == 0 && idx == m.cursor && len(m.choices) > 0)
	}
	line("", false)
	line("Remote root    "+m.root, m.focus == 1)
	line("Local folder   "+m.local, m.focus == 2)
	line("Bookmark name  "+m.name, m.focus == 3)
	line("[ Connect ]", m.focus == 4)
	line("[ Save bookmark ]", m.focus == 5)
	line("[ New connection ]", m.focus == 6)
	line("", false)
	line(m.message, false)
	line("Tab focus · Enter select · Esc cancel · F1 help", false)
	for len(rows) < m.height {
		line("", false)
	}
	return strings.Join(rows, "\n")
}

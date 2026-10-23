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

// The startup picker is separate from the file workspace: no backend operations
// start until Connect, and saving always requires its own explicit action.
type picker struct {
	choices                      []connections.Bookmark
	cursor, focus, width, height int
	root, local, name, message   string
	bookmarkPath                 string
	leftOverride                 string
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
		if m.width < 60 || m.height < 21 {
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
			if y == 14 || y == 15 {
				return m.activate()
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 6
		case "shift+tab":
			m.focus = (m.focus + 5) % 6
		case "up":
			if m.focus == 0 {
				m.move(-1)
			}
		case "down":
			if m.focus == 0 {
				m.move(1)
			}
		case "enter":
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
	if len(m.choices) == 0 {
		m.message = "No remotes. Run rclone config, then restart."
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
		m.message = "Bookmark saved (same name replaces). Tab to Connect when ready."
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
	line("lazyftp  /  Open a connection", false)
	line("Choose a saved bookmark or an existing rclone remote", false)
	line("Credentials stay in rclone. Discovery does not test authentication.", false)
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
			s = "No remotes found — run rclone config, then restart."
		}
		line(s, m.focus == 0 && idx == m.cursor && len(m.choices) > 0)
	}
	line("Edit fields: type to append · Backspace · Ctrl+U clears", false)
	line("Remote root    "+m.root, m.focus == 1)
	line("Local folder   "+m.local, m.focus == 2)
	line("Bookmark name  "+m.name, m.focus == 3)
	line("[ Connect ]  Opens live browsing; copies require confirmation", m.focus == 4)
	line("[ Save bookmark ]  Explicit save; same name replaces", m.focus == 5)
	line("Only name, remote name, root and local folder are saved.", false)
	line("Never enter passwords, tokens or signed URLs in these fields.", false)
	line(m.message, false)
	line("Tab / Shift+Tab focus · Enter select/activate · Esc cancel", false)
	for len(rows) < m.height {
		line("", false)
	}
	return strings.Join(rows, "\n")
}

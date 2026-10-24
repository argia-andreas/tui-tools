package ui

import (
	"io/fs"
	"path"
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
)

type renameState struct {
	pane                int
	source              storage.Location
	name                []rune
	cursor, scroll      int
	confirming, running bool
}

type renameFinished struct {
	source, destination storage.Location
	err                 error
}

func (m Model) renameDisabled() string {
	if _, ok := m.browser.(storage.Renamer); !ok || m.transferer == nil {
		return "Rename unavailable — demo is read-only"
	}
	if m.deleting {
		return "Wait for the current deletion to finish before renaming"
	}
	if m.focus != 1 && m.focus != 2 {
		return "Focus a file pane to rename"
	}
	if m.panes[m.active()].detached {
		return "Connect a remote or open a local folder first"
	}
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return "Finish or cancel the current transfer queue first"
	}
	p := m.panes[m.active()]
	if p.loading || p.err != nil {
		return "Wait for a successful directory listing"
	}
	entries := p.visible()
	if p.cursor < 0 || p.cursor >= len(entries) || entries[p.cursor].IsDir {
		return "Choose a file, not a directory, to rename"
	}
	return ""
}

func (m Model) startRename() (tea.Model, tea.Cmd) {
	if reason := m.renameDisabled(); reason != "" {
		m.status = reason
		return m, nil
	}
	i := m.active()
	p := m.panes[i]
	name := []rune(p.visible()[p.cursor].Name)
	source := p.location
	source.Path = path.Join(source.Path, string(name))
	m.rename = &renameState{pane: i, source: source, name: name, cursor: len(name)}
	m.modal, m.status, m.filtering = "", "", false
	return m, nil
}

func (m Model) renameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	r := *m.rename
	// Copy the editable buffer: Bubble Tea models are values.
	r.name = append([]rune(nil), r.name...)
	m.rename = &r
	key := msg.String()
	if r.running {
		// Remote moves can be copy+delete. Let the bounded operation finish
		// rather than imply that Esc or quitting can roll it back.
		m.status = "Renaming… wait for the result; cancellation cannot roll back a started rename"
		return m, nil
	}
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if key == "esc" {
		m.rename, m.modal, m.status = nil, "", "Rename canceled — no changes"
		return m, nil
	}
	if r.confirming {
		switch key {
		case "left", "right", "tab", "shift+tab":
			m.choice = 1 - m.choice
		case "up":
			r.scroll = max(0, r.scroll-1)
		case "down":
			w, h := m.modalSize()
			r.scroll = min(r.scroll+1, max(0, len(m.renameDetails(w))-(h-4)))
		case "enter":
			if m.choice == 0 {
				m.rename, m.modal, m.status = nil, "", "Rename canceled — no changes"
				return m, nil
			}
			r.running = true
			m.modal = ""
			m.status = "Renaming… wait for the result"
			destination := r.source
			destination.Path = path.Join(path.Dir(r.source.Path), string(r.name))
			return m, func() tea.Msg {
				err := m.browser.(storage.Renamer).Rename(m.ctx, r.source, destination)
				return renameFinished{source: r.source, destination: destination, err: err}
			}
		}
		return m, nil
	}
	switch key {
	case "enter":
		name := string(r.name)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			m.status = "Rename needs a single filename, not an empty name or path"
			return m, nil
		}
		if name == path.Base(r.source.Path) {
			m.rename, m.status = nil, "Name unchanged — no changes"
			return m, nil
		}
		r.confirming = true
		m.modal, m.choice, m.status = "file-rename", 0, ""
	case "left":
		r.cursor = max(0, r.cursor-1)
	case "right":
		r.cursor = min(len(r.name), r.cursor+1)
	case "home", "ctrl+a":
		r.cursor = 0
	case "end", "ctrl+e":
		r.cursor = len(r.name)
	case "ctrl+u":
		r.name, r.cursor = nil, 0
	case "backspace", "ctrl+h":
		if r.cursor > 0 {
			r.name = append(r.name[:r.cursor-1], r.name[r.cursor:]...)
			r.cursor--
		}
	case "delete":
		if r.cursor < len(r.name) {
			r.name = append(r.name[:r.cursor], r.name[r.cursor+1:]...)
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			input := msg.Runes
			if msg.Type == tea.KeySpace {
				input = []rune{' '}
			}
			r.name = append(r.name[:r.cursor], append(append([]rune(nil), input...), r.name[r.cursor:]...)...)
			r.cursor += len(input)
		}
	}
	return m, nil
}

func (m Model) finishRename(msg renameFinished) (tea.Model, tea.Cmd) {
	if m.rename == nil || !m.rename.running || m.rename.source != msg.source {
		return m, nil
	}
	r := *m.rename
	m.rename, m.modal = nil, ""
	m.status = "Renamed " + strconv.Quote(path.Base(msg.source.Path)) + " → " + strconv.Quote(path.Base(msg.destination.Path))
	name := path.Base(msg.destination.Path)
	if msg.err != nil {
		m.status = "Rename failed: " + msg.err.Error() + " — refreshed; press R to try again"
		name = path.Base(msg.source.Path)
		if msg.err == fs.ErrExist {
			r.running, r.confirming, r.scroll = false, false, 0
			m.rename = &r
			m.status = "Name already exists — source unchanged; edit name · Enter review · Esc cancel"
		}
	}
	var cmds []tea.Cmd
	dir := msg.source
	dir.Path = path.Dir(dir.Path)
	for i := range m.panes {
		p := &m.panes[i]
		if p.location == dir {
			if p.selected[path.Base(msg.source.Path)] && msg.err == nil {
				delete(p.selected, path.Base(msg.source.Path))
				p.selected[name] = true
			}
			p.generation++
			p.loading, p.revealName = true, name
			cmds = append(cmds, m.list(i))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) renameInput(w int) string {
	r := m.rename
	before := text(string(r.name[:r.cursor]))
	after := text(string(r.name[r.cursor:]))
	for ansi.StringWidth(before) >= w && before != "" {
		before = string([]rune(before)[1:])
	}
	return fit(ansi.Cut(before+"▏"+after, 0, w), w)
}

func (m Model) renameDetails(w int) []string {
	r := m.rename
	dir := r.source
	dir.Path = path.Dir(dir.Path)
	content := "Location: " + strconv.Quote(dir.String()) + "\nFrom: " + strconv.Quote(path.Base(r.source.Path)) + "\nTo:   " + strconv.Quote(string(r.name))
	return strings.Split(lipgloss.NewStyle().Width(w-6).Render(content), "\n")
}

func (m Model) renameLines(w, h int) []string {
	r := m.rename
	lines := m.renameDetails(w)
	n := max(1, h-4)
	start := min(r.scroll, max(0, len(lines)-n))
	lines = lines[start:min(len(lines), start+n)]
	for len(lines) < n {
		lines = append(lines, "")
	}
	lines = append(lines, "← → choose · Enter confirm · Esc cancel · ↑↓ scroll")
	buttons := ""
	for i, label := range []string{"Cancel", "Rename"} {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Focus)).Background(lipgloss.Color(m.theme.Background))
		if i == m.choice {
			style = style.Bold(true).Foreground(lipgloss.Color(m.theme.SelectionText)).Background(lipgloss.Color(m.theme.Selection))
		}
		buttons += style.Render(" [" + label + "] ")
	}
	return append(lines, buttons)
}

package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"terminalui"
)

func (m Model) navFrame() terminalui.Frame {
	f := terminalui.Frame{Bounds: terminalui.Rect{W: m.width, H: m.height}}
	f.Regions = append(f.Regions, terminalui.RenderActionBar(m.headerActions(), terminalui.Rect{Y: 1, W: m.width, H: 1}, m.focus, m.style()).Regions...)
	for _, pane := range m.frames() {
		f.Regions = append(f.Regions, pane.Regions...)
	}
	f.Regions = append(f.Regions, terminalui.RenderActionBar(m.footerActions(), terminalui.Rect{Y: m.height - 4, W: m.width, H: 1}, m.focus, m.style()).Regions...)
	return f
}
func (m Model) selectedID() string {
	if m.focus == "sidebar" || m.view == "topics" {
		return fmt.Sprintf("topic:%d", m.topicIndex)
	}
	return fmt.Sprintf("row:%d", m.row)
}
func (m Model) navigation(msg tea.Msg) (tea.Model, tea.Cmd) {
	frame := m.navFrame()
	r := terminalui.HandleNavigation(msg, frame, m.focus, m.selectedID())
	if r.Intent != nil {
		id := r.Intent.ID
		switch r.Intent.Kind {
		case terminalui.Focus:
			if strings.HasPrefix(id, "field:") {
				m.selectID(id)
			} else {
				m.focus = id
			}
		case terminalui.Select:
			m.selectID(id)
		case terminalui.Activate:
			if strings.HasPrefix(id, "topic:") || strings.HasPrefix(id, "row:") {
				m.selectID(id)
				return m.execute("open")
			}
			return m.execute(id)
		case terminalui.Scroll:
			if id == "inspector" || id == "metadata" {
				m.scroll = max(0, m.scroll+r.Intent.Delta)
			} else {
				m.move(r.Intent.Delta, id)
			}
		}
		return m, nil
	}
	if r.Consumed {
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	k := key.String()
	if m.focus == "metadata" {
		switch k {
		case "pgdown", "j", "down":
			m.scroll += max(1, m.height-12)
			return m, nil
		case "pgup", "k", "up":
			m.scroll = max(0, m.scroll-max(1, m.height-12))
			return m, nil
		}
	}
	if m.focus == "inspector" {
		switch k {
		case "j", "down":
			m.field = min(m.field+1, m.fieldCount()-1)
			m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
			return m, nil
		case "k", "up":
			m.field = max(0, m.field-1)
			m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
			return m, nil
		case "pgdown":
			m.scroll += max(1, m.height-18)
			return m, nil
		case "pgup":
			m.scroll = max(0, m.scroll-max(1, m.height-18))
			return m, nil
		case "h", "left":
			m.hscroll = max(0, m.hscroll-8)
			return m, nil
		case "l", "right":
			m.hscroll += 8
			return m, nil
		case "]":
			data, _, _, _ := m.fieldData()
			m.readerWindow = min(max(0, (len(data)-1)/(256<<10)), m.readerWindow+1)
			m.scroll = 0
			return m, nil
		case "[":
			m.readerWindow = max(0, m.readerWindow-1)
			m.scroll = 0
			return m, nil
		}
	}
	switch k {
	case "j", "down":
		m.move(1, m.focus)
		return m, nil
	case "k", "up":
		m.move(-1, m.focus)
		return m, nil
	case "pgdown":
		m.move(max(1, m.height-12), m.focus)
		return m, nil
	case "pgup":
		m.move(-max(1, m.height-12), m.focus)
		return m, nil
	}
	for _, a := range m.actions() {
		for _, shortcut := range a.Keys {
			if shortcut == k {
				return m.execute(a.ID)
			}
		}
	}
	return m, nil
}
func (m *Model) selectID(id string) {
	if strings.HasPrefix(id, "topic:") {
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "topic:"))
		m.topicIndex = n
		m.scroll = 0
		if m.view == "topics" {
			m.focus = "list"
		} else {
			m.focus = "sidebar"
		}
	}
	if strings.HasPrefix(id, "row:") {
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "row:"))
		m.row = n
		m.focus = "list"
		m.field = 1
		m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
	}
	if strings.HasPrefix(id, "field:") {
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "field:"))
		m.field = n
		m.focus = "inspector"
		m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
	}
}
func (m *Model) move(delta int, owner string) {
	if owner == "sidebar" || m.view == "topics" {
		m.topicIndex = max(0, min(m.topicIndex+delta, len(m.visibleTopics())-1))
		m.scroll = 0
		return
	}
	if owner == "list" {
		m.row = max(0, min(m.row+delta, m.rowCount()-1))
		m.field = 1
		m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
	}
}
func (m Model) fieldCount() int {
	r, ok := m.selectedRecord()
	if !ok {
		return 2
	}
	return 2 + len(r.Headers)
}

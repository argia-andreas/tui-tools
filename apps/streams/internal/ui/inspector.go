package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"streams/internal/stream"
	"terminalui"
)

func containsLiteral(b []byte, s string) bool { return bytes.Contains(b, []byte(s)) }
func (m Model) selectedRecord() (stream.Record, bool) {
	if len(m.records) == 0 {
		return stream.Record{}, false
	}
	return m.records[min(m.row, len(m.records)-1)], true
}
func (m Model) fieldData() (data []byte, name string, index int, err error) {
	r, ok := m.selectedRecord()
	if !ok {
		return nil, "", 0, fmt.Errorf("Select an event")
	}
	field := min(m.field, len(r.Headers)+1)
	name = "value"
	index = 0
	if field == 0 {
		name = "key"
	} else if field >= 2 {
		name = "header"
		index = field - 2
	}
	data, err = stream.FieldPayload(r, name, index)
	return
}
func (m Model) inspectorFrame(b terminalui.Rect) terminalui.Frame {
	body := m.inspectorBody(terminalui.Rect{X: b.X + 1, Y: b.Y + 1, W: b.W - 2, H: b.H - 2})
	return m.panel("INSPECTOR", "inspector", b, body.Lines, body.Regions)
}

func (m Model) inspectorBody(b terminalui.Rect) terminalui.Frame {
	f := terminalui.Frame{Bounds: b, Regions: []terminalui.Region{{ID: "inspector", Kind: terminalui.PaneRegion, Bounds: b}}}
	r, ok := m.selectedRecord()
	if !ok {
		f.Lines = []string{" Select an event to preview"}
		return f
	}
	mode := []string{"json", "raw", "hex"}[m.readerMode]
	f.Lines = []string{m.paint(fmt.Sprintf(" %s · p%d / offset %d", safe(r.ID.Topic.Name), r.ID.Partition, r.ID.Offset), m.style().Foreground), m.paint(" "+r.Time.UTC().Format("2006-01-02 15:04:05.000Z")+" · "+safe(r.TimestampType), m.style().Muted), ""}
	tabs := []struct {
		label string
		field int
	}{{"Value", 1}, {"Key", 0}}
	if len(r.Headers) > 0 {
		tabs = append(tabs, struct {
			label string
			field int
		}{fmt.Sprintf("Headers %d", len(r.Headers)), 2})
	}
	line := " "
	for _, tab := range tabs {
		label := "[" + tab.label + "]"
		selected := m.field == tab.field || tab.field == 2 && m.field >= 2
		f.Regions = append(f.Regions, terminalui.Region{ID: fmt.Sprintf("field:%d", tab.field), Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: b.X + ansi.StringWidth(line), Y: b.Y + len(f.Lines), W: len(label), H: 1}})
		line += m.selectedLine(label, selected) + " "
	}
	f.Lines = append(f.Lines, line)
	if m.field >= 2 {
		start := max(0, m.field-2-3)
		for i := start; i < len(r.Headers) && i < start+max(1, min(5, b.H-9)); i++ {
			h := r.Headers[i]
			label := fmt.Sprintf(" [%d] %s · %d bytes", i, safe(h.Key), len(h.Value))
			if h.Value == nil {
				label += " · null"
			}
			f.Regions = append(f.Regions, terminalui.Region{ID: fmt.Sprintf("field:%d", i+2), Kind: terminalui.FieldRegion, Bounds: terminalui.Rect{X: b.X, Y: b.Y + len(f.Lines), W: b.W, H: 1}})
			f.Lines = append(f.Lines, m.selectedLine(label, i+2 == m.field))
		}
	}
	data, _, _, _ := m.fieldData()
	start := min(len(data), m.readerWindow*(256<<10))
	end := min(len(data), start+(256<<10))
	content := stream.Content(data[start:end], mode)
	colored := mode == "json" && json.Valid(data[start:end]) && !strings.HasPrefix(content[0], "JSON unavailable")
	f.Lines = append(f.Lines, m.paint(fmt.Sprintf(" %s · v changes mode · %d bytes", mode, len(data)), m.style().Muted), "")
	for i := min(m.scroll, max(0, len(content)-1)); i < len(content) && len(f.Lines) < b.H-1; i++ {
		line := content[i]
		if colored {
			line = highlightJSON(line)
		}
		f.Lines = append(f.Lines, " "+ansi.Cut(line, m.hscroll, m.hscroll+max(1, b.W-2)))
	}
	if len(data) > 256<<10 {
		f.Lines = append(f.Lines, m.paint(fmt.Sprintf(" bytes %d–%d · [ ] windows", start, end), m.style().Muted))
	}
	return f
}

func (m Model) payloadBounds() terminalui.Rect {
	return m.overlayBounds(min(160, m.width-4), m.height-4)
}
func (m Model) payloadSpec() terminalui.SurfaceSpec {
	r, _ := m.selectedRecord()
	body := m.inspectorBody(terminalui.SurfaceBodyBounds(m.payloadBounds()))
	for i, line := range body.Lines {
		body.Lines[i] = m.paint(line, m.style().Foreground)
	}
	return terminalui.SurfaceSpec{ID: "payload", Title: fmt.Sprintf("PAYLOAD / %s · p%d / offset %d", safe(r.ID.Topic.Name), r.ID.Partition, r.ID.Offset), CancelID: "back", FocusID: m.modalFocus, FocusOrder: []string{"field:1", "field:0", "field:2", "inspector", "back", "copy", "mode"}, Body: body, Actions: []terminalui.Action{{ID: "back", Label: "Close", Keys: []string{"esc"}}, {ID: "copy", Label: "Copy field", Keys: []string{"y"}}, {ID: "mode", Label: "JSON / raw / hex", Keys: []string{"v"}}}}
}
func (m Model) payloadFrame() terminalui.Frame {
	return terminalui.RenderSurface(m.payloadSpec(), m.payloadBounds(), m.style())
}
func (m Model) payloadInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && (m.modalFocus == "inspector" || strings.HasPrefix(m.modalFocus, "field:")) {
		switch key.String() {
		case "j", "down":
			if m.field >= 2 {
				m.field = min(m.field+1, m.fieldCount()-1)
				m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
			} else {
				m.scroll++
			}
			return m, nil
		case "k", "up":
			if m.field >= 2 {
				m.field = max(2, m.field-1)
				m.scroll, m.hscroll, m.readerWindow = 0, 0, 0
			} else {
				m.scroll = max(0, m.scroll-1)
			}
			return m, nil
		case "pgdown":
			m.scroll += max(1, m.height-16)
			return m, nil
		case "pgup":
			m.scroll = max(0, m.scroll-max(1, m.height-16))
			return m, nil
		case "h", "left":
			m.hscroll = max(0, m.hscroll-8)
			return m, nil
		case "l", "right":
			m.hscroll += 8
			return m, nil
		case "[", "]":
			data, _, _, _ := m.fieldData()
			delta := 1
			if key.String() == "[" {
				delta = -1
			}
			m.readerWindow = max(0, min(max(0, (len(data)-1)/(256<<10)), m.readerWindow+delta))
			m.scroll = 0
			return m, nil
		}
	}
	result := terminalui.HandleSurface(msg, m.payloadSpec(), m.payloadFrame())
	if result.Intent != nil {
		i := result.Intent
		switch i.Kind {
		case terminalui.Dismiss:
			return m.execute("back")
		case terminalui.Activate:
			return m.execute(i.ID)
		case terminalui.Scroll:
			m.scroll = max(0, m.scroll+i.Delta)
		case terminalui.Focus:
			m.modalFocus = i.ID
			if strings.HasPrefix(i.ID, "field:") {
				n, _ := strconv.Atoi(strings.TrimPrefix(i.ID, "field:"))
				m.field = n
				m.scroll, m.readerWindow, m.hscroll = 0, 0, 0
			}
		}
	}
	return m, nil
}

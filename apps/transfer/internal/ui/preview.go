package ui

import (
	"context"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/storage"
)

const previewLimit = 64 * 1024

type previewState struct {
	location  storage.Location
	focus     string
	loading   bool
	err       error
	lines     []string
	truncated bool
	binary    bool
}

type previewLoaded struct {
	generation int
	data       []byte
	err        error
}

func (m Model) previewDisabled() string {
	if m.panes[m.active()].detached {
		return "Choose a source before previewing"
	}
	if _, ok := m.browser.(storage.Reader); !ok {
		return "Content preview is unavailable for this browser"
	}
	p := m.panes[m.active()]
	if p.loading {
		return "Wait for the directory to finish loading"
	}
	if p.err != nil {
		return "Refresh the directory before previewing a file"
	}
	entries := p.visible()
	if p.cursor < 0 || p.cursor >= len(entries) {
		return "Choose a file to preview"
	}
	if entries[p.cursor].IsDir {
		return "Choose a file, not a directory, to preview"
	}
	return ""
}

func (m Model) startPreview() (tea.Model, tea.Cmd) {
	if reason := m.previewDisabled(); reason != "" {
		m.status = reason
		return m, nil
	}
	if m.previewCancel != nil {
		m.previewCancel()
	}
	p := m.panes[m.active()]
	loc := p.location
	loc.Path = path.Join(loc.Path, p.visible()[p.cursor].Name)
	parent := m.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	m.previewCancel = cancel
	m.previewGeneration++
	generation := m.previewGeneration
	m.preview = &previewState{location: loc, focus: "preview-body", loading: true}
	m.modal, m.choice = "preview", 0
	reader := m.browser.(storage.Reader)
	return m, func() tea.Msg {
		defer cancel()
		data, err := reader.Read(ctx, loc, previewLimit+1)
		// Bound retained data even if a Reader violates its limit contract.
		if len(data) > previewLimit+1 {
			data = data[:previewLimit+1]
		}
		return previewLoaded{generation: generation, data: data, err: err}
	}
}

func (m Model) applyPreview(msg previewLoaded) Model {
	if m.modal != "preview" || m.preview == nil || msg.generation != m.previewGeneration {
		return m
	}
	p := *m.preview
	p.loading, p.err = false, msg.err
	p.lines, p.binary, p.truncated = nil, false, false
	if msg.err == nil {
		data := msg.data
		p.truncated = len(data) > previewLimit
		if p.truncated {
			data = data[:previewLimit]
			// A byte limit can split an otherwise valid final UTF-8 rune.
			start := len(data) - 1
			for start > 0 && !utf8.RuneStart(data[start]) && len(data)-start < utf8.UTFMax {
				start--
			}
			if !utf8.FullRune(data[start:]) {
				data = data[:start]
			}
		}
		p.binary = !utf8.Valid(data) || strings.ContainsRune(string(data), 0)
		if !p.binary {
			clean := previewText(string(data))
			if len(data) > 0 {
				p.lines = strings.Split(clean, "\n")
			}
		}
	}
	m.preview = &p
	m.choice = 0
	return m
}

// Sanitize before adding any trusted UI styling. In particular, tabs cannot
// advance the terminal cursor and format characters cannot reorder UI labels.
func previewText(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n':
			return r
		case '\t':
			return ' '
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, ansi.Strip(strings.ToValidUTF8(s, "�")))
}

func previewPage(h int) int { return max(0, h-5) }

func (m Model) closePreview() (tea.Model, tea.Cmd) {
	if m.previewCancel != nil {
		m.previewCancel()
	}
	m.previewCancel = nil
	m.previewGeneration++
	m.preview, m.modal, m.choice = nil, "", 0
	return m, nil
}

func (m Model) scrollPreview(delta int) Model {
	_, h := m.modalSize()
	last := 0
	if m.preview != nil && !m.preview.loading && m.preview.err == nil && !m.preview.binary {
		last = max(0, len(m.preview.lines)-max(1, previewPage(h)))
	}
	m.choice = max(0, min(last, m.choice+delta))
	return m
}

func (m Model) previewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, h := m.modalSize()
	page := previewPage(h)
	switch msg.String() {
	case "up", "k":
		m.choice--
	case "down", "j":
		m.choice++
	case "pgup", "ctrl+u":
		m.choice -= max(1, page)
	case "pgdown", "ctrl+d", " ":
		m.choice += max(1, page)
	case "home", "g":
		m.choice = 0
	case "end", "G":
		if m.preview != nil {
			m.choice = len(m.preview.lines)
		}
	}
	return m.scrollPreview(0), nil
}

// w and h include the surrounding block's borders. Reserve a path, a status,
// and a footer so truncation and loading/error notices never scroll away.
func (m Model) previewLines(w, h int) (string, []string) {
	title := "CONTENT PREVIEW · 64 KiB maximum"
	if m.preview == nil {
		return title, []string{"No preview requested"}
	}
	p := m.preview
	lines := []string{strings.ReplaceAll(previewText(p.location.String()), "\n", " ")}
	status := "Text preview · read-only"
	switch {
	case p.loading:
		status = "Loading…"
	case p.err != nil:
		status = "Preview failed: " + strings.ReplaceAll(previewText(p.err.Error()), "\n", " ")
	case p.binary:
		status = "Binary or invalid UTF-8 content · not displayed"
	case len(p.lines) == 0:
		status = "Empty file"
	}
	if p.truncated {
		status = "Truncated at 64 KiB · " + status
	}
	lines = append(lines, status)
	page := previewPage(h)
	if !p.loading && p.err == nil && !p.binary {
		start := max(0, min(m.choice, max(0, len(p.lines)-max(1, page))))
		lines = append(lines, p.lines[start:min(len(p.lines), start+page)]...)
	}
	lines = lines[:min(len(lines), max(0, h-3))]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(0, w-2), "…")
	}
	return title, lines
}

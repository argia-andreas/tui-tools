package ui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
)

type copyState struct {
	source, destination storage.Location
	size                int64
	checking, conflict  bool
	auto                bool // only untouched new files in a confirmed batch
	err                 error
}

type destinationChecked struct {
	operation int
	exists    bool
	err       error
}
type copyFinished struct {
	source, destination storage.Location
	err                 error
}
type transferJob struct {
	source, destination storage.Location
	size                int64
	state               string
	err                 error
}

func (m Model) copyEntries() []storage.Entry {
	p := m.panes[m.active()]
	var chosen []storage.Entry
	for _, e := range p.entries {
		if p.selected[e.Name] {
			chosen = append(chosen, e)
		}
	}
	if len(chosen) == 0 {
		entries := p.visible()
		if len(entries) == 0 || entries[p.cursor].Name == ".." {
			return nil
		}
		chosen = append(chosen, entries[p.cursor])
	}
	return chosen
}

func (m Model) copyEntry() (storage.Entry, string) {
	p := m.panes[m.active()]
	if p.loading || p.err != nil {
		return storage.Entry{}, "Wait for a successful directory listing"
	}
	entries := m.copyEntries()
	if len(entries) == 0 {
		return storage.Entry{}, "No file to copy"
	}
	return entries[0], ""
}

func (m Model) copyDisabled() string {
	if m.transferer == nil {
		return "Transfers arrive in live mode; demo is read-only"
	}
	if m.panes[m.active()].detached {
		return "Choose a source before copying"
	}
	if m.panes[1-m.active()].detached {
		return "Choose a destination before copying"
	}
	if m.panes[1-m.active()].loading || m.panes[1-m.active()].err != nil {
		return "Wait for a successful destination listing"
	}
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return "Finish or cancel the current transfer queue first"
	}
	if m.panes[0].location.Remote != "" && m.panes[1].location.Remote != "" {
		return "Remote-to-remote copies are not supported yet"
	}
	_, reason := m.copyEntry()
	return reason
}

func (m Model) checkDestination() (tea.Model, tea.Cmd) {
	p := *m.pending
	p.checking, p.conflict, p.err = true, false, nil
	m.pending, m.modal, m.choice = &p, "copy", 0
	m.operation++
	op := m.operation
	ctx, cancel := context.WithCancel(m.ctx)
	m.queueCancel = cancel
	return m, func() tea.Msg {
		defer cancel()
		e, err := m.transferer.Stat(ctx, p.destination)
		if errors.Is(err, fs.ErrNotExist) {
			return destinationChecked{operation: op}
		}
		if err == nil && e.IsDir {
			err = fmt.Errorf("destination is a directory; choose another filename")
		}
		return destinationChecked{operation: op, exists: err == nil, err: err}
	}
}

func (m Model) copyButtons() []string {
	if m.pending.checking {
		return []string{"Cancel"}
	}
	if m.pending.err != nil {
		return []string{"Cancel", "Rename"}
	}
	if m.pending.conflict {
		return []string{"Cancel", "Skip", "Overwrite", "Rename"}
	}
	return []string{"Cancel", "Copy", "Rename"}
}

func (m Model) copyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only the destination-name field owns text/Enter; surface chrome owns
	// cancellation, focus and explicit buttons.
	if m.modal == "rename" && m.choice == 0 {
		switch msg.String() {
		case "enter":
			return m.checkCopyName()
		case "backspace", "ctrl+h":
			m.query = dropRune(m.query)
		case "ctrl+u":
			m.query = ""
		case " ":
			m.query += " "
		default:
			if msg.Type == tea.KeyRunes {
				m.query += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m Model) checkCopyName() (tea.Model, tea.Cmd) {
	if m.modal != "rename" || m.pending == nil {
		return m, nil
	}
	if m.query == "" || m.query == "." || m.query == ".." || strings.ContainsAny(m.query, "/\\") || strings.IndexFunc(m.query, unicode.IsControl) >= 0 {
		m.status = "Rename needs a single filename, not a path"
		return m, nil
	}
	p := *m.pending
	p.destination.Path = path.Join(path.Dir(p.destination.Path), m.query)
	p.auto = false
	m.pending, m.status = &p, ""
	return m.checkDestination()
}

func (m Model) copyAction(id string) (tea.Model, tea.Cmd) {
	if m.pending == nil || (m.modal != "copy" && m.modal != "rename") {
		return m, nil
	}
	if id == "cancel" {
		if m.queueIndex < len(m.queue) || m.queueCancel != nil {
			m.cancelWork()
		}
		m.modal, m.pending = "", nil
		return m, nil
	}
	if m.modal == "rename" {
		if id == "check-name" {
			return m.checkCopyName()
		}
		return m, nil
	}
	// Recheck the current destination result, not the button set seen before
	// an async completion or resize.
	available := false
	for _, label := range m.copyButtons() {
		available = available || strings.ToLower(label) == id
	}
	if !available {
		return m, nil
	}
	switch id {
	case "skip":
		m.job = &transferJob{source: m.pending.source, destination: m.pending.destination, state: "Skipped"}
		m.status, m.modal, m.pending = "Skipped — destination preserved", "", nil
		if m.queueIndex < len(m.queue) {
			m.queue[m.queueIndex] = *m.job
			m.queueIndex++
			return m.nextQueueModel()
		}
	case "rename":
		m.modal, m.query, m.choice, m.status = "rename", path.Base(m.pending.destination.Path), 0, ""
	case "copy", "overwrite":
		return m.startCopy(m.pending.conflict)
	}
	return m, nil
}

func (m Model) startCopy(overwrite bool) (tea.Model, tea.Cmd) {
	p := *m.pending
	ctx, cancel := context.WithCancel(m.ctx)
	m.transferCancel = cancel
	m.job = &transferJob{source: p.source, destination: p.destination, size: p.size, state: "Running"}
	if m.queueIndex < len(m.queue) {
		m.queue[m.queueIndex] = *m.job
		m.jobCursor = m.queueIndex
	}
	m.jobs, m.modal, m.pending = true, "", nil
	m.status = "Copying " + p.source.String()
	return m, func() tea.Msg {
		return copyFinished{source: p.source, destination: p.destination, err: m.transferer.Copy(ctx, p.source, p.destination, overwrite)}
	}
}

func (m Model) copyLines(w, h int) (string, []string) {
	p := m.pending
	title := "CONFIRM SINGLE-FILE COPY"
	lines := []string{"  From: " + text(p.source.String()), "  To:   " + text(p.destination.String()), fmt.Sprintf("  Size: %d B", p.size)}
	if m.modal == "rename" {
		return "RENAME DESTINATION", []string{"  New filename (Ctrl+U clears):", "  " + text(m.query) + "▏", "", "  Enter checks name · Esc cancels"}
	}
	detail := "  Destination is new. Copy requires confirmation."
	if p.checking {
		detail = "  Checking destination…"
	}
	if p.conflict {
		title = "DESTINATION EXISTS"
		detail = "  Overwrite replaces this file. Default: Cancel."
	}
	if p.err != nil {
		detail = "  " + text(p.err.Error())
	}
	lines = append(lines, m.paint(detail, m.theme.Warning))
	if h >= 10 {
		lines = append(lines, "", m.paint("  This choice applies to this file only.", m.theme.Muted), "  Tab / arrows choose · Enter confirms · Esc cancels")
	}
	return title, lines
}

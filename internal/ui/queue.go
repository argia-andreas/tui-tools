package ui

import (
	"context"
	"fmt"
	"path"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
)

type queuePlanned struct {
	operation int
	jobs      []transferJob
	err       error
}

// Planning is read-only. No work starts until the complete plan is confirmed.
func (m Model) planQueue(entries []storage.Entry) (tea.Model, tea.Cmd) {
	m.operation++
	op := m.operation
	ctx, cancel := context.WithCancel(m.ctx)
	m.queueCancel, m.queuePlanning = cancel, true
	m.queue, m.queueIndex, m.choice, m.modal = nil, 0, 0, "batch"
	source, destination := m.panes[m.active()].location, m.panes[1-m.active()].location
	return m, func() tea.Msg {
		var jobs []transferJob
		var walk func(storage.Location, storage.Location, []storage.Entry, int) error
		walk = func(src, dst storage.Location, children []storage.Entry, depth int) error {
			if depth > 64 {
				return fmt.Errorf("folder nesting exceeds 64 levels")
			}
			for _, e := range children {
				if err := ctx.Err(); err != nil {
					return err
				}
				if e.Name == "" || e.Name == "." || e.Name == ".." || strings.ContainsAny(e.Name, "/\\") || strings.IndexFunc(e.Name, unicode.IsControl) >= 0 {
					return fmt.Errorf("unsafe filename in source listing")
				}
				s, d := src, dst
				s.Path, d.Path = path.Join(src.Path, e.Name), path.Join(dst.Path, e.Name)
				// Local Stat rejects symlinks and special files, including entries
				// whose kind changed since the visible listing was fetched.
				if s.Remote == "" {
					actual, err := m.transferer.Stat(ctx, s)
					if err != nil {
						return err
					}
					e.IsDir, e.Size = actual.IsDir, actual.Size
				}
				if s.Remote == d.Remote && (s.Path == d.Path || (e.IsDir && strings.HasPrefix(path.Clean(d.Path)+"/", path.Clean(s.Path)+"/"))) {
					return fmt.Errorf("destination must not be the source or inside it")
				}
				if e.IsDir {
					children, err := m.browser.List(ctx, s)
					if err != nil {
						return err
					}
					if err := walk(s, d, children, depth+1); err != nil {
						return err
					}
				} else {
					if len(jobs) >= 10000 {
						return fmt.Errorf("queue exceeds 10000 files; choose a smaller folder")
					}
					jobs = append(jobs, transferJob{source: s, destination: d, size: e.Size, state: "Queued"})
				}
			}
			return nil
		}
		err := walk(source, destination, entries, 0)
		return queuePlanned{operation: op, jobs: jobs, err: err}
	}
}

func (m Model) acceptPlan(msg queuePlanned) (tea.Model, tea.Cmd) {
	if msg.operation != m.operation || !m.queuePlanning {
		return m, nil
	}
	m.queueCancel()
	m.queueCancel, m.queuePlanning = nil, false
	if msg.err != nil {
		m.modal, m.status = "", "Planning failed: "+msg.err.Error()
		return m, nil
	}
	m.queue, m.queueIndex = msg.jobs, len(msg.jobs) // not runnable until confirmation
	if len(msg.jobs) == 0 {
		m.modal, m.status = "", "No files to copy — empty directories/prefixes are not created"
	}
	return m, nil
}

func (m Model) batchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelWork()
	case "tab", "shift+tab", "left", "right", "up", "down":
		if !m.queuePlanning {
			m.choice = 1 - m.choice
		}
	case "enter":
		if m.choice == 0 {
			m.cancelWork()
		} else if !m.queuePlanning {
			m.queueIndex, m.modal, m.jobs = 0, "", true
			return m.nextQueueModel()
		}
	}
	return m, nil
}

func (m Model) nextQueueModel() (tea.Model, tea.Cmd) {
	next, cmd := m.nextQueued()
	return next, cmd
}

func (m Model) nextQueued() (Model, tea.Cmd) {
	for m.queueIndex < len(m.queue) && m.queue[m.queueIndex].state != "Queued" {
		m.queueIndex++
	}
	if m.queueIndex >= len(m.queue) {
		m.modal, m.pending = "", nil
		m.status = "Queue finished — " + m.queueSummary()
		return m, nil
	}
	j := m.queue[m.queueIndex]
	m.jobCursor = m.queueIndex
	m.pending = &copyState{source: j.source, destination: j.destination, size: j.size, auto: true}
	next, cmd := m.checkDestination()
	return next.(Model), cmd
}

func (m Model) cancelDisabled() string {
	if m.queuePlanning || m.queueIndex < len(m.queue) || (m.job != nil && m.job.state == "Running") {
		return ""
	}
	return "No unfinished transfers"
}

func (m *Model) cancelWork() {
	if m.previewCancel != nil {
		m.previewCancel()
		m.previewCancel = nil
		m.previewGeneration++
	}
	if m.queueCancel != nil {
		m.queueCancel()
		m.queueCancel = nil
	}
	if m.transferCancel != nil {
		m.transferCancel()
	}
	for i := range m.queue {
		if m.queue[i].state == "Queued" {
			m.queue[i].state = "Canceled"
		}
	}
	if m.job == nil || m.job.state != "Running" {
		m.queueIndex = len(m.queue)
	}
	m.queuePlanning = false
	m.operation++
	m.pending, m.modal = nil, ""
	m.status = "Canceled unfinished work — partial destination files may remain; retry rechecks conflicts"
}

func (m Model) retryDisabled() string {
	if m.cancelDisabled() == "" {
		return "Finish or cancel current work first"
	}
	for _, j := range m.queue {
		if j.state == "Failed" || j.state == "Canceled" {
			return ""
		}
	}
	if m.job != nil && (m.job.state == "Failed" || m.job.state == "Canceled") {
		return ""
	}
	return "No failed or canceled transfers to retry"
}

func (m Model) retryQueue() (tea.Model, tea.Cmd) {
	if reason := m.retryDisabled(); reason != "" {
		m.status = reason
		return m, nil
	}
	if len(m.queue) == 0 {
		m.queue = []transferJob{*m.job}
	}
	for i := range m.queue {
		if m.queue[i].state == "Failed" || m.queue[i].state == "Canceled" {
			m.queue[i].state = "Queued"
			m.queue[i].err = nil
		}
	}
	m.queueIndex, m.choice, m.modal = len(m.queue), 0, "batch"
	return m, nil
}

func (m Model) queueSummary() string {
	counts := map[string]int{}
	for _, j := range m.queue {
		counts[j.state]++
	}
	return fmt.Sprintf("%d completed · %d skipped · %d failed · %d canceled · %d queued · %d running", counts["Completed"], counts["Skipped"], counts["Failed"], counts["Canceled"], counts["Queued"], counts["Running"])
}

func (m Model) batchLines(h int) (string, []string) {
	lines := []string{"  Planning files… no writes yet."}
	if !m.queuePlanning {
		lines = []string{fmt.Sprintf("  %d captured file jobs. One transfer runs at a time.", len(m.queue)), "  " + m.queueSummary(), "  New files copy after confirmation; each conflict pauses.", "  Empty directories are not created. Cancel may leave partial files."}
		if len(m.queue) > 0 {
			lines = append(lines, "  From: "+text(m.queue[0].source.String()), "  To:   "+text(m.queue[0].destination.String()))
		}
	}
	lines = lines[:min(len(lines), h-4)]
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	buttons := []string{"Cancel"}
	if !m.queuePlanning {
		buttons = append(buttons, "Start queue")
	}
	row := " "
	for i, label := range buttons {
		button := " [" + label + "] "
		color := m.theme.Muted
		if i == m.choice {
			color = m.theme.Focus
		}
		row += m.paint(button, color)
	}
	return "CONFIRM TRANSFER QUEUE", append(lines, row)
}

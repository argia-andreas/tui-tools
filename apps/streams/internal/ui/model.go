package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"streams/internal/config"
	"streams/internal/stream"
	"streams/internal/theme"
	"terminalui"
)

type Options struct {
	Context             context.Context
	Backend             stream.Backend
	Connection          string
	Connections         []string
	OpenConnection      func(context.Context, string) (stream.Backend, error)
	Fixture, TrustOSC52 bool
	SettingsPath        string
}

type Model struct {
	options                                          Options
	session                                          *session
	width, height                                    int
	connectionGen, readGen                           uint64
	groupGen                                         uint64
	catalogGen                                       uint64
	switching                                        bool
	view, focus, status                              string
	modalFocus                                       string
	topics                                           []stream.Topic
	groups                                           []string
	group                                            stream.GroupObservation
	topicIndex, row, top, field, scroll              int
	readerMode, readerWindow, hscroll                int
	records                                          []stream.Record
	request                                          stream.ReadRequest
	cursor                                           stream.Cursor
	loading, following, paused, detail, showInternal bool
	filter, find                                     string
	settings                                         config.Settings
	palette                                          terminalui.Palette
	paletteOpen, help                                bool
	helpOffset                                       int
	editor                                           *editor
	jobCancel                                        context.CancelFunc
}
type catalogMsg struct {
	gen    uint64
	ticket uint64
	topics []stream.Topic
	groups []string
	kind   string
	err    error
}
type groupMsg struct {
	gen    uint64
	ticket uint64
	group  stream.GroupObservation
	err    error
}
type batchMsg struct {
	gen        uint64
	connection uint64
	batch      stream.Batch
	err        error
}
type tickMsg struct{ gen uint64 }
type statusMsg string
type connectionMsg struct {
	gen     uint64
	name    string
	backend stream.Backend
	err     error
}

func New(o Options) Model {
	if o.Context == nil {
		o.Context = context.Background()
	}
	s := &session{backends: []stream.Backend{o.Backend}}
	m := Model{options: o, session: s, connectionGen: 1, view: "topics", focus: "list", settings: config.DefaultSettings(), status: "Loading topics…"}
	if o.SettingsPath != "" {
		var err error
		m.settings, err = config.LoadSettings(o.SettingsPath)
		if err != nil {
			m.status = err.Error()
		}
	}
	return m
}
func (m Model) Close() {
	if m.jobCancel != nil {
		m.jobCancel()
	}
	m.session.close()
}
func (m Model) Init() tea.Cmd           { return m.catalog("topics") }
func (m Model) style() terminalui.Style { s := theme.Midnight(); s.ASCII = m.settings.ASCII; return s }
func (m Model) fits() bool              { return m.width >= 60 && m.height >= 16 }

func (m Model) catalog(kind string) tea.Cmd {
	backend, ctx, gen := m.options.Backend, m.options.Context, m.connectionGen
	ticket := m.catalogGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		r := catalogMsg{gen: gen, ticket: ticket, kind: kind}
		if kind == "groups" {
			r.groups, r.err = backend.Groups(ctx)
		} else {
			r.topics, r.err = backend.Topics(ctx)
		}
		return r
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		if m.focus == "metadata" && m.width < 110 {
			m.focus = "list"
		}
		if m.focus == "sidebar" && (m.width < 140 || !m.settings.Sidebar) || m.focus == "inspector" && !m.detail && (m.width < 110 || !m.settings.Inspector) {
			m.focus = "list"
		}
		return m, nil
	case statusMsg:
		m.status = string(v)
		return m, nil
	case catalogMsg:
		if v.gen != m.connectionGen || v.ticket != m.catalogGen || m.view != v.kind || m.switching {
			return m, nil
		}
		m.loading = false
		if v.err != nil {
			m.status = v.err.Error()
			return m, nil
		}
		if v.kind == "groups" {
			m.groups = v.groups
			m.status = fmt.Sprintf("%d groups · explicit refresh · read only", len(m.groups))
		} else {
			m.topics = v.topics
			m.topicIndex = min(m.topicIndex, max(0, len(m.visibleTopics())-1))
			m.status = fmt.Sprintf("%d topics · Open to read · counts are not event totals", len(m.topics))
		}
		return m, nil
	case groupMsg:
		if v.gen != m.connectionGen || v.ticket != m.groupGen || m.view != "group" {
			return m, nil
		}
		m.loading = false
		if v.err != nil {
			m.status = v.err.Error()
		} else {
			m.group = v.group
			m.status = "Group observation · high-watermark lag · not an atomic snapshot"
		}
		return m, nil
	case connectionMsg:
		if v.gen != m.connectionGen {
			if v.backend != nil {
				v.backend.Close()
			}
			return m, nil
		}
		m.switching = false
		m.readGen = m.session.invalidate()
		m.catalogGen++
		m.loading = false
		if v.err != nil {
			m.status = v.err.Error()
			return m, nil
		}
		m.options.Backend, m.options.Connection = v.backend, v.name
		m.topics, m.groups, m.records = nil, nil, nil
		m.group = stream.GroupObservation{}
		m.cursor = stream.Cursor{}
		m.request = stream.ReadRequest{}
		m.view, m.row, m.top = "topics", 0, 0
		m.topicIndex = 0
		m.filter = ""
		m.status = "Connected · loading topics"
		return m, m.catalog("topics")
	case batchMsg:
		if v.gen != m.readGen || v.connection != m.connectionGen || m.switching {
			return m, nil
		}
		m.loading = false
		if v.err != nil {
			m.following = false
			m.paused = true
			m.status = v.err.Error()
			return m, nil
		}
		if !m.accept(v.batch) {
			m.readGen = m.session.invalidate()
			m.following = false
			m.paused = true
			m.status = "Paused before selected event eviction · Resume or seek deliberately"
			return m, nil
		}
		m.cursor = stream.CloneCursor(v.batch.Next)
		m.status = fmt.Sprintf("%d loaded · %s", len(m.records), stopLabel(v.batch.Stop))
		if len(v.batch.Issues) > 0 {
			if v.batch.Stop != "deadline" {
				m.status += fmt.Sprintf(" · partition %d: %s", v.batch.Issues[0].Partition, v.batch.Issues[0].Err)
			}
			m.following = false
			m.paused = true
		}
		if v.batch.Stop == "error" || v.batch.Stop == "limit" {
			m.following = false
			m.paused = true
		}
		if m.following {
			gen := m.readGen
			return m, tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{gen} })
		}
		m.session.invalidate()
		return m, nil
	case tickMsg:
		if v.gen != m.readGen || !m.following || m.loading {
			return m, nil
		}
		m.loading = true
		gen, connection, s, ctx := m.readGen, m.connectionGen, m.session, m.options.Context
		return m, func() tea.Msg { return batchMsg{gen: gen, connection: connection, batch: s.next(ctx, gen)} }
	}
	// Async lifecycle runs above capture; input always belongs to the visible scope.
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		m.Close()
		return m, tea.Quit
	}
	if !m.fits() {
		if key, ok := msg.(tea.KeyMsg); ok {
			if key.String() == "q" {
				m.Close()
				return m, tea.Quit
			}
			if key.String() == "esc" {
				m.editor = nil
				m.paletteOpen = false
				m.help = false
			}
		}
		return m, nil
	}
	if m.help {
		r := terminalui.HandleHelp(msg, m.actions(), m.helpTopics(), &m.helpOffset, m.helpFrame())
		if r.Intent != nil && r.Intent.Kind == terminalui.Dismiss {
			m.help = false
		}
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "f1" {
		m.help = true
		return m, nil
	}
	if m.editor != nil {
		return m.editInput(msg)
	}
	if m.detail {
		return m.payloadInput(msg)
	}
	if m.paletteOpen {
		r := m.palette.Handle(msg, m.actions(), m.paletteFrame())
		if r.Intent != nil {
			switch r.Intent.Kind {
			case terminalui.Dismiss:
				m.paletteOpen = false
			case terminalui.Help:
				m.help = true
			case terminalui.Activate:
				m.paletteOpen = false
				return m.execute(r.Intent.ID)
			}
		}
		return m, nil
	}
	return m.navigation(msg)
}

func stopLabel(stop string) string {
	switch stop {
	case "end":
		return "window complete"
	case "deadline":
		return "scan incomplete; deadline / invisible gaps possible"
	case "records":
		return "record limit; Continue available"
	case "bytes":
		return "byte limit; Continue available"
	case "continue":
		return "FOLLOWING"
	case "limit":
		return "limit refused; seek explicitly"
	case "error":
		return "read stopped"
	}
	return stop
}

func (m *Model) accept(b stream.Batch) bool {
	all := make([]stream.Record, 0, len(m.records)+len(b.Records))
	all = append(all, m.records...)
	all = append(all, b.Records...)
	n := 0
	for _, r := range all {
		n += r.Bytes()
	}
	drop := 0
	for len(all)-drop > 2000 || n > 16<<20 {
		n -= all[drop].Bytes()
		drop++
	}
	if drop > 0 && len(m.records) > 0 && m.row < drop {
		return false
	}
	m.records = all[drop:]
	m.row = max(0, m.row-drop)
	return true
}

func (m Model) visibleTopics() []stream.Topic {
	var ts []stream.Topic
	for _, t := range m.topics {
		if (!t.Internal || m.showInternal) && strings.Contains(strings.ToLower(t.Ref.Name), strings.ToLower(m.filter)) {
			ts = append(ts, t)
		}
	}
	return ts
}
func (m Model) selectedTopic() (stream.Topic, bool) {
	ts := m.visibleTopics()
	if len(ts) == 0 {
		return stream.Topic{}, false
	}
	return ts[min(m.topicIndex, len(ts)-1)], true
}

func (m Model) begin(q stream.ReadRequest, clear bool) (tea.Model, tea.Cmd) {
	if m.switching {
		m.status = "Connection switch in progress"
		return m, nil
	}
	m.readGen = m.session.invalidate()
	gen := m.readGen
	connection := m.connectionGen
	m.request = q
	m.following = q.Follow || q.Mode == "follow"
	m.paused = false
	m.loading = true
	m.view = "events"
	m.focus = "list"
	m.detail = false
	if clear {
		m.records = nil
		m.row, m.top, m.field, m.scroll = 0, 0, 1, 0
		m.cursor = stream.Cursor{}
	}
	m.status = "Reading committed events…"
	s, ctx, backend := m.session, m.options.Context, m.options.Backend
	return m, func() tea.Msg {
		b, err := s.open(ctx, backend, gen, q)
		return batchMsg{gen: gen, connection: connection, batch: b, err: err}
	}
}

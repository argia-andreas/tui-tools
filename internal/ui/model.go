package ui

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
)

type Options struct {
	Browser      storage.Browser
	Context      context.Context
	Transferer   storage.Transferer
	Locations    []storage.Location
	Left, Right  storage.Location
	Theme        theme.Theme
	Icons        bool
	Disconnected bool
}

type pane struct {
	location   storage.Location
	entries    []storage.Entry
	cursor     int
	selected   map[string]bool
	filter     string
	generation int
	loading    bool
	err        error
	parent     bool
	revealName string
}

type listed struct {
	pane, generation int
	entries          []storage.Entry
	err              error
}

type Model struct {
	browser                     storage.Browser
	ctx                         context.Context
	transferer                  storage.Transferer
	locations                   []storage.Location
	pending                     *copyState
	deleteTarget                *storage.Location
	deleting                    bool
	job                         *transferJob
	queue                       []transferJob
	queueIndex                  int
	jobCursor                   int
	queuePlanning               bool
	queueCancel                 context.CancelFunc
	transferCancel              context.CancelFunc
	preview                     *previewState
	rename                      *renameState
	previewGeneration           int
	previewCancel               context.CancelFunc
	operation                   int
	panes                       [2]pane
	theme                       theme.Theme
	icons                       bool
	width, height               int
	focus                       int // 0 navigation, 1 local, 2 remote, 3 inspector, 4 jobs
	endpoint                    int // last focused file pane, retained while inspecting
	navigation, inspector, jobs bool
	modal                       string
	query                       string
	choice                      int
	filtering                   bool
	status                      string
	navCursor                   int
	disconnected                bool
	destinationChoice           int
	folderPath                  string
}

func New(o Options) Model {
	m := Model{browser: o.Browser, theme: o.Theme, icons: o.Icons, focus: 1, navigation: true,
		status: "Demo workspace — fixture data only; no network or disk writes"}
	m.transferer, m.locations = o.Transferer, o.Locations
	m.ctx = o.Context
	if m.ctx == nil {
		m.ctx = context.Background()
	}
	if len(m.locations) == 0 {
		m.locations = navLocations
	}
	if m.transferer != nil {
		m.navigation = false
		m.status = ""
	}
	m.disconnected = o.Disconnected
	if m.disconnected {
		m.navigation = false
		if len(o.Locations) == 0 {
			m.locations = []storage.Location{o.Left}
		}
	}
	m.panes[0] = pane{location: o.Left, selected: map[string]bool{}, loading: true, parent: o.Transferer != nil}
	m.panes[1] = pane{location: o.Right, selected: map[string]bool{}, loading: !m.disconnected, parent: o.Transferer != nil}
	for i := range m.panes {
		if m.panes[i].hasParent() {
			m.panes[i].cursor = 1
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.list(0), m.list(1))
}

func (m Model) list(i int) tea.Cmd {
	if i == 1 && m.disconnected {
		return nil
	}
	p := m.panes[i]
	return func() tea.Msg {
		entries, err := m.browser.List(m.ctx, p.location)
		return listed{pane: i, generation: p.generation, entries: entries, err: err}
	}
}

func (m *Model) navigate(i int, loc storage.Location) tea.Cmd {
	if i == 1 {
		m.disconnected = false
	}
	p := &m.panes[i]
	p.location, p.cursor, p.filter = loc, 0, ""
	p.revealName = ""
	if p.hasParent() {
		p.cursor = 1
	}
	p.entries = nil
	p.selected = map[string]bool{}
	p.generation++
	p.loading, p.err = true, nil
	return m.list(i)
}

func (m Model) active() int {
	if m.focus == 2 {
		return 1
	}
	if m.focus == 1 {
		return 0
	}
	return m.endpoint
}

func (p pane) hasParent() bool {
	return p.parent && p.location.Path != "" && path.Clean(p.location.Path) != "/"
}

func (p pane) visible() []storage.Entry {
	var entries []storage.Entry
	if p.hasParent() && p.filter == "" {
		entries = append(entries, storage.Entry{Name: "..", IsDir: true})
	}
	for _, e := range p.entries {
		if strings.Contains(strings.ToLower(e.Name), strings.ToLower(p.filter)) {
			entries = append(entries, e)
		}
	}
	return entries
}

func (m Model) regions() []int {
	r := []int{}
	if m.navigation && m.width >= 90 {
		r = append(r, 0)
	}
	r = append(r, 1, 2)
	if m.inspector && m.width >= 120 {
		r = append(r, 3)
	}
	if m.jobs {
		r = append(r, 4)
	}
	return r
}

func (m *Model) fixFocus() {
	for _, r := range m.regions() {
		if r == m.focus {
			return
		}
	}
	m.focus = m.endpoint + 1
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenLocalFolderRequested:
		return m.execute("folder")
	case ConnectionOpened:
		if msg.Browser != nil {
			m.browser = msg.Browser
		}
		if msg.Transferer != nil {
			m.transferer = msg.Transferer
		}
		m.disconnected = false
		m.status = ""
		m.focus, m.endpoint = 1, 0
		locations := append([]storage.Location{msg.Right}, msg.Locations...)
		locations = append(locations, m.locations...)
		m.locations = nil
		seen := map[storage.Location]bool{}
		// Navigation groups remotes before local places; repeated connections
		// must not duplicate roots or put a remote under the Places heading.
		for _, remote := range []bool{true, false} {
			for _, loc := range locations {
				if (loc.Remote != "") == remote && !seen[loc] {
					m.locations = append(m.locations, loc)
					seen[loc] = true
				}
			}
		}
		m.navCursor = 0
		var local tea.Cmd
		if msg.Left.Path != "" && msg.Left != m.panes[0].location {
			local = m.navigate(0, msg.Left)
		}
		return m, tea.Batch(local, m.navigate(1, msg.Right))
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.fixFocus()
	case listed:
		p := &m.panes[msg.pane]
		if p.generation == msg.generation {
			p.entries, p.err, p.loading = msg.entries, msg.err, false
			if p.revealName != "" {
				for i, e := range p.visible() {
					if e.Name == p.revealName {
						p.cursor = i
					}
				}
				p.revealName = ""
			}
			if p.cursor >= len(p.visible()) {
				p.cursor = 0
			}
		}
	case renameFinished:
		return m.finishRename(msg)
	case destinationChecked:
		if m.pending != nil && msg.operation == m.operation {
			if m.queueCancel != nil {
				m.queueCancel()
				m.queueCancel = nil
			}
			p := *m.pending
			p.checking, p.conflict, p.err = false, msg.exists, msg.err
			m.pending = &p
			m.choice = 0
			if p.auto && !p.conflict && p.err == nil {
				return m.startCopy(false)
			}
		}
	case queuePlanned:
		return m.acceptPlan(msg)
	case previewLoaded:
		m = m.applyPreview(msg)
	case deleteFinished:
		return m.finishDelete(msg)
	case editFinished:
		m.status = "Editor closed — saved changes synchronized"
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		m.panes[msg.pane].generation++
		m.panes[msg.pane].loading = true
		return m, m.list(msg.pane)
	case copyFinished:
		if m.transferCancel != nil {
			m.transferCancel()
			m.transferCancel = nil
		}
		m.job = &transferJob{source: msg.source, destination: msg.destination, state: "Completed"}
		if msg.err != nil {
			m.job.state = "Failed"
			m.job.err = msg.err
			if errors.Is(msg.err, context.Canceled) {
				m.job.state = "Canceled"
			}
		}
		m.status = m.job.state + ": " + msg.destination.String()
		var cmds []tea.Cmd
		for i := range m.panes {
			loc := msg.destination
			loc.Path = path.Dir(loc.Path)
			if m.panes[i].location == loc {
				m.panes[i].generation++
				m.panes[i].loading = true
				cmds = append(cmds, m.list(i))
			}
		}
		if m.queueIndex < len(m.queue) {
			m.queue[m.queueIndex] = *m.job
			m.queueIndex++
			next, cmd := m.nextQueued()
			m = next
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	case tea.MouseMsg:
		return m.mouse(msg)
	case tea.KeyMsg:
		key := msg.String()
		if m.rename != nil {
			return m.renameKey(msg)
		}
		if key == "ctrl+c" {
			m.cancelWork()
			return m, tea.Quit
		}
		if key == "ctrl+x" && !m.filtering && m.modal != "rename" && m.modal != "palette" {
			m.cancelWork()
			return m, nil
		}
		if key == "f1" && (m.modal == "" || m.modal == "palette" || m.modal == "help") {
			return m.execute("help")
		}
		if m.modal != "" {
			return m.modalKey(msg)
		}
		if m.filtering {
			p := &m.panes[m.active()]
			switch key {
			case "esc":
				p.filter = ""
				m.filtering = false
			case "enter":
				m.filtering = false
			case "backspace", "ctrl+h":
				p.filter = dropRune(p.filter)
			case " ":
				p.filter += " "
			default:
				if msg.Type == tea.KeyRunes {
					p.filter += string(msg.Runes)
				}
			}
			p.cursor = 0
			return m, nil
		}
		if key == "ctrl+p" {
			m.modal = "palette"
			m.query = ""
			m.choice = 0
			return m, nil
		}
		for _, a := range m.actions() {
			if a.shortcut == key && a.shortcut != "" {
				return m.execute(a.id)
			}
		}
		switch key {
		case "ctrl+h", "ctrl+l":
			m.focus = 1
			if key == "ctrl+l" {
				m.focus = 2
			}
			m.endpoint = m.focus - 1
		case "tab", "shift+tab":
			r := m.regions()
			idx := 0
			for i, v := range r {
				if v == m.focus {
					idx = i
				}
			}
			step := 1
			if key == "shift+tab" {
				step = len(r) - 1
			}
			m.focus = r[(idx+step)%len(r)]
			if m.focus == 1 || m.focus == 2 {
				m.endpoint = m.focus - 1
			}
		case "j", "down", "k", "up":
			delta := 1
			if key == "k" || key == "up" {
				delta = -1
			}
			if m.focus == 2 && m.disconnected {
				m.destinationChoice = max(0, min(1, m.destinationChoice+delta))
				return m, nil
			}
			if m.focus == 0 {
				m.navCursor = (m.navCursor + delta + len(m.locations)) % len(m.locations)
			}
			if m.focus == 4 {
				m.jobCursor = max(0, min(len(m.queue)-1, m.jobCursor+delta))
			}
			if m.focus == 1 || m.focus == 2 {
				p := &m.panes[m.active()]
				n := len(p.visible())
				if n > 0 {
					p.cursor = max(0, min(n-1, p.cursor+delta))
				}
			}
		case "enter":
			if m.focus == 2 && m.disconnected {
				if m.destinationChoice == 1 {
					return m.execute("folder")
				}
				return m.execute("connect")
			}
			if m.focus == 0 {
				loc := m.locations[m.navCursor]
				i := 1
				if loc.Remote == "" {
					i = 0
				}
				m.focus = i + 1
				m.endpoint = i
				cmd := m.navigate(i, loc)
				return m, cmd
			}
			if m.focus == 1 || m.focus == 2 {
				i := m.active()
				p := m.panes[i]
				entries := p.visible()
				if len(entries) > 0 && entries[p.cursor].IsDir {
					loc := p.location
					loc.Path = path.Join(loc.Path, entries[p.cursor].Name)
					cmd := m.navigate(i, loc)
					return m, cmd
				}
				if len(entries) > 0 {
					m.inspector = true
				}
			}
		case "backspace", "h":
			if m.focus == 1 || (m.focus == 2 && !m.disconnected) {
				i := m.active()
				loc := m.panes[i].location
				loc.Path = path.Dir(loc.Path)
				cmd := m.navigate(i, loc)
				return m, cmd
			}
		case " ":
			if m.focus == 1 || m.focus == 2 {
				p := &m.panes[m.active()]
				entries := p.visible()
				if len(entries) > 0 && entries[p.cursor].Name != ".." {
					name := entries[p.cursor].Name
					p.selected[name] = !p.selected[name]
				}
			}
		case "/":
			if m.focus == 1 || (m.focus == 2 && !m.disconnected) {
				m.filtering = true
			}
		}
	}
	return m, nil
}

func dropRune(s string) string {
	r := []rune(s)
	if len(r) > 0 {
		return string(r[:len(r)-1])
	}
	return ""
}

var navLocations = []storage.Location{
	{Remote: "demo-ftp", Path: "/incoming"}, {Remote: "demo-ftp", Path: "/archive"},
	{Remote: "demo-blob", Path: "/assets"}, {Path: "/project"}, {Path: "/downloads"},
}

type action struct{ id, label, shortcut, keywords, disabled string }

func (m Model) actions() []action {
	direction := "right"
	if m.active() == 1 {
		direction = "left"
	}
	a := []action{
		{"copy", "Copy to " + direction + " pane", "c", "upload download transfer", m.copyDisabled()},
		{"preview", "Preview selected file", "p", "inspect text", m.previewDisabled()},
		{"edit", "Edit selected file in Neovim", "e", "editor nvim save remote", m.editDisabled()},
		{"navigation", "Toggle navigation sidebar", "[", "connections places", ""},
		{"inspector", "Toggle file inspector", "]", "metadata details sidebar", ""},
		{"jobs", "Toggle transfer drawer", "t", "queue jobs", ""},
		{"refresh", "Refresh focused filesystem", "ctrl+r", "reload browse", ""},
		{"icons", "Toggle filetype icons", "", "glyphs nerd font", ""},
		{"help", "Keyboard shortcuts", "?", "help keys", ""},
		{"delete", "Delete selected file", "D", "remove file", m.deleteDisabled()},
		{"file-rename", "Rename selected file", "R", "name move filename", m.renameDisabled()},
		{"cancel", "Cancel unfinished transfers", "ctrl+x", "stop queue", m.cancelDisabled()},
		{"retry", "Retry unfinished transfers", "r", "recover failed canceled queue", m.retryDisabled()},
	}
	if m.transferer != nil {
		a = append(a, action{"connect", "Connect a remote", "", "destination connection ftp blob s3", m.connectionDisabled()},
			action{"folder", "Open local destination folder", "", "destination directory path", m.connectionDisabled()})
	}
	for _, name := range theme.Names() {
		a = append(a, action{"theme:" + name, "Theme: " + name, "", "colors style appearance", ""})
	}
	a = append(a, action{"quit", "Quit workspace", "q", "exit close", ""})
	return a
}

func (m Model) matches() []action {
	a := m.actions()
	if m.query == "" {
		return a
	}
	labels := make([]string, len(a))
	for i, v := range a {
		labels[i] = v.label
	}
	var found []action
	for _, match := range fuzzy.Find(m.query, labels) {
		found = append(found, a[match.Index])
	}
	if len(found) == 0 {
		for i, v := range a {
			labels[i] = v.keywords
		}
		for _, match := range fuzzy.Find(m.query, labels) {
			found = append(found, a[match.Index])
		}
	}
	return found
}

func (m Model) modalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal == "delete" {
		return m.deleteKey(msg)
	}
	if m.modal == "folder" {
		return m.folderKey(msg)
	}
	if m.modal == "preview" {
		return m.previewKey(msg)
	}
	if m.modal == "batch" {
		return m.batchKey(msg)
	}
	if m.modal == "copy" || m.modal == "rename" {
		return m.copyKey(msg)
	}
	key := msg.String()
	if key == "esc" || key == "ctrl+p" {
		m.modal = ""
		return m, nil
	}
	if m.modal == "help" {
		switch key {
		case "up", "k":
			m.choice = max(0, m.choice-1)
		case "down", "j":
			m.choice = min(max(0, len(shortcutLines())-(min(20, m.height-4)-3)), m.choice+1)
		}
		return m, nil
	}
	switch key {
	case "down", "ctrl+n":
		if n := len(m.matches()); n > 0 {
			m.choice = (m.choice + 1) % n
		}
	case "up", "ctrl+k":
		if n := len(m.matches()); n > 0 {
			m.choice = (m.choice + n - 1) % n
		}
	case "enter":
		a := m.matches()
		if len(a) > 0 {
			chosen := a[min(m.choice, len(a)-1)]
			if chosen.disabled != "" {
				m.status = chosen.disabled
				return m, nil
			}
			m.modal = ""
			return m.execute(chosen.id)
		}
	case "backspace", "ctrl+h":
		m.query = dropRune(m.query)
		m.choice = 0
	case " ":
		m.query += " "
		m.choice = 0
	default:
		if msg.Type == tea.KeyRunes {
			m.query += string(msg.Runes)
			m.choice = 0
		}
	}
	return m, nil
}

func (m Model) execute(id string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(id, "theme:") {
		t, err := theme.Get(strings.TrimPrefix(id, "theme:"))
		if err == nil {
			m.theme = t
			m.status = "Theme: " + t.Name
		}
		return m, nil
	}
	switch id {
	case "connect":
		if reason := m.connectionDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		local := m.panes[0].location.Path
		return m, func() tea.Msg { return ConnectRequested{Local: local, Destination: true} }
	case "folder":
		if reason := m.connectionDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		m.folderPath = m.panes[0].location.Path
		m.modal = "folder"
		m.status = ""
	case "navigation":
		m.navigation = !m.navigation
		m.fixFocus()
	case "inspector":
		m.inspector = !m.inspector
		m.fixFocus()
	case "jobs":
		m.jobs = !m.jobs
		m.fixFocus()
	case "icons":
		m.icons = !m.icons
		m.status = fmt.Sprintf("Filetype icons: %t — enable with a Nerd Font", m.icons)
	case "refresh":
		i := m.active()
		m.panes[i].generation++
		m.panes[i].loading = true
		return m, m.list(i)
	case "help":
		m.modal = "help"
		m.choice = 0
	case "quit":
		m.cancelWork()
		return m, tea.Quit
	case "cancel":
		m.cancelWork()
		return m, nil
	case "retry":
		return m.retryQueue()
	case "copy":
		if reason := m.copyDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		if entries := m.copyEntries(); len(entries) != 1 || entries[0].IsDir {
			return m.planQueue(entries)
		}
		m.queue, m.queueIndex = nil, 0
		i := m.active()
		e, _ := m.copyEntry()
		source, destination := m.panes[i].location, m.panes[1-i].location
		source.Path = path.Join(source.Path, e.Name)
		destination.Path = path.Join(destination.Path, e.Name)
		m.pending = &copyState{source: source, destination: destination, size: e.Size}
		return m.checkDestination()
	case "delete":
		return m.confirmDelete()
	case "preview":
		return m.startPreview()
	case "edit":
		return m.startEdit()
	case "file-rename":
		return m.startRename()
	}
	return m, nil
}

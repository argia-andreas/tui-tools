package ui

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
	"lazyftp/internal/theme"
	"terminalui"
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
	location     storage.Location
	entries      []storage.Entry
	cursor       int
	selected     map[string]bool
	filter       string
	generation   int
	loading      bool
	err          error
	parent       bool
	revealName   string
	detached     bool
	sourceChoice int
	lastLocal    string
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
	focus                       int // 0 navigation, 1 left, 2 right, 3 inspector, 4 jobs
	controlFocus                string
	endpoint                    int // last focused file pane, retained while inspecting
	navigation, inspector, jobs bool
	modal                       string
	palette                     terminalui.Palette
	help                        bool
	helpOffset                  int
	query                       string // Existing copy-destination rename field, not palette state.
	choice                      int
	filtering                   bool
	status                      string
	navCursor                   int
	folderPath                  string
	folderPane                  int
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
	if o.Disconnected {
		m.navigation = false
		if len(o.Locations) == 0 {
			m.locations = []storage.Location{o.Left}
		}
	}
	m.panes[0] = pane{location: o.Left, selected: map[string]bool{}, loading: true, parent: o.Transferer != nil, lastLocal: o.Left.Path}
	m.panes[1] = pane{location: o.Right, selected: map[string]bool{}, loading: !o.Disconnected, detached: o.Disconnected, parent: o.Transferer != nil, lastLocal: o.Left.Path}
	for i := range m.panes {
		if m.panes[i].location.Remote == "" && m.panes[i].location.Path != "" {
			m.panes[i].lastLocal = m.panes[i].location.Path
		}
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
	if m.panes[i].detached {
		return nil
	}
	p := m.panes[i]
	return func() tea.Msg {
		entries, err := m.browser.List(m.ctx, p.location)
		return listed{pane: i, generation: p.generation, entries: entries, err: err}
	}
}

func (m *Model) navigate(i int, loc storage.Location) tea.Cmd {
	m.controlFocus = ""
	p := &m.panes[i]
	p.detached = false
	if loc.Remote == "" {
		p.lastLocal = loc.Path
	}
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
	return !p.detached && p.parent && p.location.Path != "" && path.Clean(p.location.Path) != "/"
}

func (p pane) visible() []storage.Entry {
	if p.detached {
		return nil
	}
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
	if m.controlFocus != "" {
		visible := false
		frame := m.interactionFrame()
		if m.rename != nil && !m.rename.confirming {
			frame = m.inlineRenameFrame()
		}
		for _, r := range frame.Regions {
			if r.ID == m.controlFocus {
				visible = true
				break
			}
		}
		if !visible {
			m.controlFocus = ""
		}
	}
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
		m.focus, m.endpoint = msg.Pane+1, msg.Pane
		return m.execute("folder")
	case ConnectionOpened:
		if reason := m.connectionDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		if msg.Browser != nil {
			m.browser = msg.Browser
		}
		if msg.Transferer != nil {
			m.transferer = msg.Transferer
		}
		m.status = ""
		m.focus, m.endpoint = msg.Pane+1, msg.Pane
		locations := append([]storage.Location{msg.Location}, msg.Locations...)
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
		if msg.Local != "" && (storage.Location{Path: msg.Local}) != m.panes[0].location {
			local = m.navigate(0, storage.Location{Path: msg.Local})
		}
		return m, tea.Batch(local, m.navigate(msg.Pane, msg.Location))
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.fixFocus()
	case listed:
		p := &m.panes[msg.pane]
		if !p.detached && p.generation == msg.generation {
			name := ""
			if entries := p.visible(); p.cursor >= 0 && p.cursor < len(entries) {
				name = entries[p.cursor].Name
			}
			p.entries, p.err, p.loading = msg.entries, msg.err, false
			if name != "" {
				p.cursor = 0
				for i, e := range p.visible() {
					if e.Name == name {
						p.cursor = i
						break
					}
				}
			}
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
		if m.panes[msg.pane].detached || m.panes[msg.pane].generation != msg.generation {
			return m, nil
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
			if !m.panes[i].detached && m.panes[i].location == loc {
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
		if !m.workspaceFits() {
			return m, nil
		}
		if m.help {
			return m.helpInput(msg)
		}
		if msg.Y == 0 && (m.modal != "" || m.rename != nil) {
			result := terminalui.HandleActionBar(msg, m.barActions(), m.actionBarFrame(), "")
			if result.Intent != nil && result.Intent.ID == "help" {
				return m.execute("help")
			}
			if result.Consumed && m.modal != "palette" {
				return m, nil
			}
		}
		if m.modal == "palette" {
			return m.paletteInput(msg)
		}
		return m.mouse(msg)
	case tea.KeyMsg:
		if !m.workspaceFits() {
			if msg.Type == tea.KeyCtrlC {
				if m.rename != nil {
					return m.renameKey(msg)
				}
				m.cancelWork()
				return m, tea.Quit
			}
			if msg.String() == "esc" {
				switch {
				case m.help:
					return m.helpInput(msg)
				case m.modal == "palette":
					return m.paletteInput(msg)
				case m.rename != nil:
					return m.renameKey(msg)
				case m.modal != "":
					return m.surfaceInput(msg)
				case m.filtering:
					m.panes[m.active()].filter, m.filtering = "", false
				}
			}
			return m, nil
		}
		if msg.String() == "f1" {
			return m.execute("help")
		}
		if m.help {
			if msg.Type == tea.KeyCtrlC {
				if m.rename != nil {
					return m.renameKey(msg)
				}
				m.cancelWork()
				return m, tea.Quit
			}
			return m.helpInput(msg)
		}
		if m.modal == "palette" {
			// FTP's interrupt still cancels work and quits. Other palette input
			// stays captured; the shared component does not own app cleanup.
			if msg.Type == tea.KeyCtrlC {
				m.cancelWork()
				return m, tea.Quit
			}
			return m.paletteInput(msg)
		}
		key := msg.String()
		if m.rename != nil {
			if m.rename.confirming && !m.rename.running && key != "ctrl+c" {
				return m.surfaceInput(msg)
			}
			return m.renameKey(msg)
		}
		if key == "ctrl+c" {
			m.cancelWork()
			return m, tea.Quit
		}
		if key == "ctrl+x" && !m.filtering && m.modal != "rename" {
			m.cancelWork()
			return m, nil
		}
		if m.modal != "" {
			return m.surfaceInput(msg)
		}
		if m.filtering {
			if key == "tab" || key == "shift+tab" || (key == "enter" && (m.controlFocus == "filter-apply" || m.controlFocus == "filter-clear")) {
				return m.navigationInput(msg)
			}
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
			return m.execute("palette")
		}
		for _, a := range m.actions() {
			for _, shortcut := range a.Keys {
				if shortcut == key {
					return m.execute(a.ID)
				}
			}
		}
		if key == "tab" || key == "shift+tab" || (key == "enter" && !m.panes[m.active()].detached) {
			return m.navigationInput(msg)
		}
		switch key {
		case "ctrl+h", "ctrl+l":
			m.controlFocus = ""
			m.focus = 1
			if key == "ctrl+l" {
				m.focus = 2
			}
			m.endpoint = m.focus - 1
		case "j", "down", "k", "up":
			m.controlFocus = ""
			delta := 1
			if key == "k" || key == "up" {
				delta = -1
			}
			if (m.focus == 1 || m.focus == 2) && m.panes[m.active()].detached {
				p := &m.panes[m.active()]
				p.sourceChoice = max(0, min(len(sourceButtons)-1, p.sourceChoice+delta))
				return m, nil
			}
			if m.focus == 0 && len(m.locations) > 0 {
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
			if m.controlFocus != "" {
				return m.navigationInput(msg)
			}
			return m.openSelection()
		case "backspace", "h":
			return m.execute("parent")
		case " ":
			return m.toggleMark()
		case "/":
			return m.execute("filter")
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

func (m Model) actions() []terminalui.Action {
	direction := "right"
	if m.active() == 1 {
		direction = "left"
	}
	a := []terminalui.Action{
		{ID: "open", Label: "Open highlighted entry", Keywords: "directory inspect"},
		{ID: "copy", Label: "Copy to " + direction + " pane", Keys: []string{"c"}, Keywords: "upload download transfer", Unavailable: m.copyDisabled()},
		{ID: "preview", Label: "Preview selected file", Keys: []string{"p"}, Keywords: "inspect text", Unavailable: m.previewDisabled()},
		{ID: "edit", Label: "Edit selected file in Neovim", Keys: []string{"e"}, Keywords: "editor nvim save remote", Unavailable: m.editDisabled()},
		{ID: "navigation", Label: "Toggle navigation sidebar", Keys: []string{"["}, Keywords: "connections places"},
		{ID: "inspector", Label: "Toggle file inspector", Keys: []string{"]"}, Keywords: "inspect metadata details sidebar"},
		{ID: "jobs", Label: "Toggle transfer drawer", Keys: []string{"t"}, Keywords: "queue jobs"},
		{ID: "refresh", Label: "Refresh focused filesystem", Keys: []string{"ctrl+r"}, Keywords: "reload browse"},
		{ID: "icons", Label: "Toggle filetype icons", Keywords: "glyphs nerd font"},
		{ID: "help", Label: "Keyboard shortcuts", Keys: []string{"f1", "?"}, Keywords: "help keys"},
		{ID: "delete", Label: "Delete selected file", Keys: []string{"D"}, Keywords: "remove file", Unavailable: m.deleteDisabled(), Dangerous: true},
		{ID: "file-rename", Label: "Rename selected file", Keys: []string{"R"}, Keywords: "name move filename", Unavailable: m.renameDisabled()},
		{ID: "cancel", Label: "Cancel unfinished transfers", Keys: []string{"ctrl+x"}, Keywords: "stop queue", Unavailable: m.cancelDisabled()},
		{ID: "retry", Label: "Retry unfinished transfers", Keys: []string{"r"}, Keywords: "recover failed canceled queue", Unavailable: m.retryDisabled()},
	}
	if m.transferer != nil {
		target := strings.ToLower(paneName(m.active())) + " pane"
		detachReason := m.connectionDisabled()
		if m.panes[m.active()].detached {
			detachReason = "Pane has no source"
		}
		a = append(a, terminalui.Action{ID: "connect", Label: "Saved connection in " + target, Keywords: "connect remote ftp blob s3", Unavailable: m.connectionDisabled()},
			terminalui.Action{ID: "new", Label: "New connection in " + target, Keywords: "setup remote ftp blob s3", Unavailable: m.connectionDisabled()},
			terminalui.Action{ID: "folder", Label: "Open local folder in " + target, Keywords: "destination directory path", Unavailable: m.connectionDisabled()},
			terminalui.Action{ID: "detach", Label: "Detach " + target, Keywords: "disconnect source close", Unavailable: detachReason})
	}
	for _, name := range theme.Names() {
		a = append(a, terminalui.Action{ID: "theme:" + name, Label: "Theme: " + name, Keywords: "colors style appearance"})
	}
	a = append(a, terminalui.Action{ID: "quit", Label: "Quit workspace", Keys: []string{"q"}, Keywords: "exit close"})
	return a
}

func (m Model) paletteInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	result := m.palette.Handle(msg, m.actions(), m.paletteFrame())
	if result.Intent != nil {
		switch result.Intent.Kind {
		case terminalui.Dismiss:
			m.modal = ""
		case terminalui.Help:
			return m.execute("help")
		case terminalui.Activate:
			if result.Intent.ID == "help" {
				return m.execute("help")
			}
			m.modal = ""
			return m.execute(result.Intent.ID)
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
	case "filter-apply", "filter-clear":
		if m.filtering {
			if id == "filter-clear" {
				m.panes[m.active()].filter = ""
			}
			m.filtering, m.controlFocus = false, ""
		}
		return m, nil
	case "open":
		return m.openSelection()
	case "mark":
		return m.toggleMark()
	case "parent":
		if (m.focus == 1 || m.focus == 2) && !m.panes[m.active()].detached {
			i := m.active()
			loc := m.panes[i].location
			loc.Path = path.Dir(loc.Path)
			m.controlFocus = ""
			return m, m.navigate(i, loc)
		}
		return m, nil
	case "filter":
		if (m.focus == 1 || m.focus == 2) && !m.panes[m.active()].detached {
			m.filtering = true
		}
		return m, nil
	case "palette":
		if m.modal == "" && !m.filtering && m.rename == nil && m.workspaceFits() {
			m.modal = "palette"
			m.palette = terminalui.Palette{}
		}
		return m, nil
	case "connect", "new":
		if reason := m.connectionDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		i, local := m.active(), m.panes[m.active()].lastLocal
		return m, func() tea.Msg { return ConnectRequested{Local: local, Destination: true, Pane: i, New: id == "new"} }
	case "detach":
		if reason := m.connectionDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		i := m.active()
		p := m.panes[i]
		m.panes[i] = pane{detached: true, generation: p.generation + 1, lastLocal: p.lastLocal, parent: p.parent, selected: map[string]bool{}}
		if m.previewCancel != nil {
			m.previewCancel()
			m.previewCancel = nil
		}
		m.previewGeneration++
		m.preview, m.filtering = nil, false
		m.focus, m.endpoint = i+1, i
		m.status = strings.ToLower(paneName(i)) + " pane detached"
	case "folder":
		if reason := m.connectionDisabled(); reason != "" {
			m.status = reason
			return m, nil
		}
		m.folderPane, m.folderPath = m.active(), m.panes[m.active()].lastLocal
		m.modal = "folder"
		m.choice = 0
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
		if m.panes[i].detached {
			m.status = "Choose a source before refreshing"
			return m, nil
		}
		m.panes[i].generation++
		m.panes[i].loading = true
		return m, m.list(i)
	case "help":
		m.help = !m.help
		m.helpOffset = 0
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

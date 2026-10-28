package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/connections"
	"lazyftp/internal/theme"
	"terminalui"
)

// Session files can outlive a dismissed wizard or an in-flight command. The
// registry closes late arrivals too, so quitting cannot orphan a private config.
type connectionSessions struct {
	mu     sync.Mutex
	closed bool
	items  map[*connections.Candidate]bool
}

func (s *connectionSessions) add(c *connections.Candidate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = c.Close()
		return
	}
	if s.items == nil {
		s.items = map[*connections.Candidate]bool{}
	}
	s.items[c] = true
}

func (s *connectionSessions) release(c *connections.Candidate) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		return
	} // Keep ownership for another cleanup attempt at exit.
	s.mu.Lock()
	delete(s.items, c)
	s.mu.Unlock()
}

func (s *connectionSessions) Close() {
	s.mu.Lock()
	s.closed = true
	items := s.items
	s.items = nil
	s.mu.Unlock()
	for c := range items {
		_ = c.Close()
	}
}

type wizardField struct {
	key, label string
	secret     bool
	options    []string
}

type connectionWizard struct {
	ctx                              context.Context
	theme                            theme.Theme
	local, source, target            string
	destination                      bool
	pane                             int
	sessions                         *connectionSessions
	values                           map[string]string
	kind, step, focus, width, height int
	remember, busy, verified         bool
	dismissed                        bool
	help                             bool
	helpOffset                       int
	message                          string
	messageScroll                    int
	request                          int
	cancel                           context.CancelFunc
	candidate                        *connections.Candidate
}

type wizardTested struct {
	owner     *connectionWizard
	request   int
	candidate *connections.Candidate
	err       error
}
type wizardConnected struct {
	owner     *connectionWizard
	request   int
	candidate *connections.Candidate
	err       error
}

func newWizard(ctx context.Context, t theme.Theme, local, source, target string, sessions *connectionSessions, request int) *connectionWizard {
	if source == target {
		source = ""
	} // The persistent config may not exist yet.
	return &connectionWizard{ctx: ctx, theme: t, local: local, source: source, target: target, sessions: sessions, request: request,
		width: 80, height: 24, values: map[string]string{"security": "Explicit FTPS", "port": "21", "root": "/", "azure_auth": "Azure CLI session", "s3_provider": "AWS", "s3_auth": "Profile / environment", "region": "us-east-1"}}
}

// Formatting a model must never dump its raw input map, including with %#v.
func (w connectionWizard) String() string   { return w.View() }
func (w connectionWizard) GoString() string { return w.View() }

func (w *connectionWizard) fields() []wizardField {
	f := []wizardField{{key: "name", label: "Connection name"}}
	switch w.kind {
	case 0:
		f = append(f, wizardField{key: "host", label: "Host"}, wizardField{key: "port", label: "Port"}, wizardField{key: "user", label: "User"}, wizardField{key: "pass", label: "Password", secret: true}, wizardField{key: "security", label: "Security", options: []string{"Explicit FTPS", "Implicit FTPS", "Insecure FTP"}}, wizardField{key: "root", label: "Remote root"})
	case 1:
		f = append(f, wizardField{key: "account", label: "Storage account"}, wizardField{key: "azure_auth", label: "Authentication", options: []string{"Azure CLI session", "Managed identity", "SAS URL", "Account key"}})
		switch w.values["azure_auth"] {
		case "SAS URL":
			f = append(f, wizardField{key: "sas_url", label: "SAS URL", secret: true})
		case "Account key":
			f = append(f, wizardField{key: "key", label: "Account key", secret: true})
		}
		f = append(f, wizardField{key: "root", label: "Container / prefix"})
	case 2:
		f = append(f, wizardField{key: "s3_provider", label: "Service", options: []string{"AWS", "S3-compatible"}}, wizardField{key: "s3_auth", label: "Credentials", options: []string{"Profile / environment", "Access keys"}})
		if w.values["s3_auth"] == "Access keys" {
			f = append(f, wizardField{key: "access_key_id", label: "Access key ID", secret: true}, wizardField{key: "secret_access_key", label: "Secret access key", secret: true})
		} else {
			f = append(f, wizardField{key: "profile", label: "Profile (optional)"})
		}
		f = append(f, wizardField{key: "region", label: "Region"})
		if w.values["s3_provider"] == "S3-compatible" {
			f = append(f, wizardField{key: "endpoint", label: "Endpoint URL"})
		}
		f = append(f, wizardField{key: "root", label: "Bucket / prefix"})
	}
	return f
}

func (w *connectionWizard) draft() connections.Draft {
	d := connections.Draft{Name: w.values["name"], SourceConfig: w.source, Values: map[string]string{}}
	copyKeys := func(keys ...string) {
		for _, k := range keys {
			if v := w.values[k]; v != "" {
				d.Values[k] = v
			}
		}
	}
	switch w.kind {
	case 0:
		d.Backend = "ftp"
		copyKeys("host", "port", "user", "pass")
		d.Values["tls"], d.Values["explicit_tls"] = "false", "false"
		if w.values["security"] == "Explicit FTPS" {
			d.Values["explicit_tls"] = "true"
		}
		if w.values["security"] == "Implicit FTPS" {
			d.Values["tls"] = "true"
		}
	case 1:
		d.Backend = "azureblob"
		copyKeys("account")
		switch w.values["azure_auth"] {
		case "Azure CLI session":
			d.Values["use_az"] = "true"
		case "Managed identity":
			d.Values["use_msi"] = "true"
		case "SAS URL":
			copyKeys("sas_url")
		case "Account key":
			copyKeys("key")
		}
	case 2:
		d.Backend = "s3"
		d.Values["provider"] = "AWS"
		if w.values["s3_provider"] == "S3-compatible" {
			d.Values["provider"] = "Other"
			d.Values["force_path_style"] = "true"
			copyKeys("endpoint")
		}
		copyKeys("region")
		if w.values["s3_auth"] == "Access keys" {
			d.Values["env_auth"] = "false"
			copyKeys("access_key_id", "secret_access_key")
		} else {
			d.Values["env_auth"] = "true"
			copyKeys("profile")
		}
	}
	return d
}

func (w *connectionWizard) root() string { return "/" + strings.TrimPrefix(w.values["root"], "/") }

func (w *connectionWizard) invalidate() {
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	w.request++
	w.verified, w.busy = false, false
	w.messageScroll = 0
	w.sessions.release(w.candidate)
	w.candidate = nil
}

func (w *connectionWizard) test() tea.Cmd {
	w.invalidate()
	w.step, w.focus, w.busy = 2, 0, true
	w.message = "Testing a read-only listing…"
	ctx, cancel := context.WithTimeout(w.ctx, 45*time.Second)
	w.cancel = cancel
	draft, root, target, sessions, request := w.draft(), w.root(), w.target, w.sessions, w.request
	return func() tea.Msg {
		defer cancel()
		if _, err := exec.LookPath("rclone"); err != nil {
			return wizardTested{owner: w, request: request, err: fmt.Errorf("Install rclone (or run ./.agents/setup), then test the connection again.")}
		}
		if draft.Backend == "azureblob" && draft.Values["use_az"] == "true" {
			if _, err := exec.LookPath("az"); err != nil {
				return wizardTested{owner: w, request: request, err: fmt.Errorf("Azure CLI is not installed. Install az and run az login outside this app, then retry; or choose another authentication method.")}
			}
		}
		c, err := connections.Prepare(ctx, target, draft)
		if err == nil {
			sessions.add(c)
			err = c.Test(ctx, root)
			if err != nil && draft.Backend == "azureblob" && draft.Values["use_az"] == "true" {
				err = fmt.Errorf("Listing failed. Run az login outside this app and check the account, root and storage permissions, then test again.")
			}
		}
		if err != nil || ctx.Err() != nil {
			sessions.release(c)
			if err == nil {
				err = fmt.Errorf("Connection test canceled. Test again before connecting.")
			}
			return wizardTested{owner: w, request: request, err: err}
		}
		return wizardTested{owner: w, request: request, candidate: c}
	}
}

func (w *connectionWizard) connect() tea.Cmd {
	w.messageScroll = 0
	if !w.verified || w.candidate == nil || w.busy {
		w.message = "Test these details successfully before connecting."
		return nil
	}
	w.busy = true
	ctx, cancel := context.WithCancel(w.ctx)
	w.cancel = cancel
	c, remember, request := w.candidate, w.remember, w.request
	w.message = "Opening connection…"
	return func() tea.Msg {
		defer cancel()
		var err error
		if remember {
			err = c.Save(ctx)
		}
		if err == nil {
			err = ctx.Err()
		}
		return wizardConnected{owner: w, request: request, candidate: c, err: err}
	}
}

func (w *connectionWizard) count() int {
	if w.step == 0 {
		return 4
	}
	if w.step == 1 {
		return len(w.fields()) + 2
	}
	return 4
}

func (w *connectionWizard) back() {
	opening := w.busy && w.verified
	w.invalidate()
	switch w.step {
	case 0:
		w.dismissed = true
	case 1:
		w.step, w.focus = 0, w.kind
		w.message = ""
	case 2:
		w.step, w.focus = 1, 0
		w.message = "Test canceled. Review details and test again."
		if opening {
			w.message = "Opening canceled. Any completed explicit save is not undone."
		}
	}
}

func (w *connectionWizard) activate() tea.Cmd {
	if w.busy {
		return nil
	}
	if w.step == 1 && w.focus < len(w.fields()) {
		fields := w.fields()
		if len(fields[w.focus].options) > 0 {
			w.cycle(1)
		} else {
			w.focus++
		}
		return nil
	}
	return w.executeWizard(w.focusID())
}

func (w *connectionWizard) cycle(delta int) {
	f := w.fields()[w.focus]
	for i, v := range f.options {
		if v == w.values[f.key] {
			w.invalidate()
			w.values[f.key] = f.options[(i+delta+len(f.options))%len(f.options)]
			if f.key == "security" && (w.values["port"] == "21" || w.values["port"] == "990") {
				w.values["port"] = "21"
				if w.values["security"] == "Implicit FTPS" {
					w.values["port"] = "990"
				}
			}
			return
		}
	}
}

func (w *connectionWizard) input(msg tea.Msg) tea.Cmd {
	spec, frame := w.surface()
	if w.help {
		r := terminalui.HandleHelp(msg, spec.Actions, w.helpTopics(), &w.helpOffset, w.helpFrame())
		if r.Intent != nil && r.Intent.Kind == terminalui.Dismiss {
			w.help = false
		}
		return nil
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "f1" {
		w.help = true
		return nil
	}
	route := false
	switch k := msg.(type) {
	case tea.MouseMsg:
		route = true
	case tea.KeyMsg:
		route = k.String() == "esc" || k.String() == "tab" || k.String() == "shift+tab" || (k.String() == "enter" && (w.step != 1 || w.focus >= len(w.fields())))
	}
	if route {
		r := terminalui.HandleSurface(msg, spec, frame)
		if r.Intent != nil {
			switch r.Intent.Kind {
			case terminalui.Dismiss:
				w.back()
			case terminalui.Help:
				w.help = true
			case terminalui.Focus:
				w.setFocus(r.Intent.ID)
			case terminalui.Activate:
				return w.executeWizard(r.Intent.ID)
			}
		}
		return nil
	}
	if w.busy || w.width < 60 || w.height < 22 {
		return nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if k.String() == "pgdown" || k.String() == "pgup" {
		if k.String() == "pgdown" {
			w.messageScroll++
		} else {
			w.messageScroll = max(0, w.messageScroll-1)
		}
		return nil
	}
	key := k.String()
	if w.step == 0 {
		switch key {
		case "j":
			key = "down"
		case "k":
			key = "up"
		}
	}
	switch key {
	case "tab", "down":
		w.setFocus(terminalui.NextFocus(spec.FocusOrder, w.focusID(), false))
	case "shift+tab", "up":
		w.setFocus(terminalui.NextFocus(spec.FocusOrder, w.focusID(), true))
	case "enter":
		if w.width >= 60 && w.height >= 22 {
			return w.activate()
		}
	case "left", "right":
		if w.step == 1 && w.focus < len(w.fields()) {
			delta := 1
			if k.String() == "left" {
				delta = -1
			}
			w.cycle(delta)
		}
	case " ":
		if w.step == 2 && w.focus == 2 {
			return w.executeWizard("remember")
		}
		fallthrough
	default:
		if w.step != 1 || w.focus >= len(w.fields()) {
			return nil
		}
		f := w.fields()[w.focus]
		if len(f.options) > 0 {
			return nil
		}
		value := w.values[f.key]
		switch k.String() {
		case "ctrl+u":
			value = ""
		case "backspace":
			r := []rune(value)
			if len(r) > 0 {
				value = string(r[:len(r)-1])
			}
		default:
			if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
				for _, r := range k.Runes {
					if !unicode.IsControl(r) {
						value += string(r)
					}
				}
				if k.Type == tea.KeySpace {
					value += " "
				}
			}
		}
		if value != w.values[f.key] {
			w.invalidate()
			w.values[f.key] = value
		}
	}
	return nil
}

type wizardRow struct {
	text  string
	focus int
}

func (w *connectionWizard) rows() []wizardRow {
	width := min(76, w.width)
	inner := max(54, width-6)
	compact := w.height < 28
	heading := "New connection"
	if w.destination {
		heading += " · " + paneHeading(w.pane) + " pane"
	}
	rows := []wizardRow{{"", -1}, {heading, -1}, {"", -1}}
	progress := []string{"○ Type", "○ Details", "○ Test"}
	for i := range progress {
		if i < w.step {
			progress[i] = strings.Replace(progress[i], "○", "✓", 1)
		}
		if i == w.step {
			progress[i] = strings.Replace(progress[i], "○", "●", 1)
		}
	}
	rows = append(rows, wizardRow{strings.Join(progress, "  →  "), -1}, wizardRow{"", -1})
	add := func(s string, focus int) { rows = append(rows, wizardRow{s, focus}) }
	note := func(s string) {
		for _, line := range strings.Split(ansi.Hardwrap(s, inner, true), "\n") {
			add(line, -1)
		}
	}
	if w.step == 0 {
		descriptions := []string{"Connect to an FTP or FTPS server", "Microsoft Azure object storage", "Amazon S3 or compatible storage"}
		for i, s := range []string{"⇄  FTP / FTPS", "☁  Azure Blob Storage", "▱  S3 — AWS or compatible"} {
			add(s+"  ›", i)
			add("   "+descriptions[i], i)
			add("", -1)
		}
		note("No settings are saved yet.")
	} else if w.step == 1 {
		for i, f := range w.fields() {
			v := w.values[f.key]
			if f.secret && v != "" {
				v = strings.Repeat("•", min(12, len([]rune(v))))
			}
			if len(f.options) > 0 {
				v = "‹ " + v + " ›"
			}
			valueWidth := inner - 25
			if v == "" {
				v = "·"
			}
			v = ansi.Truncate(v, valueWidth, "…")
			add(fmt.Sprintf("%-20s │ %s%s │", f.label, v, strings.Repeat(" ", valueWidth-ansi.StringWidth(v))), i)
			if !compact {
				add("", -1)
			}
		}
		add("", -1)
		note("Read-only test · Nothing saved yet.")
		if w.kind == 0 && w.values["security"] == "Insecure FTP" {
			note("Warning: FTP sends credentials and files unencrypted.")
		}
		if w.kind == 1 && w.values["azure_auth"] == "Azure CLI session" {
			note("Uses an existing az login session; no browser sign-in here.")
		}
	} else {
		switch {
		case w.busy:
			add("◌ Read-only test / connection in progress", -1)
		case w.verified:
			add("✓ Ready to connect · Read access verified", -1)
		default:
			add("! Could not verify connection", -1)
		}
		add("", -1)
		add("Connection: "+w.values["name"], -1)
		add("Root: "+w.root(), -1)
		note("Session temp config is removed on clean exit.")
		add("", -1)
		check := "[ ]"
		if w.remember {
			check = "[x]"
		}
		add(check+" Remember connection and credentials", 2)
		note("Private rclone config file, not a keychain; not encrypted.")
		add("", -1)
	}
	add("", -1)
	message := strings.Fields(w.message)
	var status []string
	if len(message) > 0 {
		status = strings.Split(ansi.Hardwrap(strings.Join(message, " "), inner, true), "\n")
	}
	// Collapse decorative space before sacrificing any controls or status help.
	budget := w.height - 3
	for len(rows)+len(status)+1 > budget {
		removed := false
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i].text == "" {
				rows = append(rows[:i], rows[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			break
		}
	}
	available := max(1, budget-len(rows)-1)
	help := "Tab/↑↓ focus · Enter select · Esc back · F1 help"
	if len(status) > available {
		start := min(w.messageScroll, len(status)-available)
		status = status[start : start+available]
		help = "PgUp/PgDn status · Tab focus · Esc back · F1 help"
	}
	for _, line := range status {
		add(line, -1)
	}
	add(help, -1)
	return rows
}

func (w *connectionWizard) View() string {
	if w.width < 60 || w.height < 22 {
		return "Resize to at least 60 columns × 22 rows. Esc cancels."
	}
	f := w.surfaceFrame()
	if w.help {
		f = w.helpFrame()
	}
	return strings.Join(f.Lines, "\n")
}

func (w *connectionWizard) focusID() string {
	return w.focusIDAt(w.focus)
}

func (w *connectionWizard) focusIDAt(focus int) string {
	if focus == 101 {
		return "help"
	}
	if w.step == 0 {
		return []string{"ftp", "azure", "s3", "back"}[min(3, max(0, focus))]
	}
	if w.step == 1 {
		fields := w.fields()
		if focus < len(fields) {
			return "field:" + fields[max(0, focus)].key
		}
		if focus == len(fields) {
			return "test"
		}
		return "back"
	}
	return map[int]string{0: "back", 1: "test", 2: "remember", 3: "connect", 100: "back"}[focus]
}

func (w *connectionWizard) setFocus(id string) {
	if id == "help" {
		w.focus = 101
		return
	}
	for i := 0; i < w.count(); i++ {
		if w.focusIDAt(i) == id {
			w.focus = i
			return
		}
	}
}

func (w *connectionWizard) executeWizard(id string) tea.Cmd {
	if id == "back" {
		w.back()
		return nil
	}
	if (w.busy && id != "help") || w.width < 60 || w.height < 22 {
		return nil
	}
	visible := false
	for _, r := range w.surfaceFrame().Regions {
		if r.ID == id {
			visible = true
		}
	}
	if !visible {
		return nil
	}
	switch id {
	case "help":
		w.help = true
	case "ftp", "azure", "s3":
		if w.step != 0 {
			return nil
		}
		w.kind = map[string]int{"ftp": 0, "azure": 1, "s3": 2}[id]
		w.step, w.focus = 1, 0
		if w.kind != 0 {
			w.values["root"] = ""
		}
	case "test":
		if w.step == 1 || w.step == 2 {
			return w.test()
		}
	case "connect":
		if w.step == 2 {
			return w.connect()
		}
	case "remember":
		if w.step == 2 {
			w.remember = !w.remember
		}
	default:
		if w.step == 1 {
			for i, f := range w.fields() {
				if "field:"+f.key == id && len(f.options) > 0 {
					w.focus = i
					w.cycle(1)
				}
			}
		}
	}
	return nil
}

func (w *connectionWizard) surfaceBounds() terminalui.Rect {
	width := min(76, w.width)
	x := max(0, (w.width-width)/2)
	h := min(w.height, len(w.rows())+3)
	return terminalui.Rect{X: x, Y: max(0, (w.height-h)/2), W: width, H: h}
}

func (w *connectionWizard) surface() (terminalui.SurfaceSpec, terminalui.Frame) {
	bounds := w.surfaceBounds()
	b := terminalui.SurfaceBodyBounds(bounds)
	s := terminalui.SurfaceSpec{ID: "connection-wizard", Title: "New connection", CancelID: "back", FocusID: w.focusID(), Body: terminalui.Frame{Bounds: b}}
	if w.width < 60 || w.height < 22 {
		return s, terminalui.Frame{}
	}
	back := "Back"
	if w.step == 2 {
		back = "Edit details"
	}
	if w.busy {
		back = "Cancel / back"
	}
	s.Actions = []terminalui.Action{{ID: "back", Label: back}}
	if !w.busy {
		if w.step == 1 {
			s.Actions = append(s.Actions, terminalui.Action{ID: "test", Label: "Test connection — read-only"})
		}
		if w.step == 2 {
			label := "Connect for this session"
			if w.width < 76 || !w.verified {
				label = "Connect"
			}
			if w.remember {
				label = "Save and connect"
			}
			a := terminalui.Action{ID: "connect", Label: label}
			if !w.verified {
				a.Unavailable = "Test first"
			}
			if w.verified {
				s.Actions = append(s.Actions, a, terminalui.Action{ID: "test", Label: "Test again"})
			} else {
				s.Actions = append(s.Actions, terminalui.Action{ID: "test", Label: "Test again"}, a)
			}
		}
	}
	s.Actions = append(s.Actions, terminalui.Action{ID: "help", Label: "Help"})
	for y, row := range w.rows() {
		if y >= b.H {
			break
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(w.theme.Foreground)).Background(lipgloss.Color(w.theme.Background))
		if row.focus < 0 {
			style = style.Foreground(lipgloss.Color(w.theme.Muted))
		}
		if row.focus == w.focus && !w.busy {
			style = style.Foreground(lipgloss.Color(w.theme.SelectionText)).Background(lipgloss.Color(w.theme.Selection))
		}
		s.Body.Lines = append(s.Body.Lines, style.Render(terminalui.Fit("  "+safeConnectionText(row.text), b.W)))
		if row.focus >= 0 && !w.busy {
			id := w.focusIDAt(row.focus)
			kind := terminalui.ControlRegion
			if w.step == 1 && row.focus < len(w.fields()) && len(w.fields()[row.focus].options) == 0 {
				kind = terminalui.FieldRegion
			}
			s.Body.Regions = append(s.Body.Regions, terminalui.Region{ID: id, Kind: kind, Bounds: terminalui.Rect{X: b.X + 2, Y: b.Y + y, W: max(0, b.W-4), H: 1}})
		}
	}
	if w.busy && s.FocusID != "help" {
		s.FocusID = "back"
	}
	frame := terminalui.RenderSurface(s, bounds, w.theme.TerminalStyle(false))
	regions := frame.Regions
	for i := 0; i < w.count(); i++ {
		id := w.focusIDAt(i)
		for _, r := range regions {
			if r.ID == id {
				s.FocusOrder = append(s.FocusOrder, id)
				break
			}
		}
	}
	if w.busy {
		s.FocusOrder = []string{"back"}
	}
	for _, r := range regions {
		if r.ID == "help" {
			s.FocusOrder = append(s.FocusOrder, "help")
			break
		}
	}
	return s, frame
}

func (w *connectionWizard) surfaceSpec() terminalui.SurfaceSpec {
	spec, _ := w.surface()
	return spec
}

func (w *connectionWizard) surfaceFrame() terminalui.Frame {
	_, frame := w.surface()
	return frame
}

func (w *connectionWizard) helpTopics() []terminalui.HelpTopic {
	return []terminalui.HelpTopic{{Group: "Connection setup", Keys: "Tab / arrows", Description: "Focus fields and controls. Type to edit; Ctrl+U clears; left/right changes options."}, {Group: "Connection setup", Keys: "Test / Connect", Description: "Testing is read-only. Connect uses a private session config. Remember explicitly permits Save and connect."}, {Group: "Connection setup", Keys: "Esc / Back / Close", Description: "Cancel the current operation and go back, invalidating tested details."}}
}

func (w *connectionWizard) helpFrame() terminalui.Frame {
	return terminalui.RenderHelp(w.surfaceSpec().Actions, w.helpTopics(), w.helpOffset, w.surfaceBounds(), w.theme.TerminalStyle(false))
}

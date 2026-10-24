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
	sessions                         *connectionSessions
	values                           map[string]string
	kind, step, focus, width, height int
	remember, busy, verified         bool
	dismissed                        bool
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
	if w.step == 0 {
		if w.focus == 3 {
			w.back()
			return nil
		}
		w.kind = w.focus
		w.step, w.focus = 1, 0
		if w.kind != 0 {
			w.values["root"] = ""
		}
		return nil
	}
	if w.step == 1 {
		if w.focus == len(w.fields())+1 {
			w.back()
			return nil
		}
		if w.focus == len(w.fields()) {
			return w.test()
		}
		fields := w.fields()
		if len(fields[w.focus].options) > 0 {
			w.cycle(1)
		} else {
			w.focus++
		}
		return nil
	}
	switch w.focus {
	case 0:
		w.back()
	case 1:
		return w.test()
	case 2:
		w.remember = !w.remember
	case 3:
		return w.connect()
	}
	return nil
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
	if mouse, ok := msg.(tea.MouseMsg); ok {
		if mouse.Action != tea.MouseActionPress || mouse.Button != tea.MouseButtonLeft {
			return nil
		}
		rows := w.rows()
		x, y, width := w.geometry(len(rows))
		row := mouse.Y - y - 1
		if mouse.X > x && mouse.X < x+width-1 && row >= 0 && row < len(rows) && rows[row].focus >= 0 {
			if w.busy {
				if rows[row].focus == 100 {
					w.back()
				}
				return nil
			}
			w.focus = rows[row].focus
			if w.step != 1 || w.focus >= len(w.fields()) || len(w.fields()[w.focus].options) > 0 {
				return w.activate()
			}
		}
		return nil
	}
	if w.busy {
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
		w.focus = (w.focus + 1) % w.count()
	case "shift+tab", "up":
		w.focus = (w.focus + w.count() - 1) % w.count()
	case "enter":
		return w.activate()
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
			w.remember = !w.remember
			return nil
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
	_, _, width := w.geometry(0)
	inner := max(54, width-6)
	compact := w.height < 28
	rows := []wizardRow{{"", -1}, {"New connection", -1}, {"", -1}}
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
		add("[ Back ]", 3)
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
		add("[ Test connection — read-only ]", len(w.fields()))
		add("[ Back ]", len(w.fields())+1)
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
		label := "[ Connect for this session ]"
		if w.remember {
			label = "[ Save and connect ]"
		}
		add(label, 3)
		add("[ Edit details ]", 0)
		add("[ Test again — read-only ]", 1)
		if w.busy {
			add("[ Cancel / back ]", 100)
		}
	}
	add("", -1)
	message := strings.Fields(w.message)
	var status []string
	if len(message) > 0 {
		status = strings.Split(ansi.Hardwrap(strings.Join(message, " "), inner, true), "\n")
	}
	// Collapse decorative space before sacrificing any controls or status help.
	budget := w.height - 2
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
	help := "Tab / ↑↓ focus · Enter select · ←→ options · Esc back"
	if len(status) > available {
		start := min(w.messageScroll, len(status)-available)
		status = status[start : start+available]
		help = "PgUp/PgDn help · Tab focus · Enter select · Esc back"
	}
	for _, line := range status {
		add(line, -1)
	}
	add(help, -1)
	return rows
}

func (w *connectionWizard) geometry(rows int) (x, y, width int) {
	width = min(76, w.width)
	return max(0, (w.width-width)/2), max(0, (w.height-rows-2)/2), width
}

func (w *connectionWizard) View() string {
	if w.width < 60 || w.height < 22 {
		return "Resize to at least 60 columns × 22 rows. Esc cancels."
	}
	rows := w.rows()
	_, _, width := w.geometry(len(rows))
	var lines []string
	for _, r := range rows {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(w.theme.Foreground)).Background(lipgloss.Color(w.theme.Background)).Width(width - 6)
		if r.focus < 0 {
			style = style.Foreground(lipgloss.Color(w.theme.Muted))
		}
		if r.text == "New connection" || strings.Contains(r.text, "→") || strings.HasPrefix(r.text, "! ") {
			style = style.Foreground(lipgloss.Color(w.theme.Focus)).Bold(true)
		}
		if strings.HasPrefix(r.text, "✓ ") {
			style = style.Foreground(lipgloss.Color(w.theme.Success)).Bold(true)
		}
		if w.step == 2 && r.focus == 3 && !w.verified {
			style = style.Foreground(lipgloss.Color(w.theme.Muted))
		}
		if r.focus == w.focus && (!w.busy || r.focus == 100) {
			style = style.Foreground(lipgloss.Color(w.theme.SelectionText)).Background(lipgloss.Color(w.theme.Selection))
		}
		text := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, r.text)
		lines = append(lines, style.Render(ansi.Truncate(text, width-6, "…")))
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(w.theme.Focus)).Background(lipgloss.Color(w.theme.Background)).Padding(0, 2).Render(strings.Join(lines, "\n"))
}

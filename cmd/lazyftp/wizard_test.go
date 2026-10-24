package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"lazyftp/internal/theme"
	"lazyftp/internal/ui"
)

func wizardKey(m workspace, key string) (workspace, tea.Cmd) {
	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "enter":
		k = tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		k = tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		k = tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		k = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+u":
		k = tea.KeyMsg{Type: tea.KeyCtrlU}
	case "down":
		k = tea.KeyMsg{Type: tea.KeyDown}
	case "right":
		k = tea.KeyMsg{Type: tea.KeyRight}
	case " ":
		k = tea.KeyMsg{Type: tea.KeySpace}
	}
	return updateWorkspace(m, k)
}

func setupWizard(t *testing.T) (workspace, string, *connectionSessions) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
  obscure) read -r value; printf 'obscured-fixture\n';;
  listremotes) exit 0;;
  lsf) if [ -n "$WIZARD_STARTED" ]; then printf started > "$WIZARD_STARTED"; exec /bin/sleep 30; fi; [ "$WIZARD_FAIL" != yes ];;
  lsjson) printf '[]';;
  *) exit 9;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "rclone"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("WIZARD_FAIL", "")
	t.Setenv("WIZARD_STARTED", "")
	theme, _ := theme.Get("")
	target := filepath.Join(t.TempDir(), "rclone.conf")
	sessions := &connectionSessions{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); sessions.Close() })
	m := workspace{model: workspaceProbe{events: "workspace;"}, ctx: ctx, theme: theme, config: target, persistentConfig: target, sessions: sessions, bookmarkPath: filepath.Join(t.TempDir(), "bookmarks")}
	m, _ = updateWorkspace(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m, cmd := updateWorkspace(m, ui.ConnectRequested{Local: t.TempDir()})
	m = runWorkspace(m, cmd)
	// New connection is available even when there are no configured remotes.
	m, cmd = wizardKey(m, "enter")
	if cmd != nil || !strings.Contains(m.View(), "FTP / FTPS") {
		t.Fatalf("empty picker did not open wizard: %s", m.View())
	}
	return m, target, sessions
}

func fillWizardFTP(m workspace, name string) workspace {
	for _, key := range []string{"enter", name, "tab", "localhost", "tab", "ctrl+u", "2121", "tab", "reader", "tab", "fixture-password-ONLY", "tab", "tab", "tab"} {
		m, _ = wizardKey(m, key)
	}
	return m
}

func wizardClick(t *testing.T, m workspace, label string) (workspace, tea.Cmd) {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if i := strings.Index(row, label); i >= 0 {
			return updateWorkspace(m, tea.MouseMsg{X: ansi.StringWidth(row[:i]) + 1, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		}
	}
	t.Fatalf("visible wizard action %q missing", label)
	return m, nil
}

func TestWizardConnectionTypesVimNavigation(t *testing.T) {
	for _, tc := range []struct {
		name, keys, field string
	}{
		{"azure", "j", "Storage account"},
		{"s3", "jj", "Service"},
		{"back up", "jjk", "Storage account"},
		{"wrap", "kk", "Service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _ := setupWizard(t)
			for _, key := range tc.keys {
				m, _ = wizardKey(m, string(key))
			}
			m, _ = wizardKey(m, "enter")
			if !strings.Contains(m.View(), tc.field) {
				t.Fatalf("wrong connection details after %q: %s", tc.keys, m.View())
			}
			for _, key := range []string{"j", "k", "tab"} {
				m, _ = wizardKey(m, key)
			}
			nameRow := ""
			for _, row := range strings.Split(ansi.Strip(m.View()), "\n") {
				if strings.Contains(row, "Connection name") {
					nameRow = row
				}
			}
			if !strings.Contains(nameRow, "jk") {
				t.Fatalf("navigation stole field input: %s", m.View())
			}
		})
	}
}

func TestWizardSessionConnectMasksSecretsAndNeverSavesByDefault(t *testing.T) {
	m, target, sessions := setupWizard(t)
	m = fillWizardFTP(m, "session-one")
	for _, render := range []string{m.View(), fmt.Sprintf("%+v", m), fmt.Sprintf("%#v", m)} {
		if strings.Contains(render, "fixture-password-ONLY") || !strings.Contains(render, "••••") {
			t.Fatal("model formatting exposed or did not mask password")
		}
	}
	if !strings.Contains(m.View(), "Explicit FTPS") || strings.Contains(m.View(), "Warning: FTP sends") {
		t.Fatal("FTPS must be the quiet secure default")
	}
	m, cmd := wizardKey(m, "enter")
	if cmd == nil || !strings.Contains(m.View(), "Testing a read-only listing") {
		t.Fatal("test must run asynchronously")
	}
	result := cmd().(wizardTested)
	if result.err != nil {
		t.Fatal(result.err)
	}
	sessionPath := result.candidate.ConfigPath
	m, _ = updateWorkspace(m, result)
	if !strings.Contains(m.View(), "Listing verified; write access not tested.") || !strings.Contains(m.View(), "[ ] Remember") {
		t.Fatalf("untruthful test/default persistence state: %s", m.View())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("test persisted credentials")
	}
	m, cmd = wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "connected=") || !strings.Contains(m.View(), "session-one:/") || !strings.Contains(m.View(), "backend=true/true") {
		t.Fatalf("session did not connect: %s", m.View())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("default Connect persisted credentials")
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatal("active candidate was closed too early")
	}
	sessions.Close()
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatal("quit leaked private session config")
	}
}

func TestWizardExplicitRememberSavesOnlyNewConnectionAcrossSessions(t *testing.T) {
	m, target, _ := setupWizard(t)
	m = fillWizardFTP(m, "session-only")
	m, cmd := wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	m, cmd = wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	// Start a second wizard using the first session snapshot, but the original
	// persistent target. Remember must not accidentally save session-only secrets.
	m, cmd = updateWorkspace(m, ui.ConnectRequested{Local: t.TempDir()})
	m = runWorkspace(m, cmd)
	m, _ = wizardClick(t, m, "New connection")
	m = fillWizardFTP(m, "remembered")
	m, cmd = wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	m, _ = wizardKey(m, "shift+tab")
	m, _ = wizardKey(m, " ")
	if !strings.Contains(m.View(), "[x] Remember") || !strings.Contains(m.View(), "not a keychain; not encrypted") || !strings.Contains(m.View(), "[ Save and connect ]") {
		t.Fatal("persistence consent/warning missing")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("checking Remember saved before explicit action")
	}
	m, _ = wizardKey(m, "tab")
	m, cmd = wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	data, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(data), "[remembered]") || strings.Contains(string(data), "[session-only]") {
		t.Fatalf("wrong persistent remotes (error %v)", err)
	}
	if !strings.Contains(m.View(), "remembered:/") {
		t.Fatalf("saved connection did not open: %s", m.View())
	}
}

func TestWizardFailureCancelStaleAndEditInvalidateVerification(t *testing.T) {
	m, target, _ := setupWizard(t)
	m = fillWizardFTP(m, "retry")
	t.Setenv("WIZARD_FAIL", "yes")
	m, cmd := wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "connection test failed") || strings.Contains(m.View(), "Listing verified") {
		t.Fatal("failed test looked successful")
	}
	// Untested Connect remains inert, including through mouse activation.
	m, _ = wizardKey(m, "down")
	m, _ = wizardKey(m, "down")
	m, _ = wizardKey(m, "down")
	m, cmd = wizardKey(m, "enter")
	if cmd != nil || !strings.Contains(m.View(), "Test these details successfully") {
		t.Fatal("failed candidate connected")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("failed test saved")
	}
	t.Setenv("WIZARD_FAIL", "")
	m, _ = wizardKey(m, "esc") // details, focus first field
	for i := 0; i < 7; i++ {
		m, _ = wizardKey(m, "tab")
	}
	m, cmd = wizardKey(m, "enter")
	// Completion can race cancellation; delivering the result afterwards must
	// neither verify changed input nor leak its private config.
	result := cmd().(wizardTested)
	if result.err != nil {
		t.Fatal(result.err)
	}
	path := result.candidate.ConfigPath
	m, _ = wizardKey(m, "esc")
	m, cleanup := updateWorkspace(m, result)
	m = runWorkspace(m, cleanup)
	if strings.Contains(m.View(), "Listing verified") {
		t.Fatal("stale success verified canceled details")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale result leaked candidate")
	}
	m, _ = wizardKey(m, "ctrl+u")
	m, _ = wizardKey(m, "changed")
	for i := 0; i < 7; i++ {
		m, _ = wizardKey(m, "tab")
	}
	m, cmd = wizardKey(m, "enter")
	m, _ = wizardKey(m, "esc") // cancel before the command even begins
	m = runWorkspace(m, cmd)
	if strings.Contains(m.View(), "Listing verified") || strings.Contains(m.View(), "connected=") {
		t.Fatal("canceled test connected")
	}
}

func TestWizardTypeMappingsAndMouseAndLayout(t *testing.T) {
	m, _, _ := setupWizard(t)
	// Click Azure in the centered modal using its visible text, not private focus.
	for y, row := range strings.Split(ansi.Strip(m.View()), "\n") {
		if x := strings.Index(row, "Azure Blob Storage"); x >= 0 {
			m, _ = updateWorkspace(m, tea.MouseMsg{X: x + 1, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			break
		}
	}
	if !strings.Contains(m.View(), "Storage account") || !strings.Contains(m.View(), "Azure CLI session") {
		t.Fatal("mouse type selection failed")
	}
	m, _ = wizardKey(m, "azure")
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "account")
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "right") // managed identity
	m, _ = wizardKey(m, "right") // SAS
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "sas-secret-fixture")
	if strings.Contains(m.View(), "sas-secret-fixture") || !strings.Contains(m.View(), "SAS URL") {
		t.Fatal("SAS URL leaked")
	}
	d := m.wizard.draft()
	if d.Backend != "azureblob" || d.Values["sas_url"] != "sas-secret-fixture" || d.Values["use_az"] != "" || d.Values["use_msi"] != "" {
		t.Fatal("Azure auth keys were mixed")
	}
	m, _ = wizardKey(m, "esc")
	m, _ = wizardKey(m, "down")
	m, _ = wizardKey(m, "enter") // S3
	if !strings.Contains(m.View(), "Profile / environment") {
		t.Fatal("S3 did not default to external credentials")
	}
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "right") // compatible
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "right") // access keys
	d = m.wizard.draft()
	if d.Backend != "s3" || d.Values["provider"] != "Other" || d.Values["env_auth"] != "false" || d.Values["force_path_style"] != "true" {
		t.Fatal("S3 mapping incorrect")
	}
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "access-id-fixture")
	m, _ = wizardKey(m, "tab")
	m, _ = wizardKey(m, "secret-key-fixture")
	if strings.Contains(fmt.Sprintf("%+v %#v", m, m), "access-id-fixture") || strings.Contains(m.View(), "secret-key-fixture") {
		t.Fatal("S3 credentials leaked")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 120, Height: 30}, {Width: 60, Height: 22}} {
		m, _ = updateWorkspace(m, size)
		for _, row := range strings.Split(m.View(), "\n") {
			if ansi.StringWidth(row) > size.Width {
				t.Fatalf("wizard overflow at %+v", size)
			}
		}
	}
}

func TestWizardRunningTestCancellationAndVerifiedEditCleanup(t *testing.T) {
	m, _, _ := setupWizard(t)
	m = fillWizardFTP(m, "cancel-running")
	marker := filepath.Join(t.TempDir(), "listing-started")
	t.Setenv("WIZARD_STARTED", marker)
	m, cmd := wizardKey(m, "enter")
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("test process did not start")
		}
		runtime.Gosched()
	}
	m, _ = wizardClick(t, m, "[ Cancel / back ]")
	select {
	case result := <-done:
		m = runWorkspace(m, func() tea.Msg { return result })
	case <-time.After(3 * time.Second):
		t.Fatal("canceled listing process did not stop promptly")
	}
	if strings.Contains(m.View(), "Listing verified") || !strings.Contains(m.View(), "Test canceled") {
		t.Fatal("canceled test replaced the details screen")
	}
	t.Setenv("WIZARD_STARTED", "")
	for i := 0; i < 7; i++ {
		m, _ = wizardKey(m, "tab")
	}
	m, cmd = wizardKey(m, "enter")
	result := cmd().(wizardTested)
	if result.err != nil {
		t.Fatal(result.err)
	}
	path := result.candidate.ConfigPath
	m, _ = updateWorkspace(m, result)
	if !strings.Contains(m.View(), "Listing verified") {
		t.Fatal("successful test did not verify")
	}
	m, _ = wizardKey(m, "esc")
	m, _ = wizardKey(m, "changed")
	if strings.Contains(m.View(), "Listing verified") {
		t.Fatal("editing retained verification")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("editing leaked the obsolete tested candidate")
	}
}

func TestWizardSaveFailureDoesNotConnectAndAllowsSessionOnly(t *testing.T) {
	m, target, _ := setupWizard(t)
	m = fillWizardFTP(m, "conflict")
	m, cmd := wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	if err := os.WriteFile(target, []byte("[conflict]\ntype = ftp\nhost = other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, _ = wizardKey(m, "shift+tab")
	m, _ = wizardKey(m, " ")
	m, _ = wizardKey(m, "tab")
	m, cmd = wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "already exists") || strings.Contains(m.View(), "connected=") {
		t.Fatal("save failure connected or was hidden")
	}
	data, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(data), "host = other") || strings.Contains(string(data), "obscured-fixture") {
		t.Fatal("failed save overwrote an existing remote")
	}
	m, _ = wizardKey(m, "shift+tab")
	m, _ = wizardKey(m, " ")
	m, _ = wizardKey(m, "tab")
	m, cmd = wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "conflict:/") {
		t.Fatal("session connection unavailable after save failure")
	}
}

func TestWizardAzureCLIHelpAndInsecureFTPIsOptIn(t *testing.T) {
	m, _, _ := setupWizard(t)
	for _, key := range []string{"down", "enter", "azure-cli", "tab", "example", "tab", "tab", "container", "tab"} {
		m, _ = wizardKey(m, key)
	}
	m, cmd := wizardKey(m, "enter")
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "Azure CLI is not installed") || !strings.Contains(m.View(), "az login") || strings.Contains(m.View(), "Listing verified") {
		t.Fatal("CLI authentication help missing or untested auth was verified")
	}
	m, _, _ = setupWizard(t)
	m = fillWizardFTP(m, "insecure-opt-in")
	m, _ = wizardKey(m, "shift+tab")
	m, _ = wizardKey(m, "shift+tab") // security
	m, _ = wizardKey(m, "right")
	if !strings.Contains(m.View(), "Implicit FTPS") || !strings.Contains(m.View(), "Port                 │ 2121") {
		t.Fatal("implicit FTPS did not preserve an explicitly entered port")
	}
	m, _ = wizardKey(m, "right")
	if !strings.Contains(m.View(), "Warning: FTP sends credentials and files unencrypted.") {
		t.Fatal("insecure FTP missing warning")
	}
	d := m.wizard.draft()
	if d.Values["tls"] != "false" || d.Values["explicit_tls"] != "false" || d.Values["port"] != "2121" {
		t.Fatal("insecure FTP mapping incorrect")
	}
}

func TestWizardMouseBackReturnsToPickerWithoutClosingWorkspace(t *testing.T) {
	m, _, _ := setupWizard(t)
	m, _ = wizardKey(m, "enter")
	m, cmd := wizardClick(t, m, "[ Back ]")
	if cmd != nil || !strings.Contains(m.View(), "FTP / FTPS") {
		t.Fatal("details Back did not restore type choice")
	}
	m, cmd = wizardClick(t, m, "[ Back ]")
	m = runWorkspace(m, cmd)
	if !strings.Contains(m.View(), "Open a connection") {
		t.Fatal("type Back did not restore picker")
	}
	m, _ = wizardKey(m, "esc")
	if !strings.Contains(m.View(), "workspace;") {
		t.Fatal("mouse Back lost retained workspace")
	}
}

func TestWizardResponsiveRowsAndPointerGeometry(t *testing.T) {
	theme, _ := theme.Get("")
	for _, size := range []tea.WindowSizeMsg{{Width: 60, Height: 22}, {Width: 80, Height: 24}, {Width: 100, Height: 36}} {
		for kind := 0; kind < 3; kind++ {
			for step := 0; step < 3; step++ {
				w := newWizard(context.Background(), theme, "", "", "", &connectionSessions{}, 0)
				w.width, w.height, w.kind, w.step = size.Width, size.Height, kind, step
				w.values["s3_auth"], w.values["s3_provider"] = "Access keys", "S3-compatible"
				w.values["azure_auth"] = "Azure CLI session"
				w.values["name"], w.values["root"] = strings.Repeat("界", 100), strings.Repeat("/long-root", 30)
				w.message = strings.Repeat("Check credentials and network. ", 30) + "END-OF-HELP"
				view := ansi.Strip(w.View())
				if len(strings.Split(view, "\n")) > size.Height {
					t.Fatalf("vertical overflow at %+v kind=%d step=%d:\n%s", size, kind, step, view)
				}
				for _, row := range strings.Split(view, "\n") {
					if ansi.StringWidth(row) != min(76, size.Width) {
						t.Fatalf("incorrect card width at %+v: %q", size, row)
					}
				}
				seen := map[int]bool{}
				rows := w.rows()
				x, y, _ := w.geometry(len(rows))
				for index, row := range rows {
					if row.focus >= 0 {
						seen[row.focus] = true
						continue
					}
					if row.text == "" {
						focus := w.focus
						w.input(tea.MouseMsg{X: x + 3, Y: y + index + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
						if w.focus != focus || w.step != step {
							t.Fatal("decorative padding activated an action")
						}
					}
				}
				for focus := 0; focus < w.count(); focus++ {
					if !seen[focus] {
						t.Fatalf("inaccessible control %d at %+v kind=%d step=%d", focus, size, kind, step)
					}
				}
				for i := 0; i < 100; i++ {
					w.input(tea.KeyMsg{Type: tea.KeyPgDown})
				}
				if !strings.Contains(w.View(), "END-OF-HELP") {
					t.Fatal("long help cannot be read to the end")
				}
				for i := 0; i < 100; i++ {
					w.input(tea.KeyMsg{Type: tea.KeyPgUp})
				}
				if !strings.Contains(w.View(), "Check credentials and network.") {
					t.Fatal("long help cannot be read from the beginning")
				}
				// Hit the visible row through the same centered geometry as View.
				label, wantStep := "[ Back ]", max(0, step-1)
				if step == 2 {
					label = "[ Edit details ]"
				}
				for index, row := range w.rows() {
					if row.text == label {
						w.input(tea.MouseMsg{X: x + 3, Y: y + index + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
						if w.step != wantStep || (step == 0 && !w.dismissed) {
							t.Fatalf("visible %s hit wrong action", label)
						}
						break
					}
				}
			}
		}
	}
}

func TestWizardTypeDescriptionMouseSelectsItsCard(t *testing.T) {
	m, _, _ := setupWizard(t)
	m, cmd := wizardClick(t, m, "Microsoft Azure object storage")
	if cmd != nil || m.wizard.step != 1 || m.wizard.kind != 1 || !strings.Contains(m.View(), "Storage account") {
		t.Fatal("description click did not select Azure")
	}
}

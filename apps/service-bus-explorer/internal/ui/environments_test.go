package ui_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
	"servicebusexplorer/internal/ui"
)

// Execute the requested read/save commands, but not the following periodic timer.
func commands(m tea.Model, cmd tea.Cmd, count int) tea.Model {
	for range count {
		if cmd == nil {
			break
		}
		m, cmd = m.Update(cmd())
	}
	return m
}

func scopeAction(m tea.Model, query string) (tea.Model, tea.Cmd) {
	m = press(press(m, "ctrl+p"), query)
	return key(m, "enter")
}

func scopeWorkspace(t *testing.T, s explorer.Store, provider explorer.ScopeProvider, p config.Preferences, path string) tea.Model {
	t.Helper()
	m := ui.New(s, ui.Options{Mode: "fixture", Preferences: p, ScopeProvider: provider, Connection: "fixture", ConnectionLabel: "Offline sample catalog", ConfigPath: path})
	return commands(m, m.Init(), 2)
}

func TestEnvironmentSnapshotCreationInclusionAndPersistence(t *testing.T) {
	s, err := fixture.NewScoped()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	p := config.EnsureLocal(config.Default(), "fixture", "Offline fixture", explorer.Namespace{Profile: "fixture", Address: "local-lab", Name: "local-lab"})
	m := scopeWorkspace(t, s, s, p, path)
	m = choose(m, "New environment")
	m = press(m, "Production")
	m = press(press(m, "enter"), "enter") // Browse subscriptions.
	m = press(press(m, "down"), "enter")  // Sample production.
	for range 3 {
		m = press(m, "down")
	}
	m = press(m, "enter") // Resource group preview, not a wildcard.
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Preview resource-group") || !strings.Contains(view, "prod-archive") || strings.Contains(view, "prod-payments") {
		t.Fatalf("wrong group snapshot\n%s", view)
	}
	m = press(press(press(m, "tab"), "enter"), "esc") // Cancel-first group; explicitly Add listed.
	for range 3 {
		m = press(m, "down")
	}
	m = press(m, "enter")                   // Review.
	m, cmd := key(press(m, "tab"), "enter") // Cancel-first review; explicitly Apply & use.
	m = commands(m, cmd, 2)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Environment: Production") || !strings.Contains(view, "6 scoped entities available") || strings.Contains(view, "128 queued") {
		t.Fatalf("new environments must start Selected/empty\n%s", view)
	}
	m = choose(m, "Dashboard inclusion")
	m = press(m, "/")
	m = press(m, "orders")
	m = press(m, "enter")
	m = press(m, "a") // Select the two matching queues, not the namespace's other entities.
	m = press(m, "s")
	p, err = config.Load(path)
	if err != nil || len(p.Environments) != 2 {
		t.Fatalf("save: %+v %v", p, err)
	}
	e := p.Environments[1]
	if e.Name != "Production" || e.All || len(e.Namespaces) != 2 || len(e.Watchlist) != 2 || e.Namespaces[0].ResourceGroup != "rg-core" {
		t.Fatalf("snapshot or inclusion lost: %+v", e)
	}
	reloaded := scopeWorkspace(t, s, s, p, path)
	if view := ansi.Strip(reloaded.View()); !strings.Contains(view, "2 watched entities") || !strings.Contains(view, "prod-archive") || strings.Contains(view, "prod-payments") || strings.Contains(view, "local-lab") {
		t.Fatalf("restart escaped saved scope\n%s", view)
	}
	// Discovery scope and dashboard inclusion are distinct: the other four
	// entities remain browsable, but other namespaces never enter Queues.
	reloaded = choose(reloaded, "Browse all Queues")
	if view := ansi.Strip(reloaded.View()); !strings.Contains(view, "6 discovered entities") || strings.Contains(view, "prod-payments") {
		t.Fatalf("inclusion accidentally became a discovery filter\n%s", view)
	}
	for _, size := range [][2]int{{160, 48}, {80, 24}, {60, 18}} {
		reloaded, _ = reloaded.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		reloaded = choose(reloaded, "Switch environment")
		assertFrame(t, reloaded, size[0], size[1])
		reloaded = press(reloaded, "qjk")
		if !strings.Contains(ansi.Strip(reloaded.View()), "qjk_") {
			t.Fatal("switcher printable keys escaped its search")
		}
		reloaded = press(press(reloaded, "esc"), "esc")
	}
}

type scopedScenario struct {
	*fixture.Store
	failed map[string]bool
	empty  map[string]bool
	active int64
}

func (s *scopedScenario) DiscoverScope(ctx context.Context, targets []explorer.Namespace) []explorer.NamespaceDiscovery {
	results := s.Store.DiscoverScope(ctx, targets)
	for i := range results {
		n := results[i].Namespace.Address
		if s.failed[n] {
			results[i].Err = errors.New("permission denied")
			results[i].Resources = nil
		} else if s.empty[n] {
			results[i].Resources = nil
		}
	}
	return results
}

func (s *scopedScenario) Counts(context.Context, explorer.Resource) (explorer.Counts, error) {
	return explorer.Counts{State: "available", Active: s.active, DeadLetter: 2}, nil
}

func TestEnvironmentFailuresRetainStaleCountsAndDistinguishMissing(t *testing.T) {
	base, _ := fixture.NewScoped()
	s := &scopedScenario{Store: base, failed: map[string]bool{}, empty: map[string]bool{}, active: 37}
	core, _ := s.ResolveNamespace(context.Background(), "prod-core")
	archive, _ := s.ResolveNamespace(context.Background(), "prod-archive")
	r := explorer.Resource{Profile: "fixture", Namespace: core.Address, Queue: "orders", AzureSubscriptionID: core.AzureSubscriptionID, ResourceGroup: core.ResourceGroup}
	p := config.Default()
	p.Environments = []config.Environment{{ID: "prod", Name: "Production", Connection: "fixture", Namespaces: []explorer.Namespace{core, archive}, Watchlist: []explorer.Resource{r}}}
	p.ActiveEnvironment = "prod"
	m := scopeWorkspace(t, s, s, p, "")
	s.failed[core.Address] = true
	m, cmd := scopeAction(m, "Rediscover selected")
	m = commands(m, cmd, 2)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Partial discovery: 1 of 2") || !strings.Contains(view, "37~ queued") {
		t.Fatalf("permission failure became empty or authoritative counts\n%s", view)
	}
	m = choose(m, "Discovery details")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "permission denied") {
		t.Fatalf("missing namespace error\n%s", view)
	}
	m = press(m, "esc")
	s.failed[core.Address], s.empty[core.Address] = false, true
	m, cmd = scopeAction(m, "Rediscover selected")
	m = commands(m, cmd, 2)
	m = choose(m, "Dashboard inclusion")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Not found in successful listing") || strings.Contains(view, "permission denied") {
		t.Fatalf("successful missing resource not distinguished\n%s", view)
	}
}

func TestEnvironmentFirstRunEmptyAndFailedSave(t *testing.T) {
	s, _ := fixture.NewScoped()
	p := config.Default()
	p.Environments = []config.Environment{}
	m := scopeWorkspace(t, s, s, p, t.TempDir()) // A directory cannot be replaced by preferences.
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Choose what to discover") || strings.Contains(view, "128 queued") {
		t.Fatalf("first run enumerated before selection\n%s", view)
	}
	m = press(m, "t")
	m = choose(m, "Save preferences")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Save failed:") || !strings.Contains(view, "unsaved preferences") {
		t.Fatalf("save failure claimed durability\n%s", view)
	}
	base, _ := s.ResolveNamespace(context.Background(), "prod-core")
	p.Environments = []config.Environment{{ID: "empty", Name: "Empty", Connection: "fixture", Namespaces: []explorer.Namespace{base}, All: true}}
	p.ActiveEnvironment = "empty"
	provider := &scopedScenario{Store: s, empty: map[string]bool{base.Address: true}}
	m = scopeWorkspace(t, provider, provider, p, "")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "successfully checked namespaces") || strings.Contains(view, "could not be checked") {
		t.Fatalf("successful empty namespace confused with permission failure\n%s", view)
	}
}

func TestEnvironmentEditsRetainExcludedSelectionsAndDeleteWithoutFallback(t *testing.T) {
	s, _ := fixture.NewScoped()
	core, _ := s.ResolveNamespace(context.Background(), "prod-core")
	archive, _ := s.ResolveNamespace(context.Background(), "prod-archive")
	p := config.EnsureLocal(config.Default(), "fixture", "Production", core)
	p.Environments[0].Namespaces = append(p.Environments[0].Namespaces, archive)
	p.Environments[0].All = false
	for _, ns := range []explorer.Namespace{core, archive} {
		p.Environments[0].Watchlist = append(p.Environments[0].Watchlist, explorer.Resource{Profile: "fixture", Namespace: ns.Address, Queue: "orders", AzureSubscriptionID: ns.AzureSubscriptionID, ResourceGroup: ns.ResourceGroup})
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	m := scopeWorkspace(t, s, s, p, path)
	m = choose(m, "Edit current environment")
	m = press(press(m, "ctrl+u"), "Production renamed")
	m = press(press(press(m, "tab"), "tab"), "enter")
	m = press(press(m, " "), "esc") // Remove core, retain its inclusion selection.
	for range 3 {
		m = press(m, "down")
	}
	m = press(m, "enter")
	m, cmd := key(press(m, "tab"), "enter")
	m = commands(m, cmd, 2)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Environment: Production renamed") || !strings.Contains(view, "1 watched entities") || strings.Contains(view, "prod-core") {
		t.Fatalf("removed namespace remained browsable\n%s", view)
	}
	m = choose(m, "Dashboard inclusion")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "[saved] prod-core / orders · Outside current discovery scope") {
		t.Fatalf("excluded selection was lost or presented as active\n%s", view)
	}
	m = press(m, "esc")
	m = choose(m, "Edit current environment")
	m = press(press(press(press(m, "tab"), "tab"), "tab"), "enter")
	m = press(press(m, "prod-core"), "enter")
	m = press(press(m, "tab"), "enter")
	m, cmd = key(press(m, "tab"), "enter")
	m = commands(m, cmd, 2)
	if !strings.Contains(ansi.Strip(m.View()), "2 watched entities") {
		t.Fatal("re-adding the same namespace did not restore inclusion")
	}
	m = choose(m, "Manage environments")
	m = press(m, "d")
	m = press(m, "Production copy")
	for range 4 {
		m = press(m, "tab")
	}
	m = press(press(m, "enter"), "a") // Apply without switching.
	m = press(m, "x")                 // Delete current original, not the copy.
	m = press(m, "tab")               // Cancel-first: focus exact-name authorization.
	m = press(m, "Production renamed")
	m = press(m, "tab") // Separate visible Delete activation from typing authorization.
	m, cmd = key(m, "enter")
	m = commands(m, cmd, 1)
	m = press(m, "esc")
	m = choose(m, "Save preferences")
	p, err := config.Load(path)
	if err != nil || p.ActiveEnvironment != "" || len(p.Environments) != 1 || p.Environments[0].Name != "Production copy" || len(p.Environments[0].Watchlist) != 2 {
		t.Fatalf("delete silently switched or duplicate lost selections: %+v, %v\n%s", p, err, ansi.Strip(m.View()))
	}
	if !strings.Contains(ansi.Strip(m.View()), "Choose what to discover") {
		t.Fatal("deleted active environment silently fell back to another inventory")
	}
}

func TestEnvironmentSwitchIgnoresLateCountsAndGuardsReviews(t *testing.T) {
	base, _ := fixture.NewScoped()
	s := &scopedScenario{Store: base, active: 128}
	ns, _ := s.ResolveNamespace(context.Background(), "prod-core")
	p := config.Default()
	for _, name := range []string{"Production", "Staging"} {
		p.Environments = append(p.Environments, config.Environment{ID: name, Name: name, Connection: "fixture", Namespaces: []explorer.Namespace{ns}, All: true})
	}
	p.ActiveEnvironment = "Production"
	m := ui.New(s, ui.Options{Mode: "fixture", Preferences: p, ScopeProvider: s, Connection: "fixture"})
	var model tea.Model = m
	model, oldCountCommand := model.Update(m.Init()())
	oldCounts := oldCountCommand()
	s.active = 7
	model = choose(model, "Switch environment")
	model = press(model, "Staging")
	model, cmd := key(model, "enter")
	model = commands(model, cmd, 2)
	model, _ = model.Update(oldCounts)
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Environment: Staging") || !strings.Contains(view, "7 queued") || strings.Contains(view, "128 queued") {
		t.Fatalf("late prior-environment counts escaped scope generation\n%s", view)
	}
	model = press(press(model, "]"), "enter")
	model = choose(model, "Resend selected")
	model = press(model, "ctrl+p")
	if view := ansi.Strip(model.View()); strings.Contains(view, "Ctrl+P / ACTIONS") || !strings.Contains(view, "Review active") {
		t.Fatalf("switcher opened during message review\n%s", view)
	}
}

func TestEnvironmentPanelsFromCompactInspectorAndLateClipboard(t *testing.T) {
	s, _ := fixture.NewScoped()
	ns, _ := s.ResolveNamespace(context.Background(), "prod-core")
	p := config.Default()
	for _, name := range []string{"Production", "Staging"} {
		p.Environments = append(p.Environments, config.Environment{ID: name, Name: name, Connection: "fixture", Namespaces: []explorer.Namespace{ns}, All: true})
	}
	p.ActiveEnvironment = "Production"
	m := ui.New(s, ui.Options{Mode: "fixture", Preferences: p, ScopeProvider: s, Connection: "fixture", Clipboard: func(string) (bool, error) { return true, nil }})
	model := commands(m, m.Init(), 2)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = press(press(model, "]"), "enter")
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Production") || !strings.Contains(view, "INSPECTOR") {
		t.Fatalf("compact inspector lost environment context\n%s", view)
	}
	model, copyCommand := key(model, "Y")
	if copyCommand == nil {
		t.Fatal("full-body copy did not start")
	}
	lateCopy := copyCommand()
	for _, size := range [][2]int{{80, 24}, {60, 18}} {
		model, _ = model.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		model = choose(model, "Switch environment")
		view := ansi.Strip(model.View())
		if !strings.Contains(view, "Switch environment") || !strings.Contains(view, "Production") || strings.Contains(view, "INSPECTOR /") {
			t.Fatalf("environment panel hidden by compact inspector\n%s", view)
		}
		assertFrame(t, model, size[0], size[1])
		model = press(model, "esc")
	}
	model = choose(model, "Switch environment")
	model = press(model, "Staging")
	model, cmd := key(model, "enter")
	model = commands(model, cmd, 2)
	before := model.View()
	model, _ = model.Update(lateCopy)
	if model.View() != before {
		t.Fatal("late clipboard result changed the new environment")
	}
}

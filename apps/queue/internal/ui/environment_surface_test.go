package ui

import (
	"context"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
	"servicebusexplorer/internal/fixture"
	"terminalui"
)

func environmentSurfaceModel(t *testing.T, page string) Model {
	t.Helper()
	s, err := fixture.NewScoped()
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.ResolveNamespace(context.Background(), "prod-core")
	if err != nil {
		t.Fatal(err)
	}
	p := config.EnsureLocal(config.Default(), "fixture", "Production", n)
	m := New(s, Options{Mode: "fixture", Preferences: p, ScopeProvider: s, Connection: "fixture"})
	m.width, m.height = 80, 24
	m.env.page, m.env.draft = page, p.Environments[0]
	m.env.available = []explorer.Namespace{n}
	m.items = []item{{resource: explorer.Resource{Profile: "fixture", Namespace: n.Address, Queue: "orders", AzureSubscriptionID: n.AzureSubscriptionID, ResourceGroup: n.ResourceGroup}}}
	return m
}

func environmentInputKey(m Model, kind tea.KeyType) Model {
	next, _ := m.environmentInput(tea.KeyMsg{Type: kind})
	return next.(Model)
}

func environmentHit(t *testing.T, m Model, id string, kind terminalui.RegionKind) tea.MouseMsg {
	t.Helper()
	b := m.environmentBounds()
	f := terminalui.RenderSurface(m.environmentSurface(b.W, b.H), b, m.theme.TerminalStyle(m.ascii))
	for _, r := range f.Regions {
		if r.ID == id && r.Kind == kind {
			return tea.MouseMsg{X: r.Bounds.X + r.Bounds.W - 1, Y: r.Bounds.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
		}
	}
	t.Fatalf("missing visible target %s (%v)", id, kind)
	return tea.MouseMsg{}
}

func TestEnvironmentSurfaceLifecycleNeverCapturesInput(t *testing.T) {
	m := environmentSurfaceModel(t, "edit")
	for _, msg := range []tea.Msg{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}, tea.MouseMsg{}} {
		if _, _, captured := m.environmentUpdate(msg); captured {
			t.Fatal("lifecycle handler captured input")
		}
	}
	before := m.env.draft.Name
	next, cmd := m.environmentInput(catalogLoaded{id: m.env.request, known: &m.env.available[0]})
	if cmd != nil || next.(Model).env.draft.Name != before || next.(Model).env.page != "edit" {
		t.Fatal("input handler captured lifecycle")
	}
}

func TestEnvironmentSurfacePointerSelectAndSeparateCheckbox(t *testing.T) {
	for _, page := range []string{"namespaces", "inclusion"} {
		t.Run(page, func(t *testing.T) {
			m := environmentSurfaceModel(t, page)
			m.prefs.Environments[0].All = false
			m.env.draft.Namespaces = nil
			id := m.environmentRows()[0]
			click := environmentHit(t, m, id, terminalui.RowRegion)
			next, cmd := m.environmentInput(click)
			m = next.(Model)
			if cmd != nil || m.env.dirty || len(m.env.draft.Namespaces) != 0 || len(m.prefs.Environments[0].Watchlist) != 0 {
				t.Fatal("row press activated domain action")
			}
			check := environmentHit(t, m, "check:"+id, terminalui.ControlRegion)
			next, _ = m.environmentInput(check)
			m = next.(Model)
			if page == "namespaces" && len(m.env.draft.Namespaces) != 1 {
				t.Fatal("namespace checkbox did not add exact namespace")
			}
			if page == "inclusion" && len(m.prefs.Environments[0].Watchlist) != 1 {
				t.Fatal("resource checkbox did not add exact resource")
			}
			check.Action = tea.MouseActionRelease
			next, cmd = m.environmentInput(check)
			if cmd != nil || len(next.(Model).env.draft.Namespaces) != len(m.env.draft.Namespaces) || len(next.(Model).prefs.Environments[0].Watchlist) != len(m.prefs.Environments[0].Watchlist) {
				t.Fatal("release repeated activation")
			}
		})
	}
}

func TestEnvironmentSurfaceConfirmationDefaultsAndVisibleTraversal(t *testing.T) {
	for _, page := range []string{"review", "group", "all", "discard", "quit", "delete"} {
		t.Run(page, func(t *testing.T) {
			m := environmentSurfaceModel(t, page)
			m.env.back = "edit"
			if m.environmentFocus() != "back" {
				t.Fatal("confirmation not cancel-first")
			}
			next, cmd := m.environmentInput(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil || next.(Model).env.page == page {
				t.Fatal("default Enter did not cancel")
			}
			m = environmentInputKey(m, tea.KeyTab)
			if m.environmentFocus() == "back" {
				t.Fatal("Tab failed to reach next visible control")
			}
		})
	}
}

func TestEnvironmentSurfaceTextHelpAndControlPOwnership(t *testing.T) {
	m := environmentSurfaceModel(t, "edit")
	m.env.draft.Name = ""
	for _, value := range []string{"q", "j", "k", "s", "?"} {
		next, _ := m.environmentInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
		m = next.(Model)
	}
	if m.env.draft.Name != "qjks?" {
		t.Fatalf("text escaped owner: %q", m.env.draft.Name)
	}
	m.env.index = 2
	m = environmentInputKey(m, tea.KeyCtrlP)
	if m.env.index != 2 || m.palette {
		t.Fatal("Ctrl+P moved row or opened palette")
	}
	m.help = true
	// Main shared Help captures first; neither its opening nor closing mutates
	// the staged form. Assert via the real Update route rather than local input.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.help || m.env.draft.Name != "qjks?" || !m.env.dirty {
		t.Fatal("Help changed parent draft")
	}
}

func TestEnvironmentSurfaceDismissInvalidatesAndCompactFreezes(t *testing.T) {
	m := environmentSurfaceModel(t, "namespaces")
	m.env.busy, m.env.query = true, "filter"
	before := m.env.request
	click := tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	next, _ := m.environmentInput(click)
	m = next.(Model)
	if m.env.query != "" || m.env.page != "namespaces" || m.env.busy || m.env.request <= before {
		t.Fatal("outside press skipped search-clear/invalidation")
	}
	next, _, _ = m.environmentUpdate(catalogLoaded{id: before, namespaces: nil})
	if len(next.(Model).env.available) != 1 {
		t.Fatal("late metadata changed current catalog")
	}
	m.width, m.height, m.env.page = 50, 10, "edit"
	name := m.env.draft.Name
	next, _ = m.environmentInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hidden")})
	if next.(Model).env.draft.Name != name {
		t.Fatal("compact input edited hidden field")
	}
	m.env.page, m.env.back, m.env.dirty = "quit", "edit", true
	next, cmd := m.environmentInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = next.(Model)
	if cmd != nil || !strings.Contains(m.env.err, "draft is not saved") {
		t.Fatal("compact Save accepted unapplied draft")
	}
	m.width = 47
	next, cmd = m.environmentInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd != nil || next.(Model).env.page != "quit" {
		t.Fatal("hidden compact quit route activated")
	}
}

func TestEnvironmentSurfaceNativeIdentitiesAndAllRefusal(t *testing.T) {
	m := environmentSurfaceModel(t, "namespaces")
	other := m.env.available[0]
	other.AzureSubscriptionID = "different-subscription"
	m.env.available = append(m.env.available, other)
	ids := m.environmentRows()
	if ids[0] == ids[1] {
		t.Fatal("same-name namespaces share identity")
	}
	old := environmentHit(t, m, "check:"+ids[0], terminalui.ControlRegion)
	m.width, m.height = 50, 10
	next, cmd := m.environmentInput(old)
	if cmd != nil || next.(Model).env.dirty {
		t.Fatal("pre-resize hidden pointer activated")
	}
	m = environmentSurfaceModel(t, "inclusion")
	m.prefs.Environments[0].All = true
	b := m.environmentBounds()
	f := terminalui.RenderSurface(m.environmentSurface(b.W, b.H), b, m.theme.TerminalStyle(m.ascii))
	for _, r := range f.Regions {
		if strings.HasPrefix(r.ID, "check:") {
			t.Fatal("All mode rendered active checkbox")
		}
	}
	next, _ = m.environmentDo("toggle")
	if len(next.(Model).prefs.Environments[0].Watchlist) != 0 {
		t.Fatal("All mode toggled checkbox")
	}
}

func TestEnvironmentSurfaceKeyPointerOperationsAgree(t *testing.T) {
	for _, tc := range []struct{ page, action, want string }{
		{"edit", "browse", "subscriptions"}, {"edit", "selected", "selected"},
		{"edit", "known", "known"}, {"edit", "review", "review"},
		{"selected", "identity", "details"}, {"selected", "remove", "selected"},
		{"switch", "manage", "manage"}, {"manage", "edit", "edit"},
		{"manage", "duplicate", "edit"}, {"manage", "delete-review", "delete"},
		{"review", "apply", "manage"}, {"group", "add-group", "namespaces"},
		{"all", "enable-all", "inclusion"}, {"discard", "discard", "manage"},
		{"delete", "delete", "switch"},
	} {
		t.Run(tc.page+"/"+tc.action, func(t *testing.T) {
			for _, pointer := range []bool{false, true} {
				m := environmentSurfaceModel(t, tc.page)
				m.env.editID = m.env.draft.ID
				m.env.group = slices.Clone(m.env.available)
				if tc.page == "review" {
					// Apply only leaves the current selection alone. Editing an
					// active scope intentionally re-discovers it instead.
					m.prefs.ActiveEnvironment = ""
				}
				if tc.page == "delete" {
					m.env.query = m.env.draft.Name
				}
				var msg tea.Msg
				if pointer {
					msg = environmentHit(t, m, tc.action, terminalui.ControlRegion)
				} else {
					m.env.focus, m.env.focusPage = tc.action, tc.page
					msg = tea.KeyMsg{Type: tea.KeyEnter}
				}
				next, _ := m.environmentInput(msg)
				got := next.(Model)
				if got.env.page != tc.want {
					t.Fatalf("pointer=%v: got page %q, want %q (error %q)", pointer, got.env.page, tc.want, got.env.err)
				}
				if tc.action == "remove" && (len(got.env.draft.Namespaces) != 0 || !got.env.dirty) {
					t.Fatal("Remove lost draft semantics")
				}
				if tc.action == "duplicate" && (got.env.draft.ID == m.env.draft.ID || got.env.draft.Name != "" || len(got.env.draft.Namespaces) != 1) {
					t.Fatal("Duplicate lost exact membership/isolation")
				}
				if tc.action == "delete" && (len(got.prefs.Environments) != 0 || got.prefs.ActiveEnvironment != "") {
					t.Fatal("Delete silently fell back to another scope")
				}
				if tc.action == "apply" && (got.prefs.ActiveEnvironment != "" || got.revision <= m.revision) {
					t.Fatal("Apply only switched scope or failed to apply session preferences")
				}
			}
		})
	}
}

func TestEnvironmentSurfaceMinimumFormTraversalDoesNotTrap(t *testing.T) {
	m := environmentSurfaceModel(t, "edit")
	m.width, m.height = 60, 18
	for _, want := range []string{"name", "browse", "selected", "known", "review", "back", "name"} {
		if got := m.environmentFocus(); got != want {
			t.Fatalf("minimum form focus = %q, want %q", got, want)
		}
		m = environmentInputKey(m, tea.KeyTab)
	}
}

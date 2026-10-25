package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"servicebusexplorer/internal/azure"
	"servicebusexplorer/internal/config"
	"servicebusexplorer/internal/explorer"
)

func TestDefaultsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "preferences.json")
	p, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(p, config.Default()) {
		t.Fatalf("missing file: %+v, %v", p, err)
	}
	if p.Version != 1 || p.Theme != "midnight" || !p.Sidebar || !p.Inspector || p.Table {
		t.Fatalf("incorrect defaults: %+v", p)
	}
	p.Theme, p.Table, p.Sidebar, p.Inspector = "gruvbox", true, false, false
	p.Watchlists = map[string][]explorer.Resource{
		"fixture": {
			{Profile: "fixture", Namespace: "one", Queue: "orders"},
			{Profile: "fixture", Namespace: "two", Queue: "orders"},
			{Profile: "fixture", Namespace: "one", Topic: "events", Subscription: "billing"},
			{Profile: "fixture", Namespace: "one", Topic: "events"},
		},
		"emulator": {},
	}
	if err := config.Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("roundtrip: %+v, %v", got, err)
	}
	if list, ok := got.Watchlists["emulator"]; !ok || list == nil || len(list) != 0 {
		t.Fatal("explicit empty list lost")
	}
	delete(p.Watchlists, "emulator")
	if err := config.Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Watchlists["emulator"]; ok {
		t.Fatal("absent profile became explicit empty list")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file permissions: %v, %v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v, %v", entries, err)
	}
}

func TestRejectInvalidWithoutClobber(t *testing.T) {
	for name, raw := range map[string]string{
		"future":           `{"version":2,"theme":"midnight"}`,
		"missing version":  `{"theme":"midnight"}`,
		"theme":            `{"version":1,"theme":"unknown"}`,
		"corrupt":          `{`,
		"trailing":         `{"version":1,"theme":"midnight"} {}`,
		"credentials":      `{"version":1,"theme":"midnight","credentials":"secret"}`,
		"nested body":      `{"version":1,"theme":"midnight","watchlists":{"fixture":[{"Profile":"fixture","Namespace":"one","Queue":"orders","body":"secret"}]}}`,
		"profile mismatch": `{"version":1,"theme":"midnight","watchlists":{"fixture":[{"Profile":"emulator","Namespace":"one","Queue":"orders"}]}}`,
		"null list":        `{"version":1,"theme":"midnight","watchlists":{"fixture":null}}`,
		"invalid target":   `{"version":1,"theme":"midnight","watchlists":{"fixture":[{"Profile":"fixture","Namespace":"one","Queue":"orders","Topic":"events"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "preferences.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := config.Load(path); err == nil {
				t.Fatal("invalid config accepted")
			}
			if err := config.Save(path, config.Default()); err == nil {
				t.Fatal("invalid existing config replaced")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != raw {
				t.Fatalf("existing file changed: %q, %v", got, err)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := config.Save(path, config.Default()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	p := config.Default()
	p.Version = 2
	if err := config.Save(path, p); err == nil {
		t.Fatal("invalid new preferences accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("invalid input clobbered valid file")
	}
}

func TestPath(t *testing.T) {
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := config.Path()
	if err != nil || got != filepath.Join(dir, "service-bus-explorer", "preferences.json") {
		t.Fatalf("Path() = %q, %v", got, err)
	}
}

func TestEnsureLocal(t *testing.T) {
	ns := explorer.Namespace{Profile: "fixture", Address: "local-lab", Name: "local-lab"}
	resource := explorer.Resource{Profile: "fixture", Namespace: "excluded", Queue: "orders"}
	for _, mode := range []string{"absent", "empty", "selected"} {
		t.Run(mode, func(t *testing.T) {
			p := config.Default()
			if mode != "absent" {
				p.Watchlists = map[string][]explorer.Resource{"fixture": {}}
			}
			if mode == "selected" {
				p.Watchlists["fixture"] = []explorer.Resource{resource}
			}
			before := p.Clone()
			got := config.EnsureLocal(p, "fixture", " Local lab ", ns)
			if !reflect.DeepEqual(p, before) {
				t.Fatal("migration mutated input")
			}
			e := got.Environments[0]
			if e.ID != "fixture" || e.Name != "Local lab" || e.Connection != "fixture" ||
				!reflect.DeepEqual(e.Namespaces, []explorer.Namespace{ns}) || e.All != (mode == "absent") ||
				!reflect.DeepEqual(e.Watchlist, p.Watchlists["fixture"]) || got.ActiveEnvironment != "fixture" {
				t.Fatalf("incorrect migration: %+v", got)
			}
			if again := config.EnsureLocal(got, "fixture", "Changed", ns); !reflect.DeepEqual(again, got) {
				t.Fatal("migration not idempotent")
			}
			if mode == "selected" {
				got.Environments[0].Watchlist[0].Queue = "changed"
				if p.Watchlists["fixture"][0] != resource || got.Watchlists["fixture"][0] != resource {
					t.Fatal("migrated watchlist aliases legacy list")
				}
			}
			path := filepath.Join(t.TempDir(), "preferences.json")
			if err := config.Save(path, got); err != nil {
				t.Fatal(err)
			}
			restored, err := config.Load(path)
			if err != nil || !reflect.DeepEqual(restored, got) {
				t.Fatalf("environment roundtrip failed: %+v, %v", restored, err)
			}
		})
	}
	p := config.Default()
	p.ActiveEnvironment = "unavailable"
	p = config.EnsureLocal(p, "fixture", "Local", ns)
	if p.ActiveEnvironment != "unavailable" {
		t.Fatal("unavailable active environment replaced")
	}
	// Keep a different connection's environment: nil/empty alone cannot record
	// that fixture was deleted when other environments survive.
	p.Environments = []config.Environment{{ID: "other", Name: "Other", Connection: "emulator",
		Namespaces: []explorer.Namespace{{Profile: "emulator", Address: "localhost:5672", Name: "local-lab"}}}}
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := config.Save(path, p); err != nil {
		t.Fatal(err)
	}
	p, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.EnsureLocal(p, "fixture", "Local", ns); !reflect.DeepEqual(got, p) {
		t.Fatal("deleted environment recreated")
	}
	for _, marker := range [][]config.Environment{nil, {}} {
		p.Environments = marker
		if err := config.Save(path, p); err != nil {
			t.Fatal(err)
		}
		restored, err := config.Load(path)
		if err != nil || !reflect.DeepEqual(restored.Environments, marker) {
			t.Fatalf("nil/empty distinction lost: %+v, %v", restored, err)
		}
		if got := config.EnsureLocal(restored, "fixture", "Local", ns); len(got.Environments) != 0 {
			t.Fatal("deleted final environment recreated")
		}
	}
	p = config.Default()
	p.Environments = []config.Environment{}
	if got := config.EnsureLocal(p, "fixture", "Local", ns); len(got.Environments) != 0 {
		t.Fatal("explicit empty environments ignored")
	}
}

func TestClone(t *testing.T) {
	p := config.Default()
	p.Watchlists = map[string][]explorer.Resource{"fixture": {{Profile: "fixture", Namespace: "one", Queue: "orders"}}, "empty": {}}
	p = config.EnsureLocal(p, "fixture", "Local", explorer.Namespace{Profile: "fixture", Address: "one", Name: "one"})
	before := p.Clone()
	c := p.Clone()
	if !reflect.DeepEqual(c, p) {
		t.Fatal("clone differs")
	}
	c.Watchlists["fixture"][0].Queue = "changed"
	c.Watchlists["new"] = nil
	c.Environments[0].Name = "changed"
	c.Environments[0].Namespaces[0].Name = "changed"
	c.Environments[0].Watchlist[0].Queue = "changed"
	c.InitializedConnections[0] = "changed"
	if !reflect.DeepEqual(p, before) {
		t.Fatal("clone shares mutable state")
	}
}

func TestEnvironmentValidation(t *testing.T) {
	base := config.Default()
	base.ActiveEnvironment = "unavailable"
	base.Environments = []config.Environment{{ID: "prod", Name: strings.Repeat("界", 40), Connection: "azure:tenant",
		Namespaces: []explorer.Namespace{{Profile: "azure:tenant", Address: "orders.servicebus.windows.net", Name: "orders",
			AzureSubscriptionID: "sub-one", AzureSubscriptionName: "Production subscription", ResourceGroup: "group-one"}},
		Watchlist: []explorer.Resource{{Profile: "azure:tenant", Namespace: "excluded.servicebus.windows.net", Queue: "orders", AzureSubscriptionID: "old-sub", ResourceGroup: "old-group"}},
	}}
	for name, change := range map[string]func(*config.Environment){
		"empty name":           func(e *config.Environment) { e.Name = "" },
		"untrimmed":            func(e *config.Environment) { e.Name = " Production " },
		"too long":             func(e *config.Environment) { e.Name += "界" },
		"control":              func(e *config.Environment) { e.Name = "Prod\x1b[31m" },
		"no id":                func(e *config.Environment) { e.ID = "" },
		"no connection":        func(e *config.Environment) { e.Connection = "" },
		"no namespaces":        func(e *config.Environment) { e.Namespaces = nil },
		"wrong profile":        func(e *config.Environment) { e.Namespaces[0].Profile = "other" },
		"url":                  func(e *config.Environment) { e.Namespaces[0].Address = "sb://orders.servicebus.windows.net" },
		"query":                func(e *config.Environment) { e.Namespaces[0].Address += "?key=secret" },
		"credentials":          func(e *config.Environment) { e.Namespaces[0].Address = "user:secret@orders.servicebus.windows.net" },
		"name path":            func(e *config.Environment) { e.Namespaces[0].Name = "orders/secret" },
		"name mismatch":        func(e *config.Environment) { e.Namespaces[0].Name = "other" },
		"missing group":        func(e *config.Environment) { e.Namespaces[0].ResourceGroup = "" },
		"missing subscription": func(e *config.Environment) { e.Namespaces[0].AzureSubscriptionID = "" },
		"duplicate address": func(e *config.Environment) {
			n := e.Namespaces[0]
			n.Address = strings.ToUpper(n.Address)
			e.Namespaces = append(e.Namespaces, n)
		},
		"cloud on local":                 func(e *config.Environment) { e.Connection = "fixture"; e.Namespaces[0].Profile = "fixture" },
		"local on cloud":                 func(e *config.Environment) { e.Namespaces[0].Address = "localhost:5672" },
		"watchlist mismatch":             func(e *config.Environment) { e.Watchlist[0].Profile = "other" },
		"watchlist missing group":        func(e *config.Environment) { e.Watchlist[0].ResourceGroup = "" },
		"watchlist missing subscription": func(e *config.Environment) { e.Watchlist[0].AzureSubscriptionID = "" },
		"duplicate resource":             func(e *config.Environment) { e.Watchlist = append(e.Watchlist, e.Watchlist[0]) },
		"invalid resource":               func(e *config.Environment) { e.Watchlist[0].Topic = "events" },
	} {
		t.Run(name, func(t *testing.T) {
			p := base.Clone()
			change(&p.Environments[0])
			if err := config.Save(filepath.Join(t.TempDir(), "preferences.json"), p); err == nil {
				t.Fatal("invalid environment accepted")
			}
		})
	}
	for _, duplicateID := range []bool{true, false} {
		p := base.Clone()
		p.Environments[0].Name = "Production Σ"
		other := p.Environments[0]
		if duplicateID {
			other.Name = "Other"
		} else {
			other.ID = "other"
			other.Name = "production ς"
		}
		p.Environments = append(p.Environments, other)
		if err := config.Save(filepath.Join(t.TempDir(), "preferences.json"), p); err == nil {
			t.Fatal("duplicate ID/name accepted")
		}
	}
	for _, selected := range []bool{true, false} {
		p := base.Clone()
		p.Environments[0].All = !selected
		p.Environments[0].Watchlist = nil
		p.Environments[0].Namespaces[0].AzureSubscriptionID = ""
		p.Environments[0].Namespaces[0].AzureSubscriptionName = ""
		p.Environments[0].Namespaces[0].ResourceGroup = ""
		if err := config.Save(filepath.Join(t.TempDir(), "preferences.json"), p); err != nil {
			t.Fatalf("known namespace / nil inclusion rejected: %v", err)
		}
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := config.Save(path, base); err != nil {
		t.Fatalf("valid retained selection rejected: %v", err)
	}
	got, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(got, base) {
		t.Fatalf("management identity not retained: %+v, %v", got, err)
	}
}

func TestOfflineSampleEnvironment(t *testing.T) {
	p := config.Default()
	e := config.Environment{ID: "sample", Name: "Offline sample", Connection: "fixture"}
	for _, sample := range []struct{ address, subscription, group string }{
		{"prod-core", "sample-production", "rg-core"},
		{"prod-archive", "sample-production", "rg-core"},
		{"prod-payments", "sample-production", "rg-payments"},
		{"prod-notifications", "sample-shared", "rg-shared"},
		{"stage-core", "sample-staging", "rg-core"},
	} {
		e.Namespaces = append(e.Namespaces, explorer.Namespace{
			Profile: "fixture", Address: sample.address, Name: sample.address,
			AzureSubscriptionID: sample.subscription, ResourceGroup: sample.group,
		})
		e.Watchlist = append(e.Watchlist, explorer.Resource{
			Profile: "fixture", Namespace: sample.address, Queue: "orders",
			AzureSubscriptionID: sample.subscription, ResourceGroup: sample.group,
		})
	}
	p.Environments = []config.Environment{e}
	p.ActiveEnvironment = e.ID
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := config.Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("offline sample roundtrip: %+v, %v", got, err)
	}
	for _, change := range []func(*config.Environment){
		func(e *config.Environment) { e.Namespaces[0].Address = "prod-core.servicebus.windows.net" },
		func(e *config.Environment) { e.Watchlist[0].Namespace = "prod-core.servicebus.windows.net" },
		func(e *config.Environment) { e.Namespaces[0].ResourceGroup = "" },
		func(e *config.Environment) { e.Watchlist[0].ResourceGroup = "" },
		func(e *config.Environment) {
			e.Connection = "emulator"
			for i := range e.Namespaces {
				e.Namespaces[i].Profile = "emulator"
			}
			e.Watchlist = nil
		},
		func(e *config.Environment) {
			e.Connection = "emulator"
			for i := range e.Namespaces {
				e.Namespaces[i].Profile = "emulator"
				e.Namespaces[i].AzureSubscriptionID, e.Namespaces[i].ResourceGroup = "", ""
			}
			for i := range e.Watchlist {
				e.Watchlist[i].Profile = "emulator"
			}
		},
	} {
		invalid := p.Clone()
		change(&invalid.Environments[0])
		if err := config.Save(path, invalid); err == nil {
			t.Fatal("cloud endpoint, incomplete identity or emulator metadata accepted")
		}
	}
}

func TestProviderIdentityRoundTrip(t *testing.T) {
	local := config.EnsureLocal(config.Default(), "emulator", "Local lab", azure.LocalNamespace())
	cloud := config.Default()
	cloud.Environments = []config.Environment{{ID: "prod", Name: "Production", Connection: "azure:tenant",
		Namespaces: []explorer.Namespace{{Profile: "azure:tenant", Address: "orders.servicebus.windows.net", Name: "orders",
			AzureSubscriptionID: "sub-one", ResourceGroup: "rg(prod)"}},
		Watchlist: []explorer.Resource{{Profile: "azure:tenant", Namespace: "orders.servicebus.windows.net", Queue: "orders",
			AzureSubscriptionID: "sub-one", ResourceGroup: "rg(prod)"}},
	}}
	for name, p := range map[string]config.Preferences{"emulator": local, "cloud parentheses": cloud} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "preferences.json")
			if err := config.Save(path, p); err != nil {
				t.Fatal(err)
			}
			got, err := config.Load(path)
			if err != nil || !reflect.DeepEqual(got, p) {
				t.Fatalf("provider roundtrip: %+v, %v", got, err)
			}
		})
	}
	for _, name := range []string{"", " Local lab", "Local lab ", "Local\nlab", "Local\x1b[31m"} {
		p := local.Clone()
		p.Environments[0].Namespaces[0].Name = name
		if err := config.Save(filepath.Join(t.TempDir(), "preferences.json"), p); err == nil {
			t.Fatalf("invalid local display name accepted: %q", name)
		}
	}
	for _, group := range []string{"", ".", "..", strings.Repeat("r", 91), "rg/prod", "rg?key=secret", "rg prod", "rg\nprod"} {
		for _, watchlist := range []bool{false, true} {
			p := cloud.Clone()
			if watchlist {
				p.Environments[0].Watchlist[0].ResourceGroup = group
			} else {
				p.Environments[0].Namespaces[0].ResourceGroup = group
			}
			if err := config.Save(filepath.Join(t.TempDir(), "preferences.json"), p); err == nil {
				t.Fatalf("invalid resource group accepted: %q (watchlist=%v)", group, watchlist)
			}
		}
	}
}

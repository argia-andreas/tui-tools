package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadProfilesAndPrivateReferences(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("password", "private-value\n")
	p := write("connections.json", `{"connections":[{"name":"z-local","brokers":["127.0.0.1:9092","[::1]:9092"],"local_plaintext":true},{"name":"a-remote","brokers":["broker.example:9093"],"tls":true,"sasl":"SCRAM-SHA-512","username":"viewer","password_file":"password"}]}`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Names(), []string{"a-remote", "z-local"}) {
		t.Fatal(c.Names())
	}
	remote, err := c.Connection("a-remote")
	if err != nil {
		t.Fatal(err)
	}
	if remote.PasswordFile != filepath.Join(dir, "password") {
		t.Fatal("relative path was not resolved")
	}
	remote.Brokers[0] = "changed:1"
	again, _ := c.Connection("a-remote")
	if again.Brokers[0] != "broker.example:9093" {
		t.Fatal("returned profile aliases config")
	}
	if _, err := c.Connection("nope"); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

func TestStrictConfig(t *testing.T) {
	valid := `{"name":"local","brokers":["localhost:9092"],"local_plaintext":true}`
	cases := []string{
		`null`, `{}`, `{"connections":[]}`, `{"connections":[` + valid + `,` + valid + `]}`,
		`{"connections":[` + valid + `],"connections":[]}`,
		`{"connections":[{"name":"x","name":"y","brokers":["localhost:9092"],"local_plaintext":true}]}`,
		`{"connections":[` + strings.Replace(valid, `"name"`, `"Name"`, 1) + `]}`,
		`{"connections":[` + strings.Replace(valid, `"local_plaintext":true`, `"local_plaintext":true,"tls":null`, 1) + `]}`,
		`{"connections":[` + valid + `],"extra":1}`,
		`{"connections":[` + valid + `]} {}`,
		`{"connections":[` + strings.Replace(valid, `"localhost:9092"`, `"remote.example:9092"`, 1) + `]}`,
		`{"connections":[` + strings.Replace(valid, `"localhost:9092"`, `"localhost:70000"`, 1) + `]}`,
		`{"connections":[` + strings.Replace(valid, `"local_plaintext":true`, `"local_plaintext":true,"tls":true`, 1) + `]}`,
		strings.Repeat(" ", maxConfig+1),
	}
	for i, body := range cases {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(p, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestSecurityCombinations(t *testing.T) {
	base := Profile{Name: "test", Brokers: []string{"broker:9093"}, TLS: true}
	for _, mechanism := range []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		p := base
		p.SASL = mechanism
		p.Username = "reader"
		p.PasswordFile = "password"
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, change := range []func(*Profile){
		func(p *Profile) { p.TLS = false }, func(p *Profile) { p.CertFile = "cert" },
		func(p *Profile) { p.SASL = "PLAIN" }, func(p *Profile) { p.PasswordFile = "secret" },
		func(p *Profile) { p.SASL = "OAUTHBEARER" },
		func(p *Profile) {
			p.TLS = false
			p.LocalPlaintext = true
			p.Brokers = []string{"127.0.0.1:9092"}
			p.Username = "reader"
		},
	} {
		p := base
		change(&p)
		if p.Validate() == nil {
			t.Fatal("unsafe combination accepted")
		}
	}
}

func TestPrivateFilesRefuseUnsafeInputs(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("do-not-log-this"), 0600); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadPrivateFile(secret); err != nil || string(b) != "do-not-log-this" {
		t.Fatal("private file rejected", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{link, dir, filepath.Join(dir, "missing")} {
		if _, err := ReadPrivateFile(p); err == nil || strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "do-not-log-this") {
			t.Fatal("unsafe file or diagnostic", err)
		}
	}
	if err := os.Chmod(secret, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateFile(secret); err == nil {
		t.Fatal("group-readable secret accepted")
	}
	if err := os.Chmod(secret, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, make([]byte, maxConfig+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateFile(secret); err == nil {
		t.Fatal("oversized secret accepted")
	}
}

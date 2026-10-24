package elastic_test

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"logviewer/internal/config"
	"logviewer/internal/elastic"
	"logviewer/internal/fixture"
	"logviewer/internal/logs"
)

func TestHTTPSReadBoundary(t *testing.T) {
	response := `{"timed_out":false,"_shards":{"failed":0},"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_source":{"@timestamp":"2026-10-23T14:26:00Z","message":"fixture-secret-hidden","fields":{"large":9007199254740993}}}]}}`
	status := http.StatusOK
	var leaked atomic.Bool
	sink := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(r.Header.Get("Authorization") != "") }))
	defer sink.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/logview-lab-events/_search" || r.Header.Get("Authorization") != "ApiKey test-key" {
			t.Error("incorrect read scope")
		}
		var query map[string]any
		if json.NewDecoder(r.Body).Decode(&query) != nil || query["size"] != float64(500) {
			t.Error("incorrect bounded query")
		}
		w.Header().Set("Location", sink.URL)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	dir := t.TempDir()
	cert, _ := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	ca := filepath.Join(dir, "ca.pem")
	key := filepath.Join(dir, "key")
	_ = os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0600)
	_ = os.WriteFile(key, []byte("test-key"), 0600)
	c := config.Config{Source: "fixture", Endpoint: server.URL, Target: "logview-lab-events", CAFile: ca, APIKeyFile: key}
	client, err := elastic.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	service, err := logs.NewService(client, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Search(context.Background(), fixture.Query())
	if err != nil || len(page.Events) != 1 || page.Events[0].Text("message") != "[REDACTED]" || !strings.Contains(string(page.Events[0].Values["fields"]), "9007199254740993") {
		t.Fatalf("unsafe or corrupted response: %v", err)
	}
	for _, bad := range []string{
		`{"timed_out":true,"hits":{"hits":[]}}`,
		`{"_shards":{"failed":1},"hits":{"hits":[]}}`,
		`{"error":"fixture-secret-hidden"}`,
		response + ` {}`,
		strings.Repeat(" ", (8<<20)+1),
	} {
		response = bad
		if page, err := service.Search(context.Background(), fixture.Query()); err == nil || len(page.Events) != 0 || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatal("bad response did not fail closed")
		}
	}
	status = http.StatusForbidden
	if _, err := service.Search(context.Background(), fixture.Query()); err == nil {
		t.Fatal("403 accepted")
	}
	status = http.StatusTemporaryRedirect
	if _, err := service.Search(context.Background(), fixture.Query()); err == nil || leaked.Load() {
		t.Fatal("redirect accepted or credential forwarded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Search(ctx, fixture.Query(), ""); err == nil {
		t.Fatal("canceled request succeeded")
	}
	c.Endpoint = strings.Replace(server.URL, "https:", "http:", 1)
	if _, err := elastic.New(c); err == nil {
		t.Fatal("plaintext accepted")
	}
}

func TestLocalLab(t *testing.T) {
	path := os.Getenv("LOGVIEW_LAB_CONFIG")
	if path == "" {
		t.Skip("explicit local lab integration only")
	}
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	client, err := elastic.New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	service, err := logs.NewService(client, c.Policy())
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.Search(context.Background(), c.Query())
	if err != nil || len(page.Events) != 22 || page.More {
		t.Fatalf("lab page: count=%d more=%v err=%v", len(page.Events), page.More, err)
	}
	var stack, unknown bool
	for _, event := range page.Events {
		b, _ := json.Marshal(event)
		if strings.Contains(string(b), "fixture-secret-") || strings.Contains(string(b), `\u001b`) {
			t.Fatal("unsafe lab output")
		}
		stack = stack || strings.Contains(string(event.Values["fields"]), "process.go:127")
		unknown = unknown || event.Text("level") == "UNKNOWN"
		if event.Text("level") == "UNKNOWN" && (!strings.Contains(string(event.Values["fields"]), "9007199254740993") || !strings.Contains(string(event.Values["fields"]), `"enabled":false`)) {
			t.Fatal("real structured integer/boolean types were corrupted")
		}
	}
	if !stack || !unknown {
		t.Fatal("lost multiline stack or missing severity")
	}
	q := c.Query()
	q.Window.Start = time.Date(2026, 10, 23, 14, 26, 1, 10_000_000, time.UTC)
	q.Window.End = time.Date(2026, 10, 23, 14, 26, 1, 38_000_000, time.UTC)
	page, err = service.Search(context.Background(), q)
	if err != nil || len(page.Events) != 1 || page.Events[0].Text("message") != "POST /orders accepted · order=8042" {
		t.Fatal("real half-open time predicate ignored: start must be included, end excluded")
	}
	t.Log("PASS: real TLS HTTP → read-only adapter → safe service; 22 events, stack, UNKNOWN, canary exclusion, half-open time predicate")
}

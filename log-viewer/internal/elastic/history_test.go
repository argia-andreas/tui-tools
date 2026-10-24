package elastic_test

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"logviewer/internal/config"
	"logviewer/internal/elastic"
	"logviewer/internal/fixture"
	"logviewer/internal/logs"
)

func tlsClient(t *testing.T, handler http.HandlerFunc) *elastic.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cert, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	ca, key := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "key")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("test-key"), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := elastic.New(config.Config{Source: "fixture", Endpoint: server.URL, Target: "events", CAFile: ca, APIKeyFile: key})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Shutdown(context.Background()) })
	return client
}

func TestNativeHistoryLifecycle(t *testing.T) {
	var mu sync.Mutex
	live := map[string]bool{}
	client := tlsClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body struct {
			ID  string `json:"id"`
			PIT struct {
				ID string `json:"id"`
			} `json:"pit"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case r.Method == "DELETE" && r.URL.Path == "/_pit":
			delete(live, body.ID)
			fmt.Fprint(w, `{"succeeded":true}`)
		case r.Method == "POST" && r.URL.Path == "/events/_pit":
			token := rand.Text()
			live[token] = true
			_ = json.NewEncoder(w).Encode(map[string]string{"id": token})
		case r.Method == "POST" && r.URL.Path == "/_search":
			if !live[body.PIT.ID] {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"error":"expired backend context"}`)
				return
			}
			// Simulate native PIT ID rotation. Stale cleanup leaves an observable live resource.
			delete(live, body.PIT.ID)
			token := rand.Text()
			live[token] = true
			hit := `{"_index":"events","_id":"%s","sort":[1792765560000,%d],"_source":{"@timestamp":"2026-10-23T14:26:00Z","message":"same"}}`
			fmt.Fprintf(w, `{"pit_id":%q,"hits":{"hits":[`+hit+`,`+hit+`]}}`, token, "a", 2, "b", 1)
		default:
			t.Errorf("unexpected read route %s", r.URL.Path)
			w.WriteHeader(400)
		}
	})
	ctx := context.Background()
	q := fixture.Query()
	q.Limits.Events = 1
	first, err := client.Search(ctx, q, "")
	if err != nil || first.Cursor == "" || !first.More || len(first.Events) != 1 {
		t.Fatalf("first page: %+v %v", first, err)
	}
	var wg sync.WaitGroup
	successes := make(chan string, 2)
	for range 2 {
		wg.Go(func() {
			p, err := client.Search(ctx, q, first.Cursor)
			if err == nil {
				successes <- p.Cursor
			}
		})
	}
	wg.Wait()
	close(successes)
	tokens := []string{}
	for token := range successes {
		tokens = append(tokens, token)
	}
	if len(tokens) != 1 {
		t.Fatalf("cursor replay succeeded: %d", len(tokens))
	}
	if err := client.Close(ctx, tokens[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Search(ctx, q, tokens[0]); err == nil {
		t.Fatal("closed cursor broadened search")
	}
	mu.Lock()
	if len(live) != 0 {
		t.Error("Close left a live PIT")
	}
	mu.Unlock()
	p, err := client.Search(ctx, q, "")
	if err != nil {
		t.Fatal(err)
	}
	changed := q
	changed.Scope.Services = []string{"different"}
	if _, err := client.Search(ctx, changed, p.Cursor); err == nil {
		t.Fatal("cursor scope was mutable")
	}
	mu.Lock()
	if len(live) != 0 {
		t.Error("rejected scope change left a live PIT")
	}
	mu.Unlock()
	p, err = client.Search(ctx, q, "")
	if err != nil {
		t.Fatal(err)
	}
	// Expire the backend snapshot, not private provider state. It must fail rather
	// than silently opening a replacement snapshot with a broader/fresh query.
	mu.Lock()
	clear(live)
	mu.Unlock()
	if page, err := client.Search(ctx, q, p.Cursor); err == nil || len(page.Events) != 0 {
		t.Fatal("expired PIT broadened search")
	}
	for range 16 {
		if _, err := client.Search(ctx, q, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := client.Search(ctx, q, ""); err == nil {
		t.Fatal("unbounded sessions")
	}
	if err := client.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Search(ctx, q, ""); err == nil {
		t.Fatal("shutdown client reopened")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(live) != 0 {
		t.Fatalf("Shutdown left %d live PITs", len(live))
	}
}

func TestUnsupportedFiltersFailBeforeIO(t *testing.T) {
	client := tlsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unsupported filter reached backend")
		w.WriteHeader(400)
	})
	base := fixture.Query()
	for _, mutate := range []func(*logs.Query){
		func(q *logs.Query) { q.Message = logs.MessageFilter{Mode: "literal", Text: "x"} },
		func(q *logs.Query) { q.Message = logs.MessageFilter{Mode: "phrase", Text: "x", CaseSensitive: true} },
		func(q *logs.Query) { q.Levels = []string{"WARN+"} },
		func(q *logs.Query) {
			q.Fields = []logs.FieldFilter{{Path: "/fields/auth/token", Value: json.RawMessage(`"x"`)}}
		},
		func(q *logs.Query) {
			q.Fields = []logs.FieldFilter{{Path: "/fields/job/attempt", Value: json.RawMessage(`"1"`)}}
		},
		func(q *logs.Query) {
			q.Fields = []logs.FieldFilter{{Path: "/fields/job/retryable", Value: json.RawMessage(`null`)}}
		},
		func(q *logs.Query) {
			q.Fields = []logs.FieldFilter{{Path: "/service", Value: json.RawMessage(`{"query":"all"}`)}}
		},
	} {
		q := base
		mutate(&q)
		if page, err := client.Search(context.Background(), q, ""); err == nil || len(page.Events) != 0 {
			t.Fatal("unsupported filter accepted")
		}
	}
}

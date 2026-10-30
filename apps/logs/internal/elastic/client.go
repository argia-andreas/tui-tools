// Package elastic implements bounded read-only Elasticsearch ECS searches.
package elastic

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"logviewer/internal/config"
	"logviewer/internal/logs"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

type Client struct {
	config   config.Config
	http     *http.Client
	key      string
	mu       sync.Mutex // Serializes native cursor advancement, including token replay.
	sessions map[string]*session
	closed   bool
}
type session struct {
	pit, query string
	after      []json.RawMessage
	expires    time.Time
	timer      *time.Timer
}

func New(c config.Config) (*Client, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("source requires an HTTPS origin")
	}
	ca, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load source CA")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("invalid source CA")
	}
	f, err := os.Open(c.APIKeyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load source credential")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("credential must be a private regular file")
	}
	key, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(key) > 8192 {
		return nil, fmt.Errorf("invalid source credential")
	}
	k := strings.TrimSpace(string(key))
	if k == "" || strings.ContainsAny(k, "\r\n \t") {
		return nil, fmt.Errorf("invalid source credential")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, DisableCompression: true, ResponseHeaderTimeout: 10 * time.Second, MaxResponseHeaderBytes: 64 << 10}
	return &Client{config: c, key: k, sessions: make(map[string]*session), http: &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("source redirects forbidden") }}}, nil
}

func (c *Client) request(ctx context.Context, method, path string, body any) ([]byte, error) {
	fail := func() ([]byte, error) { return nil, fmt.Errorf("Elasticsearch read failed") }
	b, err := json.Marshal(body)
	if err != nil {
		return fail()
	}
	if body == nil {
		b = nil
	} // PIT open accepts no request body, not JSON null.
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.config.Endpoint, "/")+path, bytes.NewReader(b))
	if err != nil {
		return fail()
	}
	req.Header.Set("Authorization", "ApiKey "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fail()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Encoding") != "" {
		return fail()
	}
	b, err = io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil || len(b) > 8<<20 || !json.Valid(b) {
		return fail()
	}
	return b, nil
}

// All lifecycle helpers run with mu held. Cleanup has its own bounded context,
// so a canceled search still releases its PIT.
func (c *Client) release(ctx context.Context, token string, s *session) error {
	delete(c.sessions, token)
	if s.timer != nil {
		s.timer.Stop()
	}
	_, err := c.request(ctx, http.MethodDelete, "/_pit", map[string]any{"id": s.pit})
	return err
}
func (c *Client) cleanup(token string, s *session) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.release(ctx, token, s)
}
func (c *Client) Close(ctx context.Context, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[token]
	if s == nil {
		return fmt.Errorf("history expired; reload required")
	}
	return c.release(ctx, token, s)
}
func (c *Client) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	var first error
	for token, s := range c.sessions {
		if err := c.release(ctx, token, s); first == nil {
			first = err
		}
	}
	c.http.CloseIdleConnections()
	return first
}

type nativeHit struct {
	Index  string            `json:"_index"`
	ID     string            `json:"_id"`
	Sort   []json.RawMessage `json:"sort"`
	Source json.RawMessage   `json:"_source"`
}

func (c *Client) Search(ctx context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fail := func() (logs.NativePage, error) { return logs.NativePage{}, fmt.Errorf("Elasticsearch read failed") }
	filter, err := c.filter(q)
	if err != nil || c.closed {
		if s := c.sessions[cursor]; s != nil {
			c.cleanup(cursor, s)
		}
		return fail()
	}
	frozen, _ := json.Marshal(q)
	s := c.sessions[cursor]
	if cursor != "" {
		if s == nil {
			return fail()
		}
		delete(c.sessions, cursor)
		if s.timer != nil {
			s.timer.Stop()
		}
		if s.query != string(frozen) || !time.Now().Before(s.expires) {
			c.cleanup(cursor, s)
			return fail()
		}
	} else {
		if len(c.sessions) >= 16 {
			return fail()
		}
		b, err := c.request(ctx, http.MethodPost, "/"+url.PathEscape(c.config.Target)+"/_pit?keep_alive=2m", nil)
		var pit struct {
			ID string `json:"id"`
		}
		if err != nil || json.Unmarshal(b, &pit) != nil || pit.ID == "" {
			return fail()
		}
		s = &session{pit: pit.ID, query: string(frozen)}
	}
	retained := false
	defer func() {
		if !retained {
			c.cleanup(cursor, s)
		}
	}()
	body := map[string]any{"size": q.Limits.Events + 1, "track_total_hits": false, "sort": []any{map[string]string{"@timestamp": "desc"}, map[string]string{"_shard_doc": "desc"}}, "pit": map[string]string{"id": s.pit, "keep_alive": "2m"}, "query": filter,
		"_source": []string{"@timestamp", "log.level", "service.name", "service.node.name", "message", "trace.id", "fields"}}
	if len(s.after) != 0 {
		body["search_after"] = s.after
	}
	b, err := c.request(ctx, http.MethodPost, "/_search", body)
	if err != nil {
		return fail()
	}
	var result struct {
		PIT      string `json:"pit_id"`
		TimedOut bool   `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits *struct {
			Hits []nativeHit `json:"hits"`
		} `json:"hits"`
	}
	if json.Unmarshal(b, &result) != nil {
		return fail()
	}
	if result.PIT != "" {
		s.pit = result.PIT
	}
	if result.Hits == nil || result.TimedOut || result.Shards.Failed != 0 || len(result.Hits.Hits) > q.Limits.Events+1 {
		return fail()
	}
	page := logs.NativePage{More: len(result.Hits.Hits) > q.Limits.Events}
	if page.More {
		result.Hits.Hits = result.Hits.Hits[:q.Limits.Events]
	}
	for _, hit := range result.Hits.Hits {
		event, err := decode(hit, q)
		if err != nil {
			return fail()
		}
		page.Events = append(page.Events, event)
	}
	if page.More {
		s.after = result.Hits.Hits[len(result.Hits.Hits)-1].Sort
		token := rand.Text()
		c.sessions[token] = s
		s.expires = time.Now().Add(2 * time.Minute)
		s.timer = time.AfterFunc(2*time.Minute, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.sessions[token] == s {
				c.cleanup(token, s)
			}
		})
		page.Cursor = token
		retained = true
	}
	// Continue in descending native order, display the bounded page chronologically.
	slices.Reverse(page.Events)
	return page, nil
}

func decode(hit nativeHit, q logs.Query) (logs.Event, error) {
	fail := func() (logs.Event, error) { return logs.Event{}, fmt.Errorf("invalid event") }
	if len(hit.Source) > 1<<20 || hit.Index == "" || hit.ID == "" || len(hit.Sort) != 2 {
		return fail()
	}
	var source struct {
		Time    time.Time `json:"@timestamp"`
		Message string    `json:"message"`
		Log     struct {
			Level string `json:"level"`
		} `json:"log"`
		Service struct {
			Name string `json:"name"`
			Node struct {
				Name string `json:"name"`
			} `json:"node"`
		} `json:"service"`
		Trace struct {
			ID string `json:"id"`
		} `json:"trace"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if json.Unmarshal(hit.Source, &source) != nil || source.Time.IsZero() || source.Time.Before(q.Window.Start) || !source.Time.Before(q.Window.End) {
		return fail()
	}
	level := strings.ToUpper(source.Log.Level)
	if !slices.Contains(knownLevels, level) {
		level = "UNKNOWN"
	}
	return logs.Event{Ref: logs.NativeRef{Index: hit.Index, ID: hit.ID}, Time: source.Time, Level: level, Service: source.Service.Name, Instance: source.Service.Node.Name, Message: source.Message, TraceID: source.Trace.ID, Fields: source.Fields}, nil
}

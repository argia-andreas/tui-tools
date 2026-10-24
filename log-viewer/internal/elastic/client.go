// Package elastic implements bounded read-only Elasticsearch ECS searches.
package elastic

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"logviewer/internal/config"
	"logviewer/internal/logs"
)

type Client struct {
	config config.Config
	http   *http.Client
	key    string
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
	return &Client{config: c, key: k, http: &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("source redirects forbidden") }}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

func (c *Client) Search(ctx context.Context, q logs.Query, cursor string) (logs.NativePage, error) {
	fail := func() (logs.NativePage, error) { return logs.NativePage{}, fmt.Errorf("Elasticsearch read failed") }
	if cursor != "" || q.Scope.Source != c.config.Source || q.Limits.Events < 1 || q.Limits.Events > 500 || !q.Window.End.After(q.Window.Start) {
		return fail()
	}
	body := map[string]any{"size": q.Limits.Events, "track_total_hits": true, "sort": []any{map[string]string{"@timestamp": "desc"}}, "_source": []string{"@timestamp", "log.level", "service.name", "service.node.name", "message", "trace.id", "fields"}, "query": map[string]any{"range": map[string]any{"@timestamp": map[string]string{"gte": q.Window.Start.UTC().Format(time.RFC3339Nano), "lt": q.Window.End.UTC().Format(time.RFC3339Nano)}}}}
	b, err := json.Marshal(body)
	if err != nil {
		return fail()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.config.Endpoint, "/")+"/"+url.PathEscape(c.config.Target)+"/_search", bytes.NewReader(b))
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
	if err != nil || len(b) > 8<<20 {
		return fail()
	}
	var result struct {
		TimedOut bool `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits *struct {
			Total struct {
				Value    int    `json:"value"`
				Relation string `json:"relation"`
			} `json:"total"`
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if json.Unmarshal(b, &result) != nil || result.Hits == nil || result.TimedOut || result.Shards.Failed != 0 || len(result.Hits.Hits) > q.Limits.Events {
		return fail()
	}
	page := logs.NativePage{More: result.Hits.Total.Value > len(result.Hits.Hits) || result.Hits.Total.Relation != "eq"}
	for _, hit := range result.Hits.Hits {
		if len(hit.Source) > 1<<20 {
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
		page.Events = append(page.Events, logs.Event{Time: source.Time, Level: source.Log.Level, Service: source.Service.Name, Instance: source.Service.Node.Name, Message: source.Message, TraceID: source.Trace.ID, Fields: source.Fields})
	}
	// Fetch the newest bounded window, display it chronologically. Stable older
	// history and equal-timestamp PIT tie-breaking belong to Slice 3.
	slices.Reverse(page.Events)
	return page, nil
}

package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"logviewer/internal/logs"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

var knownLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

func (c *Client) filter(q logs.Query) (map[string]any, error) {
	fail := func() (map[string]any, error) { return nil, fmt.Errorf("unsupported search filter") }
	if q.Scope.Source != c.config.Source || logs.ValidateQuery(q) != nil {
		return fail()
	}
	filters := []any{map[string]any{"range": map[string]any{"@timestamp": map[string]string{"gte": q.Window.Start.UTC().Format(time.RFC3339Nano), "lt": q.Window.End.UTC().Format(time.RFC3339Nano)}}}}
	if len(q.TraceIDs) > 0 {
		filters = append(filters, map[string]any{"terms": map[string]any{"trace.id": q.TraceIDs}})
	}
	for _, field := range q.Fields {
		native := nativeField(field.Path)
		if native == "" {
			return fail()
		}
		if field.Path == "/level" {
			var level string
			_ = json.Unmarshal(field.Value, &level) // Validated scalar type above.
			if level == "UNKNOWN" {
				filters = append(filters, unknownLevel())
			} else if slices.Contains(knownLevels, level) {
				filters = append(filters, levelTerm(level))
			} else {
				return fail()
			}
		} else {
			// Keep the validated scalar's JSON type and full integer precision.
			filters = append(filters, map[string]any{"term": map[string]any{native: field.Value}})
		}
	}
	if len(q.Scope.Services) > 0 {
		filters = append(filters, map[string]any{"terms": map[string]any{"service.name": q.Scope.Services}})
	}
	if len(q.Levels) > 0 {
		selected := []any{}
		for _, level := range q.Levels {
			if level == "UNKNOWN" {
				selected = append(selected, unknownLevel())
			} else if slices.Contains(knownLevels, level) {
				selected = append(selected, levelTerm(level))
			} else {
				return fail()
			}
		}
		filters = append(filters, map[string]any{"bool": map[string]any{"should": selected, "minimum_should_match": 1}})
	}
	// This is analyzed phrase matching, not a literal substring or case-sensitive
	// search. The caller must label it accordingly; unsupported modes never fall back.
	if q.Message.Text != "" {
		filters = append(filters, map[string]any{"match_phrase": map[string]string{"message": q.Message.Text}})
	}
	return map[string]any{"bool": map[string]any{"filter": filters}}, nil
}
func levelTerm(level string) any {
	return map[string]any{"term": map[string]any{"log.level": map[string]any{"value": level, "case_insensitive": true}}}
}

func unknownLevel() any {
	known := []any{}
	for _, level := range knownLevels {
		known = append(known, levelTerm(level))
	}
	return map[string]any{"bool": map[string]any{"must_not": known}}
}

func nativeField(path string) string {
	switch path {
	case "/service":
		return "service.name"
	case "/instance":
		return "service.node.name"
	case "/trace.id":
		return "trace.id"
	case "/level":
		return "log.level"
	case "/fields/job/id":
		return "fields.job.id"
	case "/fields/job/attempt":
		return "fields.job.attempt"
	case "/fields/job/retryable":
		return "fields.job.retryable"
	case "/fields/http/status_code":
		return "fields.http.status_code"
	case "/fields/order/id":
		return "fields.order.id"
	case "/fields/region":
		return "fields.region"
	default:
		return ""
	}
}

// Values only exposes the public permitted field name, never arbitrary native
// paths or caller-controlled aggregation DSL. Suggestions share every facet.
func (c *Client) Values(ctx context.Context, q logs.Query, field, keyword string) (logs.Values, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fail := func() (logs.Values, error) { return logs.Values{}, fmt.Errorf("Elasticsearch suggestions failed") }
	filter, err := c.filter(q)
	if err != nil || c.closed || field != "service" || len(keyword) > 128 {
		return fail()
	}
	limit := min(q.Limits.Events, 50)
	// Keyword input is literal, never a caller-controlled glob or regexp.
	literal := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`).Replace(keyword)
	query := map[string]any{"bool": map[string]any{"filter": []any{filter, map[string]any{"wildcard": map[string]any{"service.name": map[string]any{"value": "*" + literal + "*", "case_insensitive": true}}}}}}
	terms := map[string]any{"field": "service.name", "size": limit + 1, "order": map[string]string{"_key": "asc"}}
	// ECS names are scalar in this source. Explicit value allowlists also keep
	// suggestions scoped if a malformed document has multiple service values.
	if len(q.Scope.Services) > 0 {
		terms["include"] = q.Scope.Services
	}
	for _, predicate := range q.Fields {
		if predicate.Path == "/service" {
			var name string
			_ = json.Unmarshal(predicate.Value, &name)
			terms["include"] = []string{name}
		}
	}
	body := map[string]any{"size": 0, "query": query, "aggs": map[string]any{"values": map[string]any{"terms": terms}}}
	b, err := c.request(ctx, http.MethodPost, "/"+url.PathEscape(c.config.Target)+"/_search", body)
	if err != nil {
		return fail()
	}
	var result struct {
		TimedOut bool `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Aggs *struct {
			Values *struct {
				Other   int `json:"sum_other_doc_count"`
				Buckets []struct {
					Key string `json:"key"`
				} `json:"buckets"`
			} `json:"values"`
		} `json:"aggregations"`
	}
	if json.Unmarshal(b, &result) != nil || result.TimedOut || result.Shards.Failed != 0 || result.Aggs == nil || result.Aggs.Values == nil {
		return fail()
	}
	agg := result.Aggs.Values
	if len(agg.Buckets) > limit+1 {
		return fail()
	}
	out := logs.Values{More: len(agg.Buckets) > limit || agg.Other > 0}
	used := 0
	for _, bucket := range agg.Buckets {
		if len(out.Items) == limit {
			break
		}
		if len(bucket.Key) > 256 {
			return fail()
		}
		if !strings.Contains(strings.ToLower(bucket.Key), strings.ToLower(keyword)) || (len(q.Scope.Services) > 0 && !slices.Contains(q.Scope.Services, bucket.Key)) {
			continue
		}
		if used+len(bucket.Key) > q.Limits.Bytes {
			out.More = true
			out.Partial = true
			break
		}
		used += len(bucket.Key)
		out.Items = append(out.Items, bucket.Key)
	}
	return out, nil
}

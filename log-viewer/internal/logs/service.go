package logs

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Service is the only provider access path. This slice supports one explicitly
// configured source. MCP authorization is deliberately not implemented.
type Service struct {
	provider Provider
	policy   OutputPolicy
	paths    [][]string
	patterns []*regexp.Regexp
	mu       sync.Mutex
	sessions map[string]session
	closed   bool
}

type session struct {
	query         Query
	native        string
	expires       time.Time
	events, bytes int
}

func NewService(provider Provider, policy OutputPolicy) (*Service, error) {
	if provider == nil || policy.Source == "" || len(policy.Fields) == 0 {
		return nil, fmt.Errorf("an explicit provider, source and output fields are required")
	}
	s := &Service{provider: provider, policy: policy, sessions: make(map[string]session)}
	s.policy.Fields = append([]string(nil), policy.Fields...)
	for _, path := range policy.RedactPaths {
		parts, err := pointer(path)
		if err != nil {
			return nil, err
		}
		s.paths = append(s.paths, parts)
	}
	var err error
	s.patterns, err = compilePatterns(policy.Patterns)
	return s, err
}

func (s *Service) Search(ctx context.Context, q Query) (Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Page{}, fmt.Errorf("investigation service closed")
	}
	s.expire()
	return s.search(ctx, q, session{})
}

// ValidateQuery accepts structured filters, never backend syntax or offset-free time.
func ValidateQuery(q Query) error {
	if !q.Window.End.After(q.Window.Start) || q.Window.Start.IsZero() || q.Limits.Events < 1 || q.Limits.Events > 500 || q.Limits.Bytes < 1 || q.Limits.Bytes > 32<<20 {
		return fmt.Errorf("invalid time window or read budget")
	}
	if len(q.Scope.Services) > 32 || len(q.Levels) > 7 || len(q.Message.Text) > 4096 || (q.Message.Mode != "" && q.Message.Mode != "phrase") || q.Message.CaseSensitive || (q.Message.Text != "" && q.Message.Mode != "phrase") {
		return fmt.Errorf("unsupported or oversized search filter")
	}
	for _, app := range q.Scope.Services {
		if len(app) == 0 || len(app) > 256 || Clean(app) != app || strings.ContainsRune(app, '\n') {
			return fmt.Errorf("invalid exact application")
		}
	}
	for _, level := range q.Levels {
		if !slices.Contains([]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "UNKNOWN"}, level) {
			return fmt.Errorf("invalid severity")
		}
	}
	if len(q.TraceIDs) > 32 || len(q.Fields) > 16 {
		return fmt.Errorf("too many exact predicates")
	}
	for _, trace := range q.TraceIDs {
		if len(trace) == 0 || len(trace) > 256 || Clean(trace) != trace || strings.ContainsRune(trace, '\n') {
			return fmt.Errorf("invalid trace ID")
		}
	}
	for _, field := range q.Fields {
		if !slices.Contains(FilterFields(), field.Path) || len(field.Value) > 4096 {
			return fmt.Errorf("field is not filterable")
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(field.Value))
		decoder.UseNumber()
		if !json.Valid(field.Value) || decoder.Decode(&value) != nil {
			return fmt.Errorf("invalid scalar predicate")
		}
		switch field.Path {
		case "/fields/job/attempt", "/fields/http/status_code", "/fields/order/id":
			number, ok := value.(json.Number)
			if !ok {
				return fmt.Errorf("field requires integer")
			}
			if _, err := number.Int64(); err != nil {
				return fmt.Errorf("field requires int64")
			}
		case "/fields/job/retryable":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("field requires boolean")
			}
		default:
			text, ok := value.(string)
			if !ok || text == "" || len(text) > 256 || strings.Contains(text, "[REDACTED]") || Clean(text) != text || strings.ContainsRune(text, '\n') {
				return fmt.Errorf("field requires a complete safe string")
			}
			if field.Path == "/level" && !slices.Contains([]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "UNKNOWN"}, text) {
				return fmt.Errorf("invalid severity predicate")
			}
		}
	}
	return nil
}

// FilterFields is the explicit local source's indexed scalar contract. It is not
// arbitrary field discovery or a claim about an unidentified production schema.
func FilterFields() []string {
	return []string{"/service", "/instance", "/trace.id", "/level", "/fields/job/id", "/fields/job/attempt", "/fields/job/retryable", "/fields/http/status_code", "/fields/order/id", "/fields/region"}
}

func (s *Service) FilterFields() []string {
	var fields []string
	for _, field := range FilterFields() {
		parts, _ := pointer(field)
		if !slices.Contains(s.policy.Fields, parts[0]) {
			continue
		}
		denied := false
		for _, path := range s.paths {
			if len(path) > len(parts) {
				continue
			}
			match := true
			for i, part := range path {
				match = match && (part == "*" || part == parts[i])
			}
			denied = denied || match
		}
		if !denied {
			fields = append(fields, field)
		}
	}
	return fields
}

func (s *Service) validate(q Query) error {
	if q.Scope.Source != s.policy.Source {
		return fmt.Errorf("source access denied")
	}
	if err := ValidateQuery(q); err != nil {
		return err
	}
	if (len(q.Scope.Services) > 0 && !slices.Contains(s.policy.Fields, "service")) || (len(q.Levels) > 0 && !slices.Contains(s.policy.Fields, "level")) || (q.Message.Text != "" && !slices.Contains(s.policy.Fields, "message")) {
		return fmt.Errorf("filter field not available")
	}
	if len(q.TraceIDs) > 0 && !slices.Contains(s.FilterFields(), "/trace.id") {
		return fmt.Errorf("trace field not available")
	}
	for _, field := range q.Fields {
		if !slices.Contains(s.FilterFields(), field.Path) {
			return fmt.Errorf("filter field not available")
		}
		var text string
		_ = json.Unmarshal(field.Value, &text)
		for _, pattern := range s.patterns {
			if pattern.MatchString(text) {
				return fmt.Errorf("redacted predicate not available")
			}
		}
	}
	return nil
}

func (s *Service) search(ctx context.Context, q Query, prior session) (Page, error) {
	if err := s.validate(q); err != nil {
		return Page{}, err
	}
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	q.Scope.Services = slices.Clone(q.Scope.Services)
	q.Levels = slices.Clone(q.Levels)
	q.TraceIDs = slices.Clone(q.TraceIDs)
	q.Fields = slices.Clone(q.Fields)
	for i := range q.Fields {
		q.Fields[i].Value = slices.Clone(q.Fields[i].Value)
	}
	page, err := s.provider.Search(ctx, q, prior.native)
	if err != nil {
		s.closeNative(prior.native)
		return Page{}, fmt.Errorf("source read failed")
	}
	keep := false
	defer func() {
		if !keep {
			s.closeNative(page.Cursor)
		}
	}()
	if len(page.Events) > q.Limits.Events {
		return Page{}, fmt.Errorf("provider exceeded event budget")
	}
	result := Page{More: page.More, Partial: page.Partial}
	bytes := 0
	for _, event := range page.Events {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		safe, size, err := s.safeEvent(event)
		if err != nil {
			return Page{}, err
		}
		if bytes+size > q.Limits.Bytes || prior.events+len(result.Events) >= 10000 || prior.bytes+bytes+size > 32<<20 {
			result.More = true
			result.Partial = true
			break
		}
		bytes += size
		safe.Handle = rand.Text()
		result.Events = append(result.Events, safe)
	}
	prior.events += len(result.Events)
	prior.bytes += bytes
	if (prior.events >= 10000 || prior.bytes >= 32<<20) && result.More {
		result.Partial = true
	}
	if page.Cursor != "" && result.More && !result.Partial {
		if len(s.sessions) >= 16 {
			var oldest string
			for key, entry := range s.sessions {
				if oldest == "" || entry.expires.Before(s.sessions[oldest].expires) {
					oldest = key
				}
			}
			s.closeNative(s.sessions[oldest].native)
			delete(s.sessions, oldest)
		}
		result.Next = rand.Text()
		s.sessions[result.Next] = session{query: q, native: page.Cursor, expires: time.Now().Add(2 * time.Minute), events: prior.events, bytes: prior.bytes}
		keep = true
	}
	return result, nil
}

// Next consumes a process-local handle once. Callers cannot rewrite its query.
func (s *Service) Next(ctx context.Context, handle string) (Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Page{}, fmt.Errorf("investigation service closed")
	}
	s.expire()
	entry, ok := s.sessions[handle]
	if !ok {
		return Page{}, fmt.Errorf("history expired or closed; reload required")
	}
	delete(s.sessions, handle)
	if err := ctx.Err(); err != nil {
		s.closeNative(entry.native)
		return Page{}, err
	}
	return s.search(ctx, entry.query, entry)
}

func (s *Service) closeNative(cursor string) {
	if p, ok := s.provider.(HistoryProvider); ok && cursor != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.Close(ctx, cursor)
	}
}

func (s *Service) expire() {
	for key, entry := range s.sessions {
		if !time.Now().Before(entry.expires) {
			s.closeNative(entry.native)
			delete(s.sessions, key)
		}
	}
}

func (s *Service) Close(_ context.Context, handle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.sessions[handle]; ok {
		s.closeNative(entry.native)
		delete(s.sessions, handle)
	}
	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for key, entry := range s.sessions {
		s.closeNative(entry.native)
		delete(s.sessions, key)
	}
	return nil
}

func (s *Service) Values(ctx context.Context, q Query, field, keyword string) (Values, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Values{}, fmt.Errorf("investigation service closed")
	}
	if q.Scope.Source != s.policy.Source || field != "service" || !slices.Contains(s.policy.Fields, "service") {
		return Values{}, fmt.Errorf("suggestion field not available")
	}
	if err := s.validate(q); err != nil {
		return Values{}, err
	}
	if len(keyword) > 256 || Clean(keyword) != keyword || strings.ContainsRune(keyword, '\n') {
		return Values{}, fmt.Errorf("invalid suggestion keyword")
	}
	p, ok := s.provider.(HistoryProvider)
	if !ok {
		return Values{}, fmt.Errorf("suggestions unavailable")
	}
	values, err := p.Values(ctx, q, field, keyword)
	if err != nil || len(values.Items) > 100 {
		return Values{}, fmt.Errorf("source suggestions failed")
	}
	out := Values{More: values.More, Partial: values.Partial}
	for _, value := range values.Items {
		if len(value) > 256 || Clean(value) != value || strings.ContainsRune(value, '\n') {
			out.Partial = true
			continue
		}
		allowed := true
		for _, pattern := range s.patterns {
			if pattern.MatchString(value) {
				allowed = false
			}
		}
		if allowed {
			out.Items = append(out.Items, value)
		} else {
			out.Partial = true
		}
	}
	return out, nil
}

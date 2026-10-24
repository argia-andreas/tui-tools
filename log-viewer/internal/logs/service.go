package logs

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
)

// Service is the only provider access path. This slice supports one explicitly
// configured fixture source. MCP authorization is deliberately not implemented.
type Service struct {
	provider Provider
	policy   OutputPolicy
	paths    [][]string
	patterns []*regexp.Regexp
}

func NewService(provider Provider, policy OutputPolicy) (*Service, error) {
	if provider == nil || policy.Source == "" || len(policy.Fields) == 0 {
		return nil, fmt.Errorf("an explicit provider, source and output fields are required")
	}
	s := &Service{provider: provider, policy: policy}
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
	if q.Scope.Source != s.policy.Source {
		return Page{}, fmt.Errorf("source access denied")
	}
	if !q.Window.End.After(q.Window.Start) || q.Limits.Events < 1 || q.Limits.Events > 500 || q.Limits.Bytes < 1 || q.Limits.Bytes > 32<<20 {
		return Page{}, fmt.Errorf("invalid time window or read budget")
	}
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	page, err := s.provider.Search(ctx, q, "")
	if err != nil {
		return Page{}, fmt.Errorf("source read failed")
	}
	if len(page.Events) > q.Limits.Events {
		return Page{}, fmt.Errorf("provider exceeded event budget")
	}
	result := Page{More: page.More}
	bytes := 0
	for _, event := range page.Events {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		safe, size, err := s.safeEvent(event)
		if err != nil {
			return Page{}, err
		}
		if bytes+size > q.Limits.Bytes {
			result.More = true
			break
		}
		bytes += size
		safe.Handle = rand.Text()
		result.Events = append(result.Events, safe)
	}
	return result, nil
}

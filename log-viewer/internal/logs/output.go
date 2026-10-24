package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// OutputPolicy is operator-owned source output policy, not an MCP grant.
type OutputPolicy struct {
	Source      string
	Fields      []string
	RedactPaths []string // JSON pointers into the output document, including /fields
	Patterns    []string
}

// Clean removes terminal instructions before trusted UI styling is applied.
// Newlines remain data for multiline messages; tabs can't move the terminal cursor.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, ansi.Strip(strings.ToValidUTF8(s, "�")))
}

func pointer(s string) ([]string, error) {
	if !strings.HasPrefix(s, "/") {
		return nil, fmt.Errorf("redaction paths must be JSON pointers")
	}
	parts := strings.Split(s[1:], "/")
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				if j+1 == len(part) || (part[j+1] != '0' && part[j+1] != '1') {
					return nil, fmt.Errorf("invalid JSON pointer escape")
				}
				j++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func (s *Service) safeValue(v any, path []string) any {
	for _, denied := range s.paths {
		if len(path) != len(denied) {
			continue
		}
		match := true
		for i := range path {
			match = match && (path[i] == denied[i] || denied[i] == "*")
		}
		if match {
			return "[REDACTED]"
		}
	}
	switch v := v.(type) {
	case string:
		for _, pattern := range s.patterns {
			v = pattern.ReplaceAllString(v, "[REDACTED]")
		}
		v = Clean(v)
		// Cleanup can join an escape-obfuscated secret back together. Policy
		// must cover both the incoming text and the actual retained text.
		for _, pattern := range s.patterns {
			v = pattern.ReplaceAllString(v, "[REDACTED]")
		}
		return v
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			// A key is data too. Drop unsafe keys rather than renaming/colliding paths.
			if Clean(key) != key || strings.ContainsRune(key, '\n') {
				continue
			}
			redactedKey := false
			for _, pattern := range s.patterns {
				redactedKey = redactedKey || pattern.MatchString(key)
			}
			if !redactedKey {
				out[key] = s.safeValue(value, append(append([]string{}, path...), key))
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, value := range v {
			out[i] = s.safeValue(value, append(append([]string{}, path...), fmt.Sprint(i)))
		}
		return out
	default:
		return v
	}
}

func (s *Service) safeEvent(e Event) (SafeEvent, int, error) {
	doc := map[string]any{
		"time":  e.Time.UTC().Format("2006-01-02T15:04:05.000Z"),
		"level": e.Level, "service": e.Service, "instance": e.Instance,
		"message": e.Message, "trace.id": e.TraceID, "fields": e.Fields,
	}
	if e.Level == "" {
		doc["level"] = "UNKNOWN"
	}
	encoded, err := json.Marshal(doc)
	if err != nil || len(encoded) > 1<<20 {
		return SafeEvent{}, 0, fmt.Errorf("provider event exceeds output limits or has invalid fields")
	}
	var tree map[string]any
	d := json.NewDecoder(bytes.NewReader(encoded))
	d.UseNumber()
	if err := d.Decode(&tree); err != nil {
		return SafeEvent{}, 0, fmt.Errorf("invalid provider event")
	}
	safe := s.safeValue(tree, nil).(map[string]any)
	result := SafeEvent{Values: make(map[string]json.RawMessage)}
	bytes := 0
	for _, field := range s.policy.Fields {
		if value, ok := safe[field]; ok {
			data, err := json.Marshal(value)
			if err != nil {
				return SafeEvent{}, 0, fmt.Errorf("could not encode safe event")
			}
			result.Values[field] = data
			bytes += len(data) + len(field)
		}
	}
	return result, bytes, nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	var result []*regexp.Regexp
	for _, pattern := range patterns {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid source redaction pattern")
		}
		result = append(result, r)
	}
	return result, nil
}

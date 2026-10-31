package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// highlightJSON colors safe display text from an already-validated JSON source.
// A formatted line need not itself be a complete JSON document. In particular,
// this lexer neither decodes string escapes nor interprets terminal sequences.
func highlightJSON(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); {
		start := i
		color := ""
		switch text[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '{', '}', '[', ']', ':', ',':
			i++
			color = "#62758b"
		case '"':
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i = min(i+2, len(text))
					continue
				}
				if text[i] == '"' {
					i++
					color = "#a6d7a5"
					break
				}
				i++
			}
			if color != "" {
				j := i
				for j < len(text) && strings.ContainsRune(" \t\r\n", rune(text[j])) {
					j++
				}
				if j < len(text) && text[j] == ':' {
					color = "#7dcfff"
				}
			}
		default:
			// Consume the whole bare span so unknown text such as "trueish"
			// or "12oops" cannot acquire a misleading colored prefix.
			for i < len(text) && !strings.ContainsRune(" \t\r\n{}[]:,\"", rune(text[i])) {
				i++
			}
			token := text[start:i]
			switch {
			case token == "true" || token == "false":
				color = "#bb9af7"
			case token == "null":
				color = "#df9ba9"
			case jsonNumber(token):
				color = "#e5bd78"
			}
		}
		token := text[start:i]
		if color == "" {
			out.WriteString(token)
		} else {
			// Leave color-profile and NO_COLOR decisions to the existing
			// lipgloss renderer; do not set a background or expand tabs.
			out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).TabWidth(-1).Render(token))
		}
	}
	return out.String()
}

func jsonNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	if i == len(s) {
		return false
	}
	if s[i] == '0' {
		i++
	} else {
		if s[i] < '1' || s[i] > '9' {
			return false
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i < len(s) && s[i] == '.' {
		i++
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	return i == len(s)
}

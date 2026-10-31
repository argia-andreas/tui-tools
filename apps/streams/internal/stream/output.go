package stream

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SafeText never lets broker-controlled text become terminal instructions.
func SafeText(raw []byte) string {
	var b strings.Builder
	for len(raw) > 0 {
		r, n := utf8.DecodeRune(raw)
		if r == utf8.RuneError && n == 1 {
			fmt.Fprintf(&b, "\\x%02x", raw[0])
		} else if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			b.WriteString(strconv.QuoteRune(r)[1 : len(strconv.QuoteRune(r))-1])
		} else {
			b.WriteRune(r)
		}
		raw = raw[n:]
	}
	return b.String()
}

func FieldPayload(r Record, field string, index int) ([]byte, error) {
	var data []byte
	switch field {
	case "key":
		data = r.Key
	case "value":
		data = r.Value
	case "header":
		if index < 0 || index >= len(r.Headers) {
			return nil, fmt.Errorf("select a header")
		}
		data = r.Headers[index].Value
	default:
		return nil, fmt.Errorf("select key, value or a header")
	}
	if data == nil {
		return nil, fmt.Errorf("null has no bytes to copy")
	}
	return bytes.Clone(data), nil
}

// Content renders a bounded window; raw record bytes remain unchanged.
func Content(data []byte, mode string) []string {
	if data == nil {
		return []string{"<null / tombstone when value>"}
	}
	if len(data) == 0 {
		return []string{"<empty: 0 bytes>"}
	}
	const capBytes = 256 << 10
	clipped := len(data) > capBytes
	window := data[:min(len(data), capBytes)]
	var lines []string
	switch mode {
	case "hex":
		lines = strings.Split(strings.TrimSuffix(hex.Dump(window), "\n"), "\n")
	case "json":
		depth, maxDepth := 0, 0
		dec := json.NewDecoder(bytes.NewReader(window))
		for {
			t, err := dec.Token()
			if err != nil {
				break
			}
			if d, ok := t.(json.Delim); ok {
				if d == '{' || d == '[' {
					depth++
					maxDepth = max(maxDepth, depth)
				} else {
					depth--
				}
			}
		}
		var pretty bytes.Buffer
		if !clipped && maxDepth <= 64 && json.Indent(&pretty, window, "", "  ") == nil && pretty.Len() <= capBytes {
			for _, line := range strings.Split(pretty.String(), "\n") {
				lines = append(lines, SafeText([]byte(line)))
			}
		} else {
			lines = []string{"JSON unavailable (invalid, deep or large); raw window:", SafeText(window)}
		}
	default:
		for _, line := range bytes.Split(window, []byte{'\n'}) {
			lines = append(lines, SafeText(line))
		}
	}
	if clipped {
		lines = append(lines, "[display limited to 256 KiB; copy raw explicitly for full bytes]")
	}
	return lines
}

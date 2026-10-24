// Package theme preserves LazyFTP's semantic Midnight palette without importing
// its application internals. Provenance: lazyftp/internal/theme/theme.go and
// docs/plans/log-viewer/design-references.md. No theme-file import is implied.
package theme

type Theme struct {
	Background, Surface, Foreground, Muted  string
	Border, Focus, Selection, SelectionText string
	Warning, Error                          string
}

func Midnight() Theme {
	return Theme{
		Background: "#0b1423", Surface: "#101d2f", Foreground: "#dce8f4", Muted: "#91a6bb",
		Border: "#4b6a89", Focus: "#65c7f7", Selection: "#8af0ce", SelectionText: "#071a24",
		Warning: "#efc46f", Error: "#f47d91",
	}
}

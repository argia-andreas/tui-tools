package theme_test

import (
	"math"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"servicebusexplorer/internal/theme"
)

func TestPresets(t *testing.T) {
	wantNames := []string{"midnight", "gruvbox", "light"}
	if got := theme.Names(); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("Names() = %v, want %v", got, wantNames)
	}
	valid := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	for _, name := range wantNames {
		t.Run(name, func(t *testing.T) {
			preset, err := theme.Get(name)
			if err != nil {
				t.Fatal(err)
			}
			if preset.Name != name {
				t.Fatalf("Name = %q, want %q", preset.Name, name)
			}
			for role, color := range map[string]string{
				"background": preset.Background, "surface": preset.Surface,
				"foreground": preset.Foreground, "muted": preset.Muted,
				"border": preset.Border, "focus": preset.Focus,
				"selection": preset.Selection, "selection text": preset.SelectionText,
				"queued": preset.Queued, "dead letter": preset.DeadLetter, "warning": preset.Warning,
			} {
				if !valid.MatchString(color) {
					t.Fatalf("%s = %q, want #RRGGBB", role, color)
				}
			}
			for _, background := range []string{preset.Background, preset.Surface} {
				for role, foreground := range map[string]string{
					"foreground": preset.Foreground, "muted": preset.Muted,
					"queued": preset.Queued, "dead letter": preset.DeadLetter, "warning": preset.Warning,
				} {
					if ratio := contrast(foreground, background); ratio < 4.5 {
						t.Errorf("%s on %s contrast = %.2f, want >= 4.5", role, background, ratio)
					}
				}
			}
			if ratio := contrast(preset.SelectionText, preset.Selection); ratio < 4.5 {
				t.Errorf("selection text contrast = %.2f, want >= 4.5", ratio)
			}
		})
	}
	want, err := theme.Get("midnight")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := theme.Get(""); err != nil || got != want {
		t.Fatalf("Get(empty) = %+v, %v; want midnight", got, err)
	}
	// Neither the returned list nor a customized Theme may mutate a preset.
	names := theme.Names()
	names[0] = "changed"
	custom := want
	custom.Background = "#ffffff"
	if got, err := theme.Get("midnight"); err != nil || got != want || theme.Names()[0] != "midnight" {
		t.Fatal("caller changes affected presets")
	}
}

func TestInvalidNames(t *testing.T) {
	for _, name := range []string{"unknown", "Midnight", " midnight", "midnight ", "gruvbox-dark", "../light", "light\n"} {
		t.Run(name, func(t *testing.T) {
			if _, err := theme.Get(name); err == nil {
				t.Fatalf("Get(%q) accepted an invalid name", name)
			}
		})
	}
}

// contrast computes the WCAG sRGB relative-luminance ratio for public colors.
func contrast(a, b string) float64 {
	luminance := func(color string) float64 {
		var result float64
		for i, weight := range []float64{0.2126, 0.7152, 0.0722} {
			n, _ := strconv.ParseUint(color[1+2*i:3+2*i], 16, 8)
			channel := float64(n) / 255
			if channel <= 0.04045 {
				channel /= 12.92
			} else {
				channel = math.Pow((channel+0.055)/1.055, 2.4)
			}
			result += channel * weight
		}
		return result
	}
	x, y := luminance(a), luminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

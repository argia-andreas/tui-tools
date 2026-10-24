package ui_test

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"logviewer/internal/fixture"
	"logviewer/internal/logs"
	"logviewer/internal/theme"
	"logviewer/internal/ui"
)

type multilineProvider struct{}

func (multilineProvider) Search(context.Context, logs.Query, string) (logs.NativePage, error) {
	return logs.NativePage{Events: []logs.Event{{
		Time:  fixture.Query().Window.Start,
		Level: "ERROR\nnot another event", Service: "checkout-api\nmetadata",
		Message: "First message line\nSecond message line\n" + strings.Repeat("long UTF-8 Åsa ", 100),
	}}}, nil
}

func TestMultilineDataCannotBreakLayout(t *testing.T) {
	s, err := logs.NewService(multilineProvider{}, fixture.Policy())
	if err != nil {
		t.Fatal(err)
	}
	m := ui.New(ui.Options{Service: s, Query: fixture.Query(), Theme: theme.Midnight(), Context: context.Background()})
	apply := func(msg tea.Msg) { next, _ := m.Update(msg); m = next.(ui.Model) }
	apply(m.Init()())
	for _, width := range []int{60, 80, 119, 120, 139, 140, 160} {
		for _, height := range []int{16, 24, 48} {
			apply(tea.WindowSizeMsg{Width: width, Height: height})
			for _, key := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyEnter}, {Type: tea.KeyRunes, Runes: []rune{'G'}}, {Type: tea.KeyEsc}} {
				apply(key)
				lines := strings.Split(ansi.Strip(m.View()), "\n")
				if len(lines) > height {
					t.Fatalf("%dx%d: data expanded frame to %d lines", width, height, len(lines))
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > width {
						t.Fatalf("%dx%d: row escaped frame", width, height)
					}
				}
			}
		}
	}
}

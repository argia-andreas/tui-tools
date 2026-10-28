package ui_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"lazyftp/internal/storage"
	"lazyftp/internal/ui"
)

func TestWorkspaceFileClickSelectsAndExplicitInspectDoesNotRead(t *testing.T) {
	reads := 0
	d := start(t, previewBrowser{read: func(context.Context, storage.Location, int64) ([]byte, error) {
		reads++
		return []byte("explicit content"), nil
	}})
	d.click(t, "second.txt")
	if strings.Contains(d.screen(), "INSPECTOR") || reads != 0 {
		t.Fatal("file row activated instead of selecting")
	}
	d.click(t, "Open/Inspect")
	requireText(t, d, "INSPECTOR")
	if reads != 0 {
		t.Fatal("inspection performed an unrequested content read")
	}
	// This hint is an actual control, not an inert copy of the p shortcut.
	d.click(t, "p / Actions → Preview")
	requireText(t, d, "explicit content")
	if reads != 1 {
		t.Fatalf("press/release performed %d reads", reads)
	}
}

func TestWorkspaceWheelKeepsPointedPaneLocalAndEndpointRemembered(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("/")
	d.key("README")
	d.key("enter")
	// Wheel changes the right cursor without moving focus away from the left.
	d.deliver(tea.MouseMsg{X: 110, Y: 5, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	d.key("enter")
	parts := strings.Split(d.screen(), "INSPECTOR")
	if len(parts) != 2 || !strings.Contains(parts[1], "README.md") || !strings.Contains(parts[1], "/project") {
		t.Fatal("wheel stole file endpoint/focus")
	}
	d.key("ctrl+l")
	d.key("enter")
	parts = strings.Split(d.screen(), "INSPECTOR")
	if len(parts) != 2 || !strings.Contains(parts[1], "résumé.pdf") {
		t.Fatal("wheel did not move pointed right pane by three rows")
	}
}

func TestWorkspaceFilterPointerApplyAndClear(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.click(t, "/ Filter")
	d.key("配置")
	d.click(t, "Enter apply")
	requireText(t, d, "配置.json")
	if strings.Contains(d.screen(), "Filter:") || strings.Contains(d.screen(), "README.md") {
		t.Fatal("pointer Apply did not retain filter")
	}
	d.click(t, "/ Filter")
	d.click(t, "Esc clear")
	requireText(t, d, "README.md")
}

func TestInlineRenameHasPointerReviewCancelAndKeyboardTraversal(t *testing.T) {
	source := storage.Location{Path: "/local/original.txt"}
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{source: "retain"}}}
	d := renameDriver(t, s)
	d.key("R")
	d.deliver(tea.KeyMsg{Type: tea.KeyCtrlU})
	d.key("renamed.txt")
	d.click(t, "[ Review ]")
	requireText(t, d, "RENAME FILE?")
	d.click(t, "[ Cancel ]")
	d.key("R")
	d.key("tab")
	d.key("tab")
	d.key("enter")
	requireText(t, d, "Rename canceled")
	if len(s.files) != 1 || s.files[source] != "retain" {
		t.Fatal("inline cancellation mutated files")
	}
}

func TestWorkspaceTabReachesVisibleHeaderControlAndRepairsHiddenPane(t *testing.T) {
	d := start(t, storage.NewDemo())
	d.key("tab") // right pane
	d.key("tab") // Actions: visible controls now follow pane stops.
	d.key("enter")
	requireText(t, d, "ACTIONS · Ctrl+P")
	d.key("esc")
	d.key("]")
	d.key("ctrl+l")
	d.key("tab") // inspector
	d.deliver(tea.WindowSizeMsg{Width: 80, Height: 24})
	d.key("enter")
	requireText(t, d, "shipment.json") // repaired focus returned to remembered right pane
	if strings.Contains(d.screen(), "ACTIONS · Ctrl+P") {
		t.Fatal("hidden focus activated another control")
	}
}

func TestWorkspaceRefreshRetainsNativeSelectedEntryAcrossReorder(t *testing.T) {
	reversed := false
	d := start(t, browserFunc(func(_ context.Context, loc storage.Location) ([]storage.Entry, error) {
		if loc.Path != "/project" {
			return []storage.Entry{{Name: "nested.txt"}}, nil
		}
		a, b := storage.Entry{Name: "alpha", IsDir: true}, storage.Entry{Name: "beta", IsDir: true}
		if reversed {
			return []storage.Entry{b, a}, nil
		}
		return []storage.Entry{a, b}, nil
	}))
	d.click(t, "beta/")
	reversed = true
	d.key("ctrl+r")
	d.key("enter")
	requireText(t, d, "LOCAL /project/beta")
	requireText(t, d, "nested.txt")
}

func TestWorkspaceListingErrorRetryHintUsesCurrentCheckedRefresh(t *testing.T) {
	failing := true
	d := start(t, browserFunc(func(context.Context, storage.Location) ([]storage.Entry, error) {
		if failing {
			return nil, errors.New("offline listing")
		}
		return []storage.Entry{{Name: "recovered.txt"}}, nil
	}))
	requireText(t, d, "offline listing")
	failing = false
	d.click(t, "Ctrl+R retry")
	requireText(t, d, "recovered.txt")
}

func TestDetachedSourcePointerKeepsChoiceForKeyboardReactivation(t *testing.T) {
	s := &renameStorage{memoryStorage{files: map[storage.Location]string{{Path: "/local/keep.txt"}: "keep"}}}
	d := renameDriver(t, s)
	d.click(t, "[ Detach ]")
	d.click(t, "[ New connection ]")
	_, cmd := d.model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("source selection did not retain keyboard activation route")
	}
	request, ok := cmd().(ui.ConnectRequested)
	if !ok || !request.New || request.Pane != 0 {
		t.Fatalf("keyboard chose a different source: %+v", request)
	}
}

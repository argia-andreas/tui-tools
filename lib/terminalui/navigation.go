package terminalui

import tea "github.com/charmbracelet/bubbletea"

// NextFocus wraps through the supplied current visible IDs. An absent current
// ID starts at the first (last backwards); an empty list returns an empty ID.
// Applications own focus-transition effects and must filter visibility first.
func NextFocus(visible []string, current string, backwards bool) string {
	if len(visible) == 0 {
		return ""
	}
	for i, id := range visible {
		if id == current {
			if backwards {
				return visible[(i+len(visible)-1)%len(visible)]
			}
			return visible[(i+1)%len(visible)]
		}
	}
	if backwards {
		return visible[len(visible)-1]
	}
	return visible[0]
}

// HandleNavigation interprets the current visible workspace frame. Pane, control
// and destination regions are Tab stops in frame order; data rows are not. Apps
// own selection, cursor movement, focus-transition effects and checked activation.
// Recompute the frame after resize or data changes, using native identity IDs.
func HandleNavigation(msg tea.Msg, frame Frame, focusID, selectedID string) Result {
	var regions []Region
	var order []string
	seen := map[string]bool{}
	for _, region := range frame.Regions {
		clipped := intersection(region.Bounds, frame.Bounds)
		if region.ID == "" || clipped.W == 0 || clipped.H == 0 {
			continue
		}
		if (region.Kind == ControlRegion || region.Kind == DestinationRegion) && clipped != region.Bounds {
			continue
		}
		region.Bounds = clipped
		regions = append(regions, region)
		if region.Kind != RowRegion && region.Kind != FieldRegion && !seen[region.ID] {
			order = append(order, region.ID)
			seen[region.ID] = true
		}
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab", "shift+tab":
			if next := NextFocus(order, focusID, key.String() == "shift+tab"); next != "" {
				return request(Focus, next)
			}
		case "enter":
			for _, region := range regions {
				if region.ID == focusID && (region.Kind == ControlRegion || region.Kind == DestinationRegion) {
					return request(Activate, region.ID)
				}
			}
			for _, pane := range regions {
				if pane.ID != focusID || pane.Kind != PaneRegion {
					continue
				}
				for _, row := range regions {
					if row.ID == selectedID && (row.Kind == RowRegion || row.Kind == DestinationRegion) && intersection(row.Bounds, pane.Bounds) == row.Bounds {
						return request(Activate, row.ID)
					}
				}
				return Result{Consumed: true}
			}
		}
		return Result{}
	}
	mouse, ok := msg.(tea.MouseMsg)
	if !ok || mouse.Action != tea.MouseActionPress || !contains(frame.Bounds, mouse.X, mouse.Y) {
		return Result{}
	}
	if mouse.Button == tea.MouseButtonWheelUp || mouse.Button == tea.MouseButtonWheelDown {
		for _, pane := range regions {
			if pane.Kind == PaneRegion && contains(pane.Bounds, mouse.X, mouse.Y) {
				delta := 3
				if mouse.Button == tea.MouseButtonWheelUp {
					delta = -3
				}
				return Result{Consumed: true, Intent: &Intent{Kind: Scroll, ID: pane.ID, Delta: delta}}
			}
		}
		return Result{}
	}
	if mouse.Button != tea.MouseButtonLeft {
		return Result{}
	}
	for _, kind := range []RegionKind{ControlRegion, DestinationRegion, FieldRegion, RowRegion, PaneRegion} {
		for _, region := range regions {
			if region.Kind != kind || !contains(region.Bounds, mouse.X, mouse.Y) {
				continue
			}
			intent := Focus
			if kind == RowRegion {
				intent = Select
			} else if kind == ControlRegion || kind == DestinationRegion {
				intent = Activate
			}
			return request(intent, region.ID)
		}
	}
	return Result{}
}

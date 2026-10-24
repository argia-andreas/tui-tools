# Architecture: LazyFTP opening screen

User approved the displayed mockup and implementation on 2026-10-24, waiving further approval pauses for this scope.

## Fit
The existing Bubble Tea picker owns rendering, input, row hit testing and the retained connection details. Keep these responsibilities together. The workspace still owns discovery, wizard routing and transitions to files. The existing wizard and file UI are unchanged.

## Endpoints / data / external
None added. Existing bookmark/configured-remote ordering and authentication contracts remain intact. Theme colors continue to come from the shared semantic theme.

## Flow
Startup opens a loading picker. Discovery replaces its choices without stealing a user's selected action. Up/Down, j/k, optional Tab aliases and wheel input traverse one logical sequence of entries and actions. Enter routes through existing details, wizard or local browsing behavior. Rendering and mouse hit testing continue to share row geometry.

## Tradeoffs
Retain current detail-form focus identifiers rather than inventing a second selection framework. Translate action focus to a logical index only when moving through the chooser. Keep an explicit scroll offset for a stable viewport, including backtracking from actions. Loading is distinct from empty/error; detailed discovery diagnostics are only shown in help.

package dash

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Render draws a screen: rows stacked by weight, panels within a row sharing the
// width equally.
//
// This is the only place that decides how much room anything gets. Five views
// used to each work that out for themselves, which is why switching tabs moved
// the gauges by a line.
func Render(screen Screen, src Source, width, height int) string {
	if len(screen.Rows) == 0 {
		return ""
	}

	heights := Heights(screen.Rows, height)

	rendered := make([]string, 0, len(screen.Rows))
	for i, row := range screen.Rows {
		rendered = append(rendered, renderRow(row, src, width, heights[i]))
	}
	return strings.Join(rendered, "\n")
}

func renderRow(row Row, src Source, width, height int) string {
	if len(row.Widgets) == 0 {
		return strings.Repeat("\n", max(height-1, 0))
	}

	// The last panel takes the remainder, so a row always fills the width
	// exactly rather than leaving a ragged column on the right.
	each := width / len(row.Widgets)
	panels := make([]string, 0, len(row.Widgets))
	for i, w := range row.Widgets {
		panelWidth := each
		if i == len(row.Widgets)-1 {
			panelWidth = width - each*(len(row.Widgets)-1)
		}
		panels = append(panels, w.Render(src, panelWidth, height))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, panels...)
}

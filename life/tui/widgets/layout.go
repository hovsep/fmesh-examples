package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

// Every row carrying a bar -- a vital sign's sparkline, a gauge, an organ's
// damage, a felt sensation -- is laid out on one grid:
//
//	label            value   unit   bar                      marker
//	|<--- 16 --->| |<- 6 ->| |<-6->| |<-- what is left -->| |<- 7 ->|
//
// Fixed columns are what makes the bars line up. Sized per row, as they were,
// a metric whose value happened to be a digit wider pushed its bar out of step
// with the row above it, and two panels of the same width disagreed about where
// their bars started and how long they were.
//
// The marker column is fixed even though most rows only put a single health
// mark in it, because the alternative is that a view with a longer marker -- the
// Body view's status word -- gets shorter bars than the view beside it, and the
// bars stop agreeing the moment you switch tabs.
const (
	LabelWidth  = 16
	ValueWidth  = 6
	UnitWidth   = 6
	MarkerWidth = 7
	MinBarWidth = 8

	// columnGap is the single space between columns.
	columnGap = 1

	// rowOverhead is everything on a row that is not the bar.
	rowOverhead = LabelWidth + ValueWidth + UnitWidth + MarkerWidth + 4*columnGap
)

// BarWidth returns how wide the bar on a row of total display width should be.
// Every widget sizes its bar with this, so rows of equal width agree however
// they are otherwise built.
func BarWidth(total int) int {
	return max(total-rowOverhead, MinBarWidth)
}

// Marker fits a row's trailing mark -- a health symbol, a status word -- to its
// column, so what precedes it is the same width on every row.
func Marker(s string) string { return padRight(truncate(s, MarkerWidth), MarkerWidth) }

// IconWidth is the column an emoji sits in. Emoji are drawn two cells wide, but
// not all of them measure two, so the column is padded to size rather than
// trusted to be it.
const IconWidth = 2

// Icon fits a glyph to the icon column, which is carved out of the label
// column: a row with an icon still puts its name, value and bar where a row
// without one does.
func Icon(s string) string { return padRight(s, IconWidth) }

// Bar renders a proportional bar exactly width cells wide: the filled part in
// the given style, the rest as a track that recedes.
func Bar(fraction float64, width int, filled lipgloss.Style) string {
	cells := min(max(int(fraction*float64(width)), 0), width)
	return filled.Render(strings.Repeat(styles.ProgressFullChar, cells)) +
		trackStyle.Render(strings.Repeat(styles.ProgressEmptyChar, width-cells))
}

// trackStyle draws the unfilled part of every bar. It is dimmer than any value
// so a bar reads as one shape at a glance, rather than as two competing ones.
var trackStyle = lipgloss.NewStyle().Foreground(styles.ColorBorder)

// Label fits a name to the label column, padding it out or, when it is too
// long, cutting it short rather than shoving the whole row sideways.
func Label(s string) string { return padRight(truncate(s, LabelWidth), LabelWidth) }

// Value right-aligns a reading in the value column, so the digits line up under
// each other and the columns after it never move.
func Value(s string) string { return padLeft(truncate(s, ValueWidth), ValueWidth) }

// Unit fits a unit to its column.
func Unit(s string) string { return padRight(truncate(s, UnitWidth), UnitWidth) }

// padRight, padLeft and truncate all measure in display cells (lipgloss.Width),
// so units with multi-byte glyphs like "cmH₂O" and "°C" still line up.
func padRight(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func padLeft(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// truncate shortens s to w cells, marking the cut with an ellipsis so a clipped
// label reads as clipped rather than as a different word.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w || w <= 0 {
		return s
	}
	if w == 1 {
		return "…"
	}

	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)) > w-1 {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}

// Package dash is the dashboard as a description rather than as code.
//
// A screen is rows of panels, each panel a widget with a metric or two named on
// it. That is the whole model, and it is deliberately less than a real
// dashboarding tool offers: no queries, no transformations, no per-panel
// options beyond what a widget needs to draw itself. What it buys is that
// adding a screen is a literal in a table rather than a file, and that every
// screen is laid out by one renderer instead of by five that each round their
// heights differently.
//
// Widgets read through Source, which is the single aggregated telemetry stream
// the simulation publishes -- every value the body reports arrives keyed by
// "<subject>::<port>", and the catalog says how to draw each one. A widget
// therefore names a metric and nothing else: no widget knows where a number came
// from, and none can ask the body for anything the body has not published.
package dash

import (
	"time"

	"github.com/hovsep/fmesh-examples/simulation/life/telemetry"
)

// Source is what a widget may read: the aggregated stream, and the catalog that
// describes it.
type Source interface {
	// Value is the latest reading of a metric, by its catalog port name.
	Value(metric string) float64

	// Series is the recent history of a metric, oldest first.
	Series(metric string) []float64

	// Scalar is one named number carried inside a composite signal.
	Scalar(metric, scalar string) float64

	// Display is how the catalog says to draw a metric.
	Display(metric string) (telemetry.Display, bool)

	// Elapsed is how long the body has been simulated.
	Elapsed() time.Duration

	// Alive reports whether the body is still living, which several widgets
	// draw differently.
	Alive() bool
}

// Widget draws one panel.
//
// Height is what the row allotted it; a widget that wants less should still
// return that many lines, since the renderer stacks rows by count rather than
// measuring them.
type Widget interface {
	Render(src Source, width, height int) string
}

// Row is a horizontal band of panels sharing the width equally.
//
// Weight is the row's share of the vertical space, so a waveform can be given
// three times the height of the gauges beneath it without anybody counting
// lines. A zero weight means one.
type Row struct {
	Weight  int
	Widgets []Widget
}

// Screen is one tab.
type Screen struct {
	View  telemetry.View
	Title string
	Rows  []Row
}

// Heights divides the available lines between rows by weight.
//
// The remainder goes to the earliest rows rather than being dropped, so a
// three-row screen in an odd number of lines still fills the space exactly.
func Heights(rows []Row, total int) []int {
	if len(rows) == 0 {
		return nil
	}

	weights := make([]int, len(rows))
	sum := 0
	for i, r := range rows {
		weights[i] = max(r.Weight, 1)
		sum += weights[i]
	}

	heights := make([]int, len(rows))
	assigned := 0
	for i, w := range weights {
		heights[i] = max(total*w/sum, 1)
		assigned += heights[i]
	}

	for i := 0; assigned < total; i = (i + 1) % len(heights) {
		heights[i]++
		assigned++
	}
	return heights
}

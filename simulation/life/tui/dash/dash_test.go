package dash

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/simulation/life/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSource is a dashboard's whole world: some readings and their descriptions.
type fakeSource struct {
	values   map[string]float64
	displays map[string]telemetry.Display
}

func (f fakeSource) Value(metric string) float64 { return f.values[metric] }
func (f fakeSource) Series(string) []float64     { return nil }
func (f fakeSource) Scalar(_, _ string) float64  { return 0 }
func (f fakeSource) Elapsed() time.Duration      { return time.Minute }
func (f fakeSource) Alive() bool                 { return true }
func (f fakeSource) Display(m string) (telemetry.Display, bool) {
	d, ok := f.displays[m]
	return d, ok
}

func testSource() fakeSource {
	return fakeSource{
		values: map[string]float64{"heart_rate": 72, "glycemia": 30},
		displays: map[string]telemetry.Display{
			"heart_rate": {Label: "Heart Rate", Unit: "bpm", Min: 0, Max: 200,
				HealthMin: 50, HealthMax: 100, Decimals: 0},
			"glycemia": {Label: "Glucose", Unit: "mg/dL", Min: 0, Max: 200,
				HealthMin: 70, HealthMax: 140, Decimals: 0},
		},
	}
}

// Test_HeightsFillExactly is the invariant that used to be a convention.
//
// Five views each worked out their own row heights, and they disagreed by a line
// or two, so switching tabs nudged everything down the screen. One renderer
// decides now, and what it must guarantee is that the rows add up to precisely
// the space it was given -- no more, no fewer.
func Test_HeightsFillExactly(t *testing.T) {
	rows := []Row{{Weight: 2}, {Weight: 3}, {Weight: 3}}

	for _, total := range []int{8, 9, 10, 17, 23, 40, 41} {
		heights := Heights(rows, total)

		sum := 0
		for _, h := range heights {
			assert.Positive(t, h, "every row gets at least a line")
			sum += h
		}
		assert.Equal(t, total, sum, "rows must fill %d lines exactly", total)
	}
}

func Test_HeightsRespectWeight(t *testing.T) {
	heights := Heights([]Row{{Weight: 1}, {Weight: 3}}, 40)
	require.Len(t, heights, 2)
	assert.Greater(t, heights[1], heights[0]*2, "a weight of three should be much taller than one")
}

// Test_RenderFillsTheWidth is the horizontal half of the same promise: a row of
// panels covers the width exactly, with no ragged column left on the right.
func Test_RenderFillsTheWidth(t *testing.T) {
	screen := Screen{
		Rows: []Row{
			{Widgets: []Widget{
				Stat{Metric: "heart_rate"},
				Stat{Metric: "glycemia"},
				Stat{Metric: "heart_rate"},
			}},
			{Widgets: []Widget{Bars{Title: "B", Metrics: []string{"heart_rate"}}}},
		},
	}

	for _, width := range []int{80, 100, 101, 137} {
		out := Render(screen, testSource(), width, 20)

		require.NotEmpty(t, out)
		for _, line := range strings.Split(out, "\n") {
			assert.LessOrEqual(t, lipgloss.Width(line), width,
				"no line may exceed the width it was given")
		}
	}
}

func Test_RenderIsExactlyAsTallAsAsked(t *testing.T) {
	screen := Screen{Rows: []Row{
		{Weight: 1, Widgets: []Widget{Stat{Metric: "heart_rate"}}},
		{Weight: 2, Widgets: []Widget{Stats{Metrics: []string{"heart_rate", "glycemia"}}}},
	}}

	for _, height := range []int{6, 12, 25} {
		out := Render(screen, testSource(), 100, height)
		assert.Len(t, strings.Split(out, "\n"), height,
			"a screen must be exactly the height it was given")
	}
}

// Test_WidgetsWarnOnUnhealthyValues checks the one piece of judgement a widget
// makes: the catalog says what a healthy range is, and a reading outside it is
// drawn differently. Nobody writes a threshold in a screen definition.
func Test_WidgetsWarnOnUnhealthyValues(t *testing.T) {
	src := testSource()

	healthy := Stat{Metric: "heart_rate"}.Render(src, 40, 4)
	unhealthy := Stat{Metric: "glycemia"}.Render(src, 40, 4)

	assert.NotEqual(t, lipgloss.Width(healthy), 0)
	assert.NotEqual(t, healthy, unhealthy,
		"a glucose of 30 must not be drawn the same as a heart rate of 72")
}

func Test_ScreenWithNoRowsDrawsNothing(t *testing.T) {
	assert.Empty(t, Render(Screen{}, testSource(), 80, 10))
}

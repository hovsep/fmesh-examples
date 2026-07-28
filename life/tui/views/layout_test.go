package views

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/widgets"
)

// The dashboard's bars are only readable if they agree with each other: every
// row of every panel has to start its bar in the same column and end it in the
// same one. These tests render the real views and check the geometry, because
// the failure mode is invisible to a compiler and easy to reintroduce.

const testWidth = 120

// ansi matches the escape sequences lipgloss wraps around styled text.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// barRunes is the alphabet every bar and sparkline is drawn from, and that
// nothing else on a row uses.
const barRunes = "█░▁▂▃▄▅▆▇"

// bar is one run of bar characters: where it starts on the line and how wide it
// is, both in display cells.
type bar struct{ start, width int }

// barsOn finds every bar on a line. A line can carry more than one, since views
// put panels side by side.
//
// Columns are measured with lipgloss.Width over the text preceding a run, not
// by adding up rune widths: an emoji written as a code point plus its
// presentation selector is two runes and one two-cell glyph, and counting the
// runes separately would report the rest of that line a cell out of place.
func barsOn(line string) []bar {
	plain := ansi.ReplaceAllString(line, "")

	var bars []bar
	start := -1
	for i, r := range plain {
		isBar := strings.ContainsRune(barRunes, r)
		switch {
		case isBar && start < 0:
			start = i
		case !isBar && start >= 0:
			bars = append(bars, bar{lipgloss.Width(plain[:start]), lipgloss.Width(plain[start:i])})
			start = -1
		}
	}
	if start >= 0 {
		bars = append(bars, bar{lipgloss.Width(plain[:start]), lipgloss.Width(plain[start:])})
	}
	return bars
}

// stateWithReadings returns an app state carrying a plausible value for every
// metric the dashboard knows about, so each panel has something to draw.
func stateWithReadings() *models.AppState {
	state := models.NewAppState(512)
	for _, s := range telemetry.Signals(telemetry.DefaultSubject) {
		metadata := models.SignalRegistry[s.Key]
		// A full history, varying, so every sparkline fills its column.
		for i := range 200 {
			span := metadata.MaxRange - metadata.MinRange
			state.UpdateSignal(s.Key, metadata.MinRange+span*(0.3+0.2*float64(i%3)))
		}
	}

	// Feelings and organ damage are published as their own keys.
	for _, feeling := range []string{"thirsty", "hungry", "tired"} {
		state.UpdateSignal(telemetry.DefaultSubject+telemetry.PathSeparator+"feelings"+
			telemetry.ScalarSeparator+feeling, 0.5)
	}
	for _, organ := range telemetry.DamagedOrgans {
		state.UpdateSignal(telemetry.DefaultSubject+telemetry.PathSeparator+organ.Port+"_damage", 0.4)
	}
	return state
}

// assertBarsAgree checks that every bar in a rendered view is the same width,
// and that bars in the same column of a side-by-side layout start together.
func assertBarsAgree(t *testing.T, name, rendered string) {
	t.Helper()

	widths := map[int][]string{}
	starts := map[int]bool{}

	for _, line := range strings.Split(rendered, "\n") {
		plain := ansi.ReplaceAllString(line, "")
		for _, b := range barsOn(line) {
			widths[b.width] = append(widths[b.width], plain)
			starts[b.start] = true
		}
	}

	if len(widths) == 0 {
		t.Fatalf("%s: rendered no bars at all", name)
	}

	if len(widths) > 1 {
		t.Errorf("%s: bars are not the same width -- found %d widths:", name, len(widths))
		for w, lines := range widths {
			t.Errorf("  width=%d  e.g. %q", w, lines[0])
		}
	}

	// One start column per panel column. More than the panels present means a
	// row is pushing its bar out of line with the rows above it.
	if len(starts) > panelColumns(rendered) {
		t.Errorf("%s: bars start in %d different columns (%v), want one per panel",
			name, len(starts), slices.Sorted(maps.Keys(starts)))
	}
}

// panelColumns reports how many panels sit side by side, counted from the
// widest row's border characters.
func panelColumns(rendered string) int {
	most := 1
	for _, line := range strings.Split(rendered, "\n") {
		most = max(most, strings.Count(ansi.ReplaceAllString(line, ""), "│")/2)
	}
	return most
}

func TestMetricViewBarsAgree(t *testing.T) {
	state := stateWithReadings()

	// The metabolic tab is the one with the most rows, and the one where a
	// value's digit count used to shove its neighbours' bars out of line.
	for _, view := range []telemetry.View{
		telemetry.ViewMetabolic, telemetry.ViewNervous, telemetry.ViewRespiratory,
	} {
		v := NewMetricsView(state, "TEST", view, telemetry.DefaultSubject)
		assertBarsAgree(t, string(rune('0'+view))+" metrics view", v.Render(testWidth, 40))
	}
}

func TestFeelingsViewBarsAgree(t *testing.T) {
	state := stateWithReadings()

	// Both panels at once: the felt sensations on the left and the readings
	// behind them on the right ("WHY") are the same width, so their bars have to
	// line up with each other, not just within a panel.
	v := NewFeelingsView(state, telemetry.DefaultSubject)
	assertBarsAgree(t, "feelings view", v.Render(testWidth, 40))
}

func TestBodyViewBarsAgree(t *testing.T) {
	state := stateWithReadings()

	v := NewBodyView(state, telemetry.DefaultSubject)
	assertBarsAgree(t, "body view", v.Render(testWidth, 40))
}

func TestBarsAgreeAcrossViewsOfTheSameWidth(t *testing.T) {
	state := stateWithReadings()

	// A tab switch should not move the bars. Panels of equal width put them in
	// the same place whatever kind of bar they are.
	metabolic := NewMetricsView(state, "TEST", telemetry.ViewMetabolic, telemetry.DefaultSubject).Render(testWidth, 40)
	body := NewBodyView(state, telemetry.DefaultSubject).Render(testWidth, 40)

	if got, want := firstBar(t, body), firstBar(t, metabolic); got != want {
		t.Errorf("bars move between tabs: metabolic is %+v, body is %+v", want, got)
	}
}

func firstBar(t *testing.T, rendered string) bar {
	t.Helper()

	for _, line := range strings.Split(rendered, "\n") {
		if bars := barsOn(line); len(bars) > 0 {
			return bars[0]
		}
	}
	t.Fatal("no bars in the rendered view")
	return bar{}
}

func TestBarWidthLeavesRoomForItsColumns(t *testing.T) {
	// Whatever the panel width, a row fits in it: the columns, the bar and the
	// marker together never exceed what the panel gave.
	for _, width := range []int{60, 80, 120, 200} {
		barWidth := widgets.BarWidth(width)
		used := widgets.LabelWidth + widgets.ValueWidth + widgets.UnitWidth +
			widgets.MarkerWidth + barWidth + 4

		if used > width {
			t.Errorf("at width %d a row needs %d cells", width, used)
		}
		if barWidth < widgets.MinBarWidth {
			t.Errorf("at width %d the bar shrank to %d", width, barWidth)
		}
	}
}

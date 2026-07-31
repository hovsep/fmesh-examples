package widgets

import (
	"math"

	"github.com/guptarohit/asciigraph"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

// ECG draws a clean, scrolling electrocardiogram trace.
//
// It does not plot the heart's raw activation samples: that signal is a one-tick
// R-peak spike, so sampling and downsampling it aliases -- peaks land at random
// heights and jump as the window scrolls. Instead, like a patient monitor, it
// synthesises an idealised P-QRS-T waveform at the real heart rate and scrolls it
// by elapsed simulated time. The rate (and whether there is a beat at all) is
// real; only the trace shape is drawn for legibility.
type ECG struct {
	Title string

	// BeatsPerMinute drives the spacing of the complexes. Zero or negative draws
	// a flatline (no heartbeat).
	BeatsPerMinute float64

	// ElapsedSeconds is simulated time; advancing it scrolls the trace left.
	ElapsedSeconds float64

	// WindowSeconds is how much time the trace spans left-to-right.
	WindowSeconds float64
}

// ecgLower/ecgUpper fix the Y axis so the baseline and R-peaks never move.
const (
	ecgLower = -0.45
	ecgUpper = 1.15
)

// samplesPerColumn is how finely each drawn column is searched for its extreme.
//
// Sixteen puts a sample within about a fifth of the R peak's width whatever the
// phase, so the height it reports varies by a couple of percent -- comfortably
// less than one character row, which is the resolution anybody is looking at.
const samplesPerColumn = 16

func NewECG(title string, bpm, elapsedSeconds float64) *ECG {
	return &ECG{Title: title, BeatsPerMinute: bpm, ElapsedSeconds: elapsedSeconds, WindowSeconds: 5}
}

// Render draws the trace within the given width and height (title included).
func (e *ECG) Render(width, height int) string {
	title := styles.PanelTitleStyle.Render(e.Title)

	plotWidth := max(width-12, 10) // leave room for the y-axis labels
	plotHeight := max(height-2, 3)

	plot := asciigraph.Plot(e.columns(plotWidth),
		asciigraph.Width(plotWidth),
		asciigraph.Height(plotHeight),
		asciigraph.LowerBound(ecgLower),
		asciigraph.UpperBound(ecgUpper),
		asciigraph.Precision(1),
		asciigraph.SeriesColors(asciigraph.Red),
	)
	return title + "\n" + plot
}

// columns builds one value per drawn column -- but not one *sample* per column.
//
// Sampling once per column looks like it cannot alias, since it produces exactly
// the columns asciigraph draws. It aliases badly. The R peak is a spike about
// 0.009 of a cycle wide while a column spans nearer 0.06 of one, so the peak is
// several times narrower than the gap between samples: whether a sample lands on
// it is luck, and as the trace scrolls that luck changes every frame. The peaks
// flickered, each redraw catching the spike at a different height on its way
// past.
//
// So each column is sampled across the slice of time it covers and keeps the
// largest excursion it found. That is how any waveform display downsamples: draw
// the extreme within each column, not whatever happened to sit at its left edge.
func (e *ECG) columns(plotWidth int) []float64 {
	data := make([]float64, plotWidth)
	if e.BeatsPerMinute <= 0 || e.WindowSeconds <= 0 || plotWidth < 2 {
		return data
	}

	period := 60.0 / e.BeatsPerMinute
	columnSeconds := e.WindowSeconds / float64(plotWidth-1)

	for i := range data {
		// Column 0 is the oldest edge, the last column is "now".
		start := e.ElapsedSeconds - e.WindowSeconds + float64(i)*columnSeconds

		var peak float64
		for k := range samplesPerColumn {
			t := start + (float64(k)/samplesPerColumn)*columnSeconds
			phase := math.Mod(t/period, 1)
			if phase < 0 {
				phase++
			}
			if v := ecgAmplitude(phase); math.Abs(v) > math.Abs(peak) {
				peak = v
			}
		}
		data[i] = peak
	}
	return data
}

// ecgAmplitude returns the idealised ECG amplitude at a phase in [0,1) of one
// cardiac cycle: a small P wave, a sharp QRS complex, and a rounded T wave.
func ecgAmplitude(phase float64) float64 {
	p := 0.12 * gaussian(phase, 0.18, 0.028)  // P wave
	q := -0.12 * gaussian(phase, 0.37, 0.010) // Q dip
	r := 1.00 * gaussian(phase, 0.40, 0.009)  // R peak
	s := -0.28 * gaussian(phase, 0.44, 0.012) // S dip
	t := 0.30 * gaussian(phase, 0.64, 0.045)  // T wave
	return p + q + r + s + t
}

func gaussian(x, mu, sigma float64) float64 {
	d := (x - mu) / sigma
	return math.Exp(-0.5 * d * d)
}

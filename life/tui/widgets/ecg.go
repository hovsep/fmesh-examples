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

func NewECG(title string, bpm, elapsedSeconds float64) *ECG {
	return &ECG{Title: title, BeatsPerMinute: bpm, ElapsedSeconds: elapsedSeconds, WindowSeconds: 5}
}

// Render draws the trace within the given width and height (title included).
func (e *ECG) Render(width, height int) string {
	title := styles.PanelTitleStyle.Render(e.Title)

	plotWidth := max(width-12, 10) // leave room for the y-axis labels
	plotHeight := max(height-2, 3)

	// One amplitude sample per plot column: because we generate exactly the
	// columns asciigraph draws, there is no resampling and nothing aliases.
	data := make([]float64, plotWidth)
	if e.BeatsPerMinute > 0 && e.WindowSeconds > 0 {
		period := 60.0 / e.BeatsPerMinute
		for i := range data {
			// Column 0 is the oldest edge, the last column is "now".
			t := e.ElapsedSeconds - e.WindowSeconds + (float64(i)/float64(plotWidth-1))*e.WindowSeconds
			phase := math.Mod(t/period, 1)
			if phase < 0 {
				phase++
			}
			data[i] = ecgAmplitude(phase)
		}
	}

	plot := asciigraph.Plot(data,
		asciigraph.Width(plotWidth),
		asciigraph.Height(plotHeight),
		asciigraph.LowerBound(ecgLower),
		asciigraph.UpperBound(ecgUpper),
		asciigraph.Precision(1),
		asciigraph.SeriesColors(asciigraph.Red),
	)
	return title + "\n" + plot
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

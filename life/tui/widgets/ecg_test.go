package widgets

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// peakAcrossScroll walks the trace forward the way scrolling does and reports
// the lowest and highest R-peak it drew.
func peakAcrossScroll(t *testing.T, plotWidth int, samples func(*ECG) []float64) (low, high float64) {
	t.Helper()

	low, high = math.Inf(1), math.Inf(-1)
	// Two seconds at fifty frames a second, which is what the UI publishes at.
	for frame := range 100 {
		e := NewECG("ecg", 60, float64(frame)*0.02)

		peak := math.Inf(-1)
		for _, v := range samples(e) {
			peak = max(peak, v)
		}
		low, high = min(low, peak), max(high, peak)
	}
	require.False(t, math.IsInf(low, 0), "the trace drew nothing")
	return low, high
}

// Test_ECGPeaksDoNotFlickerWhileScrolling is the bug this widget was reported
// with: the R-peaks changed height as they travelled right to left.
//
// The cause was sampling, not the waveform. One sample per drawn column reads as
// though it cannot alias -- it produces exactly the columns the plot draws --
// but the R peak is several times narrower than a column is wide, so each frame
// caught the spike wherever the sample happened to fall on it. The breathing
// traces never showed it because their waveforms are wide and smooth.
func Test_ECGPeaksDoNotFlickerWhileScrolling(t *testing.T) {
	const plotWidth = 80

	low, high := peakAcrossScroll(t, plotWidth, func(e *ECG) []float64 {
		return e.columns(plotWidth)
	})

	// The plot is about ten rows over a range of 1.6, so a row is ~0.16. Staying
	// well inside one row is what "does not flicker" means to an eye.
	assert.Less(t, high-low, 0.05,
		"R-peak height should barely move between frames (low %.3f, high %.3f)", low, high)
	assert.InDelta(t, 1.0, high, 0.05, "and should still reach the full R-peak")
}

// Test_ECGSingleSampleWouldFlicker pins why the fix is needed rather than just
// that it works, so nobody reverts to the simpler-looking loop.
func Test_ECGSingleSampleWouldFlicker(t *testing.T) {
	const plotWidth = 80

	// Exactly the old approach: one sample at each column's left edge.
	low, high := peakAcrossScroll(t, plotWidth, func(e *ECG) []float64 {
		data := make([]float64, plotWidth)
		period := 60.0 / e.BeatsPerMinute
		for i := range data {
			t := e.ElapsedSeconds - e.WindowSeconds +
				(float64(i)/float64(plotWidth-1))*e.WindowSeconds
			phase := math.Mod(t/period, 1)
			if phase < 0 {
				phase++
			}
			data[i] = ecgAmplitude(phase)
		}
		return data
	})

	// Measured: the peak wanders between about 0.78 and 1.00, a swing of a fifth
	// of full height. The plot is ten rows over a range of 1.6, so that is well
	// over a character row -- which is precisely the flicker that was reported.
	assert.Greater(t, high-low, 0.15,
		"one sample per column swings the peak by more than a drawn row (low %.3f, high %.3f)",
		low, high)
}

func Test_ECGFlatlinesWithoutABeat(t *testing.T) {
	e := NewECG("ecg", 0, 12)
	for _, v := range e.columns(40) {
		assert.Zero(t, v, "a stopped heart draws a flat line")
	}
}

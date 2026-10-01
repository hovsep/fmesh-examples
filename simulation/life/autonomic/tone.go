package autonomic

import (
	"fmt"

	"github.com/hovsep/fmesh/signal"
)

// Pack builds a signal that represents autonomic tone with scalars
func Pack(sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) *signal.Signal {
	return signal.New("autonomic_tone").
		WithMeta(Sympathetic, sym).
		WithMeta(Parasympathetic, paraSym).
		WithMeta(Noise, noise).
		WithMeta(Gain, gain).
		WithMeta(Cardiac, cardiacBias).
		WithMeta(Vascular, vascularBias).
		WithMeta(Respiratory, respiratoryBias).
		WithMeta(GI, giBias)
}

// Unpack unpacks a signal that represents autonomic tone
func Unpack(tone *signal.Signal) (sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64, err error) {
	if tone == nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("autonomic tone signal is nil")
	}

	s := tone.Meta()
	sym = s.ValueOrDefault(Sympathetic, 0.0)
	paraSym = s.ValueOrDefault(Parasympathetic, 0.0)
	noise = s.ValueOrDefault(Noise, 0.0)
	gain = s.ValueOrDefault(Gain, 0.0)
	cardiacBias = s.ValueOrDefault(Cardiac, 0.0)
	vascularBias = s.ValueOrDefault(Vascular, 0.0)
	respiratoryBias = s.ValueOrDefault(Respiratory, 0.0)
	giBias = s.ValueOrDefault(GI, 0.0)
	return
}

// Bias retrieves a regional bias from an autonomic tone signal
func Bias(tone *signal.Signal, region string) (float64, error) {
	if tone == nil {
		return 0, fmt.Errorf("tone is nil")
	}
	return tone.Meta().ValueOrDefault(region, 0.0), nil
}

// The four numbers a tone carries. Sympathetic and parasympathetic are the two
// opposing arms; noise keeps the body from looking machined; gain scales how
// strongly the whole tone is felt.
const (
	Sympathetic     = "sympathetic"
	Parasympathetic = "parasympathetic"
	Noise           = "noise"
	Gain            = "gain"
)

// The regions a tone is biased toward. The autonomic system does not push every
// organ equally -- a fright speeds the heart far more than it slows the gut --
// so each region reads its own bias off the same signal.
const (
	Cardiac     = "cardiac"
	Vascular    = "vascular"
	Respiratory = "respiratory"
	GI          = "gi"
)

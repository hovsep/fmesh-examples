package autonomic

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// Pack builds a signal that represents autonomic tone with scalars
func Pack(sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) *signal.Signal {
	return signal.New("autonomic_tone").
		WithScalar(common.Sympathetic, sym).
		WithScalar(common.Parasympathetic, paraSym).
		WithScalar(common.Noise, noise).
		WithScalar(common.Gain, gain).
		WithScalar(common.Cardiac, cardiacBias).
		WithScalar(common.Vascular, vascularBias).
		WithScalar(common.Respiratory, respiratoryBias).
		WithScalar(common.GI, giBias)
}

// Unpack unpacks a signal that represents autonomic tone
func Unpack(tone *signal.Signal) (sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64, err error) {
	if tone == nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("autonomic tone signal is nil")
	}

	s := tone.Scalars()
	sym = s.ValueOrDefault(common.Sympathetic, 0)
	paraSym = s.ValueOrDefault(common.Parasympathetic, 0)
	noise = s.ValueOrDefault(common.Noise, 0)
	gain = s.ValueOrDefault(common.Gain, 0)
	cardiacBias = s.ValueOrDefault(common.Cardiac, 0)
	vascularBias = s.ValueOrDefault(common.Vascular, 0)
	respiratoryBias = s.ValueOrDefault(common.Respiratory, 0)
	giBias = s.ValueOrDefault(common.GI, 0)
	return
}

// Bias retrieves a regional bias from an autonomic tone signal
func Bias(tone *signal.Signal, region string) (float64, error) {
	if tone == nil {
		return 0, fmt.Errorf("tone is nil")
	}
	return tone.Scalars().ValueOrDefault(region, 0), nil
}

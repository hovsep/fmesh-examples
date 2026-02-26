package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/signal"
)

const (
	Sympathetic     common.Label = "sympathetic"
	Parasympathetic common.Label = "parasympathetic"
	Noise           common.Label = "noise"
	Gain            common.Label = "gain"
)

// PackAutonomicTone builds a signal that represents autonomic tone
func PackAutonomicTone(sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) *signal.Signal {
	return signal.New(signal.NewGroup().Add(
		NewLevel(sym, Sympathetic),
		NewLevel(paraSym, Parasympathetic),
		NewLevel(noise, Noise),
		NewLevel(gain, Gain),
		NewBias(cardiacBias, common.Cardiac),
		NewBias(vascularBias, common.Vascular),
		NewBias(respiratoryBias, common.Respiratory),
		NewBias(giBias, common.GI),
	))
}

// UnpackAutonomicTone unpacks a signal that represents autonomic tone
func UnpackAutonomicTone(tone *signal.Signal) (sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) {
	group := tone.PayloadOrNil().(*signal.Group)
	group.ForEach(func(sig *signal.Signal) error {
		if isLevel(sig) {
			switch sig.Labels().ValueOrDefault(common.Axis, "") {
			case Sympathetic:
				sym = helper.AsF64(sig)
				return nil
			case Parasympathetic:
				paraSym = helper.AsF64(sig)
				return nil
			case Noise:
				noise = helper.AsF64(sig)
				return nil
			case Gain:
				gain = helper.AsF64(sig)
				return nil
			default:
				panic("unsupported level")
			}
		}

		if isBias(sig) {
			switch sig.Labels().ValueOrDefault(common.Region, "") {
			case common.Cardiac:
				cardiacBias = helper.AsF64(sig)
				return nil
			case common.Vascular:
				vascularBias = helper.AsF64(sig)
				return nil
			case common.Respiratory:
				respiratoryBias = helper.AsF64(sig)
				return nil
			case common.GI:
				giBias = helper.AsF64(sig)
				return nil
			default:
				panic("unsupported bias")
			}
		}

		panic("unsupported signal type in autonomic tone")
	})

	return
}

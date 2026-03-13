package helper

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// IsBias checks if a signal represents a regional bias
func IsBias(s *signal.Signal) bool {
	return s.Labels().ValueIs(common.Type, common.Bias)
}

// NewBias builds a signal that represents a regional bias
func NewBias(value float64, region string) *signal.Signal {
	return signal.New(value).AddLabel(common.Type, common.Bias).AddLabel(common.Region, region)
}

// IsLevel checks if a signal represents a level
func IsLevel(s *signal.Signal) bool {
	return s.Labels().ValueIs(common.Type, common.Level)
}

// NewLevel builds a signal that represents a level
func NewLevel(value float64, axis string) *signal.Signal {
	return signal.New(value).AddLabel(common.Type, common.Level).AddLabel(common.Axis, axis)
}

// PackAutonomicTone builds a signal that represents autonomic tone
func PackAutonomicTone(sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) *signal.Signal {
	return signal.New(signal.NewGroup().Add(
		NewLevel(sym, common.Sympathetic),
		NewLevel(paraSym, common.Parasympathetic),
		NewLevel(noise, common.Noise),
		NewLevel(gain, common.Gain),
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
		if IsLevel(sig) {
			switch sig.Labels().ValueOrDefault(common.Axis, "") {
			case common.Sympathetic:
				sym = AsF64(sig)
				return nil
			case common.Parasympathetic:
				paraSym = AsF64(sig)
				return nil
			case common.Noise:
				noise = AsF64(sig)
				return nil
			case common.Gain:
				gain = AsF64(sig)
				return nil
			default:
				panic("unsupported level")
			}
		}

		if IsBias(sig) {
			switch sig.Labels().ValueOrDefault(common.Region, "") {
			case common.Cardiac:
				cardiacBias = AsF64(sig)
				return nil
			case common.Vascular:
				vascularBias = AsF64(sig)
				return nil
			case common.Respiratory:
				respiratoryBias = AsF64(sig)
				return nil
			case common.GI:
				giBias = AsF64(sig)
				return nil
			default:
				panic("unsupported bias")
			}
		}

		panic("unsupported signal type in autonomic tone")
	})

	return
}

func GetBias(tone *signal.Signal, region string) (float64, error) {
	if tone == nil {
		return 0, fmt.Errorf("tone is nil")
	}

	return tone.PayloadOrNil().(*signal.Group).Filter(func(sig *signal.Signal) bool {
		return IsBias(sig) && sig.Labels().ValueIs(common.Region, region)
	}).First().PayloadOrNil().(float64), nil

}

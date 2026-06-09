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
	return signal.New(value).WithLabel(common.Type, common.Bias).WithLabel(common.Region, region)
}

// PackAutonomicTone builds a signal that represents autonomic tone
func PackAutonomicTone(sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) *signal.Signal {
	return signal.New(signal.NewGroup().With(
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
func UnpackAutonomicTone(tone *signal.Signal) (sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64, err error) {
	group, err := AsType[*signal.Group](tone)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("unpack autonomic tone: %w", err)
	}

	err = group.ForEach(func(sig *signal.Signal) error {
		if IsLevel(sig) {
			switch sig.Labels().ValueOrDefault(common.Axis, "") {
			case common.Sympathetic:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				sym = v
				return nil
			case common.Parasympathetic:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				paraSym = v
				return nil
			case common.Noise:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				noise = v
				return nil
			case common.Gain:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				gain = v
				return nil
			default:
				return fmt.Errorf("unsupported level: %s", sig.Labels().ValueOrDefault(common.Axis, ""))
			}
		}

		if IsBias(sig) {
			switch sig.Labels().ValueOrDefault(common.Region, "") {
			case common.Cardiac:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				cardiacBias = v
				return nil
			case common.Vascular:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				vascularBias = v
				return nil
			case common.Respiratory:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				respiratoryBias = v
				return nil
			case common.GI:
				v, err := AsF64(sig)
				if err != nil {
					return err
				}
				giBias = v
				return nil
			default:
				return fmt.Errorf("unsupported bias: %s", sig.Labels().ValueOrDefault(common.Region, ""))
			}
		}

		return fmt.Errorf("unsupported signal type in autonomic tone")
	})

	return
}

func GetBias(tone *signal.Signal, region string) (float64, error) {
	if tone == nil {
		return 0, fmt.Errorf("tone is nil")
	}

	group, err := AsType[*signal.Group](tone)
	if err != nil {
		return 0, fmt.Errorf("get bias: %w", err)
	}

	return AsF64(
		group.Filter(
			func(sig *signal.Signal) bool {
				return IsBias(sig) && sig.Labels().ValueIs(common.Region, region)
			}).First())

}

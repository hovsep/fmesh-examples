package helper

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

// PackAir packs air composition into a signal
func PackAir(nitrogen, oxygen, argon, pollution, temperature, humidity float64) (*signal.Signal, error) {
	if nitrogen+oxygen+argon+pollution != 100.00 {
		return nil, fmt.Errorf("check air composition: total amount of gases is not equal to 100%%")
	}

	dist, err := NewDistribution(DistributionMap{
		"nitrogen":  nitrogen * unit.Percent,
		"oxygen":    oxygen * unit.Percent,
		"argon":     argon * unit.Percent,
		"pollution": pollution * unit.Percent,
	})
	if err != nil {
		return nil, fmt.Errorf("pack air: %w", err)
	}

	return signal.New(
		signal.NewGroup().With(
			NewLevel(temperature*unit.Celsius, "temperature"),
			NewLevel(humidity*unit.Percent, "humidity"),
			dist.WithLabel(common.Param, "composition"),
		)).
		WithLabel("category", "gas").
		WithLabel("type", "air"), nil
}

// UnpackAir extracts all components of an air signal produced by PackAir.
func UnpackAir(airSignal *signal.Signal) (nitrogen, oxygen, argon, pollution, temperature, humidity float64, err error) {
	if !IsAir(airSignal) {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("signal is not air")
	}

	group, err := AsGroup(airSignal)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("unpack air: %w", err)
	}

	err = group.ForEach(func(s *signal.Signal) error {

		if IsLevelWithAxis(s, "temperature") {
			v, err := AsF64(s)
			if err != nil {
				return err
			}
			temperature = v
			return nil
		}

		if IsLevelWithAxis(s, "humidity") {
			v, err := AsF64(s)
			if err != nil {
				return err
			}
			humidity = v
			return nil
		}

		if !IsLevel(s) && s.Labels().ValueIs(common.Param, "composition") {
			compGroup, err := AsGroup(s)
			if err != nil {
				return err
			}
			err = compGroup.ForEach(func(levelSig *signal.Signal) error {
				if IsLevelWithAxis(levelSig, "nitrogen") {
					v, err := AsF64(levelSig)
					if err != nil {
						return err
					}
					nitrogen = v
					return nil
				}
				if IsLevelWithAxis(levelSig, "oxygen") {
					v, err := AsF64(levelSig)
					if err != nil {
						return err
					}
					oxygen = v
					return nil
				}
				if IsLevelWithAxis(levelSig, "argon") {
					v, err := AsF64(levelSig)
					if err != nil {
						return err
					}
					argon = v
					return nil
				}
				if IsLevelWithAxis(levelSig, "pollution") {
					v, err := AsF64(levelSig)
					if err != nil {
						return err
					}
					pollution = v
					return nil
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return
}

// MapAirLevel allows modifying a given air param level (temperature or humidity)
func MapAirLevel(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) (*signal.Signal, error) {
	if !IsAir(airSignal) {
		return nil, fmt.Errorf("signal is not air")
	}

	group, err := AsGroup(airSignal)
	if err != nil {
		return nil, fmt.Errorf("map air level: %w", err)
	}

	newGroup := group.MapIf(
		func(s *signal.Signal) bool { return IsLevelWithAxis(s, axis) },
		func(s *signal.Signal) *signal.Signal {
			return s.MapPayload(func(payload any) any {
				return mapFunc(payload.(float64))
			})
		},
	)

	return airSignal.MapPayload(func(_ any) any { return newGroup }), nil
}

// MapAirComposition allows modifying a given air component (nitrogen, oxygen, argon, pollution).
// The composition is automatically rebalanced after the modification so all levels sum to 100%.
func MapAirComposition(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) (*signal.Signal, error) {
	if !IsAir(airSignal) {
		return nil, fmt.Errorf("signal is not air")
	}

	group, err := AsGroup(airSignal)
	if err != nil {
		return nil, fmt.Errorf("map air composition: %w", err)
	}

	var compositionErr error
	newAirGroup := group.MapIf(
		func(s *signal.Signal) bool { return s.Labels().ValueIs(common.Param, "composition") },
		func(compositionSig *signal.Signal) *signal.Signal {
			compGroup, err := AsGroup(compositionSig)
			if err != nil {
				compositionErr = err
				return compositionSig
			}
			newCompositionGroup := compGroup.MapIf(
				func(s *signal.Signal) bool { return IsLevelWithAxis(s, axis) },
				func(s *signal.Signal) *signal.Signal {
					return s.MapPayload(func(payload any) any {
						return mapFunc(payload.(float64))
					})
				},
			)
			modified := compositionSig.MapPayload(func(_ any) any { return newCompositionGroup })
			rebalanced, err := RebalanceDistribution(modified)
			if err != nil {
				compositionErr = err
				return compositionSig
			}
			return rebalanced
		},
	)
	if compositionErr != nil {
		return nil, fmt.Errorf("map air composition: %w", compositionErr)
	}

	return airSignal.MapPayload(func(_ any) any { return newAirGroup }), nil
}

func IsAir(signal *signal.Signal) bool {
	return signal.Labels().ValueIs("category", "gas") && signal.Labels().ValueIs("type", "air")
}

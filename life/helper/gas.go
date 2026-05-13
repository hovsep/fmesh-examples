package helper

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

// PackAir packs air composition into a signal
func PackAir(nitrogen, oxygen, argon, pollution, temperature, humidity float64) *signal.Signal {
	if nitrogen+oxygen+argon+pollution != 100.00 {
		panic("Check air composition: total amount of gases is not equal to 100%")
	}

	return signal.New(
		signal.NewGroup().Add(
			NewLevel(temperature*unit.Celsius, "temperature"),
			NewLevel(humidity*unit.Percent, "humidity"),
			NewDistribution(DistributionMap{
				"nitrogen":  nitrogen * unit.Percent,
				"oxygen":    oxygen * unit.Percent,
				"argon":     argon * unit.Percent,
				"pollution": pollution * unit.Percent,
			}).AddLabel(common.Param, "composition"),
		)).
		AddLabel("category", "gas").
		AddLabel("type", "air")
}

// UnpackAir extracts all components of an air signal produced by PackAir.
func UnpackAir(airSignal *signal.Signal) (nitrogen, oxygen, argon, pollution, temperature, humidity float64) {
	if !IsAir(airSignal) {
		panic("Signal is not air")
	}

	AsGroup(airSignal).ForEach(func(s *signal.Signal) error {

		if IsLevelWithAxis(s, "temperature") {
			temperature = AsF64(s)
			return nil
		}

		if IsLevelWithAxis(s, "humidity") {
			humidity = AsF64(s)
			return nil
		}

		if !IsLevel(s) && s.Labels().ValueIs(common.Param, "composition") {
			AsGroup(s).ForEach(func(levelSig *signal.Signal) error {
				if IsLevelWithAxis(levelSig, "nitrogen") {
					nitrogen = AsF64(levelSig)
					return nil
				}
				if IsLevelWithAxis(levelSig, "oxygen") {
					oxygen = AsF64(levelSig)
					return nil
				}
				if IsLevelWithAxis(levelSig, "argon") {
					argon = AsF64(levelSig)
					return nil
				}
				if IsLevelWithAxis(levelSig, "pollution") {
					pollution = AsF64(levelSig)
					return nil
				}
				return nil
			})
		}
		return nil
	})
	return
}

// MapAirLevel allows modifying a given air param level (temperature or humidity)
func MapAirLevel(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) *signal.Signal {
	if !IsAir(airSignal) {
		panic("Signal is not air")
	}

	newGroup := AsGroup(airSignal).MapIf(
		func(s *signal.Signal) bool { return IsLevelWithAxis(s, axis) },
		func(s *signal.Signal) *signal.Signal {
			return s.MapPayload(func(payload any) any {
				return mapFunc(payload.(float64))
			})
		},
	)

	return airSignal.MapPayload(func(_ any) any { return newGroup })
}

// MapAirComposition allows modifying a given air component (nitrogen, oxygen, argon, pollution)
func MapAirComposition(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) *signal.Signal {
	if !IsAir(airSignal) {
		panic("Signal is not air")
	}

	newAirGroup := AsGroup(airSignal).MapIf(
		func(s *signal.Signal) bool { return s.Labels().ValueIs(common.Param, "composition") },
		func(compositionSig *signal.Signal) *signal.Signal {
			newCompositionGroup := AsGroup(compositionSig).MapIf(
				func(s *signal.Signal) bool { return IsLevelWithAxis(s, axis) },
				func(s *signal.Signal) *signal.Signal {
					return s.MapPayload(func(payload any) any {
						return mapFunc(payload.(float64))
					})
				},
			)
			return compositionSig.MapPayload(func(_ any) any { return newCompositionGroup })
		},
	)

	return airSignal.MapPayload(func(_ any) any { return newAirGroup })
}

func IsAir(signal *signal.Signal) bool {
	return signal.Labels().ValueIs("category", "gas") && signal.Labels().ValueIs("type", "air")
}

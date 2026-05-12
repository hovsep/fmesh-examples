package helper

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

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

// MapAirLevel allows modifying a given air param level (temperature or humidity)
func MapAirLevel(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) *signal.Signal {
	if !IsAir(airSignal) {
		panic("Signal is not air")
	}

	AsGroup(airSignal).
		Filter(func(s *signal.Signal) bool {
			return IsLevelWithAxis(s, axis)
		}).
		First().
		MapPayload(func(payload any) any {
			return mapFunc(payload.(float64))
		})

	return airSignal
}

// MapAirComposition allows modifying a given air component (nitrogen, oxygen, argon, pollution)
func MapAirComposition(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) *signal.Signal {
	if !IsAir(airSignal) {
		panic("Signal is not air")
	}

	compositionSig := AsGroup(airSignal).
		Filter(func(s *signal.Signal) bool {
			return s.Labels().ValueIs(common.Param, "composition")
		}).
		First()

	AsGroup(compositionSig).
		Filter(func(s *signal.Signal) bool {
			return IsLevelWithAxis(s, axis)
		}).
		First().
		MapPayload(func(payload any) any {
			return mapFunc(payload.(float64))
		})

	return airSignal
}

func IsAir(signal *signal.Signal) bool {
	return signal.Labels().ValueIs("category", "gas") && signal.Labels().ValueIs("type", "air")
}

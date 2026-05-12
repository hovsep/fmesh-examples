package helper

import (
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
			NewLevel(nitrogen*unit.Percent, "nitrogen").AddLabel("formula", "N2"),
			NewLevel(oxygen*unit.Percent, "oxygen").AddLabel("formula", "O2"),
			NewLevel(argon*unit.Percent, "argon").AddLabel("formula", "Ar"),
			NewLevel(pollution*unit.Percent, "pollution").AddLabel("formula", "PM25"),
		)).
		AddLabel("category", "gas").
		AddLabel("type", "air")
}

// MapAirLevel allows modifying a given air level
func MapAirLevel(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) *signal.Signal {
	if !IsAir(airSignal) {
		panic("Signal is not air")
	}

	return airSignal.MapPayload(func(payload any) any {
		return payload.(*signal.Group).
			Map(func(levelSignal *signal.Signal) *signal.Signal {
				if !IsLevelWithAxis(levelSignal, axis) {
					// No change
					return levelSignal
				}

				return levelSignal.MapPayload(func(payload any) any {
					return mapFunc(payload.(float64))
				})
			})
	})
}

func IsAir(signal *signal.Signal) bool {
	return signal.Labels().ValueIs("category", "gas") && signal.Labels().ValueIs("type", "air")
}

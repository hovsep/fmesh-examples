package helper

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

// PackAir packs air composition into a single signal with scalars
func PackAir(nitrogen, oxygen, argon, pollution, temperature, humidity float64) (*signal.Signal, error) {
	if nitrogen+oxygen+argon+pollution != 100.00 {
		return nil, fmt.Errorf("check air composition: total amount of gases is not equal to 100%%")
	}

	return signal.New("air").
		WithLabel("category", "gas").
		WithLabel("type", "air").
		WithScalar("temperature", temperature*unit.Celsius).
		WithScalar("humidity", humidity*unit.Percent).
		WithScalar("nitrogen", nitrogen*unit.Percent).
		WithScalar("oxygen", oxygen*unit.Percent).
		WithScalar("argon", argon*unit.Percent).
		WithScalar("pollution", pollution*unit.Percent), nil
}

// UnpackAir extracts all components from an air signal
func UnpackAir(airSignal *signal.Signal) (nitrogen, oxygen, argon, pollution, temperature, humidity float64, err error) {
	if !IsAir(airSignal) {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("signal is not air")
	}

	s := airSignal.Scalars()
	return s.GetOrDefault("nitrogen", 0),
		s.GetOrDefault("oxygen", 0),
		s.GetOrDefault("argon", 0),
		s.GetOrDefault("pollution", 0),
		s.GetOrDefault("temperature", 0),
		s.GetOrDefault("humidity", 0),
		nil
}

// MapAirLevel modifies a scalar on the air signal
func MapAirLevel(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) (*signal.Signal, error) {
	if !IsAir(airSignal) {
		return nil, fmt.Errorf("signal is not air")
	}
	return airSignal.WithScalar(axis, mapFunc(airSignal.Scalars().GetOrDefault(axis, 0))), nil
}

// MapAirComposition modifies a composition scalar and rebalances so all composition values sum to 100%
func MapAirComposition(airSignal *signal.Signal, axis string, mapFunc func(old float64) float64) (*signal.Signal, error) {
	if !IsAir(airSignal) {
		return nil, fmt.Errorf("signal is not air")
	}

	compKeys := []string{"nitrogen", "oxygen", "argon", "pollution"}
	scalars := airSignal.Scalars()

	oldSum := 0.0
	for _, k := range compKeys {
		oldSum += scalars.GetOrDefault(k, 0)
	}
	if oldSum == 0 {
		return nil, fmt.Errorf("cannot rebalance air composition with all zero values")
	}

	oldVal := scalars.GetOrDefault(axis, 0)
	newVal := mapFunc(oldVal)
	result := airSignal.WithScalar(axis, newVal)

	newSum := oldSum - oldVal + newVal
	if newSum != 100 && oldSum-oldVal != 0 {
		factor := (100.0 - newVal) / (oldSum - oldVal)
		for _, k := range compKeys {
			if k != axis {
				result = result.WithScalar(k, result.Scalars().GetOrDefault(k, 0)*factor)
			}
		}
	}
	return result, nil
}

// IsAir checks if the signal represents air
func IsAir(s *signal.Signal) bool {
	if s == nil {
		return false
	}
	return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
}

package helper

import (
	"maps"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// DistributionMap is a map of levels to their distribution (all levels always sum up to 100%)
type DistributionMap map[string]float64

func AsBoolOrFalse(s *signal.Signal) bool {
	return AsTypeOrDefault[bool](s, false)
}

func AsF64(s *signal.Signal) float64 {
	return AsType[float64](s)
}

func AsF64OrDefault(s *signal.Signal, defaultValue float64) float64 {
	return AsTypeOrDefault[float64](s, defaultValue)
}

func AsInt(s *signal.Signal) int {
	return AsType[int](s)
}

func AsString(s *signal.Signal) string {
	return AsType[string](s)
}

func AsType[T any](s *signal.Signal) T {
	if s == nil {
		panic("signal is nil")
	}

	payload, err := s.Payload()
	if err != nil {
		panic(err)
	}

	return payload.(T)
}

func AsTypeOrDefault[T any](s *signal.Signal, defaultValue T) T {
	if s == nil {
		return defaultValue
	}

	return s.PayloadOrDefault(defaultValue).(T)
}

// AsGroup casts a signal to a group
func AsGroup(s *signal.Signal) *signal.Group {
	return AsType[*signal.Group](s)
}

// IsLevel checks if a signal represents a level
func IsLevel(s *signal.Signal) bool {
	return s.Labels().ValueIs(common.Type, common.Level)
}

// NewLevel builds a signal that represents a level
func NewLevel(value float64, axis string) *signal.Signal {
	return signal.New(value).AddLabel(common.Type, common.Level).AddLabel(common.Axis, axis)
}

// IsLevelWithAxis checks if a signal represents a level with a specific axis
func IsLevelWithAxis(s *signal.Signal, axis string) bool {
	return IsLevel(s) && s.Labels().ValueIs(common.Axis, axis)
}

func NewDistribution(distributionMap DistributionMap) *signal.Signal {
	sum := 0.0
	for v := range maps.Values(distributionMap) {
		sum += v
	}

	if sum != 100 {
		panic("distribution does not sum up to 100")
	}

	distGroup := signal.NewGroup()

	for axis, value := range distributionMap {
		distGroup.Add(NewLevel(value, axis))
	}

	return signal.New(distGroup).AddLabel(common.Type, "distribution")
}

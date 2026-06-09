package helper

import (
	"fmt"

	"github.com/hovsep/fmesh/signal"
)

func AsBoolOrFalse(s *signal.Signal) bool {
	return AsTypeOrDefault[bool](s, false)
}

func AsF64(s *signal.Signal) (float64, error) {
	return AsType[float64](s)
}

func AsF64OrDefault(s *signal.Signal, defaultValue float64) float64 {
	return AsTypeOrDefault[float64](s, defaultValue)
}

func AsInt(s *signal.Signal) (int, error) {
	return AsType[int](s)
}

func AsString(s *signal.Signal) (string, error) {
	return AsType[string](s)
}

func AsType[T any](s *signal.Signal) (T, error) {
	var zero T
	if s == nil {
		return zero, fmt.Errorf("signal is nil")
	}

	payload, err := s.Payload()
	if err != nil {
		return zero, fmt.Errorf("signal payload: %w", err)
	}

	return payload.(T), nil
}

func AsTypeOrDefault[T any](s *signal.Signal, defaultValue T) T {
	if s == nil {
		return defaultValue
	}

	return s.PayloadOrDefault(defaultValue).(T)
}

// AsGroup casts a signal to a group
func AsGroup(s *signal.Signal) (*signal.Group, error) {
	return AsType[*signal.Group](s)
}

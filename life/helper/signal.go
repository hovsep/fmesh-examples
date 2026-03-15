package helper

import (
	"github.com/hovsep/fmesh/signal"
)

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

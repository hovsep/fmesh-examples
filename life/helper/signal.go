package helper

import (
	"github.com/hovsep/fmesh/signal"
)

func AsF64(s *signal.Signal) float64 {
	return AsType[float64](s)
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

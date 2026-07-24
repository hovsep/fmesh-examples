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

	// A carried payload of the wrong type falls back to the default rather than
	// panicking, so a changed payload type downstream degrades instead of
	// crashing the whole mesh run.
	value, ok := s.PayloadOrDefault(defaultValue).(T)
	if !ok {
		return defaultValue
	}
	return value
}

// AsGroup casts a signal to a group
func AsGroup(s *signal.Signal) (*signal.Group, error) {
	return AsType[*signal.Group](s)
}

// NumericPayload reports the signal payload as a float64 when it carries a
// number (or a bool, encoded as 1/0). Composite signals use their payload as a
// type tag ("air", "venous_blood") and keep the real values in scalars, so
// telemetry uses this to tell a measurement apart from a tag.
func NumericPayload(s *signal.Signal) (float64, bool) {
	if s == nil {
		return 0, false
	}

	switch v := s.PayloadOrNil().(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

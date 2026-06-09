package helper

import (
	"fmt"
	"strings"

	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

// PackAir packs air composition into a single signal with scalars.
// Distribution members ("composition:...") are auto-rebalanced by MapAirScalar
// because the signal declares WithLabel("distribution:composition", "true").
func PackAir(nitrogen, oxygen, argon, pollution, temperature, humidity float64) (*signal.Signal, error) {
	if nitrogen+oxygen+argon+pollution != 100.00 {
		return nil, fmt.Errorf("check air composition: total amount of gases is not equal to 100%%")
	}

	return signal.New("air").
		WithLabel("category", "gas").
		WithLabel("type", "air").
		WithLabel("distribution:composition", "true").
		WithScalar("temperature", temperature*unit.Celsius).
		WithScalar("humidity", humidity*unit.Percent).
		WithScalar("composition:nitrogen", nitrogen*unit.Percent).
		WithScalar("composition:oxygen", oxygen*unit.Percent).
		WithScalar("composition:argon", argon*unit.Percent).
		WithScalar("composition:pollution", pollution*unit.Percent), nil
}

// UnpackAir extracts all components from an air signal
func UnpackAir(airSignal *signal.Signal) (nitrogen, oxygen, argon, pollution, temperature, humidity float64, err error) {
	if airSignal == nil || !airSignal.Labels().ValueIs("category", "gas") || !airSignal.Labels().ValueIs("type", "air") {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("signal is not air")
	}

	s := airSignal.Scalars()
	return s.GetOrDefault("composition:nitrogen", 0),
		s.GetOrDefault("composition:oxygen", 0),
		s.GetOrDefault("composition:argon", 0),
		s.GetOrDefault("composition:pollution", 0),
		s.GetOrDefault("temperature", 0),
		s.GetOrDefault("humidity", 0),
		nil
}

// MapAirScalar modifies a scalar on a signal. If the key belongs to a distribution
// declared via a "distribution:<group>" label on the signal (e.g.
// "distribution:composition"), all scalars with the matching "<group>:"
// prefix are rebalanced to sum to 100.
func MapAirScalar(s *signal.Signal, key string, fn func(old float64) float64) *signal.Signal {
	old := s.Scalars().GetOrDefault(key, 0)
	newVal := fn(old)
	result := s.WithScalar(key, newVal)

	prefix := distributionPrefix(s, key)
	if prefix == "" {
		return result
	}

	return rebalanceDistribution(result, key, newVal, prefix)
}

// rebalanceDistribution rescales all scalars with the given prefix (except the
// target key) so that the distribution group sums to 100.
func rebalanceDistribution(s *signal.Signal, key string, newVal float64, prefix string) *signal.Signal {
	distScalars := s.Scalars().Filter(func(k string, _ float64) bool {
		return strings.HasPrefix(k, prefix)
	})

	var sum float64
	distScalars.ForEach(func(_ string, v float64) error {
		sum += v
		return nil
	})

	if sum == 100 || sum == 0 {
		return s
	}

	othersSum := sum - newVal
	if othersSum == 0 {
		return s
	}

	factor := (100.0 - newVal) / othersSum
	distScalars.ForEach(func(k string, v float64) error {
		if k != key {
			s = s.WithScalar(k, v*factor)
		}
		return nil
	})
	return s
}

// distributionPrefix returns the "<group>:" prefix for a key if the signal
// declares that group as a distribution via a "distribution:<group>" label.
func distributionPrefix(s *signal.Signal, key string) string {
	idx := strings.Index(key, ":")
	if idx == -1 {
		return ""
	}
	group := key[:idx]
	if s.Labels().Has("distribution:" + group) {
		return group + ":"
	}
	return ""
}

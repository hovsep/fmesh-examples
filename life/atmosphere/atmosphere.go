package atmosphere

import (
	"fmt"
	"math"
	"strings"

	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

// SeaLevelPressure is one atmosphere in mmHg. Air with no pressure stamped on
// it is read as being at sea level, which is what every caller meant before
// pressure existed.
const SeaLevelPressure = 760.0

// ScalarPressure is the barometric pressure an air signal is at, in mmHg.
//
// It is deliberately outside the "composition:" distribution and so is left
// alone by MapScalar's rebalancing. Composition is what fraction of the air
// each gas is; pressure is how much air there is. Confusing the two is the
// single most common mistake about altitude: the air on a mountain is 21%
// oxygen, exactly as at sea level, and there is simply less of it.
const ScalarPressure = "pressure"

// WithPressure stamps a barometric pressure onto an air signal.
func WithPressure(air *signal.Signal, mmHg float64) *signal.Signal {
	return air.WithScalar(ScalarPressure, mmHg)
}

// Pressure reads the barometric pressure of an air signal, defaulting to sea
// level for air that has none.
func Pressure(air *signal.Signal) float64 {
	if air == nil {
		return SeaLevelPressure
	}
	return air.Scalars().ValueOrDefault(ScalarPressure, SeaLevelPressure)
}

// PressureAtAltitude returns the barometric pressure at a height above sea
// level, in mmHg, by the barometric formula.
//
// The scale height of 8000 m is the isothermal one, and it fits the range people
// actually go to within about a percent: 563 mmHg at 2400 m against a standard
// atmosphere's 567, 382 at 5500 m against 379, and 251 on the summit of Everest
// against the 253 the 1981 American Medical Research Expedition measured with a
// barometer they carried up there.
//
// That last figure is worth the digression. The standard atmosphere predicts 236
// mmHg at 8848 m, and the expedition found 253. The mountain sits under a
// permanent bulge in the equatorial stratosphere, and those seventeen millimetres
// are roughly the difference between a summit that can be reached without
// supplementary oxygen and one that cannot. Everest is climbable partly because
// of where it is on the planet, not only how high it is.
func PressureAtAltitude(metres float64) float64 {
	return SeaLevelPressure * math.Exp(-metres/8000.0)
}

// Pack packs air composition into a single signal with scalars.
// Distribution members ("composition:...") are auto-rebalanced by MapScalar
// because the signal declares WithLabel("distribution:composition", "true").
//
// The air it produces is at sea level; use WithPressure to put it somewhere
// else.
func Pack(nitrogen, oxygen, argon, pollution, temperature, humidity float64) (*signal.Signal, error) {
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

// Unpack extracts all components from an air signal
func Unpack(airSignal *signal.Signal) (nitrogen, oxygen, argon, pollution, temperature, humidity float64, err error) {
	if airSignal == nil || !airSignal.Labels().ValueIs("category", "gas") || !airSignal.Labels().ValueIs("type", "air") {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("signal is not air")
	}

	s := airSignal.Scalars()
	return s.ValueOrDefault("composition:nitrogen", 0),
		s.ValueOrDefault("composition:oxygen", 0),
		s.ValueOrDefault("composition:argon", 0),
		s.ValueOrDefault("composition:pollution", 0),
		s.ValueOrDefault("temperature", 0),
		s.ValueOrDefault("humidity", 0),
		nil
}

// MapScalar modifies a scalar on a signal. If the key belongs to a distribution
// declared via a "distribution:<group>" label on the signal (e.g.
// "distribution:composition"), all scalars with the matching "<group>:"
// prefix are rebalanced to sum to 100.
func MapScalar(s *signal.Signal, key string, fn func(old float64) float64) *signal.Signal {
	old := s.Scalars().ValueOrDefault(key, 0)
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

// ScalarCOppm is the carbon monoxide in the air, in parts per million.
//
// It is a scalar of its own rather than a member of the composition
// distribution, and that is how carbon monoxide is actually quoted: it is a
// trace gas, dangerous at concentrations far too small to matter to the
// composition. A thousand ppm is a tenth of one percent of the air and will
// take half a body's hemoglobin out of service inside two hours. Rounding it
// into the nitrogen would be arithmetically fine and physiologically absurd.
const ScalarCOppm = "carbon_monoxide_ppm"

// WithCarbonMonoxide stamps a carbon monoxide concentration onto an air signal.
func WithCarbonMonoxide(air *signal.Signal, ppm float64) *signal.Signal {
	return air.WithScalar(ScalarCOppm, ppm)
}

// CarbonMonoxide reads it. Clean air has none.
func CarbonMonoxide(air *signal.Signal) float64 {
	if air == nil {
		return 0
	}
	return air.Scalars().ValueOrDefault(ScalarCOppm, 0)
}

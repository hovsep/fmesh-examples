package helper

import (
	"testing"

	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackAir_ValidComposition(t *testing.T) {
	s, err := PackAir(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.True(t, IsAir(s))
}

func TestPackAir_InvalidComposition(t *testing.T) {
	_, err := PackAir(50, 20, 10, 5, 26.0, 58.8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not equal to 100")
}

func TestUnpackAir_RoundTrip(t *testing.T) {
	n, o, a, p, temp, hum := 78.0, 21.0, 1.0, 0.0, 26.0, 58.8
	s, err := PackAir(n, o, a, p, temp, hum)
	require.NoError(t, err)

	rn, ro, ra, rp, rtemp, rhum, err := UnpackAir(s)
	require.NoError(t, err)
	assert.Equal(t, n*unit.Percent, rn)
	assert.Equal(t, o*unit.Percent, ro)
	assert.Equal(t, a*unit.Percent, ra)
	assert.Equal(t, p*unit.Percent, rp)
	assert.Equal(t, temp*unit.Celsius, rtemp)
	assert.Equal(t, hum*unit.Percent, rhum)
}

func TestUnpackAir_Nil(t *testing.T) {
	_, _, _, _, _, _, err := UnpackAir(nil)
	require.Error(t, err)
}

func TestUnpackAir_NonAir(t *testing.T) {
	_, _, _, _, _, _, err := UnpackAir(signal.New("not_air"))
	require.Error(t, err)
}

func TestMapAirLevel_ModifiesScalar(t *testing.T) {
	s, err := PackAir(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s, err = MapAirLevel(s, "temperature", func(old float64) float64 { return old + 1.0 })
	require.NoError(t, err)

	v := s.Scalars().GetOrDefault("temperature", 0)
	assert.Equal(t, 26.0*unit.Celsius+1.0, v)
}

func TestMapAirLevel_RejectsNonAir(t *testing.T) {
	_, err := MapAirLevel(signal.New("not_air"), "temperature", func(old float64) float64 { return old })
	require.Error(t, err)
}

func TestMapAirComposition_Rebalances(t *testing.T) {
	s, err := PackAir(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s, err = MapAirComposition(s, "pollution", func(_ float64) float64 {
		return float64(2 * unit.Percent)
	})
	require.NoError(t, err)

	assert.Equal(t, float64(2*unit.Percent), s.Scalars().GetOrDefault("pollution", 0))

	compSum := s.Scalars().GetOrDefault("nitrogen", 0) +
		s.Scalars().GetOrDefault("oxygen", 0) +
		s.Scalars().GetOrDefault("argon", 0) +
		s.Scalars().GetOrDefault("pollution", 0)
	assert.InDelta(t, 100.0, compSum, 1e-9)

	assert.Equal(t, 26.0*unit.Celsius, s.Scalars().GetOrDefault("temperature", 0))
	assert.Equal(t, 58.8*unit.Percent, s.Scalars().GetOrDefault("humidity", 0))
}

func TestMapAirComposition_NoRebalanceNeeded(t *testing.T) {
	s, err := PackAir(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s, err = MapAirComposition(s, "nitrogen", func(old float64) float64 { return old })
	require.NoError(t, err)

	compSum := s.Scalars().GetOrDefault("nitrogen", 0) +
		s.Scalars().GetOrDefault("oxygen", 0) +
		s.Scalars().GetOrDefault("argon", 0) +
		s.Scalars().GetOrDefault("pollution", 0)
	assert.InDelta(t, 100.0, compSum, 1e-9)
}

func TestMapAirComposition_RejectsNonAir(t *testing.T) {
	_, err := MapAirComposition(signal.New("not_air"), "nitrogen", func(old float64) float64 { return old })
	require.Error(t, err)
}

func TestIsAir(t *testing.T) {
	s, err := PackAir(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)
	assert.True(t, IsAir(s))
	assert.False(t, IsAir(nil))
	assert.False(t, IsAir(signal.New("not_air")))
}

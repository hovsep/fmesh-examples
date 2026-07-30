package atmosphere


import (
	"testing"

	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackAir_ValidComposition(t *testing.T) {
	s, err := Pack(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)
	require.NotNil(t, s)
}

func TestPackAir_InvalidComposition(t *testing.T) {
	_, err := Pack(50, 20, 10, 5, 26.0, 58.8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not equal to 100")
}

func TestUnpackAir_RoundTrip(t *testing.T) {
	n, o, a, p, temp, hum := 78.0, 21.0, 1.0, 0.0, 26.0, 58.8
	s, err := Pack(n, o, a, p, temp, hum)
	require.NoError(t, err)

	rn, ro, ra, rp, rtemp, rhum, err := Unpack(s)
	require.NoError(t, err)
	assert.Equal(t, n, rn)
	assert.Equal(t, o, ro)
	assert.Equal(t, a, ra)
	assert.Equal(t, p, rp)
	assert.Equal(t, temp, rtemp)
	assert.Equal(t, hum, rhum)
}

func TestUnpackAir_Nil(t *testing.T) {
	_, _, _, _, _, _, err := Unpack(nil)
	require.Error(t, err)
}

func TestUnpackAir_NonAir(t *testing.T) {
	_, _, _, _, _, _, err := Unpack(signal.New("not_air"))
	require.Error(t, err)
}

func TestMapAirScalar_StandaloneScalar(t *testing.T) {
	s, err := Pack(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s = MapScalar(s, "temperature", func(old float64) float64 { return old + 1.0 })

	v := s.Scalars().ValueOrDefault("temperature", 0)
	assert.Equal(t, 26.0+1.0, v)
}

func TestMapAirScalar_DistributionRebalances(t *testing.T) {
	s, err := Pack(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s = MapScalar(s, "composition:pollution", func(_ float64) float64 {
		return float64(2)
	})

	// Target gets exactly its mapped value
	assert.Equal(t, float64(2), s.Scalars().ValueOrDefault("composition:pollution", 0))

	// Distribution rebalanced to sum 100
	compSum := s.Scalars().ValueOrDefault("composition:nitrogen", 0) +
		s.Scalars().ValueOrDefault("composition:oxygen", 0) +
		s.Scalars().ValueOrDefault("composition:argon", 0) +
		s.Scalars().ValueOrDefault("composition:pollution", 0)
	assert.InDelta(t, 100.0, compSum, 1e-9)

	// Standalone scalars unchanged
	assert.Equal(t, 26.0, s.Scalars().ValueOrDefault("temperature", 0))
	assert.Equal(t, 58.8, s.Scalars().ValueOrDefault("humidity", 0))
}

func TestMapAirScalar_DistributionNoRebalanceNeeded(t *testing.T) {
	s, err := Pack(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s = MapScalar(s, "composition:nitrogen", func(old float64) float64 { return old })

	compSum := s.Scalars().ValueOrDefault("composition:nitrogen", 0) +
		s.Scalars().ValueOrDefault("composition:oxygen", 0) +
		s.Scalars().ValueOrDefault("composition:argon", 0) +
		s.Scalars().ValueOrDefault("composition:pollution", 0)
	assert.InDelta(t, 100.0, compSum, 1e-9)
}

func TestMapAirScalar_StandaloneNoRebalance(t *testing.T) {
	s, err := Pack(78, 21, 1, 0, 26.0, 58.8)
	require.NoError(t, err)

	s = MapScalar(s, "humidity", func(old float64) float64 { return old * 2 })

	// Only humidity changed
	assert.Equal(t, 58.8*2, s.Scalars().ValueOrDefault("humidity", 0))

	// Nothing else touched
	assert.Equal(t, 26.0, s.Scalars().ValueOrDefault("temperature", 0))

	// Composition still sums to 100
	compSum := s.Scalars().ValueOrDefault("composition:nitrogen", 0) +
		s.Scalars().ValueOrDefault("composition:oxygen", 0) +
		s.Scalars().ValueOrDefault("composition:argon", 0) +
		s.Scalars().ValueOrDefault("composition:pollution", 0)
	assert.InDelta(t, 100.0, compSum, 1e-9)
}

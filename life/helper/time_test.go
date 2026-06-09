package helper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackTick_RoundTrip(t *testing.T) {
	seq := uint64(42)
	simDur := 500 * time.Millisecond
	wallTime := time.Date(2026, 6, 10, 12, 0, 0, 123456789, time.UTC)
	dt := 10 * time.Millisecond

	tick := PackTick(seq, simDur, wallTime, dt)
	require.NotNil(t, tick)

	rseq, rsimDur, rwallTime, rdt, err := UnpackTick(tick)
	require.NoError(t, err)

	assert.Equal(t, seq, rseq)
	assert.Equal(t, simDur, rsimDur)
	assert.True(t, wallTime.Equal(rwallTime))
	assert.Equal(t, dt, rdt)
}

func TestUnpackTick_Nil(t *testing.T) {
	_, _, _, _, err := UnpackTick(nil)
	require.Error(t, err)
}

func TestTickDurationInSec(t *testing.T) {
	tick := PackTick(1, 0, time.Now(), 10*time.Millisecond)
	sec, err := TickDurationInSec(tick)
	require.NoError(t, err)
	assert.InDelta(t, 0.01, sec, 1e-9)
}

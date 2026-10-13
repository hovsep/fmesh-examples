// Tests for a whole day: determinism and bookkeeping that adds up.
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADayIsDeterministic(t *testing.T) {
	first, err := runDay()
	require.NoError(t, err)
	second, err := runDay()
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestADayAddsUp(t *testing.T) {
	report, err := runDay()
	require.NoError(t, err)

	assert.Equal(t, openingMinutes, report.Minutes)
	assert.Equal(t, len(tableSeats), report.Tables)
	assert.Greater(t, report.PartiesServed, 0)
	assert.Greater(t, report.PartiesLost, 0, "the peaks should be busy enough to lose some parties")
	assert.Less(t, report.AverageWait(), float64(patienceMinutes), "nobody who was served waited past their patience")
	assert.Greater(t, report.Utilisation(), 0.0)
	assert.Less(t, report.Utilisation(), 1.0)

	lostShare := float64(report.PartiesLost) / float64(report.PartiesLost+report.PartiesServed)
	assert.InDelta(t, 0.125, lostShare, 0.075, "aim for 5-20 %% of parties lost under FIFO with 8 tables")
}

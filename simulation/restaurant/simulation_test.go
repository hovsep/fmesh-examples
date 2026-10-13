// Tests for a whole day: determinism, bookkeeping that adds up, and the
// policies compared on the default seed.
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADayIsDeterministic(t *testing.T) {
	first, err := runDay(defaultConfig())
	require.NoError(t, err)
	second, err := runDay(defaultConfig())
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestADayAddsUp(t *testing.T) {
	report, err := runDay(defaultConfig())
	require.NoError(t, err)

	assert.Equal(t, defaultOpeningMinutes, report.Minutes)
	assert.Equal(t, len(defaultTables), report.Tables)
	assert.Greater(t, report.PartiesServed, 0)
	assert.Greater(t, report.PartiesLost, 0, "the peaks should be busy enough to lose some parties")
	assert.Less(t, report.AverageWait(), float64(defaultPatienceMinutes), "nobody who was served waited past their patience")
	assert.Greater(t, report.Utilisation(), 0.0)
	assert.Less(t, report.Utilisation(), 1.0)

	lostShare := float64(report.PartiesLost) / float64(report.PartiesLost+report.PartiesServed)
	assert.InDelta(t, 0.125, lostShare, 0.075, "aim for 5-20 %% of parties lost under FIFO with 8 tables")
}

func TestBestFitLosesNoMoreGuestsThanFIFO(t *testing.T) {
	fifo, err := runDay(defaultConfig())
	require.NoError(t, err)

	cfg := defaultConfig()
	cfg.HostPolicy = policyBestFit
	bestFit, err := runDay(cfg)
	require.NoError(t, err)

	assert.LessOrEqual(t, bestFit.GuestsLost, fifo.GuestsLost)
	assert.GreaterOrEqual(t, bestFit.Revenue(), fifo.Revenue())
}

func TestRunDayRejectsAnUnknownPolicy(t *testing.T) {
	cfg := defaultConfig()
	cfg.HostPolicy = "coin-toss"
	_, err := runDay(cfg)
	assert.Error(t, err)
}

func TestClockTime(t *testing.T) {
	assert.Equal(t, "11:00", clockTime(0))
	assert.Equal(t, "12:05", clockTime(65))
	assert.Equal(t, "22:59", clockTime(defaultOpeningMinutes-1))
}

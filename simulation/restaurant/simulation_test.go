// Tests for a whole day: determinism and bookkeeping that adds up.
package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runDay does what main does without the printing: builds the restaurant,
// runs the mesh once per minute for the whole day and returns the report.
func runDay(t *testing.T) Report {
	t.Helper()
	fm, err := newRestaurant()
	require.NoError(t, err)
	for minute := range openingMinutes {
		err := fm.Components().ForEach(func(c *component.Component) error {
			if tick := c.InputByName("tick"); tick != nil {
				return tick.PutPayloads(minute)
			}
			return nil
		})
		require.NoError(t, err)
		_, err = fm.Run(context.Background())
		require.NoError(t, err, "minute %d", minute)
	}
	return readReport(fm)
}

func TestADayIsDeterministic(t *testing.T) {
	assert.Equal(t, runDay(t), runDay(t))
}

func TestADayAddsUp(t *testing.T) {
	report := runDay(t)

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

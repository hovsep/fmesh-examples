// Tests for the report component and the Report figures.
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportAddsUpTheDay(t *testing.T) {
	const tables, revenuePerGuest = 2, 10
	report, err := newReport(tables, revenuePerGuest)
	require.NoError(t, err)
	fm := alone(t, report)

	// Two minutes pass; in the second one two parties leave and one gives up.
	tick(t, fm, report, 0)
	activate(t, fm, report, map[string][]any{
		"tick": {1},
		"served": {
			Served{Party: Party{ID: 1, Size: 2, MealDuration: 50}, Seats: 2, WaitedMinutes: 4, LeftAt: 1},
			Served{Party: Party{ID: 2, Size: 4, MealDuration: 70}, Seats: 4, WaitedMinutes: 8, LeftAt: 1},
		},
		"lost": {Party{ID: 3, Size: 5}},
	})

	got := readReport(fm)
	want := Report{
		Tables: tables, RevenuePerGuest: revenuePerGuest, Minutes: 2,
		PartiesServed: 2, GuestsServed: 6, PartiesLost: 1, GuestsLost: 5,
		TotalWaitMinutes: 12, BusyTableMinutes: 120,
	}
	assert.Equal(t, want, got)
	assert.Equal(t, 6.0, got.AverageWait())
	assert.Equal(t, 30.0, got.Utilisation(), "120 busy table-minutes over 2 tables * 2 minutes (a tiny day, so far above 100 %)")
	assert.Equal(t, 60, got.Revenue())
	assert.Equal(t, 50, got.LostRevenue())
}

func TestReportStringShowsTheKeyNumbers(t *testing.T) {
	r := Report{Tables: 8, RevenuePerGuest: 25, Minutes: 720,
		PartiesServed: 40, GuestsServed: 100, PartiesLost: 5, GuestsLost: 12,
		TotalWaitMinutes: 200, BusyTableMinutes: 2880}

	text := r.String()
	assert.Contains(t, text, "Parties served :   40  (100 guests)")
	assert.Contains(t, text, "Parties lost   :    5  (12 guests)")
	assert.Contains(t, text, "Average wait   :    5.0 min")
	assert.Contains(t, text, "Utilisation    :   50.0 %")
	assert.Contains(t, text, "Revenue        : 2500 EUR")
	assert.Contains(t, text, "Lost revenue   :  300 EUR")
}

func TestEmptyReportHasNoAverages(t *testing.T) {
	assert.Equal(t, 0.0, Report{}.AverageWait())
	assert.Equal(t, 0.0, Report{}.Utilisation())
}

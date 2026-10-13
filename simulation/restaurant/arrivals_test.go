// Tests for the arrivals component.
package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrivalsProducePlausibleParties(t *testing.T) {
	arrivals, err := newArrivals(defaultSeed)
	require.NoError(t, err)
	fm := alone(t, arrivals)

	var parties []Party
	for minute := range defaultOpeningMinutes {
		tick(t, fm, arrivals, minute)
		for _, party := range outputs[Party](t, arrivals.OutputByName(portParty)) {
			assert.Equal(t, minute, party.ArrivedAt, "a party is stamped with its arrival minute")
			parties = append(parties, party)
		}
	}

	// Expected: about 0.14*120 + 0.12*180 + 0.03*420 = 51 parties. Allow a wide band.
	assert.GreaterOrEqual(t, len(parties), 30, "a day should bring a reasonable crowd")
	assert.LessOrEqual(t, len(parties), 80, "but not an absurd one")

	for i, party := range parties {
		assert.Equal(t, i+1, party.ID, "IDs run from 1 in arrival order")
		assert.GreaterOrEqual(t, party.Size, 1)
		assert.LessOrEqual(t, party.Size, 6)
		assert.GreaterOrEqual(t, party.MealDuration, minMealMinutes)
		assert.LessOrEqual(t, party.MealDuration, maxMealMinutes)
	}
}

func TestArrivalRateIsHigherAtLunchAndDinner(t *testing.T) {
	cases := []struct {
		clock string
		want  float64
	}{
		{"11:00", offPeakRate},
		{"12:00", lunchRate},
		{"13:59", lunchRate},
		{"14:00", offPeakRate},
		{"18:30", dinnerRate},
		{"21:00", offPeakRate},
	}
	for _, c := range cases {
		var hour, min int
		_, err := fmt.Sscanf(c.clock, "%d:%d", &hour, &min)
		require.NoError(t, err)
		minute := hour*60 + min - openingHour*60
		assert.Equal(t, c.want, arrivalRate(minute), "rate at %s", c.clock)
	}
}

func TestPartySizesCoverEveryWeight(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	seen := map[int]bool{}
	for range 1000 {
		seen[randomPartySize(random)] = true
	}
	for size := 1; size <= 6; size++ {
		assert.True(t, seen[size], "size %d should show up in a thousand draws", size)
	}
}

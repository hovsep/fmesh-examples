// This file holds the clock and the run loop: one mesh run per simulated minute,
// for a whole day.
package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh"
)

// openingHour is when the day starts: minute 0 of the simulation is 11:00.
const openingHour = 11

// clockMinute converts a simulation minute into minutes since midnight.
func clockMinute(minute int) int {
	return openingHour*60 + minute
}

// clockTime prints a simulation minute as wall-clock time, e.g. "12:05".
func clockTime(minute int) string {
	clock := clockMinute(minute)
	return fmt.Sprintf("%02d:%02d", clock/60, clock%60)
}

// runDay builds the restaurant, lets a whole day pass minute by minute and
// returns the bookkeeper's report. The same seed always gives the same Report.
func runDay(seats []int) (Report, error) {
	fm, err := newRestaurant(seats)
	if err != nil {
		return Report{}, err
	}
	for minute := range openingMinutes {
		if err := runMinute(fm, minute); err != nil {
			return Report{}, fmt.Errorf("minute %s: %w", clockTime(minute), err)
		}
	}
	return readReport(fm), nil
}

// runMinute is one tick of the clock: every component hears the minute, then
// the mesh runs until all signals have been handled.
func runMinute(fm *fmesh.FMesh, minute int) error {
	if err := seedTick(fm, minute); err != nil {
		return err
	}
	_, err := fm.Run(context.Background())
	return err
}

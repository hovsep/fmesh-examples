// This file holds the clock and the run loop: one mesh run per simulated minute,
// for a whole day.
package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
)

// runDay builds the restaurant, lets a whole day pass minute by minute and
// returns the bookkeeper's report. The same seed always gives the same Report.
func runDay() (Report, error) {
	fm, err := newRestaurant()
	if err != nil {
		return Report{}, err
	}
	for minute := range openingMinutes {
		if err := runMinute(fm, minute); err != nil {
			return Report{}, fmt.Errorf("minute %d: %w", minute, err)
		}
	}
	return readReport(fm), nil
}

// runMinute is one tick of the clock: every component with a "tick" input
// hears the minute, then the mesh runs until all signals have been handled.
func runMinute(fm *fmesh.FMesh, minute int) error {
	err := fm.Components().ForEach(func(c *component.Component) error {
		if tick := c.InputByName("tick"); tick != nil {
			return tick.PutPayloads(minute)
		}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = fm.Run(context.Background())
	return err
}

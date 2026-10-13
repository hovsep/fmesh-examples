// This file holds the "arrivals" component: the front door. Every minute it
// decides, with a seeded random generator, how many parties walk in, how big
// they are and how long they will stay.
package main

import (
	"context"
	"math/rand"

	"github.com/hovsep/fmesh/component"
)

// Opening hours on the wall clock (the day starts at openingHour, see simulation.go).
const (
	lunchStart  = 12 * 60 // 12:00, in minutes since midnight
	lunchEnd    = 14 * 60 // 14:00
	dinnerStart = 18 * 60 // 18:00
	dinnerEnd   = 21 * 60 // 21:00
)

// Expected number of arriving parties per minute, by time of day.
const (
	lunchRate   = 0.14
	dinnerRate  = 0.12
	offPeakRate = 0.03
)

// Each minute we draw arrivalDraws times; every draw brings one party with
// probability rate/arrivalDraws. So at most arrivalDraws parties arrive per minute
// and the average is the rate. Simple, and close enough to a Poisson arrival.
const arrivalDraws = 2

// Meal durations are uniform between these bounds (inclusive), in minutes.
const (
	minMealMinutes = 45
	maxMealMinutes = 90
)

// Party sizes and how common each one is (weights, in percent).
// Couples are the most common, then tables of four.
var partySizeWeights = map[int]int{1: 12, 2: 40, 3: 10, 4: 25, 5: 8, 6: 5}

// Keys of the arrivals' state.
const (
	stateRandom = "random"  // the seeded *rand.Rand
	stateNextID = "next-id" // the ID the next party will get
)

// newArrivals creates the front door. The seed makes every day reproducible.
func newArrivals(seed int64) (*component.Component, error) {
	return component.New("arrivals",
		component.WithDescription("the front door: parties of guests arrive, more at lunch and dinner"),
		component.WithInputs("tick"),
		component.WithOutputs("party"),
		component.WithInitialState(func(state component.State) {
			state.Set(stateRandom, rand.New(rand.NewSource(seed)))
			state.Set(stateNextID, 1)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			minute, err := firstPayload[int](this.InputByName("tick"))
			if err != nil {
				return err
			}
			random := this.State().Get(stateRandom).(*rand.Rand)
			nextID := this.State().Get(stateNextID).(int)

			for range arrivalDraws {
				if random.Float64() >= arrivalRate(minute)/arrivalDraws {
					continue
				}
				party := randomParty(random, nextID, minute)
				nextID++
				if err := this.OutputByName("party").PutPayloads(party); err != nil {
					return err
				}
			}

			this.State().Set(stateNextID, nextID)
			return nil
		}),
	)
}

// arrivalRate is the expected number of parties per minute at that minute of the day.
func arrivalRate(minute int) float64 {
	clock := clockMinute(minute)
	switch {
	case clock >= lunchStart && clock < lunchEnd:
		return lunchRate
	case clock >= dinnerStart && clock < dinnerEnd:
		return dinnerRate
	default:
		return offPeakRate
	}
}

// randomParty draws the size and the meal duration of a party arriving now.
func randomParty(random *rand.Rand, id int, minute int) Party {
	return Party{
		ID:           id,
		Size:         randomPartySize(random),
		ArrivedAt:    minute,
		MealDuration: minMealMinutes + random.Intn(maxMealMinutes-minMealMinutes+1),
	}
}

// randomPartySize picks a size according to partySizeWeights.
func randomPartySize(random *rand.Rand) int {
	totalWeight := 0
	for _, weight := range partySizeWeights {
		totalWeight += weight
	}
	// Walk the sizes in a fixed order so the same random number always gives the same size.
	draw := random.Intn(totalWeight)
	for size := 1; size <= 6; size++ {
		draw -= partySizeWeights[size]
		if draw < 0 {
			return size
		}
	}
	return 2 // unreachable, the weights cover every draw
}

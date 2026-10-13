// This file holds the "arrivals" component: the front door. Every minute it
// decides, with a seeded random generator, how many parties walk in, how big
// they are and how long they will stay.
//
// A party is one signal. Its payload is the number of guests and its labels
// (signal metadata) say who it is and when it came:
//
//	id            running number, assigned on arrival
//	arrived-at    minute of the day when the party arrived
//	meal-minutes  how many minutes the party stays once seated
//
// The same signal travels the whole restaurant: the waiting list shows it to
// the host, the host passes it to a table, the table stamps "seated-at" on it and
// hands it to the report when the party leaves.
package main

import (
	"context"
	"math/rand"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Opening hours on the wall clock, in minutes since midnight.
const (
	lunchStart  = 12 * 60 // 12:00
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

// newArrivals creates the front door. The seed makes every day reproducible.
func newArrivals(seed int64) (*component.Component, error) {
	return component.New("arrivals",
		component.WithDescription("the front door: parties of guests arrive, more at lunch and dinner"),
		component.WithInputs("tick"),
		component.WithOutputs("party"),
		component.WithInitialState(func(state component.State) {
			state.Set("random", rand.New(rand.NewSource(seed)))
			state.Set("next-id", 1)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			// FirstAs reads the payload of the first signal on the port as an int.
			minute, err := this.InputByName("tick").Signals().FirstAs[int]()
			if err != nil {
				return err
			}
			random, err := this.State().GetTyped[*rand.Rand]("random")
			if err != nil {
				return err
			}
			nextID, err := this.State().GetTyped[int]("next-id")
			if err != nil {
				return err
			}

			for range arrivalDraws {
				if !aPartyWalksIn(random, minute) {
					continue
				}
				if err := this.OutputByName("party").PutSignals(newParty(random, nextID, minute)); err != nil {
					return err
				}
				nextID++
			}
			this.State().Set("next-id", nextID)
			return nil
		}),
	)
}

// aPartyWalksIn is one draw: true with the probability that gives, over all
// the draws of the minute, the arrival rate for that time of day.
func aPartyWalksIn(random *rand.Rand, minute int) bool {
	return random.Float64() < arrivalRate(minute)/arrivalDraws
}

// newParty builds the signal for a party arriving now: a random size as the
// payload, and its id, arrival minute and a random meal duration as labels.
// A label is a named value that travels with the signal, and any component
// on the way can read it or add one.
func newParty(random *rand.Rand, id int, minute int) *signal.Signal {
	return signal.New(randomPartySize(random)).WithMetaMany(map[string]int{
		"id":           id,
		"arrived-at":   minute,
		"meal-minutes": minMealMinutes + random.Intn(maxMealMinutes-minMealMinutes+1),
	})
}

// arrivalRate is the expected number of parties per minute at that minute of the day.
func arrivalRate(minute int) float64 {
	clock := openingHour*60 + minute // minutes since midnight
	switch {
	case clock >= lunchStart && clock < lunchEnd:
		return lunchRate
	case clock >= dinnerStart && clock < dinnerEnd:
		return dinnerRate
	default:
		return offPeakRate
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

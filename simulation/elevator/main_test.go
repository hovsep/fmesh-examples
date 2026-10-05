package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sim runs the mesh tick by tick, pressing what the script says.
type sim struct {
	t     *testing.T
	fm    *fmesh.FMesh
	tick  int
	frame Frame
}

func newSim(t *testing.T) *sim {
	fm, err := getMesh()
	require.NoError(t, err)
	return &sim{t: t, fm: fm}
}

func (s *sim) press(inputs ...Input) {
	var err error
	s.tick++
	s.frame, _, err = step(s.fm, inputs)
	require.NoError(s.t, err)
}

// until runs ticks until done says so, failing after a simulated minute.
func (s *sim) until(what string, done func(Frame) bool) {
	for range int(60 / dt) {
		s.press()
		if done(s.frame) {
			return
		}
	}
	s.t.Fatalf("still waiting for %s after a minute", what)
}

func (s *sim) ctl(c string) ControllerState { return s.frame["controller-"+c].(ControllerState) }
func (s *sim) car(c string) CarState        { return s.frame["car-"+c].(CarState) }
func (s *sim) lamp(name string) bool        { l, _ := s.frame[name].(LampState); return l.On }

func press(part string) Input { return Input{Part: part, Port: portPress} }

func (s *sim) doorsOpenAt(c string, f int) func(Frame) bool {
	return func(fr Frame) bool {
		return fr["door-operator-"+c].(DoorState).Open && fr["position-sensor-"+c].(ReadingState).Floor == f
	}
}

func TestRideLevelsAndOpensTheDoors(t *testing.T) {
	s := newSim(t)
	s.press(press("car-A-button-4"))
	assert.True(t, s.lamp("car-A-led-4"), "the button lights as soon as it is pressed")

	s.until("car A to open at 4", s.doorsOpenAt("A", 4))
	assert.InDelta(t, floorPos(4), s.car("A").Pos, levelTolerance, "the car stops level with the landing")
	assert.False(t, s.lamp("car-A-led-4"), "the LED goes out on arrival")
	assert.Equal(t, 1.0, s.frame["landing-door-A-4"].(OpeningState).Opening, "the car door drags the landing door")
	assert.Equal(t, 0.0, s.frame["landing-door-A-3"].(OpeningState).Opening, "the others stay shut")
	assert.False(t, s.frame["safety-chain-A"].(ChainState).OK, "open doors open the safety chain")
	assert.True(t, s.frame["lantern-A-4"].(IndicatorState).Gong)
}

func TestHallCallsGoToTheNearestCar(t *testing.T) {
	s := newSim(t)
	s.press() // the dispatcher hears from both cars on the first tick
	s.press(press("hall-up-2"), press("hall-down-5"))
	assert.True(t, s.lamp("hall-up-led-2"))
	assert.True(t, s.lamp("hall-down-led-5"))

	// A waits in the lobby and B on the top floor: each takes the call nearest to it.
	s.until("both calls answered", func(fr Frame) bool {
		return len(fr["dispatcher"].(DispatcherState).Calls) == 0
	})
	assert.False(t, s.lamp("hall-up-led-2"))
	assert.False(t, s.lamp("hall-down-led-5"))
	assert.Equal(t, 2, s.frame["position-sensor-A"].(ReadingState).Floor)
	assert.Equal(t, 5, s.frame["position-sensor-B"].(ReadingState).Floor)
}

func TestALateCallDoesNotMakeTheCarSkipItsStop(t *testing.T) {
	s := newSim(t)
	s.press()
	s.press(press("hall-down-4"))
	s.until("car B to brake for 4", func(fr Frame) bool { return fr["car-B"].(CarState).Pos < floorPos(4)+1.5 })
	s.press(press("car-B-button-1")) // too late to stop anywhere nearer, and farther than 4
	s.until("car B to open", func(fr Frame) bool { return fr["door-operator-B"].(DoorState).Open })
	assert.Equal(t, 4, s.frame["position-sensor-B"].(ReadingState).Floor)
	assert.Equal(t, []int{1}, s.ctl("B").Stops)
}

func TestTheLightCurtainReopensTheDoors(t *testing.T) {
	s := newSim(t)
	s.press(press("car-A-open"))
	s.until("the doors to start closing", func(fr Frame) bool { return fr["controller-A"].(ControllerState).Mode == modeClosing })
	s.press(Input{Part: "light-curtain-A", Port: portBlock})
	s.press()
	assert.Equal(t, modeOpening, s.ctl("A").Mode, "someone in the doorway: the doors open again")
	s.until("the doors to close at last", func(fr Frame) bool { return fr["controller-A"].(ControllerState).Mode == modeParked })
}

func TestOverspeedTripsTheGovernor(t *testing.T) {
	s := newSim(t)
	s.press(press("car-A-button-6"))
	s.until("car A to run", func(fr Frame) bool { return fr["car-A"].(CarState).Speed > 1 })
	s.press(Input{Part: "drive-A", Port: portTest})

	s.until("the governor to trip", func(fr Frame) bool { return fr["governor-A"].(GovernorState).Tripped })
	s.until("the car to stop", func(fr Frame) bool { return fr["car-A"].(CarState).Speed == 0 })
	assert.True(t, s.frame["safety-gear-A"].(EngagedState).Engaged)
	assert.Equal(t, modeHalted, s.ctl("A").Mode)
	assert.Contains(t, s.frame["safety-chain-A"].(ChainState).Open, "governor")
	assert.Less(t, s.car("A").Pos, floorPos(6), "it stops between floors")

	s.press(Input{Part: "governor-A", Port: portReset})
	s.until("car A to carry on to 6", s.doorsOpenAt("A", 6))
}

func TestTheLightGoesOutInAnIdleCar(t *testing.T) {
	s := newSim(t)
	s.until("the light to go out", func(fr Frame) bool { l, ok := fr["car-A-light"].(LampState); return ok && !l.On })
	s.press(press("car-A-button-2"))
	s.press()
	assert.True(t, s.lamp("car-A-light"), "a passenger's call turns it back on")
}

// TestNeverMovesWithADoorOpen lets the demo's visitors loose on the building
// and checks the one rule an elevator must never break.
func TestNeverMovesWithADoorOpen(t *testing.T) {
	s := newSim(t)
	visitors := rand.New(rand.NewPCG(3, 4))
	levelled := map[string]bool{}
	for range int(300 / dt) {
		s.press(visit(visitors, s.frame)...)
		for _, c := range cabs {
			if s.car(c).Speed == 0 {
				continue
			}
			assert.True(t, s.frame["door-operator-"+c].(DoorState).Closed, "car %s moves with its door open", c)
			for f := 1; f <= floors; f++ {
				assert.True(t, s.frame[fmt.Sprintf("door-lock-%s-%d", c, f)].(ContactState).Closed, "car %s moves with landing door %d open", c, f)
			}
			if r := s.frame["position-sensor-"+c].(ReadingState); r.Level {
				levelled[fmt.Sprint(c, r.Floor)] = true
			}
		}
		require.False(t, t.Failed(), "at second %.2f", float64(s.tick)*dt)
	}
	assert.Greater(t, len(levelled), 6, "the visitors keep both cars busy")
	for _, c := range cabs {
		assert.LessOrEqual(t, math.Abs(s.car(c).Speed), ratedSpeed+1e-9)
	}
}

func TestOnlyButtonsCanBePressed(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)
	for _, in := range []Input{{"hall-up-6", portPress}, {"car-C-button-1", portPress}, {"motor-A", portPress}, {"controller-A", portTick}} {
		_, err := target(fm, in)
		assert.Error(t, err, in)
	}
	_, err = target(fm, Input{"hall-down-6", portPress})
	assert.NoError(t, err)
}

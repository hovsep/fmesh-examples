package main

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Controller port names.
const (
	portCarCall   = "car-call"  // car button → controller: a floor
	portDoorBtn   = "door-btn"  // open / close button → controller
	portAssign    = "assign"    // dispatcher → controller: a hall call to serve
	portLED       = "led"       // controller → car button LEDs, led1…led6
	portDisplay   = "display"   // controller → the display in the car
	portIndicator = "indicator" // controller → the lanterns on every landing
	portLight     = "light"     // controller → the light in the car
	portStatus    = "status"    // controller → dispatcher
)

// Controller modes.
const (
	modeParked   = "parked"   // doors shut, brake on
	modeOpening  = "opening"  // doors opening at a stop
	modeDwell    = "dwell"    // doors open, people getting in and out
	modeClosing  = "closing"  // doors closing
	modeStarting = "starting" // lifting the brake
	modeRunning  = "running"  // travelling to the next stop
	modeStopping = "stopping" // at the stop, letting the brake drop
	modeHalted   = "halted"   // the safety chain opened on the way
)

const (
	profileDecel   = 0.8   // m/s², a little gentler than the drive can do, so it can always follow
	creepSpeed     = 0.05  // m/s for the last few millimetres
	stopTolerance  = 0.004 // m from the landing to stop the drive
	levelTolerance = 0.015 // m from the landing still counts as level
	dwellTime      = 3.0   // s the doors stay open
	lightTimeout   = 10.0  // s an empty, idle car keeps its light on
)

// CabStatus is what a controller tells the dispatcher.
type CabStatus struct {
	Cab    string
	Pos    float64
	Dir    int
	Stops  []int
	Served int // the floor where the doors just started to open, or 0
}

// ControllerState is the controller's panel.
type ControllerState struct {
	Mode   string `json:"mode"`
	Dir    int    `json:"dir"`
	Target int    `json:"target"`
	Stops  []int  `json:"stops"`
}

// cabControl is everything a controller remembers between activations.
type cabControl struct {
	mode    string
	dir     int
	target  int
	dwell   float64
	idle    float64
	stops   map[int]bool // every floor to stop at
	lit     map[int]bool // car buttons with their LED on
	served  []int        // floors served since the last status
	open    bool         // open button pressed since the last tick
	close   bool         // close button pressed since the last tick
	light   bool
	shown   Indicator
	reading ReadingState
	door    DoorState
	brake   bool
	beam    bool
	chain   bool
}

// newController is the cab's controller (its internals are the one part of
// the elevator this example keeps simple). It listens all the time and
// decides once per tick: open, wait, close, lift the brake, run, stop.
func newController(c string, start int) (*component.Component, error) {
	return component.New("controller-"+c,
		component.WithDescription("cab controller: serves its stops by driving the brake, the drive and the doors"),
		component.WithInputs(portTick, portCarCall, portDoorBtn, portAssign, portReading, portBrake, portDoor, portBeam, portChain),
		component.WithOutputs(portRef, portLift, portDoorCmd, portDisplay, portIndicator, portLight, portStatus, portTelemetry),
		component.WithIndexedOutputs(portLED, 1, floors),
		component.WithInitialState(func(s component.State) {
			s.Set("cab", &cabControl{
				mode: modeParked, stops: map[int]bool{}, lit: map[int]bool{}, light: true, chain: true,
				reading: ReadingState{Motion: Motion{Pos: floorPos(start)}, Floor: start, Level: true},
				door:    DoorState{Closed: true},
			})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			k := this.State().Get("cab").(*cabControl)
			if err := k.listen(this); err != nil {
				return err
			}
			if this.InputByName(portTick).HasSignals() {
				if err := k.decide(this, c); err != nil {
					return err
				}
			}
			return report(this, ControllerState{Mode: k.mode, Dir: k.dir, Target: k.target, Stops: slices.Sorted(maps.Keys(k.stops))})
		}),
	)
}

// listen records what the parts and the people report. Feedback can arrive
// in any cycle of a run; nothing moves until the next tick.
func (k *cabControl) listen(this *component.Component) error {
	k.reading = latest(this.InputByName(portReading), k.reading)
	k.door = latest(this.InputByName(portDoor), k.door)
	k.brake = latest(this.InputByName(portBrake), k.brake)
	k.beam = latest(this.InputByName(portBeam), k.beam)
	k.chain = latest(this.InputByName(portChain), k.chain)

	for _, sig := range this.InputByName(portDoorBtn).Signals().All() {
		switch sig.PayloadOrDefault("") {
		case doorOpen:
			k.open = true
		case doorClose:
			k.close = true
		}
	}

	for _, name := range []string{portCarCall, portAssign} {
		for _, sig := range this.InputByName(name).Signals().All() {
			f, err := sig.As[int]()
			if err != nil {
				return err
			}
			if f == k.reading.Floor && k.reading.Level && (k.mode == modeOpening || k.mode == modeDwell) {
				k.served = append(k.served, f) // already here with the doors open
				k.dwell = dwellTime
				continue
			}
			k.stops[f] = true
			if name == portCarCall && !k.lit[f] {
				k.lit[f] = true
				if err := this.OutputByName(ledPort(f)).PutSignals(signal.New(true)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// decide is one tick of the controller: what the brake, the drive and the
// doors do next.
func (k *cabControl) decide(this *component.Component, c string) error {
	here, level := k.reading.Floor, k.reading.Level
	ref, lift, doors := 0.0, false, doorStop

	switch k.mode {
	case modeParked:
		switch {
		case level && (k.stops[here] || k.open):
			k.mode = modeOpening
			if err := k.serve(this, here); err != nil {
				return err
			}
		case !level && len(k.stops) == 0:
			k.stops[here] = true // stopped between floors: level up with the nearest one
		case len(k.stops) > 0 && k.chain && k.door.Closed:
			k.mode = modeStarting
		}
	case modeOpening:
		doors = doorOpen
		if k.door.Open {
			k.mode, k.dwell = modeDwell, dwellTime
		}
	case modeDwell:
		doors = doorOpen
		if k.beam || k.open {
			k.dwell = dwellTime
		}
		if k.close {
			k.dwell = 0
		}
		k.dwell -= dt
		if k.dwell <= 0 && !k.beam {
			k.mode = modeClosing
		}
	case modeClosing:
		doors = doorClose
		switch {
		case k.beam || k.open || k.stops[here]:
			k.mode = modeOpening // never close on anyone
			if err := k.serve(this, here); err != nil {
				return err
			}
		case k.door.Closed:
			k.mode = modeParked
		}
	case modeStarting:
		lift = true
		k.dir = direction(k.reading.Pos, k.dir, k.stops)
		k.target = k.nextStop()
		if k.brake {
			k.mode = modeRunning
		}
	case modeRunning:
		lift = true
		k.target = k.nextStop()
		d := floorPos(k.target) - k.reading.Pos
		if math.Abs(d) < stopTolerance {
			k.mode = modeStopping
		} else {
			ref = math.Copysign(max(creepSpeed, min(ratedSpeed, math.Sqrt(2*profileDecel*math.Abs(d)))), d)
		}
	case modeStopping:
		// Hold the brake open until the motor has stopped, then let it drop.
		lift = math.Abs(k.reading.Speed) > 0
		if !lift && !k.brake {
			k.mode = modeParked
		}
	case modeHalted:
		if k.chain && k.reading.Speed == 0 {
			k.mode = modeParked
		}
	}
	if !k.chain && (k.mode == modeStarting || k.mode == modeRunning || k.mode == modeStopping) {
		k.mode, ref, lift = modeHalted, 0, false
	}
	if k.mode != modeStarting && k.mode != modeRunning {
		k.dir = direction(k.reading.Pos, k.dir, k.stops)
	}
	k.open, k.close = false, false

	if err := this.OutputByName(portRef).PutSignals(signal.New(ref)); err != nil {
		return err
	}
	if err := this.OutputByName(portLift).PutSignals(signal.New(lift)); err != nil {
		return err
	}
	if err := this.OutputByName(portDoorCmd).PutSignals(signal.New(doors)); err != nil {
		return err
	}
	return k.show(this, c)
}

// serve opens the doors at a floor: its stop is done and its LED goes out.
func (k *cabControl) serve(this *component.Component, f int) error {
	delete(k.stops, f)
	k.served = append(k.served, f)
	if k.lit[f] {
		delete(k.lit, f)
		return this.OutputByName(ledPort(f)).PutSignals(signal.New(false))
	}
	return nil
}

// show updates whatever people see: the light, the display and lanterns,
// and the status for the dispatcher.
func (k *cabControl) show(this *component.Component, c string) error {
	if k.mode == modeParked && len(k.stops) == 0 {
		k.idle += dt
	} else {
		k.idle = 0
	}
	if light := k.idle < lightTimeout; light != k.light {
		k.light = light
		if err := this.OutputByName(portLight).PutSignals(signal.New(light)); err != nil {
			return err
		}
	}

	ind := Indicator{Floor: k.reading.Floor, Dir: k.dir}
	if k.mode == modeOpening || k.mode == modeDwell {
		ind.Arrived = k.reading.Floor
	}
	if ind != k.shown {
		k.shown = ind
		if err := this.OutputByName(portDisplay).PutSignals(signal.New(ind)); err != nil {
			return err
		}
		if err := this.OutputByName(portIndicator).PutSignals(signal.New(ind)); err != nil {
			return err
		}
	}

	served := k.served
	if len(served) == 0 {
		served = []int{0}
	}
	for _, f := range served {
		if err := this.OutputByName(portStatus).PutSignals(signal.New(CabStatus{
			Cab: c, Pos: k.reading.Pos, Dir: k.dir, Stops: slices.Sorted(maps.Keys(k.stops)), Served: f,
		})); err != nil {
			return err
		}
	}
	k.served = nil
	return nil
}

// nextStop is where the car is heading: the stop it is already braking
// for, unless a nearer one ahead came in early enough to brake for it too.
func (k *cabControl) nextStop() int {
	pos, speed := k.reading.Pos, math.Abs(k.reading.Speed)
	braking := speed * speed / (2 * profileDecel)
	ahead := func(f int) float64 { return (floorPos(f) - pos) * float64(k.dir) }

	best, bestDist := 0, math.Inf(1)
	if k.stops[k.target] && ahead(k.target) > -stopTolerance {
		best, bestDist = k.target, ahead(k.target)
	}
	for f := range k.stops {
		if d := ahead(f); d > -stopTolerance && d >= braking && d < bestDist {
			best, bestDist = f, d
		}
	}
	if best == 0 { // nothing it can brake for in time: take the nearest ahead
		for f := range k.stops {
			if d := ahead(f); d > -stopTolerance && d < bestDist {
				best, bestDist = f, d
			}
		}
	}
	if best == 0 {
		return k.reading.Floor
	}
	return best
}

// direction keeps going the way the car is going while there are stops
// ahead, then turns around: the rule real elevators follow.
func direction(pos float64, dir int, stops map[int]bool) int {
	ahead := func(d int) bool {
		for f := range stops {
			if (floorPos(f)-pos)*float64(d) > stopTolerance {
				return true
			}
		}
		return false
	}
	switch {
	case dir != 0 && ahead(dir):
		return dir
	case ahead(+1):
		return +1
	case ahead(-1):
		return -1
	}
	return 0
}

func ledPort(f int) string { return fmt.Sprint(portLED, f) }

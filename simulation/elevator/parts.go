package main

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Port names.
const (
	portTick     = "tick"     // UI → cab: one second has passed
	portPress    = "press"    // UI → floor: the hall call button was pressed
	portButton   = "button"   // UI → cab: a floor button inside the cab was pressed
	portCall     = "call"     // floor → dispatcher
	portAssign   = "assign"   // dispatcher → cab: please stop at this floor
	portServed   = "served"   // dispatcher → floor: a cab is here, the lamp goes off
	portStatus   = "status"   // cab → dispatcher, display
	portLamp     = "lamp"     // floor → display
	portCmd      = "cmd"      // cab → motor, door
	portDrive    = "drive"    // cab's output to its motor
	portDoors    = "doors"    // cab's output to its door
	portPosition = "position" // motor → cab
	portState    = "state"    // door → cab
	portView     = "view"     // display → UI
)

// Motor and door commands.
const (
	cmdUp    = "up"
	cmdDown  = "down"
	cmdHold  = "hold"
	cmdOpen  = "open"
	doorOpen = "open"
	doorShut = "closed"

	doorOpenTicks = 3
)

// CabStatus is what a cab tells the dispatcher and the display.
type CabStatus struct {
	Cab    string
	Floor  int
	Dir    int // +1 up, -1 down, 0 idle
	Door   string
	Stops  []int // floors it will stop at, sorted
	Served int   // the floor whose doors just opened, or 0
}

// Lamp is a floor's call lamp.
type Lamp struct {
	Floor int
	On    bool
}

// View is everything the UI draws: the mesh's state, sent out as a signal.
type View struct {
	Cabs  map[string]CabStatus
	Lamps map[int]bool
}

func floorName(f int) string  { return fmt.Sprintf("floor-%d", f) }
func cabName(c string) string { return "cab-" + c }

// newFloor is the call button on one landing, with its lamp.
func newFloor(f int) (*component.Component, error) {
	return component.New(floorName(f),
		component.WithDescription(fmt.Sprintf("hall call button and lamp on floor %d", f)),
		component.WithInputs(portPress, portServed),
		component.WithOutputs(portCall, portLamp),
		component.WithInitialState(func(s component.State) { s.Set("lit", false) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			lit := this.State().GetOrDefault("lit", false).(bool)
			if this.InputByName(portServed).HasSignals() {
				lit = false
			}
			if this.InputByName(portPress).HasSignals() && !lit {
				lit = true
				if err := this.OutputByName(portCall).PutSignals(signal.New(f)); err != nil {
					return err
				}
			}
			this.State().Set("lit", lit)
			return this.OutputByName(portLamp).PutSignals(signal.New(Lamp{Floor: f, On: lit}))
		}),
	)
}

// newDispatcher hands every hall call to one cab, and puts out the lamp of
// a floor once a cab opens its doors there.
func newDispatcher(cabs []string) (*component.Component, error) {
	outputs := []string{}
	for _, c := range cabs {
		outputs = append(outputs, portAssign+"-"+c)
	}
	return component.New("dispatcher",
		component.WithDescription("assigns each hall call to the cab that can get there first"),
		component.WithInputs(portCall, portStatus),
		component.WithOutputs(outputs...),
		component.WithIndexedOutputs(portServed, 1, floors),
		component.WithInitialState(func(s component.State) {
			s.Set("cabs", map[string]CabStatus{})
			s.Set("pending", map[int]string{}) // floor → cab it was given to
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			known := this.State().Get("cabs").(map[string]CabStatus)
			pending := this.State().Get("pending").(map[int]string)

			for _, sig := range this.InputByName(portStatus).Signals().All() {
				st, err := sig.As[CabStatus]()
				if err != nil {
					return err
				}
				known[st.Cab] = st
				if st.Served > 0 {
					delete(pending, st.Served)
					if err := this.OutputByName(fmt.Sprint(portServed, st.Served)).PutSignals(signal.New(st.Cab)); err != nil {
						return err
					}
				}
			}

			for _, sig := range this.InputByName(portCall).Signals().All() {
				f, err := sig.As[int]()
				if err != nil {
					return err
				}
				if _, ok := pending[f]; ok {
					continue
				}
				best := slices.MinFunc(cabs, func(a, b string) int { return cost(known[a], f) - cost(known[b], f) })
				pending[f] = best
				if err := this.OutputByName(portAssign + "-" + best).PutSignals(signal.New(f)); err != nil {
					return err
				}
			}
			return nil
		}),
	)
}

// cost estimates how long a cab would take to reach floor f.
func cost(st CabStatus, f int) int {
	c := abs(st.Floor-f) + 2*len(st.Stops)
	if st.Dir != 0 && (f-st.Floor)*st.Dir < 0 {
		c += 2 * floors // it is heading away and must turn around first
	}
	return c
}

// newCab is a cab's controller: on every tick it decides whether to move,
// stop or open the doors, and tells its motor and door. Between ticks it
// listens: where the motor got to, what the door is doing, which floors the
// dispatcher and the passengers want.
func newCab(c string) (*component.Component, error) {
	return component.New(cabName(c),
		component.WithDescription("cab controller: drives the motor and the door, serves its stops"),
		component.WithInputs(portTick, portButton, portAssign, portPosition, portState),
		component.WithOutputs(portDrive, portDoors, portStatus),
		component.WithInitialState(func(s component.State) {
			s.Set("floor", 1)
			s.Set("dir", 0)
			s.Set("door", doorShut)
			s.Set("stops", map[int]bool{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			floor := this.State().Get("floor").(int)
			dir := this.State().Get("dir").(int)
			door := this.State().Get("door").(string)
			stops := this.State().Get("stops").(map[int]bool)
			served := 0

			// Listen: feedback from the motor and the door.
			floor = this.InputByName(portPosition).Signals().FirstPayloadOrDefault(floor)
			door = this.InputByName(portState).Signals().FirstPayloadOrDefault(door)

			// Listen: new stops, from inside the cab and from the dispatcher.
			for _, port := range []string{portButton, portAssign} {
				for _, sig := range this.InputByName(port).Signals().All() {
					f, err := sig.As[int]()
					if err != nil {
						return err
					}
					if f == floor && door == doorOpen {
						served = f // already here with the doors open
						continue
					}
					stops[f] = true
				}
			}

			// Decide, once per tick.
			if this.InputByName(portTick).HasSignals() {
				drive, doors := cmdHold, cmdHold
				if door == doorShut && stops[floor] {
					delete(stops, floor)
					doors, served = cmdOpen, floor
				}
				// The arrow shows where the cab goes next, even while it waits.
				dir = direction(floor, dir, stops)
				if door == doorShut && doors != cmdOpen {
					switch dir {
					case +1:
						drive = cmdUp
					case -1:
						drive = cmdDown
					}
				}
				if err := this.OutputByName(portDrive).PutSignals(signal.New(drive)); err != nil {
					return err
				}
				if err := this.OutputByName(portDoors).PutSignals(signal.New(doors)); err != nil {
					return err
				}
			}

			this.State().Set("floor", floor)
			this.State().Set("dir", dir)
			this.State().Set("door", door)
			this.State().Set("stops", stops)
			return this.OutputByName(portStatus).PutSignals(signal.New(CabStatus{
				Cab: c, Floor: floor, Dir: dir, Door: door, Served: served,
				Stops: slices.Sorted(maps.Keys(stops)),
			}))
		}),
	)
}

// direction keeps going the way the cab is going while there are stops
// ahead, then turns around: the rule real elevators follow.
func direction(floor, dir int, stops map[int]bool) int {
	ahead := func(d int) bool {
		for f := range stops {
			if (f-floor)*d > 0 {
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

// newMotor moves its cab one floor per tick.
func newMotor(c string) (*component.Component, error) {
	return component.New("motor-"+c,
		component.WithDescription("hoist motor: one floor per tick, up or down"),
		component.WithInputs(portCmd),
		component.WithOutputs(portPosition),
		component.WithInitialState(func(s component.State) { s.Set("floor", 1) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			floor := this.State().Get("floor").(int)
			switch this.InputByName(portCmd).Signals().FirstPayloadOrDefault(cmdHold) {
			case cmdUp:
				floor = min(floors, floor+1)
			case cmdDown:
				floor = max(1, floor-1)
			}
			this.State().Set("floor", floor)
			return this.OutputByName(portPosition).PutSignals(signal.New(floor))
		}),
	)
}

// newDoor opens on command and closes by itself a few ticks later.
func newDoor(c string) (*component.Component, error) {
	return component.New("door-"+c,
		component.WithDescription("cab door: opens on command, closes by itself"),
		component.WithInputs(portCmd),
		component.WithOutputs(portState),
		component.WithInitialState(func(s component.State) { s.Set("open for", 0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			left := this.State().Get("open for").(int)
			if this.InputByName(portCmd).Signals().FirstPayloadOrDefault(cmdHold) == cmdOpen {
				left = doorOpenTicks
			} else {
				left = max(0, left-1)
			}
			this.State().Set("open for", left)

			state := doorShut
			if left > 0 {
				state = doorOpen
			}
			return this.OutputByName(portState).PutSignals(signal.New(state))
		}),
	)
}

// newDisplay gathers the state of every cab and lamp into one View for the
// UI: the way state leaves the mesh.
func newDisplay() (*component.Component, error) {
	return component.New("display",
		component.WithDescription("collects cab and lamp state into one view for the UI"),
		component.WithInputs(portStatus, portLamp),
		component.WithOutputs(portView),
		component.WithInitialState(func(s component.State) {
			s.Set("view", View{Cabs: map[string]CabStatus{}, Lamps: map[int]bool{}})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			old := this.State().Get("view").(View)
			// A fresh View every time: a payload that left the component is
			// someone else's to read.
			view := View{Cabs: maps.Clone(old.Cabs), Lamps: maps.Clone(old.Lamps)}
			for _, sig := range this.InputByName(portStatus).Signals().All() {
				st, err := sig.As[CabStatus]()
				if err != nil {
					return err
				}
				view.Cabs[st.Cab] = st
			}
			for _, sig := range this.InputByName(portLamp).Signals().All() {
				l, err := sig.As[Lamp]()
				if err != nil {
					return err
				}
				view.Lamps[l.Floor] = l.On
			}
			this.State().Set("view", view)
			return this.OutputByName(portView).PutSignals(signal.New(view))
		}),
	)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

package main

import (
	"context"
	"maps"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// The building.
const (
	floors      = 6
	floorHeight = 3.5                        // metres between landings
	travel      = (floors - 1) * floorHeight // metres from the lowest landing to the top one
	dt          = 0.05                       // simulated seconds per run of the mesh
	ratedSpeed  = 1.6                        // m/s
	sheaveDiam  = 0.5                        // m, so the motor turns at about 61 rpm at rated speed
	ratedRPM    = ratedSpeed / (math.Pi * sheaveDiam) * 60
)

var cabs = []string{"A", "B"}

// Port names shared by many parts.
const (
	portTelemetry = "telemetry" // every part → panel: what the UI draws
	portPress     = "press"     // UI → any button
	portPressed   = "pressed"   // button → whoever it is wired to
	portSet       = "set"       // → lamp, LED
	portShow      = "show"      // → display, lantern
	portTick      = "tick"      // clock → controllers, light curtains
	portPulse     = "pulse"     // UI loop → clock
	portFrame     = "frame"     // panel → UI
)

// floorPos is the height of a landing above the lowest one.
func floorPos(f int) float64 { return float64(f-1) * floorHeight }

// nearestFloor is the landing closest to a height in the shaft.
func nearestFloor(pos float64) int {
	return min(floors, max(1, int(pos/floorHeight+0.5)+1))
}

// Report is one part's state on its way to the panel.
type Report struct {
	Part  string
	State any
}

// report sends a part's state to the panel. Every part has a telemetry
// output, all of them piped into the one panel: the way state leaves the mesh.
func report(this *component.Component, state any) error {
	return this.OutputByName(portTelemetry).PutSignals(signal.New(Report{Part: this.Name(), State: state}))
}

// latest is the newest payload on a port, or def when it is empty. A part
// that only listens on a port keeps the newest value it heard.
func latest[T any](p *port.Port, def T) T {
	all := p.Signals().All()
	if len(all) == 0 {
		return def
	}
	return all[len(all)-1].PayloadOrDefault(def)
}

// newButton is any push button in the building: on a press it sends what it
// stands for (a floor, a hall call, "open") to whatever it is wired to. Its
// light, if it has one, is a separate lamp lit by a controller, as in a
// real installation.
func newButton(name, description string, sends any) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(description),
		component.WithInputs(portPress),
		component.WithOutputs(portPressed),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName(portPressed).PutSignals(signal.New(sends))
		}),
	)
}

// LampState is what the UI knows about a lamp or an LED.
type LampState struct {
	On bool `json:"on"`
}

// newLamp is an LED in a button ring or the light in a cab: on or off, as
// it is told.
func newLamp(name, description string) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(description),
		component.WithInputs(portSet),
		component.WithOutputs(portTelemetry),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return report(this, LampState{On: latest(this.InputByName(portSet), false)})
		}),
	)
}

// Indicator is what a controller shows on its displays and lanterns.
type Indicator struct {
	Floor   int `json:"floor"`   // where the cab is
	Dir     int `json:"dir"`     // +1 up, -1 down, 0 idle
	Arrived int `json:"arrived"` // the floor where it stands with doors opening, or 0
}

// IndicatorState is what one display or lantern shows.
type IndicatorState struct {
	Floor int  `json:"floor"`
	Dir   int  `json:"dir"`
	Gong  bool `json:"gong"` // a lantern lights its arrow when the cab arrives at its floor
}

// newIndicator is a floor display: the one inside a cab (floor 0) or a hall
// lantern above a landing door, which also lights its arrival arrow.
func newIndicator(name, description string, floor int) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(description),
		component.WithInputs(portShow),
		component.WithOutputs(portTelemetry),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			in := latest(this.InputByName(portShow), Indicator{Floor: 1})
			return report(this, IndicatorState{Floor: in.Floor, Dir: in.Dir, Gong: floor > 0 && in.Arrived == floor})
		}),
	)
}

// ClockState is the time of the building.
type ClockState struct {
	Tick    int     `json:"tick"`
	Seconds float64 `json:"seconds"`
}

// newClock turns one pulse from outside into one tick for every part that
// keeps time: the start of every run.
func newClock() (*component.Component, error) {
	return component.New("clock",
		component.WithDescription("one tick per run, fanned out to every part that keeps time"),
		component.WithInputs(portPulse),
		component.WithOutputs(portTick, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("tick", 0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			tick := this.State().Get("tick").(int) + 1
			this.State().Set("tick", tick)
			if err := this.OutputByName(portTick).PutSignals(signal.New(tick)); err != nil {
				return err
			}
			return report(this, ClockState{Tick: tick, Seconds: float64(tick) * dt})
		}),
	)
}

// Frame is the state of every part, keyed by part name.
type Frame map[string]any

// newPanel collects the reports of every part into one frame for the UI.
func newPanel() (*component.Component, error) {
	return component.New("panel",
		component.WithDescription("collects every part's report into one frame for the UI"),
		component.WithInputs(portTelemetry),
		component.WithOutputs(portFrame),
		component.WithInitialState(func(s component.State) { s.Set("frame", Frame{}) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			// A fresh map every time: a payload that left the component is
			// someone else's to read.
			frame := maps.Clone(this.State().Get("frame").(Frame))
			for _, sig := range this.InputByName(portTelemetry).Signals().All() {
				r, err := sig.As[Report]()
				if err != nil {
					return err
				}
				frame[r.Part] = r.State
			}
			this.State().Set("frame", frame)
			return this.OutputByName(portFrame).PutSignals(signal.New(frame))
		}),
	)
}

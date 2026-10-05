package main

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// What flows through the pipes. Most signals are plain float64s (a torque, a
// voltage, a temperature); the ones below carry more than one number.

// Crank is what the crankshaft announces at the start of every stroke.
type Crank struct {
	Stroke int     `json:"stroke"` // strokes since the engine was first started
	RPM    float64 `json:"rpm"`
	DT     float64 `json:"dt"` // how long this stroke takes, in seconds
}

// Drive is a pulley turning on the accessory belt.
type Drive struct {
	RPM float64 `json:"rpm"`
	DT  float64 `json:"dt"`
}

// Current is what the alternator puts out: so many amps for so long.
type Current struct {
	Amps float64 `json:"amps"`
	DT   float64 `json:"dt"`
}

// Controls is what the cockpit tells the ECU: the pedal and the switches it reads.
type Controls struct {
	Pedal    float64 `json:"pedal"` // 0..1
	Ignition bool    `json:"ignition"`
	AC       bool    `json:"ac"`
}

// Air is what the throttle body lets through to the intake manifold.
type Air struct {
	Opening float64 `json:"opening"` // throttle plate, 0..1
	Density float64 `json:"density"` // 1 = clean filter at sea level
}

// Fuel is what the pump pushes into the rail.
type Fuel struct {
	Amount   float64 `json:"amount"`   // in charges: 1 is a full cylinder's worth at lambda 1
	Pressure float64 `json:"pressure"` // bar
}

// Injection is the ECU's order to the fuel rail for this stroke.
type Injection struct {
	Cylinder int     `json:"cylinder"`
	Amount   float64 `json:"amount"`
}

// Exhaust is the burnt gas a cylinder pushes out.
type Exhaust struct {
	Gas    float64 `json:"gas"`    // how much, as a fraction of a full cylinder
	Lambda float64 `json:"lambda"` // air/fuel ratio of what burnt, 1 = stoichiometric
	Temp   float64 `json:"temp"`   // °C
}

// Coolant is the water flowing from the engine block to the radiator.
type Coolant struct {
	Temp float64 `json:"temp"` // °C
	Flow float64 `json:"flow"` // 1 = what the water pump moves at 3000 rpm
	DT   float64 `json:"dt"`
}

// LambdaReading is the oxygen sensor's verdict. Cold, it has none.
type LambdaReading struct {
	Lambda float64 `json:"lambda"`
	Ready  bool    `json:"ready"`
}

// Status is what the ECU shows on the instrument cluster.
type Status struct {
	RPM         float64 `json:"rpm"`
	Mode        string  `json:"mode"` // off, cranking, running, rev limit, overheat, stalled, no start
	CheckEngine bool    `json:"checkEngine"`
}

// The four strokes, in the order every cylinder goes through them.
const (
	intake = iota
	compression
	power
	exhaust
)

const cylinders = 4

// The firing order of an inline four: cylinder firingOrder[n%4] makes power
// on stroke n. Spreading the power strokes evenly is what keeps it smooth.
var firingOrder = [cylinders]int{1, 3, 4, 2}

// phase is where cylinder i is in its four-stroke cycle on stroke n. The ECU
// and the camshaft use it to time fuel, spark and valves; the cylinders just
// count their own strokes and come out the same, or the engine would not run.
func phase(i, n int) int {
	return (power - slices.Index(firingOrder[:], i) + cylinders + n%cylinders) % cylinders
}

// onStroke is the cylinder in the given phase on stroke n.
func onStroke(ph, n int) int {
	for i := 1; i <= cylinders; i++ {
		if phase(i, n) == ph {
			return i
		}
	}
	return 0
}

// ve is the volumetric efficiency: how well the engine breathes at a given
// speed. It peaks in the middle of the rev range, like a real one.
func ve(rpm float64) float64 {
	d := (rpm - 4500) / 5000
	return max(0.5, 0.88-0.4*d*d)
}

func cylinderName(i int) string { return fmt.Sprintf("cylinder-%d", i) }

func clamp(v, lo, hi float64) float64 { return math.Min(hi, math.Max(lo, v)) }

// in reads the signal on an input port, or the zero value if there is none.
func in[T any](this *component.Component, port string) T {
	var zero T
	return this.InputByName(port).Signals().FirstPayloadOrDefault(zero)
}

// get reads a value from the component's state.
func get[T any](this *component.Component, key string, def T) T {
	if v, ok := this.State().GetOrDefault(key, def).(T); ok {
		return v
	}
	return def
}

// listen is how a part keeps track of a reading without waiting for it: the
// latest signal on each of the given inputs goes into the component's state
// under the port's name, and the port is cleared. Sensors report late in a
// stroke, so a part that waited for them would wait for a stroke that has
// not started yet; it uses what it heard last instead, like a real ECU.
func listen(ports ...string) component.ActivationFunc {
	return func(ctx context.Context, this *component.Component) error {
		for _, name := range ports {
			p := this.InputByName(name)
			if !p.HasSignals() {
				continue
			}
			payloads := p.Signals().AllPayloads()
			this.State().Set(name, payloads[len(payloads)-1])
			if err := p.Clear(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// emit puts one signal on each of the named outputs.
func emit(this *component.Component, payloads map[string]any) error {
	for name, payload := range payloads {
		if err := this.OutputByName(name).PutSignals(signal.New(payload)); err != nil {
			return err
		}
	}
	return nil
}

// toEachCylinder puts one signal on each of the indexed outputs prefix1..4.
func toEachCylinder(this *component.Component, prefix string, payload func(i int) any) error {
	for i := 1; i <= cylinders; i++ {
		if err := this.OutputByName(fmt.Sprint(prefix, i)).PutSignals(signal.New(payload(i))); err != nil {
			return err
		}
	}
	return nil
}

// part is a shorthand for the components that wait for one signal on each
// of the required inputs per stroke, optionally listen to others, and then act.
func part(name, description string, required, listened, outputs []string, initial map[string]any, act component.ActivationFunc) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(description),
		component.WithInputs(append(slices.Clone(required), listened...)...),
		component.WithOutputs(outputs...),
		component.WithInitialState(func(s component.State) {
			for k, v := range initial {
				s.Set(k, v)
			}
		}),
		component.WithActivationFunc(component.Sequential(
			listen(listened...),
			component.RequireInputs(required...),
			act,
		)),
	)
}

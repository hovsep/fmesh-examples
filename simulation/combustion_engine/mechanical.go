package main

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The rotating heart: crankshaft, camshaft, cylinders, valves and the belt
// that drives the accessories.

const (
	inertia    = 0.15  // kg·m², crankshaft and flywheel
	fullTorque = 172.0 // N·m from one full, perfectly burnt cylinder on its power stroke
)

// torqueSources is everything that pushes or drags the crankshaft.
func torqueSources() []string {
	sources := []string{"starter", "belt", "oil-pump"}
	for i := 1; i <= cylinders; i++ {
		sources = append(sources, cylinderName(i))
	}
	return sources
}

// strokeTime is how long half a turn takes. Below cranking speed it is
// capped so that one stroke never covers too long a stretch of time.
func strokeTime(rpm float64) float64 { return 30 / max(rpm, 300) }

// newCrankshaft sums the torque of the starter, the cylinders and everything
// that drags on it, turns faster or slower, and announces the next stroke.
// That announcement is what sets every other part in motion again: the loop
// that makes an engine run. When it stops turning it announces nothing, and
// the mesh stops with it.
func newCrankshaft() (*component.Component, error) {
	sources := torqueSources()
	required := append(sources, "oil")

	return component.New("crankshaft",
		component.WithDescription("sums the torque, turns, and starts the next stroke"),
		component.WithInputs(append(required, "key")...),
		component.WithOutputs("crank"),
		component.WithInitialState(func(s component.State) {
			s.Set("crank", Crank{Stroke: -1})
			s.Set("torque", 0.0)
		}),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			crank := get(this, "crank", Crank{})

			// The key: the first stroke starts from standstill.
			if this.InputByName("key").HasSignals() {
				crank.Stroke++
				crank.RPM = 0
				crank.DT = strokeTime(0)
				this.State().Set("crank", crank)
				return this.OutputByName("crank").PutSignals(signal.New(crank))
			}

			if err := component.RequireInputs(required...)(ctx, this); err != nil {
				return err
			}

			torque := 0.0
			for _, name := range sources {
				torque += in[float64](this, name)
			}
			// Friction grows with speed, and without oil pressure the
			// bearings run dry and drag much harder.
			friction := (8 + 0.004*crank.RPM) * (1 + max(0, 1-in[float64](this, "oil")))
			this.State().Set("torque", torque-friction)

			omega := crank.RPM * math.Pi / 30
			if torque > friction || omega > 0 {
				omega += (torque - friction) / inertia * crank.DT
			}
			crank.RPM = max(0, omega*30/math.Pi)

			if crank.RPM == 0 {
				// Still, and nothing turning it: the engine has stopped.
				this.State().Set("crank", crank)
				return nil
			}
			crank.Stroke++
			crank.DT = strokeTime(crank.RPM)
			this.State().Set("crank", crank)
			return this.OutputByName("crank").PutSignals(signal.New(crank))
		}),
	)
}

// newCamshaft turns at half the crankshaft's speed and opens each valve on
// its cylinder's intake or exhaust stroke.
func newCamshaft() (*component.Component, error) {
	return part("camshaft", "opens the intake and exhaust valves, half a turn per two strokes",
		[]string{"crank"}, nil,
		append(indexed("intake"), indexed("exhaust")...),
		map[string]any{"angle": 0.0},
		func(_ context.Context, this *component.Component) error {
			n := in[Crank](this, "crank").Stroke
			this.State().Set("angle", float64(n%4)*90)
			if err := toEachCylinder(this, "intake", func(i int) any { return phase(i, n) == intake }); err != nil {
				return err
			}
			return toEachCylinder(this, "exhaust", func(i int) any { return phase(i, n) == exhaust })
		})
}

// newValve passes what reaches it while the cam holds it open.
func newValve(kind string, i int, flow string) (*component.Component, error) {
	return part(fmt.Sprintf("%s-valve-%d", kind, i), kind+" valve: opens when the cam lobe pushes it",
		[]string{flow, "lift"}, nil, []string{flow},
		map[string]any{"open": false},
		func(_ context.Context, this *component.Component) error {
			open := in[bool](this, "lift")
			this.State().Set("open", open)
			payload := this.InputByName(flow).Signals().FirstPayloadOrNil()
			if !open {
				switch payload.(type) {
				case float64:
					payload = 0.0
				case Exhaust:
					payload = Exhaust{}
				}
			}
			return this.OutputByName(flow).PutSignals(signal.New(payload))
		})
}

// burnEfficiency is how much of a mixture's energy a spark turns into work,
// by its air/fuel ratio: best a little rich, nothing when too lean or too
// rich to light.
func burnEfficiency(lambda float64) float64 {
	if lambda < 0.6 || lambda > 1.45 {
		return 0
	}
	return max(0, 1-2.5*(lambda-0.88)*(lambda-0.88))
}

// newCylinder is one cylinder and its piston. It keeps its own place in the
// four-stroke cycle; the crankshaft moves it one stroke per trip around the
// mesh. What the intake valve lets in and what the injector sprays burns on
// the power stroke, if the spark comes, and leaves on the exhaust stroke.
func newCylinder(i int) (*component.Component, error) {
	return part(cylinderName(i), "intake → compression → power → exhaust",
		[]string{"air", "fuel", "spark"}, nil,
		[]string{"torque", "heat", "exhaust"},
		map[string]any{
			"stroke": phase(i, 0),
			"air":    0.0, "fuel": 0.0, "lambda": 0.0,
			"fired": false, "burn": 0.0,
		},
		func(_ context.Context, this *component.Component) error {
			stroke := get(this, "stroke", 0)
			air, fuel := get(this, "air", 0.0), get(this, "fuel", 0.0)
			torque, heat, gas := 0.0, 0.0, Exhaust{}
			fired, burn := false, get(this, "burn", 0.0)

			switch stroke {
			case intake:
				air, fuel = in[float64](this, "air"), in[float64](this, "fuel")
				torque = -5 * (1 - air) // pulling against a closed throttle: engine braking
				burn = 0
			case compression:
				torque = -1
			case power:
				lambda := 9.99 // air and no fuel: as lean as it gets
				if fuel > 0 {
					lambda = air / fuel
				}
				this.State().Set("lambda", lambda)
				if in[bool](this, "spark") && air > 0 {
					burn = min(air, fuel) * burnEfficiency(lambda)
				}
				fired = burn > 0
				torque = fullTorque * burn
				heat = 0.9 * burn // kJ into the water jacket
			case exhaust:
				lambda := get(this, "lambda", 0.0)
				gas = Exhaust{Gas: air, Lambda: lambda, Temp: 300 + 550*min(1, burn/0.6)}
				air, fuel = 0, 0
			}

			this.State().Set("stroke", (stroke+1)%cylinders)
			this.State().Set("air", air)
			this.State().Set("fuel", fuel)
			this.State().Set("fired", fired)
			this.State().Set("burn", burn)
			return emit(this, map[string]any{"torque": torque, "heat": heat, "exhaust": gas})
		})
}

// Pulley ratios on the accessory belt: the small alternator pulley spins fastest.
var pulleys = map[string]float64{"alternator": 2.8, "water-pump": 1.1, "ac-compressor": 1.3}

// newAccessoryBelt carries rotation from the crankshaft to the alternator,
// the water pump and the AC compressor, and their drag back. It works in
// two beats of one stroke: the crank turns it, then the accessories answer.
func newAccessoryBelt() (*component.Component, error) {
	var drives, drags []string
	for _, name := range []string{"alternator", "water-pump", "ac-compressor"} {
		drives = append(drives, "drive-"+name)
		drags = append(drags, "drag-"+name)
	}
	return component.New("accessory-belt",
		component.WithDescription("crank pulley → alternator, water pump, AC compressor; their drag back"),
		component.WithInputs(append(drags, "crank")...),
		component.WithOutputs(append(drives, "torque")...),
		component.WithInitialState(func(s component.State) { s.Set("rpm", 0.0) }),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			if this.InputByName("crank").HasSignals() {
				crank := in[Crank](this, "crank")
				this.State().Set("rpm", crank.RPM)
				out := map[string]any{}
				for name, ratio := range pulleys {
					out["drive-"+name] = Drive{RPM: crank.RPM * ratio, DT: crank.DT}
				}
				return emit(this, out)
			}
			if err := component.RequireInputs(drags...)(ctx, this); err != nil {
				return err
			}
			drag := 0.0
			for _, name := range drags {
				drag += in[float64](this, name)
			}
			return this.OutputByName("torque").PutSignals(signal.New(drag))
		}),
	)
}

// indexed lists the ports prefix1..prefix4, one per cylinder.
func indexed(prefix string) []string {
	names := make([]string, 0, cylinders)
	for i := 1; i <= cylinders; i++ {
		names = append(names, fmt.Sprint(prefix, i))
	}
	return names
}

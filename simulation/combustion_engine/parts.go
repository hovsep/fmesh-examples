package main

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Every part waits for one signal on each of its inputs per stroke
// (component.RequireInputs), so flows of different lengths, the short control
// path and the long fuel path, meet up in the right stroke.

// Target names the two cylinders that need something this stroke.
type Target struct {
	Intake int // gets fuel
	Power  int // gets a spark
}

// newCrankshaft sums the torque of the starter and every cylinder, turns
// faster or slower, and announces the next stroke. That announcement is what
// sets everything else in motion again: the loop that makes an engine run.
func newCrankshaft() (*component.Component, error) {
	torqueInputs := []string{portStarter}
	for i := 1; i <= cylinders; i++ {
		torqueInputs = append(torqueInputs, cylinderName(i))
	}

	return component.New("crankshaft",
		component.WithDescription("sums the torque, turns, and starts the next stroke"),
		component.WithInputs(append(torqueInputs, portKey)...),
		component.WithOutputs(portCrank),
		component.WithInitialState(func(s component.State) { s.Set("crank", Crank{}) }),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			if this.InputByName(portKey).HasSignals() {
				return this.OutputByName(portCrank).PutSignals(signal.New(Crank{}))
			}
			if err := component.RequireInputs(torqueInputs...)(ctx, this); err != nil {
				return err
			}

			torque := 0.0
			for _, name := range torqueInputs {
				torque += this.InputByName(name).Signals().FirstPayloadOrDefault(0.0)
			}
			starter := this.InputByName(portStarter).Signals().FirstPayloadOrDefault(0.0)

			crank, err := this.State().GetTyped[Crank]("crank")
			if err != nil {
				return err
			}
			// Torque speeds it up; friction, which grows with speed, slows it down.
			crank.RPM = max(0, crank.RPM+torque*10-(40+crank.RPM*0.2))
			if crank.RPM == 0 && starter == 0 {
				// Not turning and nothing to turn it: the engine has stopped,
				// and so does the mesh, since nothing starts another stroke.
				this.State().Set("crank", crank)
				return nil
			}
			crank.Stroke++
			this.State().Set("crank", crank)
			return this.OutputByName(portCrank).PutSignals(signal.New(crank))
		}),
	)
}

// newECU is the engine's computer. It reads the crankshaft, follows the
// driver's throttle and decides, for this stroke: whether the starter turns,
// how much fuel to draw, which cylinder gets it and which one gets a spark.
func newECU() (*component.Component, error) {
	return component.New("ecu",
		component.WithDescription("engine control unit: starter, fuel and spark timing"),
		component.WithInputs(portCrank),
		component.WithOutputs(portEngage, portLoad, portDemand, portTarget),
		component.WithInitialState(func(s component.State) {
			s.Set("started", false)
			s.Set("throttle", 0.0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			crank, err := this.InputByName(portCrank).Signals().FirstAs[Crank]()
			if err != nil {
				return err
			}

			started := this.State().GetOrDefault("started", false).(bool) || crank.RPM >= startedRPM
			this.State().Set("started", started)
			engage := !started

			throttle := driver(crank.Stroke)
			this.State().Set("throttle", throttle)

			load := 8.0 // ignition and pump
			if engage {
				load += 120 // the starter is the battery's heaviest customer
			}

			n := crank.Stroke
			target := Target{
				Intake: firingOrder[(n+2)%cylinders], // fires two strokes from now
				Power:  firingOrder[n%cylinders],
			}

			return errors.Join(
				this.OutputByName(portEngage).PutSignals(signal.New(engage)),
				this.OutputByName(portLoad).PutSignals(signal.New(load)),
				this.OutputByName(portDemand).PutSignals(signal.New(fuelIdle+(fuelFullThrottle-fuelIdle)*throttle)),
				this.OutputByName(portTarget).PutSignals(signal.New(target)),
			)
		}),
	)
}

// driver is the right foot: idle, then floor it, then back to idle.
func driver(stroke int) float64 {
	if stroke >= 100 && stroke < 220 {
		return 1
	}
	return 0
}

// newAlternator turns rotation into charging current.
func newAlternator() (*component.Component, error) {
	return component.New("alternator",
		component.WithDescription("rotation → charging current"),
		component.WithInputs(portCrank),
		component.WithOutputs(portCharge),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			crank, err := this.InputByName(portCrank).Signals().FirstAs[Crank]()
			if err != nil {
				return err
			}
			return this.OutputByName(portCharge).PutSignals(signal.New(min(crank.RPM/50, 40)))
		}),
	)
}

// newBattery charges from the alternator, discharges into the load, and
// tells every consumer the voltage it can count on. A heavy load sags it.
func newBattery() (*component.Component, error) {
	return component.New("battery",
		component.WithDescription("12 V battery: charge in, load out, volts to every consumer"),
		component.WithInputs(portCharge, portLoad),
		component.WithOutputs(portVolts),
		component.WithInitialState(func(s component.State) {
			s.Set("level", batteryCapacity*0.8)
			s.Set("volts", 12.6)
		}),
		component.WithActivationFunc(component.Sequential(
			component.RequireInputs(portCharge, portLoad),
			func(_ context.Context, this *component.Component) error {
				charge := this.InputByName(portCharge).Signals().FirstPayloadOrDefault(0.0)
				load := this.InputByName(portLoad).Signals().FirstPayloadOrDefault(0.0)

				level := this.State().GetOrDefault("level", 0.0).(float64)
				level = min(batteryCapacity, max(0, level+(charge-load)*0.02))
				volts := 10.5 + 2.1*level/batteryCapacity - load*0.012
				this.State().Set("level", level)
				this.State().Set("volts", volts)
				return this.OutputByName(portVolts).PutSignals(signal.New(volts))
			},
		)),
	)
}

// newStarter turns the crankshaft while the ECU says so and the battery can.
func newStarter() (*component.Component, error) {
	return component.New("starter",
		component.WithDescription("electric motor that turns the engine until it runs on its own"),
		component.WithInputs(portVolts, portEngage),
		component.WithOutputs(portTorque),
		component.WithActivationFunc(component.Sequential(
			component.RequireInputs(portVolts, portEngage),
			func(_ context.Context, this *component.Component) error {
				volts := this.InputByName(portVolts).Signals().FirstPayloadOrDefault(0.0)
				engaged := this.InputByName(portEngage).Signals().FirstPayloadOrDefault(false) && volts >= minVolts
				this.State().Set("engaged", engaged)

				torque := 0.0
				if engaged {
					torque = 25
				}
				return this.OutputByName(portTorque).PutSignals(signal.New(torque))
			},
		)),
	)
}

// newFuelTank hands out what the ECU asks for, while it lasts.
func newFuelTank() (*component.Component, error) {
	return component.New("fuel-tank",
		component.WithDescription("holds the fuel; gives what is asked, while it lasts"),
		component.WithInputs(portDemand),
		component.WithOutputs(portFuel),
		component.WithInitialState(func(s component.State) { s.Set("level", tankCapacity) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			demand := this.InputByName(portDemand).Signals().FirstPayloadOrDefault(0.0)
			level := this.State().GetOrDefault("level", 0.0).(float64)
			given := min(demand, level)
			this.State().Set("level", level-given)
			return this.OutputByName(portFuel).PutSignals(signal.New(given))
		}),
	)
}

// newFuelPump pushes the fuel to the injector of the cylinder on its intake
// stroke. It is electric: without voltage, no pressure, no fuel.
func newFuelPump() (*component.Component, error) {
	return component.New("fuel-pump",
		component.WithDescription("electric pump: tank → injector of the cylinder on intake"),
		component.WithInputs(portVolts, portFuel, portTarget),
		component.WithIndexedOutputs(portFuel, 1, cylinders),
		component.WithActivationFunc(component.Sequential(
			component.RequireInputs(portVolts, portFuel, portTarget),
			func(_ context.Context, this *component.Component) error {
				volts := this.InputByName(portVolts).Signals().FirstPayloadOrDefault(0.0)
				fuel := this.InputByName(portFuel).Signals().FirstPayloadOrDefault(0.0)
				target, err := this.InputByName(portTarget).Signals().FirstAs[Target]()
				if err != nil {
					return err
				}
				if volts < minVolts {
					fuel = 0
				}
				return toEachCylinder(this, portFuel, func(i int) any {
					if i == target.Intake {
						return fuel
					}
					return 0.0
				})
			},
		)),
	)
}

// newIgnitionCoil sparks the cylinder on its power stroke.
func newIgnitionCoil() (*component.Component, error) {
	return component.New("ignition-coil",
		component.WithDescription("12 V → spark, to the cylinder on its power stroke"),
		component.WithInputs(portVolts, portTarget),
		component.WithIndexedOutputs(portSpark, 1, cylinders),
		component.WithActivationFunc(component.Sequential(
			component.RequireInputs(portVolts, portTarget),
			func(_ context.Context, this *component.Component) error {
				volts := this.InputByName(portVolts).Signals().FirstPayloadOrDefault(0.0)
				target, err := this.InputByName(portTarget).Signals().FirstAs[Target]()
				if err != nil {
					return err
				}
				return toEachCylinder(this, portSpark, func(i int) any {
					return i == target.Power && volts >= minVolts
				})
			},
		)),
	)
}

// The four strokes, in the order every cylinder goes through them.
const (
	intake = iota
	compression
	power
	exhaust
)

// newCylinder is one cylinder and its piston. It keeps its own place in the
// four-stroke cycle; the crankshaft moves it one stroke per trip around the
// mesh. Fuel taken in on intake burns on the power stroke, if the spark
// comes.
func newCylinder(i int) (*component.Component, error) {
	// Where this cylinder starts, so it makes power on its turn in the firing order.
	offset := (power - slices.Index(firingOrder[:], i) + cylinders) % cylinders

	return component.New(cylinderName(i),
		component.WithDescription("intake → compression → power → exhaust"),
		component.WithInputs(portFuel, portSpark),
		component.WithOutputs(portTorque),
		component.WithInitialState(func(s component.State) {
			s.Set("stroke", offset)
			s.Set("charge", 0.0)
			s.Set("fired", false)
		}),
		component.WithActivationFunc(component.Sequential(
			component.RequireInputs(portFuel, portSpark),
			func(_ context.Context, this *component.Component) error {
				stroke := this.State().GetOrDefault("stroke", 0).(int)
				charge := this.State().GetOrDefault("charge", 0.0).(float64)
				fired := false
				torque := -0.5 // pumping air in and out is never free

				switch stroke {
				case intake:
					charge = this.InputByName(portFuel).Signals().FirstPayloadOrDefault(0.0)
				case compression:
					torque = -1
				case power:
					if this.InputByName(portSpark).Signals().FirstPayloadOrDefault(false) && charge > 0 {
						torque, fired = charge*60, true
					} else {
						torque = 0
					}
					charge = 0
				}

				this.State().Set("stroke", (stroke+1)%cylinders)
				this.State().Set("charge", charge)
				this.State().Set("fired", fired)
				return this.OutputByName(portTorque).PutSignals(signal.New(torque))
			},
		)),
	)
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

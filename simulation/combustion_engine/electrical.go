package main

import (
	"context"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Current: the ECU and the cockpit switch the relays in the fuse box, the
// fuse box tells the battery what that will draw, the battery answers with
// the voltage it can hold under that load, and the fuse box passes it on to
// every circuit that is switched on. A weak battery sags under the starter
// and nothing turns.

const (
	minVolts  = 9.0 // below this nothing electric works
	batteryAh = 4.0 // a small battery, so that a flat one is minutes away, not hours
)

// circuits is every switched circuit in the fuse box, and what it draws when on.
var circuits = map[string]float64{
	"ignition":      9, // the ECU and the ignition coil
	"starter":       140,
	"fuel-pump":     6,
	"cooling-fan":   16,
	"ac-compressor": 4, // the magnetic clutch
	"headlights":    9,
}

// circuitNames lists the circuits in a fixed order.
var circuitNames = []string{"ignition", "starter", "fuel-pump", "cooling-fan", "ac-compressor", "headlights"}

// newFuseBox works in two beats of a stroke. First the switches arrive and
// it tells the battery the load; then the battery's voltage arrives and it
// powers every circuit that is on.
func newFuseBox() (*component.Component, error) {
	return component.New("fuse-box",
		component.WithDescription("relays and fuses: switches → load to the battery, volts → each circuit"),
		component.WithInputs(append(circuitNames, "volts")...),
		component.WithOutputs(append(circuitNames, "load")...),
		component.WithInitialState(func(s component.State) {
			s.Set("on", map[string]bool{})
			s.Set("amps", map[string]float64{})
			s.Set("load", 0.0)
		}),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			if this.InputByName("volts").HasSignals() {
				volts := in[float64](this, "volts")
				on := get(this, "on", map[string]bool{})
				power := map[string]any{}
				for _, name := range circuitNames {
					power[name] = 0.0
					if on[name] {
						power[name] = volts
					}
				}
				return emit(this, power)
			}

			if err := component.RequireInputs(circuitNames...)(ctx, this); err != nil {
				return err
			}
			on, amps, load := map[string]bool{}, map[string]float64{}, 0.0
			for _, name := range circuitNames {
				on[name] = in[bool](this, name)
				if on[name] {
					amps[name] = circuits[name]
					load += circuits[name]
				}
			}
			this.State().Set("on", on)
			this.State().Set("amps", amps)
			this.State().Set("load", load)
			return this.OutputByName("load").PutSignals(signal.New(load))
		}),
	)
}

// newBattery takes what the alternator gives and what the fuse box will
// draw, and answers with the voltage it holds under that load. The emptier
// it is, the more it sags.
func newBattery() (*component.Component, error) {
	return part("battery", "12 V battery: charge in, load out, volts back",
		[]string{"charge", "load"}, nil, []string{"volts"},
		map[string]any{"soc": 0.8, "volts": 12.5, "amps": 0.0},
		func(_ context.Context, this *component.Component) error {
			charge := in[Current](this, "charge")
			load := in[float64](this, "load")
			soc := get(this, "soc", 0.8)

			// The regulator gives what the load needs and what the battery
			// can still soak up, no more.
			supply := min(charge.Amps, load+2+40*(1-soc))
			net := supply - load
			soc = clamp(soc+net*charge.DT/3600/batteryAh, 0, 1)

			volts := 11.9 + 0.8*soc
			if net >= 0 {
				volts = min(14.4, volts+0.05*net)
			} else {
				resistance := 0.015 + 0.03*(1-soc)*(1-soc)*(1-soc)*(1-soc)
				volts = max(0, volts+resistance*net)
			}
			this.State().Set("soc", soc)
			this.State().Set("volts", volts)
			this.State().Set("amps", net)
			return this.OutputByName("volts").PutSignals(signal.New(volts))
		})
}

// newAlternator turns rotation into current, and drags on the belt for it.
func newAlternator() (*component.Component, error) {
	return part("alternator", "belt-driven generator: rotation → charging current",
		[]string{"drive"}, nil, []string{"charge", "drag"},
		map[string]any{"amps": 0.0, "rpm": 0.0},
		func(_ context.Context, this *component.Component) error {
			drive := in[Drive](this, "drive")
			amps := clamp((drive.RPM-1200)/40, 0, 90)
			this.State().Set("amps", amps)
			this.State().Set("rpm", drive.RPM)
			return emit(this, map[string]any{
				"charge": Current{Amps: amps, DT: drive.DT},
				"drag":   -(0.4 + 0.04*amps),
			})
		})
}

// newStarter turns the flywheel while its circuit has power. Like any
// electric motor it pushes hardest from standstill.
func newStarter() (*component.Component, error) {
	return part("starter", "electric motor that turns the engine until it runs on its own",
		[]string{"crank", "power"}, nil, []string{"torque"},
		map[string]any{"engaged": false, "volts": 0.0},
		func(_ context.Context, this *component.Component) error {
			rpm := in[Crank](this, "crank").RPM
			volts := in[float64](this, "power")
			engaged := volts >= minVolts
			this.State().Set("engaged", engaged)
			this.State().Set("volts", volts)
			torque := 0.0
			if engaged {
				torque = max(0, 45*(1-rpm/400))
			}
			return this.OutputByName("torque").PutSignals(signal.New(torque))
		})
}

// newIgnitionCoil sparks the cylinder the ECU names, if there is voltage for it.
func newIgnitionCoil() (*component.Component, error) {
	return part("ignition-coil", "12 V → 30 kV, to the spark plug of the cylinder on its power stroke",
		[]string{"spark", "power"}, nil, indexed("spark"),
		map[string]any{"fired": 0},
		func(_ context.Context, this *component.Component) error {
			target := in[int](this, "spark")
			if in[float64](this, "power") < minVolts {
				target = 0
			}
			this.State().Set("fired", target)
			return toEachCylinder(this, "spark", func(i int) any { return i == target })
		})
}

// newHeadlights are a pure electrical load: switch them on and the
// alternator has to work harder. They dim when the voltage drops.
func newHeadlights() (*component.Component, error) {
	return part("headlights", "two 55 W lamps: a load on the alternator",
		[]string{"power"}, nil, nil,
		map[string]any{"on": false, "brightness": 0.0},
		func(_ context.Context, this *component.Component) error {
			volts := in[float64](this, "power")
			this.State().Set("on", volts > 0)
			this.State().Set("brightness", clamp((volts-minVolts)/4.5, 0, 1))
			return nil
		})
}

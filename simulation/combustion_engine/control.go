package main

import (
	"context"
	"sync"

	"github.com/hovsep/fmesh/component"
)

// The driver, the cockpit, the ECU and the instrument cluster: how the
// outside world gets into the mesh and how the engine is kept running.

const (
	idleRPM         = 800.0
	runningRPM      = 500.0 // the ECU calls it started above this
	stallRPM        = 200.0
	revLimit        = 6500.0
	maxCrankStrokes = 60 // about six seconds of cranking, then the ECU gives up
)

// Driver is everything outside the engine bay: a foot on the pedal and a
// hand on the switches. The UI (or a test) changes it whenever it likes; the
// cockpit component reads it once per stroke. It is the one place where the
// world gets into the mesh, so it is the one place that needs a lock.
type Driver struct {
	mu       sync.Mutex
	pedal    float64
	ignition bool
	ac       bool
	lights   bool
	refuel   bool
}

// DriverInput is what the driver is doing right now.
type DriverInput struct {
	Pedal    float64 `json:"pedal"`
	Ignition bool    `json:"ignition"`
	AC       bool    `json:"ac"`
	Lights   bool    `json:"lights"`
	Refuel   bool    `json:"refuel"`
}

// Set changes what the driver does.
func (d *Driver) Set(change func(in *DriverInput)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	in := DriverInput{Pedal: d.pedal, Ignition: d.ignition, AC: d.ac, Lights: d.lights, Refuel: d.refuel}
	change(&in)
	d.pedal, d.ignition, d.ac, d.lights, d.refuel = clamp(in.Pedal, 0, 1), in.Ignition, in.AC, in.Lights, in.Refuel
}

// peek is what the driver is doing, for the page.
func (d *Driver) peek() DriverInput {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DriverInput{Pedal: d.pedal, Ignition: d.ignition, AC: d.ac, Lights: d.lights, Refuel: d.refuel}
}

// read is what the cockpit sees. A refuel request is seen once.
func (d *Driver) read() DriverInput {
	d.mu.Lock()
	defer d.mu.Unlock()
	in := DriverInput{Pedal: d.pedal, Ignition: d.ignition, AC: d.ac, Lights: d.lights, Refuel: d.refuel}
	d.refuel = false
	return in
}

// newCockpit reads the pedal and the switches once per stroke and sends
// them where they go: the pedal, the ignition and the AC button to the ECU,
// the ignition and light switches to the fuse box, a refuel to the tank.
func newCockpit(driver *Driver) (*component.Component, error) {
	return part("cockpit", "accelerator pedal, ignition, AC and light switches",
		[]string{"crank"}, nil, []string{"controls", "ignition", "lights", "refuel"},
		map[string]any{"driver": DriverInput{}},
		func(_ context.Context, this *component.Component) error {
			d := driver.read()
			this.State().Set("driver", d)
			out := map[string]any{
				"controls": Controls{Pedal: d.Pedal, Ignition: d.Ignition, AC: d.AC},
				"ignition": d.Ignition,
				"lights":   d.Lights,
			}
			if d.Refuel {
				out["refuel"] = true
			}
			return emit(this, out)
		})
}

// ecuSensors are the readings the ECU listens to. They come from the end of
// the previous stroke, as in a real engine, where every decision is made on
// the last measurement.
var ecuSensors = []string{"map", "lambda", "coolant", "oil", "volts"}

// newECU is the engine's computer. Every stroke it reads the crankshaft and
// the cockpit and decides: crank or not, how far to open the throttle, how
// much fuel and into which cylinder, which cylinder to spark, whether the
// fan and the AC clutch run.
func newECU() (*component.Component, error) {
	return part("ecu", "engine control unit: starting, idle, fuel, spark, fan and AC",
		[]string{"crank", "controls"}, ecuSensors,
		[]string{"starter", "throttle", "fuel-pump", "demand", "inject", "spark", "cooling-fan", "ac-compressor", "status"},
		map[string]any{
			"mode": "off", "started": false, "cranked": 0,
			"idle": 0.03, "trim": 1.0, "fan": false, "lambdaTarget": 1.0,
			// What the sensors say before they have said anything.
			"map": 1.0, "lambda": LambdaReading{}, "coolant": ambient, "oil": 0.0, "volts": 12.5,
		},
		func(_ context.Context, this *component.Component) error {
			crank := in[Crank](this, "crank")
			ctl := in[Controls](this, "controls")
			coolant := get(this, "coolant", ambient)
			rpm := crank.RPM

			// Starting, running, stopping.
			mode, started, cranked := get(this, "mode", "off"), get(this, "started", false), get(this, "cranked", 0)
			if rpm == 0 {
				cranked = 0 // the key was just turned
			}
			engage := false
			switch {
			case !ctl.Ignition:
				started, mode = false, "off"
			case started && rpm < stallRPM:
				started, mode, cranked = false, "stalled", maxCrankStrokes
			case started:
				mode = "running"
			case rpm >= runningRPM:
				started, mode = true, "running"
			case cranked < maxCrankStrokes:
				cranked++
				engage, mode = true, "cranking"
			case mode == "cranking":
				mode = "no start"
			}
			firing := ctl.Ignition && (started || engage)
			fuelCut := false
			if started && rpm > revLimit {
				fuelCut, mode = true, "rev limit"
			}
			if started && ctl.Pedal < 0.02 && rpm > 1800 {
				fuelCut = true // coasting down: no fuel until it nears idle
			}
			if started && coolant > overheatAt {
				fuelCut, mode = crank.Stroke%2 == 0, "overheat"
			}

			// Idle: nudge the throttle until the engine turns at idle speed.
			target := idleRPM
			if coolant < 50 {
				target += 200 // fast idle while cold
			}
			if ctl.AC {
				target += 100
			}
			idle := get(this, "idle", 0.03)
			if started && ctl.Pedal < 0.02 {
				idle = clamp(idle+0.000004*(target-rpm), 0, 0.15)
			}
			throttle := idle + (1-idle)*ctl.Pedal*ctl.Pedal

			// Fuel: as much as the air the manifold pressure promises, aimed at
			// lambda 1 and corrected by what the oxygen sensor saw.
			lambdaTarget := 1.0
			switch {
			case !started:
				lambdaTarget = 0.8
			case ctl.Pedal > 0.85:
				lambdaTarget = 0.88 // full power runs rich
			case coolant < 50:
				lambdaTarget = 0.92
			}
			trim := get(this, "trim", 1.0)
			if seen := get(this, "lambda", LambdaReading{}); started && seen.Ready && lambdaTarget == 1 {
				trim = clamp(trim+0.03*(seen.Lambda-1), 0.75, 1.25)
			}
			amount := 0.0
			if firing && !fuelCut {
				amount = get(this, "map", 1.0) * ve(rpm) / lambdaTarget * trim
			}
			spark := 0
			if firing {
				spark = onStroke(power, crank.Stroke)
			}

			// The fan, with a little hysteresis, and always with the AC on.
			fan := get(this, "fan", false)
			switch {
			case coolant > fanOnAt:
				fan = true
			case coolant < fanOffAt:
				fan = false
			}
			clutch := ctl.AC && started && rpm > runningRPM && coolant < overheatAt-3

			for k, v := range map[string]any{
				"mode": mode, "started": started, "cranked": cranked, "idle": idle,
				"trim": trim, "fan": fan, "lambdaTarget": lambdaTarget, "throttle": throttle,
			} {
				this.State().Set(k, v)
			}
			return emit(this, map[string]any{
				// Relays in the fuse box.
				"starter":       engage,
				"fuel-pump":     firing,
				"cooling-fan":   ctl.Ignition && (fan || clutch),
				"ac-compressor": clutch,
				// Orders to the parts.
				"throttle": throttle,
				"demand":   amount,
				"inject":   Injection{Cylinder: onStroke(intake, crank.Stroke), Amount: amount},
				"spark":    spark,
				"status":   Status{RPM: rpm, Mode: mode, CheckEngine: mode == "no start" || mode == "overheat" || mode == "stalled"},
			})
		})
}

// gauges is everything the instrument cluster shows.
var gauges = []string{"status", "coolant", "oil", "volts", "fuel", "lambda", "map"}

// newInstrumentCluster is where the mesh's readings end up: it listens to
// the sensors and keeps the latest of each, and the UI draws its gauges from
// that.
func newInstrumentCluster() (*component.Component, error) {
	return part("instrument-cluster", "tachometer, gauges and warning lamps",
		nil, gauges, nil,
		map[string]any{
			"status": Status{Mode: "off"}, "coolant": ambient, "oil": 0.0, "volts": 12.5,
			"fuel": 0.0, "lambda": LambdaReading{}, "map": 1.0,
		},
		func(context.Context, *component.Component) error { return nil })
}

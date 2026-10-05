package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// A four-cylinder, four-stroke petrol engine, simplified until it fits on one
// screen but not further: fuel flows from the tank through the pump into the
// cylinders, current flows from the battery to the starter, the pump and the
// ignition coil (and back from the alternator), and the crankshaft's rotation
// drives the next stroke of every cylinder.
//
// One trip around the mesh is one stroke: half a turn of the crankshaft.
//
//	rotation: crankshaft → ecu, alternator          (and every cylinder → crankshaft)
//	current:  alternator → battery → starter, fuel-pump, ignition-coil
//	fuel:     ecu → fuel-tank → fuel-pump → cylinder on intake
//	spark:    ecu → ignition-coil → cylinder on power
//
// Nobody tells the mesh when to stop: the engine runs until the tank is dry,
// the crankshaft stops turning, and with it the whole mesh.
//
// Run: go run .

const cylinders = 4

// Port names.
const (
	portKey     = "key"     // the ignition key: the one signal that starts it all
	portCrank   = "crank"   // crankshaft position and speed
	portTorque  = "torque"  // what a cylinder or the starter puts on the crankshaft
	portStarter = "starter" // the crankshaft's input from the starter motor
	portVolts   = "volts"   // battery voltage, to every electric consumer
	portCharge  = "charge"  // alternator → battery
	portLoad    = "load"    // ecu → battery: the current drawn this stroke
	portEngage  = "engage"  // ecu → starter
	portDemand  = "demand"  // ecu → fuel-tank: how much fuel this stroke
	portFuel    = "fuel"    // fuel on its way to a cylinder
	portTarget  = "target"  // which cylinder the pump or the coil serves this stroke
	portSpark   = "spark"   // coil → cylinder
)

// Crank is what the crankshaft announces at the start of every stroke.
type Crank struct {
	Stroke int     // strokes since the key was turned
	RPM    float64 // crankshaft speed
}

// The firing order of an inline four: cylinder firingOrder[n%4] makes power
// on stroke n. Spreading the power strokes evenly is what keeps it smooth.
var firingOrder = [cylinders]int{1, 3, 4, 2}

// The engine's numbers. Units are made up but consistent.
const (
	idleRPM          = 750.0
	startedRPM       = 400.0 // the starter lets go above this
	fuelIdle         = 0.35  // fuel per intake stroke at idle
	fuelFullThrottle = 1.0   // and with the pedal down
	tankCapacity     = 160.0
	batteryCapacity  = 100.0
	minVolts         = 9.0 // below this nothing electric works
)

func main() {
	charge := flag.Float64("battery", 80, "battery charge at the start, in percent (try 5)")
	flag.Parse()

	fmt.Println("=== Internal Combustion Engine ===")
	fmt.Println("Fuel, current and rotation, each a flow through the mesh; one trip around it is one stroke.")
	fmt.Println()

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	handled, err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	// A dashboard: read the gauges every few strokes, often while it starts.
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			r := cc.Cycle.ActivationResults().ByName("crankshaft")
			if r == nil || component.IsWaitingForInput(r) {
				return nil
			}
			stroke := state[Crank](fm, "crankshaft", "crank").Stroke
			if (stroke <= 12 && stroke%2 == 0) || stroke%20 == 0 {
				fmt.Println(dashboard(fm))
			}
			return nil
		})
	})

	fm.ComponentByName("battery").State().Set("level", batteryCapacity**charge/100)

	fmt.Println("Turning the key...")
	if err := fm.ComponentByName("crankshaft").InputByName(portKey).PutSignals(signal.New("on")); err != nil {
		fmt.Println("Failed to turn the key:", err)
		os.Exit(1)
	}

	info, err := fm.Run(context.Background())
	if err != nil {
		fmt.Println("Engine failed:", err)
		os.Exit(1)
	}

	fmt.Println(dashboard(fm))
	fmt.Println()
	strokes := state[Crank](fm, "crankshaft", "crank").Stroke
	if state[float64](fm, "fuel-tank", "level") > 0 {
		fmt.Printf("The engine never caught (%d strokes): the battery is too weak to turn the starter.\n", strokes)
		return
	}
	fmt.Printf("The engine stopped after %d strokes (%d mesh cycles): the tank is dry.\n", strokes, info.Cycles.Len())
}

// dashboard renders the gauges from the components' state.
func dashboard(fm *fmesh.FMesh) string {
	crank := state[Crank](fm, "crankshaft", "crank")
	return fmt.Sprintf("  stroke %4d │ %5.0f rpm │ throttle %3.0f%% │ battery %4.1f V │ fuel %5.1f │ starter %s",
		crank.Stroke,
		crank.RPM,
		state[float64](fm, "ecu", "throttle")*100,
		state[float64](fm, "battery", "volts"),
		state[float64](fm, "fuel-tank", "level"),
		onOff(state[bool](fm, "starter", "engaged")),
	)
}

func state[T any](fm *fmesh.FMesh, componentName, key string) T {
	v, _ := fm.ComponentByName(componentName).State().GetTyped[T](key)
	return v
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func cylinderName(i int) string { return fmt.Sprintf("cylinder-%d", i) }

// getMesh assembles the engine.
func getMesh() (*fmesh.FMesh, error) {
	fm, err := fmesh.New("combustion engine",
		fmesh.WithDescription("a four-cylinder four-stroke engine: fuel, current and rotation as flows, one trip around the mesh per stroke"),
		// A few hundred strokes, several cycles each: well past the default 1000.
		fmesh.WithCyclesLimit(20000),
	)
	if err != nil {
		return nil, err
	}

	parts := []func() (*component.Component, error){
		newCrankshaft, newECU, newAlternator, newBattery, newStarter,
		newFuelTank, newFuelPump, newIgnitionCoil,
	}
	for i := 1; i <= cylinders; i++ {
		parts = append(parts, func() (*component.Component, error) { return newCylinder(i) })
	}
	for _, newPart := range parts {
		c, err := newPart()
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(c); err != nil {
			return nil, err
		}
	}

	return fm, wire(fm)
}

// wire draws the pipes: every flow of the engine, read as from → to.
func wire(fm *fmesh.FMesh) error {
	out := func(c, p string) *port.Port { return fm.ComponentByName(c).OutputByName(p) }
	in := func(c, p string) *port.Port { return fm.ComponentByName(c).InputByName(p) }

	pipes := []port.Pipe{
		// Rotation: the crankshaft tells the ECU where it is and spins the alternator.
		{From: out("crankshaft", portCrank), To: in("ecu", portCrank)},
		{From: out("crankshaft", portCrank), To: in("alternator", portCrank)},

		// Current: alternator → battery → every electric consumer.
		{From: out("alternator", portCharge), To: in("battery", portCharge)},
		{From: out("ecu", portLoad), To: in("battery", portLoad)},
		{From: out("battery", portVolts), To: in("starter", portVolts)},
		{From: out("battery", portVolts), To: in("fuel-pump", portVolts)},
		{From: out("battery", portVolts), To: in("ignition-coil", portVolts)},

		// Control: the ECU decides, the parts act.
		{From: out("ecu", portEngage), To: in("starter", portEngage)},
		{From: out("ecu", portDemand), To: in("fuel-tank", portDemand)},
		{From: out("ecu", portTarget), To: in("fuel-pump", portTarget)},
		{From: out("ecu", portTarget), To: in("ignition-coil", portTarget)},

		// Fuel: tank → pump → the cylinder on its intake stroke.
		{From: out("fuel-tank", portFuel), To: in("fuel-pump", portFuel)},

		// Torque: everything that turns the crankshaft.
		{From: out("starter", portTorque), To: in("crankshaft", portStarter)},
	}
	for i := 1; i <= cylinders; i++ {
		pipes = append(pipes,
			port.Pipe{From: out("fuel-pump", fmt.Sprint(portFuel, i)), To: in(cylinderName(i), portFuel)},
			port.Pipe{From: out("ignition-coil", fmt.Sprint(portSpark, i)), To: in(cylinderName(i), portSpark)},
			port.Pipe{From: out(cylinderName(i), portTorque), To: in("crankshaft", cylinderName(i))},
		)
	}
	return port.MultiPipe(pipes...)
}

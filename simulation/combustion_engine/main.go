package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// A four-cylinder petrol engine, part by part: 41 components, from the
// valves and the injectors to the fan, the fuse box and the headlights, each
// keeping its own physics, wired the way the real parts are.
//
// One trip around the mesh is one stroke, half a turn of the crankshaft, and
// nobody drives it from outside: the crankshaft's turn is what starts the
// next stroke of every part. The driver only touches the pedal and the
// switches, which the cockpit reads once per stroke; the engine runs until the
// key is turned off, the tank is dry or it stalls, and then the mesh stops by
// itself.
//
// The engine runs in the browser: go run . and open http://localhost:8080.

func main() {
	addr := flag.String("addr", "localhost:8080", "where to serve the engine's page")
	battery := flag.Float64("battery", 80, "battery charge at the start, in percent (try 5)")
	fuel := flag.Float64("fuel", 10, "fuel in the tank at the start, in liters")
	coolant := flag.Float64("coolant", ambient, "coolant temperature at the start, in °C")
	flag.Parse()

	driver := &Driver{}
	fm, err := getMesh(driver)
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

	fm.ComponentByName("battery").State().Set("soc", clamp(*battery/100, 0, 1))
	fm.ComponentByName("fuel-tank").State().Set("liters", clamp(*fuel, 0, tankLiters))
	fm.ComponentByName("engine-block").State().Set("temp", *coolant)

	fmt.Printf("=== Internal Combustion Engine: %d parts ===\n", fm.Components().Len())
	fmt.Printf("Open http://%s in a browser. Ctrl+C to quit.\n", *addr)
	if err := serve(*addr, fm, driver); err != nil {
		fmt.Println("Server failed:", err)
		os.Exit(1)
	}
}

// getMesh assembles the engine.
func getMesh(driver *Driver) (*fmesh.FMesh, error) {
	fm, err := fmesh.New("combustion engine",
		fmesh.WithDescription("a four-cylinder engine, part by part; one trip around the mesh per stroke"),
		// The engine runs for as long as it runs; only keep the latest cycles.
		fmesh.WithUnlimitedCycles(),
		fmesh.WithUnlimitedTime(),
		fmesh.WithCyclesHistoryLimit(100),
	)
	if err != nil {
		return nil, err
	}

	parts := []func() (*component.Component, error){
		newCrankshaft, newCamshaft, newAccessoryBelt,
		newStarter, newAlternator, newBattery, newFuseBox, newIgnitionCoil, newHeadlights,
		newFuelTank, newFuelPump, newFuelRail,
		newAirFilter, newThrottleBody, newIntakeManifold, newExhaustManifold, newLambdaSensor,
		newOilPump, newWaterPump, newEngineBlock, newThermostat, newRadiator, newCoolingFan, newACCompressor,
		newECU, newInstrumentCluster,
		func() (*component.Component, error) { return newCockpit(driver) },
		func() (*component.Component, error) {
			return newSensor("coolant-temp-sensor", "thermistor in the water jacket")
		},
		func() (*component.Component, error) {
			return newSensor("oil-pressure-sensor", "oil pressure sender on the main gallery")
		},
	}
	for i := 1; i <= cylinders; i++ {
		parts = append(parts,
			func() (*component.Component, error) { return newCylinder(i) },
			func() (*component.Component, error) { return newValve("intake", i, "air") },
			func() (*component.Component, error) { return newValve("exhaust", i, "exhaust") },
		)
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

// flows is every pipe in the engine, read as "component.port → component.port".
func flows() [][2]string {
	f := [][2]string{
		// Rotation: the crankshaft's turn starts the next stroke of everything it drives.
		{"crankshaft.crank", "cockpit.crank"},
		{"crankshaft.crank", "ecu.crank"},
		{"crankshaft.crank", "camshaft.crank"},
		{"crankshaft.crank", "accessory-belt.crank"},
		{"crankshaft.crank", "oil-pump.crank"},
		{"crankshaft.crank", "starter.crank"},
		{"crankshaft.crank", "air-filter.crank"},
		{"crankshaft.crank", "intake-manifold.crank"},
		{"accessory-belt.drive-alternator", "alternator.drive"},
		{"accessory-belt.drive-water-pump", "water-pump.drive"},
		{"accessory-belt.drive-ac-compressor", "ac-compressor.drive"},

		// The driver: the cockpit reads the pedal and the switches.
		{"cockpit.controls", "ecu.controls"},
		{"cockpit.ignition", "fuse-box.ignition"},
		{"cockpit.lights", "fuse-box.headlights"},
		{"cockpit.refuel", "fuel-tank.refuel"},

		// Control: the ECU decides, the parts act.
		{"ecu.throttle", "throttle-body.throttle"},
		{"ecu.demand", "fuel-tank.demand"},
		{"ecu.inject", "fuel-rail.inject"},
		{"ecu.spark", "ignition-coil.spark"},

		// Torque: everything that pushes or drags the crankshaft.
		{"starter.torque", "crankshaft.starter"},
		{"oil-pump.torque", "crankshaft.oil-pump"},
		{"alternator.drag", "accessory-belt.drag-alternator"},
		{"water-pump.drag", "accessory-belt.drag-water-pump"},
		{"ac-compressor.drag", "accessory-belt.drag-ac-compressor"},
		{"accessory-belt.torque", "crankshaft.belt"},

		// Current: the ECU switches the relays, the fuse box tells the battery
		// the load, the battery the volts, and the fuse box powers each circuit.
		{"alternator.charge", "battery.charge"},
		{"fuse-box.load", "battery.load"},
		{"battery.volts", "fuse-box.volts"},
		{"fuse-box.starter", "starter.power"},
		{"fuse-box.fuel-pump", "fuel-pump.power"},
		{"fuse-box.ignition", "ignition-coil.power"},
		{"fuse-box.cooling-fan", "cooling-fan.power"},
		{"fuse-box.ac-compressor", "ac-compressor.power"},
		{"fuse-box.headlights", "headlights.power"},

		// Fuel: tank → pump → rail → the injector of the cylinder on intake.
		{"fuel-tank.fuel", "fuel-pump.fuel"},
		{"fuel-pump.fuel", "fuel-rail.fuel"},

		// Air: filter → throttle → manifold → the open intake valve.
		{"air-filter.air", "throttle-body.air"},
		{"throttle-body.air", "intake-manifold.air"},

		// Exhaust: the open exhaust valves → manifold → oxygen sensor.
		{"exhaust-manifold.exhaust", "lambda-sensor.exhaust"},

		// Cooling: water jacket → thermostat → radiator, and the heat it took back.
		{"water-pump.flow", "engine-block.flow"},
		{"engine-block.coolant", "thermostat.coolant"},
		{"thermostat.coolant", "radiator.coolant"},
		{"radiator.cooling", "engine-block.cooling"},
		{"cooling-fan.airflow", "radiator.airflow"},
		{"ac-compressor.condenser", "radiator.condenser"},

		// Oil: pump → bearings, and its pressure sender.
		{"oil-pump.pressure", "crankshaft.oil"},
		{"oil-pump.pressure", "oil-pressure-sensor.in"},
		{"engine-block.temp", "coolant-temp-sensor.in"},

		// Sensors → the ECU and the instrument cluster.
		{"intake-manifold.map", "ecu.map"},
		{"lambda-sensor.reading", "ecu.lambda"},
		{"coolant-temp-sensor.reading", "ecu.coolant"},
		{"oil-pressure-sensor.reading", "ecu.oil"},
		{"battery.volts", "ecu.volts"},
		{"ecu.status", "instrument-cluster.status"},
		{"coolant-temp-sensor.reading", "instrument-cluster.coolant"},
		{"oil-pressure-sensor.reading", "instrument-cluster.oil"},
		{"battery.volts", "instrument-cluster.volts"},
		{"fuel-tank.level", "instrument-cluster.fuel"},
		{"lambda-sensor.reading", "instrument-cluster.lambda"},
		{"intake-manifold.map", "instrument-cluster.map"},
	}
	for _, relay := range []string{"starter", "fuel-pump", "cooling-fan", "ac-compressor"} {
		f = append(f, [2]string{"ecu." + relay, "fuse-box." + relay})
	}
	for i := 1; i <= cylinders; i++ {
		cyl := cylinderName(i)
		f = append(f,
			[2]string{fmt.Sprint("fuel-rail.injector", i), cyl + ".fuel"},
			[2]string{"intake-manifold.charge", fmt.Sprint("intake-valve-", i, ".air")},
			[2]string{fmt.Sprint("camshaft.intake", i), fmt.Sprint("intake-valve-", i, ".lift")},
			[2]string{fmt.Sprint("intake-valve-", i, ".air"), cyl + ".air"},
			[2]string{fmt.Sprint("ignition-coil.spark", i), cyl + ".spark"},
			[2]string{cyl + ".torque", "crankshaft." + cyl},
			[2]string{cyl + ".heat", "engine-block." + cyl},
			[2]string{cyl + ".exhaust", fmt.Sprint("exhaust-valve-", i, ".exhaust")},
			[2]string{fmt.Sprint("camshaft.exhaust", i), fmt.Sprint("exhaust-valve-", i, ".lift")},
			[2]string{fmt.Sprint("exhaust-valve-", i, ".exhaust"), "exhaust-manifold." + cyl},
		)
	}
	return f
}

// wire draws the pipes.
func wire(fm *fmesh.FMesh) error {
	find := func(ref string, output bool) (*port.Port, error) {
		dot := strings.LastIndex(ref, ".")
		c := fm.ComponentByName(ref[:dot])
		if c == nil {
			return nil, fmt.Errorf("no component %q", ref[:dot])
		}
		p := c.InputByName(ref[dot+1:])
		if output {
			p = c.OutputByName(ref[dot+1:])
		}
		if p == nil {
			return nil, fmt.Errorf("no port %q", ref)
		}
		return p, nil
	}

	pipes := make([]port.Pipe, 0, len(flows()))
	for _, f := range flows() {
		from, err := find(f[0], true)
		if err != nil {
			return err
		}
		to, err := find(f[1], false)
		if err != nil {
			return err
		}
		pipes = append(pipes, port.Pipe{From: from, To: to})
	}
	return port.MultiPipe(pipes...)
}

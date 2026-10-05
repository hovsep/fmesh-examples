package main

import (
	"context"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// What goes in and what comes out: fuel from the tank, air through the
// filter and the throttle, and exhaust out past the oxygen sensor.

const (
	tankLiters       = 45.0
	litersPerCharge  = 0.00006 // fuel for one full cylinder at lambda 1
	fuelPressure     = 3.5     // bar, what the pump makes on a healthy battery
	minFuelPressure  = 2.0     // below this the injectors only dribble
	exhaustAmbient   = 20.0
	lambdaSensorTemp = 250.0 // °C: below this the oxygen sensor reads nothing
)

// newFuelTank hands out what the ECU asks for, while it lasts.
func newFuelTank() (*component.Component, error) {
	return part("fuel-tank", "holds the fuel; gives what is asked, while it lasts",
		[]string{"demand"}, []string{"refuel"}, []string{"fuel", "level"},
		map[string]any{"liters": tankLiters, "capacity": tankLiters, "refuel": false},
		func(_ context.Context, this *component.Component) error {
			liters := get(this, "liters", 0.0)
			if get(this, "refuel", false) {
				liters = tankLiters
				this.State().Set("refuel", false)
			}
			given := min(in[float64](this, "demand"), liters/litersPerCharge)
			liters = max(0, liters-given*litersPerCharge)
			this.State().Set("liters", liters)
			return emit(this, map[string]any{"fuel": given, "level": liters / tankLiters})
		})
}

// newFuelPump pushes fuel from the tank into the rail. It is electric:
// without power on its circuit there is no pressure, and no fuel.
func newFuelPump() (*component.Component, error) {
	return part("fuel-pump", "electric pump in the tank: tank → fuel rail at 3.5 bar",
		[]string{"power", "fuel"}, nil, []string{"fuel"},
		map[string]any{"pressure": 0.0, "on": false},
		func(_ context.Context, this *component.Component) error {
			volts := in[float64](this, "power")
			on := volts >= minVolts
			pressure := get(this, "pressure", 0.0)
			target := 0.0
			if on {
				target = fuelPressure * min(1, volts/12)
			}
			pressure += (target - pressure) * 0.5
			this.State().Set("pressure", pressure)
			this.State().Set("on", on)

			fuel := in[float64](this, "fuel")
			if pressure < minFuelPressure {
				fuel = 0
			}
			return this.OutputByName("fuel").PutSignals(signal.New(Fuel{Amount: fuel, Pressure: pressure}))
		})
}

// newFuelRail holds the pressurised fuel and opens one injector per stroke:
// the one at the intake port of the cylinder the ECU names.
func newFuelRail() (*component.Component, error) {
	return part("fuel-rail", "fuel rail with four injectors, one at each intake port",
		[]string{"fuel", "inject"}, nil, indexed("injector"),
		map[string]any{"pressure": 0.0, "injecting": 0, "amount": 0.0},
		func(_ context.Context, this *component.Component) error {
			fuel := in[Fuel](this, "fuel")
			order := in[Injection](this, "inject")
			amount := min(fuel.Amount, order.Amount)
			if amount == 0 {
				order.Cylinder = 0
			}
			this.State().Set("pressure", fuel.Pressure)
			this.State().Set("injecting", order.Cylinder)
			this.State().Set("amount", amount)
			return toEachCylinder(this, "injector", func(i int) any {
				if i == order.Cylinder {
					return amount
				}
				return 0.0
			})
		})
}

// newAirFilter lets the engine breathe, a little less as it clogs.
func newAirFilter() (*component.Component, error) {
	return part("air-filter", "paper filter in the air box",
		[]string{"crank"}, nil, []string{"air"},
		map[string]any{"density": 0.97},
		func(_ context.Context, this *component.Component) error {
			return this.OutputByName("air").PutSignals(signal.New(get(this, "density", 1.0)))
		})
}

// newThrottleBody is the electronic throttle: a motor turns the plate
// towards whatever opening the ECU asks for.
func newThrottleBody() (*component.Component, error) {
	return part("throttle-body", "drive-by-wire throttle plate",
		[]string{"air", "throttle"}, nil, []string{"air"},
		map[string]any{"opening": 0.0},
		func(_ context.Context, this *component.Component) error {
			opening := get(this, "opening", 0.0)
			opening += (in[float64](this, "throttle") - opening) * 0.6
			this.State().Set("opening", opening)
			return this.OutputByName("air").PutSignals(signal.New(Air{Opening: opening, Density: in[float64](this, "air")}))
		})
}

// newIntakeManifold is where the pistons pull against the throttle: the
// faster they pull and the less the throttle lets through, the lower the
// pressure, and the less air each cylinder gets. Its pressure sensor (MAP)
// is how the ECU knows how much fuel to add.
func newIntakeManifold() (*component.Component, error) {
	return part("intake-manifold", "plenum and runners to the four intake valves, with a pressure sensor",
		[]string{"crank", "air"}, nil, []string{"charge", "map"},
		map[string]any{"map": 1.0},
		func(_ context.Context, this *component.Component) error {
			rpm := in[Crank](this, "crank").RPM
			air := in[Air](this, "air")
			pressure := air.Density * min(1, (0.02+air.Opening)*5.4/(rpm/1000+0.1))
			this.State().Set("map", pressure)
			return emit(this, map[string]any{"charge": pressure * ve(rpm), "map": pressure})
		})
}

// newExhaustManifold collects the four exhaust ports into one pipe, and
// glows when the engine works hard.
func newExhaustManifold() (*component.Component, error) {
	ports := make([]string, 0, cylinders)
	for i := 1; i <= cylinders; i++ {
		ports = append(ports, cylinderName(i))
	}
	return part("exhaust-manifold", "four exhaust ports into one pipe",
		ports, nil, []string{"exhaust"},
		map[string]any{"temp": exhaustAmbient, "gas": 0.0},
		func(_ context.Context, this *component.Component) error {
			temp := get(this, "temp", exhaustAmbient)
			out := Exhaust{}
			for _, name := range ports {
				gas := in[Exhaust](this, name)
				if gas.Gas > 0 {
					out = gas
					temp += (gas.Temp - temp) * 0.03
				}
			}
			temp -= (temp - exhaustAmbient) * 0.002
			out.Temp = temp
			this.State().Set("temp", temp)
			this.State().Set("gas", out.Gas)
			return this.OutputByName("exhaust").PutSignals(signal.New(out))
		})
}

// newLambdaSensor reads the oxygen left in the exhaust, once it is hot
// enough to read anything.
func newLambdaSensor() (*component.Component, error) {
	return part("lambda-sensor", "oxygen sensor: how rich or lean the engine burns",
		[]string{"exhaust"}, nil, []string{"reading"},
		map[string]any{"lambda": 0.0, "ready": false},
		func(_ context.Context, this *component.Component) error {
			gas := in[Exhaust](this, "exhaust")
			reading := LambdaReading{Lambda: get(this, "lambda", 0.0), Ready: gas.Temp >= lambdaSensorTemp}
			if gas.Gas > 0 {
				reading.Lambda = math.Round(gas.Lambda*1000) / 1000
			}
			this.State().Set("lambda", reading.Lambda)
			this.State().Set("ready", reading.Ready)
			return this.OutputByName("reading").PutSignals(signal.New(reading))
		})
}

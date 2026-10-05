package main

import (
	"context"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Heat and oil: the water jacket takes the cylinders' heat, the thermostat
// sends it to the radiator once the engine is warm, the fan helps when the
// radiator cannot keep up; the oil pump keeps the bearings floating.

const (
	ambient      = 20.0 // °C
	jacketHeat   = 4.0  // kJ per °C: water and iron, kept small so it warms up in minutes
	thermostatAt = 86.0 // °C, starts to open
	fanOnAt      = 98.0 // °C, the ECU switches the fan on
	fanOffAt     = 93.0
	overheatAt   = 115.0 // °C, the ECU cuts power to save the engine
)

// newWaterPump circulates coolant, as fast as the belt turns it.
func newWaterPump() (*component.Component, error) {
	return part("water-pump", "belt-driven coolant pump",
		[]string{"drive"}, nil, []string{"flow", "drag"},
		map[string]any{"flow": 0.0, "rpm": 0.0},
		func(_ context.Context, this *component.Component) error {
			drive := in[Drive](this, "drive")
			flow := drive.RPM / 3000
			this.State().Set("flow", flow)
			this.State().Set("rpm", drive.RPM)
			return emit(this, map[string]any{
				"flow": Coolant{Flow: flow, DT: drive.DT},
				"drag": -(0.2 + 0.6*flow),
			})
		})
}

// newEngineBlock is the water jacket around the cylinders: heat in from
// every power stroke, heat out to the radiator and to the air around it.
func newEngineBlock() (*component.Component, error) {
	required := []string{"flow"}
	for i := 1; i <= cylinders; i++ {
		required = append(required, cylinderName(i))
	}
	return part("engine-block", "cylinder block and head with their water jacket",
		required, []string{"cooling"}, []string{"coolant", "temp"},
		map[string]any{"temp": ambient, "cooling": 0.0},
		func(_ context.Context, this *component.Component) error {
			flow := in[Coolant](this, "flow")
			temp := get(this, "temp", ambient)
			heat := 0.0
			for i := 1; i <= cylinders; i++ {
				heat += in[float64](this, cylinderName(i))
			}
			heat -= get(this, "cooling", 0.0) + 0.03*(temp-ambient)*flow.DT
			this.State().Set("cooling", 0.0)
			temp += heat / jacketHeat
			this.State().Set("temp", temp)
			return emit(this, map[string]any{
				"coolant": Coolant{Temp: temp, Flow: flow.Flow, DT: flow.DT},
				"temp":    temp,
			})
		})
}

// newThermostat keeps a cold engine's coolant away from the radiator, and
// opens as it warms up.
func newThermostat() (*component.Component, error) {
	return part("thermostat", "wax valve between the engine and the radiator",
		[]string{"coolant"}, nil, []string{"coolant"},
		map[string]any{"opening": 0.0},
		func(_ context.Context, this *component.Component) error {
			coolant := in[Coolant](this, "coolant")
			opening := clamp((coolant.Temp-thermostatAt)/10, 0, 1)
			this.State().Set("opening", opening)
			coolant.Flow *= opening
			return this.OutputByName("coolant").PutSignals(signal.New(coolant))
		})
}

// newRadiator gives the coolant's heat to the air: more with more coolant
// through it, much more with the fan blowing, less when the AC condenser in
// front of it heats that air first.
func newRadiator() (*component.Component, error) {
	return part("radiator", "coolant → air heat exchanger, with the AC condenser in front",
		[]string{"coolant"}, []string{"airflow", "condenser"}, []string{"cooling"},
		map[string]any{"airflow": 0.0, "condenser": 0.0, "kw": 0.0, "outlet": ambient},
		func(_ context.Context, this *component.Component) error {
			coolant := in[Coolant](this, "coolant")
			air := ambient + 8*get(this, "condenser", 0.0)
			kw := max(0, 1.1*(coolant.Temp-air)*(0.2+0.9*get(this, "airflow", 0.0))*min(1.2, coolant.Flow))
			this.State().Set("kw", kw)
			this.State().Set("outlet", coolant.Temp-kw/max(coolant.Flow*20, 1))
			return this.OutputByName("cooling").PutSignals(signal.New(kw * coolant.DT))
		})
}

// newCoolingFan blows through the radiator while its circuit has power.
func newCoolingFan() (*component.Component, error) {
	return part("cooling-fan", "electric fan behind the radiator",
		[]string{"power"}, nil, []string{"airflow"},
		map[string]any{"speed": 0.0, "on": false},
		func(_ context.Context, this *component.Component) error {
			volts := in[float64](this, "power")
			on := volts >= minVolts
			target := 0.0
			if on {
				target = min(1, volts/13.5)
			}
			speed := get(this, "speed", 0.0)
			speed += (target - speed) * 0.05
			this.State().Set("speed", speed)
			this.State().Set("on", on)
			return this.OutputByName("airflow").PutSignals(signal.New(speed))
		})
}

// newSensor is a sensor that reports what it measures, as it is.
func newSensor(name, description string) (*component.Component, error) {
	return part(name, description,
		[]string{"in"}, nil, []string{"reading"},
		map[string]any{"value": 0.0},
		func(_ context.Context, this *component.Component) error {
			value := in[float64](this, "in")
			this.State().Set("value", value)
			return this.OutputByName("reading").PutSignals(signal.New(value))
		})
}

// newOilPump is driven straight off the crankshaft: oil pressure builds with speed.
func newOilPump() (*component.Component, error) {
	return part("oil-pump", "gear pump in the sump, driven by the crankshaft",
		[]string{"crank"}, nil, []string{"pressure", "torque"},
		map[string]any{"pressure": 0.0},
		func(_ context.Context, this *component.Component) error {
			rpm := in[Crank](this, "crank").RPM
			pressure := 4.5 * (1 - math.Exp(-rpm/900))
			this.State().Set("pressure", pressure)
			return emit(this, map[string]any{"pressure": pressure, "torque": -(0.3 + 0.0004*rpm)})
		})
}

// newACCompressor pumps refrigerant while its magnetic clutch has power.
// That costs the engine torque, and the condenser heats the radiator's air.
func newACCompressor() (*component.Component, error) {
	return part("ac-compressor", "belt-driven compressor with a magnetic clutch",
		[]string{"drive", "power"}, nil, []string{"drag", "condenser"},
		map[string]any{"engaged": false, "pressure": 0.0},
		func(_ context.Context, this *component.Component) error {
			drive := in[Drive](this, "drive")
			engaged := in[float64](this, "power") >= minVolts
			drag, condenser, target := -0.2, 0.0, 2.0
			if engaged {
				drag, condenser, target = -(5 + 0.001*drive.RPM), 1.0, min(16, 6+drive.RPM/200)
			}
			pressure := get(this, "pressure", 0.0)
			pressure += (target - pressure) * 0.05
			this.State().Set("engaged", engaged)
			this.State().Set("pressure", pressure)
			return emit(this, map[string]any{"drag": drag, "condenser": condenser})
		})
}

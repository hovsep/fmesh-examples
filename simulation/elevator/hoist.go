package main

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The hoist: everything from the drive to the counterweight. Each tick runs
// through it as a wave, one part per cycle:
//
//	controller → drive → motor → encoder, sheave → car, counterweight
//	controller → brake → motor
//	car → position sensor, governor, limit switches → controller, safety chain
//
// Each part acts on one input (its trigger) and only remembers the others,
// so it moves once per tick however many cycles the wave takes.

// Hoist port names.
const (
	portRef      = "ref"      // controller → drive: wanted speed, m/s
	portFeedback = "feedback" // encoder → drive
	portChain    = "chain"    // safety chain → drive, brake, controller: true while every contact is closed
	portTest     = "test"     // UI → drive: overspeed test
	portPower    = "power"    // drive → motor
	portBrake    = "brake"    // brake → motor, controller: released or not
	portLift     = "lift"     // controller → brake: lift it or let it drop
	portShaft    = "shaft"    // motor → encoder, sheave: shaft speed as rope metres per second
	portSpeed    = "speed"    // encoder → drive, controller
	portRope     = "rope"     // sheave → car, counterweight
	portClamp    = "clamp"    // safety gear → car
	portPosition = "position" // car → sensors, governor, landing doors
	portReading  = "reading"  // position sensor → controller
	portContact  = "contact"  // any safety contact → safety chain, safety gear
	portReset    = "reset"    // UI → governor
)

const (
	driveAccel    = 1.0  // m/s², the steepest ramp the drive allows
	brakeLiftTime = 0.3  // s for the brake to lift clear
	overspeed     = 1.15 // the governor trips at this share of rated speed
	testSpeed     = 1.35 // the overspeed test pushes the drive this far past rated
)

// Motion is where the car is and how fast it goes.
type Motion struct {
	Pos   float64 `json:"pos"`   // m above the lowest landing
	Speed float64 `json:"speed"` // m/s, up is positive
}

// Contact is one switch in the safety chain.
type Contact struct {
	Name   string
	Closed bool
}

func rpm(speed float64) float64 { return speed / (math.Pi * sheaveDiam) * 60 }

// DriveState is the inverter's panel.
type DriveState struct {
	Ref     float64 `json:"ref"`
	Out     float64 `json:"out"`
	Actual  float64 `json:"actual"`
	Hz      float64 `json:"hz"`
	Enabled bool    `json:"enabled"`
	Test    bool    `json:"test"`
}

// newDrive is the inverter: it ramps the motor towards the speed the
// controller asks for, no steeper than the ride allows, and cuts power the
// moment the safety chain opens.
func newDrive(c string) (*component.Component, error) {
	return component.New("drive-"+c,
		component.WithDescription("inverter: ramps the motor to the requested speed, cuts power when the safety chain opens"),
		component.WithInputs(portRef, portFeedback, portChain, portTest),
		component.WithOutputs(portPower, portTelemetry),
		component.WithInitialState(func(s component.State) {
			s.Set("out", 0.0)
			s.Set("actual", 0.0)
			s.Set("chain", true)
			s.Set("test", false)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			out := this.State().Get("out").(float64)
			actual := latest(this.InputByName(portFeedback), this.State().Get("actual").(float64))
			chain := latest(this.InputByName(portChain), this.State().Get("chain").(bool))
			test := this.State().Get("test").(bool) || this.InputByName(portTest).HasSignals()
			if !chain {
				test = false
			}
			this.State().Set("actual", actual)
			this.State().Set("chain", chain)
			this.State().Set("test", test)

			if !this.InputByName(portRef).HasSignals() {
				return nil
			}
			ref := latest(this.InputByName(portRef), 0.0)
			want := ref
			if test && ref != 0 {
				want = math.Copysign(testSpeed*ratedSpeed, ref)
			}
			switch {
			case !chain:
				out = 0
			case want > out:
				out = min(want, out+driveAccel*dt)
			default:
				out = max(want, out-driveAccel*dt)
			}
			this.State().Set("out", out)
			if err := this.OutputByName(portPower).PutSignals(signal.New(out)); err != nil {
				return err
			}
			return report(this, DriveState{Ref: ref, Out: out, Actual: actual, Hz: math.Abs(out) / ratedSpeed * 50, Enabled: chain, Test: test})
		}),
	)
}

// ShaftState is a rotating part.
type ShaftState struct {
	RPM float64 `json:"rpm"`
}

// newMotor is the gearless hoist motor: it turns as the drive feeds it,
// unless the brake holds its shaft.
func newMotor(c string) (*component.Component, error) {
	return component.New("motor-"+c,
		component.WithDescription("gearless hoist motor: turns as powered unless the brake holds it"),
		component.WithInputs(portPower, portBrake),
		component.WithOutputs(portShaft, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("released", false) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			released := latest(this.InputByName(portBrake), this.State().Get("released").(bool))
			this.State().Set("released", released)
			if !this.InputByName(portPower).HasSignals() {
				return nil
			}
			speed := latest(this.InputByName(portPower), 0.0)
			if !released {
				speed = 0
			}
			if err := this.OutputByName(portShaft).PutSignals(signal.New(speed)); err != nil {
				return err
			}
			return report(this, ShaftState{RPM: rpm(speed)})
		}),
	)
}

// BrakeState is the machine brake.
type BrakeState struct {
	Lift     float64 `json:"lift"` // 0 clamped … 1 clear
	Released bool    `json:"released"`
}

// newBrake is the spring-applied machine brake: it lifts slowly when the
// controller energises it and drops at once when told to or when the safety
// chain opens.
func newBrake(c string) (*component.Component, error) {
	return component.New("brake-"+c,
		component.WithDescription("machine brake: lifts when energised, drops on command or when the safety chain opens"),
		component.WithInputs(portLift, portChain),
		component.WithOutputs(portBrake, portTelemetry),
		component.WithInitialState(func(s component.State) {
			s.Set("lift", 0.0)
			s.Set("chain", true)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			chain := latest(this.InputByName(portChain), this.State().Get("chain").(bool))
			this.State().Set("chain", chain)
			if !this.InputByName(portLift).HasSignals() {
				return nil
			}
			lift := this.State().Get("lift").(float64)
			if latest(this.InputByName(portLift), false) && chain {
				lift = min(1, lift+dt/brakeLiftTime)
			} else {
				lift = 0
			}
			this.State().Set("lift", lift)
			released := lift >= 1
			if err := this.OutputByName(portBrake).PutSignals(signal.New(released)); err != nil {
				return err
			}
			return report(this, BrakeState{Lift: lift, Released: released})
		}),
	)
}

// EncoderState is the motor encoder.
type EncoderState struct {
	RPM    float64 `json:"rpm"`
	Pulses int     `json:"pulses"`
}

// newEncoder counts the motor's turns and tells the drive how fast it
// really goes.
func newEncoder(c string) (*component.Component, error) {
	const pulsesPerMetre = 2048 / (math.Pi * sheaveDiam)
	return component.New("encoder-"+c,
		component.WithDescription("motor encoder: measures the real shaft speed for the drive"),
		component.WithInputs(portShaft),
		component.WithOutputs(portSpeed, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("pulses", 0.0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			speed := latest(this.InputByName(portShaft), 0.0)
			pulses := this.State().Get("pulses").(float64) + speed*dt*pulsesPerMetre
			this.State().Set("pulses", pulses)
			if err := this.OutputByName(portSpeed).PutSignals(signal.New(speed)); err != nil {
				return err
			}
			return report(this, EncoderState{RPM: rpm(speed), Pulses: int(pulses)})
		}),
	)
}

// SheaveState is the traction sheave.
type SheaveState struct {
	Angle float64 `json:"angle"` // radians, for the UI to turn the wheel
}

// newSheave is the traction sheave on the motor shaft: it pulls the ropes,
// the car on one side and the counterweight on the other.
func newSheave(c string) (*component.Component, error) {
	return component.New("sheave-"+c,
		component.WithDescription("traction sheave: turns shaft rotation into rope travel"),
		component.WithInputs(portShaft),
		component.WithOutputs(portRope, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("angle", 0.0) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			speed := latest(this.InputByName(portShaft), 0.0)
			angle := math.Mod(this.State().Get("angle").(float64)+speed*dt/(sheaveDiam/2), 2*math.Pi)
			this.State().Set("angle", angle)
			if err := this.OutputByName(portRope).PutSignals(signal.New(speed)); err != nil {
				return err
			}
			return report(this, SheaveState{Angle: angle})
		}),
	)
}

// CarState is the car itself.
type CarState struct {
	Motion
	Clamped bool `json:"clamped"`
}

// newCar is the car frame hanging on the ropes: it goes where the ropes
// take it, unless the safety gear has gripped the rails.
func newCar(c string, start int) (*component.Component, error) {
	return component.New("car-"+c,
		component.WithDescription("car frame on the ropes: moves with them unless the safety gear grips the rails"),
		component.WithInputs(portRope, portClamp),
		component.WithOutputs(portPosition, portTelemetry),
		component.WithInitialState(func(s component.State) {
			s.Set("pos", floorPos(start))
			s.Set("clamped", false)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			clamped := latest(this.InputByName(portClamp), this.State().Get("clamped").(bool))
			this.State().Set("clamped", clamped)
			if !this.InputByName(portRope).HasSignals() {
				return nil
			}
			speed := latest(this.InputByName(portRope), 0.0)
			if clamped {
				speed = 0
			}
			pos := this.State().Get("pos").(float64) + speed*dt
			this.State().Set("pos", pos)
			m := Motion{Pos: pos, Speed: speed}
			if err := this.OutputByName(portPosition).PutSignals(signal.New(m)); err != nil {
				return err
			}
			return report(this, CarState{Motion: m, Clamped: clamped})
		}),
	)
}

// PosState is anything with a height in the shaft.
type PosState struct {
	Pos float64 `json:"pos"`
}

// newCounterweight hangs on the other end of the ropes: it goes down as
// the car goes up.
func newCounterweight(c string, start int) (*component.Component, error) {
	return component.New("counterweight-"+c,
		component.WithDescription("counterweight: the other end of the ropes"),
		component.WithInputs(portRope),
		component.WithOutputs(portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("pos", travel-floorPos(start)) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			pos := this.State().Get("pos").(float64) - latest(this.InputByName(portRope), 0.0)*dt
			this.State().Set("pos", pos)
			return report(this, PosState{Pos: pos})
		}),
	)
}

// GovernorState is the overspeed governor.
type GovernorState struct {
	RPM     float64 `json:"rpm"`
	Tripped bool    `json:"tripped"`
}

// newGovernor is the overspeed governor, turned by its own rope tied to the
// car: past the trip speed it opens its contact in the safety chain and
// sets off the safety gear, until a technician resets it.
func newGovernor(c string) (*component.Component, error) {
	return component.New("governor-"+c,
		component.WithDescription("overspeed governor: trips the safety chain and the safety gear past the trip speed"),
		component.WithInputs(portPosition, portReset),
		component.WithOutputs(portContact, portTelemetry),
		component.WithInitialState(func(s component.State) { s.Set("tripped", false) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			tripped := this.State().Get("tripped").(bool)
			if this.InputByName(portReset).HasSignals() {
				tripped = false
			}
			m := latest(this.InputByName(portPosition), Motion{})
			if math.Abs(m.Speed) > overspeed*ratedSpeed {
				tripped = true
			}
			this.State().Set("tripped", tripped)
			if err := this.OutputByName(portContact).PutSignals(signal.New(Contact{Name: "governor", Closed: !tripped})); err != nil {
				return err
			}
			return report(this, GovernorState{RPM: rpm(m.Speed) * 2, Tripped: tripped})
		}),
	)
}

// EngagedState is the safety gear.
type EngagedState struct {
	Engaged bool `json:"engaged"`
}

// newSafetyGear grips the guide rails when the governor trips.
func newSafetyGear(c string) (*component.Component, error) {
	return component.New("safety-gear-"+c,
		component.WithDescription("safety gear: grips the guide rails when the governor trips"),
		component.WithInputs(portContact),
		component.WithOutputs(portClamp, portTelemetry),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			engaged := !latest(this.InputByName(portContact), Contact{Closed: true}).Closed
			if err := this.OutputByName(portClamp).PutSignals(signal.New(engaged)); err != nil {
				return err
			}
			return report(this, EngagedState{Engaged: engaged})
		}),
	)
}

// ReadingState is the position sensor's view of the car.
type ReadingState struct {
	Motion
	Floor int  `json:"floor"`
	Level bool `json:"level"`
}

// newPositionSensor reads the coded tape along the shaft: the exact height
// of the car, the nearest floor and whether the car is level with it.
func newPositionSensor(c string) (*component.Component, error) {
	return component.New("position-sensor-"+c,
		component.WithDescription("tape reader: the car's height, nearest floor and whether it is level"),
		component.WithInputs(portPosition),
		component.WithOutputs(portReading, portTelemetry),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			m := latest(this.InputByName(portPosition), Motion{})
			f := nearestFloor(m.Pos)
			r := ReadingState{Motion: m, Floor: f, Level: math.Abs(m.Pos-floorPos(f)) < levelTolerance}
			if err := this.OutputByName(portReading).PutSignals(signal.New(r)); err != nil {
				return err
			}
			return report(this, r)
		}),
	)
}

// ContactState is a switch.
type ContactState struct {
	Closed bool `json:"closed"`
}

// newLimitSwitch is a final limit switch past the last landing: if the car
// ever overruns, it opens the safety chain.
func newLimitSwitch(c, end string, tripped func(pos float64) bool) (*component.Component, error) {
	return component.New(fmt.Sprintf("limit-%s-%s", end, c),
		component.WithDescription(fmt.Sprintf("final limit switch at the %s of the shaft", end)),
		component.WithInputs(portPosition),
		component.WithOutputs(portContact, portTelemetry),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			closed := !tripped(latest(this.InputByName(portPosition), Motion{}).Pos)
			if err := this.OutputByName(portContact).PutSignals(signal.New(Contact{Name: end + " limit", Closed: closed})); err != nil {
				return err
			}
			return report(this, ContactState{Closed: closed})
		}),
	)
}

// ChainState is the safety chain.
type ChainState struct {
	OK   bool     `json:"ok"`
	Open []string `json:"open"`
}

// newSafetyChain is the series circuit through every safety contact of one
// shaft: the landing door locks, the car door, the limit switches and the
// governor. One open contact, and the drive and the brake lose power.
func newSafetyChain(c string, contacts []string) (*component.Component, error) {
	return component.New("safety-chain-"+c,
		component.WithDescription("series circuit through every safety contact: one open contact stops the car"),
		component.WithInputs(portContact),
		component.WithOutputs(portChain, portTelemetry),
		component.WithInitialState(func(s component.State) {
			closed := map[string]bool{}
			for _, name := range contacts {
				closed[name] = true
			}
			s.Set("closed", closed)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			closed := this.State().Get("closed").(map[string]bool)
			for _, sig := range this.InputByName(portContact).Signals().All() {
				ct, err := sig.As[Contact]()
				if err != nil {
					return err
				}
				closed[ct.Name] = ct.Closed
			}
			open := []string{}
			for _, name := range contacts {
				if !closed[name] {
					open = append(open, name)
				}
			}
			ok := len(open) == 0
			if err := this.OutputByName(portChain).PutSignals(signal.New(ok)); err != nil {
				return err
			}
			return report(this, ChainState{OK: ok, Open: open})
		}),
	)
}

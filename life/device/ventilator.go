// Package device holds the machines that stand in for organs.
//
// They exist to make a point about the simulation rather than about medicine: a
// component is a contract, not an implementation. A mechanical ventilator does
// not work like a diaphragm at all -- one pushes air in under positive pressure
// on a schedule of its own, the other pulls it in by making the chest negative
// when the blood asks -- and yet the lungs cannot tell the difference, because
// what reaches them is the same port carrying the same signal. Swapping one for
// the other needs no change anywhere else in the body.
package device

//@TODO: let's drop the whole package for sake of simplicity

import (
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
)

// Ventilator settings and state.
const (
	// StateRate is the set rate in breaths per minute; zero means switched off.
	StateRate common.State = "set_rate"

	// StatePhase is where the machine is in its current breath.
	StatePhase common.State = "phase"

	// DefaultRate is what the machine delivers when switched on without a rate.
	DefaultRate = 14 * PerMinute

	// PeakInspiratoryPressure is how hard the machine pushes, expressed as the
	// equivalent pleural pressure so that it drives the same lungs the diaphragm
	// does.
	//
	// A real ventilator raises airway pressure rather than lowering pleural
	// pressure; the lungs see a pressure difference either way, and modelling
	// the difference rather than the sign keeps one port contract instead of
	// two. It is a simplification, and it is the only one: everything else about
	// the machine is unlike a diaphragm.
	PeakInspiratoryPressure = 4.5 * CmH2O

	// inspiratoryTime is the fraction of each cycle spent pushing. Machines
	// commonly use 1:2, the same as quiet breathing.
	inspiratoryFraction = 1.0 / 3.0
)

// Command verb this device answers to.
const VerbVentilate = "ventilate"

// ScalarRate is the breaths per minute a ventilate command asks for.
const ScalarRate = "rate"

// GetVentilator returns a mechanical ventilator.
//
// It emits exactly what organ:diaphragm emits, on exactly the same port, and
// takes none of the same inputs: it has no autonomic tone, feels no carbon
// dioxide, and will go on breathing for a body that cannot. That indifference is
// the machine's whole character -- and the reason a ventilated patient's blood
// gases are the operator's responsibility rather than their own.
func GetVentilator() (*component.Component, error) {
	c, err := component.New("device:ventilator",
		component.WithDescription("Mechanical ventilator: positive pressure at a set rate, deaf to the blood"),
		component.WithInputs(common.TimePort, common.ControlPort),
		component.WithOutputs("pleural_pressure"),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			acceptSettings,
			deliverBreath,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateRate, 0.0) // switched off
			state.Set(StatePhase, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("device:ventilator: %w", err)
	}
	return c, nil
}

func acceptSettings(this *component.Component) error {
	return helper.ForEachCommand(this, common.ControlPort, func(name string, args *meta.Scalars) error {
		if helper.CommandVerb(name) != VerbVentilate {
			return nil
		}

		rate := args.ValueOrDefault(ScalarRate, DefaultRate)
		this.State().Set(StateRate, max(rate, 0))
		if rate <= 0 {
			this.Logger().Println("ventilator off")
		} else {
			this.Logger().Printf("ventilating at %.0f breaths/min", rate)
		}
		return nil
	})
}

// deliverBreath drives the chest on the machine's own schedule.
func deliverBreath(this *component.Component) error {
	if !this.InputByName(common.TimePort).HasSignals() {
		return nil
	}

	rate := this.State().Get(StateRate).(float64)
	if rate <= 0 {
		// Switched off: emit nothing at all, so the body breathes for itself.
		// A machine that published a resting pressure would be quietly fighting
		// the diaphragm it is meant to be standing by for.
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName(common.TimePort).Signals().First())
	if err != nil {
		return err
	}

	phase := math.Mod(this.State().Get(StatePhase).(float64)+dt*rate/60.0, 1.0)
	this.State().Set(StatePhase, phase)

	return this.OutputByName("pleural_pressure").PutPayloads(pressureAt(phase))
}

// riseFraction is how much of the inspiratory phase the machine spends coming up
// to pressure. Real ventilators ramp rather than step, and for good reason: a
// step into an airway of any resistance produces a flow spike measured in litres
// per second. Ours reached 2.3 L/s before this was added.
const riseFraction = 0.35

// releaseDecay sets how fast the expiratory valve lets the pressure fall away.
// Opening it instantly is a step in the other direction and produces the same
// flow spike the ramp above was added to prevent.
const releaseDecay = 6.0

// pressureAt is the machine's waveform: a ramp to a held plateau, then a passive
// release. It is as unlike the diaphragm's smooth sinusoid as the two mechanisms
// are unlike -- a machine holds a pressure, a muscle pulls and lets go -- and the
// lungs take either without knowing the difference.
func pressureAt(phase float64) float64 {
	if phase < inspiratoryFraction {
		inspiratory := phase / inspiratoryFraction
		effort := min(inspiratory/riseFraction, 1.0)
		return organ.BasePleuralPressure - PeakInspiratoryPressure*effort
	}

	expiratory := (phase - inspiratoryFraction) / (1 - inspiratoryFraction)
	return organ.BasePleuralPressure - PeakInspiratoryPressure*math.Exp(-releaseDecay*expiratory)
}

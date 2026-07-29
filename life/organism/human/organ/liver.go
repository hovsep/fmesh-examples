package organ

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/life/plugin/receptor"
	"github.com/hovsep/fmesh/component"
)

// The liver, as the body's glucose bank.
//
// Every other tissue in this simulation takes fuel out of the blood. The liver
// is the only one that puts it back, and the only one that decides how much --
// which is why blood sugar was, until it existed, a number that drifted toward
// 90 for no reason anyone could point at. Now it has a reason: something
// measures it (organ:pancreas), something is told about it (this), and the
// telling can be interrupted.
//
// The liver does far more than this. It is left at glucose deliberately: one
// organ doing one job well says more about the simulation than an organ with
// nine half-modelled ones.
const (
	// LiverO2PerMinute is the liver's resting oxygen demand, mL/min. It is one
	// of the largest single consumers in the body, and its share arrives partly
	// through the portal vein carrying blood the gut has already used -- which
	// is why the liver tolerates being second in line better than any other
	// organ, and why it is not modelled as being second in line here.
	LiverO2PerMinute = 40.0

	// BasalHepaticGlucoseOutput is how fast a fasting liver releases glucose
	// into the blood, in mg/dL of blood per second.
	//
	// The clinical figure is about 2 mg per kg of body weight per minute: for a
	// 70 kg adult, 140 mg a minute, arriving in roughly 50 dL of blood. That is
	// 2.8 mg/dL a minute, and the whole circulating pool of sugar -- about four
	// and a half grams, less than a spoonful -- turns over every half hour. The
	// body keeps almost no sugar in the blood, which is exactly why losing the
	// control loop is dangerous so quickly.
	BasalHepaticGlucoseOutput = 2.8 / 60.0

	// glucagonBoost is how much extra glucose maximal glucagon can pull out of
	// storage, as a multiple of the basal rate. Several times over: the liver
	// holds about a hundred grams of glycogen against a circulating pool of four,
	// and can empty it far faster than anything can spend it.
	glucagonBoost = 5.0

	// insulinSuppression is what maximal insulin does, in the same units. It
	// exceeds 1 because insulin does not merely switch hepatic output off: it
	// reverses it, and the liver starts taking sugar out of the blood and
	// putting it away. That sign change is the whole of what insulin is for.
	//
	// Six, because a body under maximal insulin disposes of glucose at roughly
	// ten mg per kg per minute against a basal turnover of two -- five times the
	// basal rate taken *out* of the blood, on top of the one that is no longer
	// going in. Anything less and a large meal outruns the disposal and blood
	// sugar climbs until it hits the ceiling on the arithmetic, which is a
	// diabetic's curve rather than a healthy one.
	insulinSuppression = 6.0

	stateGlucoseFlux common.State = "glucose_flux"
)

// GetLiver returns the liver.
func GetLiver() (*component.Component, error) {
	c, err := component.New("organ:liver",
		component.WithDescription("Liver: releases and stores glucose as the pancreas directs"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "liver"}),
			perfusion.New(perfusion.Config{Organ: "liver", O2PerMinute: LiverO2PerMinute}),
			receptor.For(bloodstream.HormoneInsulin, bloodstream.HormoneGlucagon),
		),
		component.WithInputs(common.TimePort),
		component.WithOutputs("glucose_flux"),
		component.WithActivationFunc(damage.FlatlineWhenFailed(regulateGlucose)),
		component.WithInitialState(func(state component.State) {
			state.Set(stateGlucoseFlux, BasalHepaticGlucoseOutput)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:liver: %w", err)
	}
	return c, nil
}

// regulateGlucose sets how fast the liver is releasing sugar into the blood, or
// taking it back out.
//
// The liver never measures blood sugar here. It measures two hormones, and does
// what they say -- which is faithful, and is what makes the failure modes
// interesting. Destroy the pancreas and the liver goes on obeying the last
// instructions until they wash out, then falls back to its basal rate and stops
// responding to anything. Destroy the liver and the pancreas goes on shouting
// into a blood supply where nothing is listening.
func regulateGlucose(this *component.Component) error {
	if !this.InputByName(common.TimePort).HasSignals() {
		return nil
	}

	glucagon := receptor.Level(this, bloodstream.HormoneGlucagon)
	insulin := receptor.Level(this, bloodstream.HormoneInsulin)

	// A liver short of oxygen cannot do this work either; gluconeogenesis is
	// expensive, and a failing liver is a classic cause of low blood sugar.
	flux := BasalHepaticGlucoseOutput *
		(1 + glucagonBoost*glucagon - insulinSuppression*insulin) *
		perfusion.Sufficiency(this)

	this.State().Set(stateGlucoseFlux, flux)
	return this.OutputByName("glucose_flux").PutPayloads(flux)
}

// GlucoseFlux is the liver's current net glucose output, mg/dL per second.
// Negative means it is taking sugar out of the blood.
func GlucoseFlux(c *component.Component) float64 {
	flux, _ := c.State().Get(stateGlucoseFlux).(float64)
	return flux
}

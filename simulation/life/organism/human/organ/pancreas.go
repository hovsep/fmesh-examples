package organ

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/simulation/life/bloodstream"
	"github.com/hovsep/fmesh-examples/simulation/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/simulation/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation/sim"
	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
	"github.com/hovsep/fmesh/component"
)

// The endocrine pancreas: the islets that decide whether the body is storing
// fuel or spending it.
//
// It is a sensor and a transmitter and nothing else. It tastes the sugar in the
// blood perfusing it and answers with one of two hormones -- and because both go
// out on the same bus the adrenal glands already use, nothing downstream had to
// be taught that an endocrine pancreas exists. The liver grew receptors; every
// other organ remains deaf, exactly as a tissue without the receptor is deaf in
// life.
//
// Destroying this component is type 1 diabetes: the sensing is gone, the liver
// stops being told anything, and blood sugar is left to whatever eating and
// burning do to it. That is worth doing in front of an audience.
const (
	// PancreasO2PerMinute is the islets' oxygen demand, mL/min. The endocrine
	// pancreas is a gram or two of tissue scattered through an organ that is
	// mostly digestive gland, and its share of the body's oxygen is tiny.
	PancreasO2PerMinute = 3.0

	// GlycemicSetpoint is the blood sugar the islets defend, in mg/dL. Above it
	// the beta cells release insulin; below it the alpha cells release glucagon.
	// Neither is secreted exactly at the setpoint, which is what makes it one.
	GlycemicSetpoint = bloodstream.DefaultGlucoseLevel

	// The spans over which each cell type goes from silent to flat out, in
	// mg/dL away from the setpoint.
	//
	// They are deliberately, and physiologically, lopsided. Insulin secretion
	// climbs gently and is half-maximal only around 150 mg/dL, well into the
	// range a diabetic lives in; glucagon is flat out eight times sooner, by the
	// time sugar has fallen a mere fifteen points. The body treats a sugar of
	// 250 as something to deal with over the afternoon, and a sugar of 75 as
	// already worth acting on -- which is the right way round, because
	// hyperglycaemia takes decades to do its damage and hypoglycaemia takes
	// minutes.
	//
	// The narrow span is also what keeps blood sugar steady under load. A wide
	// one leaves a proportional controller with a large standing error: an
	// earlier span of 40 let a running body settle at 50 mg/dL and stay there,
	// which is a hypoglycaemic collapse a runner does not actually have.
	insulinSpan  = 120.0
	glucagonSpan = 15.0
)

// Flat-out secretion holds the circulating level at saturation: it replaces
// exactly what the blood clears. Deriving the rate from the half-life rather
// than picking a number means the two cannot drift apart when one is retuned.
var (
	maxInsulinRate  = math.Ln2 / bloodstream.InsulinHalfLifeSec
	maxGlucagonRate = math.Ln2 / bloodstream.GlucagonHalfLifeSec
)

// GetPancreas returns the endocrine pancreas.
func GetPancreas() (*component.Component, error) {
	c, err := component.New("organ:pancreas",
		component.WithDescription("Pancreatic islets: insulin above the setpoint, glucagon below it"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "pancreas"}),
			perfusion.New(perfusion.Config{Organ: "pancreas", O2PerMinute: PancreasO2PerMinute}),
		),
		component.WithInputs(sim.TimePort),
		component.WithActivationFunc(component.When(damage.Working, secretePancreaticHormones)),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:pancreas: %w", err)
	}
	return c, nil
}

// secretePancreaticHormones reads the sugar going past and answers it.
//
// The islets have no idea what the liver will do about it, and no way of finding
// out. They state a condition of the blood; whether anything acts on it is a
// question about receptors, somewhere else. Cutting that thread -- by destroying
// either end -- is how the two halves of diabetes differ: no signal sent, or a
// signal sent and not heard.
func secretePancreaticHormones(_ context.Context, this *component.Component) error {
	if !this.InputByName(sim.TimePort).HasSignals() {
		return nil
	}

	glucose := perfusion.Read(this).Glucose

	// Only one arm is ever active: sugar is either above the setpoint or below
	// it. Both being zero at the setpoint itself is what holds the body there.
	insulin := mathx.Clamp((glucose-GlycemicSetpoint)/insulinSpan, 0, 1)
	glucagon := mathx.Clamp((GlycemicSetpoint-glucose)/glucagonSpan, 0, 1)

	if insulin <= 0 && glucagon <= 0 {
		return nil
	}

	return this.OutputByName(bloodstream.ReturnPort).PutSignals(
		bloodstream.HormoneSecretion(bloodstream.HormoneInsulin, insulin*maxInsulinRate),
		bloodstream.HormoneSecretion(bloodstream.HormoneGlucagon, glucagon*maxGlucagonRate),
	)
}

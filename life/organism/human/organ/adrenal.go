package organ

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/autonomic"
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/component"
)

// The adrenal glands: the body's answer to a stressor, on two clocks.
//
// The medulla releases adrenaline within seconds of sympathetic drive -- the
// fright response, gone again in minutes. The cortex releases cortisol over
// minutes and clears it over hours -- the response to a situation rather than to
// a moment. The same falling blood pressure provokes both, and the difference
// between them is the difference between surviving the next thirty seconds and
// surviving the next six hours.
//
// Modelling both is what makes the endocrine system worth having in a
// simulation: one axis would just look like a slower nerve.
const (
	// AdrenalO2PerMinute is the glands' oxygen demand, mL/min. They are small.
	AdrenalO2PerMinute = 2.0

	// Peak secretion rates, in circulating level per second. Adrenaline can
	// take the blood from nothing to saturated in about ten seconds; cortisol
	// takes several minutes.
	maxAdrenalineRate = 0.10 * PerSecond
	maxCortisolRate   = 0.004 * PerSecond

	// Sympathetic drive below this is ordinary living and provokes nothing.
	// Above it, secretion rises with the drive.
	secretionThreshold = 0.35 * DNCS

	stateSympathetic common.State = "sympathetic_tone"
)

// GetAdrenal returns the adrenal glands.
func GetAdrenal() (*component.Component, error) {
	c, err := component.New("organ:adrenal",
		component.WithDescription("Adrenal glands: adrenaline in seconds, cortisol over minutes"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "adrenal"}),
			perfusion.New(perfusion.Config{Organ: "adrenal", O2PerMinute: AdrenalO2PerMinute}),
		),
		component.WithInputs(common.TimePort, "autonomic_tone"),
		component.WithActivationFunc(damage.FlatlineWhenFailed(secreteStressHormones)),
		component.WithInitialState(func(state component.State) {
			state.Set(stateSympathetic, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:adrenal: %w", err)
	}
	return c, nil
}

// secreteStressHormones puts both stress hormones onto the blood bus in
// proportion to how hard the sympathetic system is driving.
//
// The glands emit a secretion *rate*, not a level: what circulates is the
// balance between this and the blood's clearance, so a gland that stops
// secreting does not have to cancel anything. That is why adrenaline fades on
// its own within minutes while cortisol is still there an hour later.
func secreteStressHormones(this *component.Component) error {
	if in := this.InputByName("autonomic_tone"); in.HasSignals() {
		if tone, err := autonomic.Bias(in.Signals().First(), common.Sympathetic); err == nil {
			this.State().Set(stateSympathetic, tone)
		}
	}

	if !this.InputByName(common.TimePort).HasSignals() {
		return nil
	}

	stress := mathx.Clamp(
		(this.State().Get(stateSympathetic).(float64)-secretionThreshold)/(1-secretionThreshold),
		0, 1)
	if stress <= 0 {
		return nil
	}

	return this.OutputByName(bloodstream.ReturnPort).PutSignals(
		bloodstream.HormoneSecretion(bloodstream.HormoneAdrenaline, stress*maxAdrenalineRate),
		bloodstream.HormoneSecretion(bloodstream.HormoneCortisol, stress*maxCortisolRate),
	)
}

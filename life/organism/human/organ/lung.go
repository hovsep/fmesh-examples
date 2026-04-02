package organ

import (
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	TidalBreathingRate = 12 * PerMinute // per minute
	tidalVolume        = 500.0 * Milliliter
)

// GetLung returns a single lung organ component
func GetLung(side common.Side) *component.Component {
	return component.New("organ:lung_"+side).
		WithDescription("A lung").
		WithInitialState(func(state component.State) {
			state.Set(common.Phase, 0.0)
			state.Set(common.Rate, TidalBreathingRate)

			state.Set("inhale_ratio", 0.4)
			state.Set("pause_after_inhale", 0.05)
			state.Set("pause_after_exhale", 0.05)

			state.Set("min_volume", 1200.0)
			state.Set("max_volume", 1700.0)

			state.Set("volume", tidalVolume)
			state.Set("prev_volume", tidalVolume)

			state.Set("pleural_pressure", -5.0) // cmH2O baseline
			state.Set("alveolar_pressure", 0.0)

			state.Set(common.DamageLevel, defaultDamageLevel)
		}).
		AddInputs(
			"time",                  // Simulation clock (dt)
			"autonomic_tone",        // Controls breathing rate
			"diaphragm_contraction", // Drives pleural pressure (primary inhale force)
			"inspired_gas",          // Incoming air composition
		).
		AddOutputs(
			"exhaled_gas",          // Gas leaving lungs
			"phase",                // Breathing cycle phase ∈ [0,1)
			"flow",                 // Airflow (ml/s), signed (+ inhale, - exhale)
			"volume",               // Lung volume (ml)
			"alveolar_pressure",    // Pressure inside alveoli
			"pleural_pressure",     // Pressure in pleural cavity
			"gas_composition",      // Alveolar gas composition
			"respiratory_rate",     // Breaths per minute
			"inspiration_duration", // Seconds
			"exhalation_duration",  // Seconds
			"lung_efficiency",      // Gas exchange efficiency
			"alveolar_dead_space",  // Non-exchanging volume
			"stretch_ratio",        // volume / max_volume
		).
		WithActivationFunc(helper.Pipeline(
			handleLungAutonomicTone,
			handleOscillation,
			handleMechanics,
			handlePressures,
			handleGasExchange,
			publishOutputs,
		))
}

func handleLungAutonomicTone(this *component.Component) error {
	if !this.InputByName("autonomic_tone").HasSignals() {
		return nil
	}

	bias, err := helper.GetBias(this.InputByName("autonomic_tone").Signals().First(), common.Respiratory)
	if err != nil {
		return err
	}

	this.State().Update(common.Rate, func(v any) any {
		return int(8 + (30-8)*bias) // 8–30 breaths/min
	})

	return nil
}

func handleOscillation(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	this.State().Update(common.Phase, func(old any) any {
		phase := old.(float64)
		rate := this.State().Get(common.Rate).(int)

		cycle := 60.0 / float64(rate)
		step := dt / cycle

		return math.Mod(phase+step, 1.0)
	})

	return nil
}

func handleMechanics(this *component.Component) error {
	phase := this.State().Get(common.Phase).(float64)

	volume := computeVolume(this, phase)

	prev := this.State().Get("prev_volume").(float64)
	dt, _ := helper.TickDurationInSec(this.InputByName("time").Signals().First())

	flow := (volume - prev) / dt

	this.State().Set("volume", volume)
	this.State().Set("prev_volume", volume)
	this.State().Set("flow", flow)

	return nil
}

func computeVolume(this *component.Component, phase float64) float64 {
	inhaleRatio := this.State().Get("inhale_ratio").(float64)
	pauseIn := this.State().Get("pause_after_inhale").(float64)
	pauseOut := this.State().Get("pause_after_exhale").(float64)

	minVol := this.State().Get("min_volume").(float64)
	maxVol := this.State().Get("max_volume").(float64)

	inhaleEnd := inhaleRatio
	hold1End := inhaleEnd + pauseIn
	exhaleEnd := hold1End + (1 - inhaleRatio - pauseIn - pauseOut)

	var normalized float64
	var volume float64

	switch {
	case phase < inhaleEnd:
		normalized = phase / inhaleRatio
		volume = lerp(minVol, maxVol, smoothstep(normalized))

	case phase < hold1End:
		volume = maxVol

	case phase < exhaleEnd:
		normalized = (phase - hold1End) / (exhaleEnd - hold1End)
		volume = lerp(maxVol, minVol, smoothstep(normalized))

	default:
		volume = minVol
	}

	return volume
}

func handlePressures(this *component.Component) error {
	volume := this.State().Get("volume").(float64)
	minVol := this.State().Get("min_volume").(float64)
	maxVol := this.State().Get("max_volume").(float64)

	// Normalize stretch
	stretch := (volume - minVol) / (maxVol - minVol)

	// Pleural pressure: more negative during inhale
	pleural := -5.0 - 3.0*stretch

	// Alveolar pressure: follows flow (very simplified)
	flow := this.State().Get("flow").(float64)
	alveolar := -flow * 0.01

	this.State().Set("pleural_pressure", pleural)
	this.State().Set("alveolar_pressure", alveolar)
	this.State().Set("stretch_ratio", stretch)

	return nil
}

func handleGasExchange(this *component.Component) error {
	// Placeholder — structure matters more now

	this.State().Set("lung_efficiency", 0.25)
	this.State().Set("alveolar_dead_space", 150.0)

	return nil
}

func publishOutputs(this *component.Component) error {
	this.OutputByName("phase").PutPayloads(this.State().Get(common.Phase))
	this.OutputByName("volume").PutPayloads(this.State().Get("volume"))
	this.OutputByName("flow").PutPayloads(this.State().Get("flow"))
	this.OutputByName("alveolar_pressure").PutPayloads(this.State().Get("alveolar_pressure"))
	this.OutputByName("pleural_pressure").PutPayloads(this.State().Get("pleural_pressure"))
	this.OutputByName("stretch_ratio").PutPayloads(this.State().Get("stretch_ratio"))

	this.OutputByName("respiratory_rate").PutPayloads(this.State().Get(common.Rate))

	return nil
}

func smoothstep(x float64) float64 {
	return x * x * (3 - 2*x)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

package organ

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/component"
)

type BreathingPhase string

const (
	tidalBreathingRate                = 12    // per minute
	tidalVolume                       = 500.0 // ml
	Inhale             BreathingPhase = "inhale"
	Exhale             BreathingPhase = "exhale"
)

// GetLung returns a single lung organ component
func GetLung(side common.Side) *component.Component {
	return component.New("organ:lung_"+side).
		WithDescription("A lung").
		WithInitialState(func(state component.State) {
			state.Set(common.Phase, 0.00)
			state.Set(common.Rate, tidalBreathingRate) //
			state.Set("residual_volume", 1200.00)
			state.Set("volume", tidalVolume) // Tidal volume
			state.Set("inspiratory_reserve", 1900.00)
			state.Set("expiratory_reserve", 700.00)
			state.Set(common.DamageLevel, defaultDamageLevel)
		}).
		AddInputs(
			"time", //do we really need time?
			"autonomic_tone",
			"diaphragm_contraction",
			// @TODO: add "intercostal_muscles_contraction", and make it as 20% of lung moving contribution along with the diaphragm (but they must trigger only at exercise time, not resting)
			"inspired_gas",
		).
		AddOutputs(
			"exhaled_gas",
			"phase",
			"volume",
			"alveolar_pressure",
			"pleural_pressure",
			"gas_composition",
			"respiratory_rate",
			"inspiration_duration",
			"exhalation_duration",
			"lung_efficiency",     // Fraction of oxygen extracted per breath
			"alveolar_dead_space", //Volume that doesn’t participate in gas exchange
			"stretch_ratio",       //Current volume / maximum lung volume
		).
		WithActivationFunc(func(this *component.Component) error {
			this.OutputByName("phase").PutPayloads(Inhale, Exhale)
			return nil
		})
}

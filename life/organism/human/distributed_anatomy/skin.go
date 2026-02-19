package da

import "github.com/hovsep/fmesh/component"

// GetSkin ...
func GetSkin() *component.Component {
	return component.New("da:skin").
		WithDescription("Skin").
		AddInputs("time", "thermal_load", "radiation", "mechanical_load").
		AddOutputs("temperature_change", "pain_signal", "hydration_loss").
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

package da

import "github.com/hovsep/fmesh/component"

// GetSkin returns the skin component
func GetSkin() *component.Component {
	c, err := component.New("da:skin",
		component.WithDescription("Skin"),
		component.WithInputs("time", "thermal_load", "radiation", "mechanical_load"),
		component.WithOutputs("temperature_change", "pain_signal", "hydration_loss"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}

package da

import "github.com/hovsep/fmesh/component"

// GetGITract ...
func GetGITract() *component.Component {
	return component.New("da:gi_tract").
		WithDescription("GI / Digestive Tract").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

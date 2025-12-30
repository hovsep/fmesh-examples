package organ

import "github.com/hovsep/fmesh/component"

func GetGIComponent() *component.Component {
	return component.New("gi").
		WithDescription("GI / Digestive Tract").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

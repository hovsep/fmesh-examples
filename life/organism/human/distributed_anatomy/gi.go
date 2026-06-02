package da

import "github.com/hovsep/fmesh/component"

// GetGITract returns the GI tract component
func GetGITract() *component.Component {
	c, err := component.New("da:gi_tract",
		component.WithDescription("GI / Digestive Tract"),
		component.WithInputs("time"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}

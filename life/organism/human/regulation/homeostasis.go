package regulation

import "github.com/hovsep/fmesh/component"

// GetHomeostasis ...
func GetHomeostasis() *component.Component {
	c, err := component.New("regulation:homeostasis",
		component.WithDescription("Homeostasis regulation system. Runs all the time and tries to keep important levels within ranges"),
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

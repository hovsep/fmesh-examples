package physiology

import "github.com/hovsep/fmesh/component"

// GetEndocrineAxis ...
func GetEndocrineAxis() *component.Component {
	c, err := component.New("physiology:endocrine_axis",
		component.WithDescription("Endocrine system"),
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

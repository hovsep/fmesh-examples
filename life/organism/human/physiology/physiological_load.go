package physiology

import "github.com/hovsep/fmesh/component"

// GetPhysiologicalLoad ...
func GetPhysiologicalLoad() *component.Component {
	c, err := component.New("physiology:physiological_load",
		component.WithDescription("Physiological load (e.g., thermal, mechanical, radiation etc)"),
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

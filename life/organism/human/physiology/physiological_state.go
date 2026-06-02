package physiology

import "github.com/hovsep/fmesh/component"

// GetPhysiologicalState ...
func GetPhysiologicalState() *component.Component {
	c, err := component.New("physiology:physiological_state",
		component.WithDescription("Internal physiological state (e.g., temperature, blood pressure etc)"),
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

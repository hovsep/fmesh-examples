package physiology

import "github.com/hovsep/fmesh/component"

// GetAutonomicCoordination ...
func GetAutonomicCoordination() *component.Component {
	return component.New("physiology:autonomic_coordination").
		WithDescription("Autonomic coordination system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})

	//TODO:
	//Translate systemic load into:
	//
	//Heart rate modulation
	//
	//Vasoconstriction signals
	//
	//Breathing drive

}

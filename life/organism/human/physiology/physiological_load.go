package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// GetPhysiologicalLoad ...
func GetPhysiologicalLoad() (*component.Component, error) {
	c, err := component.New("physiology:physiological_load",
		component.WithDescription("Physiological load (e.g., thermal, mechanical, radiation etc)"),
		component.WithInputs("time"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:physiological_load: %w", err)
	}
	return c, nil
}

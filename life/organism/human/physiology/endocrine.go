package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// GetEndocrineAxis ...
func GetEndocrineAxis() (*component.Component, error) {
	c, err := component.New("physiology:endocrine_axis",
		component.WithDescription("Endocrine system"),
		component.WithInputs("time"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:endocrine_axis: %w", err)
	}
	return c, nil
}

package da

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// GetNervousSystem returns the nervous system component
func GetNervousSystem() (*component.Component, error) {
	c, err := component.New("da:nervous_system",
		component.WithDescription("Nervous system"),
		component.WithInputs("time"),
		component.WithActivationFunc(func(this *component.Component) error {
			//@TODO: implement or drop it completely
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:nervous_system: %w", err)
	}
	return c, nil
}

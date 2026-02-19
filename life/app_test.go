package main

import (
	"os"
	"testing"

	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
)

func Test_AppChecks(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, app *step_sim.Application)
	}{
		{
			name: "mesh is created and human component is present",
			assertions: func(t *testing.T, app *step_sim.Application) {
				assert.NotNil(t, app)
				humanComponent := app.Sim.FM.Components().FindAny(func(c *component.Component) bool {
					return c.Labels().ValueIs("role", "organism") &&
						c.Labels().ValueIs("genus", "homo") &&
						c.Labels().ValueIs("species", "sapiens")
				})
				assert.NotNil(t, humanComponent)
				assert.False(t, humanComponent.HasChainableErr())
				assert.NoError(t, humanComponent.ChainableErr())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mesh := getSimulationMesh()
			app := step_sim.NewApp(mesh, initSim, os.Stdin)
			assert.NotNil(t, app)

			if tt.assertions != nil {
				tt.assertions(t, app)
			}
		})
	}
}

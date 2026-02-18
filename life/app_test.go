package main

import (
	"context"
	"os"
	"testing"
	"time"

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

			if tt.assertions != nil {
				tt.assertions(t, app)
			}
		})
	}
}

func Test_SimChecks(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *step_sim.Simulation, cmdChan chan step_sim.Command)
	}{
		{
			name: "time advances",
			assertions: func(t *testing.T, sim *step_sim.Simulation, cmdChan chan step_sim.Command) {
				timeComponent := sim.FM.ComponentByName("time")
				t1 := timeComponent.State().Get("tick_count")

				assert.Zero(t, t1)

				go sim.Run()
				time.Sleep(100 * time.Millisecond)

				t2 := timeComponent.State().Get("tick_count")
				assert.Greater(t, t2, t1)
				cmdChan <- "exit"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdChan := make(chan step_sim.Command)
			fm := getSimulationMesh()
			sim := step_sim.NewSimulation(context.Background(), cmdChan, fm)

			if tt.assertions != nil {
				tt.assertions(t, sim, cmdChan)
			}
		})
	}
}

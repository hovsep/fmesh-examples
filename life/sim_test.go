package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/stretchr/testify/assert"
)

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

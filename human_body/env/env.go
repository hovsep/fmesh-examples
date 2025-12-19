package env

import (
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/env/factor"
	"github.com/hovsep/fmesh/signal"
)

const (
	meshName = "env"
)

// GetMesh builds the mesh that simulates the outside environment (the world)
func GetMesh() *fmesh.FMesh {
	simTime := factor.GetTimeComponent()
	temperature := factor.GetTempComponent()
	//@TODO:
	// humidity(%RH),
	// radiation,
	// uv,
	// air pressure,composition
	// noise,
	// light level,
	// gravity,
	// acceleration,
	// physical impacts (running, weight lifting, walking)
	// injury

	// Start the time
	simTime.InputByName("ctl").PutSignals(signal.New("start"))

	return fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: 0,
		TimeLimit:   10 * time.Second,
	}).
		AddComponents(simTime, temperature)
}

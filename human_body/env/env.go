package env

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/env/factor"
)

const (
	meshName = "env"
)

// GetMesh builds the mesh that simulates the outside environment (the world)
func GetMesh() *fmesh.FMesh {
	temperature := factor.GetTempComponent()
	//@TODO:
	// time (generate sim ticks, each tick is 10ms of real time, fast forwarding)
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

	return fmesh.New(meshName).
		AddComponents(temperature)
}

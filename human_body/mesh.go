package main

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/body"
	"github.com/hovsep/fmesh-examples/human_body/env"
)

func getMesh() *fmesh.FMesh {
	// Create the world
	world := env.GetMesh()

	// Create the human being
	humanBeing := body.GetComponent()

	// Put human being into the world
	world.AddComponents(humanBeing)

	return world
}

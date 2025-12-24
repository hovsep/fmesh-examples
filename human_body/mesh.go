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

	env.AddOrganisms(world, humanBeing)

	return world
}

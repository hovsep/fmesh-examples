package step_sim

import "github.com/hovsep/fmesh"

type Command string

type MeshCommandDescriptor struct {
	Description string
	Func        func(*fmesh.FMesh)
}

const (
	Pause  Command = "pause"
	Resume Command = "resume"
	Exit   Command = "exit"
	Help   Command = "help"
)

var NoopMeshCommand = func(*fmesh.FMesh) {
	return
}

func NewMeshCommandDescriptor(desc string, cmdFunc func(*fmesh.FMesh)) MeshCommandDescriptor {
	return MeshCommandDescriptor{desc, cmdFunc}
}

func (md MeshCommandDescriptor) RunWithMesh(fm *fmesh.FMesh) {
	md.Func(fm)
}

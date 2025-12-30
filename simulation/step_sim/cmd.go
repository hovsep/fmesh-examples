package step_sim

import "github.com/hovsep/fmesh"

type Command string

type MeshCommandDescriptor struct {
	Description string
	Func        func(*fmesh.FMesh)
}

const (
	cmdPause  Command = "pause"
	cmdResume Command = "resume"
	cmdExit   Command = "exit"
	cmdHelp   Command = "help"
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

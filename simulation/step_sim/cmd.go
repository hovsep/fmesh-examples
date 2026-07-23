package step_sim

import "github.com/hovsep/fmesh"

type Command string

type MeshCommandDescriptor struct {
	Description string
	Func        func(*fmesh.FMesh)
	ArgsFunc    func(fm *fmesh.FMesh, args []string) // optional; takes precedence over Func when set
}

const (
	Pause  Command = "pause"
	Resume Command = "resume"
	Exit   Command = "exit"
	Help   Command = "help"
)

var NoopMeshCommand = func(*fmesh.FMesh) {}

func NewMeshCommandDescriptor(desc string, cmdFunc func(*fmesh.FMesh)) MeshCommandDescriptor {
	return MeshCommandDescriptor{Description: desc, Func: cmdFunc}
}

// NewMeshCommandDescriptorWithArgs registers a command that receives the
// whitespace-separated arguments following the command name (e.g. "rate 100ms").
func NewMeshCommandDescriptorWithArgs(desc string, cmdFunc func(*fmesh.FMesh, []string)) MeshCommandDescriptor {
	return MeshCommandDescriptor{Description: desc, ArgsFunc: cmdFunc}
}

func (md MeshCommandDescriptor) RunWithMesh(fm *fmesh.FMesh, args []string) {
	if md.ArgsFunc != nil {
		md.ArgsFunc(fm, args)
		return
	}
	md.Func(fm)
}

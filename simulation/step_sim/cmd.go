package step_sim

import "github.com/hovsep/fmesh"

type Command string

// MeshCommand is a named action a session can invoke. Every command takes the
// whitespace-separated arguments after its name; commands that want none simply
// ignore the slice. (There used to be a separate no-arg shape with its own
// constructor and a RunWithMesh dispatcher — one signature is simpler and costs
// argument-free handlers only a "_ []string".)
type MeshCommand struct {
	Description string
	Run         func(fm *fmesh.FMesh, args []string)
}

const (
	Pause  Command = "pause"
	Resume Command = "resume"
	Exit   Command = "exit"
	Help   Command = "help"
)

func NewMeshCommand(desc string, run func(fm *fmesh.FMesh, args []string)) MeshCommand {
	return MeshCommand{Description: desc, Run: run}
}

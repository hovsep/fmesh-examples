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
	// Group is the help section this command appears under. Empty means the
	// fallback group (see showHelp), so callers only set it when they want a
	// named section.
	Group string
	Run   func(fm *fmesh.FMesh, args []string)
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

// SetGroup assigns a help group to already-registered commands, so a whole block
// of related commands can be labelled in one call instead of threading the group
// through every registration.
func (m MeshCommandMap) SetGroup(group string, names ...Command) {
	for _, name := range names {
		if cmd, ok := m[name]; ok {
			cmd.Group = group
			m[name] = cmd
		}
	}
}

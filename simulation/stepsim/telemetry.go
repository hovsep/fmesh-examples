package stepsim

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

// PortLines returns a snapshot function reading telemetry from a component's
// output port: one line per signal, as the mesh wrote it.
//
// It is the usual way to get state out of a mesh — some component renders the
// state it wants published onto a port, and this hands those lines to the
// session, which throttles them and forwards them to the sink. Neither end
// knows about the other.
//
//	lines, err := stepsim.PortLines(fm, "publisher", "stream")
//	// ...
//	session.New(eng, session.WithTelemetry(lines), session.WithSink(target))
//
// The port is resolved now rather than per step, so a wrong name is a startup
// error instead of a mesh that silently publishes nothing.
func PortLines(fm *fmesh.FMesh, componentName, portName string) (func() []string, error) {
	c := fm.ComponentByName(componentName)
	if c == nil {
		return nil, fmt.Errorf("cannot publish: no component %q in the mesh", componentName)
	}
	if c.OutputByName(portName) == nil {
		return nil, fmt.Errorf("cannot publish: component %q has no output port %q", componentName, portName)
	}

	return func() []string {
		// Read after the run and before the next one clears the port, so this
		// is exactly what the last step produced.
		signals := c.OutputByName(portName).Signals()

		lines := make([]string, 0, signals.Len())
		_ = signals.ForEach(func(sig *signal.Signal) error {
			lines = append(lines, fmt.Sprint(sig.PayloadOrNil()))
			return nil
		})
		return lines
	}, nil
}

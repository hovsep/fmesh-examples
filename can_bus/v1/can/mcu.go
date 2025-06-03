package can

import "github.com/hovsep/fmesh/component"

// NewMCU creates a microcontroller unit
// which executes embedded logic operating on frames level
func NewMCU(name string, initState func(state component.State), logic component.ActivationFunc) *component.Component {
	return component.New("mcu-" + name).
		WithInitialState(initState).
		WithActivationFunc(logic).
		WithInputs(PortCANRx). // Frame in
		WithOutputs(PortCANTx) // Frame out

}

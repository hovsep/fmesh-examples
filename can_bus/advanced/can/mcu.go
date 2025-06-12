package can

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
)

// NewMCU creates a microcontroller unit
// which executes embedded logic operating on frames level
func NewMCU(name string, initState func(state component.State), logic component.ActivationFunc) *component.Component {
	return component.New("mcu-" + name).
		WithInputs(common.PortCANRx).  // Frame in
		WithOutputs(common.PortCANTx). // Frame out
		WithInitialState(initState).
		WithActivationFunc(logic)

}

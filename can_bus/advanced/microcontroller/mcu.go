package microcontroller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
)

// New creates a microcontroller unit component
func New(name string, initState func(state component.State), af component.ActivationFunc) *component.Component {
	c, err := component.New("mcu-"+name,
		component.WithInputs(common.PortCANRx),  // Frame in
		component.WithOutputs(common.PortCANTx), // Frame out
		component.WithInitialState(initState),
		component.WithActivationFunc(af),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create MCU: %v", err))
	}
	return c
}

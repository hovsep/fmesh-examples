package diagnostics

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/obd"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

type Laptop struct {
	laptopComponent *component.Component
}

const (
	portUSBIn          = "usb_in"
	portUSBOut         = "usb_out"
	portProgrammaticIn = "pr_in"
	labelTo            = "send_to"
	labelUSB           = "usb"
)

func NewLaptop(name string) *Laptop {
	return &Laptop{
		laptopComponent: component.New(name).
			AddInputs(portUSBIn, portProgrammaticIn).
			AddOutputs(portUSBOut).
			WithActivationFunc(func(this *component.Component) error {

				// Process programmatic commands
				this.InputByName(portProgrammaticIn).Signals().ForEach(func(sig *signal.Signal) error {
					// Handle signals routed to usb port
					if sig.Labels().ValueIs(labelTo, labelUSB) {
						this.OutputByName(portUSBOut).PutSignals(sig)
					}
					return nil
				})

				// Process incoming usb data
				this.InputByName(portUSBIn).Signals().ForEach(func(sig *signal.Signal) error {
					// Just print everything to STDOUT
					this.Logger().Printf("Got data on USB port: %v", sig.PayloadOrNil())
					return nil
				})

				return nil
			}),
	}
}

func (l *Laptop) SendDataToUSB(payloads ...any) {
	l.laptopComponent.InputByName(portProgrammaticIn).
		PutSignalGroups(
			signal.NewGroup(payloads...).ForEach(func(sig *signal.Signal) error {
				sig.AddLabel(labelTo, labelUSB)
				return nil
			}),
		)
}

func (l *Laptop) ConnectToOBD(OBDSocket *can.Node) error {
	l.laptopComponent.OutputByName(portUSBOut).PipeTo(OBDSocket.MCU.InputByName(obd.PortOBDIn))
	OBDSocket.MCU.OutputByName(obd.PortOBDOut).PipeTo(l.laptopComponent.InputByName(portUSBIn))
	if l.laptopComponent.HasChainableErr() {
		return l.laptopComponent.ChainableErr()
	}

	if OBDSocket.MCU.HasChainableErr() {
		return OBDSocket.MCU.ChainableErr()
	}

	return nil
}

func (l *Laptop) GetAllComponents() []*component.Component {
	return []*component.Component{
		l.laptopComponent,
	}
}

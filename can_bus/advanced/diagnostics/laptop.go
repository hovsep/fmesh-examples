package diagnostics

import (
	"github.com/hovsep/fmesh-example/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/obd"
	"github.com/hovsep/fmesh/common"
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
			WithInputs(portUSBIn, portProgrammaticIn).
			WithOutputs(portUSBOut).
			WithActivationFunc(func(this *component.Component) error {

				// Process programmatic commands
				for _, sig := range this.InputByName(portProgrammaticIn).AllSignalsOrNil() {
					// Handle signals routed to usb port
					if sig.LabelIs(labelTo, labelUSB) {
						this.OutputByName(portUSBOut).PutSignals(sig)
					}
				}

				// Process incoming usb data
				for _, sig := range this.InputByName(portUSBIn).AllSignalsOrNil() {
					// Just print everything to STDOUT
					this.Logger().Printf("Got data on USB port: %v", sig.PayloadOrNil())
				}

				return nil
			}),
	}
}

func (l *Laptop) SendDataToUSB(payloads ...any) {
	l.laptopComponent.InputByName(portProgrammaticIn).
		PutSignals(
			signal.NewGroup(payloads...).
				WithSignalLabels(common.LabelsCollection{
					labelTo: labelUSB,
				}).
				SignalsOrNil()...,
		)
}

func (l *Laptop) ConnectToOBD(OBDSocket *can.Node) error {
	l.laptopComponent.OutputByName(portUSBOut).PipeTo(OBDSocket.MCU.InputByName(obd.PortOBDIn))
	OBDSocket.MCU.OutputByName(obd.PortOBDOut).PipeTo(l.laptopComponent.InputByName(portUSBIn))
	if l.laptopComponent.HasErr() {
		return l.laptopComponent.Err()
	}

	if OBDSocket.MCU.HasErr() {
		return OBDSocket.MCU.Err()
	}

	return nil
}

func (l *Laptop) GetAllComponents() []*component.Component {
	return []*component.Component{
		l.laptopComponent,
	}
}

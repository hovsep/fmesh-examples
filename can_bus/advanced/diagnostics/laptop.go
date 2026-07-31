package diagnostics

import (
	"context"
	"fmt"

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

func NewLaptop(name string) (*Laptop, error) {
	laptopComponent, err := component.New(name,
		component.WithInputs(portUSBIn, portProgrammaticIn),
		component.WithOutputs(portUSBOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {

			// Process programmatic commands
			this.InputByName(portProgrammaticIn).Signals().ForEachIf(func(sig *signal.Signal) bool {
				return sig.Labels().ValueIs(labelTo, labelUSB)
			}, func(sig *signal.Signal) error {
				return this.OutputByName(portUSBOut).PutSignals(sig)
			})

			// Process incoming usb data
			this.InputByName(portUSBIn).Signals().ForEach(func(sig *signal.Signal) error {
				// Just print everything to STDOUT
				this.Logger().Printf("Got data on USB port: %v", sig.Payload())
				return nil
			})

			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("laptop %s: %w", name, err)
	}

	return &Laptop{
		laptopComponent: laptopComponent,
	}, nil
}

func (l *Laptop) SendDataToUSB(payloads ...any) {
	l.laptopComponent.InputByName(portProgrammaticIn).
		PutSignalGroups(
			signal.NewGroup(payloads...).Map(func(sig *signal.Signal) *signal.Signal {
				return sig.WithLabel(labelTo, labelUSB)
			}),
		)
}

func (l *Laptop) ConnectToOBD(OBDSocket *can.Node) error {
	if err := l.laptopComponent.OutputByName(portUSBOut).PipeTo(OBDSocket.MCU.InputByName(obd.PortOBDIn)); err != nil {
		return fmt.Errorf("failed to pipe laptop to OBD: %w", err)
	}
	if err := OBDSocket.MCU.OutputByName(obd.PortOBDOut).PipeTo(l.laptopComponent.InputByName(portUSBIn)); err != nil {
		return fmt.Errorf("failed to pipe OBD to laptop: %w", err)
	}

	return nil
}

func (l *Laptop) GetAllComponents() []*component.Component {
	return []*component.Component{
		l.laptopComponent,
	}
}

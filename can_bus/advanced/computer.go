package main

import (
	"github.com/hovsep/fmesh/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	portUSBIn          = "usb_in"
	portUSBOut         = "usb_out"
	portProgrammaticIn = "pr_in"
	labelTo            = "send_to"
	labelUSB           = "usb"
)

func NewComputer(name string) *component.Component {
	return component.New(name).
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
		})
}

func sendPayloadToUSBPort(computer *component.Component, payload any) {
	computer.InputByName(portProgrammaticIn).
		PutSignals(signal.New(payload).
			WithLabels(common.LabelsCollection{
				labelTo: labelUSB,
			}))
}

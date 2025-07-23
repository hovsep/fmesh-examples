package main

import (
	"fmt"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
	"os"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/bus"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu"
)

func main() {
	// Create components:
	ptBus := bus.New("PT-CAN")      // Modern vehicles have multiple buses, this one is called "powertrain bus"
	laptop := NewComputer("laptop") // Laptop running diagnostic software and connected to vehicle via OBD socket

	// Build CAN nodes:
	obdDevice := ecu.NewOBD() // putting this into a variable, so we can connect it to the laptop
	allCanNodes := can.Nodes{
		ecu.NewECM(), // Engine Control Module
		//ecu.NewTCM(), // Transmission Control Module
		//ecu.NewACU(), // Airbag Control Unit
		obdDevice, // On Board Diagnostics
	}

	mm := component.New("mm").
		WithInputs("ctl_state", "current_bus_l", "current_bus_h", "self_activation").
		WithOutputs("bus_trigger", "self_activation").
		WithInitialState(func(state component.State) {
			state.Set("ctl_states", make(map[string]string))
			state.Set("bus_idle_count", 0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			ctlStates := this.State().Get("ctl_states").(map[string]string)
			busIdleCnt := this.State().Get("bus_idle_count").(int)

			if this.InputByName("ctl_state").HasSignals() {

				for _, sig := range this.InputByName("ctl_state").AllSignalsOrNil() {
					data := sig.PayloadOrDefault(nil).(map[string]string)
					if data != nil {
						for ctlName, ctlState := range data {
							ctlStates[ctlName] = ctlState
						}
					}
				}

				this.State().Set("ctl_states", ctlStates)
			}

			currentL := this.InputByName("current_bus_l").FirstSignalPayloadOrDefault(bus.NoVoltage).(bus.Voltage)
			currentH := this.InputByName("current_bus_h").FirstSignalPayloadOrDefault(bus.NoVoltage).(bus.Voltage)

			waiterExists := false
			for _, ctlState := range ctlStates {
				if ctlState != "IDLE" {
					waiterExists = true
					break
				}
			}

			if currentL+currentH == 0 {
				this.State().Set("bus_idle_count", busIdleCnt+1)
			} else {
				this.State().Set("bus_idle_count", 0)
			}

			if waiterExists && busIdleCnt > 3 {
				this.State().Set("bus_idle_count", 0)
				this.Logger().Println("BUS TRIGGERED")
				this.OutputByName("bus_trigger").PutSignals(signal.New(1))
				return nil
			}

			if !waiterExists && busIdleCnt > 11 {
				this.Logger().Println("Looks like no ctl waiting")
				return nil
			}

			this.OutputByName("self_activation").PutSignals(signal.New(true))
			return nil
		})

	mm.OutputByName("self_activation").PipeTo(mm.InputByName("self_activation"))

	allCanNodes.ConnectToBus(ptBus, mm)

	// Connect usb-obd cable:
	laptop.OutputByName(portUSBOut).PipeTo(obdDevice.MCU.InputByName(ecu.PortOBDIn))
	obdDevice.MCU.OutputByName(ecu.PortOBDOut).PipeTo(laptop.InputByName(portUSBIn))

	// Build the mesh
	fm := fmesh.NewWithConfig("can_bus_sim_v1", &fmesh.Config{
		ErrorHandlingStrategy: fmesh.StopOnFirstErrorOrPanic,
		Debug:                 false,
	}).
		WithComponents(laptop, ptBus, mm).
		WithComponents(allCanNodes.GetAllComponents()...)

	// Initialize the mesh:
	// send some data through laptop into OBD socket
	sendPayloadToUSBPort(laptop, diagnosticFrameGetSpeed)
	sendPayloadToUSBPort(laptop, diagnosticFrameGetRPM)

	runResult, err := fm.Run()
	if err != nil {
		fmt.Println("The mesh finished with error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles", runResult.Cycles.Len())
}

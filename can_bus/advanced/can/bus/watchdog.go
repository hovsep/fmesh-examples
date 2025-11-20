package bus

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/controller"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/physical"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	stateKeyControllerStates   = "ctl_states"      // Set of all received states
	stateKeyObservedIdleCycles = "bus_idle_cycles" // For how many cycles the bus was passive-idle (not producing a pair of voltages)
	triggerBusAfterIdleCycles  = 3 + 1             // If the bus is idle for 4 (signal propagation distance from bus to MCU in cycles + 1) consecutive cycles, request a recessive bit (simulates passive resistors pulling the lines)
	stopBusAfterIdleCycles     = 11 + 1            // If the bus remains idle for 12 (11 signals are needed for controller to start arbitration) consecutive cycles, stop watchdog self-activation to allow the bus to stop naturally)
)

func newWatchdog(name string) *component.Component {
	watchdog := component.New(name).
		WithDescription("Simulates terminal resistors and halts the bus when all nodes are idle").
		AddInputs(
			common.PortControllerState, // Each CAN node sends it's current state here
			common.PortCANL,            // Current voltage on bus low wire
			common.PortCANH,            // Current voltage on bus high wire
			common.PortSelfActivation,  // Non-parametrized (dummy) feedback-loop, so watchdog is activated in each cycle once fired
		).
		AddOutputs(
			portRecessiveBitRequest, // Control signal to bus, in order to drive the bus recessive (simulate terminal resistors effect)
			common.PortSelfActivation,
		).
		WithInitialState(func(state component.State) {
			// We will track the state of each controller
			state.Set(stateKeyControllerStates, make(controller.StateMap))

			// Tracking consecutive cycles in which the bus is silent allows us to stop the bus and whole simulation
			state.Set(stateKeyObservedIdleCycles, 0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			ctlStates := this.State().Get(stateKeyControllerStates).(controller.StateMap)
			idleCycleCount := this.State().Get(stateKeyObservedIdleCycles).(int)

			if this.InputByName(common.PortControllerState).HasSignals() {

				this.InputByName(common.PortControllerState).Signals().ForEach(func(sig *signal.Signal) error {
					ctlStateMap := sig.PayloadOrNil().(controller.StateMap)
					ctlStates = ctlStates.MergeFrom(ctlStateMap)
					return nil
				})

				this.State().Set(stateKeyControllerStates, ctlStates)
			}

			currentL := this.InputByName(common.PortCANL).Signals().FirstPayloadOrDefault(physical.NoVoltage).(physical.Voltage)
			currentH := this.InputByName(common.PortCANH).Signals().FirstPayloadOrDefault(physical.NoVoltage).(physical.Voltage)

			allControllersAreIdle := true
			for _, ctlState := range ctlStates {
				if ctlState != controller.StateIdle {
					allControllersAreIdle = false
					break
				}
			}

			if currentL+currentH == physical.NoVoltage {
				this.State().Set(stateKeyObservedIdleCycles, idleCycleCount+1)
			} else {
				this.State().Set(stateKeyObservedIdleCycles, 0)
			}

			if !allControllersAreIdle && idleCycleCount >= triggerBusAfterIdleCycles {
				this.State().Set(stateKeyObservedIdleCycles, 0)
				this.Logger().Printf("The bus is idle for %d consecutive cycles. I will request 1 recessive bit", idleCycleCount)
				this.OutputByName(portRecessiveBitRequest).PutSignals(signal.New(1))
				return nil
			}

			if allControllersAreIdle && idleCycleCount >= stopBusAfterIdleCycles {
				this.Logger().Println("Looks like all controllers are idle. I'm letting the bus to stop naturally")
				return nil
			}

			this.OutputByName(common.PortSelfActivation).PutSignals(signal.New(true))
			return nil
		})

	watchdog.OutputByName(common.PortSelfActivation).PipeTo(watchdog.InputByName(common.PortSelfActivation))

	return watchdog
}

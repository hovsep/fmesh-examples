package helper

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/component"
)

func RunSimulationAndThen(sim *step_sim.Simulation, duration time.Duration, f func()) {
	timeComponent := sim.FM.ComponentByName("time")
	timeComponent.SetupHooks(func(hooks *component.Hooks) {
		hooks.AfterActivation(func(activationContext *component.ActivationContext) error {
			_, simDuration, _, _, err := UnpackTick(activationContext.Component.OutputByName("tick").Signals().First())
			if err != nil {
				return err
			}

			if simDuration >= duration {
				fmt.Println("Sim duration reached:", simDuration)
				go sim.SendCommand(step_sim.Exit)
				return nil
			}
			return nil
		})
	})

	done := make(chan struct{})

	go func() {
		sim.Run()
		close(done)
	}()

	<-done

	f()
}

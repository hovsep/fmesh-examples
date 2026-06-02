package helper

import (
	"fmt"
	"sync"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/component"
)

func RunSimulationAndThen(sim *step_sim.Simulation, duration time.Duration, f func()) {
	// Ensure Exit is sent exactly once: the hook fires on every tick past the
	// threshold, and after the first Exit the Sim stops reading cmdChan, so
	// further sends would block forever and leak goroutines.
	var exitOnce sync.Once

	timeComponent := sim.FM.ComponentByName("time")
	timeComponent.SetupHooks(func(hooks *component.Hooks) {
		hooks.AfterActivation(func(activationContext *component.ActivationContext) error {
			_, simDuration, _, _, err := UnpackTick(activationContext.Component.OutputByName("tick").Signals().First())
			if err != nil {
				return err
			}

			if simDuration >= duration {
				exitOnce.Do(func() {
					fmt.Println("Sim duration reached:", simDuration)
					go sim.SendCommand(step_sim.Exit)
				})
				return nil
			}
			return nil
		})
	})

	done := make(chan struct{})

	go func() {
		defer close(done)
		sim.Run()
	}()

	<-done

	f()
}

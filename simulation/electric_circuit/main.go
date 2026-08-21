package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	lightBulbPowerConsumption      = 22
	lightBulbLuminousFlux          = 6000
	lightBulbWarmingPerCycle       = 0.13
	lightbulbOverheatingThreshold  = 30
	lightbulbMaxWorkingTemperature = 50
	lightbulbOverheatDegradation   = 0.7
)

func main() {
	fmt.Println("=== Electric Circuit Simulation ===")
	fmt.Println("Architecture: Battery powers a Lightbulb via a supply/demand feedback loop.")
	fmt.Println("Each cycle: lightbulb demands power -> battery supplies what it can -> lightbulb converts power to light (heating up).")
	fmt.Println("If temperature exceeds the max working temperature, the bulb burns out.")

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	fm.ComponentByName("lightbulb").InputByName("start_power_demand").PutSignals(signal.New("start"))

	runResult, err := fm.Run(context.Background())
	if err != nil {
		fmt.Println("Simulation failed with error:", err)
		return
	}

	batteryFinalLevel := fm.ComponentByName("battery").State().Get("level").(int)
	bulbTemperature := fm.ComponentByName("lightbulb").State().Get("temperature").(float64)
	fmt.Println()
	fmt.Printf("Simulation finished after %d cycles\n", runResult.Cycles.Len())
	fmt.Printf("Final battery level: %d, bulb temperature: %.1f°C\n", batteryFinalLevel, bulbTemperature)
	if bulbTemperature > lightbulbMaxWorkingTemperature {
		fmt.Println("Outcome: The lightbulb burned out from overheating.")
	} else if batteryFinalLevel == 0 {
		fmt.Println("Outcome: The battery died before the bulb could burn out.")
	} else {
		fmt.Println("Outcome: Simulation completed with both components still operational.")
	}
	fmt.Println("=== Simulation Complete ===")
}

func getMesh() (*fmesh.FMesh, error) {
	battery, err := component.New("battery",
		component.WithDescription("electric battery with initial charge level"),
		component.WithInputs("power_demand"),
		component.WithOutputs("power_supply"),
		component.WithInitialState(func(state component.State) {
			state.Set("level", 1000)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			level := this.State().Get("level").(int)
			defer func() { this.State().Set("level", level) }()

			if this.InputByName("power_demand").HasSignals() {
				demandedCurrent := this.InputByName("power_demand").Signals().FirstPayloadOrDefault(0)
				suppliedCurrent := min(level, demandedCurrent)
				if suppliedCurrent > 0 {
					this.OutputByName("power_supply").PutSignals(signal.New(suppliedCurrent))
					if suppliedCurrent < demandedCurrent {
						this.Logger().Println("LOW BATTERY")
					}
					level = max(0, level-suppliedCurrent)
				} else {
					this.Logger().Println("BATTERY DIED")
				}
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("battery component: %w", err)
	}

	lightbulb, err := component.New("lightbulb",
		component.WithDescription("electric lightbulb"),
		component.WithInputs("power_supply", "start_power_demand"),
		component.WithOutputs("light_supply", "power_demand"),
		component.WithInitialState(func(state component.State) {
			state.Set("temperature", 26.0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			temperature := this.State().Get("temperature").(float64)
			defer func() { this.State().Set("temperature", temperature) }()

			if !this.InputByName("start_power_demand").HasSignals() {
				inputPower := this.InputByName("power_supply").Signals().FirstPayloadOrDefault(0)

				if inputPower >= lightBulbPowerConsumption {
					lightEmission := lightBulbLuminousFlux / temperature * 100
					if temperature >= lightbulbOverheatingThreshold {
						this.Logger().Println("OVERHEATING. LIGHT EMISSION WILL SIGNIFICANTLY DEGRADE")
						lightEmission *= lightbulbOverheatDegradation
					}
					this.OutputByName("light_supply").PutSignals(signal.New(lightEmission))
					temperature += lightBulbWarmingPerCycle
					if temperature > lightbulbMaxWorkingTemperature {
						this.Logger().Println("BURNOUT")
						return nil
					}
				} else {
					this.Logger().Println("POWER STARVATION")
				}
			}

			this.OutputByName("power_demand").PutSignals(signal.New(lightBulbPowerConsumption))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("lightbulb component: %w", err)
	}

	if err := battery.OutputByName("power_supply").PipeTo(lightbulb.InputByName("power_supply")); err != nil {
		return nil, fmt.Errorf("pipe battery→lightbulb: %w", err)
	}
	if err := lightbulb.OutputByName("power_demand").PipeTo(battery.InputByName("power_demand")); err != nil {
		return nil, fmt.Errorf("pipe lightbulb→battery: %w", err)
	}

	fm, err := fmesh.New("battery_and_lightbulb",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
		fmesh.WithDescription("simple electric simulation"),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if err := fm.AddComponents(battery, lightbulb); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	return fm, nil
}

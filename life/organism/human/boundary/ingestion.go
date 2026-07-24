package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Scalars on the intake_intent signal produced by controller:intake.
const (
	ScalarWaterMl  = "water_ml"
	ScalarFoodKcal = "food_kcal"
	ScalarToxin    = "toxin"
)

// GetIngestion returns the boundary between the outside world and the gut.
//
// It translates an intent to swallow into physiological loads. Nothing is
// absorbed here: what arrives lands in the stomach, and da:gi_tract decides how
// fast it makes it into the body.
func GetIngestion() (*component.Component, error) {
	c, err := component.New("boundary:ingestion",
		component.WithDescription("Transforms intake signals (food, water, substances) into physiological ingestion and absorption signals"),
		component.WithInputs(
			common.TimePort,
			"intake_intent",   // from controller:intake
			"food_properties", // optional: temperature, type, calories
		),
		component.WithOutputs(
			"nutrient_load",  // to the GI tract
			"hydration_load", // to the GI tract
			"substance_load", // medicine, toxins
		),
		component.WithActivationFunc(handleIngestion),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:ingestion: %w", err)
	}
	return c, nil
}

func handleIngestion(this *component.Component) error {
	in := this.InputByName("intake_intent")
	if !in.HasSignals() {
		return nil
	}

	return in.Signals().ForEach(func(sig *signal.Signal) error {
		water := sig.Scalars().ValueOrDefault(ScalarWaterMl, 0)
		food := sig.Scalars().ValueOrDefault(ScalarFoodKcal, 0)

		if water > 0 {
			if err := this.OutputByName("hydration_load").PutSignals(
				signal.New(water).
					WithLabel("category", "ingestion").
					WithScalar(common.WaterMl, water),
			); err != nil {
				return err
			}
		}

		if food > 0 {
			// Food arrives with a volume as well as an energy: a meal is also
			// something to digest, and it is the volume that makes a stomach full.
			if err := this.OutputByName("nutrient_load").PutSignals(
				signal.New(food).
					WithLabel("category", "ingestion").
					WithScalar(common.GlucoseKcal, food).
					WithScalar(common.WaterMl, food*mealWaterMlPerKcal),
			); err != nil {
				return err
			}
		}

		// Inhaled toxins (cigarette smoke) are not swallowed; they leave here as a
		// substance load that the lungs pick up as damage.
		toxin := sig.Scalars().ValueOrDefault(ScalarToxin, 0)
		if toxin > 0 {
			if err := this.OutputByName("substance_load").PutSignals(
				signal.New(toxin).
					WithLabel("category", "ingestion").
					WithScalar(ScalarToxin, toxin),
			); err != nil {
				return err
			}
		}

		if water == 0 && food == 0 && toxin == 0 {
			this.Logger().Println("ingestion intent carried nothing to swallow")
		}
		return nil
	})
}

// mealWaterMlPerKcal is how much fluid comes along with food. Roughly a
// millilitre per kilocalorie, which is about right for ordinary meals.
const mealWaterMlPerKcal = 1.0

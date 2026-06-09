package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

func GetIngestion() (*component.Component, error) {
	c, err := component.New("boundary:ingestion",
		component.WithDescription("Transforms intake signals (food, water, substances) into physiological ingestion and absorption signals"),
		component.WithInputs(
			"time",
			"intake_intent",   // from IntakeController
			"food_properties", // optional: temperature, type, calories
		),
		component.WithOutputs(
			"nutrient_load",  // to GI tract
			"hydration_load", // to kidneys/blood
			"substance_load", // medicine, toxins
		),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:ingestion: %w", err)
	}
	return c, nil
}

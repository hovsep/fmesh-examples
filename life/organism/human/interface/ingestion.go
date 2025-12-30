package controller

import "github.com/hovsep/fmesh/component"

func GetIngestionInterface() *component.Component {
	return component.New("ingestion_interface").
		WithDescription("Transforms intake signals (food, water, substances) into physiological ingestion and absorption signals").
		AddInputs(
			"time",
			"intake_intent",   // from IntakeController
			"food_properties", // optional: temperature, type, calories
		).
		AddOutputs(
			"nutrient_load",  // to GI tract
			"hydration_load", // to kidneys/blood
			"substance_load", // medicine, toxins
		).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

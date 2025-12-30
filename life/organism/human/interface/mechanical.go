package controller

import "github.com/hovsep/fmesh/component"

func GetMechanicalInterface() *component.Component {
	return component.New("mechanical_interface").
		WithDescription("Transforms mechanical stimuli (loads, movement, posture) into signals for musculoskeletal and cardiovascular systems").
		AddInputs(
			"time",
			"physical_activity", // operator command (e.g., exercise intensity)
			"external_forces",   // habitat/environment forces, if any
		).
		AddOutputs(
			"muscle_load",     // to muscles
			"skeletal_stress", // to skeletal system
			"cardio_load",     // to heart/circulation
		).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

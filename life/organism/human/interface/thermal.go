package controller

import "github.com/hovsep/fmesh/component"

func GetThermalInterface() *component.Component {
	return component.New("thermal_interface").
		WithDescription("Transforms environmental thermal signals into body heat load, cold/heat stress signals").
		AddInputs(
			"time",
			"ambient_temperature",
			"ambient_humidity",
			"radiation", // sun UV / IR exposure
		).
		AddOutputs(
			"heat_load", // to skin and cardiovascular system
			"cold_load", // to skin and shivering reflex
		).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

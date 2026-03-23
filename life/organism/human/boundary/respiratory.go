package boundary

import "github.com/hovsep/fmesh/component"

func GetRespiratory() *component.Component {
	return component.New("boundary:respiratory").
		WithDescription("Transforms environmental gas signals into chemical levels and lung input for circulation").
		AddInputs(
			"time",
			"gas_composition", // O2, CO2, pollutants
			"gas_humidity",
			"gas_temperature",
		).
		AddOutputs(
			"inspired_gas", // to lungs
		).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}

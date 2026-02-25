package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

const (
	Sympathetic             common.Label = "sympathetic"
	Parasympathetic         common.Label = "parasympathetic"
	Noise                   common.Label = "noise"
	Gain                    common.Label = "gain"
	RegionalBiasCardiac     common.Label = "regional_bias:cardiac"
	RegionalBiasVascular    common.Label = "regional_bias:vascular"
	RegionalBiasRespiratory common.Label = "regional_bias:respiratory"
	RegionalBiasGI          common.Label = "regional_bias:gi"
)

// NewLevel builds a signal that represents a level
func NewLevel(value float64, axis string) *signal.Signal {
	return signal.New(signal.NewGroup().Add(
		signal.New(value).AddLabel(common.Type, common.Level).AddLabel(common.Axis, axis),
	))
}

// NewAutonomicTone builds a signal that represents autonomic tone
func NewAutonomicTone(sym, paraSym, noise, gain, cardiacBias, vascularBias, respiratoryBias, giBias float64) *signal.Signal {
	return signal.New(signal.NewGroup().Add(
		NewLevel(sym, Sympathetic),
		NewLevel(paraSym, Parasympathetic),
		NewLevel(noise, Noise),
		NewLevel(gain, Gain),
		NewLevel(cardiacBias, RegionalBiasCardiac),
		NewLevel(vascularBias, RegionalBiasVascular),
		NewLevel(respiratoryBias, RegionalBiasRespiratory),
		NewLevel(giBias, RegionalBiasGI),
	))
}

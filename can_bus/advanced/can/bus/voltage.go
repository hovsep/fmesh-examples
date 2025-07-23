package bus

import "github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"

type Voltage float64

const (
	NoVoltage           = Voltage(0.0)
	DominantHighVoltage = Voltage(3.7)
	DominantLowVoltage  = Voltage(1.5)

	RecessiveVoltage = Voltage(2.5)
)

// VoltageToBit converts voltages to bit
func VoltageToBit(vLow, vHigh Voltage) codec.Bit {
	// TODO: make it less strict, use thresholds instead of exact matching
	if vLow == DominantLowVoltage && vHigh == DominantHighVoltage {
		return codec.ProtocolDominantBit
	}

	return codec.ProtocolRecessiveBit
}

package can

type Voltage float64

const (
	DominantHighVoltage = Voltage(3.7)
	DominantLowVoltage  = Voltage(1.5)

	RecessiveVoltage = Voltage(2.5)
)

// voltageToBit converts voltages to bit
func voltageToBit(vLow, vHigh Voltage) Bit {
	// TODO: make it less strict, use thresholds instead of exact matching
	if vLow == DominantLowVoltage && vHigh == DominantHighVoltage {
		return ProtocolDominantBit
	}

	return ProtocolRecessiveBit
}

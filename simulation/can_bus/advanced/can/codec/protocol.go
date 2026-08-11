package codec

const (
	ProtocolDominantBit  = Bit(false)
	ProtocolRecessiveBit = Bit(true)

	ProtocolBitStuffingStep = 5

	ProtocolMaxDataBytes = 8
	ProtocolMaxID        = 0x7FF // 11-bit max

	ProtocolSOFSize = 1
	ProtocolIDSize  = 11
	ProtocolDLCSize = 4
	ProtocolEOFSize = 7
	ProtocolIFSSize = 3
)

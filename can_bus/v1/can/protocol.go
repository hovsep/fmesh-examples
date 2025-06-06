package can

const (
	protocolBitStuffingStep = 5
	protocolMaxDataLen      = 8     // In bytes
	protocolMaxFrameID      = 0x7FF // 11-bit max
	protocolIDBitsCount     = 11
	protocolDLCBitsCount    = 4
	protocolDominantBit     = Bit(false)
	protocolRecessiveBit    = Bit(true)
)

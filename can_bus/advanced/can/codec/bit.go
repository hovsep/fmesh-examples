package codec

// Bit represents a single bit (true = 1, false = 0)
type Bit bool

func (bit Bit) String() string {
	if bit {
		return "1"
	}
	return "0"
}

func (bit Bit) IsDominant() bool {
	return bit == ProtocolDominantBit
}

func (bit Bit) IsRecessive() bool {
	return bit == ProtocolRecessiveBit
}

func (bit Bit) IsStuffed(previousBits Bits, stuffingStep int) bool {
	if stuffingStep < 1 || stuffingStep > previousBits.Len()+1 {
		return false
	}

	// Need stuffingStep - 1 previous bits for comparison
	count := stuffingStep - 1
	if previousBits.Len() < count {
		return false
	}

	// Check if the last (stuffingStep - 1) bits are equal to this one
	for i := 0; i < count; i++ {
		if previousBits[previousBits.Len()-1-i] != bit {
			return false // not a stuffing sequence
		}
	}

	return true // bit is a stuffed bit
}

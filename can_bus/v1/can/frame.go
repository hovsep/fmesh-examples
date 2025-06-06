package can

const (
	dataLenInBytes = 8
	maxFrameID     = 0x7FF // 11-bit max
)

// Frame represents a simplified CAN frame (no memory optimizations)
type Frame struct {
	// SOF
	Id uint32 // CAN node ID (11 bits)
	// RTR
	// IDE
	// r0
	// r1
	DLC  uint8                // Data length code (4 bits)
	Data [dataLenInBytes]byte // Payload
	// CRC
	// ACK
	// EOF
	// IFS
}

func (frame *Frame) isValid() bool {
	if frame.Id > maxFrameID {
		return false
	}

	if frame.DLC > dataLenInBytes {
		return false
	}

	return true
}

// toBits encodes the CAN frame into a slice of bits (bools)
// Format: 11 bits ID | 4-bit DLC | DLC * 8-bit Data
func (frame *Frame) toBits() Bits {
	var bits Bits

	// 1. Encode 11-bit CAN ID (MSB first)
	for i := 10; i >= 0; i-- {
		bits = append(bits, (frame.Id>>i)&1 == 1)
	}

	// 2. Encode 4-bit DLC (Data Length Code)
	for i := 3; i >= 0; i-- {
		bits = append(bits, (frame.DLC>>i)&1 == 1)
	}

	// 3. Encode each data byte (DLC * 8 bits)
	for i := 0; i < int(frame.DLC); i++ {
		b := frame.Data[i]
		for j := 7; j >= 0; j-- {
			bits = append(bits, (b>>j)&1 == 1)
		}
	}

	return bits
}

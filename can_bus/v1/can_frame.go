package main

// CanFrame represents a simplified CAN frame
type CanFrame struct {
	// SOF
	Id uint32 // CAN node ID (11 bits)
	// RTR
	// IDE
	// r0
	// r1
	DLC  uint8   // Data length code (4 bits)
	Data [8]byte // Payload
	// CRC
	// ACK
	// EOF
	// IFS
}

var (
	// Some pre-defined frames
	startEngine = &CanFrame{
		Id:   0x100,
		DLC:  1,
		Data: [8]byte{0x01},
	}
)

func (frame *CanFrame) isValid() bool {
	// Check that ID is 11 bits max
	if frame.Id > 0x7FF {
		return false
	}

	// Check that DLC is between 0 and 8
	if frame.DLC > 8 {
		return false
	}

	return true
}

// toBits encodes the CAN frame into a slice of bits (bools)
// Format: 11 bits ID | 4 bits DLC | DLC * 8 bits Data
func (frame *CanFrame) toBits() Bits {
	var bits []bool

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

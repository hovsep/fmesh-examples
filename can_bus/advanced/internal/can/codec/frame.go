package codec

import (
	"errors"
	"fmt"
)

// Frame represents a simplified CAN frame (no memory optimizations)
type Frame struct {
	// SOF
	Id uint32 // CAN node ID (11 bits)
	// RTR
	// IDE
	// r0
	// r1
	DLC  uint8                      // Data length code (4 bits)
	Data [ProtocolMaxDataBytes]byte // Payload
	// CRC
	// ACK
	// EOF
	// IFS
}

func (frame *Frame) IsValid() bool {
	if frame.Id > ProtocolMaxID {
		return false
	}

	if frame.DLC > ProtocolMaxDataBytes {
		return false
	}

	return true
}

// ToBits encodes the CAN frame into a slice of bits
// Format: 1 bit SOF| 11 bits ID | 4-bit DLC | DLC * 8-bit Data
func (frame *Frame) ToBits() Bits {
	var bits Bits

	// SOF (Start Of the Frame)
	bits = append(bits, ProtocolDominantBit)

	// Encode 11-bit CAN ID (MSB first)
	for i := 10; i >= 0; i-- {
		idBit := Bit((frame.Id>>i)&1 == 1)
		bits = append(bits, idBit)
	}

	// Encode 4-bit DLC (Data Length Code)
	for i := 3; i >= 0; i-- {
		dlcBit := Bit((frame.DLC>>i)&1 == 1)
		bits = append(bits, dlcBit)
	}

	// Encode each data byte (DLC * 8 bits)
	for i := 0; i < int(frame.DLC); i++ {
		b := frame.Data[i]
		for j := 7; j >= 0; j-- {
			dataBit := Bit((b>>j)&1 == 1)
			bits = append(bits, dataBit)
		}
	}

	return bits.WithStuffing(ProtocolBitStuffingStep).WithEOF()
}

// FromBits decodes a CAN frame from a Bits slice
func FromBits(bits Bits) (*Frame, error) {
	if len(bits) < ProtocolIDSize+ProtocolDLCSize {
		return nil, errors.New("bit slice too short to contain a valid CAN frame")
	}

	// 1. Decode ID
	var id uint32
	for i := 0; i < ProtocolIDSize; i++ {
		if bits[i] {
			id |= 1 << (ProtocolIDSize - 1 - i)
		}
	}

	// 2. Decode DLC
	var dlc uint8
	for i := 0; i < ProtocolDLCSize; i++ {
		if bits[ProtocolIDSize+i] {
			dlc |= 1 << (ProtocolDLCSize - 1 - i)
		}
	}

	// 3. Validate DLC
	if dlc > ProtocolMaxDataBytes {
		return nil, fmt.Errorf("invalid DLC: %d", dlc)
	}

	// 4. Decode Data
	expectedBits := ProtocolIDSize + ProtocolDLCSize + int(dlc)*8
	if len(bits) < expectedBits {
		return nil, fmt.Errorf("not enough bits for data, expected %d, got %d", expectedBits, len(bits))
	}

	var data [ProtocolMaxDataBytes]byte
	offset := ProtocolIDSize + ProtocolDLCSize
	for i := 0; i < int(dlc); i++ {
		var b byte
		for j := 0; j < 8; j++ {
			if bits[offset+i*8+j] {
				b |= 1 << (7 - j)
			}
		}
		data[i] = b
	}

	frame := &Frame{
		Id:   id,
		DLC:  dlc,
		Data: data,
	}

	if !frame.IsValid() {
		return nil, errors.New("decoded frame is invalid")
	}

	return frame, nil
}

func (frame *Frame) String() string {
	frameBits := frame.ToBits()
	frameBitsUnstuffed := frameBits.WithoutStuffing(ProtocolBitStuffingStep)

	ranges := struct {
		sof  [2]byte
		id   [2]byte
		dlc  [2]byte
		data [2]byte
		eof  [2]byte
	}{
		sof: [2]byte{0, ProtocolSOFSize},
	}

	ranges.id = [2]byte{ranges.sof[1], ranges.sof[1] + ProtocolIDSize}
	ranges.dlc = [2]byte{ranges.id[1], ranges.id[1] + ProtocolDLCSize}
	ranges.data = [2]byte{ranges.dlc[1], ranges.dlc[1] + frame.DLC*8}
	ranges.eof = [2]byte{ranges.data[1], ranges.data[1] + ProtocolEOFSize}

	return fmt.Sprintf("\n \n %#v "+
		"\n SOF: %s"+
		"\n ID: %s"+
		"\n DLC: %s"+
		"\n Data: %s"+
		"\n EOF: %s"+
		"\n unstuffed: %s"+
		"\n raw: %s \n \n",
		frame,
		frameBitsUnstuffed[ranges.sof[0]:ranges.sof[1]],
		frameBitsUnstuffed[ranges.id[0]:ranges.id[1]],
		frameBitsUnstuffed[ranges.dlc[0]:ranges.dlc[1]],
		frameBitsUnstuffed[ranges.data[0]:ranges.data[1]],
		frameBitsUnstuffed[ranges.eof[0]:ranges.eof[1]],
		frameBitsUnstuffed,
		frameBits)
}

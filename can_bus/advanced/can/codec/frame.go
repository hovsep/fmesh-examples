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

// ISOTPMessage represents a simplified iso-15765 message (transport protocol over CAN-frames)
// for simplicity we support only single-frame messages
type ISOTPMessage struct {
	Len       uint8
	ServiceID uint8
	PID       uint8 // Parameter ID
	Data      []byte
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

func (frame *Frame) ToISOTPMessage() (*ISOTPMessage, error) {
	if frame.DLC == 0 {
		return nil, errors.New("frame has zero DLC")
	}

	pci := frame.Data[0]
	frameType := pci >> 4
	if frameType != 0 {
		return nil, fmt.Errorf("unsupported ISO-TP frame type: %d (only single frame supported)", frameType)
	}

	payloadLen := pci & 0x0F
	if payloadLen == 0 || payloadLen > 7 {
		return nil, fmt.Errorf("invalid single frame payload length: %d", payloadLen)
	}

	if int(payloadLen)+1 > int(frame.DLC) {
		return nil, fmt.Errorf("frame DLC (%d) less than expected payload size (%d)", frame.DLC, payloadLen+1)
	}

	// payload bytes start from Data[1]
	if payloadLen < 2 {
		return nil, fmt.Errorf("payload too short to contain service and PID, length: %d", payloadLen)
	}

	serviceID := frame.Data[1]
	pid := frame.Data[2]

	return &ISOTPMessage{
		Len:       payloadLen,
		ServiceID: serviceID,
		PID:       pid,
	}, nil
}

func FromISOTPMessage(msg *ISOTPMessage, id uint32) (*Frame, error) {
	if msg == nil {
		return nil, errors.New("nil ISOTPMessage")
	}

	baseLen := 2 // ServiceID + PID
	totalLen := baseLen + len(msg.Data)

	if totalLen > 7 {
		return nil, fmt.Errorf("payload length too long for single-frame ISO-TP (max 7), got %d", totalLen)
	}

	frame := &Frame{
		Id:  id,
		DLC: 8, // Full CAN frame length
	}

	// PCI byte: single frame (0x0) + length of payload
	frame.Data[0] = byte(0x0<<4) | byte(totalLen)

	// Service ID and PID
	frame.Data[1] = msg.ServiceID
	frame.Data[2] = msg.PID

	// Additional data
	copy(frame.Data[3:], msg.Data)

	return frame, nil
}

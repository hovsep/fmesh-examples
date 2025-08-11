package microcontroller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/codec"
)

// ISOTPMessage represents a simplified iso-15765 message (transport protocol over CAN-frames)
// for simplicity we support only single-frame messages
type ISOTPMessage struct {
	ServiceID ServiceID
	PID       ParameterID // Parameter ID
	Data      []byte
}

const (
	ValidISOTPFrameDLC   = 8
	MaxBytesInISOTPFrame = 5
)

func NewISOTPMessage() *ISOTPMessage {
	return &ISOTPMessage{}
}

func (msg *ISOTPMessage) FromCANFrame(frame *codec.Frame) (*ISOTPMessage, error) {
	if frame.DLC == 0 {
		return nil, errors.New("frame has zero DLC")
	}

	if frame.DLC != ValidISOTPFrameDLC {
		return nil, errors.New("given frame is not valid ISO-TP frame")
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

	dataLen := int(payloadLen) - 2
	if dataLen < 0 {
		return nil, fmt.Errorf("payload length too short for ServiceID and PID: %d", payloadLen)
	}

	dataBytes := make([]byte, dataLen)
	copy(dataBytes, frame.Data[3:3+dataLen])

	return &ISOTPMessage{
		ServiceID: ServiceID(serviceID),
		PID:       ParameterID(pid),
		Data:      dataBytes,
	}, nil
}

func (msg *ISOTPMessage) ToCANFrame(id uint32) (*codec.Frame, error) {
	if msg == nil {
		return nil, errors.New("nil ISOTPMessage")
	}

	baseLen := 2 // ServiceID + PID
	totalLen := baseLen + len(msg.Data)

	if totalLen > 7 {
		return nil, fmt.Errorf("payload length too long for single-frame ISO-TP (max 7), got %d", totalLen)
	}

	frame := &codec.Frame{
		Id:  id,
		DLC: 8, // Full CAN frame length
	}

	// PCI byte: single frame (0x0) + length of payload
	frame.Data[0] = byte(0x0<<4) | byte(totalLen)

	// Service ID and PID
	frame.Data[1] = byte(msg.ServiceID)
	frame.Data[2] = byte(msg.PID)

	// Additional data
	copy(frame.Data[3:], msg.Data)

	return frame, nil
}

// FitDataIntoSingleFrame is a helper function which allows to truncate data exceeding 1 frame
func FitDataIntoSingleFrame(data []byte) []byte {
	return data[:MaxBytesInISOTPFrame]
}

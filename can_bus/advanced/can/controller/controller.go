package controller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"

	"github.com/hovsep/fmesh/component"
)

const (
	stateKeyTxQueue                          = "tx_queue"
	stateKeyRxBuffer                         = "rx_buffer"
	stateKeyControllerState                  = "controller_state"
	stateKeyConsecutiveRecessiveBitsObserved = "consecutive_recessive_observed"
)

// New creates a stateful CAN controller
// which converts frames to bits and vice versa
func New(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInputs(common.PortCANTx, common.PortCANRx).  // Frame in, bits in
		WithOutputs(common.PortCANTx, common.PortCANRx). // Bits out, frame out
		WithInitialState(func(state component.State) {
			state.Set(stateKeyTxQueue, TxQueue{})         // Multiple bit buffers we are trying to send to transceiver
			state.Set(stateKeyRxBuffer, codec.NewBits(0)) // Single bit buffer we are trying to build from bits coming from transceiver
			state.Set(stateKeyControllerState, stateIdle)
			state.Set(stateKeyConsecutiveRecessiveBitsObserved, 0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			err := handleIncomingFrames(this)
			if err != nil {
				return fmt.Errorf("failed to handle incoming frames: %w", err)
			}

			// Get current bit set on the bus
			currentBit, err := getCurrentBit(this)
			if err != nil {
				return fmt.Errorf("failed to determine current bit on the bus: %w", err)
			}

			// Observe consecutive recessive bits to detect special situations on BUS (like SOF or EOF)
			err = trackConsecutiveBits(this, currentBit)
			if err != nil {
				return fmt.Errorf("failed to track consecutive bits: %w", err)
			}

			// Run the main state machine:
			return runStateMachine(this, currentBit)
		})
}

// Enqueue new frames coming from MCU
func handleIncomingFrames(this *component.Component) error {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
	defer func() {
		this.State().Set(stateKeyTxQueue, txQueue)
	}()

	for _, sig := range this.InputByName(common.PortCANTx).AllSignalsOrNil() {
		frame, ok := sig.PayloadOrNil().(*codec.Frame)
		if !ok || !frame.IsValid() {
			return errors.New("received corrupted frame")
		}

		frameBits, lastIDBitIndex := frame.ToBits()

		this.Logger().Println("ID ends at ", lastIDBitIndex)
		txQueue = append(txQueue, &TxQueueItem{
			// Add IFS and 1 extra recessive bit
			Buf:            codec.NewBitBuffer(frameBits.WithIFS().WithBits(codec.ProtocolRecessiveBit)),
			LastIDBitIndex: lastIDBitIndex,
		})
		this.Logger().Printf("got a frame to send: %s, items in tx-queue: %d", frameBits, len(txQueue))
	}
	return nil
}

func getCurrentBit(this *component.Component) (codec.Bit, error) {
	if !this.InputByName(common.PortCANRx).HasSignals() {
		return codec.ProtocolRecessiveBit, errors.New("unable to read current bit, cannot proceed with protocol decision")
	}
	if this.InputByName(common.PortCANRx).Buffer().Len() > 1 {
		return codec.ProtocolRecessiveBit, errors.New("received more than one bit")
	}
	currentBit := this.InputByName(common.PortCANRx).FirstSignalPayloadOrNil().(codec.Bit)
	this.Logger().Println("observing current bit on bus:", currentBit)

	return currentBit, nil
}

func trackConsecutiveBits(this *component.Component, currentBit codec.Bit) error {
	consecutiveRecessiveBitsObserved := this.State().Get(stateKeyConsecutiveRecessiveBitsObserved).(int)

	if currentBit.IsRecessive() {
		consecutiveRecessiveBitsObserved++
	} else {
		consecutiveRecessiveBitsObserved = 0
	}

	this.State().Set(stateKeyConsecutiveRecessiveBitsObserved, consecutiveRecessiveBitsObserved)
	return nil
}

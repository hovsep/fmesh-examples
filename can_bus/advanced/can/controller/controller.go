package controller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/signal"

	"github.com/hovsep/fmesh/component"
)

const (
	stateKeyTxQueue                          = "tx_queue"
	stateKeyRxBuffer                         = "rx_buffer"
	stateKeyControllerState                  = "controller_state"
	stateKeyConsecutiveRecessiveBitsObserved = "consecutive_recessive_observed"
	stateKeyBitsExpected                     = "bits_expected"
)

// New creates a stateful CAN controller
// which converts frames to bits and vice versa
func New(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInputs(common.PortCANTx, common.PortCANRx).           // Frame in, bits in
		WithOutputs(common.PortCANTx, common.PortCANRx, "to-mm"). // Bits out, frame out
		WithInitialState(func(state component.State) {
			state.Set(stateKeyTxQueue, TxQueue{})
			state.Set(stateKeyRxBuffer, codec.NewBits(0))
			state.Set(stateKeyControllerState, stateIdle)
			state.Set(stateKeyConsecutiveRecessiveBitsObserved, 0)
			state.Set(stateKeyBitsExpected, 0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			defer func() {
				this.Logger().Printf("exiting AF of %s", this.Name())
			}()
			err := handleIncomingFrames(this)
			if err != nil {
				return fmt.Errorf("failed to handle incoming frames: %w", err)
			}

			// Get current bit set on the bus
			currentBit, err := getCurrentBit(this)
			if err != nil && err.Error() == "unable to read current bit, cannot proceed with protocol decision" {
				this.Logger().Println("bus died, pinging")
				this.OutputByName("to-mm").PutSignals(signal.New("I see no signals on bus, are you alive ?"))

				return nil
			}
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

		frameBits := frame.ToBits()

		txQueue = append(txQueue, &TxQueueItem{
			// Add IFS and 1 extra recessive bit
			Buf: codec.NewBitBuffer(frameBits.WithIFS().WithBits(codec.ProtocolRecessiveBit)),
		})
		this.Logger().Printf("got a frame to send stuffed: %s, unstuffed: %s, items in tx-queue: %d", frameBits, frameBits.WithoutStuffing(codec.ProtocolBitStuffingStep), len(txQueue))
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
	//this.Logger().Println("observing current bit on bus:", currentBit)

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

func runStateMachine(this *component.Component, currentBit codec.Bit) error {
	currentState := this.State().Get(stateKeyControllerState).(State)
	for {
		this.Logger().Println("current state:", currentState)
		nextState, err := getNextState(this, currentState, currentBit)
		if err != nil {
			return fmt.Errorf("failed to switch from state: %s error: %w", currentState, err)
		}
		if nextState == currentState {
			// No transitions, exit
			return nil
		} else {
			this.Logger().Printf("state transition: %s -> %s", currentState, nextState)
		}
		currentState = nextState
		this.State().Set(stateKeyControllerState, currentState)
	}
	return errors.New("did not manage to exit correctly from main state machine loop")
}

func getNextState(this *component.Component, currentState State, currentBit codec.Bit) (State, error) {
	switch currentState {
	case stateIdle:
		return handleIdleState(this, currentBit)
	case stateWaitForBusIdle:
		return handleWaitForBusIdleState(this)
	case stateArbitration:
		return handleArbitrationState(this, currentBit)

	case stateTransmit:
		return handleTransmitState(this)
	case stateReceive:
		return handleReceiveState(this, currentBit)
	default:
		return currentState, fmt.Errorf("end up in incorrect state: %v", currentState)
	}
}

func handleIdleState(this *component.Component, currentBit codec.Bit) (State, error) {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)

	// SOF detected, became passive listener
	if currentBit.IsDominant() {
		return stateReceive, nil
	}

	if len(txQueue) > 0 {
		return stateWaitForBusIdle, nil
	}

	// passive-idle situation: controller does not want to write, there is nothing to read
	return stateIdle, nil
}

func handleWaitForBusIdleState(this *component.Component) (State, error) {
	consecutiveRecessiveBitsObserved := this.State().Get(stateKeyConsecutiveRecessiveBitsObserved).(int)

	if consecutiveRecessiveBitsObserved > codec.ProtocolEOFSize+codec.ProtocolIFSSize {
		this.Logger().Println("i've seen ", consecutiveRecessiveBitsObserved, " recessives")
		// The bus looks idle. It's time to start transmitting
		return stateArbitration, nil
	}
	// Continue waiting
	this.Logger().Println("waiting for more consecutive recessive bits, seen so far:", consecutiveRecessiveBitsObserved)
	return stateWaitForBusIdle, nil
}

func handleArbitrationState(this *component.Component, currentBit codec.Bit) (State, error) {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
	rxBuf := this.State().Get(stateKeyRxBuffer).(codec.Bits)
	defer func() {
		this.State().Set(stateKeyRxBuffer, rxBuf)
	}()

	txItem := txQueue[0]

	if txItem.Buf.Available() == 0 {
		return stateArbitration, errors.New("already processed buffer is still in tx-queue")
	}

	// Receive own sent bits (and skip anything before we started writing first bit)
	if txItem.Buf.Offset > 0 {
		rxBuf = rxBuf.WithBits(currentBit)
		this.Logger().Println("in arbitration rxBuf:", rxBuf)
	}

	// Check if arbitration is won
	rxUnstuffed := rxBuf.WithoutStuffing(codec.ProtocolBitStuffingStep)
	if rxUnstuffed.Len() == codec.ProtocolIDSize+1 {
		idReceived := rxUnstuffed[1:]
		idTransmitted := txItem.Buf.Bits.WithoutStuffing(codec.ProtocolBitStuffingStep)[1 : codec.ProtocolIDSize+1]
		wonArbitration := idReceived.Equals(idTransmitted)
		if wonArbitration {
			this.Logger().Println("won arbitration with rx:", rxUnstuffed)
			return stateTransmit, nil
		}
	}

	// After first bit is written (as we are checking previous transmitted bit)
	if txItem.Buf.Offset > 1 {
		// Check if arbitration is lost
		if currentBit != txItem.Buf.PreviousBit() {
			// Lost arbitration
			if currentBit.IsDominant() && txItem.Buf.PreviousBit().IsRecessive() {
				this.Logger().Println("lost arbitration. backoff")
			}

			// Or bus error happened
			if currentBit.IsRecessive() && txItem.Buf.PreviousBit().IsDominant() {
				return stateArbitration, errors.New("bus error, recessive bit won arbitration. backoff")
			}

			txItem.Buf.ResetOffset() // Backoff, retry later

			return stateReceive, nil
		}
	}

	txBit := txItem.Buf.NextBit()
	if txItem.Buf.Offset == 0 {
		this.Logger().Println("write SOF bit:", txBit)
	} else {
		this.Logger().Println("write arbitration (ID) bit:", txBit)
	}

	this.OutputByName(common.PortCANTx).PutSignals(signal.New(txBit))
	txItem.Buf.IncreaseOffset()

	return stateArbitration, nil
}

func handleTransmitState(this *component.Component) (State, error) {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
	defer func() {
		this.State().Set(stateKeyTxQueue, txQueue)
	}()

	txItem := txQueue[0]

	if txItem.Buf.Offset <= codec.ProtocolIDSize {
		return stateTransmit, errors.New("must be in arbitration state")
	}

	// Check if we finished transmitting the buffer
	if txItem.Buf.Available() == 0 {
		this.Logger().Println("a buffer is successfully transmitted, remove it from the queue")

		txQueue = txQueue[1:]
		this.State().Set(stateKeyRxBuffer, codec.NewBits(0)) //Clear observed bits
		return stateIdle, nil
	}

	txBit := txItem.Buf.NextBit()
	//this.Logger().Println("write frame bit:", txBit)
	this.OutputByName(common.PortCANTx).PutSignals(signal.New(txBit))
	txItem.Buf.IncreaseOffset()

	return stateTransmit, nil
}

func handleReceiveState(this *component.Component, currentBit codec.Bit) (State, error) {
	rxBuf := this.State().Get(stateKeyRxBuffer).(codec.Bits)
	bitsExpected := this.State().Get(stateKeyBitsExpected).(int)

	if rxBuf.Len() == 0 {
		this.State().Set(stateKeyBitsExpected, 1+codec.ProtocolIDSize+codec.ProtocolDLCSize)
		this.Logger().Println("receiving SOF")
	}

	rxBuf = rxBuf.WithBits(currentBit)
	rxUnstuffed := rxBuf.WithoutStuffing(codec.ProtocolBitStuffingStep)
	this.Logger().Printf("rxBuf stuffed: %s unstuffed %s", rxBuf, rxUnstuffed)

	// All CAN frames begin with 3 fixed-size fields: SOF (1), ID(11) and DLC
	// when we have received enough bits - we decode ID and DLC
	// knowing DLC allows us to know exactly how many data bits we expect
	frameFixedSizePart := codec.ProtocolSOFSize + codec.ProtocolIDSize + codec.ProtocolDLCSize
	if rxUnstuffed.Len() >= frameFixedSizePart && rxUnstuffed.Len() == bitsExpected {

		// Decode ID and DLC
		if bitsExpected == frameFixedSizePart {
			idBits := rxUnstuffed[codec.ProtocolSOFSize : codec.ProtocolSOFSize+codec.ProtocolIDSize]
			dlcBits := rxUnstuffed[codec.ProtocolSOFSize+codec.ProtocolIDSize:]
			expectedBytes := dlcBits.ToInt()
			this.Logger().Println("id: ", idBits, " dlc:", dlcBits, " dlc (bytes):", expectedBytes)

			// We know the DLC, let's expect data bits
			this.State().Set(stateKeyBitsExpected, 16+expectedBytes*8+codec.ProtocolEOFSize)
		} else {
			// Decode data
			this.Logger().Println("received all expected data and EOF")
			// Check for valid EOF
			firstEOFbitIndex := rxUnstuffed.Len() - codec.ProtocolEOFSize
			if !rxUnstuffed[firstEOFbitIndex:].AllBitsAre(codec.ProtocolRecessiveBit) {
				return stateReceive, errors.New("received all expected bits, but do not see correct EOF")
			}

			// Assemble CAN frame
			rxFrame, err := codec.FromBits(rxUnstuffed[1:firstEOFbitIndex])
			if err != nil {
				return stateReceive, fmt.Errorf("failed to assemble frame: %w", err)
			}
			this.OutputByName(common.PortCANRx).PutSignals(signal.New(rxFrame))
			// Clear state
			this.State().Set(stateKeyRxBuffer, codec.NewBits(0))
			this.State().Set(stateKeyConsecutiveRecessiveBitsObserved, 0)
			this.State().Set(stateKeyBitsExpected, 0)
			return stateIdle, nil
		}
	}
	this.State().Set(stateKeyRxBuffer, rxBuf)
	return stateReceive, nil
}

package controller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/common"
	"github.com/hovsep/fmesh/signal"

	"github.com/hovsep/fmesh/component"
)

const (
	stateKeyTxQueue                          = "tx_queue"
	stateKeyRxBuffer                         = "rx_buffer"
	stateKeyControllerState                  = "controller_state"
	stateKeyConsecutiveRecessiveBitsObserved = "consecutive_recessive_observed"
	stateKeyBitsExpected                     = "bits_expected"

	frameFixedPartBits = codec.ProtocolSOFSize + codec.ProtocolIDSize + codec.ProtocolDLCSize
)

var (
	errNoBitOnBus = errors.New("no bit set on bus")
)

// New creates a stateful CAN controller
// which converts frames to bits and vice versa
func New(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInputs(common.PortCANTx, common.PortCANRx).                              // Frame in, bits in
		WithOutputs(common.PortCANTx, common.PortCANRx, common.PortControllerState). // Bits out, frame out, notify when bus is idle
		WithInitialState(func(state component.State) {
			state.Set(stateKeyTxQueue, TxQueue{})
			state.Set(stateKeyRxBuffer, codec.NewBits(0))
			state.Set(stateKeyControllerState, StateIdle)
			state.Set(stateKeyConsecutiveRecessiveBitsObserved, 0)
			state.Set(stateKeyBitsExpected, 0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			defer func() {
				// Report current state to bus watchdog
				ctlState := this.State().Get(stateKeyControllerState).(State)
				this.OutputByName(common.PortControllerState).PutSignals(signal.New(StateMap{
					this.Name(): ctlState,
				}))
			}()

			refreshLoggerPrefix(this)

			err := handleIncomingFrames(this)
			if err != nil {
				return fmt.Errorf("failed to handle incoming frames: %w", err)
			}

			// Get current bit set on the bus
			currentBit, err := getCurrentBit(this)
			if err != nil && errors.Is(err, errNoBitOnBus) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("failed to determine current bit on the bus: %w", err)
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
		this.Logger().Printf("got a frame from MCU to send: %s items in tx-queue: %d", frame, len(txQueue))
	}
	return nil
}

func getCurrentBit(this *component.Component) (codec.Bit, error) {
	if !this.InputByName(common.PortCANRx).HasSignals() {
		return codec.ProtocolRecessiveBit, errNoBitOnBus
	}
	if this.InputByName(common.PortCANRx).Buffer().Len() > 1 {
		return codec.ProtocolRecessiveBit, errors.New("received more than one bit")
	}
	currentBit := this.InputByName(common.PortCANRx).FirstSignalPayloadOrNil().(codec.Bit)

	return currentBit, nil
}

func runStateMachine(this *component.Component, currentBit codec.Bit) error {
	currentState := this.State().Get(stateKeyControllerState).(State)
	for {
		nextState, err := getNextState(this, currentState, currentBit)
		if err != nil {
			return fmt.Errorf("failed to switch from state: %s error: %w", currentState, err)
		}

		err = handleStateTransition(this, currentState, nextState)
		if err != nil {
			return fmt.Errorf("failed to handle state transition: : %w", currentState.To(nextState), err)
		}

		if nextState == currentState {
			// No transitions, exit
			return nil
		} else {
			this.Logger().Print("state transition:", currentState.To(nextState))
		}
		currentState = nextState
		this.State().Set(stateKeyControllerState, currentState)
		refreshLoggerPrefix(this)
	}
	return errors.New("did not manage to exit correctly from main state machine loop")
}

func getNextState(this *component.Component, currentState State, currentBit codec.Bit) (State, error) {
	switch currentState {
	case StateIdle:
		return handleIdleState(this, currentBit)
	case StateWaitForBusIdle:
		return handleWaitForBusIdleState(this, currentBit)
	case StateArbitration:
		return handleArbitrationState(this, currentBit)

	case StateTransmit:
		return handleTransmitState(this)
	case StateReceive:
		return handleReceiveState(this, currentBit)
	default:
		return currentState, fmt.Errorf("end up in incorrect state: %v", currentState)
	}
}

func handleIdleState(this *component.Component, currentBit codec.Bit) (State, error) {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)

	// SOF detected, became passive listener
	if currentBit.IsDominant() {
		return StateReceive, nil
	}

	if len(txQueue) > 0 {
		return StateWaitForBusIdle, nil
	}

	// passive-idle situation: controller does not want to write, there is nothing to read
	return StateIdle, nil
}

func handleWaitForBusIdleState(this *component.Component, currentBit codec.Bit) (State, error) {
	// Check if some other node started transmitting
	// SOF detected, became passive listener
	if currentBit.IsDominant() {
		return StateReceive, nil
	}

	// Track consecutive recessive bits
	consecutiveRecessiveBitsObserved := this.State().Get(stateKeyConsecutiveRecessiveBitsObserved).(int)
	consecutiveRecessiveBitsObserved++
	this.State().Set(stateKeyConsecutiveRecessiveBitsObserved, consecutiveRecessiveBitsObserved)

	if consecutiveRecessiveBitsObserved > codec.ProtocolEOFSize+codec.ProtocolIFSSize {
		this.Logger().Println("i've seen ", consecutiveRecessiveBitsObserved, " recessives")
		// The bus looks idle. It's time to start transmitting
		return StateArbitration, nil
	}
	// Continue waiting
	this.Logger().Println("waiting for more consecutive recessive bits, seen so far:", consecutiveRecessiveBitsObserved)
	return StateWaitForBusIdle, nil
}

func handleArbitrationState(this *component.Component, currentBit codec.Bit) (State, error) {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
	rxBuf := this.State().Get(stateKeyRxBuffer).(codec.Bits)
	defer func() {
		this.State().Set(stateKeyRxBuffer, rxBuf)
	}()

	txItem := txQueue[0]

	if txItem.Buf.Available() == 0 {
		return StateArbitration, errors.New("already processed buffer is still in tx-queue")
	}

	// Receive own sent bits (and skip anything before we started writing first bit)
	if txItem.Buf.Offset > 0 {
		rxBuf = rxBuf.WithBits(currentBit)
	}

	// Check if arbitration is won
	rxUnstuffed := rxBuf.WithoutStuffing(codec.ProtocolBitStuffingStep)
	if rxUnstuffed.Len() == codec.ProtocolIDSize+1 {
		idReceived := rxUnstuffed[1:]
		idTransmitted := txItem.Buf.Bits.WithoutStuffing(codec.ProtocolBitStuffingStep)[1 : codec.ProtocolIDSize+1]
		wonArbitration := idReceived.Equals(idTransmitted)
		if wonArbitration {
			this.Logger().Println("won arbitration")
			return StateTransmit, nil
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
				return StateArbitration, errors.New("bus error, recessive bit won arbitration. backoff")
			}

			txItem.Buf.ResetOffset() // Backoff, retry later
			return StateReceive, nil
		}
	}

	txBit := txItem.Buf.NextBit()

	this.OutputByName(common.PortCANTx).PutSignals(signal.New(txBit))
	txItem.Buf.IncreaseOffset()

	return StateArbitration, nil
}

func handleReceiveState(this *component.Component, currentBit codec.Bit) (State, error) {
	rxBuf := this.State().Get(stateKeyRxBuffer).(codec.Bits)
	bitsExpected := this.State().Get(stateKeyBitsExpected).(int)

	rxUnstuffed := rxBuf.WithoutStuffing(codec.ProtocolBitStuffingStep)
	bitsReceived := rxUnstuffed.Len()

	if bitsExpected == 0 {
		// Nothing is received, let's expect the first 16 fixed bits
		bitsExpected = frameFixedPartBits
		this.State().Set(stateKeyBitsExpected, bitsExpected)

		if bitsReceived == 0 {
			this.Logger().Println("receiving the very first bit, must be SOF: ", currentBit)
		} else {
			this.Logger().Println("looks like I've lost the arbitration, so I'm receiving next bit: ", currentBit, " already received:", bitsReceived)
		}

	}

	rxBuf = rxBuf.WithBits(currentBit)
	//this.Logger().Println("added a bit to my raw RX:", rxBuf)
	rxUnstuffed = rxBuf.WithoutStuffing(codec.ProtocolBitStuffingStep)
	bitsReceived = rxUnstuffed.Len()

	// All CAN frames begin with 3 fixed-size fields: SOF (1), ID(11) and DLC
	// when we have received enough bits - we decode ID and DLC
	// knowing DLC allows us to know exactly how many data bits we expect
	if bitsReceived >= frameFixedPartBits && bitsReceived == bitsExpected {

		// Decode ID and DLC
		if bitsExpected == frameFixedPartBits {
			idBits := rxUnstuffed[codec.ProtocolSOFSize : codec.ProtocolSOFSize+codec.ProtocolIDSize]
			dlcBits := rxUnstuffed[codec.ProtocolSOFSize+codec.ProtocolIDSize:]
			expectedBytes := dlcBits.ToInt()
			this.Logger().Println("id: ", idBits, " dlc:", dlcBits, " dlc (bytes):", expectedBytes)

			// We know the DLC, let's expect the all bits
			this.State().Set(stateKeyBitsExpected, frameFixedPartBits+expectedBytes*8+codec.ProtocolEOFSize)
		} else {
			// Decode data
			this.Logger().Println("received all expected data and EOF")
			// Check for valid EOF
			firstEOFBitIndex := rxUnstuffed.Len() - codec.ProtocolEOFSize
			if !rxUnstuffed[firstEOFBitIndex:].AllBitsAre(codec.ProtocolRecessiveBit) {
				return StateReceive, errors.New("received all expected bits, but do not see correct EOF")
			}

			// Assemble CAN frame
			rxFrame, err := codec.FromBits(rxUnstuffed[1:firstEOFBitIndex])
			if err != nil {
				return StateReceive, fmt.Errorf("failed to assemble frame: %w", err)
			}

			this.Logger().Println("assembled frame:", rxFrame)

			this.OutputByName(common.PortCANRx).PutSignals(signal.New(rxFrame))
			return StateIdle, nil
		}
	}
	this.State().Set(stateKeyRxBuffer, rxBuf)
	return StateReceive, nil
}

func handleTransmitState(this *component.Component) (State, error) {
	txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
	defer func() {
		this.State().Set(stateKeyTxQueue, txQueue)
	}()

	txItem := txQueue[0]

	if txItem.Buf.Offset <= codec.ProtocolIDSize {
		return StateTransmit, errors.New("must be in arbitration state")
	}

	// Check if we finished transmitting the buffer
	if txItem.Buf.Available() == 0 {
		this.Logger().Println("a buffer is successfully transmitted, remove it from the queue")

		txQueue = txQueue[1:]
		return StateIdle, nil
	}

	txBit := txItem.Buf.NextBit()
	//this.Logger().Println("write frame bit:", txBit)
	this.OutputByName(common.PortCANTx).PutSignals(signal.New(txBit))
	txItem.Buf.IncreaseOffset()

	return StateTransmit, nil
}

func handleStateTransition(this *component.Component, previousState, nextState State) error {
	switch previousState.To(nextState) {

	// Lost arbitration
	case StateArbitration.To(StateReceive):
		// Forget bits collected during arbitration
		// do not clear rx when lost the arbitration: this.State().Set(stateKeyRxBuffer, codec.NewBits(0))
		return nil

	// Successfully finished transmitting
	case StateTransmit.To(StateIdle):
		this.State().Set(stateKeyRxBuffer, codec.NewBits(0))
		return nil

	// Wanted to start transmitting, but received SOF
	case StateWaitForBusIdle.To(StateReceive):
		this.State().Set(stateKeyConsecutiveRecessiveBitsObserved, 0)
		return nil

	// When decided to start transmitting
	case StateWaitForBusIdle.To(StateArbitration):
		this.State().Set(stateKeyConsecutiveRecessiveBitsObserved, 0)
		return nil

	// Successfully received a frame
	case StateReceive.To(StateIdle):
		// Clear everything, prepare to receive next frame
		this.State().Set(stateKeyRxBuffer, codec.NewBits(0))
		this.State().Set(stateKeyBitsExpected, 0)
		return nil
	}
	return nil
}

func refreshLoggerPrefix(this *component.Component) {
	ctlState := this.State().Get(stateKeyControllerState).(State)
	this.Logger().SetPrefix(fmt.Sprintf("%s [%s] : ", this.Name(), ctlState))
}

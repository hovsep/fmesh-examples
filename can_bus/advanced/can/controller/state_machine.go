package controller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// State is the main DFSM of CAN-controller
type State byte

const (
	stateIdle State = iota
	stateWaitForBusIdle
	stateArbitration
	stateTransmit
	stateReceive
)

var (
	controllerStateNames = []string{
		"IDLE",
		"WAITING FOR BUS IDLE",
		"ARBITRATION",
		"TRANSMIT",
		"RECEIVE",
	}
)

func (state State) String() string {
	return controllerStateNames[state]
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
			this.Logger().Println("no state transition, exiting")
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

	if consecutiveRecessiveBitsObserved > codec.ProtocolEOFBitsCount+codec.ProtocolIFSBitsCount {
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

	// Receive own sent bits
	if txItem.Buf.Offset > 0 {
		rxBuf = rxBuf.WithBits(currentBit)
		this.Logger().Println("observed rxBuf:", rxBuf)
	}

	// Check if arbitration is won
	wonArbitration := txItem.IDIsTransmitted()
	if wonArbitration {
		return stateTransmit, nil
	}

	// After SOF
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

	if txItem.Buf.Offset <= codec.ProtocolIDBitsCount {
		return stateTransmit, errors.New("must be in arbitration state")
	}

	// Check if we finished transmitting the buffer
	if txItem.Buf.Available() == 0 {
		this.Logger().Println("a buffer is successfully transmitted, remove it from the queue")
		txQueue = txQueue[1:]
		return stateIdle, nil
	}

	txBit := txItem.Buf.NextBit()
	this.Logger().Println("write frame bit:", txBit)
	this.OutputByName(common.PortCANTx).PutSignals(signal.New(txBit))
	txItem.Buf.IncreaseOffset()

	return stateTransmit, nil
}

func handleReceiveState(this *component.Component, currentBit codec.Bit) (State, error) {
	rxBuf := this.State().Get(stateKeyRxBuffer).(codec.Bits)
	defer func() {
		this.State().Set(stateKeyRxBuffer, rxBuf)
	}()

	if rxBuf.Len() == 0 {
		this.Logger().Println("receiving SOF")
	} else {
		rxUnstuffed := rxBuf.WithoutStuffing(codec.ProtocolBitStuffingStep)
		idUnstuffed := rxUnstuffed[1:]

		if idUnstuffed.Len() == codec.ProtocolIDBitsCount {
			this.Logger().Print("ID detected:", idUnstuffed.String())
		}

		if idUnstuffed.Len() < codec.ProtocolIDBitsCount {
			this.Logger().Println("adding ID:", idUnstuffed.String())
		} else {
			if rxUnstuffed.Len() < 1+codec.ProtocolIDBitsCount+codec.ProtocolDLCBitsCount {
				this.Logger().Println("adding DLC")
			}

			if rxUnstuffed.Len() == 1+codec.ProtocolIDBitsCount+codec.ProtocolDLCBitsCount {
				dlcUnstuffed := rxUnstuffed[1+codec.ProtocolIDBitsCount:]
				this.Logger().Println("DLC detected:", dlcUnstuffed.String(), " expecting ", 8*dlcUnstuffed.ToInt(), " more bits")
			}

		}
	}

	rxBuf = rxBuf.WithBits(currentBit)
	this.Logger().Println("rxBuf:", rxBuf)

	eofDetected := false // TODO fix this
	if eofDetected {
		this.Logger().Println("EOF, final rxBuf:", rxBuf)
		// Build frame and put on port
		return stateIdle, nil
	}
	return stateReceive, nil
}

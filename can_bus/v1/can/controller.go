package can

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	stateKeyTxQueue                          = "tx_queue"
	stateKeyRxBuffer                         = "rx_buffer"
	stateKeyControllerState                  = "controller_state"
	stateKeyConsecutiveRecessiveBitsObserved = "consecutive_recessive_observed"
)

// NewController creates a CAN controller
// which converts frames to bits and vice versa
func NewController(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInputs(PortCANTx, PortCANRx).  // Frame in, bits in
		WithOutputs(PortCANTx, PortCANRx). // Bits out, frame out
		WithInitialState(func(state component.State) {
			state.Set(stateKeyTxQueue, TxQueue{})            // Multiple bit buffers we are trying to send to transceiver
			state.Set(stateKeyRxBuffer, NewEmptyBitBuffer()) // Single bit buffer we are trying to build from bits coming from transceiver
			state.Set(stateKeyControllerState, controllerStateIdle)
			state.Set(stateKeyConsecutiveRecessiveBitsObserved, byte(0))
		}).
		WithActivationFunc(func(this *component.Component) error {
			// Extract the state
			txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
			rxBuf := this.State().Get(stateKeyRxBuffer).(*BitBuffer)
			currentState := this.State().Get(stateKeyControllerState).(ControllerState)
			consecutiveRecessiveBitsObserved := this.State().Get(stateKeyConsecutiveRecessiveBitsObserved).(byte)

			defer func() {
				// Save latest changes in state
				this.State().Set(stateKeyTxQueue, txQueue)
				this.State().Set(stateKeyRxBuffer, rxBuf)
				this.State().Set(stateKeyControllerState, currentState)
				this.State().Set(stateKeyConsecutiveRecessiveBitsObserved, consecutiveRecessiveBitsObserved)
			}()

			// Enqueue new frames coming from MCU
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*Frame)
				if !ok || !frame.isValid() {
					return errors.New("received corrupted frame")
				}

				frameBits, lastIDBitIndex := frame.ToBits()

				this.Logger().Println("ID ends at ", lastIDBitIndex)
				txQueue = append(txQueue, &TxQueueItem{
					// Add IFS and 1 extra recessive bit
					Buf:            NewBitBuffer(frameBits.WithIFS().WithExtraBits(ProtocolRecessiveBit)),
					LastIDBitIndex: lastIDBitIndex,
				})
				this.Logger().Printf("got a frame to send: %s, items in tx-queue: %d", frameBits, len(txQueue))
			}

			// Get current bit set on the bus
			if !this.InputByName(PortCANRx).HasSignals() {
				return errors.New("unable to read current bit, cannot proceed with protocol decision")
			}
			if this.InputByName(PortCANRx).Buffer().Len() > 1 {
				return errors.New("received more than one bit")
			}
			currentBit := this.InputByName(PortCANRx).FirstSignalPayloadOrNil().(Bit)
			this.Logger().Println("observing current bit on bus:", currentBit)

			if currentBit.IsRecessive() {
				consecutiveRecessiveBitsObserved++
			} else {
				consecutiveRecessiveBitsObserved = 0
			}

			// Main state machine:
			for {
				switch currentState {
				case controllerStateIdle:
					logCurrentState(this, currentState)
					// SOF detected, became passive listener
					if currentBit.IsDominant() {
						logStateTransition(this, currentState, controllerStateReceive)
						currentState = controllerStateReceive
						continue
					} else {
						if len(txQueue) > 0 {
							logStateTransition(this, currentState, controllerStateWaitForBusIdle)
							currentState = controllerStateWaitForBusIdle
							continue
						}

						// passive-idle situation: controller does not want to write, there is nothing to read
						this.Logger().Println("exit: passive-idle, recessive bits observed so far:", consecutiveRecessiveBitsObserved)
						return nil
					}

				case controllerStateWaitForBusIdle:
					logCurrentState(this, currentState)
					if consecutiveRecessiveBitsObserved > ProtocolEOFBitsCount+ProtocolIFSBitsCount {
						this.Logger().Println("i've seen ", consecutiveRecessiveBitsObserved, " recessives")
						// The bus looks idle, it's time to start transmitting
						logStateTransition(this, currentState, controllerStateArbitration)
						currentState = controllerStateArbitration
						continue
					}
					// Continue waiting
					this.Logger().Println("exit: waiting for more consecutive recessive bits, seen so far:", consecutiveRecessiveBitsObserved)
					return nil
				case controllerStateArbitration:
					logCurrentState(this, currentState)
					txItem := txQueue[0]

					if txItem.Buf.Available() == 0 {
						return errors.New("already processed buffer is still in tx-queue")
					}

					// Check if arbitration is won
					wonArbitration := txItem.IDIsTransmitted()
					if wonArbitration {
						logStateTransition(this, currentState, controllerStateTransmit)
						currentState = controllerStateTransmit
						continue
					}

					// After SOF
					if txItem.Buf.Offset > 1 {

						// Check if arbitration is lost
						if currentBit != txItem.Buf.PreviousBit() {
							// Lost arbitration
							if currentBit.IsDominant() && txItem.Buf.PreviousBit().IsRecessive() {
								this.Logger().Println("lost arbitration. backoff")
							}

							// Or bus error happen
							if currentBit.IsRecessive() && txItem.Buf.PreviousBit().IsDominant() {
								return errors.New("bus error, recessive bit won arbitration. backoff")
							}

							txItem.Buf.ResetOffset() // Backoff, retry later

							logStateTransition(this, currentState, controllerStateReceive)
							currentState = controllerStateReceive
							continue
						}
					}

					txBit := txItem.Buf.NextBit()
					if txItem.Buf.Offset == 0 {
						this.Logger().Println("write SOF bit:", txBit)
					} else {
						this.Logger().Println("write arbitration (ID) bit:", txBit)
					}

					this.OutputByName(PortCANTx).PutSignals(signal.New(txBit))
					txItem.Buf.IncreaseOffset()

					return nil
				case controllerStateTransmit:
					logCurrentState(this, currentState)
					txItem := txQueue[0]

					if txItem.Buf.Offset <= ProtocolIDBitsCount {
						return errors.New("must be in arbitration state")
					}

					// Check if we finished transmitting the buffer
					if txItem.Buf.Available() == 0 {
						this.Logger().Println("a buffer is successfully transmitted, remove it from the queue")
						txQueue = txQueue[1:]
						logStateTransition(this, currentState, controllerStateIdle)
						currentState = controllerStateIdle
						continue
					}

					txBit := txItem.Buf.NextBit()
					this.Logger().Println("write frame bit:", txBit)
					this.OutputByName(PortCANTx).PutSignals(signal.New(txBit))
					txItem.Buf.IncreaseOffset()

					return nil
				case controllerStateReceive:
					logCurrentState(this, currentState)
					this.Logger().Println("recessive bits observed so far:", consecutiveRecessiveBitsObserved)

					rxBuf.AppendBit(currentBit)
					this.Logger().Println("rxBuf:", rxBuf.Bits)

					if consecutiveRecessiveBitsObserved == ProtocolEOFBitsCount {
						this.Logger().Println("EOF, final rxBuf:", rxBuf.Bits)
						// Build frame and put on port
						currentState = controllerStateIdle
						return nil
					}
					return nil
				default:
					return fmt.Errorf("end up in incorrect state: %v", currentState)
				}
			}
			return errors.New("did not manage to exit correctly from main state machine loop")
		})
}

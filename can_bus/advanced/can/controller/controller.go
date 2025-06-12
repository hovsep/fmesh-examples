package controller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"

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
		WithInputs(common.PortCANTx, common.PortCANRx).  // Frame in, bits in
		WithOutputs(common.PortCANTx, common.PortCANRx). // Bits out, frame out
		WithInitialState(func(state component.State) {
			state.Set(stateKeyTxQueue, TxQueue{})                  // Multiple bit buffers we are trying to send to transceiver
			state.Set(stateKeyRxBuffer, codec.NewEmptyBitBuffer()) // Single bit buffer we are trying to build from bits coming from transceiver
			state.Set(stateKeyControllerState, stateIdle)
			state.Set(stateKeyConsecutiveRecessiveBitsObserved, byte(0))
		}).
		WithActivationFunc(func(this *component.Component) error {
			// Extract the state
			txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
			rxBuf := this.State().Get(stateKeyRxBuffer).(*codec.BitBuffer)
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
			for _, sig := range this.InputByName(common.PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*codec.Frame)
				if !ok || !frame.IsValid() {
					return errors.New("received corrupted frame")
				}

				frameBits, lastIDBitIndex := frame.ToBits()

				this.Logger().Println("ID ends at ", lastIDBitIndex)
				txQueue = append(txQueue, &TxQueueItem{
					// Add IFS and 1 extra recessive bit
					Buf:            codec.NewBitBuffer(frameBits.WithIFS().WithExtraBits(codec.ProtocolRecessiveBit)),
					LastIDBitIndex: lastIDBitIndex,
				})
				this.Logger().Printf("got a frame to send: %s, items in tx-queue: %d", frameBits, len(txQueue))
			}

			// Get current bit set on the bus
			if !this.InputByName(common.PortCANRx).HasSignals() {
				return errors.New("unable to read current bit, cannot proceed with protocol decision")
			}
			if this.InputByName(common.PortCANRx).Buffer().Len() > 1 {
				return errors.New("received more than one bit")
			}
			currentBit := this.InputByName(common.PortCANRx).FirstSignalPayloadOrNil().(codec.Bit)
			this.Logger().Println("observing current bit on bus:", currentBit)

			if currentBit.IsRecessive() {
				consecutiveRecessiveBitsObserved++
			} else {
				consecutiveRecessiveBitsObserved = 0
			}

			// Main state machine:
			for {
				switch currentState {
				case stateIdle:
					logCurrentState(this, currentState)
					// SOF detected, became passive listener
					if currentBit.IsDominant() {
						logStateTransition(this, currentState, stateReceive)
						currentState = stateReceive
						continue
					} else {
						if len(txQueue) > 0 {
							logStateTransition(this, currentState, stateWaitForBusIdle)
							currentState = stateWaitForBusIdle
							continue
						}

						// passive-idle situation: controller does not want to write, there is nothing to read
						return nil
					}

				case stateWaitForBusIdle:
					logCurrentState(this, currentState)
					if consecutiveRecessiveBitsObserved > codec.ProtocolEOFBitsCount+codec.ProtocolIFSBitsCount {
						this.Logger().Println("i've seen ", consecutiveRecessiveBitsObserved, " recessives")
						// The bus looks idle. It's time to start transmitting
						logStateTransition(this, currentState, stateArbitration)
						currentState = stateArbitration
						continue
					}
					// Continue waiting
					this.Logger().Println("exit: waiting for more consecutive recessive bits, seen so far:", consecutiveRecessiveBitsObserved)
					return nil
				case stateArbitration:
					logCurrentState(this, currentState)
					txItem := txQueue[0]

					if txItem.Buf.Available() == 0 {
						return errors.New("already processed buffer is still in tx-queue")
					}

					// Check if arbitration is won
					wonArbitration := txItem.IDIsTransmitted()
					if wonArbitration {
						logStateTransition(this, currentState, stateTransmit)
						currentState = stateTransmit
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

							logStateTransition(this, currentState, stateReceive)
							currentState = stateReceive
							continue
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

					return nil
				case stateTransmit:
					logCurrentState(this, currentState)
					txItem := txQueue[0]

					if txItem.Buf.Offset <= codec.ProtocolIDBitsCount {
						return errors.New("must be in arbitration state")
					}

					// Check if we finished transmitting the buffer
					if txItem.Buf.Available() == 0 {
						this.Logger().Println("a buffer is successfully transmitted, remove it from the queue")
						txQueue = txQueue[1:]
						logStateTransition(this, currentState, stateIdle)
						currentState = stateIdle
						continue
					}

					txBit := txItem.Buf.NextBit()
					this.Logger().Println("write frame bit:", txBit)
					this.OutputByName(common.PortCANTx).PutSignals(signal.New(txBit))
					txItem.Buf.IncreaseOffset()

					return nil
				case stateReceive:
					logCurrentState(this, currentState)

					if rxBuf.Len() == 0 {
						this.Logger().Println("adding SOF")
					} else {
						rxUnstuffed := rxBuf.Bits.WithoutStuffing(codec.ProtocolBitStuffingStep)
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

					rxBuf.AppendBit(currentBit)
					this.Logger().Println("rxBuf:", rxBuf.Bits)

					eofDetected := false // TODO fix this
					if eofDetected {
						this.Logger().Println("EOF, final rxBuf:", rxBuf.Bits)
						// Build frame and put on port
						currentState = stateIdle
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

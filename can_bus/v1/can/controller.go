package can

import (
	"errors"

	"github.com/hovsep/fmesh/signal"

	"github.com/hovsep/fmesh/component"
)

// TxQueue represents the queue of frames (in the form of bit buffers) to be transmitted
type TxQueue []*BitBuffer

type ArbitrationState byte

const (
	stateKeyTxQueue          = "tx_queue"
	stateKeyRxBuffer         = "rx_buffer"
	stateKeyArbitrationState = "arbitration_state"

	arbitrationStateIn   ArbitrationState = iota // Controller is still competing with others
	arbitrationStateLost                         // Dominant bit took over, this controller needs to stop transmitting
	arbitrationStateWon                          // This controller won the arbitration, it is now the only writer, so no need to perform arbitration check (but still need to check for errors)
)

// NewController creates a CAN controller
// which converts frames to bits and vice versa
func NewController(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInitialState(func(state component.State) {
			state.Set(stateKeyTxQueue, TxQueue{})            // Multiple bit buffers we are trying to send to transceiver
			state.Set(stateKeyRxBuffer, NewEmptyBitBuffer()) // Single bit buffer we are trying to build from bits coming from transceiver
			state.Set(stateKeyArbitrationState, arbitrationStateIn)
		}).
		WithActivationFunc(func(this *component.Component) error {
			// Extract the state
			txQueue := this.State().Get(stateKeyTxQueue).(TxQueue)
			rxBuf := this.State().Get(stateKeyRxBuffer).(*BitBuffer)
			arbitrationState := this.State().Get(stateKeyArbitrationState).(ArbitrationState)

			defer func() {
				this.State().Set(stateKeyTxQueue, txQueue)
				this.State().Set(stateKeyRxBuffer, rxBuf)
				this.State().Set(stateKeyArbitrationState, arbitrationState)
			}()

			// Handle new frames coming from MCU
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*Frame)
				if !ok || !frame.isValid() {
					return errors.New("received corrupted frame")
				}

				frameBits := frame.ToBits()
				txQueue = append(txQueue, NewBitBuffer(frameBits))
				this.Logger().Printf("got a frame to send: %s, items in tx-queue: %d", frameBits, len(txQueue))
			}

			// Get current bit on the bus
			var currentBit Bit
			currentBitIsSet := this.InputByName(PortCANRx).HasSignals()
			if currentBitIsSet {
				currentBit = this.InputByName(PortCANRx).FirstSignalPayloadOrNil().(Bit)
				this.Logger().Println("read current bit on bus:", currentBit)

				rxBuf.AppendBit(currentBit)
				this.Logger().Println("rxBuf:", rxBuf.Bits)
			}

			// Check if there are frames to write and pop one
			if len(txQueue) > 0 {
				bbToProcess := txQueue[0]

				if bbToProcess.Available() == 0 {
					this.Logger().Println("error: processed buffer is still in tx-queue")
				}

				if !currentBitIsSet && bbToProcess.Offset == 0 {
					// We are sending the very first bit on idle bus, no arbitration, just writing the first bit
					bitToSend := bbToProcess.NextBit()

					this.Logger().Println("write:", bitToSend)
					this.OutputByName(PortCANTx).PutSignals(signal.New(bitToSend))
					bbToProcess.IncreaseOffset()
				}

				if currentBitIsSet && bbToProcess.Offset > 0 {

					// Check arbitration state (only while sending ID)
					if bbToProcess.Offset < IDBitsCount {
						this.Logger().Println("in arbitration")
						arbitrationState = arbitrationStateIn
					} else {
						this.Logger().Println("arbitration won")
						arbitrationState = arbitrationStateWon
					}

					// Perform check if still in arbitration
					if arbitrationState == arbitrationStateIn {
						if currentBit != bbToProcess.PreviousBit() {
							// Lost arbitration
							arbitrationState = arbitrationStateLost

							if currentBit == DominantBit && bbToProcess.PreviousBit() == RecessiveBit {
								this.Logger().Println("lost arbitration. backoff")
								bbToProcess.ResetOffset()
							}

							// Also check for transmitting errors
							if currentBit == RecessiveBit && bbToProcess.PreviousBit() == DominantBit {
								panic("bus error, recessive bit won arbitration. backoff")
							}
						}
					}

					if arbitrationState != arbitrationStateLost {
						if bbToProcess.Available() > 0 {
							bitToSend := bbToProcess.NextBit()
							this.Logger().Println("write:", bitToSend)
							this.OutputByName(PortCANTx).PutSignals(signal.New(bitToSend))
							bbToProcess.IncreaseOffset()
						}

						// Check if we finished processing the buffer
						if bbToProcess.Available() == 0 {
							this.Logger().Println("a buffer is successfully transmitted, remove it from the queue")
							txQueue = txQueue[1:]
						}
					}
				}
			}

			return nil
		}).
		WithInputs(PortCANTx, PortCANRx). // Frame in, bits in
		WithOutputs(PortCANTx, PortCANRx) // Bits out, frame out
}

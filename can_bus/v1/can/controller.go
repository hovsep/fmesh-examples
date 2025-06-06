package can

import (
	"errors"

	"github.com/hovsep/fmesh/signal"

	"github.com/hovsep/fmesh/component"
)

// TxQueue represents the queue of frames (in the form of bit buffers) to be transmitted
type TxQueue []*BitBuffer

const (
	stateTxQueue  = "tx_queue"
	stateRxBuffer = "rx_buffer"
)

// NewController creates a CAN controller
// which converts frames to bits and vice versa
func NewController(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInitialState(func(state component.State) {
			state.Set(stateTxQueue, TxQueue{})            // Multiple bit buffers we are trying to send to transceiver
			state.Set(stateRxBuffer, NewEmptyBitBuffer()) // Single bit buffer we are trying to build from bits coming from transceiver
		}).
		WithActivationFunc(func(this *component.Component) error {
			txQueue := this.State().Get(stateTxQueue).(TxQueue)
			rxBuf := this.State().Get(stateRxBuffer).(*BitBuffer)
			defer func() {
				this.State().Set(stateTxQueue, txQueue)
			}()

			// Handle new frames coming from MCU
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*Frame)
				if !ok || !frame.isValid() {
					return errors.New("received corrupted frame")
				}

				txQueue = append(txQueue, NewBitBuffer(frame.toBits()))
				this.Logger().Printf("got a frame to send: %s, items in tx-queue: %d", frame.toBits(), len(txQueue))
			}

			// Get current bit on the bus
			var currentBit Bit
			currentBitIsSet := this.InputByName(PortCANRx).HasSignals()
			if currentBitIsSet {
				currentBit = this.InputByName(PortCANRx).FirstSignalPayloadOrNil().(Bit)
				this.Logger().Println("read current bit on bus:", currentBit)

				rxBuf.AppendBit(currentBit)
				this.Logger().Println("rxBuf:", rxBuf)
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
					// Check arbitration
					if currentBit != bbToProcess.PreviousBit() {
						// Lost arbitration
						if currentBit == DominantBit && bbToProcess.PreviousBit() == RecessiveBit {
							this.Logger().Println("lost arbitration. backoff")
							bbToProcess.ResetOffset()
						}

						if currentBit == RecessiveBit && bbToProcess.PreviousBit() == DominantBit {
							panic("bus error, recessive bit won arbitration. backoff")
						}

					} else {
						// In arbitration
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

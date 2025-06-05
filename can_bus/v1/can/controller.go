package can

import (
	"errors"

	"github.com/hovsep/fmesh/signal"

	"github.com/hovsep/fmesh/component"
)

// TxQueue represents the queue of frames to be transmitted
type TxQueue []*BitBuffer

const (
	StateTxQueue = "tx_queue"
)

// NewController creates a CAN controller
// which converts frames to bits and vice versa
func NewController(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInitialState(func(state component.State) {
			state.Set(StateTxQueue, TxQueue{})

		}).
		WithActivationFunc(func(this *component.Component) error {
			txQueue := this.State().Get(StateTxQueue).(TxQueue)
			defer func() {
				this.State().Set(StateTxQueue, txQueue)
			}()

			// Handle new frames coming from MCU
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*Frame)
				if !ok || !frame.isValid() {
					return errors.New("received corrupted frame")
				}

				txQueue = append(txQueue, &BitBuffer{
					Bits: frame.toBits(),
					Pos:  0,
				})
				this.Logger().Println("got a frame to send: ", frame.toBits().String(), " tx-queue len=", len(txQueue))
			}

			// Check if there are frames to write and pop one
			if len(txQueue) > 0 {
				buffer := txQueue[0]

				// Just write the first bit
				if buffer.Pos == 0 {
					firstBitToTransmit := buffer.Bits[0]
					this.Logger().Println("write first bit: ", firstBitToTransmit.String())
					this.OutputByName(PortCANTx).PutSignals(signal.New(firstBitToTransmit))
					buffer.Pos++
				} else {
					// In order to write next bits, need to wait for a bit from can to compare them
					if this.InputByName(PortCANRx).HasSignals() {
						bitOnBus := this.InputByName(PortCANRx).FirstSignalPayloadOrNil().(Bit)
						this.Logger().Println("bit on bus:", bitOnBus.String())

						// Check if arbitration is lost
						lastWrittenBit := buffer.Bits[buffer.Pos]
						if bitOnBus != lastWrittenBit {
							this.Logger().Println("lost arbitration, bit on bus: ", bitOnBus.String(), " last written bit:", lastWrittenBit.String())
						} else {
							this.Logger().Println("in arbitration")
							if buffer.Pos < len(buffer.Bits)-1 {
								nextBitToTransmit := buffer.Bits[buffer.Pos+1]
								this.Logger().Println("write next bit: ", nextBitToTransmit.String())
								this.OutputByName(PortCANTx).PutSignals(signal.New(nextBitToTransmit))
								buffer.Pos++
							} else {
								this.Logger().Println("all bits are written, removing the frame from queue")
								txQueue = txQueue[1:]
							}
						}
					}
				}

			}

			return nil
		}).
		WithInputs(PortCANTx, PortCANRx). // Frame in, bits in
		WithOutputs(PortCANTx, PortCANRx) // Bits out, frame out
}

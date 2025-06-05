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

			// TODO (write path):
			// continue transmitting from the latest bit and compare with latest read bit, check if arbitration is lost and retry

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
				this.Logger().Println("got a frame to send: ", frame.toBits().String())
			}

			// Check if there are frames to write and pop one
			if len(txQueue) > 0 {
				buffer := txQueue[0]

				// Write next bit
				if buffer.Pos < len(buffer.Bits)-1 {
					this.OutputByName(PortCANTx).PutSignals(signal.New(buffer.Bits[buffer.Pos+1]))
					buffer.Pos++
				}
			}

			// Read path (building frame from bits coming from transceiver)
			for _, sig := range this.InputByName(PortCANRx).AllSignalsOrNil() {
				bitRead := sig.PayloadOrNil().(Bit)
				this.Logger().Println("read a bit: ", bitRead.String())
			}

			return nil
		}).
		WithInputs(PortCANTx, PortCANRx). // Frame in, bits in
		WithOutputs(PortCANTx, PortCANRx) // Bits out, frame out
}

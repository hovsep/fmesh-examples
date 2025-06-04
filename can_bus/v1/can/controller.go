package can

import (
	"errors"
	"github.com/hovsep/fmesh/signal"

	"github.com/hovsep/fmesh/component"
)

// ControllerQueueItem holds the information about frame being sent
type ControllerQueueItem struct {
	FrameBits   Bits // Frame in bits representation
	BitsWritten int  // How many bits are already transmitted
}

type ControllerTxQueue []*ControllerQueueItem

const (
	ControllerStateTxQueue = "tx_queue"
)

// NewController creates a CAN controller
// which converts frames to bits and vice versa
func NewController(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithInitialState(func(state component.State) {
			state.Set(ControllerStateTxQueue, ControllerTxQueue{})
		}).
		WithActivationFunc(func(this *component.Component) error {
			framesQueue := this.State().Get(ControllerStateTxQueue).(ControllerTxQueue)
			defer func() {
				this.State().Set(ControllerStateTxQueue, framesQueue)
			}()

			// TODO (write path):
			// continue transmitting from the latest bit and compare with latest read bit, check if arbitration is lost and retry

			// Handle new frames coming from MCU
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*Frame)
				if !ok || !frame.isValid() {
					return errors.New("controller received corrupted frame")
				}

				framesQueue = append(framesQueue, &ControllerQueueItem{
					FrameBits:   frame.toBits(),
					BitsWritten: 0,
				})
				this.Logger().Println("got a frame to send: ", frame.toBits().String())
			}

			// Check if there are frames to write and pop one
			if len(framesQueue) > 0 {
				itemToWrite := framesQueue[0]

				// Write next bit
				if itemToWrite.BitsWritten < len(itemToWrite.FrameBits) {
					this.OutputByName(PortCANTx).PutSignals(signal.New(itemToWrite.FrameBits[itemToWrite.BitsWritten+1]))
					itemToWrite.BitsWritten++
				}

			}

			return nil
		}).
		WithInputs(PortCANTx, PortCANRx). // Frame in, bits in
		WithOutputs(PortCANTx, PortCANRx) // Bits out, frame out
}

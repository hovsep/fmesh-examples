package ecu

// AcceptedIds indicates which frame IDs given ECU listens to and processes
type AcceptedIds map[uint32]bool

const (
	// Common ECU memory keys
	ecuMemCanID       = "can_id"       // The ID of given CAN node
	ecuMemSerial      = "serial_no"    // Device serial number
	ecuMemLog         = "log"          // Internal log
	ecuMemAcceptedIds = "accepted_ids" // Instance of AcceptedIds
)

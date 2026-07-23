package protocol

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// SignalUpdate represents a parsed signal update from the protocol
type SignalUpdate struct {
	Key   string
	Value float64
}

// Reader reads from the Unix socket and parses the text protocol
type Reader struct {
	socketPath string
	conn       net.Conn
	Updates    chan SignalUpdate
	Errors     chan error
}

// NewReader creates a new protocol reader
func NewReader(socketPath string) *Reader {
	return &Reader{
		socketPath: socketPath,
		Updates:    make(chan SignalUpdate, 100),
		Errors:     make(chan error, 10),
	}
}

// Connect establishes connection to the Unix socket
func (r *Reader) Connect() error {
	conn, err := net.Dial("unix", r.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to socket %s: %w", r.socketPath, err)
	}

	r.conn = conn
	return nil
}

// Start begins reading from the socket in a goroutine
func (r *Reader) Start() {
	go r.read()
}

func (r *Reader) read() {
	defer func() {
		if r.conn != nil {
			r.conn.Close()
		}
	}()

	scanner := bufio.NewScanner(r.conn)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		update, err := r.parseLine(line)
		if err != nil {
			if !strings.Contains(err.Error(), "skipping non-numeric") {
				select {
				case r.Errors <- err:
				default:
				}
			}
			continue
		}

		select {
		case r.Updates <- update:
		default:
			<-r.Updates
			r.Updates <- update
		}
	}

	if err := scanner.Err(); err != nil {
		r.Errors <- fmt.Errorf("scanner error: %w", err)
	}
}

// parseLine parses a single line from the protocol (format: "<key> <value>\n")
func (r *Reader) parseLine(line string) (SignalUpdate, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return SignalUpdate{}, fmt.Errorf("invalid line format: %s", line)
	}

	key := parts[0]
	valueStr := parts[1]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return SignalUpdate{}, fmt.Errorf("skipping non-numeric value for %s: %s", key, valueStr)
	}

	return SignalUpdate{
		Key:   key,
		Value: value,
	}, nil
}

// Close closes the connection and channels
func (r *Reader) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
	close(r.Updates)
	close(r.Errors)
}

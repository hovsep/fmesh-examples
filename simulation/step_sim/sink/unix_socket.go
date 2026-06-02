package sink

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

type ClientsRegistry struct {
	mu      sync.Mutex
	Clients map[net.Conn]struct{}
}

type UnixSocketSink struct {
	socketPath      string
	listener        net.Listener
	clientsRegistry ClientsRegistry
	stream          chan string
	done            chan struct{}
	closeOnce       sync.Once
	wg              sync.WaitGroup
	dropped         uint64
}

func newClientsRegistry() ClientsRegistry {
	return ClientsRegistry{
		Clients: make(map[net.Conn]struct{}),
	}
}

func NewUnixSocketSink(socketPath string) (*UnixSocketSink, error) {
	listener, err := getListener(socketPath)
	if err != nil {
		return nil, err
	}

	streamChan := make(chan string, 1000)
	sink := &UnixSocketSink{
		socketPath:      socketPath,
		clientsRegistry: newClientsRegistry(),
		stream:          streamChan,
		done:            make(chan struct{}),
		listener:        listener,
	}

	// Accept connection from socket
	sink.wg.Add(1)
	go sink.acceptConnections()

	// Broadcast aggregated state updates among clients
	sink.wg.Add(1)
	go sink.broadcast()
	return sink, nil
}

// Close stops the sink goroutines and releases the listener.
// It is safe to call multiple times.
func (s *UnixSocketSink) Close() error {
	var err error
	s.closeOnce.Do(func() {
		fmt.Println("Shutting down the sink...")

		// Signal goroutines to stop and unblock the blocking Accept call
		close(s.done)
		if closeErr := s.listener.Close(); closeErr != nil {
			err = fmt.Errorf("failed to close listener: %w", closeErr)
		}

		// Wait for acceptConnections and broadcast to finish
		s.wg.Wait()

		// Close any remaining client connections
		s.clientsRegistry.mu.Lock()
		for c := range s.clientsRegistry.Clients {
			_ = c.Close()
			delete(s.clientsRegistry.Clients, c)
		}
		s.clientsRegistry.mu.Unlock()
	})
	return err
}

// Publish is non-blocking and lossy on purpose: telemetry must never
// apply back-pressure to the simulation loop. Dropped lines are counted.
func (s *UnixSocketSink) Publish(line string) error {
	select {
	case <-s.done:
		return nil
	case s.stream <- line:
		return nil
	default:
		atomic.AddUint64(&s.dropped, 1)
		return nil
	}
}

// Dropped returns the number of lines dropped because the buffer was full.
func (s *UnixSocketSink) Dropped() uint64 {
	return atomic.LoadUint64(&s.dropped)
}

func (s *UnixSocketSink) acceptConnections() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Stop cleanly when the sink is shutting down or the listener is closed
			select {
			case <-s.done:
				fmt.Println("Stopping accepting connections to the sink...")
				return
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				fmt.Println("Stopping accepting connections to the sink...")
				return
			}
			fmt.Println("accept error:", err)
			continue
		}

		fmt.Println("New client connected")
		s.clientsRegistry.Add(conn)
	}
}

func (s *UnixSocketSink) broadcast() {
	defer s.wg.Done()

	for {
		select {
		case <-s.done:
			fmt.Println("Stopping broadcasting...")
			return
		case line := <-s.stream:
			s.clientsRegistry.mu.Lock()
			for c := range s.clientsRegistry.Clients {
				if _, err := fmt.Fprintln(c, line); err != nil {
					// Remove disconnected clients (always drop, regardless of Close result)
					if closeErr := c.Close(); closeErr != nil {
						fmt.Println("error closing client:", closeErr)
					}
					delete(s.clientsRegistry.Clients, c)
					fmt.Println("Client disconnected")
				}
			}
			s.clientsRegistry.mu.Unlock()
		}
	}
}

func getListener(socketPath string) (net.Listener, error) {
	// Remove old socket if exists
	if socketPath == "" {
		return nil, errors.New("socket path cannot be empty")
	}
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	fmt.Println("Sink listening on", socketPath)
	return listener, nil
}

func (c *ClientsRegistry) Add(conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Clients[conn] = struct{}{}
}

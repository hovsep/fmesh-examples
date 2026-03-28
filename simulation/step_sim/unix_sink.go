package step_sim

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
)

type ClientsRegistry struct {
	sync.Mutex
	Clients map[net.Conn]struct{}
}

type UnixSink struct {
	ctx             context.Context
	listener        net.Listener
	clientsRegistry ClientsRegistry
	stream          chan string
}

func newClientsRegistry() ClientsRegistry {
	return ClientsRegistry{
		Clients: make(map[net.Conn]struct{}),
	}
}

func NewUnixSink(ctx context.Context, socketPath string) (*UnixSink, error) {
	listener, err := getListener(socketPath)
	if err != nil {
		return nil, err
	}

	streamChan := make(chan string, 1000)
	sink := &UnixSink{
		ctx:             ctx,
		clientsRegistry: newClientsRegistry(),
		stream:          streamChan,
		listener:        listener,
	}

	// Accept connection from socket
	go sink.acceptConnections()

	// Broadcast aggregated state updates among clients
	go sink.broadcast()
	return sink, nil
}

func (s *UnixSink) Close() error {
	fmt.Println("Shutting down the sink...")
	err := s.listener.Close()
	if err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}
	return nil
}

func (s *UnixSink) Publish(line string) error {
	s.stream <- line
	return nil
}

func (s *UnixSink) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			fmt.Println("Stopping accepting connections to the sink...")
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				fmt.Println("accept error:", err)
				continue
			}

			fmt.Println("New client connected")
			s.clientsRegistry.Add(conn)
		}

	}
}

func (s *UnixSink) broadcast() {
	for line := range s.stream {
		select {
		case <-s.ctx.Done():
			fmt.Println("Stopping broadcasting...")
			return
		default:
			s.clientsRegistry.Lock()
			for c := range s.clientsRegistry.Clients {
				_, err := fmt.Fprintln(c, line)
				if err != nil {
					// Remove disconnected clients
					err := c.Close()
					if err != nil {
						return
					}
					fmt.Println("Client disconnected")
					delete(s.clientsRegistry.Clients, c)
				}
			}
			s.clientsRegistry.Unlock()
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
	c.Lock()
	defer c.Unlock()
	c.Clients[conn] = struct{}{}
}

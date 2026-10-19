package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The engine runs in a browser. The page sends the driver's hands and feet
// over plain HTTP, and gets the state of every part back as a stream of
// server-sent events: the mesh is the model, the page only draws it. No
// dependency beyond the standard library, and the page is one file.

//go:embed web/index.html
var page []byte

// snapshot is what the page draws: every part's own state, as it is.
type snapshot struct {
	Running         bool            `json:"running"`
	Reason          string          `json:"reason"` // why it stopped
	Driver          DriverInput     `json:"driver"`
	FiringOrder     [cylinders]int  `json:"firingOrder"`
	CyclesPerStroke float64         `json:"cyclesPerStroke"` // mesh cycles each trip around the engine takes
	Parts           json.RawMessage `json:"parts"`
}

type server struct {
	fm     *fmesh.FMesh
	driver *Driver
	start  chan struct{}
	sleep  func(time.Duration) // how a stroke waits for the wall clock; tests make it a no-op

	mu        sync.Mutex
	running   bool
	reason    string
	parts     []byte // every part's state, as of the last stroke published
	perStroke float64
	latest    []byte
	listeners map[chan []byte]struct{}
}

// newServer wraps the engine for the page. It is not running yet: call
// runEngine on its own goroutine and serve handler.
func newServer(fm *fmesh.FMesh, driver *Driver) *server {
	s := &server{
		fm: fm, driver: driver,
		start:     make(chan struct{}, 1),
		sleep:     time.Sleep,
		reason:    "press Start",
		listeners: map[chan []byte]struct{}{},
	}
	s.publish()
	return s
}

// handler is the page and its two endpoints.
func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /events", s.events)
	mux.HandleFunc("POST /control", s.control)
	return mux
}

// serve runs the engine and serves its page at addr (":0" picks a free port).
func serve(addr string, fm *fmesh.FMesh, driver *Driver) error {
	s := newServer(fm, driver)
	go s.runEngine()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("Open http://%s in a browser. Ctrl+C to quit.\n", ln.Addr())
	return (&http.Server{Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second}).Serve(ln)
}

// runEngine turns the key whenever Start is pressed and lets the mesh run
// until the engine stops by itself.
func (s *server) runEngine() {
	var (
		lastStroke  = -1
		cycles      int // since the last stroke
		lastPublish time.Time
		simulated   float64 // seconds since the pacing clock was set
		clockStart  time.Time
	)
	s.fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(context.Context, *fmesh.CycleContext) error {
			cycles++
			crank := state[Crank](s.fm, "crankshaft", "crank")
			if crank.Stroke == lastStroke {
				return nil
			}
			if lastStroke >= 0 {
				s.mu.Lock()
				s.perStroke = s.perStroke*0.9 + float64(cycles)*0.1
				s.mu.Unlock()
			}
			lastStroke, cycles = crank.Stroke, 0

			// Keep simulated time in step with the wall clock.
			if clockStart.IsZero() {
				clockStart, simulated = time.Now(), 0
			}
			simulated += crank.DT
			wait := time.Until(clockStart.Add(time.Duration(simulated * float64(time.Second))))
			switch {
			case wait > 0:
				s.sleep(wait)
			case wait < -200*time.Millisecond:
				clockStart, simulated = time.Now(), 0 // fell behind: do not try to catch up
			}

			if time.Since(lastPublish) >= 33*time.Millisecond {
				lastPublish = time.Now()
				s.publish()
			}
			return nil
		})
	})

	for range s.start {
		clockStart = time.Time{}

		err := s.fm.ComponentByName("crankshaft").InputByName("key").PutSignals(signal.New("start"))
		if err == nil {
			_, err = s.fm.Run(context.Background())
		}

		s.driver.Set(func(d *DriverInput) { d.Ignition = false })
		s.mu.Lock()
		s.running, s.reason = false, stopReason(s.fm)
		if err != nil {
			s.reason = err.Error()
		}
		s.mu.Unlock()
		s.publish()
	}
}

// stopReason says why the engine is standing still.
func stopReason(fm *fmesh.FMesh) string {
	switch mode := state[string](fm, "ecu", "mode"); {
	case state[float64](fm, "fuel-tank", "liters") == 0:
		return "the tank is empty"
	case mode == "cranking" && !state[bool](fm, "starter", "engaged"):
		return "the battery is too weak to turn the starter"
	case mode == "no start":
		return "it would not start"
	case mode == "stalled":
		return "it stalled"
	default:
		return "the key was turned off"
	}
}

func state[T any](fm *fmesh.FMesh, componentName, key string) T {
	v, _ := fm.ComponentByName(componentName).State().Get(key).(T)
	return v
}

// publish sends every part's state to every open page. It runs on the
// mesh's goroutine, between cycles, or while the mesh is not running, so it
// never reads a state that a part is writing.
func (s *server) publish() {
	parts := map[string]component.State{}
	for c := range s.fm.Components().All() {
		parts[c.Name()] = c.State()
	}
	data, err := json.Marshal(parts)
	if err != nil {
		fmt.Println("Failed to encode the engine's state:", err)
		return
	}
	s.mu.Lock()
	s.parts = data
	s.mu.Unlock()
	s.broadcast()
}

// broadcast sends the latest snapshot to every open page.
func (s *server) broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(snapshot{
		Running: s.running, Reason: s.reason,
		Driver: s.driver.peek(), FiringOrder: firingOrder, CyclesPerStroke: s.perStroke, Parts: s.parts,
	})
	if err != nil {
		fmt.Println("Failed to encode the engine's state:", err)
		return
	}
	s.latest = data
	for ch := range s.listeners {
		select {
		case <-ch: // a slow page skips a frame rather than holding up the engine
		default:
		}
		ch <- data
	}
}

// events streams snapshots to one page.
func (s *server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch := make(chan []byte, 1)
	s.mu.Lock()
	s.listeners[ch] = struct{}{}
	ch <- s.latest
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.listeners, ch)
		s.mu.Unlock()
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// control takes the driver's input from the page. Every field is optional.
func (s *server) control(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pedal    *float64 `json:"pedal"`
		Ignition *bool    `json:"ignition"`
		Refuel   bool     `json:"refuel"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.driver.Set(func(d *DriverInput) {
		if req.Pedal != nil {
			d.Pedal = *req.Pedal
		}
		if req.Ignition != nil {
			d.Ignition = *req.Ignition
		}
		d.Refuel = d.Refuel || req.Refuel
	})

	// Turning the key on a still engine starts it. The engine counts as
	// running from this moment, so a second press cannot queue a second
	// start that would turn the key again after the engine stops.
	s.mu.Lock()
	starting := req.Ignition != nil && *req.Ignition && !s.running
	if starting {
		s.running, s.reason = true, ""
	}
	running := s.running
	s.mu.Unlock()
	if starting {
		s.start <- struct{}{}
	}
	if !running || starting {
		s.broadcast() // show the switch move even while the mesh is still
	}
	w.WriteHeader(http.StatusNoContent)
}

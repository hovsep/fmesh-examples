package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
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
	Scale           float64         `json:"scale"`  // simulated seconds per real second
	Driver          DriverInput     `json:"driver"`
	FiringOrder     [cylinders]int  `json:"firingOrder"`
	CyclesPerStroke float64         `json:"cyclesPerStroke"` // mesh cycles each trip around the engine takes
	Parts           json.RawMessage `json:"parts"`
}

type server struct {
	fm     *fmesh.FMesh
	driver *Driver
	start  chan struct{}

	mu        sync.Mutex
	running   bool
	reason    string
	scale     float64
	parts     []byte // every part's state, as of the last stroke published
	perStroke float64
	latest    []byte
	listeners map[chan []byte]struct{}
}

func serve(addr string, fm *fmesh.FMesh, driver *Driver) error {
	s := &server{
		fm: fm, driver: driver,
		start:     make(chan struct{}, 1),
		scale:     1,
		reason:    "press Start",
		listeners: map[chan []byte]struct{}{},
	}
	s.publish()
	go s.runEngine()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /events", s.events)
	mux.HandleFunc("POST /control", s.control)
	return (&http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}).ListenAndServe()
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
		clockScale  float64
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

			// Keep simulated time in step with the wall clock (times the scale).
			scale := s.timeScale()
			if scale != clockScale || clockStart.IsZero() {
				clockStart, clockScale, simulated = time.Now(), scale, 0
			}
			simulated += crank.DT
			wait := time.Until(clockStart.Add(time.Duration(simulated / scale * float64(time.Second))))
			switch {
			case wait > 0:
				time.Sleep(wait)
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
		s.mu.Lock()
		s.running, s.reason = true, ""
		s.mu.Unlock()
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

func (s *server) timeScale() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scale
}

// publish sends every part's state to every open page. It runs on the
// mesh's goroutine, between cycles, or while the mesh is not running, so it
// never reads a state that a part is writing.
func (s *server) publish() {
	parts := map[string]component.State{}
	for _, c := range s.fm.Components().AllOrdered() {
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
		Running: s.running, Reason: s.reason, Scale: s.scale,
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
		AC       *bool    `json:"ac"`
		Lights   *bool    `json:"lights"`
		Refuel   bool     `json:"refuel"`
		Scale    *float64 `json:"scale"`
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
		if req.AC != nil {
			d.AC = *req.AC
		}
		if req.Lights != nil {
			d.Lights = *req.Lights
		}
		d.Refuel = d.Refuel || req.Refuel
	})

	s.mu.Lock()
	if req.Scale != nil {
		s.scale = clamp(*req.Scale, 0.01, 1)
	}
	running := s.running
	s.mu.Unlock()

	if req.Ignition != nil && *req.Ignition && !running {
		select {
		case s.start <- struct{}{}:
		default: // already starting
		}
	}
	if !running {
		s.broadcast() // show the switch move even while the mesh is still
	}
	w.WriteHeader(http.StatusNoContent)
}

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/hovsep/fmesh"
)

//go:embed index.html
var page []byte

// building runs the mesh in real time and fans every frame out to the
// browsers watching it.
type building struct {
	fm *fmesh.FMesh

	mu       sync.Mutex
	inputs   []Input
	watchers map[chan []byte]bool
}

// serve runs the building and serves the page that shows it.
func serve(fm *fmesh.FMesh, addr string, demo bool) error {
	b := &building{fm: fm, watchers: map[chan []byte]bool{}}
	go b.run(demo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /events", b.events)
	mux.HandleFunc("POST /press", b.press)

	fmt.Printf("The building is open: http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// run is the building's life: one step of the mesh every 50 ms.
func (b *building) run(demo bool) {
	visitors := rand.New(rand.NewPCG(1, 2))
	ticker := time.NewTicker(time.Duration(dt * float64(time.Second)))
	defer ticker.Stop()
	var frame Frame
	for range ticker.C {
		b.mu.Lock()
		inputs := b.inputs
		b.inputs = nil
		b.mu.Unlock()
		if demo {
			inputs = append(inputs, visit(visitors, frame)...)
		}

		var cycles int
		var err error
		frame, cycles, err = step(b.fm, inputs)
		if err != nil {
			fmt.Println("Step failed:", err)
			continue
		}
		msg, err := json.Marshal(map[string]any{"parts": frame, "cycles": cycles, "components": b.fm.Components().Len()})
		if err != nil {
			fmt.Println("Frame failed:", err)
			continue
		}

		b.mu.Lock()
		for w := range b.watchers {
			select {
			case w <- msg:
			default: // a slow browser skips a frame rather than holding up the building
			}
		}
		b.mu.Unlock()
	}
}

// events streams frames to one browser as server-sent events.
func (b *building) events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	frames := make(chan []byte, 1)
	b.mu.Lock()
	b.watchers[frames] = true
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.watchers, frames)
		b.mu.Unlock()
	}()

	flusher, _ := w.(http.Flusher)
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-frames:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// press takes one click from the page: {"part": "hall-up-3", "port": "press"}.
func (b *building) press(w http.ResponseWriter, r *http.Request) {
	var in Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := target(b.fm, in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	b.mu.Lock()
	b.inputs = append(b.inputs, in)
	b.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// visit is the demo's visitors: now and then someone calls a cab, and
// whoever gets into one picks a floor.
func visit(rnd *rand.Rand, frame Frame) []Input {
	var inputs []Input
	if rnd.Float64() < dt/4 { // a call every four seconds or so
		f := 1 + rnd.IntN(floors)
		dir := "up"
		if f == floors || (f > 1 && rnd.IntN(2) == 0) {
			dir = "down"
		}
		inputs = append(inputs, Input{Part: fmt.Sprintf("hall-%s-%d", dir, f), Port: portPress})
	}
	for _, c := range cabs {
		ctl, _ := frame["controller-"+c].(ControllerState)
		door, _ := frame["door-operator-"+c].(DoorState)
		if ctl.Mode == modeDwell && door.Open && rnd.Float64() < dt {
			inputs = append(inputs, Input{Part: fmt.Sprintf("car-%s-button-%d", c, 1+rnd.IntN(floors)), Port: portPress})
		}
	}
	return inputs
}

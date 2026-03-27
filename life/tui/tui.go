package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"
)

type Event struct {
	Kind  string
	Value float64
	Int   int
}
type State struct {
	HeartRate int

	HCA []float64
	BA  []float64

	MaxPoints int
}

func main() {
	conn, err := net.Dial("unix", "/tmp/habitat_mesh.sock")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	const maxPoints = 50

	events := make(chan Event, 1000)
	stateCh := make(chan State, 1)

	go ingest(conn, events)
	go stateManager(events, stateCh, maxPoints)
	renderLoop(stateCh)
}

func parseLine(line string) (Event, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Event{}, false
	}

	key := fields[0]
	val := fields[1]

	switch {
	case strings.HasPrefix(key, "human-Leon::heart_rate"):
		i, err := strconv.Atoi(val)
		if err != nil {
			return Event{}, false
		}
		return Event{Kind: "hr", Int: i}, true

	case strings.HasPrefix(key, "human-Leon::heart_cardiac_activation"):
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return Event{}, false
		}
		return Event{Kind: "hca", Value: f}, true

	case strings.HasPrefix(key, "human-Leon::brain_activity"):
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return Event{}, false
		}
		return Event{Kind: "ba", Value: f}, true
	}

	return Event{}, false
}

func stateManager(events <-chan Event, stateCh chan<- State, maxPoints int) {
	state := State{
		MaxPoints: maxPoints,
		HCA:       make([]float64, 0, maxPoints),
		BA:        make([]float64, 0, maxPoints),
	}

	for e := range events {
		switch e.Kind {
		case "hr":
			state.HeartRate = e.Int

		case "hca":
			state.HCA = append(state.HCA, e.Value)
			if len(state.HCA) > state.MaxPoints {
				state.HCA = state.HCA[1:]
			}

		case "ba":
			state.BA = append(state.BA, e.Value)
			if len(state.BA) > state.MaxPoints {
				state.BA = state.BA[1:]
			}
		}

		// non-blocking snapshot publish
		select {
		case stateCh <- state:
		default:
		}
	}
}

func ingest(conn net.Conn, out chan<- Event) {
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		if e, ok := parseLine(scanner.Text()); ok {
			out <- e
		}
	}
}

func renderLoop(stateCh <-chan State) {
	ticker := time.NewTicker(100 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	var latest State

	for {
		select {
		case s := <-stateCh:
			latest = s

		case <-ticker.C:
			draw(latest)
		}
	}
}

func draw(s State) {
	fmt.Print("\033[H\033[2J")

	if len(s.HCA) > 0 {
		fmt.Println(
			asciigraph.Plot(
				s.HCA,
				asciigraph.Width(80),
				asciigraph.Height(5),
				asciigraph.SeriesColors(asciigraph.Red),
				asciigraph.Caption("ECG (Heart) — BPM: "+strconv.Itoa(s.HeartRate)),
			),
		)
	}

	if len(s.BA) > 0 {
		fmt.Println(
			asciigraph.Plot(
				s.BA,
				asciigraph.Width(80),
				asciigraph.Height(5),
				asciigraph.SeriesColors(asciigraph.Blue),
				asciigraph.Caption("Brain activity"),
			),
		)
	}
}

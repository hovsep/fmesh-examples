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
	Key   string
	Value float64
}

type State struct {
	Signals   map[string][]float64
	MaxPoints int
}

type SignalConfig struct {
	Key   string
	Label string
	Color asciigraph.AnsiColor
}

func main() {
	conn, err := net.Dial("unix", "/tmp/habitat_mesh.sock")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rows := []SignalConfig{
		//{
		//	Key:   "human-Leon::heart_rate",
		//	Label: "Heart Rate (BPM)",
		//	Color: asciigraph.Red,
		//},
		/*{
			Key:   "human-Leon::heart_cardiac_activation",
			Label: "Cardiac Activation",
			Color: asciigraph.Green,
		},*/
		/*{
			Key:   "human-Leon::brain_activity",
			Label: "Brain Activity",
			Color: asciigraph.Blue,
		},
		*/
		{
			Key:   "human-Leon::pleural_pressure",
			Label: "Pleural Pressure (cmH₂O)",
			Color: asciigraph.Orange,
		},

		{
			Key:   "human-Leon::lung_left_flow",
			Label: "Left Lung Airflow (mL/s)  [+ = inspiration, - = expiration]",
			Color: asciigraph.Pink,
		},
		{
			Key:   "human-Leon::lung_right_flow",
			Label: "Right Lung Airflow (mL/s)  [+ = inspiration, - = expiration]",
			Color: asciigraph.Pink,
		},
		{
			Key:   "human-Leon::lung_left_volume",
			Label: "Left Lung Volume (mL)",
			Color: asciigraph.Blue,
		},
		{
			Key:   "human-Leon::lung_right_volume",
			Label: "Right Lung Volume (mL)",
			Color: asciigraph.Red,
		},
		{
			Key:   "human-Leon::lung_left_alveolar_pressure",
			Label: "Left Lung AP",
			Color: asciigraph.Blue,
		},
		{
			Key:   "human-Leon::lung_right_alveolar_pressure",
			Label: "Right Lung AP",
			Color: asciigraph.Red,
		},
	}

	configMap := make(map[string]SignalConfig)
	for _, r := range rows {
		configMap[r.Key] = r
	}

	// One breath cycle at 12 BPM ≈ 5 s. With ~100 data points/second (10 ms
	// real-time sleep per tick in initSim), 500 points covers exactly one cycle,
	// filling the 80-column plot with a complete, readable waveform.
	const maxPoints = 2000

	events := make(chan Event, 1000)
	stateCh := make(chan State, 1)

	go ingest(conn, events, configMap)
	go stateManager(events, stateCh, maxPoints)
	renderLoop(stateCh, rows)
}

// ---------------- parsing ----------------

func parseLine(line string, cfg map[string]SignalConfig) (Event, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Event{}, false
	}

	key := fields[0]
	val := fields[1]

	for _, s := range cfg {
		if strings.HasPrefix(key, s.Key) {

			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return Event{}, false
			}
			return Event{Key: s.Key, Value: f}, true
		}
	}

	return Event{}, false
}

func ingest(conn net.Conn, out chan<- Event, cfg map[string]SignalConfig) {
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		if e, ok := parseLine(scanner.Text(), cfg); ok {
			out <- e
		}
	}
}

// ---------------- state ----------------

func stateManager(events <-chan Event, stateCh chan<- State, maxPoints int) {
	state := State{
		Signals:   map[string][]float64{},
		MaxPoints: maxPoints,
	}

	for e := range events {

		buf := state.Signals[e.Key]
		buf = append(buf, e.Value)

		if len(buf) > state.MaxPoints {
			buf = buf[1:]
		}

		state.Signals[e.Key] = buf

		select {
		case stateCh <- state:
		default:
		}
	}
}

// ---------------- render ----------------

func renderLoop(stateCh <-chan State, rows []SignalConfig) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var latest State

	for {
		select {
		case s := <-stateCh:
			latest = s

		case <-ticker.C:
			draw(latest, rows)
		}
	}
}

func draw(s State, rows []SignalConfig) {
	fmt.Print("\033[H\033[2J")

	for _, sig := range rows {
		data := s.Signals[sig.Key]

		if len(data) == 0 {
			continue
		}

		caption := sig.Label

		fmt.Println(
			asciigraph.Plot(
				data,
				asciigraph.Width(1000),
				asciigraph.Height(50),
				asciigraph.SeriesColors(sig.Color),
				asciigraph.Caption(caption),
			),
		)
	}
}

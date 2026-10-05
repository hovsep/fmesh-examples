package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// Two elevators in a six-floor building. The UI and the mesh talk both ways:
// button presses go in as signals, and the state of the building comes back
// out as a signal the UI draws.
//
//	UI → mesh:  press → floor-N, button → cab-X, tick → cab-X
//	calls:      floor-N → dispatcher → cab-X, and served → floor-N (lamp off)
//	each cab:   cab-X → motor-X → cab-X, cab-X → door-X → cab-X
//	mesh → UI:  cab-X status, floor-N lamp → display → view
//
// Every Run of the mesh is one second of the building's life. Components
// keep their state between runs, so the mesh is the building and the UI only
// pushes time and buttons into it.
//
// Run: go run .   (or go run . -demo for a scripted rush hour)

const floors = 6

var cabs = []string{"A", "B"}

func main() {
	demo := flag.Bool("demo", false, "play a scripted scenario instead of reading buttons from the keyboard")
	tick := flag.Duration("tick", time.Second, "how long one simulated second takes")
	flag.Parse()

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	handled, err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	if *demo || !isTerminal(os.Stdin) {
		if err := playDemo(fm, *tick/3); err != nil {
			fmt.Println("Elevator failed:", err)
			os.Exit(1)
		}
		return
	}
	if err := playInteractive(fm, *tick); err != nil {
		fmt.Println("Elevator failed:", err)
		os.Exit(1)
	}
}

// step pushes one second and any button presses into the mesh, runs it, and
// returns the view of the building the mesh sends back.
func step(fm *fmesh.FMesh, presses []string) (View, error) {
	for _, p := range presses {
		target, err := parsePress(fm, p)
		if err != nil {
			return View{}, err
		}
		if err := target.PutSignals(signal.New(pressedFloor(p))); err != nil {
			return View{}, err
		}
	}
	for _, c := range cabs {
		if err := fm.ComponentByName(cabName(c)).InputByName(portTick).PutSignals(signal.New(1)); err != nil {
			return View{}, err
		}
	}

	if _, err := fm.Run(context.Background()); err != nil {
		return View{}, err
	}

	views := fm.ComponentByName("display").OutputByName(portView).Signals().All()
	return views[len(views)-1].PayloadOrDefault(View{}), nil
}

// parsePress turns what the user typed into the port it presses:
// "4" is the call button on floor 4, "a4" is button 4 inside cab A.
func parsePress(fm *fmesh.FMesh, p string) (*port.Port, error) {
	f := pressedFloor(p)
	if f < 1 || f > floors {
		return nil, fmt.Errorf("no such floor in %q (1-%d)", p, floors)
	}
	if len(p) == 1 {
		return fm.ComponentByName(floorName(f)).InputByName(portPress), nil
	}
	cab := fm.ComponentByName(cabName(strings.ToUpper(p[:1])))
	if cab == nil {
		return nil, fmt.Errorf("no such cab in %q", p)
	}
	return cab.InputByName(portButton), nil
}

func pressedFloor(p string) int {
	f, _ := strconv.Atoi(p[len(p)-1:])
	return f
}

// The demo: people call cabs and ride them, then the building goes quiet.
var script = map[int][]string{
	1:  {"5"},      // someone on 5 wants to go down
	2:  {"3"},      // and someone on 3
	7:  {"a1"},     // the one picked up on 5 rides to the lobby
	8:  {"b6"},     // the one on 3 goes up to 6
	10: {"2", "4"}, // two more calls while both cabs are busy
	16: {"a3"},     // the one picked up on 2 goes to 3
	18: {"b1"},     // the one picked up on 4 goes down to the lobby
}

func playDemo(fm *fmesh.FMesh, delay time.Duration) error {
	fmt.Println("=== Elevator (demo) ===")
	for t := 1; ; t++ {
		view, err := step(fm, script[t])
		if err != nil {
			return err
		}
		fmt.Printf("\nsecond %d  %s\n%s", t, strings.Join(script[t], " "), render(view))
		if t > 18 && idle(view) {
			fmt.Println("\nEverybody has arrived; the building is quiet.")
			return nil
		}
		time.Sleep(delay)
	}
}

func playInteractive(fm *fmesh.FMesh, tick time.Duration) error {
	presses := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			presses <- strings.TrimSpace(scanner.Text())
		}
		close(presses)
	}()

	fmt.Print("\033[H\033[2J")
	help := fmt.Sprintf("Call a cab: 1-%d   Ride: a1-a%d, b1-b%d   Quit: q   (then Enter)", floors, floors, floors)
	var pending []string
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for t := 1; ; t++ {
		view, err := step(fm, pending)
		if err != nil {
			fmt.Println(err)
		}
		pending = nil
		// Redraw the building in place, without touching the line being typed.
		frame := fmt.Sprintf("=== Elevator ===  second %d\n%s%s\n", t, render(view), help)
		fmt.Print("\0337\033[H" + strings.ReplaceAll(frame, "\n", "\033[K\n") + "\0338")
		if t == 1 {
			fmt.Print("\n> ")
		}

		select {
		case p, ok := <-presses:
			if !ok || p == "q" {
				return nil
			}
			if _, err := parsePress(fm, p); err == nil {
				pending = append(pending, p)
			}
			fmt.Print("> ")
			<-ticker.C
		case <-ticker.C:
		}
	}
}

// render draws the shafts: one column per cab, one row per floor, the call
// lamps on the right. A cab is [▲] or [▼] while moving, [■] when idle and
// [ ] with its doors open; a dot marks a floor the cab will stop at.
func render(v View) string {
	var b strings.Builder
	b.WriteString("       A     B   call\n")
	for f := floors; f >= 1; f-- {
		fmt.Fprintf(&b, "  %d ", f)
		for _, c := range cabs {
			st := v.Cabs[c]
			cell := "     "
			switch {
			case st.Floor == f && st.Door == doorOpen:
				cell = " [ ] "
			case st.Floor == f && st.Dir > 0:
				cell = " [▲] "
			case st.Floor == f && st.Dir < 0:
				cell = " [▼] "
			case st.Floor == f:
				cell = " [■] "
			case contains(st.Stops, f):
				cell = "  ·  "
			}
			b.WriteString("│" + cell)
		}
		lamp := ""
		if v.Lamps[f] {
			lamp = "●"
		}
		fmt.Fprintf(&b, "│  %s\n", lamp)
	}
	return b.String()
}

func idle(v View) bool {
	for _, st := range v.Cabs {
		if st.Dir != 0 || st.Door == doorOpen || len(st.Stops) > 0 {
			return false
		}
	}
	for _, on := range v.Lamps {
		if on {
			return false
		}
	}
	return true
}

func contains(s []int, n int) bool {
	for _, x := range s {
		if x == n {
			return true
		}
	}
	return false
}

// getMesh builds the building.
func getMesh() (*fmesh.FMesh, error) {
	fm, err := fmesh.New("elevator",
		fmesh.WithDescription("two cabs, six floors: buttons in, building state out, one run per second"),
	)
	if err != nil {
		return nil, err
	}

	dispatcher, err := newDispatcher(cabs)
	if err != nil {
		return nil, err
	}
	display, err := newDisplay()
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(dispatcher, display); err != nil {
		return nil, err
	}

	for f := 1; f <= floors; f++ {
		floor, err := newFloor(f)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(floor); err != nil {
			return nil, err
		}
		if err := port.MultiPipe(
			port.Pipe{From: floor.OutputByName(portCall), To: dispatcher.InputByName(portCall)},
			port.Pipe{From: dispatcher.OutputByName(fmt.Sprint(portServed, f)), To: floor.InputByName(portServed)},
			port.Pipe{From: floor.OutputByName(portLamp), To: display.InputByName(portLamp)},
		); err != nil {
			return nil, err
		}
	}

	for _, c := range cabs {
		cab, err := newCab(c)
		if err != nil {
			return nil, err
		}
		motor, err := newMotor(c)
		if err != nil {
			return nil, err
		}
		door, err := newDoor(c)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(cab, motor, door); err != nil {
			return nil, err
		}
		if err := port.MultiPipe(
			// The cab's two feedback loops: it commands, the motor and the door report back.
			port.Pipe{From: cab.OutputByName(portDrive), To: motor.InputByName(portCmd)},
			port.Pipe{From: motor.OutputByName(portPosition), To: cab.InputByName(portPosition)},
			port.Pipe{From: cab.OutputByName(portDoors), To: door.InputByName(portCmd)},
			port.Pipe{From: door.OutputByName(portState), To: cab.InputByName(portState)},
			// And the loop through the dispatcher.
			port.Pipe{From: cab.OutputByName(portStatus), To: dispatcher.InputByName(portStatus)},
			port.Pipe{From: dispatcher.OutputByName(portAssign + "-" + c), To: cab.InputByName(portAssign)},
			port.Pipe{From: cab.OutputByName(portStatus), To: display.InputByName(portStatus)},
		); err != nil {
			return nil, err
		}
	}

	return fm, nil
}

// isTerminal reports whether f is an interactive terminal rather than a file
// or a pipe.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

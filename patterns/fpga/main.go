package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
)

// An FPGA is a board of generic, identical logic cells. You do not run a
// program on it; you describe a circuit in a hardware description language,
// synthesis turns it into a configuration, and flashing that configuration
// makes the cells become the circuit.
//
// Here the board is a mesh: every cell, pin and the clock is a component,
// every wire a pipe. A Verilog-like file is synthesised into look-up tables
// at runtime, and the mesh is assembled from them. The same board is flashed
// twice: first it is a 4-bit adder, then a 4-bit counter. No Go code
// describes either circuit.
//
// Run: go run .   (FMESH_GRAPH=1 draws the board flashed with the counter)

var (
	//go:embed adder4.v
	adderSource string
	//go:embed counter4.v
	counterSource string
)

func main() {
	fmt.Println("=== FPGA ===")
	fmt.Printf("A board of %d generic logic cells, programmed at runtime from a hardware description.\n\n", BoardCells)

	d, err := Synthesize(counterSource)
	if err != nil {
		fmt.Println("Failed to synthesise the counter:", err)
		os.Exit(1)
	}
	board, err := Flash(d)
	if err != nil {
		fmt.Println("Failed to flash the counter:", err)
		os.Exit(1)
	}
	handled, err := internal.HandleGraphFlag(board)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	adder, err := flash(adderSource)
	if err != nil {
		fmt.Println("Failed to flash the adder:", err)
		os.Exit(1)
	}
	for _, sum := range [][2]int{{5, 7}, {9, 9}, {15, 1}, {6, 3}} {
		got, err := add(adder, sum[0], sum[1])
		if err != nil {
			fmt.Println("Adder failed:", err)
			os.Exit(1)
		}
		fmt.Printf("  %2d + %2d = %2d   (%04b + %04b = %05b)\n", sum[0], sum[1], got, sum[0], sum[1], got)
	}
	fmt.Println()

	// The very same board, flashed again: now it counts.
	counter, err := flash(counterSource)
	if err != nil {
		fmt.Println("Failed to flash the counter:", err)
		os.Exit(1)
	}
	fmt.Print("  clock │")
	for tick := 1; tick <= 20; tick++ {
		en := tick <= 17 // let go of enable for the last three ticks
		if err := Tick(counter, map[string]bool{"en": en}); err != nil {
			fmt.Println("Counter failed:", err)
			os.Exit(1)
		}
		fmt.Printf(" %d", word(counter, "q", 4))
		if tick == 17 {
			fmt.Print(" │ en=0 │")
		}
	}
	fmt.Println()
}

// flash synthesises a design, prints what each cell was programmed with,
// and returns the board as a mesh.
func flash(source string) (*fmesh.FMesh, error) {
	d, err := Synthesize(source)
	if err != nil {
		return nil, err
	}
	fm, err := Flash(d)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Flashing %s: %d of %d cells used\n", d.Name, len(d.Cells), BoardCells)
	for i, c := range d.Cells {
		assign, kind := "=", "LUT"
		if c.Registered {
			assign, kind = "<=", "LUT + flip-flop"
		}
		fmt.Printf("  %s  %-6s %-2s %-30s %s %s\n", cellName(i), c.Signal, assign, c.Source, kind, c.TableString())
	}
	fmt.Println()
	return fm, nil
}

// add drives a and b onto the adder's pins, ticks once, and reads the sum.
func add(fm *fmesh.FMesh, a, b int) (int, error) {
	pins := map[string]bool{}
	for bit := range 4 {
		pins[fmt.Sprint("a", bit)] = a&(1<<bit) != 0
		pins[fmt.Sprint("b", bit)] = b&(1<<bit) != 0
	}
	if err := Tick(fm, pins); err != nil {
		return 0, err
	}
	sum := word(fm, "s", 4)
	if Read(fm, "cout") {
		sum |= 1 << 4
	}
	return sum, nil
}

// word reads output pins prefix0..prefix{n-1} as a binary number.
func word(fm *fmesh.FMesh, prefix string, n int) int {
	v := 0
	for bit := range n {
		if Read(fm, fmt.Sprint(prefix, bit)) {
			v |= 1 << bit
		}
	}
	return v
}

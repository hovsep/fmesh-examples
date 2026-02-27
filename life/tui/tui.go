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

func main() {
	// Connect to Unix socket
	conn, err := net.Dial("unix", "/tmp/sim.sock")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	var values []float64
	const maxPoints = 50 // max points to plot

	for scanner.Scan() {
		line := scanner.Text()

		// Extract brain_activity value
		if strings.HasPrefix(line, "human-Leon::brain_activity") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			val, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				continue
			}

			// Append value, keep maxPoints
			values = append(values, val)
			if len(values) > maxPoints {
				values = values[1:]
			}
			plotValues := append([]float64{0.0, 1.0}, values...)
			// Clear terminal and print graph
			fmt.Print("\033[H\033[2J") // ANSI clear screen
			graph := asciigraph.Plot(plotValues, asciigraph.Height(10), asciigraph.Caption("Brain Activity"))
			fmt.Println(graph)
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := scanner.Err(); err != nil {
		log.Println("scanner error:", err)
	}
}

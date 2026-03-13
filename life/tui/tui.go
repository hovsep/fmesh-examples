package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

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
	var heartRate int
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "human-Leon::heart_rate") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			heartRate, err = strconv.Atoi(parts[1])
		}

		// Extract brain_activity value
		if strings.HasPrefix(line, "human-Leon::heart_cardiac_activation") {
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
			plotValues := append([]float64{}, values...)
			// Clear terminal and print graph
			fmt.Print("\033[H\033[2J") // ANSI clear screen
			graph := asciigraph.Plot(plotValues, asciigraph.Height(10), asciigraph.Caption("Heart rate: "+strconv.Itoa(heartRate)))
			fmt.Println(graph)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("scanner error:", err)
	}
}

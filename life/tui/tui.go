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
	conn, err := net.Dial("unix", "/tmp/habitat_mesh.sock")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	const maxPoints = 50 // max points to plot
	var heartRate int
	HCAValues := make([]float64, 0, maxPoints)
	BAValues := make([]float64, 0, maxPoints)

	for scanner.Scan() {
		line := scanner.Text()
		redraw := false

		if strings.HasPrefix(line, "human-Leon::heart_rate") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if r, e := strconv.Atoi(parts[1]); e == nil {
					heartRate = r
					redraw = true
				}
			}
		}

		if strings.HasPrefix(line, "human-Leon::heart_cardiac_activation") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				val, err := strconv.ParseFloat(parts[1], 64)
				if err == nil {
					HCAValues = append(HCAValues, val)
					if len(HCAValues) > maxPoints {
						HCAValues = HCAValues[1:]
					}
					redraw = true
				}
			}
		}

		if strings.HasPrefix(line, "human-Leon::brain_activity") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				val, err := strconv.ParseFloat(parts[1], 64)
				if err == nil {
					BAValues = append(BAValues, val)
					if len(BAValues) > maxPoints {
						BAValues = BAValues[1:]
					}
					redraw = true
				}
			}
		}

		if redraw && (len(HCAValues) > 0 || len(BAValues) > 0) {
			fmt.Print("\033[H\033[2J") // ANSI clear screen

			if len(HCAValues) > 0 {
				heartPlot := asciigraph.Plot(HCAValues, asciigraph.Width(80), asciigraph.Height(5), asciigraph.SeriesColors(asciigraph.Red), asciigraph.Caption("ECG (Heart) — BPM: "+strconv.Itoa(heartRate)))
				fmt.Println(heartPlot)
			}
			if len(BAValues) > 0 {
				brainPlot := asciigraph.Plot(BAValues, asciigraph.Width(80), asciigraph.Height(5), asciigraph.SeriesColors(asciigraph.Blue), asciigraph.Caption("Brain activity"))
				fmt.Println(brainPlot)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("scanner error:", err)
	}
}

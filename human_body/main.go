package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/hovsep/fmesh-examples/human_body/body"
	"github.com/hovsep/fmesh-examples/human_body/env"
	"github.com/hovsep/fmesh/signal"
)

// @TODO: implement basic abstractions: ResourcePool, Oscillator ,Gauge, Controller, Router

func main() {

	rl, _ := readline.New("> ")
	defer rl.Close()

	// @TODO: implement  start , stop and stream

	// Create the world
	world := env.GetMesh()

	// Create the human being
	humanBeing := body.GetComponent()

	// Put human being into the world
	world.AddComponents(humanBeing)

	// Application control channel
	appCtlChan := make(chan string)

	go func() {
		for {
			select {
			case cmd := <-appCtlChan:
				switch cmd {
				case "time:stop":
					return
				}
			}

			world.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))

			_, err := world.Run()

			if err != nil {
				fmt.Println("Simulation cycle finished with error: ", err)
				os.Exit(1)
			}
		}
	}()

	for {
		cmd, err := rl.Readline()
		if err != nil {
			break
		}

		appCtlChan <- strings.ToLower(cmd)

	}

	fmt.Println("Simulation finished successfully")
}

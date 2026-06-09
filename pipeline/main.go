package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	portIn             = "in"
	portOut            = "out"
	tokenizerDelimiter = " "
)

// This example demonstrates a simple pipeline implementation
// The pipeline consists of multiple processing stages:
// 1. Reads a line of text from standard input.
// 2. Writes the input text to a file for persistence.
// 3. Reads the stored text back into memory.
// 4. Tokenizes the text into individual words based on a specified delimiter.
// 5. Filters out unwanted tokens based on a predefined blocklist.
// 6. Counts the frequency of each unique token.
// 7. Saves the token frequency counts to a new file.
//
// Each stage is represented as a reusable F-Mesh component, allowing for easy modifications
// and extensions. The pipeline executes sequentially, passing processed data between components.
func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Initialize the pipeline by sending the first signal.
	// While we could directly reference the entry-point component using fm.ComponentByName("read-stdin"),
	// leveraging our custom "stage" label provides a more flexible and semantically meaningful approach.
	// This ensures cleaner and more maintainable code, especially if component names will change in the future.
	fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("stage", "1")
	}).InputByName(portIn).PutSignals(signal.New("start"))

	_, err = fm.Run()
	if err != nil {
		fmt.Println("Pipeline finished with error:", err)
		os.Exit(1)
	}

	fmt.Println("Pipeline finished successfully")

	// Extract the filename from the latest stage
	resultFileName := fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("stage", strconv.Itoa(fm.Components().Len()))
	}).OutputByName(portOut).Signals().FirstPayloadOrDefault("")

	if resultFileName != "" {
		fmt.Println("Check results in the file: ", resultFileName)
	}
}

func getMesh() *fmesh.FMesh {
	return buildPipeline(
		"demo-pipeline",
		// Just pass stages in the desired order:
		getStdInReader("read-stdin", "Please input text and press ENTER"),
		getFileWriter("persist-input"),
		getFileReader("read-file"),
		getTokenizer("tokenize", tokenizerDelimiter),
		getFilter("remove-stop-words", map[string]bool{"yes": true, "no": true}),
		getTokenCounter("counter-tokens"),
		getFileWriter("persist-results"),
	)
}

// getFileReader creates a component that reads the whole file
// in: one signal with file name
// out: file contents as single signal
func getFileReader(name string) *component.Component {
	c, err := component.New(name,
		component.WithDescription("read file"),
		component.WithActivationFunc(func(this *component.Component) error {
			// We expect exactly one signal with file name
			fileName := this.InputByName(portIn).Signals().FirstPayloadOrDefault("").(string)
			if fileName == "" {
				return errors.New("no input filename")
			}

			root, err := os.OpenRoot(".")
			if err != nil {
				return err
			}

			file, err := root.Open(fileName)
			if err != nil {
				return err
			}
			defer func() {
				err = file.Close()
				if err != nil {
					this.Logger().Println("failed to close file: ", fileName)
				}

				err = root.Close()
				if err != nil {
					this.Logger().Println("failed to close root")
				}
			}()

			contents, err := io.ReadAll(file)
			if err != nil {
				return err
			}

			this.OutputByName(portOut).PutSignals(signal.New(string(contents)))
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create file reader: %v", err))
	}
	return c
}

// getFileWriter creates a component that writes data into file
// in: multiple signals containing string
// out: 1 signal with file name
// NOTE: the filename is generated dynamically, newline is added to each signal payload when written to file
func getFileWriter(name string) *component.Component {
	c, err := component.New(name,
		component.WithDescription("write to file"),
		component.WithActivationFunc(func(this *component.Component) error {
			root, err := os.OpenRoot(".")
			if err != nil {
				return err
			}
			fileName := fmt.Sprintf("stage-%s_%s_%d", this.Labels().ValueOrDefault("stage", ""), this.Name(), time.Now().UnixNano())
			file, err := root.Create(fileName)
			if err != nil {
				return err
			}
			defer func() {
				err = file.Close()
				if err != nil {
					this.Logger().Println("failed to close file: ", fileName)
				}

				err = root.Close()
				if err != nil {
					this.Logger().Println("failed to close root")
				}
			}()

			// Write all signals into the file (we assume they all are strings)
			writeErr := this.InputByName(portIn).Signals().ForEach(func(s *signal.Signal) error {
				_, err = file.WriteString(s.PayloadOrDefault("").(string) + "\n")
				return err
			})
			if writeErr != nil {
				return writeErr
			}

			err = file.Sync()
			if err != nil {
				return err
			}
			this.OutputByName(portOut).PutSignals(signal.New(fileName))
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create file writer: %v", err))
	}
	return c
}

// getStdInReader creates a component that blocks and reads text from STDIN
// in: any signal(s) to activate
// out: 1 signal with whole scanned text
func getStdInReader(name, prompt string) *component.Component {
	c, err := component.New(name,
		component.WithDescription("read a line from stdin"),
		component.WithActivationFunc(func(this *component.Component) error {
			scanner := bufio.NewScanner(os.Stdin)

			fmt.Println(prompt)
			ok := scanner.Scan()
			if !ok {
				return errors.New("failed to read from STDIN")
			}
			input := scanner.Text()

			if input != "" {
				this.OutputByName(portOut).PutSignals(signal.New(scanner.Text()))
			}

			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create stdin reader: %v", err))
	}
	return c
}

// getTokenizer creates a component that splits a string into tokens
// in: 1 signal with string
// out: multiple signals each containing one token
func getTokenizer(name, delimiter string) *component.Component {
	c, err := component.New(name,
		component.WithDescription("tokenize text"),
		component.WithActivationFunc(func(this *component.Component) error {
			text := this.InputByName(portIn).Signals().FirstPayloadOrDefault("").(string)
			if text == "" {
				this.Logger().Println("got empty text. Aborting activation")
				return nil
			}

			tokens := strings.Split(text, delimiter)

			if len(tokens) == 0 {
				this.Logger().Println("No tokens after tokenization")
			}

			for _, t := range tokens {
				t = strings.TrimSuffix(t, "\n")
				if t == "" {
					continue
				}
				this.OutputByName(portOut).PutSignals(signal.New(t))
			}

			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create tokenizer: %v", err))
	}
	return c
}

// getFilter creates a component that filters out tokens from blockedList
// in: multiple signals with tokens
// out: multiple signals with tokens (filtered)
func getFilter(name string, blockList map[string]bool) *component.Component {
	c, err := component.New(name,
		component.WithDescription("filter-tokens"),
		component.WithActivationFunc(func(this *component.Component) error {
			filtered := signal.NewGroup()

			this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				if !blockList[sig.PayloadOrDefault("").(string)] {
					filtered = filtered.With(sig)
				}
				return nil
			})

			this.OutputByName(portOut).PutSignalGroups(filtered)
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create filter: %v", err))
	}
	return c
}

// getTokenCounter creates a component that counts tokens
// in: multiple signals with tokens
// out: multiple signals with strings "<token>:<frequency>"
func getTokenCounter(name string) *component.Component {
	c, err := component.New(name,
		component.WithDescription("count tokens"),
		component.WithActivationFunc(func(this *component.Component) error {
			counters := make(map[string]int)

			this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				counters[sig.PayloadOrDefault("").(string)]++
				return nil
			})
			for t, count := range counters {
				this.OutputByName(portOut).PutSignals(signal.New(fmt.Sprintf("%s:%d", t, count)))
			}
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create token counter: %v", err))
	}
	return c
}

// buildPipeline accepts multiple components and builds a pipeline of them
// each component will be setup with standard interface (input/output ports)
//
//	Also, each component will be assigned a "stage" label which allows referring
//
// to components by stage index instead of the name
func buildPipeline(name string, components ...*component.Component) *fmesh.FMesh {
	stageIndex := 1

	for _, c := range components {
		// We can add custom labels
		c.AddLabel("stage", strconv.Itoa(stageIndex))

		if err := c.AddInputs(portIn); err != nil {
			panic(fmt.Sprintf("failed to add input port to %s: %v", c.Name(), err))
		}
		if err := c.AddOutputs(portOut); err != nil {
			panic(fmt.Sprintf("failed to add output port to %s: %v", c.Name(), err))
		}

		stageIndex++
	}

	fm, err := fmesh.New(name)
	if err != nil {
		panic(fmt.Sprintf("failed to create mesh: %v", err))
	}

	for _, c := range components {
		if err := fm.AddComponents(c); err != nil {
			panic(fmt.Sprintf("failed to add component %s: %v", c.Name(), err))
		}
	}

	stageIndex = 1
	for _, c := range components {
		// Connect stages with pipes
		if stageIndex > 1 {
			prev := components[stageIndex-2]
			if err := prev.OutputByName(portOut).PipeTo(c.InputByName(portIn)); err != nil {
				panic(fmt.Sprintf("failed to pipe stage %d to %d: %v", stageIndex-1, stageIndex, err))
			}
		}
		stageIndex++
	}

	return fm
}

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
	err := internal.HandleGraphFlag(fm)
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
	return component.New(name).
		WithDescription("read file").
		WithActivationFunc(func(this *component.Component) error {
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
		})
}

// getFileWriter creates a component that writes data into file
// in: multiple signals containing string
// out: 1 signal with file name
// NOTE: the filename is generated dynamically, newline is added to each signal payload when written to file
func getFileWriter(name string) *component.Component {
	return component.New(name).
		WithDescription("write to file").
		WithActivationFunc(func(this *component.Component) error {
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
			this.InputByName(portIn).Signals().ForEach(func(s *signal.Signal) error {
				_, err = file.WriteString(s.PayloadOrDefault("").(string) + "\n")
				return err
			})

			if this.HasChainableErr() {
				return this.ChainableErr()
			}

			err = file.Sync()
			if err != nil {
				return err
			}
			this.OutputByName(portOut).PutSignals(signal.New(fileName))
			return nil
		})
}

// getStdInReader creates a component that blocks and reads text from STDIN
// in: any signal(s) to activate
// out: 1 signal with whole scanned text
func getStdInReader(name, prompt string) *component.Component {
	return component.New(name).
		WithDescription("read a line from stdin").
		WithActivationFunc(func(this *component.Component) error {
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
		})
}

// getTokenizer creates a component that splits a string into tokens
// in: 1 signal with string
// out: multiple signals each containing one token
func getTokenizer(name, delimiter string) *component.Component {
	return component.New(name).
		WithDescription("tokenize text").
		WithActivationFunc(func(this *component.Component) error {
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
		})
}

// getFilter creates a component that filters out tokens from blockedList
// in: multiple signals with tokens
// out: multiple signals with tokens (filtered)
func getFilter(name string, blockList map[string]bool) *component.Component {
	return component.New(name).
		WithDescription("filter-tokens").
		WithActivationFunc(func(this *component.Component) error {
			filtered := signal.NewGroup()

			this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				if !blockList[sig.PayloadOrDefault("").(string)] {
					filtered = filtered.Add(sig)
				}
				return nil
			})

			this.OutputByName(portOut).PutSignalGroups(filtered)
			return nil
		})
}

// getTokenCounter creates a component that counts tokens
// in: multiple signals with tokens
// out: multiple signals with strings "<token>:<frequency>"
func getTokenCounter(name string) *component.Component {
	return component.New(name).
		WithDescription("count tokens").
		WithActivationFunc(func(this *component.Component) error {
			counters := make(map[string]int)

			this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				counters[sig.PayloadOrDefault("").(string)]++
				return nil
			})
			for t, count := range counters {
				this.OutputByName(portOut).PutSignals(signal.New(fmt.Sprintf("%s:%d", t, count)))
			}
			return nil
		})
}

// buildPipeline accepts multiple components and builds a pipeline of them
// each component will be setup with standard interface (input/output ports)
//
//	Also, each component will be assigned a "stage" label which allows referring
//
// to components by stage index instead of the name
func buildPipeline(name string, components ...*component.Component) *fmesh.FMesh {
	stageIndex := 1
	fm := fmesh.New(name)

	for _, c := range components {
		// We can add custom labels
		c.AddLabel("stage", strconv.Itoa(stageIndex))

		fm = fm.AddComponents(withPipelineInterface(c))

		// Connect stages with pipes
		if stageIndex > 1 {
			// Use stage-index semantics to connect components
			fm.Components().FindAny(func(c *component.Component) bool {
				return c.Labels().ValueIs("stage", strconv.Itoa(stageIndex-1))
			}).OutputByName(portOut). // Connect from
							PipeTo(c.InputByName(portIn)) // Connect to
		}
		stageIndex++
	}

	return fm
}

// withPipelineInterface defines the common interface shared by all components
// as we are building a pipeline each component will have one input and one output
func withPipelineInterface(c *component.Component) *component.Component {
	return c.AddInputs(portIn).AddOutputs(portOut)
}

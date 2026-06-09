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

func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("stage", "1")
	}).InputByName(portIn).PutSignals(signal.New("start"))

	_, err = fm.Run()
	if err != nil {
		fmt.Println("Pipeline finished with error:", err)
		os.Exit(1)
	}

	fmt.Println("Pipeline finished successfully")

	resultFileName := fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("stage", strconv.Itoa(fm.Components().Len()))
	}).OutputByName(portOut).Signals().FirstPayloadOrDefault("")

	if resultFileName != "" {
		fmt.Println("Check results in the file:", resultFileName)
	}
}

func getMesh() (*fmesh.FMesh, error) {
	stdinReader, err := getStdInReader("read-stdin", "Please input text and press ENTER")
	if err != nil {
		return nil, fmt.Errorf("getStdInReader: %w", err)
	}
	persistInput, err := getFileWriter("persist-input")
	if err != nil {
		return nil, fmt.Errorf("getFileWriter: %w", err)
	}
	fileReader, err := getFileReader("read-file")
	if err != nil {
		return nil, fmt.Errorf("getFileReader: %w", err)
	}
	tokenizer, err := getTokenizer("tokenize", tokenizerDelimiter)
	if err != nil {
		return nil, fmt.Errorf("getTokenizer: %w", err)
	}
	filter, err := getFilter("remove-stop-words", map[string]bool{"yes": true, "no": true})
	if err != nil {
		return nil, fmt.Errorf("getFilter: %w", err)
	}
	tokenCounter, err := getTokenCounter("counter-tokens")
	if err != nil {
		return nil, fmt.Errorf("getTokenCounter: %w", err)
	}
	persistResults, err := getFileWriter("persist-results")
	if err != nil {
		return nil, fmt.Errorf("getFileWriter: %w", err)
	}
	return buildPipeline(
		"demo-pipeline",
		stdinReader,
		persistInput,
		fileReader,
		tokenizer,
		filter,
		tokenCounter,
		persistResults,
	)
}

func getFileReader(name string) (*component.Component, error) {
	c, err := component.New(name,
		component.WithDescription("read file"),
		component.WithActivationFunc(func(this *component.Component) error {
			fileName := this.InputByName(portIn).Signals().FirstPayloadOrDefault("").(string)
			if fileName == "" {
				return errors.New("no input filename")
			}

			root, err := os.OpenRoot(".")
			if err != nil {
				return err
			}
			defer root.Close()

			file, err := root.Open(fileName)
			if err != nil {
				return err
			}
			defer file.Close()

			contents, err := io.ReadAll(file)
			if err != nil {
				return err
			}

			this.OutputByName(portOut).PutSignals(signal.New(string(contents)))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("getFileReader: %w", err)
	}
	return c, nil
}

func getFileWriter(name string) (*component.Component, error) {
	c, err := component.New(name,
		component.WithDescription("write to file"),
		component.WithActivationFunc(func(this *component.Component) error {
			root, err := os.OpenRoot(".")
			if err != nil {
				return err
			}
			defer root.Close()

			fileName := fmt.Sprintf("stage-%s_%s_%d", this.Labels().ValueOrDefault("stage", ""), this.Name(), time.Now().UnixNano())
			file, err := root.Create(fileName)
			if err != nil {
				return err
			}
			defer file.Close()

			writeErr := this.InputByName(portIn).Signals().ForEach(func(s *signal.Signal) error {
				_, err = file.WriteString(s.PayloadOrDefault("").(string) + "\n")
				return err
			})
			if writeErr != nil {
				return writeErr
			}

			if err := file.Sync(); err != nil {
				return err
			}

			this.OutputByName(portOut).PutSignals(signal.New(fileName))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("getFileWriter: %w", err)
	}
	return c, nil
}

func getStdInReader(name, prompt string) (*component.Component, error) {
	c, err := component.New(name,
		component.WithDescription("read a line from stdin"),
		component.WithActivationFunc(func(this *component.Component) error {
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Println(prompt)
			if !scanner.Scan() {
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
		return nil, fmt.Errorf("getStdInReader: %w", err)
	}
	return c, nil
}

func getTokenizer(name, delimiter string) (*component.Component, error) {
	c, err := component.New(name,
		component.WithDescription("tokenize text"),
		component.WithActivationFunc(func(this *component.Component) error {
			text := this.InputByName(portIn).Signals().FirstPayloadOrDefault("").(string)
			if text == "" {
				this.Logger().Println("got empty text. Aborting activation")
				return nil
			}

			tokens := strings.Split(text, delimiter)
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
		return nil, fmt.Errorf("getTokenizer: %w", err)
	}
	return c, nil
}

func getFilter(name string, blockList map[string]bool) (*component.Component, error) {
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
		return nil, fmt.Errorf("getFilter: %w", err)
	}
	return c, nil
}

func getTokenCounter(name string) (*component.Component, error) {
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
		return nil, fmt.Errorf("getTokenCounter: %w", err)
	}
	return c, nil
}

func buildPipeline(name string, components ...*component.Component) (*fmesh.FMesh, error) {
	stageIndex := 1
	for _, c := range components {
		c.AddLabel("stage", strconv.Itoa(stageIndex))
		if err := c.AddInputs(portIn); err != nil {
			return nil, fmt.Errorf("add input to %s: %w", c.Name(), err)
		}
		if err := c.AddOutputs(portOut); err != nil {
			return nil, fmt.Errorf("add output to %s: %w", c.Name(), err)
		}
		stageIndex++
	}

	fm, err := fmesh.New(name)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}

	for _, c := range components {
		if err := fm.AddComponents(c); err != nil {
			return nil, fmt.Errorf("add %s: %w", c.Name(), err)
		}
	}

	stageIndex = 1
	for _, c := range components {
		if stageIndex > 1 {
			prev := components[stageIndex-2]
			if err := prev.OutputByName(portOut).PipeTo(c.InputByName(portIn)); err != nil {
				return nil, fmt.Errorf("pipe stage %d→%d: %w", stageIndex-1, stageIndex, err)
			}
		}
		stageIndex++
	}

	return fm, nil
}

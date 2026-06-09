package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// This example processes 1 url every 3 seconds
// NOTE: urls are not crawled concurrently, because fm has only 1 worker (crawler component)
func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	urls := []string{
		"http://fffff.com",
		"https://google.com",
		"http://habr.com",
		"http://localhost:80",
		"https://postman-echo.com/delay/1",
		"https://postman-echo.com/delay/3",
		"https://postman-echo.com/delay/5",
		"https://postman-echo.com/delay/10",
	}

	ticker := time.NewTicker(3 * time.Second)
	resultsChan := make(chan []any)
	doneChan := make(chan struct{}) // Signals when all urls are processed

	// Producer goroutine
	go func() {
		for {
			<-ticker.C
			if len(urls) == 0 {
				close(resultsChan)
				return
			}
			// Pop url
			url := urls[0]
			urls = urls[1:]

			fmt.Println("produce:", url)

			fm.Components().ByName("web crawler").InputByName("url").PutSignals(signal.New(url))
			_, err := fm.Run()
			if err != nil {
				fmt.Println("fmesh returned error ", err)
			}

			if fm.Components().ByName("web crawler").OutputByName("headers").HasSignals() {
				results, err := fm.Components().ByName("web crawler").OutputByName("headers").Signals().AllPayloads()
				if err != nil {
					fmt.Println("Failed to get results ", err)
				}
				fm.Components().ByName("web crawler").OutputByName("headers").Clear() // @TODO maybe we can add fm.Reset() for cases when FMesh is reused (instead of cleaning ports explicitly)
				resultsChan <- results
			}
		}
	}()

	// Consumer goroutine
	go func() {
		for {
			r, ok := <-resultsChan
			if !ok {
				fmt.Println("results chan is closed. shutting down the reader")
				doneChan <- struct{}{}
				return
			}
			fmt.Printf("consume: %v \n", r)

		}
	}()

	<-doneChan
}

func getMesh() *fmesh.FMesh {
	// Setup dependencies
	client := &http.Client{}

	// Define components
	crawler, err := component.New("web crawler",
		component.WithDescription("gets http headers from given url"),
		component.WithInputs("url"),
		component.WithOutputs("errors", "headers"),
		component.WithActivationFunc(func(this *component.Component) error {
			if !this.InputByName("url").HasSignals() {
				return component.ErrWaitingForInputs
			}

			allUrls, err := this.InputByName("url").Signals().AllPayloads()
			if err != nil {
				return err
			}

			for _, urlVal := range allUrls {

				url := urlVal.(string)
				// All urls will be crawled sequentially
				// in order to call them concurrently we need run each request in separate goroutine and handle synchronization (e.g. waitgroup)
				response, err := client.Get(url)
				if err != nil {
					this.OutputByName("errors").PutSignals(signal.New(fmt.Errorf("got error: %w from url: %s", err, url)))
					continue
				}

				if len(response.Header) == 0 {
					this.OutputByName("errors").PutSignals(signal.New(fmt.Errorf("no headers for url %s", url)))
					continue
				}

				this.OutputByName("headers").PutSignals(signal.New(map[string]http.Header{
					url: response.Header,
				}))
			}

			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create crawler component: %v", err))
	}

	logger, err := component.New("error logger",
		component.WithDescription("logs http errors"),
		component.WithInputs("error"),
		component.WithActivationFunc(func(this *component.Component) error {
			if !this.InputByName("error").HasSignals() {
				return component.ErrWaitingForInputs
			}

			allErrors, err := this.InputByName("error").Signals().AllPayloads()
			if err != nil {
				return err
			}

			for _, errVal := range allErrors {
				e := errVal.(error)
				if e != nil {
					fmt.Println("Error logger says:", e)
				}
			}

			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger component: %v", err))
	}

	// Define pipes
	if err := crawler.OutputByName("errors").PipeTo(logger.InputByName("error")); err != nil {
		panic(fmt.Sprintf("failed to pipe crawler to logger: %v", err))
	}

	fm, err := fmesh.New("web scraper",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create mesh: %v", err))
	}
	if err := fm.AddComponents(crawler, logger); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

	return fm
}

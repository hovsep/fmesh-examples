package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// ServiceFunc is the type for our service calls
type ServiceFunc func() (string, error)

// Plugin wraps a ServiceFunc
type Plugin func(next ServiceFunc) ServiceFunc

func main() {
	// Create service (random failure)
	service := getService()

	// Create client with retry plugin
	client := getClient(RetryPlugin(3, 500*time.Millisecond))

	// Wrap the service with client plugins
	serviceWithPlugins := client(service)

	// Call the service
	resp, err := serviceWithPlugins()
	if err != nil {
		fmt.Println("Final error:", err)
		return
	}
	fmt.Println("Response:", resp)
}

// getClient returns a function that applies plugins
func getClient(plugins ...Plugin) func(ServiceFunc) ServiceFunc {
	return func(service ServiceFunc) ServiceFunc {
		wrapped := service
		// Apply plugins in reverse order
		for i := len(plugins) - 1; i >= 0; i-- {
			wrapped = plugins[i](wrapped)
		}
		return wrapped
	}
}

// RetryPlugin returns a plugin that retries the call
func RetryPlugin(maxAttempts int, delay time.Duration) Plugin {
	return func(next ServiceFunc) ServiceFunc {
		return func() (string, error) {
			var err error
			var resp string
			for i := 0; i < maxAttempts; i++ {
				resp, err = next()
				if err == nil {
					return resp, nil
				}
				fmt.Println("Retrying due to error:", err)
				time.Sleep(delay)
			}
			return resp, err
		}
	}
}

// getService returns a dummy service function that fails randomly
func getService() ServiceFunc {
	return func() (string, error) {
		if rand.Float32() < 0.7 { // 70% chance to fail
			return "", errors.New("random failure")
		}
		return "success", nil
	}
}

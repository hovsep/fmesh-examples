package console

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

// waitForLine reads one captured line, or fails if none arrives.
func waitForLine(t *testing.T, c *Capture) string {
	t.Helper()
	select {
	case line := <-c.Lines:
		return line
	case <-time.After(2 * time.Second):
		t.Fatal("nothing was captured")
		return ""
	}
}

func TestCaptureStdout_CatchesPlainPrints(t *testing.T) {
	c, err := CaptureStdout(16)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Restore()

	fmt.Println("hello from the simulation")

	if got := waitForLine(t, c); got != "hello from the simulation" {
		t.Fatalf("captured %q", got)
	}
}

func TestCaptureStdout_CatchesLoggersBoundBeforeCapture(t *testing.T) {
	// fmesh gives every component a log.New(os.Stdout, ...) when the component is
	// constructed, which happens long before the console starts. Such a logger
	// holds the original stdout, so swapping the os.Stdout variable would not
	// redirect it: its output would bypass the console and tear up the frame it
	// had drawn. This is the case that matters.
	logger := log.New(os.Stdout, "organ:heart: ", 0)

	c, err := CaptureStdout(16)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Restore()

	logger.Println("Neural drive too low")

	if got := waitForLine(t, c); got != "organ:heart: Neural drive too low" {
		t.Fatalf("captured %q", got)
	}
}

func TestCaptureStdout_RestoresStdout(t *testing.T) {
	original := os.Stdout

	c, err := CaptureStdout(16)
	if err != nil {
		t.Fatal(err)
	}
	c.Restore()

	if os.Stdout != original {
		t.Fatal("Restore did not put the original os.Stdout back")
	}

	// Restore must be safe to call twice: Run defers it and also calls it
	// explicitly before printing any shutdown message.
	c.Restore()
}

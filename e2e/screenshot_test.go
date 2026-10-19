package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// The README pictures. E2E_SCREENSHOTS=1 go test -run Screenshot . retakes
// them from the running demos; without the variable these tests skip.

func screenshotDir(t *testing.T, demo string) string {
	t.Helper()
	if os.Getenv("E2E_SCREENSHOTS") == "" {
		t.Skip("set E2E_SCREENSHOTS=1 to retake the README pictures")
	}
	dir, err := filepath.Abs(filepath.Join("..", "simulation", demo))
	require.NoError(t, err)
	return dir
}

func TestElevatorScreenshot(t *testing.T) {
	dir := screenshotDir(t, "elevator")
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL)
	clickPart(t, page, "hall-up-3", "press")
	clickPart(t, page, "car-B-button-2", "press")
	waitFor(t, page, "car A to open on 3", "cur['door-operator-A'].open && cur['position-sensor-A'].floor === 3", 20*time.Second)
	_, err := page.Locator("canvas").Screenshot(playwright.LocatorScreenshotOptions{Path: playwright.String(filepath.Join(dir, "screenshot.png"))})
	require.NoError(t, err)
}

func TestEngineScreenshot(t *testing.T) {
	dir := screenshotDir(t, "combustion_engine")
	d := startDemo(t, "engine", 0, "-coolant", "80")
	page := openEngine(t, d.URL)
	startEngine(t, page)
	require.NoError(t, page.Keyboard().Down("KeyW"))
	waitFor(t, page, "the revs to rise", "snap.parts.crankshaft.crank.rpm > 3500", 10*time.Second)
	require.NoError(t, page.Keyboard().Up("KeyW"))
	time.Sleep(300 * time.Millisecond)
	_, err := page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(filepath.Join(dir, "screenshot.png")), FullPage: playwright.Bool(true)})
	require.NoError(t, err)
	stopEngine(t, page)
}

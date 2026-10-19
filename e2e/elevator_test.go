package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The elevator page is a canvas: there is no DOM to read, so the tests
// read the page's own variables (the latest frame `cur`, the hit
// rectangles `hits`) and click where the page says a button is.

const elevatorComponents = 119

// openElevator opens the page and waits for the first frame. Every frame
// after that is checked for the one rule an elevator must never break.
func openElevator(t *testing.T, url string, opts ...playwright.BrowserNewContextOptions) playwright.Page {
	t.Helper()
	if len(opts) == 0 {
		opts = []playwright.BrowserNewContextOptions{{Viewport: &playwright.Size{Width: 1300, Height: 900}}}
	}
	page := openPage(t, url, opts...)
	waitFor(t, page, "the first frame", "cur !== null && online", 5*time.Second)
	eval(t, page, `() => {
		window.__violations = [];
		es.addEventListener('message', e => {
			const f = JSON.parse(e.data).parts;
			for (const c of CABS) {
				if (!f['car-' + c] || f['car-' + c].speed === 0) continue;
				if (!f['door-operator-' + c].closed) __violations.push('car ' + c + ' moves with its door open at ' + f.clock.seconds + ' s');
				for (let i = 1; i <= FLOORS; i++) if (!f['door-lock-' + c + '-' + i].closed) __violations.push('car ' + c + ' moves with landing door ' + i + ' open');
			}
		});
	}`)
	t.Cleanup(func() {
		if v, err := page.Evaluate("window.__violations"); err == nil {
			assert.Empty(t, v, "safety violations seen by the page")
		}
	})
	return page
}

// hitOf finds the hit rectangle the page registered for a part and port.
func hitOf(t *testing.T, page playwright.Page, part, port string) rect {
	t.Helper()
	v := eval(t, page, `([part, port]) => { const h = hits.find(h => h.part === part && h.port === port); return h ? [h.x, h.y, h.w, h.h] : null; }`, []any{part, port})
	require.NotNil(t, v, "no button for %s/%s on the page", part, port)
	xs := v.([]any)
	return rect{num(xs[0]), num(xs[1]), num(xs[2]), num(xs[3])}
}

// clickPart clicks the middle of a part's button, as the mouse would.
func clickPart(t *testing.T, page playwright.Page, part, port string) {
	t.Helper()
	r := hitOf(t, page, part, port)
	x, y := canvasPoint(t, page, r.X+r.W/2, r.Y+r.H/2)
	require.NoError(t, page.Mouse().Click(x, y))
}

// tapPart does the same with a finger.
func tapPart(t *testing.T, page playwright.Page, part, port string) {
	t.Helper()
	r := hitOf(t, page, part, port)
	x, y := canvasPoint(t, page, r.X+r.W/2, r.Y+r.H/2)
	require.NoError(t, page.Touchscreen().Tap(int(x), int(y)))
}

func partState(t *testing.T, page playwright.Page, part string) map[string]any {
	t.Helper()
	v, _ := eval(t, page, "name => cur[name] || null", part).(map[string]any)
	return v
}

func TestElevatorPageGoesLive(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL)
	assert.Equal(t, float64(elevatorComponents), num(eval(t, page, "meta.components")))
	assert.InDelta(t, 10, num(eval(t, page, "meta.cycles")), 3)
	assert.Equal(t, "parked", partState(t, page, "controller-A")["mode"])
	assert.Equal(t, float64(1), num(partState(t, page, "position-sensor-A")["floor"]))
	assert.Equal(t, float64(6), num(partState(t, page, "position-sensor-B")["floor"]))
	before := num(eval(t, page, "cur.clock.tick"))
	time.Sleep(300 * time.Millisecond)
	assert.Greater(t, num(eval(t, page, "cur.clock.tick")), before, "frames keep arriving")
}

// TestElevatorEveryButtonOnThePageExists: every rectangle the page offers
// to click is a part the mesh accepts a press on, and every button a
// visitor needs is on the page.
func TestElevatorEveryButtonOnThePageExists(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL)
	v := eval(t, page, `() => [...new Set(hits.map(h => h.part + '/' + h.port))].sort()`)
	seen := map[string]bool{}
	for _, h := range v.([]any) {
		part, port, _ := strings.Cut(h.(string), "/")
		seen[h.(string)] = true
		assert.Equal(t, 204, d.post("/press", fmt.Sprintf(`{"part": %q, "port": %q}`, part, port)), h)
	}
	for f := 1; f < 6; f++ {
		assert.True(t, seen[fmt.Sprintf("hall-up-%d/press", f)])
		assert.True(t, seen[fmt.Sprintf("hall-down-%d/press", f+1)])
	}
	for _, c := range []string{"A", "B"} {
		for f := 1; f <= 6; f++ {
			assert.True(t, seen[fmt.Sprintf("car-%s-button-%d/press", c, f)])
		}
		assert.True(t, seen["car-"+c+"-open/press"])
		assert.True(t, seen["car-"+c+"-close/press"])
		assert.True(t, seen["light-curtain-"+c+"/block"])
	}
	assert.False(t, seen["hall-up-6/press"], "no up button on the top floor")
	assert.False(t, seen["hall-down-1/press"], "no down button in the lobby")
}

func TestElevatorHallCallBringsTheNearestCar(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL)
	clickPart(t, page, "hall-up-3", "press")
	waitFor(t, page, "the hall LED", "cur['hall-up-led-3'].on", 2*time.Second)
	waitFor(t, page, "car A to open its doors on 3", "cur['door-operator-A'].open && cur['position-sensor-A'].floor === 3", 20*time.Second)
	assert.InDelta(t, 7.0, num(partState(t, page, "car-A")["pos"]), 0.015, "level with the landing")
	assert.False(t, truthy(partState(t, page, "hall-up-led-3")["on"]), "the LED went out")
	assert.Equal(t, float64(1), num(partState(t, page, "landing-door-A-3")["opening"]))
	assert.Equal(t, float64(0), num(partState(t, page, "landing-door-A-2")["opening"]))
	assert.True(t, truthy(partState(t, page, "lantern-A-3")["gong"]))
	assert.Equal(t, 17.5, num(partState(t, page, "car-B")["pos"]), "car B stayed on the top floor")
}

func TestElevatorRideAndWalkThroughTheDoorway(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL)
	clickPart(t, page, "car-B-button-4", "press")
	waitFor(t, page, "the button LED", "cur['car-B-led-4'].on", 2*time.Second)
	waitFor(t, page, "car B to open on 4", "cur['door-operator-B'].open && cur['position-sensor-B'].floor === 4", 20*time.Second)
	assert.False(t, truthy(partState(t, page, "car-B-led-4")["on"]))

	waitFor(t, page, "the doors to start closing", "cur['controller-B'].mode === 'closing'", 10*time.Second)
	clickPart(t, page, "light-curtain-B", "block") // step into the doorway
	waitFor(t, page, "the curtain to see someone", "cur['light-curtain-B'].blocked", 2*time.Second)
	waitFor(t, page, "the doors to open again", "cur['controller-B'].mode === 'opening' || cur['controller-B'].mode === 'dwell'", 3*time.Second)
	waitFor(t, page, "the doors to close at last", "cur['controller-B'].mode === 'parked'", 15*time.Second)
}

func TestElevatorRapidClicksLoseNothing(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL)
	for f := 2; f <= 6; f++ {
		clickPart(t, page, fmt.Sprintf("car-A-button-%d", f), "press")
	}
	waitFor(t, page, "all five LEDs", "[2,3,4,5,6].every(f => cur['car-A-led-' + f].on)", 2*time.Second)
	assert.Equal(t, []float64{2, 3, 4, 5, 6}, nums(partState(t, page, "controller-A")["stops"]), "every click became a stop, once")
	waitFor(t, page, "the first stop served", "cur['door-operator-A'].open && cur['position-sensor-A'].floor === 2", 15*time.Second)
	assert.False(t, truthy(partState(t, page, "car-A-led-2")["on"]))
	assert.True(t, truthy(partState(t, page, "car-A-led-3")["on"]), "the rest still wait")
}

func TestElevatorOnAPhone(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	page := openElevator(t, d.URL, playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{Width: 390, Height: 844}, DeviceScaleFactor: playwright.Float(3),
		IsMobile: playwright.Bool(true), HasTouch: playwright.Bool(true),
	})
	tapPart(t, page, "hall-up-2", "press")
	waitFor(t, page, "the hall LED after a tap", "cur['hall-up-led-2'].on", 2*time.Second)
}

func TestElevatorTwoBrowsersSeeOneBuilding(t *testing.T) {
	d := startDemo(t, "elevator", 0)
	one := openElevator(t, d.URL)
	two := openElevator(t, d.URL)
	clickPart(t, one, "hall-down-5", "press")
	waitFor(t, two, "the other browser to see the LED", "cur['hall-down-led-5'].on", 2*time.Second)
}

func TestElevatorComesBackWhenTheServerDoes(t *testing.T) {
	port := freePort(t)
	d := startDemo(t, "elevator", port)
	page := openElevator(t, d.URL)
	d.stop()
	waitFor(t, page, "the page to notice", "!online", 5*time.Second)
	startDemo(t, "elevator", port)
	waitFor(t, page, "the page to reconnect", "online && cur.clock.tick < 200", 15*time.Second)
}

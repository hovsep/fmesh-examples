package e2e

import (
	"math"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The engine page has real buttons and a status line, so most checks read
// the DOM; the canvas is checked through the page's own drawing state
// (`snap`, `spin`, `turn`).

const engineComponents = 39

func openEngine(t *testing.T, url string) playwright.Page {
	t.Helper()
	page := openPage(t, url)
	waitFor(t, page, "the first snapshot", "snap !== null", 5*time.Second)
	return page
}

func status(t *testing.T, page playwright.Page) string {
	t.Helper()
	s, err := page.Locator("#status").TextContent()
	require.NoError(t, err)
	return s
}

var rpmRe = regexp.MustCompile(`(\d+) rpm`)

func statusRPM(t *testing.T, page playwright.Page) float64 {
	t.Helper()
	m := rpmRe.FindStringSubmatch(status(t, page))
	if m == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

// startEngine clicks Start and waits until it runs on its own.
func startEngine(t *testing.T, page playwright.Page) {
	t.Helper()
	require.NoError(t, page.Locator("#start").Click())
	waitFor(t, page, "the engine to run", "snap.running && snap.parts.ecu.mode === 'running'", 15*time.Second)
}

func stopEngine(t *testing.T, page playwright.Page) {
	t.Helper()
	require.NoError(t, page.Locator("#start").Click())
	waitFor(t, page, "the engine to stop", "!snap.running", 15*time.Second)
}

func TestEnginePageBoots(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)
	assert.Equal(t, "stopped: press Start", status(t, page))
	sub, _ := page.Locator("#sub").TextContent()
	assert.Contains(t, sub, "39 parts")
	assert.Equal(t, float64(engineComponents), num(eval(t, page, "Object.keys(snap.parts).length")))
	for _, id := range []string{"start", "gas", "refuel"} {
		n, _ := page.Locator("#" + id).Count()
		assert.Equal(t, 1, n, id)
	}
	for _, gone := range []string{"#ac", "#lights", "#sound", "#hand", "#floor", "[data-scale]"} {
		n, _ := page.Locator(gone).Count()
		assert.Zero(t, n, gone)
	}
	label, _ := page.Locator("#start").TextContent()
	assert.Contains(t, label, "Start")
}

func TestEngineStartsIdlesAndStops(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)
	require.NoError(t, page.Locator("#start").Click())
	waitFor(t, page, "cranking or running", "/^(cranking|running)/.test(document.getElementById('status').textContent)", 5*time.Second)
	waitFor(t, page, "running", "snap.running && snap.parts.ecu.mode === 'running'", 15*time.Second)
	label, _ := page.Locator("#start").TextContent()
	assert.Contains(t, label, "Stop")
	class, _ := page.Locator("#start").GetAttribute("class")
	assert.Contains(t, class, "running")

	// A cold engine idles fast, about 1000 rpm, once the first seconds are over.
	time.Sleep(3 * time.Second)
	rpm := statusRPM(t, page)
	assert.InDelta(t, 1000, rpm, 250, "status: %s", status(t, page))
	assert.Regexp(t, `^running · \d+ rpm$`, status(t, page))

	stopEngine(t, page)
	assert.Equal(t, "stopped: the key was turned off", status(t, page))
	label, _ = page.Locator("#start").TextContent()
	assert.Contains(t, label, "Start")
	assert.Zero(t, num(eval(t, page, "snap.parts.crankshaft.crank.rpm")))
}

func TestEnginePedalByKeyAndByButton(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)
	startEngine(t, page)

	require.NoError(t, page.Keyboard().Down("KeyW"))
	waitFor(t, page, "the pedal to go down", "snap.driver.pedal > 0.9", 3*time.Second)
	waitFor(t, page, "the revs to rise", "snap.parts.crankshaft.crank.rpm > 3000", 5*time.Second)
	waitFor(t, page, "the rev limiter", "snap.parts.ecu.mode === 'rev limit'", 10*time.Second)
	assert.Less(t, num(eval(t, page, "snap.parts.crankshaft.crank.rpm")), 6800.0)
	require.NoError(t, page.Keyboard().Up("KeyW"))
	waitFor(t, page, "the pedal to spring back", "snap.driver.pedal === 0", 3*time.Second)
	waitFor(t, page, "the revs to fall", "snap.parts.crankshaft.crank.rpm < 1500", 10*time.Second)

	gas := page.Locator("#gas")
	require.NoError(t, gas.Hover())
	require.NoError(t, page.Mouse().Down())
	waitFor(t, page, "the button to press the pedal", "snap.driver.pedal > 0.5", 3*time.Second)
	class, _ := gas.GetAttribute("class")
	assert.Contains(t, class, "held")
	require.NoError(t, page.Mouse().Move(10, 10)) // slide off the button while holding
	require.NoError(t, page.Mouse().Up())
	waitFor(t, page, "the button to let go", "snap.driver.pedal === 0", 3*time.Second)
	class, _ = gas.GetAttribute("class")
	assert.NotContains(t, class, "held")
	stopEngine(t, page)
}

func TestEngineLosingFocusLetsGoOfThePedal(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)
	startEngine(t, page)
	require.NoError(t, page.Keyboard().Down("KeyW"))
	waitFor(t, page, "the pedal to go down", "snap.driver.pedal > 0.5", 3*time.Second)
	eval(t, page, "() => window.dispatchEvent(new Event('blur'))")
	waitFor(t, page, "the pedal to spring back without a key-up", "snap.driver.pedal === 0", 3*time.Second)
	require.NoError(t, page.Keyboard().Up("KeyW"))
	stopEngine(t, page)
}

func TestEngineHeldKeyTogglesTheKeyOnce(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)
	// A held key repeats its keydown; only the first one may turn the key.
	eval(t, page, `() => { for (let i = 0; i < 4; i++) window.dispatchEvent(new KeyboardEvent('keydown', { code: 'KeyI', repeat: i > 0 })); }`)
	waitFor(t, page, "the engine to start", "snap.driver.ignition", 3*time.Second)
	time.Sleep(500 * time.Millisecond)
	assert.True(t, truthy(eval(t, page, "snap.driver.ignition")), "a repeat turned it off again")
	stopEngine(t, page)
}

func TestEngineRunsDryAndRefuels(t *testing.T) {
	d := startDemo(t, "engine", 0, "-fuel", "0.02")
	page := openEngine(t, d.URL)
	startEngine(t, page)
	require.NoError(t, page.Keyboard().Down("KeyW"))
	waitFor(t, page, "the tank to run dry", "!snap.running", 30*time.Second)
	require.NoError(t, page.Keyboard().Up("KeyW"))
	assert.Equal(t, "stopped: the tank is empty", status(t, page))

	require.NoError(t, page.Keyboard().Press("KeyF"))
	waitFor(t, page, "the refuel to be noted", "snap.driver.refuel", 2*time.Second)
	class, _ := page.Locator("#refuel").GetAttribute("class")
	assert.Contains(t, class, "on")
	startEngine(t, page)
	waitFor(t, page, "the tank to be full", "snap.parts['fuel-tank'].liters > 40 && !snap.driver.refuel", 5*time.Second)
	stopEngine(t, page)
}

func TestEngineFlatBatteryOnlyClicks(t *testing.T) {
	d := startDemo(t, "engine", 0, "-battery", "5")
	page := openEngine(t, d.URL)
	for range 2 {
		require.NoError(t, page.Locator("#start").Click())
		waitFor(t, page, "the starter to give up", "!snap.running && snap.reason !== 'press Start'", 5*time.Second)
		assert.Equal(t, "stopped: the battery is too weak to turn the starter", status(t, page))
	}
}

func TestEngineDoubleClickOnStartStartsOnce(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)
	require.NoError(t, page.Locator("#start").Dblclick())
	waitFor(t, page, "the engine to run", "snap.running && snap.parts.ecu.mode === 'running'", 15*time.Second)
	assert.True(t, truthy(eval(t, page, "snap.driver.ignition")), "the second click did not turn it off")
	stopEngine(t, page)
	time.Sleep(1500 * time.Millisecond)
	assert.False(t, truthy(eval(t, page, "snap.running")), "it started again by itself")
	assert.Equal(t, "stopped: the key was turned off", status(t, page))
}

func TestEngineTwoTabsShareOneEngine(t *testing.T) {
	d := startDemo(t, "engine", 0)
	one := openEngine(t, d.URL)
	two := openEngine(t, d.URL)
	startEngine(t, one)
	waitFor(t, two, "the other tab to see it running", "snap.running", 5*time.Second)
	require.NoError(t, one.Keyboard().Down("KeyW"))
	waitFor(t, two, "the other tab to see the pedal", "snap.driver.pedal > 0.5", 3*time.Second)
	require.NoError(t, one.Keyboard().Up("KeyW"))
	stopEngine(t, one)
	waitFor(t, two, "the other tab to see it stop", "!snap.running", 5*time.Second)
}

func TestEngineComesBackWhenTheServerDoes(t *testing.T) {
	port := freePort(t)
	d := startDemo(t, "engine", port)
	page := openEngine(t, d.URL)
	d.stop()
	waitFor(t, page, "the page to notice", "document.getElementById('status').textContent === 'reconnecting…'", 5*time.Second)
	startDemo(t, "engine", port)
	waitFor(t, page, "the page to reconnect", "document.getElementById('status').textContent === 'stopped: press Start'", 15*time.Second)
}

// TestEngineWheelsTurnRightOnScreen is the animation audit. A wheel drawn
// with N spokes may advance at most half a spoke period per frame, or on a
// 60 Hz screen it looks still or backwards; the starter pinion turns only
// while it is engaged; a still engine draws still wheels.
func TestEngineWheelsTurnRightOnScreen(t *testing.T) {
	d := startDemo(t, "engine", 0)
	page := openEngine(t, d.URL)

	// Record every step turn() takes, and feed the page a snapshot of our own.
	eval(t, page, `() => {
		window.__steps = [];
		const real = turn;
		turn = (key, ratio, spokes) => {
			const before = spin[key] || 0, after = real(key, ratio, spokes);
			let step = after - before; if (step < 0) step += 2 * Math.PI;
			__steps.push({ key, spokes, step });
			return after;
		};
	}`)
	show := func(rpm float64, starter bool) {
		eval(t, page, `([rpm, starter]) => {
			snap.running = rpm > 0;
			snap.parts.crankshaft.crank = { stroke: 10, rpm, dt: 30 / Math.max(rpm, 300) };
			snap.parts.starter.engaged = starter;
			snapAt = performance.now();
			__steps = [];
		}`, []any{rpm, starter})
		time.Sleep(400 * time.Millisecond)
	}
	steps := func() []map[string]any {
		var out []map[string]any
		for _, s := range eval(t, page, "__steps").([]any) {
			out = append(out, s.(map[string]any))
		}
		return out
	}
	maxStep := func(key string) (float64, float64) {
		spokes, worst := 0.0, 0.0
		for _, s := range steps() {
			if s["key"] == key {
				spokes, worst = num(s["spokes"]), math.Max(worst, num(s["step"]))
			}
		}
		return spokes, worst
	}

	// Flat out: every wheel still stays under half its spoke period per frame.
	show(6500, false)
	keys := map[string]bool{}
	for _, s := range steps() {
		keys[s["key"].(string)] = true
	}
	require.Contains(t, keys, "flywheel")
	require.Contains(t, keys, "alt")
	for key := range keys {
		spokes, worst := maxStep(key)
		require.Positive(t, spokes, key)
		assert.Less(t, worst, math.Pi/spokes, "%s turns more than half a spoke per frame: it would look still or backwards", key)
		assert.Positive(t, worst, "%s does not turn at all", key)
	}
	assert.NotContains(t, keys, "starter", "the pinion stays out of a running engine")

	// Cranking: the pinion turns, and so does the flywheel.
	show(250, true)
	_, pinion := maxStep("starter")
	assert.Positive(t, pinion, "the starter pinion does not turn while cranking")
	_, fly := maxStep("flywheel")
	assert.Positive(t, fly)

	// Standing still: nothing turns.
	show(0, false)
	for _, s := range steps() {
		assert.Zero(t, num(s["step"]), "%s turns on a still engine", s["key"])
	}
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The server is the page's side of the engine: it starts the mesh on the
// key, streams every part's state and takes the driver's input. These
// tests drive it over HTTP the way the page does, with the wall-clock
// pacing switched off so a whole run takes milliseconds.

// bench is an engine on a test server.
type bench struct {
	t      *testing.T
	s      *server
	url    string
	client *http.Client
}

func newBench(t *testing.T, setup map[string]map[string]any) *bench {
	t.Helper()
	driver := &Driver{}
	fm, err := getMesh(driver)
	require.NoError(t, err)
	for name, st := range setup {
		for k, v := range st {
			fm.ComponentByName(name).State().Set(k, v)
		}
	}
	s := newServer(fm, driver)
	s.sleep = func(time.Duration) {}
	go s.runEngine()
	ts := httptest.NewServer(s.handler())
	b := &bench{t: t, s: s, url: ts.URL, client: ts.Client()}
	t.Cleanup(func() {
		b.control(`{"ignition": false}`) // leave the engine still, not spinning on
		b.waitStopped()
		ts.Close()
	})
	return b
}

// control posts the driver's input and returns the status code.
func (b *bench) control(body string) int {
	b.t.Helper()
	resp, err := b.client.Post(b.url+"/control", "application/json", strings.NewReader(body))
	require.NoError(b.t, err)
	resp.Body.Close()
	return resp.StatusCode
}

// stream opens /events and returns a function that waits for the next
// snapshot, failing the test after two seconds.
func (b *bench) stream() (next func() snapshot, stop func()) {
	b.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url+"/events", nil)
	require.NoError(b.t, err)
	resp, err := b.client.Do(req)
	require.NoError(b.t, err)
	assert.Equal(b.t, "text/event-stream", resp.Header.Get("Content-Type"))
	snaps := make(chan snapshot, 64)
	go func() {
		defer close(snaps)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if data, ok := bytes.CutPrefix(sc.Bytes(), []byte("data: ")); ok {
				var snap snapshot
				if err := json.Unmarshal(data, &snap); err != nil {
					b.t.Errorf("bad snapshot: %v in %s", err, data)
					return
				}
				snaps <- snap
			}
		}
	}()
	next = func() snapshot {
		b.t.Helper()
		select {
		case snap, ok := <-snaps:
			require.True(b.t, ok, "the stream ended")
			return snap
		case <-time.After(2 * time.Second):
			b.t.Fatal("no snapshot for two seconds")
			return snapshot{}
		}
	}
	stop = func() { cancel(); resp.Body.Close() }
	return next, stop
}

// until reads snapshots until done says so, failing after five seconds.
func (b *bench) until(what string, next func() snapshot, done func(snapshot) bool) snapshot {
	b.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if snap := next(); done(snap) {
			return snap
		}
	}
	b.t.Fatalf("still waiting for %s after five seconds", what)
	return snapshot{}
}

func (b *bench) waitStopped() {
	b.t.Helper()
	for range 500 {
		b.s.mu.Lock()
		running := b.s.running
		b.s.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	b.t.Error("the engine kept running")
}

// parts decodes the snapshot's parts.
func parts(t *testing.T, snap snapshot) map[string]map[string]any {
	t.Helper()
	var p map[string]map[string]any
	require.NoError(t, json.Unmarshal(snap.Parts, &p))
	return p
}

func mode(t *testing.T, snap snapshot) string {
	m, _ := parts(t, snap)["ecu"]["mode"].(string)
	return m
}

func TestThePageAndItsEndpoints(t *testing.T) {
	b := newBench(t, nil)

	resp, err := b.client.Get(b.url + "/")
	require.NoError(t, err)
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	assert.Contains(t, body.String(), "<canvas")

	for _, path := range []string{"/nothing", "/events/x"} {
		resp, err := b.client.Get(b.url + path)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
	}
	resp, err = b.client.Get(b.url + "/control")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	assert.Equal(t, http.StatusBadRequest, b.control(`{"pedal": "all the way"}`))
	assert.Equal(t, http.StatusBadRequest, b.control(`{"pedal": 1, "note": "`+strings.Repeat("x", 2000)+`"}`))
	assert.Equal(t, http.StatusNoContent, b.control(`{}`))
}

func TestANewPageGetsTheLatestSnapshotAtOnce(t *testing.T) {
	b := newBench(t, nil)
	next, stop := b.stream()
	defer stop()
	snap := next()
	assert.False(t, snap.Running)
	assert.Equal(t, "press Start", snap.Reason)
	assert.Equal(t, firingOrder, snap.FiringOrder)
	p := parts(t, snap)
	assert.Len(t, p, 39)
	assert.Equal(t, "off", mode(t, snap))
	assert.Equal(t, 0.8, p["battery"]["soc"])
}

func TestStartRunsAndStopReportsWhy(t *testing.T) {
	b := newBench(t, nil)
	next, stop := b.stream()
	defer stop()
	next()

	assert.Equal(t, http.StatusNoContent, b.control(`{"ignition": true}`))
	snap := b.until("the engine to run", next, func(s snapshot) bool { return s.Running && mode(t, s) == "running" })
	assert.True(t, snap.Driver.Ignition)
	assert.Equal(t, "", snap.Reason)
	assert.Greater(t, snap.CyclesPerStroke, 1.0)

	b.control(`{"ignition": false}`)
	snap = b.until("the engine to stop", next, func(s snapshot) bool { return !s.Running })
	assert.Equal(t, "the key was turned off", snap.Reason)
	assert.False(t, snap.Driver.Ignition)
	assert.Zero(t, state[Crank](b.s.fm, "crankshaft", "crank").RPM)
}

// TestPressingStartTwiceStartsOnce: a second press while the engine is
// starting must not queue a start that would turn the key again later.
func TestPressingStartTwiceStartsOnce(t *testing.T) {
	b := newBench(t, nil)
	next, stop := b.stream()
	defer stop()
	next()

	b.control(`{"ignition": true}`)
	b.control(`{"ignition": true}`)
	b.control(`{"ignition": true}`)
	b.until("the engine to run", next, func(s snapshot) bool { return s.Running && mode(t, s) == "running" })
	assert.Empty(t, b.s.start, "no start waiting behind the running engine")

	b.control(`{"ignition": false}`)
	b.until("the engine to stop", next, func(s snapshot) bool { return !s.Running })
	time.Sleep(100 * time.Millisecond)
	b.s.mu.Lock()
	running := b.s.running
	b.s.mu.Unlock()
	assert.False(t, running, "it started again by itself")
}

func TestStartingAgainAfterEveryKindOfStop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  map[string]map[string]any
		drive  string
		reason string
		fix    string
	}{
		{"key off", nil, `{"ignition": false}`, "the key was turned off", ""},
		{"empty tank", map[string]map[string]any{"fuel-tank": {"liters": 0.02}}, `{"pedal": 0.6}`, "the tank is empty", `{"refuel": true}`},
		{"flat battery", map[string]map[string]any{"battery": {"soc": 0.05}}, ``, "the battery is too weak to turn the starter", ""},
		{"no start", map[string]map[string]any{"air-filter": {"density": 0.0}}, ``, "it would not start", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newBench(t, tc.setup)
			next, stop := b.stream()
			defer stop()
			next()

			b.control(`{"ignition": true}`)
			if tc.drive != "" {
				b.until("the engine to turn", next, func(s snapshot) bool { return s.Running })
				b.control(tc.drive)
			}
			snap := b.until("the engine to stop", next, func(s snapshot) bool { return !s.Running && s.Reason != "" })
			assert.Equal(t, tc.reason, snap.Reason)

			// Mend what stopped it and turn the key again: the same mesh runs again.
			if tc.fix != "" {
				b.control(tc.fix)
			}
			switch tc.name {
			case "flat battery":
				b.s.fm.ComponentByName("battery").State().Set("soc", 0.8)
			case "no start":
				b.s.fm.ComponentByName("air-filter").State().Set("density", 0.97)
			}
			b.control(`{"ignition": true}`)
			snap = b.until("the engine to run again", next, func(s snapshot) bool { return s.Running && mode(t, s) == "running" })
			if tc.fix != "" {
				liters, _ := parts(t, snap)["fuel-tank"]["liters"].(float64)
				assert.Greater(t, liters, tankLiters-1, "the refuel was taken")
			}
		})
	}
}

func TestTheDriverIsSeenAtOnce(t *testing.T) {
	b := newBench(t, nil)
	next, stop := b.stream()
	defer stop()
	next()

	// Standing still, every change is broadcast as soon as it is made.
	b.control(`{"pedal": 0.5}`)
	assert.Equal(t, 0.5, next().Driver.Pedal)
	b.control(`{"pedal": 7}`)
	assert.Equal(t, 1.0, next().Driver.Pedal, "the pedal only goes so far")
	b.control(`{"refuel": true}`)
	assert.True(t, next().Driver.Refuel, "a refuel waits until the engine reads it")
	b.control(`{"pedal": 0}`)
	next()

	// Running, the cockpit reads it once per stroke and the pedal shows in the ECU.
	b.control(`{"ignition": true}`)
	b.until("the engine to run", next, func(s snapshot) bool { return s.Running && mode(t, s) == "running" })
	b.control(`{"pedal": 1}`)
	b.until("the ECU to see the pedal", next, func(s snapshot) bool {
		throttle, _ := parts(t, s)["ecu"]["throttle"].(float64)
		return throttle > 0.9 && !s.Driver.Refuel
	})
}

func TestASlowPageDoesNotHoldUpTheEngine(t *testing.T) {
	b := newBench(t, nil)
	// One page that never reads, one that does.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, b.url+"/events", nil)
	resp, err := b.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	next, stop := b.stream()
	defer stop()
	next()

	b.control(`{"ignition": true}`)
	b.until("the engine to run", next, func(s snapshot) bool { return s.Running && mode(t, s) == "running" })
	b.control(`{"ignition": false}`)
	b.until("the engine to stop", next, func(s snapshot) bool { return !s.Running })

	cancel()
	assert.Eventually(t, func() bool {
		b.s.mu.Lock()
		defer b.s.mu.Unlock()
		return len(b.s.listeners) == 1
	}, time.Second, 10*time.Millisecond, "the gone page is forgotten")
}

func TestEverySnapshotIsWholeAndFinite(t *testing.T) {
	b := newBench(t, map[string]map[string]any{"engine-block": {"temp": 95.0}, "fuel-tank": {"liters": 0.5}})
	next, stop := b.stream()
	defer stop()
	b.control(`{"ignition": true}`)
	b.control(`{"pedal": 0.8}`)
	seen := 0
	for seen < 40 {
		snap := next()
		for name, st := range parts(t, snap) {
			require.NotEmpty(t, st, name)
		}
		require.NotContains(t, string(snap.Parts), "NaN")
		require.NotContains(t, string(snap.Parts), "Inf")
		seen++
	}
}

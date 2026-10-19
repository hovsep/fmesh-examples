package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The server is the page's side of the building: it steps the mesh on a
// ticker, streams every frame and queues the clicks. These tests drive it
// over HTTP the way the page does, with a 1 ms tick so a ride takes a
// fraction of a second.

// frame as the page sees it.
type wireFrame struct {
	Parts      map[string]map[string]any `json:"parts"`
	Cycles     int                       `json:"cycles"`
	Components int                       `json:"components"`
}

func (f wireFrame) on(lamp string) bool { v, _ := f.Parts[lamp]["on"].(bool); return v }
func (f wireFrame) floor(cab string) int {
	v, _ := f.Parts["position-sensor-"+cab]["floor"].(float64)
	return int(v)
}

// site is a building on a test server.
type site struct {
	t      *testing.T
	b      *building
	url    string
	client *http.Client
}

func newSite(t *testing.T, demo bool, tick time.Duration) *site {
	t.Helper()
	fm, err := getMesh()
	require.NoError(t, err)
	b := newBuilding(fm)
	b.tick = tick
	ctx, cancel := context.WithCancel(context.Background())
	go b.run(ctx, demo)
	ts := httptest.NewServer(b.handler())
	t.Cleanup(func() { cancel(); ts.Close() })
	return &site{t: t, b: b, url: ts.URL, client: ts.Client()}
}

// press posts one click and returns the status code.
func (s *site) press(body string) int {
	s.t.Helper()
	resp, err := s.client.Post(s.url+"/press", "application/json", strings.NewReader(body))
	require.NoError(s.t, err)
	resp.Body.Close()
	return resp.StatusCode
}

func click(part string) string { return fmt.Sprintf(`{"part": %q, "port": "press"}`, part) }

// watch opens /events and returns a function that waits for the next
// frame, failing the test after two seconds.
func (s *site) watch() (next func() wireFrame, stop func()) {
	s.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/events", nil)
	require.NoError(s.t, err)
	resp, err := s.client.Do(req)
	require.NoError(s.t, err)
	assert.Equal(s.t, "text/event-stream", resp.Header.Get("Content-Type"))
	frames := make(chan wireFrame, 64)
	go func() {
		defer close(frames)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if data, ok := bytes.CutPrefix(sc.Bytes(), []byte("data: ")); ok {
				var f wireFrame
				if err := json.Unmarshal(data, &f); err != nil {
					s.t.Errorf("bad frame: %v in %s", err, data)
					return
				}
				frames <- f
			}
		}
	}()
	next = func() wireFrame {
		s.t.Helper()
		select {
		case f, ok := <-frames:
			require.True(s.t, ok, "the stream ended")
			return f
		case <-time.After(2 * time.Second):
			s.t.Fatal("no frame for two seconds")
			return wireFrame{}
		}
	}
	stop = func() { cancel(); resp.Body.Close() }
	return next, stop
}

// until reads frames until done says so, failing after five seconds.
func (s *site) until(what string, next func() wireFrame, done func(wireFrame) bool) wireFrame {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f := next(); done(f) {
			return f
		}
	}
	s.t.Fatalf("still waiting for %s after five seconds", what)
	return wireFrame{}
}

func TestThePageAndItsEndpoints(t *testing.T) {
	s := newSite(t, false, time.Millisecond)

	resp, err := s.client.Get(s.url + "/")
	require.NoError(t, err)
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	assert.Contains(t, body.String(), "<canvas")

	resp, err = s.client.Get(s.url + "/press")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	assert.Equal(t, http.StatusNoContent, s.press(click("hall-up-3")))
	assert.Equal(t, http.StatusNoContent, s.press(`{"part": "light-curtain-B", "port": "block"}`))
	assert.Equal(t, http.StatusBadRequest, s.press(click("hall-up-6")), "no up button on the top floor")
	assert.Equal(t, http.StatusBadRequest, s.press(click("motor-A")), "not a button")
	assert.Equal(t, http.StatusBadRequest, s.press(`{"part": "controller-A", "port": "tick"}`), "not a port the page may reach")
	assert.Equal(t, http.StatusBadRequest, s.press(`{"part": `))
	assert.Equal(t, http.StatusBadRequest, s.press(`{"part": "hall-up-3", "port": "press", "note": "`+strings.Repeat("x", 2000)+`"}`))
}

func TestFramesStreamAsTheBuildingLives(t *testing.T) {
	s := newSite(t, false, time.Millisecond)
	next, stop := s.watch()
	defer stop()
	f := next()
	assert.Equal(t, 119, f.Components)
	assert.Equal(t, "parked", f.Parts["controller-A"]["mode"])
	assert.Equal(t, 1, f.floor("A"))
	assert.Equal(t, floors, f.floor("B"))
	assert.InDelta(t, 10, f.Cycles, 3, "a tick takes about ten cycles to settle")
	tick := func(f wireFrame) float64 { v, _ := f.Parts["clock"]["tick"].(float64); return v }
	t1, t2 := tick(f), tick(next())
	assert.Greater(t, t2, t1, "the clock moves between frames")
}

func TestAClickRidesTheCar(t *testing.T) {
	s := newSite(t, false, time.Millisecond)
	next, stop := s.watch()
	defer stop()
	next()

	assert.Equal(t, http.StatusNoContent, s.press(click("car-A-button-4")))
	s.until("the LED to light", next, func(f wireFrame) bool { return f.on("car-A-led-4") })
	f := s.until("car A to open at 4", next, func(f wireFrame) bool {
		open, _ := f.Parts["door-operator-A"]["open"].(bool)
		return open && f.floor("A") == 4
	})
	assert.False(t, f.on("car-A-led-4"), "the LED goes out on arrival")
	assert.Equal(t, 1.0, f.Parts["landing-door-A-4"]["opening"])
	assert.Equal(t, false, f.Parts["safety-chain-A"]["ok"], "open doors open the chain")
}

func TestClicksInOneTickLandTogether(t *testing.T) {
	s := newSite(t, false, 50*time.Millisecond) // a slow tick: two quick clicks queue for one step
	next, stop := s.watch()
	defer stop()
	next()
	assert.Equal(t, http.StatusNoContent, s.press(click("hall-up-2")))
	assert.Equal(t, http.StatusNoContent, s.press(click("hall-down-5")))
	f := s.until("both LEDs", next, func(f wireFrame) bool { return f.on("hall-up-led-2") })
	assert.True(t, f.on("hall-down-led-5"), "both arrived in the same frame")
}

func TestManyPeoplePressingAtOnce(t *testing.T) {
	s := newSite(t, false, time.Millisecond)
	next, stop := s.watch()
	defer stop()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 10 {
				part := fmt.Sprintf("car-%s-button-%d", cabs[i%2], 1+(i+j)%floors)
				assert.Equal(t, http.StatusNoContent, s.press(click(part)))
			}
		}()
	}
	wg.Wait()
	s.until("every stop served", next, func(f wireFrame) bool {
		a, _ := f.Parts["controller-A"]["stops"].([]any)
		b, _ := f.Parts["controller-B"]["stops"].([]any)
		return len(a) == 0 && len(b) == 0 && f.Parts["controller-A"]["mode"] == "parked" && f.Parts["controller-B"]["mode"] == "parked"
	})
}

func TestASlowBrowserSkipsFramesAndIsForgotten(t *testing.T) {
	s := newSite(t, false, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.url+"/events", nil)
	resp, err := s.client.Do(req) // a page that never reads
	require.NoError(t, err)
	next, stop := s.watch()
	defer stop()
	for range 50 {
		next() // the reading page still gets its frames
	}
	cancel()
	resp.Body.Close()
	assert.Eventually(t, func() bool {
		s.b.mu.Lock()
		defer s.b.mu.Unlock()
		return len(s.b.watchers) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestTheDemoRunsOnItsOwn(t *testing.T) {
	s := newSite(t, true, time.Millisecond)
	next, stop := s.watch()
	defer stop()
	s.until("a visitor to call a car", next, func(f wireFrame) bool {
		calls, _ := f.Parts["dispatcher"]["calls"].([]any)
		return len(calls) > 0
	})
	s.until("a car to move", next, func(f wireFrame) bool {
		a, _ := f.Parts["car-A"]["speed"].(float64)
		b, _ := f.Parts["car-B"]["speed"].(float64)
		return a != 0 || b != 0
	})
}

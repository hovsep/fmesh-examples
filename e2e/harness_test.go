// Package e2e drives the browser demos the way a visitor does: a real
// binary on a free port, a real Chrome, clicks and keys in, the page's own
// state out. Run it on its own, one package at a time:
//
//	cd e2e && go test -count=1 -v -run Elevator .
//	cd e2e && go test -count=1 -v -run Engine .
//
// E2E_HEADFUL=1 shows the browser. The tests are serial by design.
package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

var (
	binaries = map[string]string{} // demo → built binary
	pw       *playwright.Playwright
	browser  playwright.Browser
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "fmesh-e2e-")
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer os.RemoveAll(dir)
	for demo, src := range map[string]string{"elevator": "../simulation/elevator", "engine": "../simulation/combustion_engine"} {
		bin := filepath.Join(dir, demo)
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("building %s: %v\n%s", demo, err, out)
			return 1
		}
		binaries[demo] = bin
	}

	if err := playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true}); err != nil {
		fmt.Println("installing the Playwright driver:", err)
		return 1
	}
	pw, err = playwright.Run()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	defer pw.Stop()
	headful := os.Getenv("E2E_HEADFUL") != ""
	opts := playwright.BrowserTypeLaunchOptions{Channel: playwright.String("chrome"), Headless: playwright.Bool(!headful)}
	if headful {
		opts.SlowMo = playwright.Float(50)
	}
	browser, err = pw.Chromium.Launch(opts)
	if err != nil {
		fmt.Println("launching Chrome:", err)
		return 1
	}
	defer browser.Close()
	return m.Run()
}

// freePort asks the kernel for a port nobody listens on.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// demo is one running binary.
type demo struct {
	t   *testing.T
	cmd *exec.Cmd
	URL string
	Log *strings.Builder
}

// startDemo runs a demo binary on the given port (0 for any) with extra
// flags, and waits until it serves its page.
func startDemo(t *testing.T, name string, port int, flags ...string) *demo {
	t.Helper()
	args := append([]string{"-addr", fmt.Sprintf("localhost:%d", port)}, flags...)
	cmd := exec.Command(binaries[name], args...)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = cmd.Stdout
	require.NoError(t, cmd.Start())
	d := &demo{t: t, cmd: cmd, Log: &strings.Builder{}}
	t.Cleanup(d.stop)

	urls := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		re := regexp.MustCompile(`http://\S+`)
		for sc.Scan() {
			line := sc.Text()
			d.Log.WriteString(line + "\n")
			if u := re.FindString(line); u != "" {
				select {
				case urls <- u:
				default:
				}
			}
		}
	}()
	select {
	case d.URL = <-urls:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not say where it listens:\n%s", name, d.Log)
	}
	require.Eventually(t, func() bool {
		resp, err := http.Get(d.URL)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 50*time.Millisecond)
	return d
}

func (d *demo) stop() {
	if d.cmd.Process == nil {
		return
	}
	_ = d.cmd.Process.Kill()
	_ = d.cmd.Wait()
	if d.t.Failed() {
		d.t.Logf("%s output:\n%s", filepath.Base(d.cmd.Path), d.Log)
	}
}

// post sends JSON to the demo and returns the status code.
func (d *demo) post(path, body string) int {
	d.t.Helper()
	resp, err := http.Post(d.URL+path, "application/json", strings.NewReader(body))
	require.NoError(d.t, err)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// openPage opens the demo in a fresh browser context. Any JavaScript error
// or console error fails the test at the end.
func openPage(t *testing.T, url string, opts ...playwright.BrowserNewContextOptions) playwright.Page {
	t.Helper()
	o := playwright.BrowserNewContextOptions{Viewport: &playwright.Size{Width: 1400, Height: 1000}}
	if len(opts) > 0 {
		o = opts[0]
	}
	ctx, err := browser.NewContext(o)
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })
	page, err := ctx.NewPage()
	require.NoError(t, err)

	var errs []string
	page.OnPageError(func(err error) { errs = append(errs, "page error: "+err.Error()) })
	page.OnConsole(func(m playwright.ConsoleMessage) {
		if m.Type() == "error" && !strings.Contains(m.Text(), "Failed to load resource") {
			errs = append(errs, "console error: "+m.Text())
		}
	})
	page.OnResponse(func(r playwright.Response) { // the browser's favicon probe is the one 404 a page may have
		if r.Status() >= 400 && !strings.HasSuffix(r.URL(), "/favicon.ico") {
			errs = append(errs, fmt.Sprintf("%d from %s", r.Status(), r.URL()))
		}
	})
	t.Cleanup(func() {
		for _, e := range errs {
			t.Error(e)
		}
	})
	_, err = page.Goto(url)
	require.NoError(t, err)
	return page
}

// eval runs JavaScript in the page and returns its value.
func eval(t *testing.T, page playwright.Page, js string, args ...any) any {
	t.Helper()
	v, err := page.Evaluate(js, args...)
	require.NoError(t, err, js)
	return v
}

// waitFor polls a JavaScript expression until it is truthy, failing the
// test after the timeout.
func waitFor(t *testing.T, page playwright.Page, what, js string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v, err := page.Evaluate(js); err == nil && truthy(v) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	v, err := page.Evaluate(js)
	t.Fatalf("still waiting for %s after %s (last value %v, %v)", what, timeout, v, err)
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

// nums converts a JSON array of numbers from the page into float64s.
func nums(v any) []float64 {
	list, _ := v.([]any)
	out := make([]float64, 0, len(list))
	for _, x := range list {
		out = append(out, num(x))
	}
	return out
}

// rect is a hit rectangle on the elevator's canvas, in canvas coordinates.
type rect struct{ X, Y, W, H float64 }

// canvasPoint maps a point in canvas coordinates to the page.
func canvasPoint(t *testing.T, page playwright.Page, x, y float64) (float64, float64) {
	t.Helper()
	box, err := page.Locator("canvas").BoundingBox()
	require.NoError(t, err)
	w := num(eval(t, page, "W")) // the canvas's logical width and height
	h := num(eval(t, page, "H"))
	return box.X + x*box.Width/w, box.Y + y*box.Height/h
}

var _ = context.Background

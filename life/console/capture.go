package console

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

// Capture redirects the process's standard output into a channel of lines.
//
// The simulation and its command handlers report what they are doing with plain
// fmt.Println, from the simulation goroutine, at moments the UI cannot predict.
// Written straight to the terminal those would land in the middle of whatever
// the UI had drawn.
//
// The redirection happens at the file-descriptor level rather than by swapping
// the os.Stdout variable, because swapping the variable only catches writers
// that look it up at the moment they write. fmesh gives every component a
// log.New(os.Stdout, ...) when the component is constructed -- long before the
// UI exists -- so such a logger holds the original file and would bypass a
// variable swap entirely. Redirecting the descriptor catches those too, along
// with anything a dependency decides to print.
type Capture struct {
	original *os.File // the os.Stdout value that was replaced
	terminal *os.File // a duplicate of the real stdout, kept for the renderer
	savedFd  int      // duplicate of the original descriptor; -1 if unavailable
	reader   *os.File
	writer   *os.File
	restored bool

	Lines chan string
}

// CaptureStdout redirects standard output into a pipe. Restore must be called to
// put it back, or the terminal is left writing into a pipe nothing reads.
func CaptureStdout(buffer int) (*Capture, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	c := &Capture{
		original: os.Stdout,
		terminal: os.Stdout,
		savedFd:  -1,
		reader:   reader,
		writer:   writer,
		Lines:    make(chan string, buffer),
	}

	// Keep a handle on the real terminal before anything is redirected, then
	// point the descriptor at the pipe.
	if savedFd, dupErr := syscall.Dup(int(os.Stdout.Fd())); dupErr == nil {
		if dupErr = syscall.Dup2(int(writer.Fd()), int(os.Stdout.Fd())); dupErr == nil {
			c.savedFd = savedFd
			c.terminal = os.NewFile(uintptr(savedFd), "/dev/stdout")
		} else {
			_ = syscall.Close(savedFd)
		}
	}
	// If the descriptor could not be redirected we still swap the variable, which
	// catches ordinary prints. Component logs would escape to the terminal, which
	// is untidy but not fatal.
	os.Stdout = writer

	go c.read()
	return c, nil
}

func (c *Capture) read() {
	defer close(c.Lines)

	scanner := bufio.NewScanner(c.reader)
	// Individual lines can be long (a help table, a jobs listing), so allow more
	// than bufio's default 64KiB before giving up on one.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		select {
		case c.Lines <- line:
		default:
			// The console is not keeping up. Drop the line rather than blocking:
			// back-pressure here would stall the simulation goroutine mid-command.
		}
	}
}

// Terminal returns the real stdout, from before the redirection.
//
// Anything that must actually reach the screen -- above all the renderer drawing
// the console itself -- has to be pointed at this. Left on os.Stdout it would
// draw into the capture pipe and feed its own frames back as captured output.
func (c *Capture) Terminal() *os.File { return c.terminal }

// Restore puts the real stdout back and stops the reader. It is safe to call
// more than once, since Run both defers it and calls it before reporting errors.
func (c *Capture) Restore() {
	if c.restored {
		return
	}
	c.restored = true

	if c.savedFd >= 0 {
		_ = syscall.Dup2(c.savedFd, int(os.Stdout.Fd()))
	}
	os.Stdout = c.original

	// Closing the write end ends the scanner, which closes Lines.
	_ = c.writer.Close()
}

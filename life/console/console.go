// Package console is an interactive front end for driving the simulation.
//
// It replaces the plain line-at-a-time prompt with something worth living in:
// command history on the arrow keys, tab completion over the registered
// commands, and a scrolling transcript that survives the simulation printing
// from its own goroutine.
//
// It lives here rather than in step_sim so the shared simulation package stays
// free of UI dependencies; step_sim only knows about the CommandSource interface.
package console

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
)

// maxTranscript bounds the scrollback so a long run cannot grow without limit.
const maxTranscript = 2000

// capturedLine carries one line of simulation output into the Bubble Tea loop.
type capturedLine string

// capturedLinesClosed says the simulation's output has ended.
type capturedLinesClosed struct{}

// Console is a step_sim.CommandSource backed by a full-screen terminal UI.
type Console struct {
	// Commands is consulted for tab completion and the command listing. It is
	// the simulation's live map, so commands registered at init are included.
	commands func() []string

	historyPath string
}

// New returns a console. commandNames is called when completing, so it sees
// whatever the simulation has registered by then.
func New(commandNames func() []string, historyPath string) *Console {
	return &Console{commands: commandNames, historyPath: historyPath}
}

// Run implements step_sim.CommandSource. It owns cmdChan and closes it on exit,
// which is what tells the simulation to stop.
func (c *Console) Run(cmdChan chan step_sim.Command) {
	defer close(cmdChan)

	// Capture before starting Bubble Tea, so nothing the simulation prints can
	// reach the terminal directly and corrupt the rendered frame.
	capture, err := captureStdout(1024)
	if err != nil {
		fmt.Println("could not capture simulation output:", err)
		return
	}
	defer capture.Restore()

	model := newModel(c, cmdChan)
	// Render to the real terminal, not to os.Stdout: that now points at the
	// capture pipe, and drawing there would make the console invisible and echo
	// its own frames back at itself.
	program := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithOutput(capture.Terminal()),
		tea.WithInput(os.Stdin),
	)

	// Feed captured output in as messages rather than touching the model
	// directly, which keeps every state change on Bubble Tea's own goroutine.
	go func() {
		for line := range capture.Lines {
			program.Send(capturedLine(line))
		}
		program.Send(capturedLinesClosed{})
	}()

	finalModel, err := program.Run()

	// Put stdout back before printing anything else, or the message goes into
	// the pipe nobody is reading any more.
	capture.Restore()

	if err != nil {
		fmt.Println("console error:", err)
	}
	if m, ok := finalModel.(model_); ok {
		if saveErr := m.history.Save(); saveErr != nil {
			fmt.Println("could not save command history:", saveErr)
		}
	}
}

// model_ is the Bubble Tea model. The trailing underscore keeps it from
// colliding with the constructor's readability.
type model_ struct {
	console *Console
	cmdChan chan step_sim.Command

	input   textinput.Model
	history *History

	transcript []string
	scroll     int // lines scrolled up from the bottom; 0 means following

	width, height int
	quitting      bool
}

func newModel(c *Console, cmdChan chan step_sim.Command) model_ {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "type a command, or 'help'"
	input.Focus()
	input.CharLimit = 0 // scenarios on one line get long

	return model_{
		console: c,
		cmdChan: cmdChan,
		input:   input,
		history: LoadHistory(c.historyPath),
		width:   100,
		height:  30,
	}
}

func (m model_) Init() tea.Cmd { return textinput.Blink }

func (m model_) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// A terminal that reports no size (some pseudo-terminals do) would
		// otherwise collapse the layout to nothing; keep the last known size.
		if msg.Width > 0 && msg.Height > 0 {
			m.width, m.height = msg.Width, msg.Height
			m.input.Width = max(msg.Width-4, 20)
		}
		return m, nil

	case capturedLine:
		m.append(string(msg))
		return m, nil

	case capturedLinesClosed:
		// The simulation has stopped printing, which means it has stopped.
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model_) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlD:
		return m.quit()

	case tea.KeyEnter:
		return m.submit()

	case tea.KeyUp:
		if line, ok := m.history.Prev(m.input.Value()); ok {
			m.input.SetValue(line)
			m.input.CursorEnd()
		}
		return m, nil

	case tea.KeyDown:
		if line, ok := m.history.Next(); ok {
			m.input.SetValue(line)
			m.input.CursorEnd()
		}
		return m, nil

	case tea.KeyTab:
		m.complete()
		return m, nil

	case tea.KeyPgUp:
		m.scrollBy(m.pageSize())
		return m, nil

	case tea.KeyPgDown:
		m.scrollBy(-m.pageSize())
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model_) submit() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")
	m.scroll = 0 // jump back to the live end on any input

	if line == "" {
		return m, nil
	}

	m.history.Add(line)
	m.append("› " + line)

	if line == string(step_sim.Exit) {
		return m.quit()
	}

	// Non-blocking: the channel is buffered, and a full one means the simulation
	// is wedged. Reporting that beats freezing the UI behind it.
	select {
	case m.cmdChan <- step_sim.Command(line):
	default:
		m.append("! simulation is not accepting commands right now")
	}
	return m, nil
}

func (m model_) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

// complete fills in the longest unambiguous completion of the current word, and
// lists the candidates when there is more than one.
func (m *model_) complete() {
	value := m.input.Value()
	fields := strings.Fields(value)

	// Only the command name completes; arguments are too varied to guess.
	if len(fields) > 1 || (len(fields) == 1 && strings.HasSuffix(value, " ")) {
		return
	}

	prefix := ""
	if len(fields) == 1 {
		prefix = fields[0]
	}

	var matches []string
	for _, name := range m.console.commands() {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	slices.Sort(matches)

	switch len(matches) {
	case 0:
		return
	case 1:
		m.input.SetValue(matches[0] + " ")
		m.input.CursorEnd()
	default:
		m.input.SetValue(commonPrefix(matches))
		m.input.CursorEnd()
		m.append("  " + strings.Join(matches, "  "))
	}
}

// commonPrefix returns the longest prefix shared by every string.
func commonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}

	prefix := values[0]
	for _, v := range values[1:] {
		for !strings.HasPrefix(v, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

func (m *model_) append(line string) {
	m.transcript = append(m.transcript, line)
	if len(m.transcript) > maxTranscript {
		m.transcript = m.transcript[len(m.transcript)-maxTranscript:]
	}

	// Scrolled-back readers stay where they are, rather than being yanked to the
	// bottom every time the simulation says something.
	if m.scroll > 0 {
		m.scroll++
	}
}

func (m *model_) scrollBy(lines int) {
	m.scroll = min(max(m.scroll+lines, 0), max(len(m.transcript)-m.pageSize(), 0))
}

// pageSize is how many transcript lines are visible, leaving room for the header
// and the input line.
func (m model_) pageSize() int {
	return max(m.height-4, 1)
}

func (m model_) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	header := headerStyle.Width(m.width).Render(" Life — simulation console ")

	visible := m.pageSize()
	end := len(m.transcript) - m.scroll
	start := max(end-visible, 0)

	body := make([]string, 0, visible)
	for _, line := range m.transcript[max(start, 0):max(end, 0)] {
		body = append(body, renderLine(line, m.width))
	}
	// Pad so the input line stays pinned to the bottom rather than drifting up
	// as the transcript fills.
	for len(body) < visible {
		body = append(body, "")
	}

	footer := hintStyle.Render("↑/↓ history · tab complete · pgup/pgdn scroll · 'help' · 'exit'")
	if m.scroll > 0 {
		footer = hintStyle.Render(fmt.Sprintf("scrolled back %d lines — pgdn to follow again", m.scroll))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Join(body, "\n"),
		m.input.View(),
		footer,
	)
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#5f5fd7"))
	hintStyle  = lipgloss.NewStyle().Faint(true)
	echoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd7af")).Bold(true)
	alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))
)

// renderLine styles a transcript line by what it is: an echoed command, a
// problem, or ordinary simulation output.
func renderLine(line string, width int) string {
	switch {
	case strings.HasPrefix(line, "› "):
		return echoStyle.Render(truncate(line, width))
	case strings.HasPrefix(line, "! "), strings.Contains(line, "invalid"), strings.HasPrefix(line, "Unknown command"):
		return alertStyle.Render(truncate(line, width))
	default:
		return truncate(line, width)
	}
}

func truncate(line string, width int) string {
	if width <= 1 || lipgloss.Width(line) <= width {
		return line
	}
	// Cut by runes so a multi-byte character is never split in half.
	runes := []rune(line)
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}

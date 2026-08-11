// Package console provides the command line that sits at the bottom of the
// dashboard: a prompt with history and tab completion, above a scrollback of
// what the simulation has printed.
//
// It also carries the pieces the integrated UI needs to survive the simulation
// printing from its own goroutine: Capture (see capture.go) redirects stdout
// into a channel, and History (see history.go) persists the command line across
// sessions. It stays in its own package so the simulation libraries need not
// know about any of it.
package console

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/simulation/sim/command"
)

// maxTranscript bounds the scrollback so a long run cannot grow without limit.
const maxTranscript = 2000

// Pane is a self-contained piece of a larger Bubble Tea model. The parent
// forwards the messages it wants the pane to handle (key input, and each
// captured output line via Append) and paints Pane.View somewhere on screen.
//
// Submitting a line sends it to the simulation, "exit" included: the simulation
// is what ends the session, and the dashboard closes when it does. ctrl+c and
// ctrl+d shortcut that by quitting the program directly.
type Pane struct {
	// commands is consulted for tab completion. It is the simulation's live
	// command list, so commands registered at init are included.
	commands func() []string
	lines    chan<- command.Line

	input   textinput.Model
	history *History

	transcript []string
	scroll     int // lines scrolled up from the bottom; 0 means following
	rows       int // visible transcript rows, set on each View

	width int
}

// NewPane returns a command pane. commandNames is called when completing, so it
// sees whatever the simulation has registered by then.
func NewPane(commandNames func() []string, historyPath string, lines chan<- command.Line) *Pane {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "type a command, or 'help'"
	input.Focus()
	input.CharLimit = 0 // scenarios on one line get long

	return &Pane{
		commands: commandNames,
		lines:    lines,
		input:    input,
		history:  LoadHistory(historyPath),
		rows:     6,
		width:    100,
	}
}

// Blink returns the cursor-blink command; the parent includes it in its Init.
func (p *Pane) Blink() tea.Cmd { return textinput.Blink }

// SetWidth tells the pane how wide it is drawn, so the input and transcript wrap
// to the right column.
func (p *Pane) SetWidth(w int) {
	if w > 0 {
		p.width = w
		p.input.Width = max(w-4, 20)
	}
}

// Update feeds a message to the pane and returns any resulting command. It
// returns tea.Quit when the user asks to exit.
func (p *Pane) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return cmd
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyCtrlD:
		return tea.Quit
	case tea.KeyEnter:
		return p.submit()
	case tea.KeyUp:
		if line, ok := p.history.Prev(p.input.Value()); ok {
			p.input.SetValue(line)
			p.input.CursorEnd()
		}
		return nil
	case tea.KeyDown:
		if line, ok := p.history.Next(); ok {
			p.input.SetValue(line)
			p.input.CursorEnd()
		}
		return nil
	case tea.KeyTab:
		p.complete()
		return nil
	case tea.KeyPgUp:
		p.scrollBy(p.rows)
		return nil
	case tea.KeyPgDown:
		p.scrollBy(-p.rows)
		return nil
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

func (p *Pane) submit() tea.Cmd {
	line := strings.TrimSpace(p.input.Value())
	p.input.SetValue("")
	p.scroll = 0 // jump back to the live end on any input

	if line == "" {
		return nil
	}

	p.history.Add(line)

	// "clear" is the console's own, the way it is in any shell: it is about the
	// window rather than the simulation, and sending it down to a mesh that has
	// never heard of a transcript would only earn an "unknown command".
	if line == clearCommand {
		p.Clear()
		return nil
	}

	p.Append("› " + line)

	// Non-blocking: the channel is buffered, and a full one means the simulation
	// is wedged. Reporting that beats freezing the UI behind it.
	select {
	case p.lines <- command.Line(line):
	default:
		p.Append("! simulation is not accepting commands right now")
	}
	return nil
}

// clearCommand empties the transcript. Named so the pane can offer it in
// completion alongside the simulation's own commands.
const clearCommand = "clear"

// Clear empties the transcript and returns to following the newest line.
//
// A simulation talks: components log what they are doing, and a few minutes of
// that buries whatever you were reading. Clearing is the same gesture as in any
// other shell, and it is why the pane keeps its own transcript rather than
// drawing straight from a stream.
func (p *Pane) Clear() {
	p.transcript = nil
	p.scroll = 0
}

// SaveHistory persists the command history; call it as the program exits.
func (p *Pane) SaveHistory() error { return p.history.Save() }

// Append adds a line to the scrollback.
func (p *Pane) Append(line string) {
	p.transcript = append(p.transcript, line)
	if len(p.transcript) > maxTranscript {
		p.transcript = p.transcript[len(p.transcript)-maxTranscript:]
	}

	// Scrolled-back readers stay where they are, rather than being yanked to the
	// bottom every time the simulation says something.
	if p.scroll > 0 {
		p.scroll++
	}
}

func (p *Pane) scrollBy(lines int) {
	p.scroll = min(max(p.scroll+lines, 0), max(len(p.transcript)-p.rows, 0))
}

// View renders the pane into height rows: the scrollback, the input line, and a
// one-line hint.
func (p *Pane) View(height int) string {
	p.rows = max(height-2, 1)

	end := len(p.transcript) - p.scroll
	start := max(end-p.rows, 0)

	body := make([]string, 0, p.rows)
	for _, line := range p.transcript[max(start, 0):max(end, 0)] {
		body = append(body, renderLine(line, p.width))
	}
	// Pad so the input line stays pinned to the bottom rather than drifting up
	// as the transcript fills.
	for len(body) < p.rows {
		body = append(body, "")
	}

	footer := hintStyle.Render("↑/↓ history · tab complete · pgup/pgdn scroll · 'help' · 'exit'")
	if p.scroll > 0 {
		footer = hintStyle.Render(fmt.Sprintf("scrolled back %d lines — pgdn to follow again", p.scroll))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(body, "\n"),
		p.input.View(),
		footer,
	)
}

// complete fills in the longest unambiguous completion of the current word, and
// lists the candidates when there is more than one.
func (p *Pane) complete() {
	value := p.input.Value()
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
	// The console's own builtin completes alongside the simulation's commands,
	// since from the prompt there is no difference between them.
	for _, name := range append(p.commands(), clearCommand) {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	slices.Sort(matches)

	switch len(matches) {
	case 0:
		return
	case 1:
		p.input.SetValue(matches[0] + " ")
		p.input.CursorEnd()
	default:
		p.input.SetValue(commonPrefix(matches))
		p.input.CursorEnd()
		p.Append("  " + strings.Join(matches, "  "))
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

var (
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

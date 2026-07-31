// Package tui is the integrated front end: the tabbed dashboard on top and a
// command line at the bottom, driving one simulation that runs in the same
// process. It is a command.Source (App.Run owns the input loop) and
// hands the simulation a channel sink to publish into, so no socket or second
// process is involved.
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tui/dash"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/protocol"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
	"github.com/hovsep/fmesh-examples/life/tui/views"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/console"
	"github.com/hovsep/fmesh-examples/simulation/sink"
)

// Render cadence bounds. The render interval controls how often the TUI
// repaints from the latest state; it is fully independent of how fast the
// simulation runs or how fast telemetry arrives.
const (
	defaultRenderInterval = 100 * time.Millisecond // 10 FPS
	minRenderInterval     = 16 * time.Millisecond  // ~60 FPS
	maxRenderInterval     = time.Second            // 1 FPS
)

// Layout constants. The command pane gets a fixed slice of the height; the
// dashboard takes the rest.
const (
	// paneHeightSmall/Large are the two sizes the console toggles between.
	//
	// Eight rows leaves about six for the transcript, which a talkative
	// simulation fills in seconds -- long enough to lose whatever you were
	// reading, short enough that you cannot scroll back to it comfortably. The
	// large size is for when the log is what you are actually watching.
	paneHeightSmall = 8
	paneHeightLarge = 20

	tabRowY = 2 // the tab bar sits just under the two-line header
)

// App is the integrated front end as a command.Source. It owns the
// telemetry sink the simulation publishes to and the Bubble Tea program that
// draws it.
type App struct {
	commands    func() []string
	historyPath string
	sink        *sink.Channel
}

// New returns an App. commandNames is called for tab completion, so it sees
// whatever the simulation has registered by the time the user completes.
func New(commandNames func() []string, historyPath string) *App {
	return &App{
		commands:    commandNames,
		historyPath: historyPath,
		// Generous buffer: a snapshot is many lines and Publish must never block
		// the simulation goroutine.
		sink: sink.NewChannel(4096),
	}
}

// Sink returns the telemetry sink to hand the session via session.WithSink.
func (a *App) Sink() sink.Sink { return a.sink }

// Run implements command.Source: it feeds typed lines to the simulation and
// returns when the user quits the dashboard, which ends the session.
//
// It also watches ctx, so a simulation that ends on its own -- a scheduled
// exit, a body that has died and been told to stop -- closes the dashboard
// instead of leaving it accepting commands nothing will ever read.
func (a *App) Run(ctx context.Context, lines chan<- command.Line) {
	// Capture before starting Bubble Tea, so nothing the simulation prints can
	// reach the terminal directly and corrupt the rendered frame.
	capture, err := console.CaptureStdout(1024)
	if err != nil {
		fmt.Println("could not capture simulation output:", err)
		return
	}
	defer capture.Restore()

	state := models.NewAppState(2000)

	// Ingestion is decoupled from rendering: a background goroutine drains
	// telemetry into the mutex-guarded state, and the Bubble Tea loop only
	// renders (at renderInterval), so a fast simulation never floods or stalls
	// the UI.
	go ingest(a.sink.Lines(), state)

	model := newModel(state, console.NewPane(a.commands, a.historyPath, lines))

	// Render to the real terminal, not to os.Stdout: that now points at the
	// capture pipe, and drawing there would make the UI invisible and echo its
	// own frames back at itself.
	program := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
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

	// Close the dashboard when the simulation ends, whoever ended it.
	go func() {
		<-ctx.Done()
		program.Quit()
	}()

	finalModel, err := program.Run()

	// Put stdout back before printing anything else, or the message goes into the
	// pipe nobody is reading any more.
	capture.Restore()
	if err != nil {
		fmt.Println("tui error:", err)
	}
	if m, ok := finalModel.(Model); ok {
		if saveErr := m.pane.SaveHistory(); saveErr != nil {
			fmt.Println("could not save command history:", saveErr)
		}
	}
}

// ingest drains telemetry lines into the application state until the channel is
// closed (which the sink does once the simulation has stopped).
func ingest(lines <-chan string, state *models.AppState) {
	for line := range lines {
		if line == "" {
			continue
		}
		update, err := protocol.Parse(line)
		if err != nil {
			// Non-numeric values are expected (labels, text) and not worth
			// surfacing; anything else is a real parse problem.
			if !strings.Contains(err.Error(), "skipping non-numeric") {
				state.SetLastError(err)
			}
			continue
		}
		state.UpdateSignal(update.Key, update.Value)
		state.IncrementTick()
	}
}

// Messages fed in from the captured-output goroutine.
type (
	capturedLine        string
	capturedLinesClosed struct{}
)

type tickMsg time.Time

// tickEvery returns a command that sends a render tick at regular intervals.
func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Model is the merged Bubble Tea model: the dashboard views plus the command
// pane. Its pointer fields (state, pane, views) are shared across the value
// copies Bubble Tea makes, so pane input and telemetry survive each Update.
type Model struct {
	state *models.AppState
	pane  *console.Pane

	screens    []dash.Screen
	source     *dash.State
	paneHeight int

	width          int
	height         int
	renderInterval time.Duration
	lungsSplit     bool
}

func newModel(state *models.AppState, pane *console.Pane) Model {
	// Screens that need bespoke rendering; everything else is generated from the
	// catalog, so a new metric appears without any code here changing.
	subject := telemetry.DefaultSubject
	legacy := &legacyViews{
		overview:    views.NewOverviewView(state),
		cardiac:     views.NewCardiacView(state),
		respiratory: views.NewRespiratoryView(state),
		feelings:    views.NewFeelingsView(state, subject),
		body:        views.NewBodyView(state, subject),
	}

	model := Model{
		state:          state,
		pane:           pane,
		screens:        screens(legacy),
		source:         dash.NewState(state, subject),
		paneHeight:     paneHeightSmall,
		width:          120,
		height:         40,
		renderInterval: defaultRenderInterval,
		lungsSplit:     true,
	}
	legacy.lungsSplit = func() bool { return model.lungsSplit }
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickEvery(m.renderInterval), m.pane.Blink())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			m.width = msg.Width
			m.height = msg.Height
			m.pane.SetWidth(msg.Width)
		}
		return m, nil

	case capturedLine:
		m.pane.Append(string(msg))
		return m, nil

	case capturedLinesClosed:
		// The simulation has stopped printing, which means it has stopped.
		return m, tea.Quit

	case tickMsg:
		// Render tick: Bubble Tea repaints via View() after this returns. The
		// interval is re-read each tick so FPS changes take effect immediately.
		return m, tickEvery(m.renderInterval)

	case tea.MouseMsg:
		if view, ok := m.tabAtMouse(msg); ok {
			m.state.SetView(view)
		}
		return m, nil

	case tea.KeyMsg:
		// Navigation uses modifiers and the mouse so that ordinary keystrokes
		// (letters, digits) always reach the command line.
		if handled := m.handleNav(msg); handled {
			return m, nil
		}
		return m, m.pane.Update(msg)
	}

	// Anything else (e.g. the cursor blink) belongs to the input.
	return m, m.pane.Update(msg)
}

// handleNav applies view/UI navigation bound to modifier keys, reporting whether
// it consumed the key. It takes a pointer so its mutations land on the copy
// Update returns.
func (m *Model) handleNav(msg tea.KeyMsg) bool {
	s := msg.String()

	// alt+1..alt+7 jump straight to a view.
	if strings.HasPrefix(s, "alt+") && len(s) == 5 && s[4] >= '1' && s[4] <= '9' {
		if idx := int(s[4] - '1'); idx < len(models.Views()) {
			m.state.SetView(models.Views()[idx])
			return true
		}
	}

	switch s {
	case "ctrl+right":
		m.state.NextView()
	case "ctrl+left":
		m.state.PrevView()
	case "alt+s":
		// Toggle the respiratory view between split and overlaid lungs.
		m.lungsSplit = !m.lungsSplit
	case "alt+c":
		// Grow the console when the log is what you are watching, and shrink it
		// again when the dashboard is.
		if m.paneHeight == paneHeightSmall {
			m.paneHeight = paneHeightLarge
		} else {
			m.paneHeight = paneHeightSmall
		}
	case "ctrl+up":
		m.renderInterval = max(m.renderInterval/2, minRenderInterval)
	case "ctrl+down":
		m.renderInterval = min(m.renderInterval*2, maxRenderInterval)
	default:
		return false
	}
	return true
}

// tabAtMouse returns the view whose tab was left-clicked, if any.
func (m Model) tabAtMouse(msg tea.MouseMsg) (models.ViewType, bool) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft || msg.Y != tabRowY {
		return 0, false
	}

	x := 0
	current := m.state.GetView()
	for _, view := range models.Views() {
		style := styles.TabInactiveStyle
		if view == current {
			style = styles.TabActiveStyle
		}
		w := lipgloss.Width(style.Render(models.ViewName(view)))
		if msg.X >= x && msg.X < x+w {
			return view, true
		}
		x += w
	}
	return 0, false
}

func (m Model) View() string {
	if err := m.state.GetLastError(); err != nil {
		return fmt.Sprintf("Error: %v\n\nPress ctrl+c to quit.", err)
	}

	if m.width < 80 || m.height < 24 {
		return fmt.Sprintf("Terminal too small!\n\nMinimum size: 80 columns x 24 rows\nCurrent size: %d columns x %d rows\n\nPlease resize your terminal.", m.width, m.height)
	}

	header := m.renderHeader()
	tabs := m.renderTabs()
	help := m.renderHelp()

	// header(2) + tabs(1) + help(1) + content + divider(1) + pane.
	paneHeight := min(m.paneHeight, max(m.height-8, 4))
	contentHeight := max(m.height-5-paneHeight, 3)

	content := m.renderView(m.state.GetView(), contentHeight)

	divider := lipgloss.NewStyle().Foreground(styles.ColorBorder).Render(strings.Repeat("─", m.width))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		tabs,
		help,
		content,
		divider,
		m.pane.View(paneHeight),
	)
}

// renderView draws one screen from the table.
func (m Model) renderView(view models.ViewType, height int) string {
	for _, screen := range m.screens {
		if screen.View == view {
			return dash.Render(screen, m.source, m.width, height)
		}
	}
	return ""
}

func (m Model) renderHeader() string {
	status := "Alive ●"
	statusStyle := styles.StatusAliveStyle
	if !m.state.GetIsAlive() {
		status = "Dead ✗"
		statusStyle = styles.StatusDeadStyle
	}

	// Simulated time and tick count come from the sim's own clock rather than the
	// TUI's wall clock, so they stay meaningful whatever speed it runs at.
	elapsed := time.Duration(m.state.GetLatestValue(models.SimDurationKey)) * time.Millisecond
	timeStr := fmt.Sprintf("Sim: %02d:%02d:%02d", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
	tickStr := fmt.Sprintf("Tick: %d", int64(m.state.GetLatestValue(models.SimTickCountKey)))

	left := styles.HeaderStyle.Render("Human Leon")
	middle := statusStyle.Render(status)
	right := styles.HeaderStyle.Render(fmt.Sprintf("%s | %s", timeStr, tickStr))

	usedWidth := lipgloss.Width(left) + lipgloss.Width(middle) + lipgloss.Width(right)
	spacingLeft := max((m.width-usedWidth)/2, 0)
	spacingRight := max(m.width-usedWidth-spacingLeft, 0)

	headerLine := left +
		lipgloss.NewStyle().Width(spacingLeft).Render("") +
		middle +
		lipgloss.NewStyle().Width(spacingRight).Render("") +
		right

	border := lipgloss.NewStyle().
		Foreground(styles.ColorBorder).
		Render(strings.Repeat("═", max(m.width, 1)))

	return headerLine + "\n" + border
}

func (m Model) renderTabs() string {
	currentView := m.state.GetView()

	var tabs []string
	for _, view := range models.Views() {
		if view == currentView {
			tabs = append(tabs, styles.TabActiveStyle.Render(models.ViewName(view)))
		} else {
			tabs = append(tabs, styles.TabInactiveStyle.Render(models.ViewName(view)))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m Model) renderHelp() string {
	fps := int(time.Second / m.renderInterval)
	helpText := fmt.Sprintf("ctrl+←/→ or alt+1-7 or click: tabs | alt+s: split lungs | alt+c: console size | ctrl+↑/↓: FPS (%d) | clear | exit/ctrl+c: quit", fps)
	return styles.HelpStyle.Render(helpText)
}

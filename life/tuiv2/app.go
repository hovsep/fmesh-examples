package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tuiv2/models"
	"github.com/hovsep/fmesh-examples/life/tuiv2/protocol"
	"github.com/hovsep/fmesh-examples/life/tuiv2/styles"
	"github.com/hovsep/fmesh-examples/life/tuiv2/views"
)

// Render cadence bounds. The render interval controls how often the TUI
// repaints from the latest state; it is fully independent of how fast the
// simulation runs or how fast updates arrive over the socket.
const (
	defaultRenderInterval = 100 * time.Millisecond // 10 FPS
	minRenderInterval     = 16 * time.Millisecond  // ~60 FPS
	maxRenderInterval     = time.Second            // 1 FPS
)

// The simulation clock, published as scalars of the habitat's tick signal.
const (
	simDurationKey  = "time::tick:sim_duration_ms"
	simTickCountKey = "time::tick:tick_count"
)

// Model represents the Bubble Tea application model
type Model struct {
	state           *models.AppState
	reader          *protocol.Reader
	overviewView    *views.OverviewView
	respiratoryView *views.RespiratoryView
	feelingsView    *views.FeelingsView
	metricViews     map[models.ViewType]*views.MetricsView
	width           int
	height          int
	renderInterval  time.Duration
}

// NewModel creates a new application model
func NewModel(socketPath string) (*Model, error) {
	// Create app state
	state := models.NewAppState(2000)

	// Create protocol reader
	reader := protocol.NewReader(socketPath)
	err := reader.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}

	// Start reading
	reader.Start()

	// Ingestion is decoupled from rendering: continuously drain updates into
	// the mutex-guarded state from a background goroutine. The Bubble Tea loop
	// only renders (at renderInterval), so a fast simulation never floods or
	// stalls the UI.
	go ingest(reader, state)

	// Screens that need bespoke rendering; everything else is generated from the
	// catalog, so a new metric appears without any code here changing.
	subject := telemetry.DefaultSubject
	metricViews := make(map[models.ViewType]*views.MetricsView)
	for _, view := range models.Views {
		metricViews[view] = views.NewMetricsView(state, strings.ToUpper(models.ViewName(view)), view, subject)
	}

	return &Model{
		state:           state,
		reader:          reader,
		overviewView:    views.NewOverviewView(state),
		respiratoryView: views.NewRespiratoryView(state),
		feelingsView:    views.NewFeelingsView(state, subject),
		metricViews:     metricViews,
		width:           120,
		height:          40,
		renderInterval:  defaultRenderInterval,
	}, nil
}

// ingest continuously drains reader updates into the application state.
func ingest(reader *protocol.Reader, state *models.AppState) {
	for {
		select {
		case update, ok := <-reader.Updates:
			if !ok {
				return
			}
			state.UpdateSignal(update.Key, update.Value)
			state.IncrementTick()
		case err, ok := <-reader.Errors:
			if !ok {
				return
			}
			state.SetLastError(err)
		}
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	return tickEvery(m.renderInterval)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.state.NextView()
			return m, nil

		case "shift+tab":
			m.state.PrevView()
			return m, nil

		case "1", "2", "3", "4", "5", "6":
			if index := int(msg.String()[0] - '1'); index < len(models.Views) {
				m.state.SetView(models.Views[index])
			}
			return m, nil

		case "+", "=":
			// Faster refresh (shorter interval)
			m.renderInterval = max(m.renderInterval/2, minRenderInterval)
			return m, nil

		case "-", "_":
			// Slower refresh (longer interval)
			m.renderInterval = min(m.renderInterval*2, maxRenderInterval)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Render tick: Bubble Tea repaints via View() after this returns. The
		// interval is re-read each tick so FPS changes take effect immediately.
		return m, tickEvery(m.renderInterval)
	}

	return m, nil
}

// View renders the application
func (m Model) View() string {
	if err := m.state.GetLastError(); err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", err)
	}

	// Ensure valid dimensions
	if m.width < 80 || m.height < 24 {
		return fmt.Sprintf("Terminal too small!\n\nMinimum size: 80 columns x 24 rows\nCurrent size: %d columns x %d rows\n\nPlease resize your terminal and press any key.", m.width, m.height)
	}

	// Render header
	header := m.renderHeader()

	// Render tabs
	tabs := m.renderTabs()

	// Render current view
	contentHeight := m.height - 6 // Account for header, tabs, help
	var content string

	switch view := m.state.GetView(); view {
	case models.ViewOverview:
		content = m.overviewView.Render(m.width, contentHeight)
	case models.ViewRespiratory:
		// Kept bespoke: breathing is best understood as waveforms over time,
		// which a list of current values cannot show.
		content = m.respiratoryView.Render(m.width, contentHeight)
	case models.ViewAffect:
		content = m.feelingsView.Render(m.width, contentHeight)
	default:
		// Every other screen is a straight list of whatever the catalog says
		// belongs to it, so a new metric needs no code here at all.
		content = m.metricViews[view].Render(m.width, contentHeight)
	}

	// Render help
	help := m.renderHelp()

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		content,
		help,
	)
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	// Status
	status := "Alive ●"
	statusStyle := styles.StatusAliveStyle
	if !m.state.GetIsAlive() {
		status = "Dead ✗"
		statusStyle = styles.StatusDeadStyle
	}

	// Simulated time and tick count come from the sim's own clock rather than
	// the TUI's wall clock, so they stay meaningful whatever speed it runs at.
	elapsed := time.Duration(m.state.GetLatestValue(simDurationKey)) * time.Millisecond
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	timeStr := fmt.Sprintf("Sim: %02d:%02d:%02d", hours, minutes, seconds)

	tickStr := fmt.Sprintf("Tick: %d", int64(m.state.GetLatestValue(simTickCountKey)))

	// Build header
	left := styles.HeaderStyle.Render("Human Leon")
	middle := statusStyle.Render(status)
	right := styles.HeaderStyle.Render(fmt.Sprintf("%s | %s", timeStr, tickStr))

	// Calculate spacing
	usedWidth := lipgloss.Width(left) + lipgloss.Width(middle) + lipgloss.Width(right)
	spacingLeft := (m.width - usedWidth) / 2
	spacingRight := m.width - usedWidth - spacingLeft

	if spacingLeft < 0 {
		spacingLeft = 0
	}
	if spacingRight < 0 {
		spacingRight = 0
	}

	headerLine := left +
		lipgloss.NewStyle().Width(spacingLeft).Render("") +
		middle +
		lipgloss.NewStyle().Width(spacingRight).Render("") +
		right

	// Add bottom border
	borderWidth := m.width
	if borderWidth < 1 {
		borderWidth = 1
	}
	border := lipgloss.NewStyle().
		Foreground(styles.ColorBorder).
		Render(strings.Repeat("═", borderWidth))

	return headerLine + "\n" + border
}

// renderTabs renders the tab bar
func (m Model) renderTabs() string {
	currentView := m.state.GetView()

	views := models.Views

	var tabs []string
	for _, view := range views {
		if view == currentView {
			tabs = append(tabs, styles.TabActiveStyle.Render(models.ViewName(view)))
		} else {
			tabs = append(tabs, styles.TabInactiveStyle.Render(models.ViewName(view)))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return tabBar + "\n"
}

// renderHelp renders the help bar
func (m Model) renderHelp() string {
	fps := int(time.Second / m.renderInterval)
	helpText := fmt.Sprintf("Tab: Next View | Shift+Tab: Prev View | 1-6: Jump to View | +/-: FPS (%d) | q: Quit", fps)
	return styles.HelpStyle.Render(helpText)
}

// Messages

type tickMsg time.Time

// Commands

// tickEvery returns a command that sends a tick message at regular intervals
func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

package cmd

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/adamgarcia4/goLearning/cassandra/logger"
	"github.com/adamgarcia4/goLearning/cassandra/node"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start interactive node manager",
	Long: `Start an interactive terminal UI for managing nodes.

Keyboard shortcuts:
  C - Create a new node
  D - Delete a node (shows selection menu)
  DD - Delete the first active node
  Q - Quit

Examples:
  cassandra interactive`,
	Run: runInteractive,
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

// State represents the current state of the interactive UI
type State int

const (
	StateNormal State = iota
	StateDeleteSelect
	StateWaitingForSecondD
)

type model struct {
	manager      *node.Manager
	nodes        []*node.Node
	state        State
	selected     int
	err          error
	logBuffer    *logger.LogBuffer
	logScroll    int // for scrolling logs
	width        int
	height       int
	lastCommand  string // Track last command for repeat (Enter key)
	numericInput string // Buffer for multi-digit numeric input in delete mode
}

func initialModel() model {
	// Initialize logger for interactive mode (no stdout, only log buffer)
	logger.Init("", false) // No prefix, no stdout
	logBuffer := logger.GetGlobalLogBuffer()
	if err := logger.AddOutput(logger.NewLogBufferWriter(logBuffer)); err != nil {
		// Use standard log since logger might not be fully initialized
		log.Fatalf("Failed to add log buffer output: %v", err)
	}

	return model{
		manager:      node.NewManager(),
		nodes:        []*node.Node{},
		state:        StateNormal,
		selected:     0,
		logBuffer:    logBuffer,
		logScroll:    0,
		numericInput: "",
	}
}

func (m model) Init() tea.Cmd {
	// Refresh nodes list periodically
	return tea.Batch(tick(), refreshNodes(m.manager))
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

type tickMsg struct{}

func refreshNodes(manager *node.Manager) tea.Cmd {
	return func() tea.Msg {
		return nodesUpdatedMsg{nodes: manager.GetNodes()}
	}
}

type nodesUpdatedMsg struct {
	nodes []*node.Node
}

type quitMsg struct{}

type shutdownCompleteMsg struct {
	err error
}

// shutdownNodes stops all nodes and sends a message when complete
func shutdownNodes(manager *node.Manager) tea.Cmd {
	return func() tea.Msg {
		err := manager.StopAll()
		return shutdownCompleteMsg{err: err}
	}
}

// keyHandler processes a key press and returns the new state and command
type keyHandler func(*model, tea.KeyMsg) (State, tea.Cmd)

// actionResult contains the result of an action
type actionResult struct {
	state       State
	lastCommand string
	err         error
}

// Action functions

// handleCreateNode creates a new node
func handleCreateNode(m *model) actionResult {
	_, err := m.manager.CreateNode()
	if err != nil {
		return actionResult{state: m.state, err: err}
	}
	m.nodes = m.manager.GetNodes()
	return actionResult{state: m.state, lastCommand: "create"}
}

// handleDeleteNode deletes a node at the given index
func handleDeleteNode(m *model, index int) actionResult {
	if err := m.manager.DeleteNode(index); err != nil {
		return actionResult{state: m.state, err: err}
	}
	m.nodes = m.manager.GetNodes()
	return actionResult{
		state:       StateNormal,
		lastCommand: fmt.Sprintf("delete:%d", index),
	}
}

// handleEnterDeleteMode transitions to delete selection mode
func handleEnterDeleteMode(m *model) State {
	if len(m.nodes) == 0 {
		m.err = fmt.Errorf("no nodes to delete")
		return m.state
	}
	m.selected = 0
	m.numericInput = ""
	return StateDeleteSelect
}

// handleCancelDelete cancels delete mode
func handleCancelDelete(m *model) State {
	m.selected = 0
	m.numericInput = ""
	m.err = nil
	return StateNormal
}

// handleScrollLogs scrolls the log view
func handleScrollLogs(m *model, direction string) {
	allEntries := m.logBuffer.GetAll()
	maxScroll := len(allEntries) - 15
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch direction {
	case "up":
		if m.logScroll < maxScroll {
			m.logScroll++
		}
	case "down":
		if m.logScroll > 0 {
			m.logScroll--
		}
	}
}

// handleRepeatLastCommand repeats the last command
func handleRepeatLastCommand(m *model) actionResult {
	if m.lastCommand == "" {
		return actionResult{state: m.state}
	}

	if strings.HasPrefix(m.lastCommand, "delete:") {
		parts := strings.Split(m.lastCommand, ":")
		if len(parts) == 2 {
			if index, err := strconv.Atoi(parts[1]); err == nil {
				if len(m.nodes) == 0 {
					return actionResult{state: m.state, err: fmt.Errorf("no nodes to delete")}
				}
				if index >= 0 && index < len(m.nodes) {
					return handleDeleteNode(m, index)
				}
				return actionResult{state: m.state, err: fmt.Errorf("node index %d no longer exists", index+1)}
			}
		}
	} else if m.lastCommand == "create" {
		return handleCreateNode(m)
	}

	return actionResult{state: m.state}
}

// Key handler functions

// handleCreateNodeKey handles C key press
func handleCreateNodeKey(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	result := handleCreateNode(m)
	m.err = result.err
	if result.lastCommand != "" {
		m.lastCommand = result.lastCommand
	}
	return result.state, nil
}

// handleFirstD handles first D press (enters delete mode or detects DD)
func handleFirstD(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateWaitingForSecondD {
		// This is the second D - delete first node
		if len(m.nodes) > 0 {
			result := handleDeleteNode(m, 0)
			m.err = result.err
			if result.lastCommand != "" {
				m.lastCommand = result.lastCommand
			}
			return result.state, nil
		}
	}
	// First D - transition to waiting for second D
	if len(m.nodes) == 0 {
		m.err = fmt.Errorf("no nodes to delete")
		return m.state, nil
	}
	return StateWaitingForSecondD, nil
}

// handleQuit handles quit commands
func handleQuit(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	return m.state, shutdownNodes(m.manager)
}

// handleEnter handles Enter key
func handleEnter(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateDeleteSelect {
		// Handle delete confirmation
		if m.numericInput != "" {
			if num, err := strconv.Atoi(m.numericInput); err == nil {
				if num >= 1 && num <= len(m.nodes) {
					index := num - 1
					result := handleDeleteNode(m, index)
					m.err = result.err
					m.numericInput = ""
					if result.lastCommand != "" {
						m.lastCommand = result.lastCommand
					}
					return result.state, nil
				}
				m.err = fmt.Errorf("node %d does not exist (max: %d)", num, len(m.nodes))
				m.numericInput = ""
				return m.state, nil
			}
			m.err = fmt.Errorf("invalid number: %s", m.numericInput)
			m.numericInput = ""
			return m.state, nil
		}
		// Delete selected node
		result := handleDeleteNode(m, m.selected)
		m.err = result.err
		if result.lastCommand != "" {
			m.lastCommand = result.lastCommand
		}
		return result.state, nil
	}
	// In normal mode, repeat last command
	result := handleRepeatLastCommand(m)
	m.err = result.err
	if result.lastCommand != "" {
		m.lastCommand = result.lastCommand
	}
	return result.state, nil
}

// handleSpace handles Space key (same as Enter in delete mode)
func handleSpace(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateDeleteSelect {
		return handleEnter(m, msg)
	}
	return m.state, nil
}

// handleEscape handles Escape key
func handleEscape(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateDeleteSelect {
		return handleCancelDelete(m), nil
	}
	if m.state == StateWaitingForSecondD {
		return StateNormal, nil
	}
	return m.state, nil
}

// handleUp handles Up/K keys
func handleUp(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateDeleteSelect {
		if m.selected > 0 {
			m.selected--
		}
		return m.state, nil
	}
	handleScrollLogs(m, "up")
	return m.state, nil
}

// handleDown handles Down/J keys
func handleDown(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateDeleteSelect {
		if m.selected < len(m.nodes)-1 {
			m.selected++
		}
		return m.state, nil
	}
	handleScrollLogs(m, "down")
	return m.state, nil
}

// handleNumeric handles numeric input (0-9)
func handleNumeric(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	if m.state == StateDeleteSelect {
		keyStr := msg.String()
		m.numericInput += keyStr
		if m.err != nil && strings.Contains(m.err.Error(), "does not exist") {
			m.err = nil
		}
		return m.state, nil
	}
	return m.state, nil
}

// handleOtherKey handles any other key press
func handleOtherKey(m *model, msg tea.KeyMsg) (State, tea.Cmd) {
	// If waiting for second D and got another key, enter delete mode
	if m.state == StateWaitingForSecondD {
		return handleEnterDeleteMode(m), nil
	}
	if m.state == StateDeleteSelect {
		// Clear numeric input on non-numeric keys
		m.numericInput = ""
	}
	return m.state, nil
}

// keyHandlers maps states to their key bindings
var keyHandlers = map[State]map[string]keyHandler{
	StateNormal: {
		"c":      handleCreateNodeKey,
		"C":      handleCreateNodeKey,
		"d":      handleFirstD,
		"D":      handleFirstD,
		"q":      handleQuit,
		"ctrl+c": handleQuit,
		"enter":  handleEnter,
		"up":     handleUp,
		"k":      handleUp,
		"down":   handleDown,
		"j":      handleDown,
	},
	StateWaitingForSecondD: {
		"d":     handleFirstD,
		"D":     handleFirstD,
		"esc":   handleEscape,
		"enter": handleEscape, // Reset on Enter if not second D
	},
	StateDeleteSelect: {
		"esc":   handleEscape,
		"enter": handleEnter,
		" ":     handleSpace,
		"up":    handleUp,
		"k":     handleUp,
		"down":  handleDown,
		"j":     handleDown,
		"0":     handleNumeric,
		"1":     handleNumeric,
		"2":     handleNumeric,
		"3":     handleNumeric,
		"4":     handleNumeric,
		"5":     handleNumeric,
		"6":     handleNumeric,
		"7":     handleNumeric,
		"8":     handleNumeric,
		"9":     handleNumeric,
	},
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Get the handler map for current state
		stateHandlers, ok := keyHandlers[m.state]
		if !ok {
			// Unknown state, treat as normal state
			stateHandlers = keyHandlers[StateNormal]
		}

		keyStr := msg.String()
		handler, found := stateHandlers[keyStr]
		if found {
			// Found specific handler for this key
			newState, cmd := handler(&m, msg)
			m.state = newState
			return m, cmd
		}

		// No specific handler found, use default handler
		newState, cmd := handleOtherKey(&m, msg)
		m.state = newState
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Refresh nodes list
		// If waiting for second D, timeout and enter delete mode
		if m.state == StateWaitingForSecondD {
			m.state = handleEnterDeleteMode(&m)
		}
		return m, tea.Batch(tick(), refreshNodes(m.manager))

	case nodesUpdatedMsg:
		m.nodes = msg.nodes
		return m, nil

	case shutdownCompleteMsg:
		// Log any shutdown errors via the logger
		if msg.err != nil {
			logger.Printf("Error stopping nodes during shutdown: %v", msg.err)
		}
		// Now quit after shutdown is complete
		return m, tea.Quit

	case quitMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(1, 2)
	s.WriteString(titleStyle.Render("Cassandra Node Manager"))
	s.WriteString("\n\n")

	// Status
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n\n")
	}

	// Nodes list
	if len(m.nodes) == 0 {
		s.WriteString("No nodes running.\n\n")
	} else {
		s.WriteString("Running Nodes:\n\n")
		for i, n := range m.nodes {
			config := n.GetConfig()
			if m.state == StateDeleteSelect && i == m.selected {
				// Highlight selected node in delete mode
				nodeStyle := lipgloss.NewStyle().
					PaddingLeft(2).
					Foreground(lipgloss.Color("196")).
					Bold(true)
				s.WriteString(nodeStyle.Render(fmt.Sprintf("[%d] > %s (port: %s)", i+1, config.NodeID, config.Port)))
				s.WriteString("\n")
			} else {
				s.WriteString(fmt.Sprintf("  [%d]   %s (port: %s)\n", i+1, config.NodeID, config.Port))
			}
		}
		s.WriteString("\n")
	}

	// Logs section - single unified box
	s.WriteString("\n")

	// Get all log entries once to avoid redundant buffer access
	allEntries := m.logBuffer.GetAll()
	totalCount := len(allEntries)

	// Get recent logs (show last 15 entries, adjusted by scroll)
	logCount := 15
	maxScroll := 100 // Maximum scroll back

	// Calculate how many entries we need to fetch
	// We need logCount entries to display, plus logScroll to scroll back
	entriesNeeded := logCount + m.logScroll
	if entriesNeeded > maxScroll+logCount {
		entriesNeeded = maxScroll + logCount
	}

	var logLines []string
	if totalCount == 0 {
		logLines = []string{"     | (no logs yet)"}
	} else {
		// Derive recent entries from allEntries (take last entriesNeeded entries)
		// If entriesNeeded > totalCount, we'll use all entries
		recentStart := totalCount - entriesNeeded
		if recentStart < 0 {
			recentStart = 0
		}
		logEntries := allEntries[recentStart:]

		// Calculate the range to display from logEntries
		// logScroll=0 means show most recent logCount entries
		// logScroll=1 means show entries starting 1 position back, etc.
		start := len(logEntries) - logCount - m.logScroll
		if start < 0 {
			start = 0
		}
		end := len(logEntries) - m.logScroll
		if end > len(logEntries) {
			end = len(logEntries)
		}
		if end <= start {
			end = start + logCount
			if end > len(logEntries) {
				end = len(logEntries)
				start = end - logCount
				if start < 0 {
					start = 0
				}
			}
		}

		// Show entries in reverse order (newest first) with line numbers
		// Most recent = 0, older entries count up
		// Line number is based on position in full buffer, not display position
		// logEntries[i] corresponds to allEntries[recentStart + i]
		// Position in full buffer = recentStart + i
		// Line number: most recent (position totalCount-1) = 0
		// So line number = totalCount - 1 - (recentStart + i)
		for i := end - 1; i >= start; i-- {
			// Calculate line number based on position in full buffer
			positionInFullBuffer := recentStart + i
			// Line number: most recent (position totalCount-1) = 0
			// So line number = totalCount - 1 - positionInFullBuffer
			lineNumber := totalCount - 1 - positionInFullBuffer
			if lineNumber < 0 {
				lineNumber = 0
			}

			// Format with line number (right-aligned, 4 digits)
			lineNum := fmt.Sprintf("%4d", lineNumber)
			logLines = append(logLines, fmt.Sprintf("%s | %s", lineNum, logger.FormatLogEntry(logEntries[i])))
		}
	}

	// Create a single log box with title - use terminal width if available, otherwise default
	boxWidth := 100
	if m.width > 0 {
		boxWidth = m.width - 4 // Leave some margin
	}

	// Combine title and content
	logContent := "Logs:\n" + strings.Join(logLines, "\n")

	logStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Height(13).
		Width(boxWidth)

	s.WriteString(logStyle.Render(logContent))
	s.WriteString("\n\n")

	// Instructions
	instructionsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		PaddingTop(1)

	if m.state == StateDeleteSelect {
		var helpText string
		if m.numericInput != "" {
			// Build fully formatted string when numeric input is present
			helpText = fmt.Sprintf("DELETE MODE: Type node number (current: %s) or Enter to confirm, Esc to cancel", m.numericInput)
		} else {
			// Format string with node count when no numeric input
			helpText = fmt.Sprintf("DELETE MODE: Use ↑/↓/j/k or type node number (1-%d, multi-digit supported), Enter to confirm, Esc to cancel", len(m.nodes))
		}
		s.WriteString(instructionsStyle.Render(helpText))
	} else {
		instructionText := "Press C to create a node | D to delete a node | DD to delete first node"

		// Add inline preview if there's a last command
		if m.lastCommand != "" {
			previewText := formatCommandPreview(m.lastCommand)
			instructionText += fmt.Sprintf(" | Enter to repeat (%s)", previewText)
		} else {
			instructionText += " | Enter to repeat last command"
		}

		instructionText += " | ↑/↓/j/k to scroll logs | Q to quit"
		s.WriteString(instructionsStyle.Render(instructionText))
	}

	return s.String()
}

// formatCommandPreview formats the last command for display
func formatCommandPreview(lastCommand string) string {
	if strings.HasPrefix(lastCommand, "delete:") {
		// Parse "delete:0" format
		parts := strings.Split(lastCommand, ":")
		if len(parts) == 2 {
			if index, err := strconv.Atoi(parts[1]); err == nil {
				// Show as multi-step: D → 1 (where 1 is index+1)
				return fmt.Sprintf("D → %d", index+1)
			}
		}
		return "D → [node]"
	} else if lastCommand == "create" {
		return "C"
	}
	return lastCommand
}

func runInteractive(cmd *cobra.Command, args []string) {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running interactive mode: %v\n", err)
	}
}

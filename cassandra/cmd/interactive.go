package cmd

import (
	"fmt"
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
  Q - Quit

Examples:
  cassandra interactive`,
	Run: runInteractive,
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

type model struct {
	manager      *node.Manager
	nodes        []*node.Node
	deleteMode   bool
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
	logBuffer := logger.GetGlobalLogBuffer()
	logger.Init("", false) // No prefix, no stdout
	logger.AddOutput(logger.NewLogBufferWriter(logBuffer))

	return model{
		manager:      node.NewManager(),
		nodes:        []*node.Node{},
		deleteMode:   false,
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			// Stop all nodes gracefully and wait for completion
			return m, shutdownNodes(m.manager)
		}

		// Handle delete mode
		if m.deleteMode {
			return m.handleDeleteMode(msg)
		}

		// Handle normal mode
		switch msg.String() {
		case "c", "C":
			// Create new node
			_, err := m.manager.CreateNode()
			if err != nil {
				m.err = err
			} else {
				m.err = nil
				m.nodes = m.manager.GetNodes()
				m.lastCommand = "create" // Remember this command
			}
			return m, nil

		case "d", "D":
			// Enter delete mode
			if len(m.nodes) == 0 {
				m.err = fmt.Errorf("no nodes to delete")
				return m, nil
			}
			m.deleteMode = true
			m.selected = 0
			m.numericInput = "" // Reset numeric input buffer
			// Don't set lastCommand yet - wait to see if a number follows
			return m, nil

		case "enter":
			// Repeat last command/sequence
			if m.lastCommand == "" {
				// No previous command, do nothing
				return m, nil
			}

			// Check if it's a delete command with index
			if strings.HasPrefix(m.lastCommand, "delete:") {
				// Parse the index from "delete:0" format
				parts := strings.Split(m.lastCommand, ":")
				if len(parts) == 2 {
					if index, err := strconv.Atoi(parts[1]); err == nil {
						// Replay the sequence: enter delete mode and delete at that index
						if len(m.nodes) == 0 {
							m.err = fmt.Errorf("no nodes to delete")
							return m, nil
						}
						// Check if index is still valid
						if index >= 0 && index < len(m.nodes) {
							if err := m.manager.DeleteNode(index); err != nil {
								m.err = err
							} else {
								m.nodes = m.manager.GetNodes()
								m.err = nil
							}
						} else {
							m.err = fmt.Errorf("node index %d no longer exists", index+1)
						}
						return m, nil
					}
				}
			} else if m.lastCommand == "create" {
				// Repeat create node
				_, err := m.manager.CreateNode()
				if err != nil {
					m.err = err
				} else {
					m.err = nil
					m.nodes = m.manager.GetNodes()
				}
				return m, nil
			}
			return m, nil

		case "esc":
			// Exit delete mode
			m.deleteMode = false
			m.selected = 0
			m.err = nil
			return m, nil

		case "up", "k":
			// Scroll logs up (show older logs)
			// Check how many total entries we have to determine max scroll
			allEntries := m.logBuffer.GetAll()
			maxScroll := len(allEntries) - 15 // Can scroll back until we have 15 entries left
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.logScroll < maxScroll {
				m.logScroll++
			}
			return m, nil

		case "down", "j":
			// Scroll logs down (show newer logs)
			if m.logScroll > 0 {
				m.logScroll--
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Refresh nodes list
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

func (m model) handleDeleteMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.deleteMode = false
			m.selected = 0
			m.err = nil
			m.numericInput = "" // Clear numeric input buffer
			return m, nil

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil

		case "down", "j":
			if m.selected < len(m.nodes)-1 {
				m.selected++
			}
			return m, nil

		case "enter", " ":
			// If there's numeric input, process that first
			if m.numericInput != "" {
				// Parse the entire input string as an integer
				if num, err := strconv.Atoi(m.numericInput); err == nil {
					// Validate: 1 <= num <= len(m.nodes)
					if num >= 1 && num <= len(m.nodes) {
						index := num - 1 // Convert to 0-based index
						// Store the sequence: delete mode + delete at this index
						m.lastCommand = fmt.Sprintf("delete:%d", index)
						if err := m.manager.DeleteNode(index); err != nil {
							m.err = err
						} else {
							m.nodes = m.manager.GetNodes()
							m.deleteMode = false
							m.selected = 0
							m.err = nil
						}
						m.numericInput = ""
						return m, nil
					} else {
						m.err = fmt.Errorf("node %d does not exist (max: %d)", num, len(m.nodes))
						m.numericInput = ""
						return m, nil
					}
				} else {
					m.err = fmt.Errorf("invalid number: %s", m.numericInput)
					m.numericInput = ""
					return m, nil
				}
			}
			// Otherwise, delete selected node
			selectedIndex := m.selected
			if err := m.manager.DeleteNode(selectedIndex); err != nil {
				m.err = err
			} else {
				m.nodes = m.manager.GetNodes()
				m.deleteMode = false
				m.selected = 0
				m.err = nil
				// Remember the sequence: delete mode + delete at this index
				m.lastCommand = fmt.Sprintf("delete:%d", selectedIndex)
			}
			return m, nil

		default:
			// Handle numeric input (supports multi-digit numbers)
			keyStr := msg.String()

			// Check if it's a digit (0-9)
			if len(keyStr) == 1 && keyStr >= "0" && keyStr <= "9" {
				// Append to numeric input buffer
				m.numericInput += keyStr
				// Clear any previous error when typing
				if m.err != nil && strings.Contains(m.err.Error(), "does not exist") {
					m.err = nil
				}
				return m, nil
			}

			// Non-numeric key, clear the buffer
			if m.numericInput != "" {
				m.numericInput = ""
			}
			return m, nil
		}
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
			if m.deleteMode && i == m.selected {
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

	if m.deleteMode {
		helpText := "DELETE MODE: Use ↑/↓/j/k or type node number (1-%d, multi-digit supported), Enter to confirm, Esc to cancel"
		if m.numericInput != "" {
			helpText = fmt.Sprintf("DELETE MODE: Type node number (current: %s) or Enter to confirm, Esc to cancel", m.numericInput)
		}
		s.WriteString(instructionsStyle.Render(fmt.Sprintf(helpText, len(m.nodes))))
	} else {
		instructionText := "Press C to create a node | D to delete a node"

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

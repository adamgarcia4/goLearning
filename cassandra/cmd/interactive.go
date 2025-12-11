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
	manager    *node.Manager
	nodes      []*node.Node
	deleteMode bool
	selected   int
	err        error
	logBuffer  *logger.LogBuffer
	logScroll  int // for scrolling logs
	width      int
	height     int
}

func initialModel() model {
	// Initialize logger for interactive mode (no stdout, only log buffer)
	logBuffer := logger.GetGlobalLogBuffer()
	logger.Init("", false) // No prefix, no stdout
	logger.AddOutput(logger.NewLogBufferWriter(logBuffer))
	
	return model{
		manager:    node.NewManager(),
		nodes:      []*node.Node{},
		deleteMode: false,
		selected:   0,
		logBuffer:  logBuffer,
		logScroll:  0,
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			// Stop all nodes before quitting
			m.manager.StopAll()
			return m, tea.Quit
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
			return m, nil

		case "esc":
			// Exit delete mode
			m.deleteMode = false
			m.selected = 0
			m.err = nil
			return m, nil

		case "up", "k":
			// Scroll logs up (show older logs)
			if m.logScroll < 100 { // limit scroll
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
			// Delete selected node
			if err := m.manager.DeleteNode(m.selected); err != nil {
				m.err = err
			} else {
				m.nodes = m.manager.GetNodes()
				m.deleteMode = false
				m.selected = 0
				m.err = nil
			}
			return m, nil

		default:
			// Try to parse as number (1-9)
			if num, err := strconv.Atoi(msg.String()); err == nil {
				index := num - 1 // Convert to 0-based index
				if index >= 0 && index < len(m.nodes) {
					m.selected = index
					if err := m.manager.DeleteNode(index); err != nil {
						m.err = err
					} else {
						m.nodes = m.manager.GetNodes()
						m.deleteMode = false
						m.selected = 0
						m.err = nil
					}
				}
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

	// Get recent logs (show last 15 entries, adjusted by scroll)
	logCount := 15
	logEntries := m.logBuffer.GetRecent(logCount + m.logScroll)

	var logLines []string
	if len(logEntries) == 0 {
		logLines = []string{"(no logs yet)"}
	} else {
		// Show the most recent entries first (reverse order)
		start := len(logEntries) - logCount
		if start < 0 {
			start = 0
		}
		for i := len(logEntries) - 1; i >= start; i-- {
			logLines = append(logLines, logger.FormatLogEntry(logEntries[i]))
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
		s.WriteString(instructionsStyle.Render("DELETE MODE: Use ↑/↓ or press 1-9 to select node, Enter/Space to delete, Esc to cancel"))
	} else {
		s.WriteString(instructionsStyle.Render("Press C to create a node | D to delete a node | ↑/↓ to scroll logs | Q to quit"))
	}

	return s.String()
}

func runInteractive(cmd *cobra.Command, args []string) {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running interactive mode: %v\n", err)
	}
}

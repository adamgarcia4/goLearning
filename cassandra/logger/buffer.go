package logger

import (
	"fmt"
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	NodeID    string
	Message   string
}

// LogBuffer is a thread-safe buffer for log entries
type LogBuffer struct {
	entries []LogEntry
	maxSize int
	mu      sync.RWMutex
}

// NewLogBuffer creates a new log buffer
func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a new log entry
func (lb *LogBuffer) Add(nodeID, message string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Message:   message,
	}

	lb.entries = append(lb.entries, entry)

	// Keep only the last maxSize entries
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[len(lb.entries)-lb.maxSize:]
	}
}

// GetRecent returns the most recent log entries
func (lb *LogBuffer) GetRecent(count int) []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if count > len(lb.entries) {
		count = len(lb.entries)
	}

	start := len(lb.entries) - count
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, count)
	copy(result, lb.entries[start:])
	return result
}

// GetAll returns all log entries
func (lb *LogBuffer) GetAll() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// Clear removes all log entries from the buffer
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.entries = make([]LogEntry, 0, lb.maxSize)
}

// FormatLogEntry formats a log entry for display
func FormatLogEntry(entry LogEntry) string {
	return fmt.Sprintf("[%s] %s: %s",
		entry.Timestamp.Format("15:04:05"),
		entry.NodeID,
		entry.Message,
	)
}


package logger

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"sync"
)

// LogBufferWriter is an io.Writer that writes to the log buffer
// It extracts node ID from log messages in the format "[nodeID] message"
type LogBufferWriter struct {
	buffer *LogBuffer
	buf    bytes.Buffer
	mu     sync.Mutex
}

var nodeIDRegex = regexp.MustCompile(`^\[([^\]]+)\]\s*(.*)$`)

// NewLogBufferWriter creates a new writer that writes to the log buffer
func NewLogBufferWriter(buffer *LogBuffer) *LogBufferWriter {
	return &LogBufferWriter{
		buffer: buffer,
	}
}

// Write implements io.Writer
func (lw *LogBufferWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	// Buffer until we get a newline
	lw.buf.Write(p)

	// Process complete lines
	for {
		line, err := lw.buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return len(p), err
		}

		// Remove newline
		line = strings.TrimSuffix(line, "\n")
		if len(line) == 0 {
			continue
		}

		// Try to extract node ID from format "[nodeID] message"
		nodeID := "system"
		message := line

		matches := nodeIDRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			nodeID = matches[1]
			message = matches[2]
		}

		// Add to log buffer
		lw.buffer.Add(nodeID, message)
	}

	return len(p), nil
}


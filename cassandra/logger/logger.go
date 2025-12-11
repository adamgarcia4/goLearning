package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Logger is a configurable logger that can write to multiple outputs
type Logger struct {
	mu       sync.Mutex
	outputs  []io.Writer
	prefix   string
	enabled  bool
}

var (
	globalLogger *Logger
	once         sync.Once
	globalBuffer *LogBuffer
	bufferOnce   sync.Once
)

// GetGlobalLogBuffer returns the global log buffer
func GetGlobalLogBuffer() *LogBuffer {
	bufferOnce.Do(func() {
		globalBuffer = NewLogBuffer(1000) // Keep last 1000 log entries
	})
	return globalBuffer
}

// Init initializes the global logger
func Init(prefix string, writeToStdout bool) {
	once.Do(func() {
		outputs := []io.Writer{}
		if writeToStdout {
			outputs = append(outputs, os.Stdout)
		}
		globalLogger = &Logger{
			outputs: outputs,
			prefix:  prefix,
			enabled: true,
		}
	})
}

// AddOutput adds an additional output writer (e.g., for TUI log buffer)
func AddOutput(w io.Writer) {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()
		globalLogger.outputs = append(globalLogger.outputs, w)
	}
}

// RemoveOutput removes an output writer
func RemoveOutput(w io.Writer) {
	if globalLogger == nil {
		return
	}
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	
	newOutputs := []io.Writer{}
	for _, output := range globalLogger.outputs {
		if output != w {
			newOutputs = append(newOutputs, output)
		}
	}
	globalLogger.outputs = newOutputs
}

// SetEnabled enables or disables logging
func SetEnabled(enabled bool) {
	if globalLogger != nil {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()
		globalLogger.enabled = enabled
	}
}

// Printf logs a formatted message
func Printf(format string, v ...interface{}) {
	if globalLogger == nil {
		// Fallback to standard log if not initialized
		log.Printf(format, v...)
		return
	}
	
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	
	if !globalLogger.enabled {
		return
	}
	
	msg := fmt.Sprintf(format, v...)
	// Remove trailing newline if present (we'll add it back)
	msg = strings.TrimSuffix(msg, "\n")
	
	// Add prefix if specified
	if globalLogger.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", globalLogger.prefix, msg)
	}
	
	// Write to all outputs
	if len(globalLogger.outputs) > 0 {
		msgWithNewline := msg + "\n"
		for _, output := range globalLogger.outputs {
			output.Write([]byte(msgWithNewline))
		}
	}
}

// Print logs a message
func Print(v ...interface{}) {
	Printf("%s", fmt.Sprint(v...))
}

// Println logs a message with newline
func Println(v ...interface{}) {
	Printf("%s", fmt.Sprintln(v...))
}

// GetGlobalLogger returns the global logger instance (for testing/debugging)
func GetGlobalLogger() *Logger {
	return globalLogger
}


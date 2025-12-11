// Package logger provides a configurable logger that can write to multiple outputs.
// Init must be called early in the application lifecycle before using other logger functions.
// Functions like AddOutput and SetEnabled will return errors if called before Init.
package logger

import (
	"errors"
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

// AddOutput adds an additional output writer (e.g., for TUI log buffer).
// Returns an error if called before Init.
func AddOutput(w io.Writer) error {
	if globalLogger == nil {
		return errors.New("logger not initialized: call logger.Init() first")
	}
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	globalLogger.outputs = append(globalLogger.outputs, w)
	return nil
}

// RemoveOutput removes an output writer.
// Returns an error if called before Init.
func RemoveOutput(w io.Writer) error {
	if globalLogger == nil {
		return errors.New("logger not initialized: call logger.Init() first")
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
	return nil
}

// SetEnabled enables or disables logging.
// Returns an error if called before Init.
func SetEnabled(enabled bool) error {
	if globalLogger == nil {
		return errors.New("logger not initialized: call logger.Init() first")
	}
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	globalLogger.enabled = enabled
	return nil
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

// Infof logs an info-level formatted message
func Infof(format string, v ...interface{}) {
	Printf("[INFO] "+format, v...)
}

// Info logs an info-level message
func Info(v ...interface{}) {
	Printf("[INFO] %s", fmt.Sprint(v...))
}

// Errorf logs an error-level formatted message
func Errorf(format string, v ...interface{}) {
	Printf("[ERROR] "+format, v...)
}

// Error logs an error-level message
func Error(v ...interface{}) {
	Printf("[ERROR] %s", fmt.Sprint(v...))
}

// GetGlobalLogger returns the global logger instance (for testing/debugging)
func GetGlobalLogger() *Logger {
	return globalLogger
}


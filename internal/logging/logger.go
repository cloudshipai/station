package logging

import (
	"io"
	"log"
	"os"
)

// Logger provides level-based logging functionality
type Logger struct {
	debugEnabled bool
	infoLogger   *log.Logger
	debugLogger  *log.Logger
}

// Global logger instance
var globalLogger *Logger

// Initialize sets up the global logger with debug mode setting
// All logging goes to stderr to avoid polluting stdout (important for MCP servers)
func Initialize(debugMode bool) {
	// Always use stderr for logging to avoid interfering with MCP stdio protocol
	var output io.Writer = os.Stderr

	globalLogger = &Logger{
		debugEnabled: debugMode,
		infoLogger:   log.New(output, "", log.LstdFlags),
		debugLogger:  log.New(output, "", log.LstdFlags),
	}
}

// Info logs informational messages (always shown)
func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.infoLogger.Printf(format, args...)
	}
}

// Debug logs debug messages (only shown when debug mode is enabled)
func Debug(format string, args ...interface{}) {
	if globalLogger != nil && globalLogger.debugEnabled {
		globalLogger.debugLogger.Printf("DEBUG: "+format, args...)
	}
}

// Error logs error messages (always shown)
func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.infoLogger.Printf("ERROR: "+format, args...)
	}
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return globalLogger != nil && globalLogger.debugEnabled
}

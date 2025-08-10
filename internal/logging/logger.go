package logging

import (
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
func Initialize(debugMode bool) {
	globalLogger = &Logger{
		debugEnabled: debugMode,
		infoLogger:   log.New(os.Stdout, "", log.LstdFlags),
		debugLogger:  log.New(os.Stdout, "", log.LstdFlags),
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

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return globalLogger != nil && globalLogger.debugEnabled
}
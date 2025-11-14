package telemetry

import (
	"fmt"
	"os"
	"time"
)

// Logger handles debug logging for faker
type Logger struct {
	file  *os.File
	debug bool
}

// NewLogger creates a new logger
func NewLogger(logPath string, debug bool) (*Logger, error) {
	if !debug {
		return &Logger{debug: false}, nil
	}

	// Create log file with absolute path
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Log error to stderr but don't fail - logging is not critical
		fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to open log file at %s: %v\n", logPath, err)
		return &Logger{debug: true}, nil
	}

	fmt.Fprintf(file, "\n\n=== New Faker Session Started at %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "[FAKER] Log file initialized successfully\n")
	file.Sync()

	return &Logger{
		file:  file,
		debug: debug,
	}, nil
}

// Log writes a log message to stderr and optionally to file
func (l *Logger) Log(format string, args ...interface{}) {
	if !l.debug {
		return
	}

	msg := fmt.Sprintf(format, args...)

	// Always log to stderr
	fmt.Fprint(os.Stderr, msg)

	// Also log to file if available
	if l.file != nil {
		fmt.Fprint(l.file, msg)
		l.file.Sync() // Flush immediately so we don't lose logs if process hangs
	}
}

// Close closes the logger and flushes any remaining data
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

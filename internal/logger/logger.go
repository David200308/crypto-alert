package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	defaultLogger *Logger
	once          sync.Once
)

// Logger wraps the standard log.Logger with date-based file rotation
type Logger struct {
	logDir      string
	currentDate string
	logFile     *os.File
	logger      *log.Logger
	mu          sync.Mutex
}

// InitLogger initializes the default logger with the specified log directory
func InitLogger(logDir string) error {
	var err error
	once.Do(func() {
		defaultLogger, err = NewLogger(logDir)
		if err != nil {
			return
		}
		// Replace standard log output
		log.SetOutput(defaultLogger)
		log.SetFlags(log.LstdFlags)
	})
	return err
}

// NewLogger creates a new logger instance with date-based file rotation
func NewLogger(logDir string) (*Logger, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	l := &Logger{
		logDir: logDir,
	}

	// Initialize the log file for today
	if err := l.rotateIfNeeded(); err != nil {
		return nil, err
	}

	return l, nil
}

// rotateIfNeeded checks if we need to rotate to a new log file based on the date
func (l *Logger) rotateIfNeeded() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	today := time.Now().Format("20060102")

	// If date hasn't changed, no need to rotate
	if l.currentDate == today && l.logFile != nil {
		return nil
	}

	// Close existing log file if open
	if l.logFile != nil {
		l.logFile.Close()
	}

	// Open new log file for today
	logFileName := filepath.Join(l.logDir, fmt.Sprintf("%s.log", today))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.logFile = logFile
	l.currentDate = today
	l.logger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.LstdFlags)

	return nil
}

// Write implements io.Writer interface
func (l *Logger) Write(p []byte) (n int, err error) {
	// Check if we need to rotate (date changed)
	if err := l.rotateIfNeeded(); err != nil {
		// If rotation fails, still write to stdout
		return os.Stdout.Write(p)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Write to stdout first
	n1, err1 := os.Stdout.Write(p)
	if err1 != nil {
		return n1, err1
	}

	// Write to file
	if l.logFile != nil {
		_, err2 := l.logFile.Write(p)
		if err2 != nil {
			return n1, err2
		}
		return n1, nil
	}

	return n1, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// GetLogger returns the default logger instance
func GetLogger() *Logger {
	return defaultLogger
}

// Printf logs a formatted message
func Printf(format string, v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.rotateIfNeeded()
		defaultLogger.mu.Lock()
		defer defaultLogger.mu.Unlock()
		if defaultLogger.logger != nil {
			defaultLogger.logger.Printf(format, v...)
		}
	} else {
		log.Printf(format, v...)
	}
}

// Println logs a message with a newline
func Println(v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.rotateIfNeeded()
		defaultLogger.mu.Lock()
		defer defaultLogger.mu.Unlock()
		if defaultLogger.logger != nil {
			defaultLogger.logger.Println(v...)
		}
	} else {
		log.Println(v...)
	}
}

// Fatalf logs a fatal error and exits
func Fatalf(format string, v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.rotateIfNeeded()
		defaultLogger.mu.Lock()
		defer defaultLogger.mu.Unlock()
		if defaultLogger.logger != nil {
			defaultLogger.logger.Fatalf(format, v...)
		}
	} else {
		log.Fatalf(format, v...)
	}
}

// Fatal logs a fatal error and exits
func Fatal(v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.rotateIfNeeded()
		defaultLogger.mu.Lock()
		defer defaultLogger.mu.Unlock()
		if defaultLogger.logger != nil {
			defaultLogger.logger.Fatal(v...)
		}
	} else {
		log.Fatal(v...)
	}
}

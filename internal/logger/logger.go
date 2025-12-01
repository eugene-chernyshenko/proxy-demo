package logger

import (
	"log"
	"os"
)

type Level int

const (
	LevelError Level = iota
	LevelInfo
	LevelDebug
)

var (
	currentLevel = LevelInfo
	logger       = log.New(os.Stderr, "", log.LstdFlags)
)

// SetLevel sets the logging level
func SetLevel(level Level) {
	currentLevel = level
}

// Debug logs a debug message with component prefix
func Debug(component, format string, v ...interface{}) {
	if currentLevel >= LevelDebug {
		logger.Printf("[DEBUG] [%s] "+format, append([]interface{}{component}, v...)...)
	}
}

// Info logs an info message with component prefix
func Info(component, format string, v ...interface{}) {
	if currentLevel >= LevelInfo {
		logger.Printf("[INFO] [%s] "+format, append([]interface{}{component}, v...)...)
	}
}

// Error logs an error message with component prefix
func Error(component, format string, v ...interface{}) {
	if currentLevel >= LevelError {
		logger.Printf("[ERROR] [%s] "+format, append([]interface{}{component}, v...)...)
	}
}


package logger

import (
	"fmt"
	"log"
	"skyport-agent/internal/config"
)

// Debug logs debug messages only when debug mode is enabled
func Debug(format string, args ...interface{}) {
	if config.IsDebugMode() {
		message := fmt.Sprintf(format, args...)
		log.Printf("[DEBUG] %s", message)
	}
}

// Info logs informational messages (always shown)
func Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("✓ %s\n", message)
}

// Warning logs warning messages (always shown)
func Warning(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("⚠ %s\n", message)
}

// Error logs error messages (always shown)
func Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("✗ %s\n", message)
}

// Success logs success messages (always shown)
func Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("✓ %s\n", message)
}

// Plain prints a plain message without any prefix (always shown)
func Plain(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// ErrorWithDetails logs an error with detailed information in debug mode
func ErrorWithDetails(msg string, err error) {
	Error("%s", msg)
	if config.IsDebugMode() && err != nil {
		log.Printf("[DEBUG] Error details: %v", err)
	}
}

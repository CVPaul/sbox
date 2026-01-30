// Package console provides colored console output utilities.
package console

import (
	"fmt"
	"os"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

// Info prints an info message
func Info(format string, args ...interface{}) {
	fmt.Printf(colorBlue+"[INFO]"+colorReset+" "+format+"\n", args...)
}

// Success prints a success message
func Success(format string, args ...interface{}) {
	fmt.Printf(colorGreen+"[OK]"+colorReset+" "+format+"\n", args...)
}

// Warning prints a warning message
func Warning(format string, args ...interface{}) {
	fmt.Printf(colorYellow+"[WARN]"+colorReset+" "+format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorRed+"[ERROR]"+colorReset+" "+format+"\n", args...)
}

// Step prints a step message
func Step(format string, args ...interface{}) {
	fmt.Printf(colorCyan+"[STEP]"+colorReset+" "+format+"\n", args...)
}

// Print prints a plain message
func Print(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Fatal prints an error message and exits
func Fatal(format string, args ...interface{}) {
	Error(format, args...)
	os.Exit(1)
}

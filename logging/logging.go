package logging

import "log"

var Verbose bool

// SetVerbose sets the verbose logging mode
func SetVerbose(verbose bool) {
	Verbose = verbose
}

// Debug logs debug messages (only in verbose mode)
func Debug(format string, v ...interface{}) {
	if Verbose {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs informational messages (always shown)
func Info(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}

// Warn logs warning messages (always shown)
func Warn(format string, v ...interface{}) {
	log.Printf("[WARN] "+format, v...)
}

// Error logs error messages (always shown)
func Error(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}

// LogV is deprecated, use Debug instead
// Kept for backward compatibility during migration
func LogV(format string, v ...interface{}) {
	if Verbose {
		log.Printf(format, v...)
	}
}

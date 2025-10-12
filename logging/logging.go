package logging

import (
	"log"
	"strings"
)

var Verbose bool
var verboseFilters map[string]bool
var verboseAll bool

// SetVerbose sets the verbose logging mode with granular filtering
// Examples:
//   - "" or "false": disable all verbose logging
//   - "true": enable all verbose logging
//   - "config,health": enable verbose for config and health modules
//   - "broadcaster.addEventToCache,main": enable broadcaster.addEventToCache method and all of main module
func SetVerbose(verboseStr string) {
	verboseFilters = make(map[string]bool)
	verboseAll = false
	Verbose = false

	if verboseStr == "" || verboseStr == "false" {
		return
	}

	if verboseStr == "true" {
		Verbose = true
		verboseAll = true
		return
	}

	// Parse comma-separated filters
	filters := strings.Split(verboseStr, ",")
	for _, filter := range filters {
		filter = strings.TrimSpace(filter)
		if filter != "" {
			verboseFilters[filter] = true
			Verbose = true // At least one filter is enabled
		}
	}
}

// IsVerbose checks if verbose logging is enabled for a specific module or method
func IsVerbose(module string, method string) bool {
	if !Verbose {
		return false
	}

	if verboseAll {
		return true
	}

	// Check if module.method is enabled
	if method != "" {
		fullName := module + "." + method
		if verboseFilters[fullName] {
			return true
		}
	}

	// Check if module is enabled (all methods)
	if verboseFilters[module] {
		return true
	}

	return false
}

// DebugMethod logs debug messages for a specific module.method (only in verbose mode)
func DebugMethod(module string, method string, format string, v ...interface{}) {
	if IsVerbose(module, method) {
		log.Printf("[DEBUG] "+module+"."+method+": "+format, v...)
	}
}

// Debug logs debug messages (deprecated - only works with --verbose true)
// Use DebugMethod instead for better granular control
func Debug(format string, v ...interface{}) {
	if verboseAll {
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
	if verboseAll {
		log.Printf(format, v...)
	}
}

package logging

import "log"

var Verbose bool

// LogV logs only when verbose mode is enabled
func LogV(format string, v ...interface{}) {
	if Verbose {
		log.Printf(format, v...)
	}
}

// SetVerbose sets the verbose logging mode
func SetVerbose(verbose bool) {
	Verbose = verbose
}

package utils

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	debugLog  *log.Logger
	verbose   bool
	logSinkMu sync.RWMutex
	logSink   func(string)
)

func EnableDebug() {
	verbose = true
	debugLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func Debugf(format string, args ...interface{}) {
	if verbose {
		message := fmt.Sprintf(format, args...)
		debugLog.Output(2, message)

		logSinkMu.RLock()
		sink := logSink
		logSinkMu.RUnlock()
		if sink != nil {
			sink(message)
		}
	}
}

// SetLogSink mirrors debug messages to an embedding application.
func SetLogSink(sink func(string)) {
	logSinkMu.Lock()
	logSink = sink
	logSinkMu.Unlock()
}

func IsVerbose() bool {
	return verbose
}

// SetDebug toggles verbose logging at runtime (off = Debugf becomes a no-op).
func SetDebug(on bool) {
	if on {
		EnableDebug()
		return
	}
	verbose = false
}

// SafeGo runs fn in a new goroutine, recovering from any panic so a crash in
// one worker cannot take down the whole process (critical when this code runs
// embedded as a library inside a mobile app).
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				Debugf("[PANIC] recovered in %s: %v", name, r)
			}
		}()
		fn()
	}()
}

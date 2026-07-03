package lookupdebug

import (
	"log"
	"os"
	"sync"
)

var (
	once    sync.Once
	enabled bool
)

// Enabled reports whether TMDB_LOOKUP_DEBUG=1 is set.
func Enabled() bool {
	once.Do(func() {
		enabled = os.Getenv("TMDB_LOOKUP_DEBUG") == "1"
	})
	return enabled
}

// Printf logs only when TMDB_LOOKUP_DEBUG=1.
func Printf(format string, args ...any) {
	if Enabled() {
		log.Printf(format, args...)
	}
}

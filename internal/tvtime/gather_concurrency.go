package tvtime

import (
	"os"
	"strconv"
)

const defaultGatherConcurrency = 24

func gatherConcurrency() int {
	raw := os.Getenv("TVTIME_GATHER_CONCURRENCY")
	if raw == "" {
		return defaultGatherConcurrency
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultGatherConcurrency
	}
	if n > 64 {
		return 64
	}
	return n
}

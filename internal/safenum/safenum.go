package safenum

import "math"

// Float64ToInt64 converts f to int64 when it is finite and within range.
func Float64ToInt64(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f < float64(math.MinInt64) || f > float64(math.MaxInt64) {
		return 0, false
	}
	return int64(f), true
}

// Float64ToInt converts f to int when it is finite and within range.
func Float64ToInt(f float64) (int, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f < float64(math.MinInt) || f > float64(math.MaxInt) {
		return 0, false
	}
	return int(f), true
}

// ClampInt32 returns n bounded to [min, max].
func ClampInt32(n, min, max int32) int32 {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

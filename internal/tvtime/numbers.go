package tvtime

import (
	"encoding/json"

	"github.com/alireza/tvtime2serializd/internal/safenum"
)

func asString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		if i, ok := safenum.Float64ToInt64(n); ok {
			return i
		}
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return 0
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		if i, ok := safenum.Float64ToInt(n); ok {
			return i
		}
	case int:
		return n
	case int64:
		if i, ok := safenum.Float64ToInt(float64(n)); ok {
			return i
		}
	case json.Number:
		if i, err := n.Int64(); err == nil {
			if out, ok := safenum.Float64ToInt(float64(i)); ok {
				return out
			}
		}
	}
	return 0
}

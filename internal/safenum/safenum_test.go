package safenum

import (
	"math"
	"testing"
)

func TestFloat64ToInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      float64
		want    int64
		wantOK  bool
	}{
		{name: "valid", in: 413215, want: 413215, wantOK: true},
		{name: "nan", in: math.NaN(), wantOK: false},
		{name: "overflow", in: float64(math.MaxInt64) * 2, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Float64ToInt64(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestClampInt32(t *testing.T) {
	t.Parallel()

	if got := ClampInt32(100, 1, 36); got != 36 {
		t.Fatalf("got %d, want 36", got)
	}
	if got := ClampInt32(0, 1, 36); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

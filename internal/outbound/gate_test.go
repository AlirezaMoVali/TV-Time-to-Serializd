package outbound

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestGateLimitsConcurrency(t *testing.T) {
	t.Parallel()

	g := NewGate(2)
	ctx := context.Background()
	var active atomic.Int32
	var peak atomic.Int32

	done := make(chan struct{}, 4)
	for range 4 {
		go func() {
			_ = g.Call(ctx, func() (int, error) {
				cur := active.Add(1)
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				active.Add(-1)
				return http.StatusOK, nil
			})
			done <- struct{}{}
		}()
	}

	for range 4 {
		<-done
	}
	if peak.Load() > 2 {
		t.Fatalf("peak concurrency %d exceeds gate max 2", peak.Load())
	}
}

func TestGateNilPassthrough(t *testing.T) {
	t.Parallel()

	var g *Gate
	called := false
	if err := g.Call(context.Background(), func() (int, error) {
		called = true
		return http.StatusOK, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected fn to run")
	}
}

func TestGate429Backoff(t *testing.T) {
	t.Parallel()

	g := NewGate(4)
	ctx := context.Background()

	start := time.Now()
	_ = g.Call(ctx, func() (int, error) {
		return http.StatusTooManyRequests, nil
	})
	if err := g.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	g.release()

	elapsed := time.Since(start)
	if elapsed < default429Cooldown/2 {
		t.Fatalf("expected backoff delay, got %v", elapsed)
	}
}

func TestCooldownDuration(t *testing.T) {
	t.Parallel()

	if d := cooldownDuration("5"); d != 5*time.Second {
		t.Fatalf("got %v", d)
	}
	if d := cooldownDuration(""); d != default429Cooldown {
		t.Fatalf("got %v", d)
	}
}

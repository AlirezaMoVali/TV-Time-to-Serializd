package outbound

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	default429Cooldown = 2 * time.Second
	max429Cooldown     = 30 * time.Second
)

// Gate limits concurrent outbound HTTP calls to one external service and
// applies a short shared cooldown after HTTP 429 responses.
type Gate struct {
	sem chan struct{}

	mu           sync.Mutex
	backoffUntil time.Time
}

func NewGate(maxConcurrent int) *Gate {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Gate{sem: make(chan struct{}, maxConcurrent)}
}

// Call acquires a slot, runs fn, and records rate-limit responses.
// A nil receiver is a no-op pass-through.
func (g *Gate) Call(ctx context.Context, fn func() (status int, err error)) error {
	if g == nil {
		_, err := fn()
		return err
	}
	if err := g.acquire(ctx); err != nil {
		return err
	}
	defer g.release()

	status, err := fn()
	if status == http.StatusTooManyRequests {
		g.triggerBackoff("")
	}
	return err
}

func (g *Gate) acquire(ctx context.Context) error {
	for {
		if err := g.waitBackoff(ctx); err != nil {
			return err
		}
		select {
		case g.sem <- struct{}{}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (g *Gate) release() {
	select {
	case <-g.sem:
	default:
	}
}

func (g *Gate) waitBackoff(ctx context.Context) error {
	g.mu.Lock()
	until := g.backoffUntil
	g.mu.Unlock()
	if !until.After(time.Now()) {
		return nil
	}
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (g *Gate) triggerBackoff(retryAfter string) {
	d := cooldownDuration(retryAfter)
	g.mu.Lock()
	next := time.Now().Add(d)
	if next.After(g.backoffUntil) {
		g.backoffUntil = next
	}
	g.mu.Unlock()
}

func cooldownDuration(retryAfter string) time.Duration {
	if retryAfter != "" {
		if sec, err := strconv.Atoi(retryAfter); err == nil && sec > 0 {
			d := time.Duration(sec) * time.Second
			if d > max429Cooldown {
				return max429Cooldown
			}
			return d
		}
		if t, err := http.ParseTime(retryAfter); err == nil {
			d := time.Until(t)
			if d < default429Cooldown {
				return default429Cooldown
			}
			if d > max429Cooldown {
				return max429Cooldown
			}
			return d
		}
	}
	return default429Cooldown
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

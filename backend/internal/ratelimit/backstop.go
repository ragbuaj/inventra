package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type bsEntry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// backstop is a per-instance, per-key token-bucket fallback consulted only when
// Redis is unavailable on auth paths. It is bounded by a hard cap plus idle
// eviction so it cannot grow without limit during an outage.
type backstop struct {
	mu      sync.Mutex
	buckets map[string]*bsEntry
	max     int
	now     func() time.Time
}

func newBackstop(max int) *backstop {
	return &backstop{buckets: make(map[string]*bsEntry), max: max, now: time.Now}
}

// Allow reports whether key is within perMin on this instance. Above the cap it
// returns true (fail-open) rather than allocating a new bucket.
func (b *backstop) Allow(key string, perMin int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.buckets[key]
	if !ok {
		if len(b.buckets) >= b.max {
			return true
		}
		e = &bsEntry{lim: rate.NewLimiter(rate.Limit(float64(perMin)/60.0), perMin)}
		b.buckets[key] = e
	}
	e.lastSeen = b.now()
	return e.lim.Allow()
}

// evictIdle removes entries unused for longer than idle.
func (b *backstop) evictIdle(idle time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cutoff := b.now().Add(-idle)
	for k, e := range b.buckets {
		if e.lastSeen.Before(cutoff) {
			delete(b.buckets, k)
		}
	}
}

func (b *backstop) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buckets)
}

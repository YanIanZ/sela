package httpapi

import (
	"sync"
	"time"
)

// bucketRates are requests per minute per client address. They protect the
// demo and the paid model endpoint; journey adds per-session AI limits.
var bucketRates = map[string]float64{
	"session":   30,
	"mutation":  180,
	"ai":        12,
	"recipient": 10,
	"read":      600,
	"research":  120,
}

type bucket struct {
	tokens float64
	last   time.Time
}

// limiter is a bounded in-memory token-bucket limiter keyed by bucket+address.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
	max     int
}

func newLimiter() *limiter {
	return &limiter{buckets: map[string]*bucket{}, now: time.Now, max: 50_000}
}

func (l *limiter) allow(name, addr string) bool {
	rate, ok := bucketRates[name]
	if !ok {
		rate = 60
	}
	burst := rate / 4
	if burst < 3 {
		burst = 3
	}
	key := name + "|" + addr
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) >= l.max {
			for k, v := range l.buckets {
				if now.Sub(v.last) > 2*time.Minute {
					delete(l.buckets, k)
				}
			}
			if len(l.buckets) >= l.max {
				return false
			}
		}
		b = &bucket{tokens: burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Minutes() * rate
	if b.tokens > burst {
		b.tokens = burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

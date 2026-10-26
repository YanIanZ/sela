package httpapi

import (
	"container/list"
	"net/netip"
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
	key    string
	tokens float64
	last   time.Time
}

// limiter is a bounded in-memory token-bucket limiter keyed by bucket and
// client address. When it is full it forgets the least recently used key
// (whose bucket has usually refilled anyway) instead of refusing new
// clients, so filling the map cannot lock other visitors out.
type limiter struct {
	mu    sync.Mutex
	items map[string]*list.Element // of *bucket
	lru   *list.List               // front is the most recently used
	now   func() time.Time
	max   int
}

func newLimiter() *limiter {
	return &limiter{items: map[string]*list.Element{}, lru: list.New(), now: time.Now, max: 50_000}
}

// limitKey groups addresses the way one client controls them: an IPv4
// address (IPv4-mapped IPv6 included) is its own key, and an IPv6 address is
// keyed by its /64 so a single host cannot mint unlimited keys.
func limitKey(addr string) string {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return addr
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is4() {
		return ip.String()
	}
	p, err := ip.Prefix(64)
	if err != nil {
		return ip.String()
	}
	return p.String()
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
	key := name + "|" + limitKey(addr)
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	var b *bucket
	if el := l.items[key]; el != nil {
		b = el.Value.(*bucket)
		l.lru.MoveToFront(el)
	} else {
		if oldest := l.lru.Back(); oldest != nil && l.lru.Len() >= l.max {
			l.lru.Remove(oldest)
			delete(l.items, oldest.Value.(*bucket).key)
		}
		b = &bucket{key: key, tokens: burst, last: now}
		l.items[key] = l.lru.PushFront(b)
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

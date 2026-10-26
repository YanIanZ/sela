package httpapi

import (
	"container/heap"
	"math"
	"sync"
	"time"
)

// rateSpec is a token bucket with its own rate, window and burst
// (ADR-0002 §9), keyed by client address, username hash, session or user.
type rateSpec struct {
	name   string
	count  float64
	window time.Duration
	burst  float64
}

// The /api/v3 auth buckets (API_SPEC_V3 §1 "Rate limits").
var (
	rlLoginClient    = rateSpec{"v3.login.client", 20, time.Minute, 10}
	rlLoginUser      = rateSpec{"v3.login.username", 10, time.Hour, 5}
	rlRegister       = rateSpec{"v3.register.client", 20, time.Hour, 5}
	rlGuest          = rateSpec{"v3.guest.client", 30, time.Hour, 10}
	rlRecoveryClient = rateSpec{"v3.recovery.client", 10, time.Hour, 3}
	rlRecoveryUser   = rateSpec{"v3.recovery.username", 5, time.Hour, 3}
	rlSessionSecret  = rateSpec{"v3.password.session", 10, time.Hour, 10}
	rlExport         = rateSpec{"v3.export.user", 6, time.Hour, 6}
)

// authBucketsMax bounds the v3 auth buckets (a few hundred bytes each, so
// some tens of MB when full).
const authBucketsMax = 200_000

// specBucket is one v3 bucket. fullAt is when it will have refilled to
// its burst; from then on it is indistinguishable from a new bucket.
type specBucket struct {
	key    string
	tokens float64
	last   time.Time
	fullAt time.Time
	index  int // position in the heap
}

// specLimiter holds the v3 auth buckets apart from the v2 limiter (review
// round 3, R3-4). The v2 limiter forgets its least recently used key when
// full, which is harmless for its minute-scale buckets but handed an
// exhausted per-username bucket (10 per hour) back full to anyone who
// flooded the shared LRU with cheap requests from many addresses. This
// store never forgets a bucket that has not refilled: when it is full it
// drops a bucket that has refilled to its burst (the one that refilled
// first, from a heap ordered by fullAt), and when there is none it refuses
// the new key with 429 and the time until one refills, so existing keys
// keep their state and their limits.
type specLimiter struct {
	mu    sync.Mutex
	items map[string]*specBucket
	heap  bucketHeap
	now   func() time.Time
	max   int
}

func newSpecLimiter() *specLimiter {
	return &specLimiter{items: map[string]*specBucket{}, now: time.Now, max: authBucketsMax}
}

// allow takes one token from the bucket of key under sp and, when it is
// empty (or the store is full of buckets that have not refilled), reports
// how long to wait.
func (l *specLimiter) allow(sp rateSpec, key string) (bool, time.Duration) {
	k := sp.name + "|" + key
	now := l.now()
	perSecond := sp.count / sp.window.Seconds()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.items[k]
	if b == nil {
		if len(l.items) >= l.max {
			oldest := l.heap[0]
			if oldest.fullAt.After(now) {
				return false, oldest.fullAt.Sub(now)
			}
			heap.Pop(&l.heap)
			delete(l.items, oldest.key)
		}
		b = &specBucket{key: k, tokens: sp.burst, last: now}
		l.items[k] = b
		heap.Push(&l.heap, b)
	}
	b.tokens = math.Min(sp.burst, b.tokens+now.Sub(b.last).Seconds()*perSecond)
	b.last = now
	allowed, wait := true, time.Duration(0)
	if b.tokens < 1 {
		allowed, wait = false, time.Duration((1-b.tokens)/perSecond*float64(time.Second))
	} else {
		b.tokens--
	}
	b.fullAt = now.Add(time.Duration((sp.burst - b.tokens) / perSecond * float64(time.Second)))
	heap.Fix(&l.heap, b.index)
	return allowed, wait
}

// size reports how many buckets are held (tests).
func (l *specLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.items)
}

// bucketHeap is a min-heap of buckets by fullAt.
type bucketHeap []*specBucket

func (h bucketHeap) Len() int           { return len(h) }
func (h bucketHeap) Less(i, j int) bool { return h[i].fullAt.Before(h[j].fullAt) }
func (h bucketHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

func (h *bucketHeap) Push(x any) {
	b := x.(*specBucket)
	b.index = len(*h)
	*h = append(*h, b)
}

func (h *bucketHeap) Pop() any {
	old := *h
	b := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	b.index = -1
	return b
}

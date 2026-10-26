package httpapi

import (
	"fmt"
	"testing"
	"time"
)

// R3-4: the v3 auth store never forgets a bucket that has not refilled.
// When full it drops only a bucket that has refilled to its burst (which
// is the same as a new one), and with none it refuses the new key with the
// time until one refills; an exhausted per-username bucket keeps its state
// through a flood of other keys, and the store stays bounded.
func TestSpecLimiterKeepsBucketsThatHaveNotRefilled(t *testing.T) {
	l := newSpecLimiter()
	l.max = 3
	now := time.Unix(1_800_000_000, 0)
	l.now = func() time.Time { return now }
	// Exhaust one username bucket (burst 5, 10 per hour: a token every 6 min).
	for i := 0; i < 5; i++ {
		if ok, _ := l.allow(rlLoginUser, "korban"); !ok {
			t.Fatalf("burst attempt %d refused", i)
		}
	}
	if ok, wait := l.allow(rlLoginUser, "korban"); ok || wait != 6*time.Minute {
		t.Fatalf("exhausted bucket: %t %s", ok, wait)
	}
	// Two client buckets (20 per minute, burst 10: full again 3 s after one use).
	l.allow(rlLoginClient, "192.0.2.1")
	l.allow(rlLoginClient, "192.0.2.2")
	// Full, and nothing has refilled: a new key is refused, not admitted by
	// forgetting someone's state.
	if ok, wait := l.allow(rlLoginClient, "192.0.2.3"); ok || wait <= 0 || wait > 3*time.Second {
		t.Fatalf("a full store admitted a new key (or gave no wait): %t %s", ok, wait)
	}
	// A flood of new keys, each arriving after the previous ones refilled,
	// for 26 min 40 s: each takes the place of a refilled client bucket.
	for i := 0; i < 400; i++ {
		now = now.Add(4 * time.Second)
		if ok, _ := l.allow(rlLoginClient, fmt.Sprintf("2001:db8:%x::/64", i)); !ok {
			t.Fatalf("flood key %d refused although refilled buckets could make room", i)
		}
	}
	if l.size() != 3 {
		t.Fatalf("store grew past its bound: %d", l.size())
	}
	// The username bucket kept its state: 26 min 40 s refilled 4.4 tokens,
	// so four attempts pass and the fifth waits. A forgotten bucket would
	// have come back with its full burst of five.
	for i := 0; i < 4; i++ {
		if ok, _ := l.allow(rlLoginUser, "korban"); !ok {
			t.Fatalf("refilled attempt %d refused", i)
		}
	}
	if ok, _ := l.allow(rlLoginUser, "korban"); ok {
		t.Fatal("the exhausted username bucket came back full after a flood of other keys")
	}
}

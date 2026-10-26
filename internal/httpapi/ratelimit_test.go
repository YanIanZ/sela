package httpapi

import (
	"fmt"
	"testing"
	"time"
)

// A full limiter must forget its least recently used key, never refuse an
// unseen client, and keep the state of keys that are still in use.
func TestLimiterEvictsLeastRecentlyUsedInsteadOfFailingClosed(t *testing.T) {
	l := newLimiter()
	l.max = 3
	now := time.Unix(1_800_000_000, 0)
	l.now = func() time.Time { return now }
	// "ai" allows a burst of 3.
	for i := 0; i < 3; i++ {
		if !l.allow("ai", "192.0.2.1") {
			t.Fatalf("burst request %d refused", i)
		}
	}
	if l.allow("ai", "192.0.2.1") {
		t.Fatal("exhausted bucket allowed")
	}
	l.allow("ai", "192.0.2.2")
	l.allow("ai", "192.0.2.3") // full: .1 .2 .3
	if l.allow("ai", "192.0.2.1") { // .1 becomes the most recent
		t.Fatal("exhausted bucket allowed")
	}
	if !l.allow("ai", "192.0.2.4") {
		t.Fatal("a full limiter refused an unseen client")
	}
	if l.allow("ai", "192.0.2.1") {
		t.Fatal("the recently used, exhausted key was evicted instead of the least recently used one")
	}
	if got := l.lru.Len(); got != 3 || len(l.items) != 3 {
		t.Fatalf("limiter grew past its bound: list %d map %d", got, len(l.items))
	}
	// A flood of unseen keys stays bounded and never fails closed.
	for i := 0; i < 1000; i++ {
		if !l.allow("read", fmt.Sprintf("198.51.%d.%d", i/256, i%256)) {
			t.Fatalf("flood key %d refused", i)
		}
	}
	if l.lru.Len() != 3 || len(l.items) != 3 {
		t.Fatalf("limiter grew past its bound: list %d map %d", l.lru.Len(), len(l.items))
	}
}

func TestLimiterKeysIPv6ByPrefix(t *testing.T) {
	l := newLimiter()
	now := time.Unix(1_800_000_000, 0)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		l.allow("ai", fmt.Sprintf("2001:db8:1:2::%x", i+1))
	}
	if l.allow("ai", "2001:db8:1:2:ffff:ffff:ffff:fffe") {
		t.Fatal("a second address in the same /64 got its own bucket")
	}
	if !l.allow("ai", "2001:db8:1:3::1") {
		t.Fatal("a different /64 shared the bucket")
	}
	for i := 0; i < 3; i++ {
		l.allow("ai", "192.0.2.9")
	}
	if l.allow("ai", "::ffff:192.0.2.9") {
		t.Fatal("an IPv4-mapped address escaped its IPv4 bucket")
	}
	for in, want := range map[string]string{
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"fe80::1%eth0":         "fe80::/64",
		"::ffff:10.0.0.1":      "10.0.0.1",
		"203.0.113.5":          "203.0.113.5",
		"not-an-ip":            "not-an-ip",
	} {
		if got := limitKey(in); got != want {
			t.Errorf("limitKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLastHopReadsEveryHeaderLine(t *testing.T) {
	for _, c := range []struct {
		lines []string
		want  string
	}{
		{[]string{"10.9.0.1", "203.0.113.7"}, "203.0.113.7"},
		{[]string{"10.9.0.1, 198.51.100.2", "203.0.113.7, "}, "203.0.113.7"},
		{[]string{"10.9.0.1, 203.0.113.8"}, "203.0.113.8"},
		{[]string{"[2001:db8::1]:443"}, "2001:db8::1"},
		{[]string{"203.0.113.9:5000"}, "203.0.113.9"},
		{[]string{"10.9.0.1", "unknown"}, ""},
		{[]string{" , "}, ""},
		{nil, ""},
	} {
		if got := lastHop(c.lines); got != c.want {
			t.Errorf("lastHop(%q) = %q, want %q", c.lines, got, c.want)
		}
	}
}

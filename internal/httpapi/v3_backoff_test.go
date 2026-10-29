package httpapi_test

import (
	"bytes"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// S1 over HTTP: an attacker sends 5 wrong passwords for each candidate,
// waits for a deploy, then sends more. An existing and an unknown username
// get the same statuses, codes, Retry-After values and bodies before and
// after the restart.
func TestV3LoginBackoffUniformAfterRestart(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	e.client().register("pelangi.ada")
	candidates := []string{"pelangi.ada", "pelangi.tidak"}
	answer := func(r resp) string {
		return fmt.Sprintf("%d %s Retry-After=%s %s", r.status, r.code(), r.header.Get("Retry-After"), bytes.TrimSpace(r.raw))
	}
	for _, u := range candidates {
		for i := 0; i < 5; i++ {
			if r := e.client().login(u, "tebakan salah nomor "+fmt.Sprint(i)); r.status != 401 {
				t.Fatalf("preload %s %d: %d %s", u, i, r.status, r.raw)
			}
		}
	}
	e.clock.Advance(time.Second)
	before := map[string]string{}
	for _, u := range candidates {
		before[u] = answer(e.client().login(u, "tebakan salah keenam"))
	}
	if before["pelangi.ada"] != before["pelangi.tidak"] {
		t.Fatalf("before the restart: %v", before)
	}
	// Within the observation window the backoff continues after the
	// restart; after it the count starts again. Either way alike for both.
	for _, step := range []struct {
		gap  time.Duration
		want []string
	}{
		{10 * time.Minute, []string{"401", "429", "429"}},
		{account.ThrottleQuiet + time.Minute, []string{"401", "401", "401"}},
	} {
		gap, want := step.gap, step.want
		e.clock.Advance(gap)
		e.restart() // a deploy: fresh limiter, same database
		after := map[string][]string{}
		for _, u := range candidates {
			for i := 0; i < 3; i++ {
				after[u] = append(after[u], answer(e.client().login(u, "tebakan salah setelah restart")))
			}
		}
		if !slices.Equal(after["pelangi.ada"], after["pelangi.tidak"]) {
			t.Fatalf("after a restart (+%s): existing %v, unknown %v", gap, after["pelangi.ada"], after["pelangi.tidak"])
		}
		var got []string
		for _, a := range after["pelangi.tidak"] {
			got = append(got, a[:3])
		}
		if !slices.Equal(got, want) {
			t.Fatalf("schedule after the restart (+%s): %v, want %v", gap, after["pelangi.tidak"], want)
		}
	}
}

// S3 over HTTP: concurrent guest starts from different clients never
// exceed SELA_MAX_GUEST_ACCOUNTS.
func TestV3GuestCapConcurrent(t *testing.T) {
	t.Parallel()
	const maxGuests, n = 2, 10
	e := newV3(t, v3opt{mutateC: func(c *account.Config) { c.MaxGuests = maxGuests }})
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	clients := make([]*v3client, n)
	for i := range clients {
		clients[i] = e.client()
	}
	start := make(chan struct{})
	for _, c := range clients {
		wg.Add(1)
		go func(c *v3client) {
			defer wg.Done()
			<-start
			r := c.call("POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": "18_plus",
				"termsPolicyVersion": policyVersion("terms_privacy"), "guestNoticeVersion": policyVersion("guest_storage")})
			mu.Lock()
			codes[r.status]++
			mu.Unlock()
		}(c)
	}
	close(start)
	wg.Wait()
	if live := e.count(`SELECT count(*) FROM users WHERE kind = 'guest'`); live != maxGuests || codes[201] != maxGuests || codes[503] != n-maxGuests {
		t.Fatalf("MaxGuests=%d: statuses %v, live guests %d", maxGuests, codes, live)
	}
}

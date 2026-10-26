package httpapi_test

// Regression tests for auth review round 3 over HTTP (R3-3, R3-4, R3-6).
// The store-level ones are in internal/account/review_round3_test.go.

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/vault"
)

const r3Wrong = "salah salah salah salah"

// R3-3 over HTTP: after 5 failures and the end of the 1 s wait, five
// concurrent attempts (what the per-username bucket lets through after
// 30 minutes) get one 401 and four 429: the first is reserved, and its
// sixth failure imposes a 2 s wait on the others. Before, all five were
// verified.
func TestR3HTTPConcurrentAttemptsWaitForBackoff(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	e.client().register("serbu.http")
	for i := 0; i < 5; i++ {
		if r := e.client().login("serbu.http", r3Wrong); r.status != 401 {
			t.Fatalf("failure %d: %d %s", i, r.status, r.raw)
		}
	}
	e.clock.Advance(30 * time.Minute) // bucket refilled to 5, backoff over, inside the observation window
	before := e.store.Hasher().Verifications()
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	retry := map[string]int{}
	start := make(chan struct{})
	for i := 0; i < 5; i++ {
		c := e.client()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r := c.call("POST", "/api/v3/auth/login", map[string]any{"username": "serbu.http", "password": r3Wrong})
			mu.Lock()
			statuses[r.status]++
			if r.status == 429 {
				retry[r.header.Get("Retry-After")]++
			}
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()
	verified := e.store.Hasher().Verifications() - before
	t.Logf("statuses %v, Retry-After %v, verified %d, throttle failures %d", statuses, retry, verified, e.count(`SELECT failures FROM login_throttle`))
	if statuses[401] != 1 || statuses[429] != 4 || verified != 1 || retry["2"] != 4 {
		t.Fatalf("one backoff step let %d attempts through (statuses %v, Retry-After %v, %d verified); want one 401, then 429 with Retry-After 2",
			statuses[401], statuses, retry, verified)
	}
}

// R3-3 over HTTP: 99 earlier failures; three wrong attempts and the right
// password arrive together. The right password never gets 200 and a
// session once the 100th attempt disabled password sign-in.
func TestR3HTTPNoSignInAfterDisable(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	e.client().register("hampir.kunci")
	for round := 0; round < 10; round++ {
		e.clock.Advance(time.Hour) // refill the per-username bucket
		if _, err := e.db.Pool.Exec(bg, `UPDATE credentials SET failed_count = 99, password_login_disabled_at = NULL; DELETE FROM login_throttle`); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var ok resp
		for i := 0; i < 3; i++ {
			c := e.client()
			wg.Add(1)
			go func() { defer wg.Done(); c.login("hampir.kunci", r3Wrong) }()
		}
		owner := e.client()
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(round%4) * time.Millisecond)
			ok = owner.login("hampir.kunci", v3Password)
		}()
		wg.Wait()
		disabled := e.count(`SELECT count(*) FROM credentials WHERE password_login_disabled_at IS NOT NULL`)
		if ok.status == 200 && disabled == 1 {
			r := owner.call("GET", "/api/v3/me", nil)
			t.Fatalf("round %d: 200 and a session (GET /me %d) for a password sign-in after the 100th attempt disabled it", round, r.status)
		}
		if ok.status != 200 && ok.status != 403 && ok.status != 429 {
			t.Fatalf("round %d: the right password got %d %s", round, ok.status, ok.raw)
		}
	}
}

// R3-4: the per-username login bucket (10 per hour) is not kept in the
// v2 limiter's shared LRU any more, so 50,000 cheap GETs from distinct
// client keys (IPv6 /64s) cannot evict an exhausted username bucket and
// hand it back full.
func TestR3HTTPUsernameBucketSurvivesReadFlood(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	attempt := func() resp { return e.client().login("tak.ada.siapa", r3Wrong) }
	for i := 0; i < 5; i++ {
		attempt()
	}
	e.clock.Advance(2 * time.Second) // the database backoff (1 s) is over; the bucket is still empty
	r := attempt()
	if r.status != 429 {
		t.Fatalf("precondition: the 6th attempt within the hour is refused by the username bucket: %d %s", r.status, r.raw)
	}
	before := r.header.Get("Retry-After")
	for i := 0; i < 50_000; i++ {
		req := httptest.NewRequest("GET", "/api/v3/auth/options", nil)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("2001:db8:%x:%x::1", i>>16, i&0xffff))
		e.h.ServeHTTP(httptest.NewRecorder(), req)
	}
	r = attempt()
	if r.status != 429 || r.code() != "rate_limited" {
		t.Fatalf("after 50,000 GETs from distinct /64s the username bucket was forgotten: %d %s", r.status, r.raw)
	}
	t.Logf("Retry-After before the flood %s, after %s", before, r.header.Get("Retry-After"))
}

// R3-6: an unreachable database answers 503 store_unavailable with
// Retry-After on every store-backed route (API_SPEC_V3 §2), not 500
// internal; the log line carries only the error class.
func TestR3HTTPUnreachableDatabaseIs503(t *testing.T) {
	t.Parallel()
	const url = "postgres://sela:x@127.0.0.1:1/none?sslmode=disable&connect_timeout=2"
	pool, err := pgxpool.New(bg, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	v, err := vault.New(vault.Keyring{Current: vault.Key{ID: "test-down", Secret: make([]byte, 32)}}, vault.Options{})
	if err != nil {
		t.Fatal(err)
	}
	clock := &v3clock{now: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)}
	s, err := account.New(pool, v, v3Config(clock))
	if err != nil {
		t.Fatal(err)
	}
	e := &v3env{t: t, logs: &syncBuffer{}, clock: clock, cfg: v3Config(clock)}
	e.opt = httpapi.Options{TrustProxy: true, Accounts: s, AccountConfig: e.cfg, Clock: clock.Now}
	e.start()
	c := e.client()
	c.setToken(strings.Repeat("A", 43))
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/v3/auth/login", map[string]any{"username": "pelangi.senja", "password": r3Wrong}},
		{"POST", "/api/v3/auth/register", map[string]any{"username": "pelangi.baru", "password": v3Password, "locale": "id", "ageBand": "18_plus",
			"termsPolicyVersion": policyVersion("terms_privacy")}},
		{"POST", "/api/v3/auth/recovery", map[string]any{"username": "pelangi.senja", "recoveryCode": "7K2M-QX9D-4HTR-PZ3B", "newPassword": v3NewPassword}},
		{"GET", "/api/v3/me", nil},
		{"POST", "/api/v3/auth/logout", nil},
	} {
		r := c.call(tc.method, tc.path, tc.body)
		if r.status != 503 || r.code() != "store_unavailable" || r.header.Get("Retry-After") != "5" {
			t.Errorf("%s %s with the database unreachable: %d %s Retry-After=%q, want 503 store_unavailable with Retry-After 5",
				tc.method, tc.path, r.status, r.raw, r.header.Get("Retry-After"))
		}
	}
	logs := e.logs.String()
	if !strings.Contains(logs, `"msg":"v3.store_unavailable"`) || strings.Contains(logs, "127.0.0.1:1") || strings.Contains(logs, "sela:x") {
		t.Fatalf("logs: %s", logs)
	}
}

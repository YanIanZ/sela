package llm

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestBackoffBounds(t *testing.T) {
	for n := 1; n <= 10; n++ {
		base := backoffCap
		if n < 4 {
			base = min(backoffBase<<(n-1), backoffCap)
		}
		for i := 0; i < 200; i++ {
			d := backoff(n)
			if d < base/2 || d > base {
				t.Fatalf("backoff(%d) = %v, want [%v, %v]", n, d, base/2, base)
			}
		}
	}
	if backoff(1) > 500*time.Millisecond || backoff(100) > 4*time.Second {
		t.Fatal("backoff exceeds its bounds")
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"":                     -1,
		"abc":                  -1,
		"-1":                   -1,
		"0":                    0,
		"3":                    3 * time.Second,
		" 7 ":                  7 * time.Second,
		"99999999999999999999": -1,
		"86400":                time.Hour,
		now.Add(5 * time.Second).Format(http.TimeFormat):  5 * time.Second,
		now.Add(-5 * time.Second).Format(http.TimeFormat): 0,
	}
	for in, want := range cases {
		if got := parseRetryAfter(in, now); got != want {
			t.Fatalf("parseRetryAfter(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIntCode(t *testing.T) {
	cases := map[string]int{`400`: 400, `"503"`: 503, `"x"`: 0, `99`: 0, `600`: 0, `null`: 0, ``: 0}
	for in, want := range cases {
		if got := intCode(json.RawMessage(in)); got != want {
			t.Fatalf("intCode(%s) = %d, want %d", in, got, want)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LOCALHOST": true, "127.0.0.1": true, "127.1.2.3": true, "::1": true,
		"10.0.0.1": false, "example.com": false, "": false, "localhost.example.com": false,
	} {
		if got := isLoopback(host); got != want {
			t.Fatalf("isLoopback(%q) = %v", host, got)
		}
	}
}

func TestStatusErrorKinds(t *testing.T) {
	for code, want := range map[int]Kind{
		401: KindAuth, 403: KindAuth, 429: KindRateLimited, 400: KindRejected, 404: KindRejected, 422: KindRejected,
		500: KindUnavailable, 503: KindUnavailable, 301: KindProtocol, 204: KindProtocol,
	} {
		e := statusError(code)
		if e.Kind != want || e.Status != code {
			t.Fatalf("statusError(%d) = %+v, want %s", code, e, want)
		}
	}
}

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

func TestPinnedRoot(t *testing.T) {
	const (
		model = "meta-models/Muse-Glimmer-30B"
		full  = "cec94fcfd6512f2cfe708138c32533b346ea4f69"
		hub   = "/root/.cache/huggingface/hub/"
	)
	cases := []struct {
		root, model, rev string
		want             bool
	}{
		{hub + "models--meta-models--Muse-Glimmer-30B/snapshots/" + full, model, full, true},
		{hub + "models--meta-models--Muse-Glimmer-30B/snapshots/" + full + "/", model, full, true},
		{"models--meta-models--Muse-Glimmer-30B/snapshots/" + full, model, full, true},
		{model + "@" + full, model, full, true},
		{hub + "models--gpt2/snapshots/abc", "gpt2", "abc", true},
		// The reviewer's probes: a longer repository name, a revision under
		// another snapshot, and a revision found inside a larger string.
		{hub + "models--meta-models--Muse-Glimmer-30B-Uncensored/snapshots/cec94fc0123456789", model, "cec94fc", false},
		{hub + "models--meta-models--Muse-Glimmer-30B/snapshots/deadbeef/" + full, model, full, false},
		{model + "@zzabczz", model, "abc", false},
		{hub + "models--meta-models--Muse-Glimmer-30B-Uncensored/snapshots/" + full, model, full, false},
		{hub + "models--meta-models--Muse-Glimmer-30B/snapshots/" + full[:7], model, full, false},
		{hub + "models--meta-models--Muse-Glimmer-30B/snapshots/" + full, model, full[:7], false},
		{hub + "models--meta-models--Muse-Glimmer-30B/snapshots/" + full + "/..", model, full, false},
		{hub + "models--meta-models--Muse-Glimmer-30B/blobs/" + full, model, full, false},
		{hub + "x-models--meta-models--Muse-Glimmer-30B/snapshots/" + full, model, full, false},
		{hub + "models--other--Muse-Glimmer-30B/snapshots/" + full, model, full, false},
		{model, model, full, false},
		{model + "@" + full + "x", model, full, false},
		{"", model, full, false},
		{hub + "models--/snapshots/", "", "", false},
		{"@", "", "", false},
	}
	for _, tc := range cases {
		if got := pinnedRoot(tc.root, tc.model, tc.rev); got != tc.want {
			t.Errorf("pinnedRoot(%q, %q, %q) = %v, want %v", tc.root, tc.model, tc.rev, got, tc.want)
		}
	}
}

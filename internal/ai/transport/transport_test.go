package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestReasonClasses(t *testing.T) {
	cases := map[Reason]Class{
		ReasonTimeout: ClassAvailability, ReasonRateLimited: ClassAvailability, ReasonSpendCap: ClassAvailability,
		ReasonAuth: ClassAvailability, ReasonUnavailable: ClassAvailability,
		ReasonRefused: ClassContent, ReasonTruncated: ClassContent, ReasonInvalidOutput: ClassContent,
		ReasonEmptyContent: ClassContent, ReasonContentFilter: ClassContent,
		ReasonModelMismatch: ClassIntegrity, ReasonTestDouble: ClassIntegrity,
		ReasonRejected: ClassPolicy, ReasonCanceled: ClassPolicy, ReasonDisabled: ClassPolicy,
	}
	for r, want := range cases {
		if got := r.Class(); got != want {
			t.Errorf("%s.Class() = %s, want %s", r, got, want)
		}
	}
	// Reasons lists exactly the reasons classified here, so a new reason
	// gets a class decision and the router sweeps pick it up.
	if len(Reasons()) != len(cases) {
		t.Fatalf("Reasons() has %d reasons, the class table %d", len(Reasons()), len(cases))
	}
	for _, r := range Reasons() {
		if _, ok := cases[r]; !ok {
			t.Errorf("reason %s has no class decision here", r)
		}
	}
}

func TestReasonOf(t *testing.T) {
	if ReasonOf(nil) != "" || ReasonOf(context.Canceled) != ReasonCanceled || ReasonOf(context.DeadlineExceeded) != ReasonTimeout {
		t.Fatal("context reasons")
	}
	wrapped := errors.Join(errors.New("x"), NewError("p", ReasonRefused, 200, "d"))
	if ReasonOf(wrapped) != ReasonRefused || ReasonOf(errors.New("other")) != ReasonUnavailable {
		t.Fatal("wrapped reasons")
	}
	if msg := NewError("anthropic", ReasonRateLimited, 429, "detail").Error(); msg != "ai/anthropic: rate_limited (HTTP 429): detail" {
		t.Fatalf("message %q", msg)
	}
}

func TestStatusError(t *testing.T) {
	cases := []struct {
		status    int
		body      string
		reason    Reason
		retryable bool
	}{
		{401, "", ReasonAuth, false},
		{403, "", ReasonAuth, false},
		{402, "", ReasonSpendCap, false},
		{429, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`, ReasonRateLimited, true},
		{429, `{"type":"error","error":{"type":"rate_limit_error","message":"x enforced_spend_limit_reached"}}`, ReasonSpendCap, false},
		{429, `{"error":{"type":"enforced_spend_limit_reached"}}`, ReasonSpendCap, false},
		{500, "", ReasonUnavailable, true},
		{502, "", ReasonUnavailable, true},
		{503, "", ReasonUnavailable, true},
		{504, "", ReasonUnavailable, true},
		{529, "", ReasonUnavailable, true},
		{501, "", ReasonUnavailable, false},
		{400, "", ReasonRejected, false},
		{404, "", ReasonRejected, false},
		{302, "", ReasonRejected, false},
	}
	for _, tc := range cases {
		e, retry := StatusError("p", Response{Status: tc.status, Body: []byte(tc.body)})
		if e.Reason != tc.reason || retry != tc.retryable || e.Status != tc.status {
			t.Errorf("%d: got %s/%v, want %s/%v", tc.status, e.Reason, retry, tc.reason, tc.retryable)
		}
		if strings.Contains(e.Error(), "slow down") {
			t.Error("error message carries the provider body")
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"": -1, "abc": -1, "-3": -1, "0": 0, "3": 3 * time.Second, "999999": time.Hour,
		now.Add(5 * time.Second).Format(http.TimeFormat):  5 * time.Second,
		now.Add(-5 * time.Second).Format(http.TimeFormat): 0,
	}
	for in, want := range cases {
		if got := ParseRetryAfter(in, now); got != want {
			t.Errorf("ParseRetryAfter(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestDoRetries(t *testing.T) {
	retryable := NewError("p", ReasonUnavailable, 503, "")
	t.Run("success after retry", func(t *testing.T) {
		n := 0
		attempts, err := Do(context.Background(), Policy{MaxRetries: 1, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			n++
			if n == 1 {
				return Attempt{Err: retryable, Retryable: true, RetryAfter: 0}
			}
			return Attempt{}
		})
		if err != nil || attempts != 2 {
			t.Fatalf("attempts %d err %v", attempts, err)
		}
	})
	t.Run("bounded", func(t *testing.T) {
		attempts, err := Do(context.Background(), Policy{MaxRetries: 1, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			return Attempt{Err: retryable, Retryable: true, RetryAfter: 0}
		})
		if ReasonOf(err) != ReasonUnavailable || attempts != 2 {
			t.Fatalf("attempts %d err %v", attempts, err)
		}
	})
	t.Run("not retryable", func(t *testing.T) {
		attempts, err := Do(context.Background(), Policy{MaxRetries: 3, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			return Attempt{Err: NewError("p", ReasonRefused, 200, ""), RetryAfter: -1}
		})
		if ReasonOf(err) != ReasonRefused || attempts != 1 {
			t.Fatalf("attempts %d err %v", attempts, err)
		}
	})
	t.Run("retry-after honoured", func(t *testing.T) {
		start := time.Now()
		n := 0
		attempts, err := Do(context.Background(), Policy{MaxRetries: 1, MaxRetryAfter: time.Second, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			n++
			if n == 1 {
				return Attempt{Err: NewError("p", ReasonRateLimited, 429, ""), Retryable: true, RetryAfter: 300 * time.Millisecond}
			}
			return Attempt{}
		})
		if err != nil || attempts != 2 || time.Since(start) < 300*time.Millisecond {
			t.Fatalf("attempts %d err %v after %s", attempts, err, time.Since(start))
		}
	})
	t.Run("retry-after too long", func(t *testing.T) {
		attempts, err := Do(context.Background(), Policy{MaxRetries: 1, MaxRetryAfter: time.Second, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			return Attempt{Err: NewError("p", ReasonRateLimited, 429, ""), Retryable: true, RetryAfter: 2 * time.Second}
		})
		if ReasonOf(err) != ReasonRateLimited || attempts != 1 {
			t.Fatalf("attempts %d err %v", attempts, err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		attempts, err := Do(ctx, Policy{MaxRetries: 1, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			return Attempt{Err: retryable, Retryable: true, RetryAfter: 500 * time.Millisecond}
		})
		if ReasonOf(err) != ReasonUnavailable || attempts != 1 {
			t.Fatalf("attempts %d err %v", attempts, err)
		}
	})
	t.Run("disabled", func(t *testing.T) {
		attempts, _ := Do(context.Background(), Policy{MaxRetries: -1, AttemptTimeout: time.Second}, func(context.Context) Attempt {
			return Attempt{Err: retryable, Retryable: true, RetryAfter: 0}
		})
		if attempts != 1 {
			t.Fatalf("attempts %d", attempts)
		}
	})
}

func TestParseBaseURL(t *testing.T) {
	live := []string{"api.anthropic.com"}
	ok := []struct {
		raw string
		o   BaseURLOptions
	}{
		{"https://api.anthropic.com", BaseURLOptions{Production: true, LiveHosts: live}},
		{"https://api.anthropic.com:443", BaseURLOptions{Production: true, LiveHosts: live}},
		{"https://proxy.example-corp.com/anthropic", BaseURLOptions{LiveHosts: live}},
		{"http://127.0.0.1:8090", BaseURLOptions{TestDouble: true}},
		{"http://localhost:8090", BaseURLOptions{TestDouble: true}},
		{"https://fakellm:8443", BaseURLOptions{TestDouble: true}},
	}
	for _, tc := range ok {
		if _, err := ParseBaseURL(tc.raw, tc.o); err != nil {
			t.Errorf("%s %+v: %v", tc.raw, tc.o, err)
		}
	}
	bad := []struct {
		raw  string
		o    BaseURLOptions
		want string
	}{
		{"", BaseURLOptions{}, "required"},
		{"ftp://x", BaseURLOptions{}, "absolute"},
		{"https://user:pw@api.anthropic.com", BaseURLOptions{}, "credentials"},
		{"https://api.anthropic.com/?key=1", BaseURLOptions{}, "query"},
		{"http://api.anthropic.com", BaseURLOptions{}, "https"},
		{"http://10.0.0.5:8090", BaseURLOptions{TestDouble: true}, "loopback"},
		{"https://127.0.0.1:8090", BaseURLOptions{}, "test-double"},
		{"https://fakellm.internal", BaseURLOptions{}, "test-double"},
		{"https://api.evil.com", BaseURLOptions{Production: true, LiveHosts: live}, "official"},
		{"http://127.0.0.1:8090", BaseURLOptions{TestDouble: true, Production: true}, "production refuses"},
		{"https://127.0.0.1:8090", BaseURLOptions{Production: true, LiveHosts: live}, "production refuses"},
		{"https://localhost", BaseURLOptions{Production: true, LiveHosts: live}, "production refuses"},
		{"https://fakellm-test-double.example.com", BaseURLOptions{Production: true, LiveHosts: live}, "production refuses"},
		{"https://sela-mock.dev", BaseURLOptions{Production: true, LiveHosts: live}, "production refuses"},
	}
	for _, tc := range bad {
		_, err := ParseBaseURL(tc.raw, tc.o)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %+v: err %v, want %q", tc.raw, tc.o, err, tc.want)
		}
		if err != nil && tc.raw != "" && strings.Contains(err.Error(), tc.raw) {
			t.Errorf("error echoes the URL: %v", err)
		}
	}
}

func TestTestDoubleLike(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://api.anthropic.com": false, "https://api.deepseek.com": false,
		"https://localhost": true, "https://127.0.0.1": true, "https://[::1]": true, "https://10.1.2.3": true,
		"https://fakellm": true, "https://x.test": true, "https://x.local": true, "https://stub.example.org": true,
	} {
		u, _ := url.Parse(raw)
		if got := TestDoubleLike(u); got != want {
			t.Errorf("TestDoubleLike(%s) = %v", raw, got)
		}
	}
}

func TestResponseTestDouble(t *testing.T) {
	h := http.Header{}
	if (Response{Header: h}).TestDouble() {
		t.Fatal("no header")
	}
	h.Set(HeaderTestDouble, "TRUE")
	if !(Response{Header: h}).TestDouble() {
		t.Fatal("header")
	}
	if ValidCredential("a\nb") || !ValidCredential("sk-abc") {
		t.Fatal("ValidCredential")
	}
}

// TestDoBeforeAttempt: the check runs before every retry (never before the
// first attempt) and an error stops the call without another attempt.
func TestDoBeforeAttempt(t *testing.T) {
	retryable := NewError("p", ReasonRateLimited, 429, "")
	stop := NewError("router", ReasonCanceled, 0, "preflight failed")
	checks, n := 0, 0
	attempts, err := Do(context.Background(), Policy{MaxRetries: 2, AttemptTimeout: time.Second,
		BeforeAttempt: func(context.Context) error {
			checks++
			return stop
		}}, func(context.Context) Attempt {
		n++
		return Attempt{Err: retryable, Retryable: true, RetryAfter: 0}
	})
	if err != stop || attempts != 1 || n != 1 || checks != 1 {
		t.Fatalf("attempts %d calls %d checks %d err %v", attempts, n, checks, err)
	}
	checks, n = 0, 0
	attempts, err = Do(context.Background(), Policy{MaxRetries: 2, AttemptTimeout: time.Second,
		BeforeAttempt: func(context.Context) error { checks++; return nil }}, func(context.Context) Attempt {
		n++
		if n < 3 {
			return Attempt{Err: retryable, Retryable: true, RetryAfter: 0}
		}
		return Attempt{}
	})
	if err != nil || attempts != 3 || checks != 2 {
		t.Fatalf("attempts %d checks %d err %v", attempts, checks, err)
	}
}

// TestTestDoubleURLNeverLive: a test-double base URL must be a loopback,
// local or test-double host and never a provider's live host.
func TestTestDoubleURLNeverLive(t *testing.T) {
	live := []string{"api.anthropic.com"}
	for _, raw := range []string{"https://api.anthropic.com", "https://API.anthropic.com:443", "https://proxy.example-corp.com",
		"https://api.deepseek.com"} {
		if _, err := ParseBaseURL(raw, BaseURLOptions{TestDouble: true, LiveHosts: live}); err == nil {
			t.Errorf("%s accepted as a test double", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.1:8090", "https://localhost:8443", "https://fakellm:8443", "https://shared-fakellm.example.org",
		"https://double.sela.test"} {
		if _, err := ParseBaseURL(raw, BaseURLOptions{TestDouble: true, LiveHosts: live}); err != nil {
			t.Errorf("%s refused as a test double: %v", raw, err)
		}
	}
}

// TestSecretNeverPrints: every fmt verb and JSON show the mask only, also
// where fmt cannot call Secret's methods (an unexported field, a struct
// printed by value): reflection reaches a pointer address only.
func TestSecretNeverPrints(t *testing.T) {
	const canary = "sk-SECRET-CANARY"
	type holder struct {
		Key  Secret
		Name string
	}
	type hidden struct {
		name string
		key  Secret
		cfg  struct{ APIKey Secret }
	}
	h := holder{Key: NewSecret(canary), Name: "x"}
	u := hidden{name: "sela", key: NewSecret(canary)}
	u.cfg.APIKey = NewSecret(canary)
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	for _, f := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%10s"} {
		out += fmt.Sprintf(f, h) + fmt.Sprintf(f, h.Key) + fmt.Sprintf(f, &h) + fmt.Sprintf(f, u) + fmt.Sprintf(f, &u) +
			fmt.Sprintf(f, []any{u, h})
	}
	for _, leak := range []string{canary, fmt.Sprintf("%x", canary), fmt.Sprintf("%X", canary)} {
		if strings.Contains(out, leak) {
			t.Fatalf("secret printed: %s", out)
		}
	}
	if h.Key.Reveal() != canary || !h.Key.IsSet() || NewSecret("  ").IsSet() || NewSecret("").String() != "" ||
		(Secret{}).Reveal() != "" || (Secret{}).IsSet() {
		t.Fatal("Reveal/IsSet")
	}
}

// partialServer answers status with the start of a body; with stall it
// then waits for the client to give up, otherwise it breaks the
// connection.
func partialServer(t *testing.T, status int, stall bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5000")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"type":"message","stop_reason":"refusal","content":[{"type":"text","text":"{\"claims\":[`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if stall {
			<-r.Context().Done()
			return
		}
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestPostBodyBreak: §4 and RR-10. A 200 whose body breaks off (a reset,
// or a stall cut by Do's attempt deadline) is invalid output, content
// class and never retried; a non-200 body that breaks off stays an
// availability failure; a caller's cancellation or operation deadline
// keeps its own reason.
func TestPostBodyBreak(t *testing.T) {
	hc := HTTPClient(nil)
	post := func(ctx context.Context, srv *httptest.Server) (bool, error) {
		_, retry, err := Post(ctx, hc, "p", srv.URL, http.Header{}, []byte(`{}`), 1<<16)
		return retry, err
	}
	retry, err := post(context.Background(), partialServer(t, http.StatusOK, false))
	if ReasonOf(err) != ReasonInvalidOutput || ReasonOf(err).Class() != ClassContent || retry {
		t.Fatalf("200 reset: %v retry %v", err, retry)
	}
	retry, err = post(context.Background(), partialServer(t, http.StatusServiceUnavailable, false))
	if ReasonOf(err) != ReasonUnavailable || !retry {
		t.Fatalf("503 reset: %v retry %v", err, retry)
	}
	stalled := partialServer(t, http.StatusOK, true)
	n, err := Do(context.Background(), Policy{MaxRetries: 1, AttemptTimeout: 100 * time.Millisecond}, func(actx context.Context) Attempt {
		retry, err := post(actx, stalled)
		return Attempt{Err: err, Retryable: retry, RetryAfter: -1}
	})
	if ReasonOf(err) != ReasonInvalidOutput || n != 1 {
		t.Fatalf("200 stall cut by the attempt deadline: %v after %d attempts", err, n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	n, err = Do(ctx, Policy{MaxRetries: 1, AttemptTimeout: 10 * time.Second}, func(actx context.Context) Attempt {
		retry, err := post(actx, stalled)
		return Attempt{Err: err, Retryable: retry, RetryAfter: -1}
	})
	if ReasonOf(err) != ReasonTimeout || n != 1 {
		t.Fatalf("200 stall cut by the operation deadline: %v after %d attempts", err, n)
	}
	ctx, cancel = context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	if _, err := post(ctx, stalled); ReasonOf(err) != ReasonCanceled {
		t.Fatalf("200 stall, caller canceled: %v", err)
	}
}

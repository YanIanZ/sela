// Package transport holds what Sela's verifier and fallback clients share:
// the failure taxonomy of ADR-0003 §4, a redirect-free HTTP client, bounded
// JSON POST attempts, Retry-After handling and base-URL checks that keep a
// test double out of production.
//
// Nothing in this package logs, and no error message carries a request or
// response body, a header value or a credential.
package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HeaderTestDouble is set to "true" by every Sela test double
// (internal/llm/fakellm). A live client that sees it refuses the response;
// a test-double client requires it.
const HeaderTestDouble = "X-Sela-Test-Double"

// Reason is a stable, content-free failure reason. The verification
// reasons are the unavailable_reason values of ADR-0003 §10.
type Reason string

// Failure reasons.
const (
	ReasonRefused       Reason = "refused"        // Claude stop_reason "refusal"
	ReasonTruncated     Reason = "truncated"      // max_tokens / finish_reason "length"
	ReasonInvalidOutput Reason = "invalid_output" // schema, enum, id or shape violation
	ReasonEmptyContent  Reason = "empty_content"  // no content at all
	ReasonContentFilter Reason = "content_filter" // provider content-moderation rejection
	ReasonTimeout       Reason = "timeout"
	ReasonRateLimited   Reason = "rate_limited"
	ReasonSpendCap      Reason = "spend_cap" // provider spend cap, billing or balance error
	ReasonAuth          Reason = "auth"      // 401/403: a credential problem on Sela's side
	ReasonUnavailable   Reason = "unavailable"
	ReasonModelMismatch Reason = "model_mismatch"       // response model differs from the pin
	ReasonTestDouble    Reason = "test_double_mismatch" // live client answered by a test double, or the reverse
	ReasonRejected      Reason = "rejected"             // other 4xx: a request bug, not an outage
	ReasonCanceled      Reason = "canceled"
	ReasonDisabled      Reason = "disabled"
)

// Reasons lists every failure reason, so routing tests can sweep them
// all (a reason added here is swept at every router position).
func Reasons() []Reason {
	return []Reason{ReasonRefused, ReasonTruncated, ReasonInvalidOutput, ReasonEmptyContent, ReasonContentFilter, ReasonTimeout,
		ReasonRateLimited, ReasonSpendCap, ReasonAuth, ReasonUnavailable, ReasonModelMismatch, ReasonTestDouble, ReasonRejected,
		ReasonCanceled, ReasonDisabled}
}

// Class is the failure class of ADR-0003 §4.
type Class string

// Failure classes.
const (
	// ClassAvailability failures happened before any content was received;
	// only they may lead to a generator fallback or an alternative verifier.
	ClassAvailability Class = "availability"
	// ClassContent failures (refusal, truncation, invalid or empty output,
	// moderation, any failure after content started) never do.
	ClassContent Class = "content"
	// ClassIntegrity failures (a different model or a test double) abstain
	// and alert the operator.
	ClassIntegrity Class = "integrity"
	// ClassPolicy failures (cancellation, request bugs, kill switches) end
	// the operation without fallback.
	ClassPolicy Class = "policy"
)

// Class returns the failure class of r.
func (r Reason) Class() Class {
	switch r {
	case ReasonTimeout, ReasonRateLimited, ReasonSpendCap, ReasonAuth, ReasonUnavailable:
		return ClassAvailability
	case ReasonModelMismatch, ReasonTestDouble:
		return ClassIntegrity
	case ReasonRejected, ReasonCanceled, ReasonDisabled:
		return ClassPolicy
	default:
		return ClassContent
	}
}

// Error is a provider failure. Its message names the provider, the reason
// and at most an HTTP status; it never contains a payload.
type Error struct {
	Provider string
	Reason   Reason
	Status   int
	err      error
}

// NewError returns an Error with a fixed, content-free detail message.
func NewError(provider string, reason Reason, status int, detail string) *Error {
	var err error
	if detail != "" {
		err = errors.New(detail)
	}
	return &Error{Provider: provider, Reason: reason, Status: status, err: err}
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("ai/")
	b.WriteString(e.Provider)
	b.WriteString(": ")
	b.WriteString(string(e.Reason))
	if e.Status != 0 {
		fmt.Fprintf(&b, " (HTTP %d)", e.Status)
	}
	if e.err != nil {
		b.WriteString(": ")
		b.WriteString(e.err.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.err }

// ReasonOf returns the Reason of err (ReasonUnavailable for an unknown
// error, ReasonCanceled or ReasonTimeout for a context error).
func ReasonOf(err error) Reason {
	var e *Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &e):
		return e.Reason
	case errors.Is(err, context.Canceled):
		return ReasonCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ReasonTimeout
	}
	return ReasonUnavailable
}

// HTTPClient returns a copy of hc (or a dedicated client) that never
// follows redirects, so an API key can never be forwarded to another host.
// The returned client has no overall Timeout: attempts are bounded by
// their context.
func HTTPClient(hc *http.Client) *http.Client {
	var c http.Client
	if hc != nil {
		c = *hc
	} else {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConnsPerHost = 16
		c.Transport = t
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// Response is one HTTP exchange with a bounded body.
type Response struct {
	Status     int
	Header     http.Header
	Body       []byte
	TooLarge   bool
	RetryAfter time.Duration // -1 when absent or invalid
}

// TestDouble reports whether the response carries the test-double header.
func (r Response) TestDouble() bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get(HeaderTestDouble)), "true")
}

// Post performs one JSON POST and reads at most maxBody bytes of the body
// (TooLarge reports more). A transport failure before a response, or while
// reading a non-200 body, returns a *Error with ReasonTimeout or
// ReasonUnavailable and retryable true when another attempt may help
// (never for a caller cancellation or a certificate failure).
//
// A 200 response whose body breaks off (a reset connection, or a stall cut
// by the attempt deadline) is content class (ADR-0003 §4: any failure
// after the first content is content, RR-10): ReasonInvalidOutput, never
// retried, so no generator fallback and no alternative verifier follow.
// Only when the caller's own context ended (a cancellation, or the
// operation deadline as opposed to Do's attempt deadline) does it return
// that context's reason instead.
func Post(ctx context.Context, hc *http.Client, provider, target string, header http.Header, body []byte, maxBody int) (Response, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return Response{}, false, NewError(provider, ReasonRejected, 0, "request cannot be built")
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return Response{}, retryableTransport(ctx, err), transportError(ctx, provider, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBody)+1))
	out := Response{Status: resp.StatusCode, Header: resp.Header, RetryAfter: ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
	if err != nil {
		// The body broke off (a stall cut by the attempt deadline, or a
		// reset connection).
		if resp.StatusCode == http.StatusOK && !callerEnded(ctx) {
			// Content had started: content class, final.
			return out, false, NewError(provider, ReasonInvalidOutput, resp.StatusCode, "response body broke off")
		}
		return out, retryableTransport(ctx, err), transportError(ctx, provider, err)
	}
	if len(raw) > maxBody {
		out.TooLarge = true
		raw = raw[:maxBody]
	}
	out.Body = raw
	return out, false, nil
}

// errAttemptDeadline is the cause Do gives an attempt's own deadline, so
// Post can tell it from the caller's context ending.
var errAttemptDeadline = errors.New("transport: attempt deadline")

// callerEnded reports whether ctx ended for a reason other than Do's
// attempt deadline: the caller canceled, or the operation deadline passed.
func callerEnded(ctx context.Context) bool {
	return ctx.Err() != nil && !errors.Is(context.Cause(ctx), errAttemptDeadline)
}

func retryableTransport(ctx context.Context, err error) bool {
	var certErr *tls.CertificateVerificationError
	return ctx.Err() == nil && !errors.As(err, &certErr)
}

// transportError maps a client error. The caller's context error wins;
// otherwise a deadline is a timeout and anything else a connection
// failure. The message is fixed (transport errors can embed URLs).
func transportError(ctx context.Context, provider string, err error) *Error {
	if cerr := ctx.Err(); cerr != nil {
		if errors.Is(cerr, context.DeadlineExceeded) {
			return NewError(provider, ReasonTimeout, 0, "deadline exceeded")
		}
		return NewError(provider, ReasonCanceled, 0, "canceled")
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return NewError(provider, ReasonTimeout, 0, "attempt timed out")
	}
	return NewError(provider, ReasonUnavailable, 0, "connection failed")
}

// spendCapMarkers identify a spend-cap or balance failure inside an error
// body. Anthropic documents a 429 "enforced_spend_limit_reached" (where the
// token appears is unverified, so type, code and message are checked for
// the exact token); DeepSeek uses 402 for an insufficient balance.
var spendCapMarkers = []string{"enforced_spend_limit_reached", "billing_error", "insufficient_balance"}

// errorBody is the shared shape of Anthropic and OpenAI-style error bodies.
// Only the type and code are compared with fixed tokens; the message is
// scanned for the spend-cap token and then dropped.
type errorBody struct {
	Error struct {
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
	} `json:"error"`
}

func spendCap(body []byte) bool {
	var eb errorBody
	if json.Unmarshal(body, &eb) != nil {
		return false
	}
	code := strings.Trim(string(eb.Error.Code), `"`)
	for _, m := range spendCapMarkers {
		if eb.Error.Type == m || code == m || strings.Contains(eb.Error.Message, m) {
			return true
		}
	}
	return false
}

// StatusError classifies a non-200 response and reports whether another
// attempt may help. 429 (unless it is a spend cap), 500, 502, 503, 504 and
// 529 (Anthropic "overloaded") are retryable availability failures; 401 and
// 403 are auth; 402 and a spend-cap 429 are spend_cap; other statuses are
// request bugs (rejected).
func StatusError(provider string, r Response) (*Error, bool) {
	code := r.Status
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return NewError(provider, ReasonAuth, code, ""), false
	case code == http.StatusPaymentRequired:
		return NewError(provider, ReasonSpendCap, code, ""), false
	case code == http.StatusTooManyRequests:
		if spendCap(r.Body) {
			return NewError(provider, ReasonSpendCap, code, ""), false
		}
		return NewError(provider, ReasonRateLimited, code, ""), true
	case code == http.StatusInternalServerError, code == http.StatusBadGateway, code == http.StatusServiceUnavailable,
		code == http.StatusGatewayTimeout, code == 529:
		return NewError(provider, ReasonUnavailable, code, ""), true
	case code >= 500:
		return NewError(provider, ReasonUnavailable, code, ""), false
	}
	return NewError(provider, ReasonRejected, code, ""), false
}

// Retry policy defaults (ADR-0003 §3.2).
const (
	DefaultMaxRetries    = 1
	DefaultMaxRetryAfter = 10 * time.Second
	backoffBase          = 500 * time.Millisecond
	backoffCap           = 4 * time.Second
)

// Policy bounds the attempts of one call.
type Policy struct {
	// MaxRetries is the number of retries after the first attempt; a
	// negative value disables retries.
	MaxRetries int
	// MaxRetryAfter: a server Retry-After longer than this ends the
	// retries (the call fails with the availability reason).
	MaxRetryAfter time.Duration
	// AttemptTimeout bounds each attempt.
	AttemptTimeout time.Duration
	// BeforeAttempt, when set, runs right before every attempt after the
	// first (after the backoff or Retry-After wait). The router passes its
	// consent and age-band check here, so a withdrawal during the wait
	// stops the retry (ADR-0003 RR-2, §8.2: "every call that has not yet
	// started"). An error ends the call with that error and no attempt.
	BeforeAttempt func(ctx context.Context) error
}

// Attempt is the outcome of one attempt, as seen by Do.
type Attempt struct {
	Err        error
	Retryable  bool
	RetryAfter time.Duration // -1 when the server sent none
}

// Do calls attempt until it succeeds, fails without being retryable, the
// retries are used up, the server asks for a longer delay than
// MaxRetryAfter, or the next attempt would start after the context
// deadline. It returns the number of attempts made and the last error.
func Do(ctx context.Context, p Policy, attempt func(ctx context.Context) Attempt) (int, error) {
	retries := p.MaxRetries
	if retries < 0 {
		retries = 0
	}
	maxAfter := p.MaxRetryAfter
	if maxAfter <= 0 {
		maxAfter = DefaultMaxRetryAfter
	}
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return n - 1, contextReason(err)
		}
		actx, cancel := context.WithTimeoutCause(ctx, p.AttemptTimeout, errAttemptDeadline)
		a := attempt(actx)
		cancel()
		if a.Err == nil {
			return n, nil
		}
		if !a.Retryable || n > retries {
			return n, a.Err
		}
		delay := Backoff(n)
		if a.RetryAfter > maxAfter {
			return n, a.Err
		}
		if a.RetryAfter >= 0 {
			delay = a.RetryAfter
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay+time.Millisecond {
			return n, a.Err
		}
		if err := sleep(ctx, delay); err != nil {
			return n, contextReason(err)
		}
		if p.BeforeAttempt != nil {
			if err := p.BeforeAttempt(ctx); err != nil {
				return n, err
			}
		}
	}
}

func contextReason(err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return NewError("router", ReasonTimeout, 0, "deadline exceeded")
	}
	return NewError("router", ReasonCanceled, 0, "canceled")
}

// Backoff returns the delay before retry n (1-based): exponential from
// 500 ms, capped at 4 s, with equal jitter.
func Backoff(n int) time.Duration {
	d := backoffCap
	if n < 4 {
		d = min(backoffBase<<(n-1), backoffCap)
	}
	half := d / 2
	return half + rand.N(half+1)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ParseRetryAfter parses a Retry-After value (delay-seconds or HTTP-date)
// and returns -1 when it is absent or invalid. Values above an hour are
// clamped to an hour.
func ParseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return -1
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs < 0 {
			return -1
		}
		if secs > int64(time.Hour/time.Second) {
			return time.Hour
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return min(max(t.Sub(now), 0), time.Hour)
	}
	return -1
}

// testDoubleSuffixes are host suffixes that never name a live provider.
var testDoubleSuffixes = []string{".localhost", ".local", ".test", ".internal", ".invalid", ".example", ".lan", ".home.arpa"}

// TestDoubleLike reports whether u points somewhere a live provider never
// is: a loopback, private, link-local or unspecified address, an IP
// literal, "localhost", a reserved or local suffix, or a host whose name
// mentions a test double or fakellm.
func TestDoubleLike(u *url.URL) bool {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	if !strings.Contains(host, ".") {
		return true // a single-label name (docker service, /etc/hosts entry)
	}
	for _, s := range testDoubleSuffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	for _, w := range []string{"fakellm", "test-double", "testdouble", "test_double", "mock", "stub"} {
		if strings.Contains(host, w) {
			return true
		}
	}
	return false
}

// IsLoopback reports whether host is "localhost" or a loopback IP literal.
func IsLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// BaseURLOptions selects the checks of ParseBaseURL.
type BaseURLOptions struct {
	// TestDouble: the URL names a labelled test double. It may use http
	// only for a loopback host. Production refuses it.
	TestDouble bool
	// Production refuses test doubles and any URL that looks like one, and
	// requires a host in LiveHosts.
	Production bool
	// LiveHosts are the provider's official API hosts.
	LiveHosts []string
}

// ParseBaseURL validates a provider base URL. Errors name the problem but
// never echo the value.
func ParseBaseURL(raw string, o BaseURLOptions) (*url.URL, error) {
	u, err := url.Parse(raw)
	switch {
	case raw == "":
		return nil, errors.New("base URL is required")
	case err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http"):
		return nil, errors.New("base URL must be an absolute http(s) URL")
	case u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.ForceQuery:
		return nil, errors.New("base URL must not contain credentials, a query or a fragment")
	}
	if o.Production && (o.TestDouble || TestDoubleLike(u)) {
		return nil, errors.New("production refuses a test-double base URL")
	}
	if o.TestDouble {
		if u.Scheme == "http" && !IsLoopback(u.Hostname()) {
			return nil, errors.New("a test-double base URL may use http only for a loopback host")
		}
		// A test double never runs at a live provider host or at an
		// ordinary public host: user text and a key must not reach a real
		// provider under a test-double label.
		for _, h := range o.LiveHosts {
			if strings.EqualFold(u.Hostname(), h) {
				return nil, errors.New("a test-double base URL must not name the provider's live host")
			}
		}
		if !TestDoubleLike(u) {
			return nil, errors.New("a test-double base URL must be a loopback, local or test-double host")
		}
		return u, nil
	}
	if u.Scheme != "https" {
		return nil, errors.New("a live base URL must use https")
	}
	if TestDoubleLike(u) {
		return nil, errors.New("a live base URL must not point at a local or test-double host")
	}
	if o.Production {
		ok := false
		for _, h := range o.LiveHosts {
			if strings.EqualFold(u.Hostname(), h) && (u.Port() == "" || u.Port() == "443") {
				ok = true
			}
		}
		if !ok {
			return nil, errors.New("production allows only the provider's official API host")
		}
	}
	return u, nil
}

// JoinPath appends path to the base URL, keeping any base path prefix.
func JoinPath(base *url.URL, path string) string {
	return strings.TrimRight(base.String(), "/") + path
}

// ValidCredential reports whether v can be sent in a header.
func ValidCredential(v string) bool {
	return !strings.ContainsAny(v, "\r\n\x00")
}

// Secret is a credential (an API key or a workspace id). It formats as
// "[redacted]" with every fmt verb (or "" when empty) and marshals to
// JSON the same way, so printing or encoding a configuration never shows
// it. Reveal returns the value for the one header that carries it.
//
// The value sits behind a pointer, so it stays hidden where fmt cannot
// call these methods: a Secret in an unexported field, or in a struct
// printed by value whose Format has a pointer receiver, is printed by
// reflection as a pointer address only. Make one with NewSecret; the zero
// Secret is empty.
type Secret struct{ v *string }

// NewSecret returns a Secret holding v (the zero Secret when v is empty).
func NewSecret(v string) Secret {
	if v == "" {
		return Secret{}
	}
	return Secret{v: &v}
}

const redacted = "[redacted]"

// Reveal returns the secret value. Use it only to set a request header.
func (s Secret) Reveal() string {
	if s.v == nil {
		return ""
	}
	return *s.v
}

// IsSet reports whether the secret has a non-blank value.
func (s Secret) IsSet() bool { return strings.TrimSpace(s.Reveal()) != "" }

func (s Secret) mask() string {
	if s.Reveal() == "" {
		return ""
	}
	return redacted
}

// String returns "[redacted]" (or "" when empty).
func (s Secret) String() string { return s.mask() }

// GoString returns the quoted mask, for %#v.
func (s Secret) GoString() string { return strconv.Quote(s.mask()) }

// Format masks the value for every verb (%v, %+v, %#v, %s, %q, %x, %d ...).
func (s Secret) Format(f fmt.State, verb rune) {
	switch verb {
	case 'q':
		_, _ = io.WriteString(f, strconv.Quote(s.mask()))
	case 'v':
		if f.Flag('#') {
			_, _ = io.WriteString(f, s.GoString())
			return
		}
		_, _ = io.WriteString(f, s.mask())
	default:
		_, _ = io.WriteString(f, s.mask())
	}
}

// MarshalJSON encodes the mask, never the value.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(s.mask()) }

// MarshalText encodes the mask, never the value.
func (s Secret) MarshalText() ([]byte, error) { return []byte(s.mask()), nil }

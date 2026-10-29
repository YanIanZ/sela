package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Retry policy. Backoff doubles from backoffBase up to backoffCap with equal
// jitter; a Retry-After of up to maxRetryAfter seconds replaces it, and a
// longer one ends the retries.
const (
	backoffBase   = 500 * time.Millisecond
	backoffCap    = 4 * time.Second
	maxRetryAfter = 10 * time.Second
	// maxDrainBytes of an error body are read (and discarded) so the
	// connection can be reused. Error bodies are never surfaced.
	maxDrainBytes = 4 << 10
)

var schemaNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// client is the OpenAI-compatible chat-completions provider used for both
// ModeModal (vLLM on Modal) and ModeTestDouble (cmd/fakellm).
type client struct {
	cfg       Config
	hc        *http.Client
	chatURL   string
	healthURL string
	modelsURL string
	endpoint  string
}

// Info describes the configured provider; it never contains credentials.
func (c *client) Info() Info {
	return Info{
		Mode:        c.cfg.Mode,
		ServedModel: c.cfg.ServedModel,
		ModelID:     c.cfg.ModelID,
		Revision:    c.cfg.Revision,
		Endpoint:    c.endpoint,
	}
}

// chatRequest is the OpenAI-compatible request body.
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	TopK        *int      `json:"top_k,omitempty"`
	Seed        *int      `json:"seed,omitempty"`
	// ChatTemplateKwargs carries reasoning_strength to the vLLM chat
	// template. Meta Model API uses ReasoningEffort instead.
	ChatTemplateKwargs map[string]string `json:"chat_template_kwargs,omitempty"`
	ReasoningEffort    string            `json:"reasoning_effort,omitempty"`
	Stream             bool              `json:"stream"`
	StreamOptions      streamOptions     `json:"stream_options"`
	ResponseFormat     *responseFormat   `json:"response_format,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

// buildBody validates req and encodes the request body. Errors are local
// and classified as KindRejected.
func (c *client) buildBody(req Request) ([]byte, error) {
	reject := func(msg string) error { return &Error{Kind: KindRejected, Err: errors.New(msg)} }
	if len(req.Messages) == 0 {
		return nil, reject("request has no messages")
	}
	for _, m := range req.Messages {
		if m.Role != RoleSystem && m.Role != RoleUser {
			return nil, reject("only system and user messages are allowed")
		}
	}
	if req.MaxTokens < 0 {
		return nil, reject("max tokens must not be negative")
	}
	body := chatRequest{
		Model:         c.cfg.ServedModel,
		Messages:      req.Messages,
		MaxTokens:     req.MaxTokens,
		Seed:          req.Seed,
		Stream:        true,
		StreamOptions: streamOptions{IncludeUsage: true},
	}
	if sp := req.Sampling; sp != nil {
		if math.IsNaN(sp.Temperature) || sp.Temperature < 0 || sp.Temperature > 2 {
			return nil, reject("temperature must be within [0, 2]")
		}
		if math.IsNaN(sp.TopP) || sp.TopP <= 0 || sp.TopP > 1 {
			return nil, reject("top_p must be within (0, 1]")
		}
		if sp.TopK < 0 {
			return nil, reject("top_k must not be negative")
		}
		temp, topP := sp.Temperature, sp.TopP
		body.Temperature, body.TopP = &temp, &topP
		// Meta Model API has no top_k parameter; leave it out there.
		if sp.TopK > 0 && c.cfg.Mode != ModeMeta {
			topK := sp.TopK
			body.TopK = &topK
		}
	}
	if c.cfg.Mode == ModeMeta {
		body.ReasoningEffort = c.cfg.ReasoningEffort
	} else {
		switch req.ReasoningStrength {
		case "":
		case ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh:
			body.ChatTemplateKwargs = map[string]string{"reasoning_strength": req.ReasoningStrength}
		default:
			return nil, reject("reasoning strength must be low, medium, high or xhigh")
		}
	}
	if body.MaxTokens == 0 {
		body.MaxTokens = DefaultMaxTokens
	}
	if req.JSONSchema != "" {
		if !json.Valid([]byte(req.JSONSchema)) {
			return nil, reject("JSON schema is not valid JSON")
		}
		name := req.SchemaName
		if name == "" {
			name = "sela"
		}
		if !schemaNameRe.MatchString(name) {
			return nil, reject("schema name must match [A-Za-z0-9_-]{1,64}")
		}
		body.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: jsonSchema{Name: name, Schema: json.RawMessage(req.JSONSchema), Strict: true},
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, reject("request cannot be encoded")
	}
	return raw, nil
}

// setAuth adds the configured credentials to r.
func (c *client) setAuth(r *http.Request) {
	if c.cfg.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	if c.cfg.Mode == ModeModal && c.cfg.ModalKey != "" {
		r.Header.Set("Modal-Key", c.cfg.ModalKey)
	}
	if c.cfg.Mode == ModeModal && c.cfg.ModalSecret != "" {
		r.Header.Set("Modal-Secret", c.cfg.ModalSecret)
	}
	r.Header.Set("User-Agent", "sela-llm")
}

// attemptState carries what one attempt observed into the retry decision.
type attemptState struct {
	status     int           // HTTP status of the response, 0 if none
	retryable  bool          // the failure may be retried (before any content)
	retryAfter time.Duration // server-requested delay, -1 when absent
	started    bool          // a content delta was delivered to onDelta
}

// Complete streams one chat completion. See Provider.Complete.
//
// Retries happen only before the first content delta, for connection
// failures (including an attempt that timed out before any response
// arrived), 429, 502, 503 and 504, and once for 500; at most
// Config.MaxRetries times. The delay is exponential backoff with jitter
// from 500 ms (cap 4 s), or the server's Retry-After when it is at most
// 10 s (a longer Retry-After ends the retries). A retry that would start
// after the caller's deadline is not attempted.
//
// On failure the returned Result carries only metadata (Attempts, Latency,
// FirstToken), never partial text. Errors are *Error values whose messages
// contain no provider payload; an error returned by onDelta is returned
// unchanged.
func (c *client) Complete(ctx context.Context, req Request, onDelta func(delta string) error) (Result, error) {
	start := time.Now()
	res, status, err := c.complete(ctx, req, onDelta, start)
	res.Latency = time.Since(start)
	if err != nil {
		res = Result{Attempts: res.Attempts, Latency: res.Latency, FirstToken: res.FirstToken}
	}
	c.observe(req.Operation, res, status, err)
	return res, err
}

func (c *client) complete(ctx context.Context, req Request, onDelta func(string) error, start time.Time) (Result, int, error) {
	var res Result
	body, err := c.buildBody(req)
	if err != nil {
		return res, 0, err
	}
	if err := ctx.Err(); err != nil {
		return res, 0, contextError(err)
	}
	firstToken := time.Duration(0)
	deltaFn := func(d string) error {
		if firstToken == 0 {
			firstToken = time.Since(start)
		}
		if onDelta != nil {
			return onDelta(d)
		}
		return nil
	}
	retried500 := false
	for attempt := 1; ; attempt++ {
		res.Attempts = attempt
		var st attemptState
		out, err := c.attempt(ctx, body, deltaFn, &st)
		res.FirstToken = firstToken
		if err == nil {
			out.Attempts, out.FirstToken = attempt, firstToken
			return out, st.status, nil
		}
		if st.started || !st.retryable || attempt > c.cfg.MaxRetries {
			return res, st.status, err
		}
		if st.status == http.StatusInternalServerError {
			if retried500 {
				return res, st.status, err
			}
			retried500 = true
		}
		delay := backoff(attempt)
		if st.retryAfter > maxRetryAfter {
			return res, st.status, err
		}
		if st.retryAfter >= 0 {
			delay = st.retryAfter
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
			return res, st.status, err
		}
		if err := sleep(ctx, delay); err != nil {
			return res, st.status, contextError(err)
		}
	}
}

// attempt performs one HTTP request and reads its stream.
func (c *client) attempt(ctx context.Context, body []byte, onDelta func(string) error, st *attemptState) (Result, error) {
	st.retryAfter = -1
	actx, cancel := context.WithTimeout(ctx, c.cfg.AttemptTimeout)
	defer cancel()
	hreq, err := http.NewRequestWithContext(actx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return Result{}, &Error{Kind: KindRejected, Err: errors.New("request cannot be built")}
	}
	c.setAuth(hreq)
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")

	resp, err := c.hc.Do(hreq)
	if err != nil {
		// No response arrived: a connection failure, or an attempt timeout
		// while waiting for headers (e.g. during a cold start). Both may be
		// retried; a caller cancellation or deadline, or a certificate that
		// does not verify (a configuration error), may not.
		var certErr *tls.CertificateVerificationError
		st.retryable = ctx.Err() == nil && !errors.As(err, &certErr)
		return Result{}, transportError(ctx, actx, err)
	}
	defer resp.Body.Close()
	st.status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		_, _ = io.CopyN(io.Discard, resp.Body, maxDrainBytes)
		st.retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		e := statusError(resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			st.retryable = true
		}
		return Result{}, e
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if mt, _, err := mime.ParseMediaType(ct); err != nil || mt != "text/event-stream" {
			return Result{}, &Error{Kind: KindProtocol, Status: resp.StatusCode, Err: errors.New("response is not an event stream")}
		}
	}
	return c.readStream(ctx, actx, resp.Body, onDelta, st)
}

// statusError maps a non-200 HTTP status to an Error that names only the
// status code.
func statusError(code int) *Error {
	kind := KindProtocol
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		kind = KindAuth
	case code == http.StatusTooManyRequests:
		kind = KindRateLimited
	case code >= 400 && code < 500:
		kind = KindRejected
	case code >= 500:
		kind = KindUnavailable
	}
	return &Error{Kind: kind, Status: code, Err: fmt.Errorf("HTTP status %d", code)}
}

// transportError classifies a failure that happened while sending the
// request or reading the response. The caller's context takes precedence;
// then the attempt deadline; anything else is a connection failure. The
// message is generic (the underlying error stays available to errors.Is
// and errors.As).
func transportError(ctx, actx context.Context, err error) *Error {
	if cerr := ctx.Err(); cerr != nil {
		return contextError(cerr)
	}
	if errors.Is(actx.Err(), context.DeadlineExceeded) {
		return &Error{Kind: KindTimeout, Err: &causeError{msg: "attempt timed out", err: context.DeadlineExceeded}}
	}
	return &Error{Kind: KindUnavailable, Err: &causeError{msg: "connection failed", err: err}}
}

// contextError maps a caller context error to KindCanceled or KindTimeout.
func contextError(err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: KindTimeout, Err: context.DeadlineExceeded}
	}
	return &Error{Kind: KindCanceled, Err: context.Canceled}
}

// causeError keeps an underlying error for errors.Is/As while exposing only
// a fixed message (transport errors can embed URLs and peer addresses).
type causeError struct {
	msg string
	err error
}

func (e *causeError) Error() string { return e.msg }
func (e *causeError) Unwrap() error { return e.err }

// backoff returns the delay before retry number n (1-based): exponential
// from backoffBase, capped at backoffCap, with equal jitter.
func backoff(n int) time.Duration {
	d := backoffCap
	if n < 4 {
		d = min(backoffBase<<(n-1), backoffCap)
	}
	half := d / 2
	return half + rand.N(half+1)
}

// parseRetryAfter parses a Retry-After header (delay-seconds or HTTP-date).
// It returns -1 when the header is absent or invalid.
func parseRetryAfter(v string, now time.Time) time.Duration {
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
		return max(t.Sub(now), 0)
	}
	return -1
}

// sleep waits for d or until ctx is done.
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

// observe emits the metadata-only event for one Complete call.
func (c *client) observe(op string, res Result, status int, err error) {
	if c.cfg.Observer == nil {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = string(outcomeKind(err))
	}
	c.cfg.Observer(Event{
		Operation:        op,
		Outcome:          outcome,
		Status:           status,
		Attempts:         res.Attempts,
		LatencyMS:        res.Latency.Milliseconds(),
		FirstTokenMS:     res.FirstToken.Milliseconds(),
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
	})
}

// outcomeKind classifies err for the observer. An error returned by the
// caller's onDelta callback is reported as KindCanceled: the caller stopped
// the generation.
func outcomeKind(err error) Kind {
	var e *Error
	if errors.As(err, &e) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return KindOf(err)
	}
	return KindCanceled
}

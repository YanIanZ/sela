// Package metaverifier is Sela's Meta verifier client (SPEC V36): one
// Chat Completions call per verifier request to the Meta Model API, with
// grammar-constrained JSON output (response_format json_schema) and
// reasoning effort "minimal".
//
// The verifier is always a different Meta model from the generator, so a
// model never verifies its own output. There is no fallback and no second
// verifier: a refusal, truncation, schema or enum violation, model
// mismatch or transport failure is an error, and nothing is ever treated
// as a pass.
//
// The client never logs. The API key comes from configuration and appears
// only in the Authorization header; errors and Identity never contain it.
package metaverifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
)

// Provider is the provider name used in errors and provenance.
const Provider = "meta_verifier"

// Defaults (Meta Model API, observed 2026-10-31).
const (
	DefaultBaseURL        = "https://api.meta.ai/v1"
	DefaultModel          = "muse-wonder-1.0"
	DefaultMaxTokens      = 3000
	DefaultAttemptTimeout = 30 * time.Second
	// LiveHost is the only host production accepts.
	LiveHost = "api.meta.ai"
)

// Test-double identity (cmd/fakellm). A test-double client sends and
// expects TestDoubleModel, so its reports can never pass as the Meta
// verifier's.
const (
	TestDoubleModel = "test-double-not-meta-verifier"
	TestDoubleLabel = "TEST DOUBLE — not the Meta verifier"
)

// Config configures the client.
type Config struct {
	// APIKey is sent as "Authorization: Bearer". Required unless
	// TestDouble; never sent to a test double.
	APIKey transport.Secret
	// TestDoubleAPIKey is the only key a test-double client sends
	// (SELA_TEST_DOUBLE_API_KEY, optional). It is never sent to Meta.
	TestDoubleAPIKey transport.Secret
	// BaseURL defaults to DefaultBaseURL (DefaultTestDoubleURL for a test
	// double). The client appends /chat/completions.
	BaseURL string
	// Model defaults to DefaultModel (TestDoubleModel for a test double).
	Model string
	// MaxTokens bounds the report (default DefaultMaxTokens).
	MaxTokens int
	// TestDouble selects a labelled test double.
	TestDouble bool
	// Production refuses test doubles and non-official hosts.
	Production bool
	// AttemptTimeout bounds one attempt (default DefaultAttemptTimeout).
	AttemptTimeout time.Duration
	// MaxRetries after the first attempt (default 1; negative disables).
	MaxRetries int
	// MaxRetryAfter: a longer Retry-After ends the retries (default 10 s).
	MaxRetryAfter time.Duration
	// HTTPClient, when set, is copied without its redirect policy.
	HTTPClient *http.Client
}

// DefaultTestDoubleURL is cmd/fakellm's default address.
const DefaultTestDoubleURL = "http://127.0.0.1:8090"

// Client is a Meta verifier. It is safe for concurrent use.
type Client struct {
	cfg      Config
	hc       *http.Client
	endpoint string
}

// New validates cfg and returns a client. Errors name fields, never
// values.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
		if cfg.TestDouble {
			cfg.BaseURL = DefaultTestDoubleURL
		}
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
		if cfg.TestDouble {
			cfg.Model = TestDoubleModel
		}
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = DefaultMaxTokens
	}
	if cfg.AttemptTimeout == 0 {
		cfg.AttemptTimeout = DefaultAttemptTimeout
	}
	switch {
	case cfg.MaxRetries == 0:
		cfg.MaxRetries = transport.DefaultMaxRetries
	case cfg.MaxRetries < 0:
		cfg.MaxRetries = -1
	}
	if cfg.MaxRetryAfter == 0 {
		cfg.MaxRetryAfter = transport.DefaultMaxRetryAfter
	}
	var problems []string
	base, err := transport.ParseBaseURL(cfg.BaseURL, transport.BaseURLOptions{
		TestDouble: cfg.TestDouble, Production: cfg.Production, LiveHosts: []string{LiveHost},
	})
	if err != nil {
		problems = append(problems, "BaseURL: "+err.Error())
	}
	if !cfg.TestDouble && !cfg.APIKey.IsSet() {
		problems = append(problems, "APIKey is required")
	}
	if !transport.ValidCredential(cfg.APIKey.Reveal()) || !transport.ValidCredential(cfg.TestDoubleAPIKey.Reveal()) {
		problems = append(problems, "APIKey and TestDoubleAPIKey must be single-line values")
	}
	if cfg.TestDouble != (cfg.Model == TestDoubleModel) {
		problems = append(problems, "Model must be the test-double name exactly when TestDouble is set")
	}
	if cfg.MaxTokens < 256 || cfg.MaxTokens > 16000 {
		problems = append(problems, "MaxTokens must be within [256, 16000]")
	}
	if cfg.AttemptTimeout < 0 || cfg.MaxRetryAfter < 0 {
		problems = append(problems, "durations must not be negative")
	}
	if len(problems) > 0 {
		return nil, errors.New("metaverifier: invalid config: " + strings.Join(problems, "; "))
	}
	return &Client{cfg: cfg, hc: transport.HTTPClient(cfg.HTTPClient), endpoint: transport.JoinPath(base, "/chat/completions")}, nil
}

// Identity describes the verifier without secrets.
func (c *Client) Identity() verifier.Identity {
	id := verifier.Identity{Provider: Provider, Model: c.cfg.Model, TestDouble: c.cfg.TestDouble}
	if c.cfg.TestDouble {
		id.Label = TestDoubleLabel
	}
	return id
}

// Format prints the client without its configuration's secrets, for every
// fmt verb (the unexported config would otherwise be printed raw).
func (c *Client) Format(f fmt.State, _ rune) {
	if c == nil {
		_, _ = io.WriteString(f, "metaverifier.Client(nil)")
		return
	}
	_, _ = fmt.Fprintf(f, "metaverifier.Client{model=%s testDouble=%t key=%s}", c.cfg.Model, c.cfg.TestDouble,
		setOrUnset(c.cfg.APIKey))
}

func setOrUnset(s transport.Secret) string {
	if s.IsSet() {
		return "set"
	}
	return "unset"
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

// chatRequest is the complete request body; there is deliberately no
// field for sampling, tools or metadata.
type chatRequest struct {
	Model           string         `json:"model"`
	MaxTokens       int            `json:"max_tokens"`
	Messages        []message      `json:"messages"`
	ReasoningEffort string         `json:"reasoning_effort"`
	ResponseFormat  responseFormat `json:"response_format"`
}

// chatResponse decodes only what the verifier checks. Reasoning text, if a
// response ever carried any, is not decoded.
type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content *string `json:"content"`
			Refusal *string `json:"refusal"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Verify sends one request and validates the report.
//
// Retries: one retry by default, only for a connection failure, 429 (not a
// spend cap), 500, 502, 503, 504 or 529, waiting the server's Retry-After
// when it is at most MaxRetryAfter and never past the context deadline. A
// 200 response is never retried: a refusal, truncation or invalid report is
// final.
func (c *Client) Verify(ctx context.Context, req verifier.Request, opts verifier.CallOptions) (verifier.Result, error) {
	start := time.Now()
	res := verifier.Result{Identity: c.Identity()}
	if err := req.Validate(); err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request violates VC-1")
	}
	user, err := verifier.UserMessage(req)
	if err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request cannot be encoded")
	}
	body, err := json.Marshal(chatRequest{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		Messages: []message{
			{Role: "system", Content: verifier.Instructions},
			{Role: "user", Content: user},
		},
		ReasoningEffort: "minimal",
		ResponseFormat: responseFormat{
			Type:       "json_schema",
			JSONSchema: jsonSchema{Name: "verifier_report", Strict: true, Schema: verifier.Schema()},
		},
	})
	if err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request cannot be encoded")
	}
	header := http.Header{}
	switch {
	case c.cfg.TestDouble:
		// A test double never receives the live key.
		if c.cfg.TestDoubleAPIKey.IsSet() {
			header.Set("Authorization", "Bearer "+c.cfg.TestDoubleAPIKey.Reveal())
		}
	case c.cfg.APIKey.IsSet():
		header.Set("Authorization", "Bearer "+c.cfg.APIKey.Reveal())
	}
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("User-Agent", "sela-ai-verifier")

	policy := transport.Policy{MaxRetries: c.cfg.MaxRetries, MaxRetryAfter: c.cfg.MaxRetryAfter, AttemptTimeout: c.cfg.AttemptTimeout}
	if opts.MaxRetryAfter > 0 {
		policy.MaxRetryAfter = opts.MaxRetryAfter
	}
	if opts.AttemptTimeout > 0 {
		policy.AttemptTimeout = opts.AttemptTimeout
	}
	policy.BeforeAttempt = opts.BeforeAttempt
	attempts, err := transport.Do(ctx, policy, func(actx context.Context) transport.Attempt {
		resp, retryable, err := transport.Post(actx, c.hc, Provider, c.endpoint, header, body, verifier.MaxResponseBytes)
		res.Status = resp.Status
		if err != nil {
			return transport.Attempt{Err: err, Retryable: retryable, RetryAfter: -1}
		}
		if resp.Status != http.StatusOK {
			e, retry := transport.StatusError(Provider, resp)
			if c.cfg.TestDouble != resp.TestDouble() && e.Reason != transport.ReasonRejected {
				// A live key answered by a test double (or the reverse) is
				// an integrity failure, not an outage.
				e = transport.NewError(Provider, transport.ReasonTestDouble, resp.Status, "test-double label mismatch")
				retry = false
			}
			return transport.Attempt{Err: e, Retryable: retry, RetryAfter: resp.RetryAfter}
		}
		rep, err := c.decode(resp, req, &res)
		if err != nil {
			return transport.Attempt{Err: err, RetryAfter: -1}
		}
		res.Report = rep
		return transport.Attempt{}
	})
	res.Attempts = attempts
	res.Latency = time.Since(start)
	if err != nil {
		res.Report = verifier.Report{}
		return res, err
	}
	return res, nil
}

// decode applies the report checks to one 200 response.
func (c *Client) decode(resp transport.Response, req verifier.Request, res *verifier.Result) (verifier.Report, error) {
	fail := func(reason transport.Reason, detail string) (verifier.Report, error) {
		return verifier.Report{}, transport.NewError(Provider, reason, resp.Status, detail)
	}
	if c.cfg.TestDouble != resp.TestDouble() {
		return fail(transport.ReasonTestDouble, "test-double label mismatch")
	}
	if resp.TooLarge {
		return fail(transport.ReasonInvalidOutput, "response body above 64 KiB")
	}
	var m chatResponse
	if err := json.Unmarshal(resp.Body, &m); err != nil {
		return fail(transport.ReasonInvalidOutput, "response is not a Chat Completions object")
	}
	res.InputTokens, res.OutputTokens = m.Usage.PromptTokens, m.Usage.CompletionTokens
	if m.Model != c.cfg.Model {
		return fail(transport.ReasonModelMismatch, "response model differs from the configured model")
	}
	res.ServedModel = m.Model
	if len(m.Choices) != 1 {
		return fail(transport.ReasonInvalidOutput, "response must carry exactly one choice")
	}
	choice := m.Choices[0]
	if choice.Message.Refusal != nil && strings.TrimSpace(*choice.Message.Refusal) != "" {
		res.RefusalCategory = "other"
		return fail(transport.ReasonRefused, "response refusal")
	}
	switch choice.FinishReason {
	case "stop":
	case "content_filter":
		res.RefusalCategory = "other"
		return fail(transport.ReasonRefused, "finish_reason content_filter")
	case "length":
		return fail(transport.ReasonTruncated, "output truncated")
	default:
		return fail(transport.ReasonInvalidOutput, "unexpected finish_reason")
	}
	if choice.Message.Content == nil || strings.TrimSpace(*choice.Message.Content) == "" {
		return fail(transport.ReasonInvalidOutput, "empty report")
	}
	rep, err := verifier.Parse(*choice.Message.Content, req)
	if err != nil {
		return fail(transport.ReasonInvalidOutput, "report violates VC-6")
	}
	return rep, nil
}

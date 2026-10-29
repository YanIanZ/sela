// Package deepseek is Sela's DeepSeek client (ADR-0003 RR-4, RR-9, VC-5):
// the OpenAI-compatible chat-completions API at https://api.deepseek.com
// with model "deepseek-flash" (served by DeepSeek-V4.1-Flash-1017; the
// response "model" field is recorded because the alias can move).
//
// Every request disables thinking, asks for response_format json_object
// (the only JSON mode; there is no schema enforcement, so Go validates
// everything) and sends no user_id and no sampling parameters. Any
// reasoning_content in a response is never decoded, stored or logged.
//
// The same client serves as the fallback generator (Generate) and, behind
// a flag that defaults to off, as the alternative verifier (Verify). The
// router decides when either is permitted; this package only calls.
package deepseek

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
const Provider = "deepseek"

// Defaults (observed provider facts, 2026-10-26; ADR-0003 §2, VC-5).
const (
	DefaultBaseURL         = "https://api.deepseek.com"
	DefaultModel           = "deepseek-flash"
	DefaultExpectedVersion = "DeepSeek-V4.1-Flash-1017"
	DefaultAttemptTimeout  = 60 * time.Second
	DefaultMaxTokens       = 2000
	DefaultMaxOutputBytes  = 32 << 10
	// LiveHost is the only host production accepts.
	LiveHost = "api.deepseek.com"
	// DefaultTestDoubleURL is cmd/fakellm's default address.
	DefaultTestDoubleURL = "http://127.0.0.1:8090"
	// maxBodyBytes bounds a response body (content plus envelope).
	maxBodyBytes = 256 << 10
)

// Test-double identity (cmd/fakellm).
const (
	TestDoubleModel = "test-double-not-deepseek"
	TestDoubleLabel = "TEST DOUBLE — not DeepSeek"
	// AlternativeLabel marks every verdict of the alternative verifier.
	AlternativeLabel = "alternative verifier (DeepSeek)"
)

// Config configures the client.
type Config struct {
	// APIKey is sent as "Authorization: Bearer". Required unless
	// TestDouble; never sent to a test double.
	APIKey transport.Secret
	// TestDoubleAPIKey is the only key a test-double client sends
	// (SELA_TEST_DOUBLE_API_KEY, optional). It is never sent to DeepSeek.
	TestDoubleAPIKey transport.Secret
	// BaseURL defaults to DefaultBaseURL; the client appends
	// /chat/completions.
	BaseURL string
	// Model defaults to DefaultModel (TestDoubleModel for a test double).
	Model string
	// ExpectedVersion is the evaluated served version. Until the response
	// field is calibrated (B-9) a response model must equal Model or
	// ExpectedVersion (case-insensitively); with PinServedVersion it must
	// equal ExpectedVersion.
	ExpectedVersion  string
	PinServedVersion bool
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
	// MaxOutputBytes bounds the generated content (default 32 KiB).
	MaxOutputBytes int
	// HTTPClient, when set, is copied without its redirect policy.
	HTTPClient *http.Client
}

// Client is a DeepSeek client. It is safe for concurrent use.
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
	if cfg.ExpectedVersion == "" && !cfg.TestDouble {
		cfg.ExpectedVersion = DefaultExpectedVersion
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
	if cfg.MaxOutputBytes == 0 {
		cfg.MaxOutputBytes = DefaultMaxOutputBytes
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
	if cfg.PinServedVersion && cfg.ExpectedVersion == "" {
		problems = append(problems, "PinServedVersion requires ExpectedVersion")
	}
	if cfg.AttemptTimeout < 0 || cfg.MaxRetryAfter < 0 || cfg.MaxOutputBytes < 0 {
		problems = append(problems, "durations and sizes must not be negative")
	}
	if len(problems) > 0 {
		return nil, errors.New("deepseek: invalid config: " + strings.Join(problems, "; "))
	}
	return &Client{cfg: cfg, hc: transport.HTTPClient(cfg.HTTPClient), endpoint: transport.JoinPath(base, "/chat/completions")}, nil
}

// Model returns the configured model id.
func (c *Client) Model() string { return c.cfg.Model }

// TestDouble reports whether the client talks to a labelled test double.
func (c *Client) TestDouble() bool { return c.cfg.TestDouble }

// Format prints the client without its configuration's secrets, for every
// fmt verb (the unexported config would otherwise be printed raw).
func (c *Client) Format(f fmt.State, _ rune) {
	if c == nil {
		_, _ = io.WriteString(f, "deepseek.Client(nil)")
		return
	}
	key := "unset"
	if c.cfg.APIKey.IsSet() {
		key = "set"
	}
	_, _ = fmt.Fprintf(f, "deepseek.Client{model=%s testDouble=%t key=%s}", c.cfg.Model, c.cfg.TestDouble, key)
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type typed struct {
	Type string `json:"type"`
}

// chatRequest is the complete request body; there is deliberately no
// field for user_id, sampling, tools or streaming.
type chatRequest struct {
	Model          string    `json:"model"`
	Messages       []message `json:"messages"`
	ResponseFormat typed     `json:"response_format"`
	Thinking       typed     `json:"thinking"`
	MaxTokens      int       `json:"max_tokens"`
	Stream         bool      `json:"stream"`
}

// chatResponse decodes only content, finish reason, model and usage;
// message.reasoning_content is never decoded.
type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string  `json:"role"`
			Content *string `json:"content"`
		} `json:"message"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Completion is one validated completion.
type Completion struct {
	// Text is the content: a JSON object to be validated by the caller.
	Text         string
	ServedModel  string
	FinishReason string
	TestDouble   bool
	InputTokens  int
	OutputTokens int
	Attempts     int
	Status       int
	Latency      time.Duration
}

// Request is one generation request.
type Request struct {
	System    string
	User      string
	MaxTokens int
	// MaxRetryAfter and AttemptTimeout override the client defaults.
	MaxRetryAfter  time.Duration
	AttemptTimeout time.Duration
	// BeforeAttempt, when set, runs right before every retry; an error
	// stops the call (the router's consent and age-band check, RR-2).
	BeforeAttempt func(ctx context.Context) error
}

// Complete sends one non-streaming chat completion and returns its content
// unparsed. Retries follow the same rules as the Claude client: one retry
// for a connection failure, 429 (not a balance error), 5xx; never after a
// 200 response.
func (c *Client) Complete(ctx context.Context, req Request) (Completion, error) {
	start := time.Now()
	out := Completion{TestDouble: c.cfg.TestDouble}
	if strings.TrimSpace(req.System) == "" || strings.TrimSpace(req.User) == "" {
		return out, transport.NewError(Provider, transport.ReasonRejected, 0, "system and user text are required")
	}
	if !strings.Contains(strings.ToLower(req.System+req.User), "json") {
		// JSON mode requires the word "json" in the prompt.
		return out, transport.NewError(Provider, transport.ReasonRejected, 0, "prompt must mention json")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	body, err := json.Marshal(chatRequest{
		Model:          c.cfg.Model,
		Messages:       []message{{Role: "system", Content: req.System}, {Role: "user", Content: req.User}},
		ResponseFormat: typed{Type: "json_object"},
		Thinking:       typed{Type: "disabled"},
		MaxTokens:      maxTokens,
	})
	if err != nil {
		return out, transport.NewError(Provider, transport.ReasonRejected, 0, "request cannot be encoded")
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
	header.Set("User-Agent", "sela-ai")
	policy := transport.Policy{MaxRetries: c.cfg.MaxRetries, MaxRetryAfter: c.cfg.MaxRetryAfter, AttemptTimeout: c.cfg.AttemptTimeout}
	if req.MaxRetryAfter > 0 {
		policy.MaxRetryAfter = req.MaxRetryAfter
	}
	if req.AttemptTimeout > 0 {
		policy.AttemptTimeout = req.AttemptTimeout
	}
	policy.BeforeAttempt = req.BeforeAttempt
	attempts, err := transport.Do(ctx, policy, func(actx context.Context) transport.Attempt {
		resp, retryable, err := transport.Post(actx, c.hc, Provider, c.endpoint, header, body, maxBodyBytes)
		out.Status = resp.Status
		if err != nil {
			return transport.Attempt{Err: err, Retryable: retryable, RetryAfter: -1}
		}
		if resp.Status != http.StatusOK {
			e, retry := transport.StatusError(Provider, resp)
			if c.cfg.TestDouble != resp.TestDouble() && e.Reason != transport.ReasonRejected {
				e = transport.NewError(Provider, transport.ReasonTestDouble, resp.Status, "test-double label mismatch")
				retry = false
			}
			return transport.Attempt{Err: e, Retryable: retry, RetryAfter: resp.RetryAfter}
		}
		if err := c.decode(resp, &out); err != nil {
			return transport.Attempt{Err: err, RetryAfter: -1}
		}
		return transport.Attempt{}
	})
	out.Attempts = attempts
	out.Latency = time.Since(start)
	if err != nil {
		out.Text = ""
		return out, err
	}
	return out, nil
}

func (c *Client) decode(resp transport.Response, out *Completion) error {
	fail := func(reason transport.Reason, detail string) error {
		out.Text = ""
		return transport.NewError(Provider, reason, resp.Status, detail)
	}
	if c.cfg.TestDouble != resp.TestDouble() {
		return fail(transport.ReasonTestDouble, "test-double label mismatch")
	}
	if resp.TooLarge {
		return fail(transport.ReasonInvalidOutput, "response body too large")
	}
	var r chatResponse
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return fail(transport.ReasonInvalidOutput, "response is not a chat completion")
	}
	if r.Usage != nil {
		out.InputTokens, out.OutputTokens = r.Usage.PromptTokens, r.Usage.CompletionTokens
	}
	if !c.modelOK(r.Model) {
		return fail(transport.ReasonModelMismatch, "response model differs from the configured model")
	}
	out.ServedModel = r.Model
	if len(r.Choices) != 1 || r.Choices[0].Index != 0 || r.Choices[0].FinishReason == nil {
		return fail(transport.ReasonInvalidOutput, "response must have exactly one finished choice")
	}
	ch := r.Choices[0]
	switch *ch.FinishReason {
	case "stop":
	case "length", "insufficient_system_resource":
		return fail(transport.ReasonTruncated, "output truncated")
	case "content_filter":
		return fail(transport.ReasonContentFilter, "content filtered by the provider")
	default:
		return fail(transport.ReasonInvalidOutput, "unexpected finish_reason")
	}
	out.FinishReason = "stop"
	if ch.Message.Content == nil || strings.TrimSpace(*ch.Message.Content) == "" {
		// JSON mode "may occasionally return empty content".
		return fail(transport.ReasonEmptyContent, "empty content")
	}
	if len(*ch.Message.Content) > c.cfg.MaxOutputBytes {
		return fail(transport.ReasonInvalidOutput, "output exceeds the configured limit")
	}
	out.Text = *ch.Message.Content
	return nil
}

// modelOK checks the response model against the configuration.
func (c *Client) modelOK(model string) bool {
	if c.cfg.TestDouble {
		return model == TestDoubleModel
	}
	if c.cfg.PinServedVersion {
		return strings.EqualFold(model, c.cfg.ExpectedVersion)
	}
	return strings.EqualFold(model, c.cfg.Model) || (c.cfg.ExpectedVersion != "" && strings.EqualFold(model, c.cfg.ExpectedVersion))
}

// Verifier is the alternative-verifier view of a Client (VC-5). Its
// verdicts carry AlternativeLabel (or TestDoubleLabel).
type Verifier struct{ c *Client }

// AsVerifier returns the alternative-verifier view of c.
func (c *Client) AsVerifier() *Verifier { return &Verifier{c: c} }

// Format prints the verifier without the client's secrets, for every fmt
// verb (fmt would otherwise print the unexported client through a bad
// verb such as %s).
func (v *Verifier) Format(f fmt.State, verb rune) {
	if v == nil {
		_, _ = io.WriteString(f, "deepseek.Verifier(nil)")
		return
	}
	_, _ = io.WriteString(f, "deepseek.Verifier{")
	v.c.Format(f, verb)
	_, _ = io.WriteString(f, "}")
}

// Identity describes the alternative verifier.
func (v *Verifier) Identity() verifier.Identity {
	id := verifier.Identity{Provider: Provider, Model: v.c.cfg.Model, TestDouble: v.c.cfg.TestDouble, Label: AlternativeLabel}
	if v.c.cfg.TestDouble {
		id.Label = TestDoubleLabel + " (" + AlternativeLabel + ")"
	}
	return id
}

// Verify sends one VC-5 request: the instructions carry the VC-2 schema as
// text, thinking is disabled, and the content is validated exactly as a
// Claude report (VC-6). Empty content is verification_unavailable.
func (v *Verifier) Verify(ctx context.Context, req verifier.Request, opts verifier.CallOptions) (verifier.Result, error) {
	res := verifier.Result{Identity: v.Identity()}
	if err := req.Validate(); err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request violates VC-1")
	}
	user, err := verifier.UserMessage(req)
	if err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request cannot be encoded")
	}
	comp, err := v.c.Complete(ctx, Request{
		System: verifier.DeepSeekInstructions(), User: user, MaxTokens: DefaultMaxTokens,
		MaxRetryAfter: opts.MaxRetryAfter, AttemptTimeout: opts.AttemptTimeout, BeforeAttempt: opts.BeforeAttempt,
	})
	res.ServedModel, res.InputTokens, res.OutputTokens = comp.ServedModel, comp.InputTokens, comp.OutputTokens
	res.Attempts, res.Status, res.Latency = comp.Attempts, comp.Status, comp.Latency
	if err != nil {
		return res, err
	}
	rep, err := verifier.Parse(comp.Text, req)
	if err != nil {
		return res, transport.NewError(Provider, transport.ReasonInvalidOutput, comp.Status, "report violates VC-6")
	}
	res.Report = rep
	return res, nil
}

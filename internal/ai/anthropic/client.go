// Package anthropic is Sela's Claude verifier client (ADR-0003 VC-4,
// VC-6, VC-7): one Messages API call per verifier request, with
// grammar-constrained JSON output (output_config.format json_schema),
// thinking "between_tools" and effort "low".
//
// It never sends sampling parameters, assistant prefill, tools,
// tool_choice, Citations, metadata.user_id or the server-side "fallbacks"
// option (a fallback would route around a refusal, RR-6). A refusal,
// max_tokens truncation, schema or enum violation, model mismatch or
// transport failure is an error; nothing is ever treated as a pass.
//
// The client never logs. The API key comes from configuration and appears
// only in the Authorization header; errors and Identity never contain it.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
)

// Provider is the provider name used in errors and provenance.
const Provider = "anthropic"

// Defaults (observed provider facts, 2026-10-26; ADR-0003 §2, VC-4).
const (
	DefaultBaseURL        = "https://api.anthropic.com"
	DefaultVersion        = "2026-10-21"
	DefaultModel          = "claude-sonnet-5-5"
	DefaultInferenceGeo   = "us"
	DefaultEffort         = "low"
	DefaultMaxTokens      = 2000
	DefaultAttemptTimeout = 30 * time.Second
	// LiveHost is the only host production accepts.
	LiveHost = "api.anthropic.com"
)

// Test-double identity (cmd/fakellm). A test-double client sends and
// expects TestDoubleModel, so its reports can never pass as Claude's.
const (
	TestDoubleModel = "test-double-not-claude"
	TestDoubleLabel = "TEST DOUBLE — not Claude"
)

// Config configures the client.
type Config struct {
	// APIKey is a workspace-scoped server key, sent as
	// "Authorization: Bearer". Required unless TestDouble; never sent to a
	// test double.
	APIKey transport.Secret
	// TestDoubleAPIKey is the only key a test-double client sends
	// (SELA_TEST_DOUBLE_API_KEY, optional). It is never sent to Anthropic.
	TestDoubleAPIKey transport.Secret
	// BaseURL defaults to DefaultBaseURL (DefaultTestDoubleURL for a test
	// double). The client appends /v1/messages.
	BaseURL string
	// WorkspaceID is sent as anthropic-workspace-id when set (never to a
	// test double); only a key that is not scoped to a workspace needs it.
	WorkspaceID transport.Secret
	// Version is the anthropic-version header (default DefaultVersion).
	Version string
	// Model defaults to DefaultModel (TestDoubleModel for a test double).
	Model string
	// InferenceGeo is "us" (default; ×1.1 price, makes the "processed in
	// the US" disclosure true) or "global".
	InferenceGeo string
	// Effort is "low" (default), "medium" or "high"; between_tools thinking
	// is accepted only at high or below.
	Effort string
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

// Client is a Claude verifier. It is safe for concurrent use.
type Client struct {
	cfg      Config
	hc       *http.Client
	endpoint string
}

var versionRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// New validates cfg and returns a client. Errors name fields, never
// values.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
		if cfg.TestDouble {
			cfg.BaseURL = DefaultTestDoubleURL
		}
	}
	if cfg.Version == "" {
		cfg.Version = DefaultVersion
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
		if cfg.TestDouble {
			cfg.Model = TestDoubleModel
		}
	}
	if cfg.InferenceGeo == "" {
		cfg.InferenceGeo = DefaultInferenceGeo
	}
	if cfg.Effort == "" {
		cfg.Effort = DefaultEffort
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
	if !transport.ValidCredential(cfg.APIKey.Reveal()) || !transport.ValidCredential(cfg.WorkspaceID.Reveal()) ||
		!transport.ValidCredential(cfg.TestDoubleAPIKey.Reveal()) || len(cfg.WorkspaceID.Reveal()) > 128 {
		problems = append(problems, "APIKey, TestDoubleAPIKey and WorkspaceID must be single-line values")
	}
	if !versionRe.MatchString(cfg.Version) {
		problems = append(problems, "Version must be a YYYY-MM-DD API version")
	}
	if cfg.TestDouble != (cfg.Model == TestDoubleModel) {
		problems = append(problems, "Model must be the test-double name exactly when TestDouble is set")
	}
	if cfg.InferenceGeo != "us" && cfg.InferenceGeo != "global" {
		problems = append(problems, "InferenceGeo must be us or global")
	}
	if cfg.Effort != "low" && cfg.Effort != "medium" && cfg.Effort != "high" {
		problems = append(problems, "Effort must be low, medium or high (between_tools thinking)")
	}
	if cfg.MaxTokens < 256 || cfg.MaxTokens > 16000 {
		problems = append(problems, "MaxTokens must be within [256, 16000]")
	}
	if cfg.AttemptTimeout < 0 || cfg.MaxRetryAfter < 0 {
		problems = append(problems, "durations must not be negative")
	}
	if len(problems) > 0 {
		return nil, errors.New("anthropic: invalid config: " + strings.Join(problems, "; "))
	}
	return &Client{cfg: cfg, hc: transport.HTTPClient(cfg.HTTPClient), endpoint: transport.JoinPath(base, "/v1/messages")}, nil
}

// Identity describes the verifier without secrets.
func (c *Client) Identity() verifier.Identity {
	id := verifier.Identity{Provider: Provider, Model: c.cfg.Model, TestDouble: c.cfg.TestDouble, InferenceGeo: c.cfg.InferenceGeo}
	if c.cfg.TestDouble {
		id.Label = TestDoubleLabel
	}
	return id
}

// Format prints the client without its configuration's secrets, for every
// fmt verb (the unexported config would otherwise be printed raw).
func (c *Client) Format(f fmt.State, _ rune) {
	if c == nil {
		_, _ = io.WriteString(f, "anthropic.Client(nil)")
		return
	}
	_, _ = fmt.Fprintf(f, "anthropic.Client{model=%s testDouble=%t geo=%s key=%s}", c.cfg.Model, c.cfg.TestDouble, c.cfg.InferenceGeo,
		setOrUnset(c.cfg.APIKey))
}

func setOrUnset(s transport.Secret) string {
	if s.IsSet() {
		return "set"
	}
	return "unset"
}

// APIVersion is the anthropic-version the client sends.
func (c *Client) APIVersion() string { return c.cfg.Version }

// InferenceGeo is the inference_geo the client sends.
func (c *Client) InferenceGeo() string { return c.cfg.InferenceGeo }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinking struct {
	Type string `json:"type"`
}

type format struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

type outputConfig struct {
	Effort string `json:"effort"`
	Format format `json:"format"`
}

// messagesRequest is the complete request body; there is deliberately no
// field for sampling, tools, metadata or fallbacks.
type messagesRequest struct {
	Model        string       `json:"model"`
	MaxTokens    int          `json:"max_tokens"`
	System       string       `json:"system"`
	Messages     []message    `json:"messages"`
	Thinking     thinking     `json:"thinking"`
	OutputConfig outputConfig `json:"output_config"`
	InferenceGeo string       `json:"inference_geo"`
}

// messagesResponse decodes only what VC-6 checks. Thinking text, if a
// response ever carried any, is not decoded.
type messagesResponse struct {
	Type        string `json:"type"`
	Model       string `json:"model"`
	StopReason  string `json:"stop_reason"`
	StopDetails *struct {
		Category *string `json:"category"`
	} `json:"stop_details"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// knownRefusalCategories are the observed stop_details categories; any
// other value is recorded as "other" so provider strings never reach a
// record.
var knownRefusalCategories = map[string]bool{
	"cyber": true, "bio": true, "frontier_llm": true, "reasoning_extraction": true, "general_harms": true,
}

// Verify sends one VC-4 request and validates the report (VC-6).
//
// Retries (ADR-0003 §3.2): one retry by default, only for a connection
// failure, 429 (not a spend cap), 500, 502, 503, 504 or 529, waiting the
// server's Retry-After when it is at most MaxRetryAfter (a longer one ends
// the retries) and never past the context deadline. A 200 response is
// never retried: a refusal, truncation or invalid report is final (RR-10).
func (c *Client) Verify(ctx context.Context, req verifier.Request, opts verifier.CallOptions) (verifier.Result, error) {
	start := time.Now()
	res := verifier.Result{Identity: c.Identity(), APIVersion: c.cfg.Version}
	if err := req.Validate(); err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request violates VC-1")
	}
	user, err := verifier.UserMessage(req)
	if err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request cannot be encoded")
	}
	body, err := json.Marshal(messagesRequest{
		Model:        c.cfg.Model,
		MaxTokens:    c.cfg.MaxTokens,
		System:       verifier.Instructions,
		Messages:     []message{{Role: "user", Content: user}},
		Thinking:     thinking{Type: "between_tools"},
		OutputConfig: outputConfig{Effort: c.cfg.Effort, Format: format{Type: "json_schema", Schema: verifier.Schema()}},
		InferenceGeo: c.cfg.InferenceGeo,
	})
	if err != nil {
		return res, transport.NewError(Provider, transport.ReasonRejected, 0, "request cannot be encoded")
	}
	header := http.Header{}
	switch {
	case c.cfg.TestDouble:
		// A test double never receives the live key or workspace.
		if c.cfg.TestDoubleAPIKey.IsSet() {
			header.Set("Authorization", "Bearer "+c.cfg.TestDoubleAPIKey.Reveal())
		}
	case c.cfg.APIKey.IsSet():
		header.Set("Authorization", "Bearer "+c.cfg.APIKey.Reveal())
	}
	header.Set("anthropic-version", c.cfg.Version)
	if !c.cfg.TestDouble && c.cfg.WorkspaceID.IsSet() {
		header.Set("anthropic-workspace-id", c.cfg.WorkspaceID.Reveal())
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

// decode applies VC-6 to one 200 response.
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
	var m messagesResponse
	if err := json.Unmarshal(resp.Body, &m); err != nil {
		return fail(transport.ReasonInvalidOutput, "response is not a Messages API object")
	}
	res.InputTokens, res.OutputTokens = m.Usage.InputTokens, m.Usage.OutputTokens
	if m.Model != c.cfg.Model {
		return fail(transport.ReasonModelMismatch, "response model differs from the configured model")
	}
	res.ServedModel = m.Model
	switch m.StopReason {
	case "end_turn":
	case "refusal":
		res.RefusalCategory = "other"
		if m.StopDetails != nil && m.StopDetails.Category != nil && knownRefusalCategories[*m.StopDetails.Category] {
			res.RefusalCategory = *m.StopDetails.Category
		}
		return fail(transport.ReasonRefused, "stop_reason refusal")
	case "max_tokens", "model_context_window_exceeded":
		return fail(transport.ReasonTruncated, "output truncated")
	default:
		return fail(transport.ReasonInvalidOutput, "unexpected stop_reason")
	}
	if len(m.Content) != 1 || m.Content[0].Type != "text" {
		return fail(transport.ReasonInvalidOutput, "response must be exactly one text block")
	}
	if strings.TrimSpace(m.Content[0].Text) == "" {
		return fail(transport.ReasonInvalidOutput, "empty report")
	}
	rep, err := verifier.Parse(m.Content[0].Text, req)
	if err != nil {
		return fail(transport.ReasonInvalidOutput, "report violates VC-6")
	}
	return rep, nil
}

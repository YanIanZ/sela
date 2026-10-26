// Package llm is Sela's only model-provider boundary.
//
// Production inference uses a Meta open-weight model served by a vLLM
// OpenAI-compatible server on Modal (see modal/ and docs/adr). The same HTTP
// client also talks to cmd/fakellm, a deterministic test double that is
// always reported as ModeTestDouble and never as Meta inference. The v2
// runtime has no fallback to any other provider: the opt-in DeepSeek fallback
// of SPEC C13/V19-V21 exists only as the standalone prototype in
// prototypes/generationfallback and is not wired in. When the configured
// provider fails, callers get a typed error and show an honest unavailable
// state (SPEC V9).
//
// The package never logs prompts, model output or credentials. Metadata
// (operation, status, latency, token counts) is reported through Observer.
package llm

import (
	"context"
	"errors"
	"time"
)

// Mode identifies what is behind the provider. Only ModeModal can produce
// live_meta provenance.
type Mode string

const (
	ModeModal      Mode = "modal"
	ModeTestDouble Mode = "test_double"
	ModeDisabled   Mode = "disabled"
)

// Role of a chat message. Only system and user roles are sent; the model's
// previous output is never replayed as an assistant message.
type Role string

const (
	RoleSystem Role = "system"
	RoleUser   Role = "user"
)

// Message is one chat message.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Request is one bounded completion request.
type Request struct {
	// Operation is a short metadata label ("ledger_draft", "opening_draft").
	Operation string
	Messages  []Message
	// JSONSchema, when set, requests guided decoding with this schema
	// (OpenAI-compatible response_format json_schema). The caller still
	// validates the output.
	JSONSchema string
	SchemaName string
	// MaxTokens bounds generated tokens. For a reasoning model this includes
	// the hidden reasoning, so it needs headroom.
	MaxTokens int
	// Sampling, when nil, leaves temperature, top_p and top_k to the
	// server's generation config (the model publisher's defaults). Meta
	// advises against greedy decoding for Muse Glimmer (greedy runs are not
	// reproducible either), so Sela sends no sampling override and compares
	// repeated runs statistically.
	Sampling *Sampling
	// ReasoningStrength is sent as chat_template_kwargs.reasoning_strength
	// ("low", "medium", "high" or "xhigh"); empty leaves the server default.
	ReasoningStrength string
	Seed              *int
}

// Sampling overrides the server's sampling defaults. TopK 0 is not sent.
type Sampling struct {
	Temperature float64
	TopP        float64
	TopK        int
}

// Reasoning strengths understood by the Muse Glimmer chat template.
const (
	ReasoningLow    = "low"
	ReasoningMedium = "medium"
	ReasoningHigh   = "high"
	ReasoningXHigh  = "xhigh"
)

// Result is a completed generation.
type Result struct {
	Text string
	// FinishReason is the server's finish_reason when it is a known value
	// ("stop", "length", "abort", "content_filter", "tool_calls" or
	// "function_call"), otherwise "other".
	FinishReason string
	// Model is the model name reported by the server; checked against the
	// configured served-model name.
	Model            string
	PromptTokens     int
	CompletionTokens int
	Attempts         int
	Latency          time.Duration
	FirstToken       time.Duration
}

// Info describes the configured provider without secrets.
type Info struct {
	Mode Mode `json:"mode"`
	// ServedModel is the name the server exposes (vLLM --served-model-name).
	ServedModel string `json:"servedModel,omitempty"`
	// ModelID and Revision are the pinned Hugging Face repository and commit.
	ModelID  string `json:"modelId,omitempty"`
	Revision string `json:"revision,omitempty"`
	// Endpoint is the host only; never a credential or full URL with query.
	Endpoint string `json:"endpoint,omitempty"`
}

// Health is a provider health observation.
type Health struct {
	State     string    `json:"state"` // "ok" | "unavailable" | "disabled"
	ModelID   string    `json:"modelId,omitempty"`
	Revision  string    `json:"revision,omitempty"`
	Latency   int64     `json:"latencyMs"`
	CheckedAt time.Time `json:"checkedAt"`
}

// Provider generates text. Implementations must be safe for concurrent use.
type Provider interface {
	Info() Info
	// Complete streams generation deltas to onDelta (may be nil) and returns
	// the full result. If onDelta returns an error, generation is aborted and
	// that error is returned. Retries happen only before the first delta.
	Complete(ctx context.Context, req Request, onDelta func(delta string) error) (Result, error)
	// Health performs a real request to the provider's health endpoint. On
	// Modal this can start a GPU container; call it only on explicit demand.
	Health(ctx context.Context) Health
}

// Kind classifies provider errors into stable application codes.
type Kind string

const (
	KindDisabled      Kind = "disabled"
	KindUnavailable   Kind = "unavailable"    // connection failure, 5xx after retries
	KindTimeout       Kind = "timeout"        // deadline exceeded
	KindCanceled      Kind = "canceled"       // caller canceled
	KindAuth          Kind = "auth"           // 401/403 from provider
	KindRateLimited   Kind = "rate_limited"   // 429 after retries
	KindRejected      Kind = "rejected"       // other 4xx (bad request, context too long)
	KindModelMismatch Kind = "model_mismatch" // server reported a different model
	KindOutputTooLong Kind = "output_too_large"
	KindProtocol      Kind = "protocol" // malformed stream/response
)

// Error is a provider error. Message never contains provider payloads.
type Error struct {
	Kind   Kind
	Status int // HTTP status when relevant
	Err    error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return "llm: " + string(e.Kind) + ": " + e.Err.Error()
	}
	return "llm: " + string(e.Kind)
}

func (e *Error) Unwrap() error { return e.Err }

// KindOf extracts the error kind (KindUnavailable for unknown errors).
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	if errors.Is(err, context.Canceled) {
		return KindCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return KindTimeout
	}
	return KindUnavailable
}

// Observer receives metadata-only events. It must not block.
type Observer func(Event)

// Event is a metadata-only provider event.
type Event struct {
	Operation        string
	Outcome          string // "ok" or an error Kind
	Status           int
	Attempts         int
	LatencyMS        int64
	FirstTokenMS     int64
	PromptTokens     int
	CompletionTokens int
}

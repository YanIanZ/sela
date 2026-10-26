// Package fakellm is Sela's deterministic OpenAI-compatible TEST DOUBLE,
// served by cmd/fakellm and used only by tests, CI and local development.
//
// It is not a model and never claims to be Meta or Llama inference: every
// response names the model ServedModel ("test-double-not-meta") and carries
// the header "X-Sela-Test-Double: true". Its output is computed from the
// request alone, so identical requests get identical responses.
//
// The handler reads the JSON object between the lines "<<<SELA_INPUT_JSON"
// and "SELA_INPUT_JSON>>>" in the last user message and answers the
// "ledger_draft" and "opening_draft" tasks by quoting the input verbatim.
// Faults (see Fault) reproduce provider and model failures for tests. It
// never logs or prints request content.
//
// The same handler also serves the TEST DOUBLES of internal/ai's other
// providers (providers.go): an Anthropic-compatible POST /v1/messages that
// reports the model "test-double-not-claude", and a DeepSeek-compatible
// POST /chat/completions that reports "test-double-not-deepseek". Both
// carry the test-double header, answer the internal/ai generation and
// verifier inputs deterministically and inject the failures listed in
// ProviderFault. They are never Claude or DeepSeek.
package fakellm

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Identity reported by the test double.
const (
	ServedModel = "test-double-not-meta"
	ModelID     = "test-double/not-meta"
	Revision    = "test-double"
	// WrongModel is the model name reported under FaultWrongModel.
	WrongModel = "llama-impostor"
)

// Headers.
const (
	// HeaderTestDouble is set to "true" on every response.
	HeaderTestDouble = "X-Sela-Test-Double"
	// HeaderFault selects a fault per request when Config.AllowFaultHeader
	// is set.
	HeaderFault = "X-Fake-Fault"
)

// Input markers; they match internal/journey's prompt wrapping.
const (
	InputOpenMarker  = "<<<SELA_INPUT_JSON"
	InputCloseMarker = "SELA_INPUT_JSON>>>"
)

// Streaming parameters.
const (
	// ChunkBytes is the approximate size of one streamed content delta.
	ChunkBytes = 16
	// OversizeBytes is the content size produced by FaultOversize.
	OversizeBytes = 100 << 10
	// DefaultSlowDelay is the delay before the first byte under FaultSlow.
	DefaultSlowDelay = 5 * time.Second

	maxRequestBytes = 1 << 20
	// created is a fixed timestamp so responses are deterministic.
	created = 1767225600
)

// Fault selects an injected failure.
type Fault string

// Faults. Content faults change the generated JSON; transport faults change
// the HTTP exchange.
const (
	FaultNone Fault = "none"
	// FaultMalformedJSON returns the first half of the normal content.
	FaultMalformedJSON Fault = "malformed_json"
	// FaultFabricatedSource cites segment "s999", which does not exist
	// (opening: adds a line with facts from no item).
	FaultFabricatedSource Fault = "fabricated_source"
	// FaultNegationFlip reverses the polarity of every claim text (opening
	// line): it drops each negator, keeping the negated word of a fused one
	// ("gabisa" becomes "bisa"), or, in a text without a negator, inserts
	// "tidak" ("not" for English) after the first word. The text therefore
	// always changes; the negated flag still describes the quote.
	FaultNegationFlip Fault = "negation_flip"
	// FaultDiagnosis appends a diagnosis label to the first claim text.
	FaultDiagnosis Fault = "diagnosis"
	// FaultInventedNumber appends a duration that nobody stated.
	FaultInventedNumber Fault = "invented_number"
	// FaultPromptInjectionEcho replaces the first text with an injected
	// instruction.
	FaultPromptInjectionEcho Fault = "prompt_injection_echo"
	// FaultWrongModel reports WrongModel as the model.
	FaultWrongModel Fault = "wrong_model"
	// FaultHTTP503, FaultHTTP429 and FaultHTTP401 answer with that status
	// (429 with "Retry-After: 1").
	FaultHTTP503 Fault = "http_503"
	FaultHTTP429 Fault = "http_429"
	FaultHTTP401 Fault = "http_401"
	// FaultSlow waits Config.SlowDelay before sending anything.
	FaultSlow Fault = "slow"
	// FaultStall sends the first content chunk, then waits for the client
	// to go away.
	FaultStall Fault = "stall"
	// FaultOversize streams OversizeBytes of content.
	FaultOversize Fault = "oversize"
	// FaultTruncatedStream ends the stream without a finish reason and
	// without [DONE] (non-stream: half a JSON body).
	FaultTruncatedStream Fault = "truncated_stream"
)

var allFaults = []Fault{
	FaultNone, FaultMalformedJSON, FaultFabricatedSource, FaultNegationFlip, FaultDiagnosis,
	FaultInventedNumber, FaultPromptInjectionEcho, FaultWrongModel, FaultHTTP503, FaultHTTP429,
	FaultHTTP401, FaultSlow, FaultStall, FaultOversize, FaultTruncatedStream,
}

// Faults lists every supported fault.
func Faults() []Fault { return append([]Fault(nil), allFaults...) }

// ParseFault parses a fault name; "" means FaultNone.
func ParseFault(s string) (Fault, error) {
	if s == "" {
		return FaultNone, nil
	}
	for _, f := range allFaults {
		if string(f) == s {
			return f, nil
		}
	}
	return "", errors.New("fakellm: unknown fault")
}

// Config configures the handler.
type Config struct {
	// APIKey, when set, is required as "Authorization: Bearer" on /v1/*
	// (like vLLM, /health stays open).
	APIKey string
	// Latency is a delay before each streamed content chunk.
	Latency time.Duration
	// Fault applies to every chat completion ("" means FaultNone).
	Fault Fault
	// AllowFaultHeader lets a request choose its fault with HeaderFault.
	// Without it, a request carrying that header is rejected with 400.
	AllowFaultHeader bool
	// SlowDelay is the FaultSlow delay; zero means DefaultSlowDelay. It is
	// also the delay of ProviderFaultTimeout.
	SlowDelay time.Duration
	// ProviderFault applies to every request on the Anthropic-compatible
	// (POST /v1/messages) and DeepSeek-compatible (POST /chat/completions)
	// routes ("" means ProviderFaultNone). AllowFaultHeader also lets a
	// request choose one with HeaderProviderFault.
	ProviderFault ProviderFault
}

type handler struct {
	cfg Config
	mux *http.ServeMux
}

// NewHandler returns the test-double HTTP handler serving GET /health,
// GET /v1/models and POST /v1/chat/completions.
func NewHandler(cfg Config) (http.Handler, error) {
	f, err := ParseFault(string(cfg.Fault))
	if err != nil {
		return nil, err
	}
	cfg.Fault = f
	pf, err := ParseProviderFault(string(cfg.ProviderFault))
	if err != nil {
		return nil, err
	}
	cfg.ProviderFault = pf
	if cfg.Latency < 0 || cfg.SlowDelay < 0 {
		return nil, errors.New("fakellm: durations must not be negative")
	}
	if cfg.SlowDelay == 0 {
		cfg.SlowDelay = DefaultSlowDelay
	}
	h := &handler{cfg: cfg, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("GET /v1/models", h.auth(h.models))
	h.mux.HandleFunc("POST /v1/chat/completions", h.auth(h.chat))
	// Provider test doubles (internal/ai): Anthropic-compatible and
	// DeepSeek-compatible, each labelled as a test double.
	h.mux.HandleFunc("POST /v1/messages", h.auth(h.anthropicMessages))
	h.mux.HandleFunc("POST /chat/completions", h.auth(h.deepseekChat))
	for _, p := range []string{"/health", "/v1/models", "/v1/chat/completions", "/v1/messages", "/chat/completions"} {
		h.mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		})
	}
	h.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found")
	})
	return h, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(HeaderTestDouble, "true")
	h.mux.ServeHTTP(w, r)
}

// auth enforces the optional bearer token.
func (h *handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.APIKey != "" {
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(h.cfg.APIKey)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		next(w, r)
	}
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok", "model_id": ModelID, "revision": Revision, "served_model": ServedModel,
	})
}

func (h *handler) models(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{{
			"id": ServedModel, "object": "model", "created": created,
			"owned_by": "sela-test-double", "root": ModelID + "@" + Revision,
		}},
	})
}

// chatRequest is the subset of the OpenAI request the double reads.
type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Stream        bool `json:"stream"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

func (h *handler) chat(w http.ResponseWriter, r *http.Request) {
	fault := h.cfg.Fault
	if v := r.Header.Get(HeaderFault); v != "" {
		f, err := ParseFault(v)
		if !h.cfg.AllowFaultHeader || err != nil {
			writeError(w, http.StatusBadRequest, "fault_header_rejected")
			return
		}
		fault = f
	}
	var req chatRequest
	body := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	err := json.NewDecoder(body).Decode(&req)
	if err == nil {
		// Consume the rest so the server notices when the client goes away
		// (it watches the connection only after the body is read).
		_, err = io.Copy(io.Discard, body)
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.Model != ServedModel {
		writeError(w, http.StatusNotFound, "model_not_found")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages_required")
		return
	}
	switch fault {
	case FaultHTTP503:
		writeError(w, http.StatusServiceUnavailable, "fake_unavailable")
		return
	case FaultHTTP429:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooManyRequests, "fake_rate_limited")
		return
	case FaultHTTP401:
		writeError(w, http.StatusUnauthorized, "fake_unauthorized")
		return
	}

	prompt := 0
	user := ""
	for _, m := range req.Messages {
		text := messageText(m.Content)
		prompt += len(text)
		if m.Role == "user" {
			user = text
		}
	}
	content := generate(user, fault)
	model := ServedModel
	if fault == FaultWrongModel {
		model = WrongModel
	}
	resp := response{
		id:               fmt.Sprintf("chatcmpl-test-double-%016x", fnv64(content)),
		model:            model,
		content:          content,
		promptTokens:     tokens(prompt),
		completionTokens: tokens(len(content)),
	}
	if fault == FaultSlow && !sleep(r, h.cfg.SlowDelay) {
		return
	}
	if req.Stream {
		usage := req.StreamOptions != nil && req.StreamOptions.IncludeUsage
		h.stream(w, r, resp, usage, fault)
		return
	}
	h.complete(w, r, resp, fault)
}

// response is one generated completion.
type response struct {
	id                             string
	model                          string
	content                        string
	promptTokens, completionTokens int
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (r response) usage() usage {
	return usage{r.promptTokens, r.completionTokens, r.promptTokens + r.completionTokens}
}

type delta struct {
	Role    string  `json:"role,omitempty"`
	Content *string `json:"content,omitempty"`
}

type chunkChoice struct {
	Index        int     `json:"index"`
	Delta        delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type chunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []chunkChoice `json:"choices"`
	Usage   *usage        `json:"usage,omitempty"`
}

// stream writes resp as server-sent events in ChunkBytes pieces.
func (h *handler) stream(w http.ResponseWriter, r *http.Request, resp response, includeUsage bool, fault Fault) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	send := func(data []byte) bool {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	sendChunk := func(choices []chunkChoice, u *usage) bool {
		raw, err := json.Marshal(chunk{ID: resp.id, Object: "chat.completion.chunk", Created: created, Model: resp.model, Choices: choices, Usage: u})
		return err == nil && send(raw)
	}
	empty := ""
	if !sendChunk([]chunkChoice{{Delta: delta{Role: "assistant", Content: &empty}}}, nil) {
		return
	}
	for i, piece := range split(resp.content, ChunkBytes) {
		if h.cfg.Latency > 0 && !sleep(r, h.cfg.Latency) {
			return
		}
		if !sendChunk([]chunkChoice{{Delta: delta{Content: &piece}}}, nil) {
			return
		}
		if fault == FaultStall && i == 0 {
			<-r.Context().Done()
			return
		}
	}
	if fault == FaultTruncatedStream {
		return
	}
	stop := "stop"
	if !sendChunk([]chunkChoice{{Delta: delta{}, FinishReason: &stop}}, nil) {
		return
	}
	if includeUsage {
		u := resp.usage()
		if !sendChunk([]chunkChoice{}, &u) {
			return
		}
	}
	send([]byte("[DONE]"))
}

// complete writes resp as one chat.completion JSON body.
func (h *handler) complete(w http.ResponseWriter, r *http.Request, resp response, fault Fault) {
	if fault == FaultStall {
		<-r.Context().Done()
		return
	}
	body, err := json.Marshal(map[string]any{
		"id": resp.id, "object": "chat.completion", "created": created, "model": resp.model,
		"choices": []map[string]any{{
			"index": 0, "message": map[string]string{"role": "assistant", "content": resp.content}, "finish_reason": "stop",
		}},
		"usage": resp.usage(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encoding_failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if fault == FaultTruncatedStream {
		body = body[:len(body)/2]
	}
	_, _ = w.Write(body)
}

// messageText returns a message's text: a JSON string, or the "text" parts
// of an OpenAI content-part array.
func messageText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// writeError writes an OpenAI-style error with a fixed code, naming the
// test-double model; it never echoes request content.
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{
		"model": ServedModel,
		"error": map[string]any{"message": "test double: " + code, "type": code, "code": status},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		status, raw = http.StatusInternalServerError, []byte(`{"error":{"message":"test double: encoding_failed"}}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// sleep waits d or until the client goes away; it reports whether the
// full delay elapsed.
func sleep(r *http.Request, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-r.Context().Done():
		return false
	}
}

// tokens is a deterministic token estimate (about four bytes per token).
func tokens(n int) int { return (n + 3) / 4 }

// fnv64 is the 64-bit FNV-1a hash, used for deterministic response IDs.
func fnv64(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

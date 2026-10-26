package llm_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/YanIanZ/sela/internal/llm"
)

const (
	servedModel = llm.TestDoubleServedModel
	// secretBody looks like a provider error that leaks credentials and
	// user text; it must never reach an error message.
	secretBody = `{"error":{"message":"sk-live-SECRET-9f8e7d leaked; user said: aku ingin menyerah","type":"internal"}}`
)

// testRequest is a small bounded request with a JSON schema.
func testRequest() llm.Request {
	return llm.Request{
		Operation:  "ledger_draft",
		Messages:   []llm.Message{{Role: llm.RoleSystem, Content: "system prompt"}, {Role: llm.RoleUser, Content: "PROMPT-CANARY user text"}},
		JSONSchema: `{"type":"object","properties":{"lines":{"type":"array"}},"required":["lines"]}`,
		SchemaName: "sela_test",
		MaxTokens:  256,
	}
}

// newTestDouble builds a ModeTestDouble provider for base after applying mut.
func newTestDouble(t testing.TB, base string, mut func(*llm.Config)) llm.Provider {
	t.Helper()
	cfg := llm.Config{Mode: llm.ModeTestDouble, BaseURL: base, ServedModel: servedModel}
	if mut != nil {
		mut(&cfg)
	}
	p, err := llm.New(cfg)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	return p
}

// sseChunk renders one chat.completion.chunk.
func sseChunk(model string, content *string, finish string, usage bool) string {
	ch := map[string]any{"id": "c1", "object": "chat.completion.chunk", "model": model}
	choice := map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": nil}
	if content != nil {
		choice["delta"] = map[string]any{"content": *content}
	}
	if finish != "" {
		choice["finish_reason"] = finish
	}
	ch["choices"] = []any{choice}
	if usage {
		ch["choices"] = []any{}
		ch["usage"] = map[string]int{"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
	}
	raw, _ := json.Marshal(ch)
	return "data: " + string(raw) + "\n\n"
}

// writeStream writes a complete, well-formed stream of pieces.
func writeStream(w http.ResponseWriter, model string, pieces ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	empty := ""
	fmt.Fprint(w, sseChunk(model, &empty, "", false))
	for _, p := range pieces {
		fmt.Fprint(w, sseChunk(model, &p, "", false))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, sseChunk(model, nil, "stop", false))
	fmt.Fprint(w, sseChunk(model, nil, "", true))
	fmt.Fprint(w, "data: [DONE]\n\n")
}

// hang reads the request body (net/http notices a client disconnect only
// after the body is consumed) and blocks until the client goes away.
func hang(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	<-r.Context().Done()
}

// counter is a concurrency-safe request counter.
type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) inc() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return c.n
}

func (c *counter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// server starts an httptest server for h.
func server(t testing.TB, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// events collects observer events.
type events struct {
	mu  sync.Mutex
	got []llm.Event
}

func (e *events) observe(ev llm.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.got = append(e.got, ev)
}

func (e *events) list() []llm.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]llm.Event(nil), e.got...)
}

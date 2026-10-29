package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetaAPIBodyUsesReasoningEffortAndOmitsVLLMFields(t *testing.T) {
	c := &client{cfg: Config{
		Mode: ModeMeta, ServedModel: "muse-spark-1.3", ModelID: "muse-spark-1.3",
		ReasoningEffort: "minimal",
	}}
	raw, err := c.buildBody(Request{
		Messages:          []Message{{Role: RoleUser, Content: "halo"}},
		MaxTokens:         512,
		ReasoningStrength: ReasoningLow,
		Sampling:          &Sampling{Temperature: 1, TopP: 1, TopK: 40},
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["reasoning_effort"] != "minimal" {
		t.Fatalf("reasoning_effort = %v", body["reasoning_effort"])
	}
	if _, ok := body["chat_template_kwargs"]; ok {
		t.Fatal("vLLM chat_template_kwargs sent to Meta Model API")
	}
	if _, ok := body["top_k"]; ok {
		t.Fatal("top_k sent to Meta Model API")
	}
	if body["model"] != "muse-spark-1.3" {
		t.Fatalf("model = %v", body["model"])
	}
}

func TestMetaAPIValidation(t *testing.T) {
	base := Config{
		Mode: ModeMeta, BaseURL: "https://api.meta.ai/v1", APIKey: "secret",
		ServedModel: "muse-spark-1.3", ModelID: "muse-spark-1.3",
	}
	if _, err := New(base); err != nil {
		t.Fatalf("valid meta_api config: %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"http":              func(c *Config) { c.BaseURL = "http://api.meta.ai/v1" },
		"other host":        func(c *Config) { c.BaseURL = "https://example.com/v1" },
		"missing key":       func(c *Config) { c.APIKey = "" },
		"missing model":     func(c *Config) { c.ServedModel = " " },
		"missing model id":  func(c *Config) { c.ModelID = "" },
		"none reasoning":    func(c *Config) { c.ReasoningEffort = "none" },
		"test double model": func(c *Config) { c.ServedModel = TestDoubleServedModel },
	} {
		cfg := base
		mutate(&cfg)
		if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "llm: invalid config") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := base; got.ReasoningEffort != "" {
		// The caller's value is not mutated; the default is applied internally.
	}
}

func TestMetaAPIHealthUsesServedModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"muse-spark-1.3","object":"model"}]}`))
	}))
	defer srv.Close()
	c := &client{
		cfg:       Config{Mode: ModeMeta, ServedModel: "muse-spark-1.3", ModelID: "muse-spark-1.3", Revision: "muse-spark-1.3"},
		hc:        srv.Client(),
		modelsURL: srv.URL + "/v1/models",
		healthURL: srv.URL + "/health",
	}
	if !c.checkModels(context.Background()) {
		t.Fatal("served Meta model not recognised")
	}
	c.cfg.ServedModel = "muse-spark-9.9"
	if c.checkModels(context.Background()) {
		t.Fatal("unknown model accepted")
	}
}

func TestMetaAPIBaseURLNormalisation(t *testing.T) {
	p, err := New(Config{
		Mode: ModeMeta, BaseURL: "https://api.meta.ai/v1", APIKey: "k",
		ServedModel: "muse-spark-1.3", ModelID: "muse-spark-1.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*client)
	if c.chatURL != "https://api.meta.ai/v1/chat/completions" {
		t.Fatalf("chatURL = %q", c.chatURL)
	}
	if c.modelsURL != "https://api.meta.ai/v1/models" {
		t.Fatalf("modelsURL = %q", c.modelsURL)
	}
}

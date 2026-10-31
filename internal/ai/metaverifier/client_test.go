package metaverifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
)

func testRequest() verifier.Request {
	return verifier.Request{
		Task: "summary_draft", Language: "id", Audience: verifier.AudienceAdult,
		Spans:  []verifier.Span{{ID: "s1", Text: "Aku susah tidur sejak dua bulan."}},
		Claims: []verifier.Claim{{ID: "c1", Text: "Saya sulit tidur.", SpanIDs: []string{"s1"}}},
	}
}

func reportBody(model string, finish string, content string) string {
	body := map[string]any{
		"id": "chatcmpl-test", "object": "chat.completion", "model": model,
		"choices": []map[string]any{{"index": 0, "finish_reason": finish,
			"message": map[string]any{"role": "assistant", "content": content}}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("X-Sela-Test-Double", "true")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New(Config{TestDoubleAPIKey: transport.NewSecret("test-key"), BaseURL: base, Model: TestDoubleModel,
		TestDouble: true, AttemptTimeout: time.Second, MaxRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestVerifyValidReport(t *testing.T) {
	content := `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"same meaning"}],"issues":[]}`
	srv := serve(t, http.StatusOK, reportBody(TestDoubleModel, "stop", content))
	c := newTestClient(t, srv.URL)
	res, err := c.Verify(context.Background(), testRequest(), verifier.CallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ServedModel != TestDoubleModel || res.Report.Claims[0].Verdict != verifier.Supported || res.InputTokens != 10 {
		t.Fatalf("result %+v", res)
	}
	if c.Identity().Provider != Provider || c.Identity().Model != TestDoubleModel {
		t.Fatalf("identity %+v", c.Identity())
	}
}

func TestVerifyFailures(t *testing.T) {
	content := `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"ok"}],"issues":[]}`
	cases := map[string]struct {
		status int
		body   string
	}{
		"model mismatch": {http.StatusOK, reportBody("other-model", "stop", content)},
		"refusal":        {http.StatusOK, reportBody(TestDoubleModel, "content_filter", "")},
		"truncated":      {http.StatusOK, reportBody(TestDoubleModel, "length", content)},
		"empty":          {http.StatusOK, reportBody(TestDoubleModel, "stop", "")},
		"invalid json":   {http.StatusOK, reportBody(TestDoubleModel, "stop", "{")},
		"bad report": {http.StatusOK, reportBody(TestDoubleModel, "stop",
			`{"claims":[{"claim_id":"zzz","verdict":"supported","span_ids":["s1"],"reason":"ok"}],"issues":[]}`)},
		"http 500": {http.StatusInternalServerError, `{}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := serve(t, tc.status, tc.body)
			c := newTestClient(t, srv.URL)
			if _, err := c.Verify(context.Background(), testRequest(), verifier.CallOptions{}); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	if _, err := New(Config{Production: true, APIKey: transport.NewSecret("k"), BaseURL: "https://api.anthropic.com"}); err == nil {
		t.Fatal("production accepted a non-Meta host")
	}
	if _, err := New(Config{APIKey: transport.NewSecret("k"), TestDouble: true, Model: TestDoubleModel}); err != nil {
		t.Fatalf("test double rejected: %v", err)
	}
	if _, err := New(Config{APIKey: transport.NewSecret("k"), Model: "not-the-double", TestDouble: true}); err == nil {
		t.Fatal("test double accepted a live model name")
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("live client accepted without a key")
	}
}

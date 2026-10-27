package anthropic

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

func testRequest() verifier.Request {
	return verifier.Request{Task: "summary_draft", Language: "en", Audience: verifier.AudienceAdult,
		Spans:  []verifier.Span{{ID: "s1", Text: "I have not slept well for two weeks."}},
		Claims: []verifier.Claim{{ID: "c1", Text: "You have not slept well for two weeks.", SpanIDs: []string{"s1"}}}}
}

// TestLiveClientRefusesTestDouble: a live client that is answered by a
// labelled test double treats it as an integrity failure (it can never be
// recorded as live_anthropic). New refuses such a URL already; this
// covers a response-level label.
func TestLiveClientRefusesTestDouble(t *testing.T) {
	h, err := fakellm.NewHandler(fakellm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	c := &Client{cfg: Config{Model: TestDoubleModel, Version: DefaultVersion, Effort: "low", MaxTokens: 2000,
		InferenceGeo: "us", AttemptTimeout: DefaultAttemptTimeout}, hc: transport.HTTPClient(nil), endpoint: srv.URL + "/v1/messages"}
	if _, err := c.Verify(context.Background(), testRequest(), verifier.CallOptions{}); transport.ReasonOf(err) != transport.ReasonTestDouble {
		t.Fatalf("err %v", err)
	}
}

// TestTestDoubleClientRequiresLabel: a test-double client refuses an
// unlabelled answer.
func TestTestDoubleClientRequiresLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"message","model":"test-double-not-claude","stop_reason":"end_turn","content":[{"type":"text","text":"{\"claims\":[{\"claim_id\":\"c1\",\"verdict\":\"supported\",\"span_ids\":[\"s1\"],\"reason\":\"ok\"}],\"issues\":[]}"}]}`)
	}))
	defer srv.Close()
	c, err := New(Config{TestDouble: true, BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Verify(context.Background(), testRequest(), verifier.CallOptions{}); transport.ReasonOf(err) != transport.ReasonTestDouble {
		t.Fatalf("err %v", err)
	}
}

// TestShapeChecks covers VC-6 response-shape rules that the fault list
// does not: other stop reasons, extra blocks and oversize bodies.
func TestShapeChecks(t *testing.T) {
	cases := map[string]struct {
		body   string
		reason transport.Reason
	}{
		"tool_use stop":    {`{"model":"test-double-not-claude","stop_reason":"tool_use","content":[{"type":"text","text":"{}"}]}`, transport.ReasonInvalidOutput},
		"context exceeded": {`{"model":"test-double-not-claude","stop_reason":"model_context_window_exceeded","content":[]}`, transport.ReasonTruncated},
		"two text blocks":  {`{"model":"test-double-not-claude","stop_reason":"end_turn","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`, transport.ReasonInvalidOutput},
		"tool block":       {`{"model":"test-double-not-claude","stop_reason":"end_turn","content":[{"type":"tool_use","text":""}]}`, transport.ReasonInvalidOutput},
		"not json":         {`<html>`, transport.ReasonInvalidOutput},
		"too large":        {`{"model":"test-double-not-claude","x":"` + string(make([]byte, verifier.MaxResponseBytes)) + `"}`, transport.ReasonInvalidOutput},
	}
	for name, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(transport.HeaderTestDouble, "true")
			_, _ = io.WriteString(w, tc.body)
		}))
		c, err := New(Config{TestDouble: true, BaseURL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Verify(context.Background(), testRequest(), verifier.CallOptions{})
		srv.Close()
		if transport.ReasonOf(err) != tc.reason {
			t.Errorf("%s: err %v, want %s", name, err, tc.reason)
		}
	}
}

// TestLiveHeaders: a live client sends the live key as Bearer and the
// workspace header when set, and never a test-double key.
func TestLiveHeaders(t *testing.T) {
	var header http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"message","model":"claude-sonnet-5-5","stop_reason":"end_turn","content":[{"type":"text","text":"{\"claims\":[{\"claim_id\":\"c1\",\"verdict\":\"supported\",\"span_ids\":[\"s1\"],\"reason\":\"ok\"}],\"issues\":[]}"}]}`)
	}))
	defer srv.Close()
	c := &Client{cfg: Config{APIKey: transport.NewSecret("sk-live-1"), TestDoubleAPIKey: transport.NewSecret("td-1"), WorkspaceID: transport.NewSecret("wrkspc_1"), Model: DefaultModel, Version: DefaultVersion,
		Effort: "low", MaxTokens: 2000, InferenceGeo: "us", AttemptTimeout: DefaultAttemptTimeout, MaxRetries: -1},
		hc: transport.HTTPClient(nil), endpoint: srv.URL + "/v1/messages"}
	if _, err := c.Verify(context.Background(), testRequest(), verifier.CallOptions{}); err != nil {
		t.Fatal(err)
	}
	if header.Get("Authorization") != "Bearer sk-live-1" || header.Get("anthropic-workspace-id") != "wrkspc_1" {
		t.Fatalf("headers %v", header)
	}
}

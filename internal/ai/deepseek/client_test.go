package deepseek_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/deepseek"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

const (
	apiKey    = "sk-ds-TEST-SECRET-CANARY"
	doubleKey = "test-double-key-456"
)

// sec makes a Secret.
var sec = transport.NewSecret

func double(t *testing.T, cfg fakellm.Config) *httptest.Server {
	t.Helper()
	cfg.APIKey = doubleKey
	h, err := fakellm.NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func client(t *testing.T, base string, timeout time.Duration) *deepseek.Client {
	t.Helper()
	// A live key is configured too: a test-double client must never send
	// it (only TestDoubleAPIKey).
	c, err := deepseek.New(deepseek.Config{APIKey: sec(apiKey), TestDoubleAPIKey: sec(doubleKey), BaseURL: base, TestDouble: true, AttemptTimeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const genUser = "data\n<<<SELA_INPUT_JSON\n" +
	`{"task":"summary_draft","schema":"sela-ai-generation-v1","language":"id","audience":"adult","spans":[{"id":"s1","role":"item","text":"Aku susah tidur sejak dua bulan."}]}` +
	"\nSELA_INPUT_JSON>>>"

func genRequest() deepseek.Request {
	return deepseek.Request{System: "Reply with one json object.", User: genUser, MaxTokens: 512}
}

func vreq() verifier.Request {
	return verifier.Request{Task: "summary_draft", Language: "id", Audience: verifier.AudienceAdult,
		Spans:  []verifier.Span{{ID: "s1", Text: "Aku susah tidur sejak dua bulan."}},
		Claims: []verifier.Claim{{ID: "c1", Text: "Kamu susah tidur sejak dua bulan.", SpanIDs: []string{"s1"}}}}
}

func TestCompleteAgainstTestDouble(t *testing.T) {
	srv := double(t, fakellm.Config{})
	c := client(t, srv.URL, 2*time.Second)
	comp, err := c.Complete(context.Background(), genRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(comp.Text, `"span_ids":["s1"]`) || comp.ServedModel != deepseek.TestDoubleModel || !comp.TestDouble ||
		comp.FinishReason != "stop" || comp.Attempts != 1 || comp.InputTokens == 0 {
		t.Fatalf("completion %+v", comp)
	}
}

// TestRequestContract: model, json_object, thinking disabled, no
// streaming, no user_id, no sampling; reasoning_content is never read.
func TestRequestContract(t *testing.T) {
	inner := double(t, fakellm.Config{})
	var body map[string]json.RawMessage
	var header http.Header
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		header = r.Header.Clone()
		_ = json.Unmarshal(raw, &body)
		req, _ := http.NewRequest(r.Method, inner.URL+r.URL.Path, bytes.NewReader(raw))
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	defer proxy.Close()
	c := client(t, proxy.URL, 2*time.Second)
	if _, err := c.Complete(context.Background(), genRequest()); err != nil {
		t.Fatal(err)
	}
	if header.Get("Authorization") != "Bearer "+doubleKey {
		t.Fatal("auth: a test double gets the test-double key only")
	}
	for k, v := range header {
		if strings.Contains(strings.Join(v, " "), apiKey) {
			t.Fatalf("the live key reached the test double in %s", k)
		}
	}
	want := map[string]string{"model": `"test-double-not-deepseek"`, "response_format": `{"type":"json_object"}`,
		"thinking": `{"type":"disabled"}`, "stream": "false", "max_tokens": "512"}
	for k, v := range want {
		if string(body[k]) != v {
			t.Fatalf("%s = %s, want %s", k, body[k], v)
		}
	}
	for _, k := range []string{"user_id", "user", "temperature", "top_p", "tools", "stream_options"} {
		if _, ok := body[k]; ok {
			t.Fatalf("body carries %s", k)
		}
	}
	if len(body) != 6 {
		t.Fatalf("body fields %v", body)
	}
}

func TestPromptMustMentionJSON(t *testing.T) {
	c := client(t, "http://127.0.0.1:1", time.Second)
	_, err := c.Complete(context.Background(), deepseek.Request{System: "Reply.", User: "hi"})
	if transport.ReasonOf(err) != transport.ReasonRejected {
		t.Fatalf("err %v", err)
	}
}

// TestFaults covers every provider fault for both roles: the fallback
// generator (Complete) and the alternative verifier (Verify).
func TestFaults(t *testing.T) {
	cases := []struct {
		fault       fakellm.ProviderFault
		genReason   transport.Reason // Complete
		verReason   transport.Reason // Verify
		genAttempts int
	}{
		{fakellm.ProviderFaultRefusal, transport.ReasonContentFilter, transport.ReasonContentFilter, 1},
		{fakellm.ProviderFaultTruncation, transport.ReasonTruncated, transport.ReasonTruncated, 1},
		{fakellm.ProviderFaultMalformedJSON, "", transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultWrongEnumCase, "", "", 1},
		{fakellm.ProviderFaultHTTP429, transport.ReasonRateLimited, transport.ReasonRateLimited, 2},
		{fakellm.ProviderFaultHTTP5xx, transport.ReasonUnavailable, transport.ReasonUnavailable, 2},
		{fakellm.ProviderFaultSpendCap, transport.ReasonSpendCap, transport.ReasonSpendCap, 1},
		{fakellm.ProviderFaultHTTP401, transport.ReasonAuth, transport.ReasonAuth, 1},
		{fakellm.ProviderFaultTimeout, transport.ReasonTimeout, transport.ReasonTimeout, 1},
		// A 200 whose body breaks off is content class (§4), never retried.
		{fakellm.ProviderFaultStall, transport.ReasonInvalidOutput, transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultWrongModel, transport.ReasonModelMismatch, transport.ReasonModelMismatch, 1},
		{fakellm.ProviderFaultReasoningLeak, "", transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultEmptyContent, transport.ReasonEmptyContent, transport.ReasonEmptyContent, 1},
		{fakellm.ProviderFaultPromptInjectionEcho, "", transport.ReasonInvalidOutput, 1},
	}
	covered := map[fakellm.ProviderFault]bool{fakellm.ProviderFaultNone: true}
	for _, tc := range cases {
		covered[tc.fault] = true
		t.Run(string(tc.fault), func(t *testing.T) {
			t.Parallel()
			srv := double(t, fakellm.Config{ProviderFault: tc.fault, SlowDelay: 3 * time.Second})
			c := client(t, srv.URL, 300*time.Millisecond)
			comp, err := c.Complete(context.Background(), genRequest())
			if got := transport.ReasonOf(err); got != tc.genReason {
				t.Fatalf("generate reason %q, want %q (%v)", got, tc.genReason, err)
			}
			if comp.Attempts != tc.genAttempts {
				t.Fatalf("attempts %d, want %d", comp.Attempts, tc.genAttempts)
			}
			if err != nil && comp.Text != "" {
				t.Fatal("failed completion carries text")
			}
			res, verr := c.AsVerifier().Verify(context.Background(), vreq(), verifier.CallOptions{})
			if got := transport.ReasonOf(verr); got != tc.verReason {
				t.Fatalf("verify reason %q, want %q (%v)", got, tc.verReason, verr)
			}
			if verr == nil && res.Report.Claims[0].Verdict != verifier.Supported {
				t.Fatalf("verdict %+v", res.Report.Claims[0])
			}
			for _, s := range []string{string(mustJSON(t, comp)), string(mustJSON(t, res)), errString(err), errString(verr)} {
				if strings.Contains(s, fakellm.ReasoningCanary+" (test double reasoning)") || strings.Contains(s, apiKey) {
					t.Fatalf("leak: %s", s)
				}
			}
			if tc.fault == fakellm.ProviderFaultReasoningLeak && !strings.HasPrefix(comp.Text, "<think>") {
				// The visible text carries the leak; the router's strict
				// parser rejects it. reasoning_content itself is never read.
				t.Fatalf("text %q", comp.Text)
			}
		})
	}
	for _, f := range fakellm.ProviderFaults() {
		if !covered[f] {
			t.Errorf("fault %s not covered", f)
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mustJSON(t *testing.T, v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAlternativeVerifierLabel(t *testing.T) {
	srv := double(t, fakellm.Config{})
	v := client(t, srv.URL, 2*time.Second).AsVerifier()
	res, err := v.Verify(context.Background(), vreq(), verifier.CallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != deepseek.Provider || !strings.Contains(res.Label, deepseek.AlternativeLabel) || !strings.Contains(res.Label, "TEST DOUBLE") {
		t.Fatalf("identity %+v", res.Identity)
	}
	live, err := deepseek.New(deepseek.Config{APIKey: sec(apiKey)})
	if err != nil {
		t.Fatal(err)
	}
	if id := live.AsVerifier().Identity(); id.Label != deepseek.AlternativeLabel || id.Model != "deepseek-flash" {
		t.Fatalf("live identity %+v", id)
	}
}

func TestConfigValidation(t *testing.T) {
	bad := []deepseek.Config{
		{},
		{APIKey: sec(apiKey), BaseURL: "http://api.deepseek.com"},
		{APIKey: sec(apiKey + "\n")},
		{APIKey: sec(apiKey), Model: deepseek.TestDoubleModel},
		{TestDouble: true, Model: "deepseek-flash"},
		{TestDouble: true, Production: true},
		{APIKey: sec(apiKey), Production: true, BaseURL: "https://deepseek-proxy.example.com"},
		{APIKey: sec(apiKey), Production: true, BaseURL: "http://127.0.0.1:8090"},
		{TestDouble: true, PinServedVersion: true},
		{TestDouble: true, BaseURL: "https://api.deepseek.com"},
		{TestDouble: true, APIKey: sec(apiKey), BaseURL: "https://deepseek-proxy.example.com"},
		{TestDouble: true, TestDoubleAPIKey: sec("a\nb")},
	}
	for i, cfg := range bad {
		_, err := deepseek.New(cfg)
		if err == nil {
			t.Errorf("config %d accepted", i)
		} else if strings.Contains(err.Error(), apiKey) {
			t.Errorf("config %d error echoes the key", i)
		}
	}
	if _, err := deepseek.New(deepseek.Config{APIKey: sec(apiKey), Production: true}); err != nil {
		t.Fatal(err)
	}
}

// TestClientNeverPrintsKey: every fmt verb prints a summary, never the key.
func TestClientNeverPrintsKey(t *testing.T) {
	c, err := deepseek.New(deepseek.Config{APIKey: sec(apiKey), Production: true})
	if err != nil {
		t.Fatal(err)
	}
	cfg := deepseek.Config{APIKey: sec(apiKey), TestDoubleAPIKey: sec(doubleKey)}
	raw, _ := json.Marshal(cfg)
	out := string(raw)
	for _, f := range []string{"%v", "%+v", "%#v", "%s", "%x", "%d"} {
		out += fmt.Sprintf(f, c) + fmt.Sprintf(f, c.AsVerifier()) + fmt.Sprintf(f, cfg)
	}
	for _, canary := range []string{apiKey, doubleKey, fmt.Sprintf("%x", apiKey)} {
		if strings.Contains(out, canary) {
			t.Fatalf("printed %q: %s", canary, out)
		}
	}
}

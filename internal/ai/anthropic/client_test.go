package anthropic_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/anthropic"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

const (
	apiKey    = "sk-ant-TEST-SECRET-CANARY"
	doubleKey = "test-double-key-123"
)

// sec makes a Secret.
var sec = transport.NewSecret

func request() verifier.Request {
	return verifier.Request{
		Task: "summary_draft", Language: "id", Audience: verifier.AudienceAdult,
		Spans: []verifier.Span{
			{ID: "s1", Text: "Aku belum pernah cerita ke siapa pun soal ini."},
			{ID: "s2", Text: "Aku susah tidur sejak dua bulan."},
			{ID: "s3", Text: "Ya."},
		},
		Claims: []verifier.Claim{
			{ID: "c1", Text: "Kamu belum pernah cerita ke siapa pun soal ini.", SpanIDs: []string{"s1"}},
			{ID: "c2", Text: "Kamu susah tidur sejak dua bulan karena kerja.", SpanIDs: []string{"s2"}},
			{ID: "c3", Text: "Kamu sudah pernah cerita ke siapa pun soal ini.", SpanIDs: []string{"s1"}},
			{ID: "c4", Text: "Kamu setuju.", SpanIDs: []string{"s3"}},
		},
	}
}

func double(t *testing.T, cfg fakellm.Config) *httptest.Server {
	t.Helper()
	if cfg.APIKey == "" {
		cfg.APIKey = doubleKey
	}
	h, err := fakellm.NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func client(t *testing.T, base string, mod func(*anthropic.Config)) *anthropic.Client {
	t.Helper()
	// A live key is configured too: a test-double client must never send
	// it (only TestDoubleAPIKey).
	cfg := anthropic.Config{APIKey: sec(apiKey), TestDoubleAPIKey: sec(doubleKey), BaseURL: base, TestDouble: true, AttemptTimeout: 2 * time.Second}
	if mod != nil {
		mod(&cfg)
	}
	c, err := anthropic.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestVerifyAgainstTestDouble(t *testing.T) {
	srv := double(t, fakellm.Config{})
	c := client(t, srv.URL, nil)
	res, err := c.Verify(context.Background(), request(), verifier.CallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []verifier.Verdict{verifier.Supported, verifier.Unsupported, verifier.Contradicted, verifier.InsufficientEvidence}
	for i, v := range want {
		if res.Report.Claims[i].Verdict != v {
			t.Fatalf("claim %d verdict %s, want %s", i, res.Report.Claims[i].Verdict, v)
		}
	}
	if !res.TestDouble || res.Label != anthropic.TestDoubleLabel || res.ServedModel != anthropic.TestDoubleModel ||
		res.APIVersion != anthropic.DefaultVersion || res.Attempts != 1 || res.InputTokens == 0 || res.OutputTokens == 0 {
		t.Fatalf("metadata %+v", res)
	}
}

// TestRequestContract checks VC-4: Bearer auth with the test-double key
// only (never the live key or workspace), anthropic-version,
// between_tools thinking, effort low, the VC-2 schema, inference_geo us,
// and nothing else (no sampling, fallbacks, metadata or tools). The live
// headers are checked in TestLiveHeaders.
func TestRequestContract(t *testing.T) {
	inner := double(t, fakellm.Config{})
	for _, workspace := range []string{"", "wrkspc_01TEST"} {
		var header http.Header
		var body map[string]json.RawMessage
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
		c := client(t, proxy.URL, func(cfg *anthropic.Config) { cfg.WorkspaceID = transport.NewSecret(workspace) })
		if _, err := c.Verify(context.Background(), request(), verifier.CallOptions{}); err != nil {
			t.Fatal(err)
		}
		proxy.Close()
		if header.Get("Authorization") != "Bearer "+doubleKey || header.Get("x-api-key") != "" {
			t.Fatal("auth must be Bearer with the test-double key only")
		}
		for k, v := range header {
			if strings.Contains(strings.Join(v, " "), apiKey) {
				t.Fatalf("the live key reached the test double in %s", k)
			}
		}
		if header.Get("anthropic-version") != "2026-10-21" || !strings.HasPrefix(header.Get("Content-Type"), "application/json") {
			t.Fatalf("headers %v", header)
		}
		if header.Get("anthropic-workspace-id") != "" {
			t.Fatalf("workspace header %q sent to a test double", header.Get("anthropic-workspace-id"))
		}
		keys := map[string]bool{}
		for k := range body {
			keys[k] = true
		}
		for _, k := range []string{"model", "max_tokens", "system", "messages", "thinking", "output_config", "inference_geo"} {
			if !keys[k] {
				t.Fatalf("body lacks %s", k)
			}
			delete(keys, k)
		}
		if len(keys) != 0 {
			t.Fatalf("unexpected body fields %v", keys)
		}
		if string(body["thinking"]) != `{"type":"between_tools"}` || string(body["inference_geo"]) != `"us"` {
			t.Fatalf("thinking %s geo %s", body["thinking"], body["inference_geo"])
		}
		var oc struct {
			Effort string `json:"effort"`
			Format struct {
				Type   string          `json:"type"`
				Schema json.RawMessage `json:"schema"`
			} `json:"format"`
		}
		_ = json.Unmarshal(body["output_config"], &oc)
		if oc.Effort != "low" || oc.Format.Type != "json_schema" || string(oc.Format.Schema) != verifier.SchemaJSON {
			t.Fatalf("output_config %s", body["output_config"])
		}
		var msgs []map[string]string
		_ = json.Unmarshal(body["messages"], &msgs)
		if len(msgs) != 1 || msgs[0]["role"] != "user" || !strings.Contains(msgs[0]["content"], verifier.InputOpenMarker) {
			t.Fatalf("messages %v", msgs)
		}
	}
}

// TestFaults covers the client's behaviour for every provider fault of the
// test double: everything except the enum-casing fault is unavailable,
// with its reason; nothing is ever treated as a pass.
func TestFaults(t *testing.T) {
	cases := []struct {
		fault    fakellm.ProviderFault
		reason   transport.Reason
		attempts int
	}{
		{fakellm.ProviderFaultRefusal, transport.ReasonRefused, 1},
		{fakellm.ProviderFaultTruncation, transport.ReasonTruncated, 1},
		{fakellm.ProviderFaultMalformedJSON, transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultWrongEnumCase, "", 1},
		{fakellm.ProviderFaultHTTP429, transport.ReasonRateLimited, 2},
		{fakellm.ProviderFaultHTTP5xx, transport.ReasonUnavailable, 2},
		{fakellm.ProviderFaultSpendCap, transport.ReasonSpendCap, 1},
		{fakellm.ProviderFaultHTTP401, transport.ReasonAuth, 1},
		{fakellm.ProviderFaultTimeout, transport.ReasonTimeout, 1},
		// A 200 whose body breaks off is content class (§4), never retried.
		{fakellm.ProviderFaultStall, transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultWrongModel, transport.ReasonModelMismatch, 1},
		{fakellm.ProviderFaultReasoningLeak, transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultEmptyContent, transport.ReasonInvalidOutput, 1},
		{fakellm.ProviderFaultPromptInjectionEcho, transport.ReasonInvalidOutput, 1},
	}
	covered := map[fakellm.ProviderFault]bool{fakellm.ProviderFaultNone: true}
	for _, tc := range cases {
		covered[tc.fault] = true
		t.Run(string(tc.fault), func(t *testing.T) {
			t.Parallel()
			srv := double(t, fakellm.Config{ProviderFault: tc.fault, SlowDelay: 3 * time.Second})
			c := client(t, srv.URL, func(cfg *anthropic.Config) { cfg.AttemptTimeout = 300 * time.Millisecond })
			res, err := c.Verify(context.Background(), request(), verifier.CallOptions{})
			if got := transport.ReasonOf(err); got != tc.reason {
				t.Fatalf("reason %q, want %q (err %v)", got, tc.reason, err)
			}
			if res.Attempts != tc.attempts {
				t.Fatalf("attempts %d, want %d", res.Attempts, tc.attempts)
			}
			if err != nil && len(res.Report.Claims) != 0 {
				t.Fatal("a failed call returned a report")
			}
			if tc.fault == fakellm.ProviderFaultRefusal && res.RefusalCategory != "general_harms" {
				t.Fatalf("refusal category %q", res.RefusalCategory)
			}
			if tc.fault == fakellm.ProviderFaultWrongEnumCase && res.Report.Claims[0].Verdict != verifier.Supported {
				t.Fatalf("enum not normalized: %+v", res.Report.Claims[0])
			}
			if err != nil {
				for _, s := range []string{err.Error(), string(mustJSON(t, res))} {
					if strings.Contains(s, fakellm.ReasoningCanary) || strings.Contains(s, apiKey) || strings.Contains(s, "IGNORE") {
						t.Fatalf("leak in %q", s)
					}
				}
			}
		})
	}
	for _, f := range fakellm.ProviderFaults() {
		if !covered[f] {
			t.Errorf("fault %s not covered", f)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestRetryAfter: a 429 is retried after the server's Retry-After when it
// is short enough, and not at all when it is longer than MaxRetryAfter.
func TestRetryAfter(t *testing.T) {
	inner := double(t, fakellm.Config{})
	var n atomic.Int32
	retryAfter := "0"
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", retryAfter)
			w.Header().Set(transport.HeaderTestDouble, "true")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"x"}}`)
			return
		}
		r2, _ := http.NewRequest(r.Method, inner.URL+r.URL.Path, r.Body)
		r2.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(r2)
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
	c := client(t, proxy.URL, nil)
	res, err := c.Verify(context.Background(), request(), verifier.CallOptions{})
	if err != nil || res.Attempts != 2 {
		t.Fatalf("attempts %d err %v", res.Attempts, err)
	}
	n.Store(0)
	retryAfter = "30"
	res, err = c.Verify(context.Background(), request(), verifier.CallOptions{MaxRetryAfter: 3 * time.Second})
	if transport.ReasonOf(err) != transport.ReasonRateLimited || res.Attempts != 1 {
		t.Fatalf("attempts %d err %v", res.Attempts, err)
	}
}

func TestInvalidRequestNeverSent(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { n.Add(1) }))
	defer srv.Close()
	c := client(t, srv.URL, nil)
	req := request()
	req.Claims[0].SpanIDs = []string{"s99"}
	if _, err := c.Verify(context.Background(), req, verifier.CallOptions{}); transport.ReasonOf(err) != transport.ReasonRejected || n.Load() != 0 {
		t.Fatalf("err %v calls %d", err, n.Load())
	}
}

func TestConfigValidation(t *testing.T) {
	const k = "sk-cfg-CANARY"
	bad := []anthropic.Config{
		{}, // live without key
		{APIKey: sec(k), BaseURL: "http://api.anthropic.com"},
		{APIKey: sec(k), BaseURL: "https://127.0.0.1:8443"},
		{APIKey: sec(k + "\nx")},
		{APIKey: sec(k), WorkspaceID: sec("w\r\n")},
		{APIKey: sec(k), Version: "latest"},
		{APIKey: sec(k), Model: anthropic.TestDoubleModel},
		{TestDouble: true, Model: anthropic.DefaultModel},
		{APIKey: sec(k), InferenceGeo: "eu"},
		{APIKey: sec(k), Effort: "xhigh"},
		{APIKey: sec(k), MaxTokens: 10},
		{APIKey: sec(k), Production: true, BaseURL: "https://proxy.example-corp.com"},
		{TestDouble: true, Production: true},
		{APIKey: sec(k), Production: true, BaseURL: "http://127.0.0.1:8090"},
		{TestDouble: true, BaseURL: "https://api.anthropic.com"},
		{TestDouble: true, APIKey: sec(k), BaseURL: "https://proxy.example-corp.com"},
		{TestDouble: true, TestDoubleAPIKey: sec("x\ny")},
	}
	for i, cfg := range bad {
		_, err := anthropic.New(cfg)
		if err == nil {
			t.Errorf("config %d accepted", i)
			continue
		}
		if cfg.APIKey.Reveal() != "" && strings.Contains(err.Error(), cfg.APIKey.Reveal()) {
			t.Errorf("config %d error echoes the key", i)
		}
	}
	c, err := anthropic.New(anthropic.Config{APIKey: sec(apiKey), Production: true})
	if err != nil {
		t.Fatal(err)
	}
	id := c.Identity()
	if id.Model != "claude-sonnet-5-5" || id.TestDouble || id.Label != "" || c.APIVersion() != "2026-10-21" || c.InferenceGeo() != "us" ||
		id.InferenceGeo != "us" {
		t.Fatalf("identity %+v", id)
	}
	if strings.Contains(string(mustJSON(t, id)), apiKey) {
		t.Fatal("identity carries the key")
	}
}

// TestDeadlineBoundsRetries: a retry that would start after the caller's
// deadline is not attempted.
func TestDeadlineBoundsRetries(t *testing.T) {
	srv := double(t, fakellm.Config{ProviderFault: fakellm.ProviderFaultHTTP429})
	c := client(t, srv.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	res, err := c.Verify(ctx, request(), verifier.CallOptions{})
	if transport.ReasonOf(err) != transport.ReasonRateLimited || res.Attempts != 1 || time.Since(start) > 400*time.Millisecond {
		t.Fatalf("attempts %d err %v after %s", res.Attempts, err, time.Since(start))
	}
}

// TestClientNeverPrintsKey: the client's unexported configuration holds the
// key; every fmt verb prints a summary instead.
func TestClientNeverPrintsKey(t *testing.T) {
	c, err := anthropic.New(anthropic.Config{APIKey: sec(apiKey), WorkspaceID: sec("wrkspc_CANARY"), Production: true})
	if err != nil {
		t.Fatal(err)
	}
	cfg := anthropic.Config{APIKey: sec(apiKey), WorkspaceID: sec("wrkspc_CANARY"), TestDoubleAPIKey: sec(doubleKey)}
	out := string(mustJSON(t, cfg))
	for _, f := range []string{"%v", "%+v", "%#v", "%s", "%x", "%d"} {
		out += fmt.Sprintf(f, c) + fmt.Sprintf(f, cfg) + fmt.Sprintf(f, &cfg)
	}
	for _, canary := range []string{apiKey, "wrkspc_CANARY", doubleKey, fmt.Sprintf("%x", apiKey)} {
		if strings.Contains(out, canary) {
			t.Fatalf("printed %q: %s", canary, out)
		}
	}
}

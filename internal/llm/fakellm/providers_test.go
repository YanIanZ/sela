package fakellm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

const verifyUser = "data\n<<<SELA_VERIFY_JSON\n" +
	`{"task":"summary_draft","language":"id","audience":"adult","spans":[{"id":"s1","text":"Aku belum pernah cerita ke siapa pun soal ini."},{"id":"s2","text":"Aku mau bunuh diri rasanya."}],` +
	`"claims":[{"id":"c1","text":"Kamu belum pernah cerita ke siapa pun soal ini.","span_ids":["s1"]},{"id":"c2","text":"Kamu sudah pernah cerita ke siapa pun soal ini.","span_ids":["s1"]},{"id":"c3","text":"Kamu mengalami depresi berat.","span_ids":["s1"]}]}` +
	"\nSELA_VERIFY_JSON>>>"

func anthropicBody(t *testing.T, extra map[string]any) []byte {
	t.Helper()
	b := map[string]any{
		"model": fakellm.AnthropicModel, "max_tokens": 2000, "system": "x",
		"messages":      []map[string]string{{"role": "user", "content": verifyUser}},
		"thinking":      map[string]string{"type": "between_tools"},
		"output_config": map[string]any{"effort": "low", "format": map[string]any{"type": "json_schema", "schema": map[string]any{}}},
	}
	for k, v := range extra {
		if v == nil {
			delete(b, k)
		} else {
			b[k] = v
		}
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func deepseekBody(t *testing.T, user string, extra map[string]any) []byte {
	t.Helper()
	b := map[string]any{
		"model": fakellm.DeepSeekModel, "max_tokens": 2000, "stream": false,
		"messages":        []map[string]string{{"role": "system", "content": "Reply with a json object."}, {"role": "user", "content": user}},
		"response_format": map[string]string{"type": "json_object"},
		"thinking":        map[string]string{"type": "disabled"},
	}
	for k, v := range extra {
		if v == nil {
			delete(b, k)
		} else {
			b[k] = v
		}
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func send(t *testing.T, ctx context.Context, url, path string, body []byte, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if path == "/v1/messages" {
		req.Header.Set("anthropic-version", "2026-10-21")
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, raw
}

type anthropicResp struct {
	ID          string `json:"id"`
	Model       string `json:"model"`
	StopReason  string `json:"stop_reason"`
	StopDetails *struct {
		Category string `json:"category"`
	} `json:"stop_details"`
	Content []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	} `json:"content"`
}

type report struct {
	Claims []struct {
		ClaimID string   `json:"claim_id"`
		Verdict string   `json:"verdict"`
		SpanIDs []string `json:"span_ids"`
	} `json:"claims"`
	Issues []struct {
		Category string `json:"category"`
		ClaimID  string `json:"claim_id"`
	} `json:"issues"`
	Approved *bool `json:"approved"`
}

func TestAnthropicDoubleVerdicts(t *testing.T) {
	srv := newServer(t, fakellm.Config{})
	resp, raw := send(t, context.Background(), srv.URL, "/v1/messages", anthropicBody(t, nil), nil)
	if resp.StatusCode != 200 || resp.Header.Get(fakellm.HeaderTestDouble) != "true" {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var m anthropicResp
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Model != fakellm.AnthropicModel || !strings.HasPrefix(m.ID, "msg_test_double_") || m.StopReason != "end_turn" || len(m.Content) != 1 {
		t.Fatalf("message %+v", m)
	}
	var rep report
	if err := json.Unmarshal([]byte(m.Content[0].Text), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Claims[0].Verdict != "supported" || rep.Claims[1].Verdict != "contradicted" || rep.Claims[2].Verdict != "unsupported" {
		t.Fatalf("verdicts %+v", rep.Claims)
	}
	cats := map[string]string{}
	for _, is := range rep.Issues {
		cats[is.Category] = is.ClaimID
	}
	if cats["medical_language"] != "c3" || cats["safety_signal"] != "" {
		t.Fatalf("issues %+v", rep.Issues)
	}
	if strings.Contains(strings.ToLower(string(raw)), "claude-sonnet") {
		t.Fatal("the double claims to be Claude")
	}
}

// TestAnthropicDoubleEnforcesContract: the double rejects what Sela's
// verifier must never send.
func TestAnthropicDoubleEnforcesContract(t *testing.T) {
	srv := newServer(t, fakellm.Config{})
	cases := map[string]struct {
		body   []byte
		header map[string]string
		status int
	}{
		"temperature":   {anthropicBody(t, map[string]any{"temperature": 0.2}), nil, 400},
		"fallbacks":     {anthropicBody(t, map[string]any{"fallbacks": "default"}), nil, 400},
		"metadata":      {anthropicBody(t, map[string]any{"metadata": map[string]string{"user_id": "u"}}), nil, 400},
		"tools":         {anthropicBody(t, map[string]any{"tools": []any{}}), nil, 400},
		"thinking off":  {anthropicBody(t, map[string]any{"thinking": map[string]string{"type": "disabled"}}), nil, 400},
		"xhigh effort":  {anthropicBody(t, map[string]any{"output_config": map[string]string{"effort": "xhigh"}}), nil, 400},
		"prefill":       {anthropicBody(t, map[string]any{"messages": []map[string]string{{"role": "user", "content": "x"}, {"role": "assistant", "content": "{"}}}), nil, 400},
		"no max_tokens": {anthropicBody(t, map[string]any{"max_tokens": nil}), nil, 400},
		"real model":    {anthropicBody(t, map[string]any{"model": "claude-sonnet-5-5"}), nil, 404},
		"no version":    {anthropicBody(t, nil), map[string]string{"anthropic-version": ""}, 400},
		"fault header":  {anthropicBody(t, nil), map[string]string{fakellm.HeaderProviderFault: "refusal"}, 400},
		"invalid json":  {[]byte(`{"model":`), nil, 400},
	}
	for name, tc := range cases {
		resp, raw := send(t, context.Background(), srv.URL, "/v1/messages", tc.body, tc.header)
		if resp.StatusCode != tc.status || resp.Header.Get(fakellm.HeaderTestDouble) != "true" || !strings.Contains(string(raw), "test double") {
			t.Errorf("%s: status %d body %s", name, resp.StatusCode, raw)
		}
	}
}

func TestAnthropicDoubleFaults(t *testing.T) {
	cases := map[fakellm.ProviderFault]func(t *testing.T, status int, h http.Header, m anthropicResp){
		fakellm.ProviderFaultRefusal: func(t *testing.T, status int, _ http.Header, m anthropicResp) {
			if status != 200 || m.StopReason != "refusal" || m.StopDetails == nil || m.StopDetails.Category != "general_harms" || len(m.Content) != 0 {
				t.Fatalf("%+v", m)
			}
		},
		fakellm.ProviderFaultTruncation: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			if m.StopReason != "max_tokens" || json.Valid([]byte(m.Content[0].Text)) {
				t.Fatalf("%+v", m)
			}
		},
		fakellm.ProviderFaultMalformedJSON: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			if m.StopReason != "end_turn" || json.Valid([]byte(m.Content[0].Text)) {
				t.Fatalf("%+v", m)
			}
		},
		fakellm.ProviderFaultWrongEnumCase: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			if !strings.Contains(m.Content[0].Text, `"SUPPORTED"`) || !strings.Contains(m.Content[0].Text, `"Medical_language"`) {
				t.Fatalf("%s", m.Content[0].Text)
			}
		},
		fakellm.ProviderFaultHTTP429: func(t *testing.T, status int, h http.Header, _ anthropicResp) {
			if status != 429 || h.Get("Retry-After") != "1" {
				t.Fatalf("status %d", status)
			}
		},
		fakellm.ProviderFaultHTTP5xx:  func(t *testing.T, status int, _ http.Header, _ anthropicResp) { expectStatus(t, status, 529) },
		fakellm.ProviderFaultSpendCap: func(t *testing.T, status int, _ http.Header, _ anthropicResp) { expectStatus(t, status, 429) },
		fakellm.ProviderFaultHTTP401:  func(t *testing.T, status int, _ http.Header, _ anthropicResp) { expectStatus(t, status, 401) },
		fakellm.ProviderFaultWrongModel: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			if m.Model != fakellm.ImpostorClaude {
				t.Fatalf("%+v", m)
			}
		},
		fakellm.ProviderFaultReasoningLeak: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			if len(m.Content) != 2 || m.Content[0].Type != "thinking" || !strings.Contains(m.Content[1].Text, fakellm.ReasoningCanary) {
				t.Fatalf("%+v", m)
			}
		},
		fakellm.ProviderFaultEmptyContent: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			if len(m.Content) != 0 || m.StopReason != "end_turn" {
				t.Fatalf("%+v", m)
			}
		},
		fakellm.ProviderFaultPromptInjectionEcho: func(t *testing.T, _ int, _ http.Header, m anthropicResp) {
			var rep report
			_ = json.Unmarshal([]byte(m.Content[0].Text), &rep)
			if rep.Approved == nil || !*rep.Approved || rep.Claims[1].Verdict != "supported" {
				t.Fatalf("%s", m.Content[0].Text)
			}
		},
	}
	srv := newServer(t, fakellm.Config{AllowFaultHeader: true})
	for f, check := range cases {
		resp, raw := send(t, context.Background(), srv.URL, "/v1/messages", anthropicBody(t, nil), map[string]string{fakellm.HeaderProviderFault: string(f)})
		var m anthropicResp
		_ = json.Unmarshal(raw, &m)
		if resp.Header.Get(fakellm.HeaderTestDouble) != "true" {
			t.Errorf("%s: unlabelled", f)
		}
		t.Run(string(f), func(t *testing.T) { check(t, resp.StatusCode, resp.Header, m) })
	}
}

func expectStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d, want %d", got, want)
	}
}

func TestProviderTimeoutAndStall(t *testing.T) {
	srv := newServer(t, fakellm.Config{AllowFaultHeader: true, SlowDelay: 5 * time.Second})
	for _, f := range []fakellm.ProviderFault{fakellm.ProviderFaultTimeout, fakellm.ProviderFaultStall} {
		for _, path := range []string{"/v1/messages", "/chat/completions"} {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			body := anthropicBody(t, nil)
			if path == "/chat/completions" {
				body = deepseekBody(t, verifyUser, nil)
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+path, bytes.NewReader(body))
			req.Header.Set("anthropic-version", "2026-10-21")
			req.Header.Set(fakellm.HeaderProviderFault, string(f))
			start := time.Now()
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_, err = io.ReadAll(resp.Body)
				resp.Body.Close()
			}
			cancel()
			if err == nil || time.Since(start) > 2*time.Second {
				t.Errorf("%s %s: err %v after %s", f, path, err, time.Since(start))
			}
		}
	}
}

type deepseekResp struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string  `json:"content"`
			Reasoning *string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

const genUser = "data\n<<<SELA_INPUT_JSON\n" +
	`{"task":"summary_draft","schema":"sela-ai-generation-v1","language":"id","audience":"adult","spans":[{"id":"s1","role":"item","text":"Aku susah tidur sejak dua bulan."},{"id":"s2","role":"item","text":"Aku belum cerita ke siapa pun."}]}` +
	"\nSELA_INPUT_JSON>>>"

func TestDeepSeekDouble(t *testing.T) {
	srv := newServer(t, fakellm.Config{})
	resp, raw := send(t, context.Background(), srv.URL, "/chat/completions", deepseekBody(t, genUser, nil), nil)
	var m deepseekResp
	_ = json.Unmarshal(raw, &m)
	if resp.StatusCode != 200 || resp.Header.Get(fakellm.HeaderTestDouble) != "true" || m.Model != fakellm.DeepSeekModel ||
		!strings.HasPrefix(m.ID, "chatcmpl-test-double-") || m.Choices[0].Message.Reasoning != nil {
		t.Fatalf("status %d %s", resp.StatusCode, raw)
	}
	var d struct {
		Declined bool   `json:"declined"`
		Risk     string `json:"risk"`
		Items    []struct {
			Text    string   `json:"text"`
			SpanIDs []string `json:"span_ids"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(m.Choices[0].Message.Content), &d); err != nil || len(d.Items) != 2 || d.Items[1].SpanIDs[0] != "s2" || d.Risk != "none" {
		t.Fatalf("generation %s %v", m.Choices[0].Message.Content, err)
	}
	// Thinking on by default: reasoning_content is emitted.
	_, raw = send(t, context.Background(), srv.URL, "/chat/completions", deepseekBody(t, genUser, map[string]any{"thinking": nil}), nil)
	_ = json.Unmarshal(raw, &m)
	if m.Choices[0].Message.Reasoning == nil || !strings.Contains(*m.Choices[0].Message.Reasoning, fakellm.ReasoningCanary) {
		t.Fatalf("no reasoning_content with thinking on: %s", raw)
	}
	// Verifier mode.
	_, raw = send(t, context.Background(), srv.URL, "/chat/completions", deepseekBody(t, verifyUser, nil), nil)
	_ = json.Unmarshal(raw, &m)
	var rep report
	if err := json.Unmarshal([]byte(m.Choices[0].Message.Content), &rep); err != nil || rep.Claims[0].Verdict != "supported" {
		t.Fatalf("report %s", m.Choices[0].Message.Content)
	}
}

func TestDeepSeekDoubleEnforcesContract(t *testing.T) {
	srv := newServer(t, fakellm.Config{})
	cases := map[string][]byte{
		"user_id":     deepseekBody(t, genUser, map[string]any{"user_id": "u-1"}),
		"stream":      deepseekBody(t, genUser, map[string]any{"stream": true}),
		"real model":  deepseekBody(t, genUser, map[string]any{"model": "deepseek-flash"}),
		"no json":     deepseekBody(t, "hello", map[string]any{"messages": []map[string]string{{"role": "user", "content": "hello"}}}),
		"no messages": deepseekBody(t, genUser, map[string]any{"messages": []map[string]string{}}),
	}
	for name, body := range cases {
		resp, raw := send(t, context.Background(), srv.URL, "/chat/completions", body, nil)
		if resp.StatusCode != 400 || resp.Header.Get(fakellm.HeaderTestDouble) != "true" || strings.Contains(string(raw), "u-1") {
			t.Errorf("%s: status %d %s", name, resp.StatusCode, raw)
		}
	}
}

func TestDeepSeekDoubleFaults(t *testing.T) {
	srv := newServer(t, fakellm.Config{AllowFaultHeader: true})
	want := map[fakellm.ProviderFault]func(status int, m deepseekResp) bool{
		fakellm.ProviderFaultRefusal: func(s int, m deepseekResp) bool {
			return s == 200 && m.Choices[0].FinishReason == "content_filter" && m.Choices[0].Message.Content == ""
		},
		fakellm.ProviderFaultTruncation:    func(s int, m deepseekResp) bool { return m.Choices[0].FinishReason == "length" },
		fakellm.ProviderFaultMalformedJSON: func(s int, m deepseekResp) bool { return !json.Valid([]byte(m.Choices[0].Message.Content)) },
		fakellm.ProviderFaultWrongEnumCase: func(s int, m deepseekResp) bool { return strings.Contains(m.Choices[0].Message.Content, `"NONE"`) },
		fakellm.ProviderFaultHTTP429:       func(s int, _ deepseekResp) bool { return s == 429 },
		fakellm.ProviderFaultHTTP5xx:       func(s int, _ deepseekResp) bool { return s == 503 },
		fakellm.ProviderFaultSpendCap:      func(s int, _ deepseekResp) bool { return s == 402 },
		fakellm.ProviderFaultHTTP401:       func(s int, _ deepseekResp) bool { return s == 401 },
		fakellm.ProviderFaultWrongModel:    func(s int, m deepseekResp) bool { return m.Model == fakellm.ImpostorDeepSeek },
		fakellm.ProviderFaultReasoningLeak: func(s int, m deepseekResp) bool {
			return strings.HasPrefix(m.Choices[0].Message.Content, "<think>") && m.Choices[0].Message.Reasoning != nil
		},
		fakellm.ProviderFaultEmptyContent: func(s int, m deepseekResp) bool { return m.Choices[0].Message.Content == "" },
		fakellm.ProviderFaultPromptInjectionEcho: func(s int, m deepseekResp) bool {
			return strings.Contains(m.Choices[0].Message.Content, "IGNORE PREVIOUS INSTRUCTIONS")
		},
	}
	for f, ok := range want {
		resp, raw := send(t, context.Background(), srv.URL, "/chat/completions", deepseekBody(t, genUser, nil), map[string]string{fakellm.HeaderProviderFault: string(f)})
		var m deepseekResp
		_ = json.Unmarshal(raw, &m)
		if resp.Header.Get(fakellm.HeaderTestDouble) != "true" || !ok(resp.StatusCode, m) {
			t.Errorf("%s: status %d %s", f, resp.StatusCode, raw)
		}
	}
}

// TestMetaRouteServesAIGeneration: the Meta test double answers the
// internal/ai generation input and keeps its content faults; the existing
// ledger and opening tasks are unaffected (covered by fakellm_test.go).
func TestMetaRouteServesAIGeneration(t *testing.T) {
	srv := newServer(t, fakellm.Config{AllowFaultHeader: true})
	s := streamContent(t, srv, genUser, nil)
	if !strings.Contains(s.content, `"text":"Aku susah tidur sejak dua bulan."`) || s.finish != "stop" {
		t.Fatalf("content %s", s.content)
	}
	s = streamContent(t, srv, genUser, map[string]string{fakellm.HeaderFault: string(fakellm.FaultDiagnosis)})
	if !strings.Contains(s.content, "kamu mengalami depresi") {
		t.Fatalf("diagnosis fault %s", s.content)
	}
	reply := strings.Replace(genUser, `"summary_draft"`, `"listening_reply"`, 1)
	reply = strings.Replace(reply, `"role":"item"`, `"role":"user"`, -1)
	s = streamContent(t, srv, reply, map[string]string{fakellm.HeaderFault: string(fakellm.FaultPromptInjectionEcho)})
	if !strings.Contains(s.content, "IGNORE PREVIOUS INSTRUCTIONS") {
		t.Fatalf("injection fault %s", s.content)
	}
	decline := strings.Replace(genUser, "Aku belum cerita", fakellm.MarkerDecline+" Aku belum cerita", 1)
	if s = streamContent(t, srv, decline, nil); !strings.Contains(s.content, `"declined":true`) {
		t.Fatalf("decline %s", s.content)
	}
}

func TestParseProviderFault(t *testing.T) {
	for _, f := range fakellm.ProviderFaults() {
		if got, err := fakellm.ParseProviderFault(string(f)); err != nil || got != f {
			t.Fatalf("%s: %v", f, err)
		}
	}
	if f, err := fakellm.ParseProviderFault(""); err != nil || f != fakellm.ProviderFaultNone {
		t.Fatal("empty")
	}
	if _, err := fakellm.ParseProviderFault("explode"); err == nil {
		t.Fatal("unknown accepted")
	}
	if _, err := fakellm.NewHandler(fakellm.Config{ProviderFault: "explode"}); err == nil {
		t.Fatal("handler accepted unknown provider fault")
	}
	if len(fakellm.ProviderFaults()) != 15 || len(fakellm.Faults()) != 15 {
		t.Fatalf("faults %d/%d", len(fakellm.ProviderFaults()), len(fakellm.Faults()))
	}
}

func TestProviderRoutesRequireKey(t *testing.T) {
	srv := newServer(t, fakellm.Config{APIKey: "double-key"})
	for _, path := range []string{"/v1/messages", "/chat/completions"} {
		resp, _ := send(t, context.Background(), srv.URL, path, anthropicBody(t, nil), nil)
		if resp.StatusCode != 401 {
			t.Errorf("%s without key: %d", path, resp.StatusCode)
		}
		resp, _ = send(t, context.Background(), srv.URL, path, nil, map[string]string{"Authorization": "Bearer double-key"})
		if resp.StatusCode == 401 {
			t.Errorf("%s with key: 401", path)
		}
		r2, _ := http.Get(srv.URL + path)
		r2.Body.Close()
		if r2.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: %d", path, r2.StatusCode)
		}
	}
}

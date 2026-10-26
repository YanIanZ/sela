package fakellm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// Identities of the provider test doubles. They are TEST DOUBLES: never
// Claude, never DeepSeek. Every response carries HeaderTestDouble, a model
// name that says so and an id with the "test_double" prefix.
const (
	// AnthropicModel is the only model POST /v1/messages accepts and
	// reports.
	AnthropicModel = "test-double-not-claude"
	// DeepSeekModel is the only model POST /chat/completions accepts and
	// reports.
	DeepSeekModel = "test-double-not-deepseek"
	// ImpostorClaude and ImpostorDeepSeek are reported under
	// ProviderFaultWrongModel.
	ImpostorClaude   = "claude-impostor"
	ImpostorDeepSeek = "deepseek-impostor"
	// ReasoningCanary is the hidden-reasoning text the doubles emit when
	// thinking is on or under ProviderFaultReasoningLeak. A client must
	// never surface it.
	ReasoningCanary = "TEST-DOUBLE-REASONING-CANARY"
	// HeaderProviderFault selects a provider fault per request when
	// Config.AllowFaultHeader is set.
	HeaderProviderFault = "X-Fake-Provider-Fault"
	// VerifyOpenMarker and VerifyCloseMarker wrap the verifier input
	// (internal/ai/verifier).
	VerifyOpenMarker  = "<<<SELA_VERIFY_JSON"
	VerifyCloseMarker = "SELA_VERIFY_JSON>>>"
	// AIGenerationSchema marks the internal/ai generation input.
	AIGenerationSchema = "sela-ai-generation-v1"
)

// Control markers a test may put in a user span to make the generator
// doubles decline or raise the risk flag (test doubles only).
const (
	MarkerDecline      = "TEST_DOUBLE_DECLINE"
	MarkerRiskCrisis   = "TEST_DOUBLE_RISK_CRISIS"
	MarkerRiskElevated = "TEST_DOUBLE_RISK_ELEVATED"
)

// ProviderFault selects an injected failure for the Anthropic-compatible
// and DeepSeek-compatible routes. It is separate from Fault, which keeps
// the Meta route's behaviour unchanged.
type ProviderFault string

// Provider faults.
const (
	ProviderFaultNone ProviderFault = "none"
	// ProviderFaultRefusal: Anthropic stop_reason "refusal" (HTTP 200);
	// DeepSeek finish_reason "content_filter" with no content.
	ProviderFaultRefusal ProviderFault = "refusal"
	// ProviderFaultTruncation: Anthropic stop_reason "max_tokens";
	// DeepSeek finish_reason "length"; half the content.
	ProviderFaultTruncation ProviderFault = "truncation"
	// ProviderFaultMalformedJSON returns half the content with a normal
	// stop reason.
	ProviderFaultMalformedJSON ProviderFault = "malformed_json"
	// ProviderFaultWrongEnumCase upper-cases verdicts and categories (and
	// the generator risk flag).
	ProviderFaultWrongEnumCase ProviderFault = "wrong_enum_case"
	// ProviderFaultHTTP429 answers 429 with "Retry-After: 1".
	ProviderFaultHTTP429 ProviderFault = "http_429"
	// ProviderFaultHTTP5xx answers 529 overloaded (Anthropic) or 503
	// (DeepSeek).
	ProviderFaultHTTP5xx ProviderFault = "http_5xx"
	// ProviderFaultSpendCap answers 429 enforced_spend_limit_reached
	// (Anthropic) or 402 insufficient balance (DeepSeek).
	ProviderFaultSpendCap ProviderFault = "spend_cap"
	// ProviderFaultHTTP401 answers 401.
	ProviderFaultHTTP401 ProviderFault = "http_401"
	// ProviderFaultTimeout waits Config.SlowDelay before answering.
	ProviderFaultTimeout ProviderFault = "timeout"
	// ProviderFaultStall sends the headers and part of the body, then
	// waits for the client to go away.
	ProviderFaultStall ProviderFault = "stall"
	// ProviderFaultWrongModel reports an impostor model.
	ProviderFaultWrongModel ProviderFault = "wrong_model"
	// ProviderFaultReasoningLeak adds hidden reasoning (an Anthropic
	// thinking block; DeepSeek reasoning_content) and leaks it into the
	// visible text.
	ProviderFaultReasoningLeak ProviderFault = "reasoning_leak"
	// ProviderFaultEmptyContent returns no content.
	ProviderFaultEmptyContent ProviderFault = "empty_content"
	// ProviderFaultPromptInjectionEcho echoes an injected instruction: a
	// verifier report gains "approved": true and marks every claim
	// supported; a generation's first item becomes the injection.
	ProviderFaultPromptInjectionEcho ProviderFault = "prompt_injection_echo"
)

var allProviderFaults = []ProviderFault{
	ProviderFaultNone, ProviderFaultRefusal, ProviderFaultTruncation, ProviderFaultMalformedJSON, ProviderFaultWrongEnumCase,
	ProviderFaultHTTP429, ProviderFaultHTTP5xx, ProviderFaultSpendCap, ProviderFaultHTTP401, ProviderFaultTimeout,
	ProviderFaultStall, ProviderFaultWrongModel, ProviderFaultReasoningLeak, ProviderFaultEmptyContent,
	ProviderFaultPromptInjectionEcho,
}

// ProviderFaults lists every provider fault.
func ProviderFaults() []ProviderFault { return append([]ProviderFault(nil), allProviderFaults...) }

// ParseProviderFault parses a provider fault name; "" means none.
func ParseProviderFault(s string) (ProviderFault, error) {
	if s == "" {
		return ProviderFaultNone, nil
	}
	for _, f := range allProviderFaults {
		if string(f) == s {
			return f, nil
		}
	}
	return "", errors.New("fakellm: unknown provider fault")
}

// providerFault returns the request's provider fault, answering 400 when
// the header is not allowed or unknown.
func (h *handler) providerFault(w http.ResponseWriter, r *http.Request, writeErr func(http.ResponseWriter, int, string)) (ProviderFault, bool) {
	fault := h.cfg.ProviderFault
	if v := r.Header.Get(HeaderProviderFault); v != "" {
		f, err := ParseProviderFault(v)
		if !h.cfg.AllowFaultHeader || err != nil {
			writeErr(w, http.StatusBadRequest, "fault_header_rejected")
			return "", false
		}
		fault = f
	}
	return fault, true
}

// readBody decodes a JSON object body into a field map.
func readBody(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, int) {
	body := http.MaxBytesReader(w, r.Body, maxRequestBytes)
	var raw map[string]json.RawMessage
	err := json.NewDecoder(body).Decode(&raw)
	if err == nil {
		_, err = io.Copy(io.Discard, body)
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, http.StatusRequestEntityTooLarge
		}
		return nil, http.StatusBadRequest
	}
	return raw, 0
}

func decodeField(raw map[string]json.RawMessage, key string, v any) bool {
	f, ok := raw[key]
	if !ok {
		return false
	}
	return json.Unmarshal(f, v) == nil
}

// transportFault answers the status faults; it reports whether it wrote a
// response.
func transportFault(w http.ResponseWriter, r *http.Request, fault ProviderFault, h *handler, writeErr func(http.ResponseWriter, int, string), anthropic bool) bool {
	switch fault {
	case ProviderFaultHTTP429:
		w.Header().Set("Retry-After", "1")
		writeErr(w, http.StatusTooManyRequests, "rate_limit_error")
		return true
	case ProviderFaultSpendCap:
		if anthropic {
			writeErr(w, http.StatusTooManyRequests, "enforced_spend_limit_reached")
		} else {
			writeErr(w, http.StatusPaymentRequired, "insufficient_balance")
		}
		return true
	case ProviderFaultHTTP5xx:
		if anthropic {
			writeErr(w, 529, "overloaded_error")
		} else {
			writeErr(w, http.StatusServiceUnavailable, "server_overloaded")
		}
		return true
	case ProviderFaultHTTP401:
		writeErr(w, http.StatusUnauthorized, "authentication_error")
		return true
	case ProviderFaultTimeout:
		if !sleep(r, h.cfg.SlowDelay) {
			return true
		}
	case ProviderFaultStall:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":"test_double_stalled",`)
		_ = http.NewResponseController(w).Flush()
		<-r.Context().Done()
		return true
	}
	return false
}

// writeAnthropicError writes an Anthropic-style error naming the test
// double; it never echoes request content.
func writeAnthropicError(w http.ResponseWriter, status int, code string) {
	typ := code
	switch code {
	case "enforced_spend_limit_reached":
		typ = "rate_limit_error"
	case "fault_header_rejected", "invalid_json", "request_too_large":
		typ = "invalid_request_error"
	}
	writeJSON(w, status, map[string]any{
		"type":  "error",
		"error": map[string]string{"type": typ, "message": "test double: " + code},
	})
}

// anthropicMessages serves POST /v1/messages: a TEST DOUBLE of the
// Anthropic Messages API that enforces the request contract Sela's
// verifier must keep (no sampling parameters, no fallbacks, metadata or
// tools, no assistant prefill, a valid thinking setting, an
// anthropic-version header) and answers with deterministic verdicts.
func (h *handler) anthropicMessages(w http.ResponseWriter, r *http.Request) {
	fault, ok := h.providerFault(w, r, writeAnthropicError)
	if !ok {
		return
	}
	if r.Header.Get("anthropic-version") == "" {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error")
		return
	}
	raw, status := readBody(w, r)
	if status != 0 {
		code := "invalid_json"
		if status == http.StatusRequestEntityTooLarge {
			code = "request_too_large"
		}
		writeAnthropicError(w, status, code)
		return
	}
	for _, k := range []string{"temperature", "top_p", "top_k", "fallbacks", "metadata", "tools", "tool_choice", "stop_sequences"} {
		if _, has := raw[k]; has {
			writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error")
			return
		}
	}
	var model string
	var maxTokens int
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	var thinking struct {
		Type string `json:"type"`
	}
	var outputConfig struct {
		Effort string `json:"effort"`
		Format *struct {
			Type string `json:"type"`
		} `json:"format"`
	}
	decodeField(raw, "model", &model)
	decodeField(raw, "max_tokens", &maxTokens)
	decodeField(raw, "messages", &messages)
	decodeField(raw, "thinking", &thinking)
	decodeField(raw, "output_config", &outputConfig)
	if model != AnthropicModel {
		writeAnthropicError(w, http.StatusNotFound, "not_found_error")
		return
	}
	badThinking := thinking.Type != "" && thinking.Type != "adaptive" && thinking.Type != "between_tools"
	badEffort := thinking.Type == "between_tools" && (outputConfig.Effort == "xhigh" || outputConfig.Effort == "max")
	if maxTokens <= 0 || len(messages) == 0 || messages[len(messages)-1].Role != "user" || badThinking || badEffort {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error")
		return
	}
	if transportFault(w, r, fault, h, writeAnthropicError, true) {
		return
	}
	user := messageText(messages[len(messages)-1].Content)
	text := verifierText(user, fault)
	stop := "end_turn"
	content := []map[string]string{{"type": "text", "text": text}}
	var stopDetails any
	switch fault {
	case ProviderFaultRefusal:
		stop, content = "refusal", []map[string]string{}
		stopDetails = map[string]string{"type": "refusal", "category": "general_harms", "explanation": "test double refusal"}
	case ProviderFaultTruncation:
		stop, content = "max_tokens", []map[string]string{{"type": "text", "text": cutBytes(text, len(text)/2)}}
	case ProviderFaultMalformedJSON:
		content = []map[string]string{{"type": "text", "text": cutBytes(text, len(text)/2)}}
	case ProviderFaultReasoningLeak:
		content = []map[string]string{
			{"type": "thinking", "thinking": ReasoningCanary + " step by step", "signature": "test-double"},
			{"type": "text", "text": "<thinking>" + ReasoningCanary + "</thinking>" + text},
		}
	case ProviderFaultEmptyContent:
		content = []map[string]string{}
	}
	respModel := AnthropicModel
	if fault == ProviderFaultWrongModel {
		respModel = ImpostorClaude
	}
	prompt := 0
	for _, m := range messages {
		prompt += len(messageText(m.Content))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": fmt.Sprintf("msg_test_double_%016x", fnv64(text)), "type": "message", "role": "assistant",
		"model": respModel, "content": content, "stop_reason": stop, "stop_sequence": nil, "stop_details": stopDetails,
		"usage": map[string]int{"input_tokens": tokens(prompt), "output_tokens": tokens(len(text))},
	})
}

// writeDeepSeekError writes an OpenAI-style error naming the test double.
func writeDeepSeekError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"message": "test double: " + code, "type": code, "code": code},
	})
}

// deepseekChat serves POST /chat/completions: a TEST DOUBLE of DeepSeek's
// OpenAI-compatible API (non-streaming only). Like the real API in JSON
// mode it requires the word "json" in the prompt; it refuses user_id. When
// thinking is not disabled it emits reasoning_content, as DeepSeek does by
// default.
func (h *handler) deepseekChat(w http.ResponseWriter, r *http.Request) {
	fault, ok := h.providerFault(w, r, writeDeepSeekError)
	if !ok {
		return
	}
	raw, status := readBody(w, r)
	if status != 0 {
		writeDeepSeekError(w, status, "invalid_json")
		return
	}
	if _, has := raw["user_id"]; has {
		writeDeepSeekError(w, http.StatusBadRequest, "invalid_request_error")
		return
	}
	var model string
	var stream bool
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	var format, thinking struct {
		Type string `json:"type"`
	}
	decodeField(raw, "model", &model)
	decodeField(raw, "stream", &stream)
	decodeField(raw, "messages", &messages)
	hasFormat := decodeField(raw, "response_format", &format)
	hasThinking := decodeField(raw, "thinking", &thinking)
	if model != DeepSeekModel {
		writeDeepSeekError(w, http.StatusBadRequest, "model_not_exist")
		return
	}
	if stream || len(messages) == 0 {
		writeDeepSeekError(w, http.StatusBadRequest, "invalid_request_error")
		return
	}
	all, user := "", ""
	for _, m := range messages {
		t := messageText(m.Content)
		all += t
		if m.Role == "user" {
			user = t
		}
	}
	if hasFormat && format.Type == "json_object" && !strings.Contains(strings.ToLower(all), "json") {
		writeDeepSeekError(w, http.StatusBadRequest, "invalid_request_error")
		return
	}
	if transportFault(w, r, fault, h, writeDeepSeekError, false) {
		return
	}
	var text string
	if strings.Contains(user, VerifyOpenMarker) {
		text = verifierText(user, fault)
	} else {
		text = providerGeneration(user, fault)
	}
	finish := "stop"
	switch fault {
	case ProviderFaultRefusal:
		finish, text = "content_filter", ""
	case ProviderFaultTruncation:
		finish, text = "length", cutBytes(text, len(text)/2)
	case ProviderFaultMalformedJSON:
		text = cutBytes(text, len(text)/2)
	case ProviderFaultReasoningLeak:
		text = "<think>" + ReasoningCanary + "</think>" + text
	case ProviderFaultEmptyContent:
		text = ""
	}
	msg := map[string]any{"role": "assistant", "content": text}
	if !hasThinking || thinking.Type != "disabled" || fault == ProviderFaultReasoningLeak {
		msg["reasoning_content"] = ReasoningCanary + " (test double reasoning)"
	}
	respModel := DeepSeekModel
	if fault == ProviderFaultWrongModel {
		respModel = ImpostorDeepSeek
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": fmt.Sprintf("chatcmpl-test-double-%016x", fnv64(text)), "object": "chat.completion", "created": created,
		"model": respModel, "system_fingerprint": "test_double",
		"choices": []map[string]any{{"index": 0, "message": msg, "finish_reason": finish}},
		"usage":   usage{tokens(len(all)), tokens(len(text)), tokens(len(all)) + tokens(len(text))},
	})
}

// verifyInput is the VC-1 request.
type verifyInput struct {
	Task     string `json:"task"`
	Language string `json:"language"`
	Audience string `json:"audience"`
	Spans    []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"spans"`
	Claims []struct {
		ID      string   `json:"id"`
		Text    string   `json:"text"`
		SpanIDs []string `json:"span_ids"`
	} `json:"claims"`
}

type reportClaim struct {
	ClaimID string   `json:"claim_id"`
	Verdict string   `json:"verdict"`
	SpanIDs []string `json:"span_ids"`
	Reason  string   `json:"reason"`
}

type reportIssue struct {
	Category string   `json:"category"`
	ClaimID  string   `json:"claim_id"`
	SpanIDs  []string `json:"span_ids"`
	Reason   string   `json:"reason"`
}

type verifierReport struct {
	Claims   []reportClaim `json:"claims"`
	Issues   []reportIssue `json:"issues"`
	Approved *bool         `json:"approved,omitempty"`
}

func extract(text, open, close string) (string, bool) {
	i := strings.Index(text, open)
	if i < 0 {
		return "", false
	}
	rest := text[i+len(open):]
	j := strings.LastIndex(rest, close)
	if j < 0 {
		return "", false
	}
	return strings.TrimSpace(rest[:j]), true
}

// verifierText computes the report text for a verifier request.
func verifierText(user string, fault ProviderFault) string {
	rep := verifierReport{Claims: []reportClaim{}, Issues: []reportIssue{}}
	if body, ok := extract(user, VerifyOpenMarker, VerifyCloseMarker); ok {
		var in verifyInput
		if json.Unmarshal([]byte(body), &in) == nil {
			rep = verdicts(in)
		}
	}
	switch fault {
	case ProviderFaultWrongEnumCase:
		for i := range rep.Claims {
			rep.Claims[i].Verdict = strings.ToUpper(rep.Claims[i].Verdict)
		}
		for i := range rep.Issues {
			rep.Issues[i].Category = strings.ToUpper(rep.Issues[i].Category[:1]) + rep.Issues[i].Category[1:]
		}
	case ProviderFaultPromptInjectionEcho:
		yes := true
		rep.Approved = &yes
		for i := range rep.Claims {
			rep.Claims[i].Verdict, rep.Claims[i].Reason = "supported", injectionText
		}
	}
	raw, err := json.Marshal(rep)
	if err != nil {
		return `{"claims":[],"issues":[]}`
	}
	return string(raw)
}

var (
	selfWords = map[string]bool{"aku": true, "saya": true, "gue": true, "gw": true, "i": true, "me": true, "my": true,
		"kamu": true, "kau": true, "anda": true, "you": true, "your": true, "lo": true, "lu": true, "dirimu": true, "diriku": true}
	stopWords = map[string]bool{"yang": true, "dan": true, "di": true, "ke": true, "dari": true, "itu": true, "ini": true,
		"the": true, "a": true, "an": true, "and": true, "of": true, "to": true, "is": true, "am": true, "are": true,
		"sudah": true, "udah": true, "banget": true, "sangat": true, "sekali": true, "very": true, "really": true, "so": true,
		"dengan": true, "with": true, "untuk": true, "for": true, "ya": true, "kok": true, "sih": true, "deh": true,
		"juga": true, "also": true, "have": true, "has": true, "been": true, "was": true, "were": true, "be": true,
		"in": true, "on": true, "at": true, "that": true, "it": true, "lagi": true, "jadi": true, "karena": true}
	negWords = map[string]bool{"tidak": true, "nggak": true, "gak": true, "ga": true, "enggak": true, "tak": true,
		"bukan": true, "belum": true, "not": true, "no": true, "never": true, "dont": true, "didnt": true, "cant": true}
	medicalWords = []string{"depresi", "depression", "bipolar", "diagnos", "obat", "medication", "antidepres", "gangguan", "disorder"}
	crisisWords  = []string{"bunuh diri", "suicide", "mengakhiri hidup", "kill myself", "end my life", "self-harm", "melukai diri"}
	manipWords   = []string{"cuma aku", "only i", "you need me", "kamu butuh aku", "nobody else"}
)

// contentTokens returns the normalized content words of s: lowercase
// letters and digits, pronouns mapped to one token, -ku/-mu/-nya suffixes
// stripped, stop words dropped.
func contentTokens(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(strings.ReplaceAll(s, "'", "")), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if selfWords[f] {
			out = append(out, "@self")
			continue
		}
		for _, suf := range []string{"nya", "ku", "mu"} {
			if len(f) > len(suf)+3 && strings.HasSuffix(f, suf) {
				f = strings.TrimSuffix(f, suf)
				break
			}
		}
		if !stopWords[f] {
			out = append(out, f)
		}
	}
	return out
}

func negated(tokens []string) bool {
	for _, t := range tokens {
		if negWords[t] {
			return true
		}
	}
	return false
}

func containsAny(s string, words []string) bool {
	s = strings.ToLower(s)
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// verdicts is the test double's deterministic, lexical "verification". It
// is not a verifier: it exists so routing and release code can be tested
// on every verdict and issue.
func verdicts(in verifyInput) verifierReport {
	spans := map[string]string{}
	for _, s := range in.Spans {
		spans[s.ID] = s.Text
	}
	rep := verifierReport{Claims: []reportClaim{}, Issues: []reportIssue{}}
	for _, c := range in.Claims {
		var src strings.Builder
		words := 0
		for _, id := range c.SpanIDs {
			src.WriteString(spans[id])
			src.WriteByte(' ')
			words += len(strings.Fields(spans[id]))
		}
		ct, sTok := contentTokens(c.Text), contentTokens(src.String())
		have := map[string]bool{}
		for _, t := range sTok {
			have[t] = true
		}
		hits := 0
		for _, t := range ct {
			if have[t] {
				hits++
			}
		}
		overlap := 0.0
		if len(ct) > 0 {
			overlap = float64(hits) / float64(len(ct))
		}
		negDiff := negated(ct) != negated(sTok)
		verdict, reason := "insufficient_evidence", "test double: the cited spans do not decide the claim"
		switch {
		case words < 3 || len(ct) == 0:
		case overlap == 1 && !negDiff:
			verdict, reason = "supported", "test double: the claim's words appear in the cited spans"
		case overlap >= 0.5 && negDiff:
			verdict, reason = "contradicted", "test double: the negation differs from the cited spans"
		case overlap > 0:
			verdict, reason = "unsupported", "test double: the claim adds words that the cited spans do not contain"
		}
		ids := append([]string{}, c.SpanIDs...)
		rep.Claims = append(rep.Claims, reportClaim{ClaimID: c.ID, Verdict: verdict, SpanIDs: ids, Reason: reason})
		if containsAny(c.Text, medicalWords) {
			rep.Issues = append(rep.Issues, reportIssue{Category: "medical_language", ClaimID: c.ID, SpanIDs: []string{},
				Reason: "test double: medical term in the claim"})
		}
		if strings.Contains(c.Text, "@") || strings.Contains(strings.ToLower(c.Text), "http") || longDigitRun(c.Text) {
			rep.Issues = append(rep.Issues, reportIssue{Category: "information_leak", ClaimID: c.ID, SpanIDs: []string{},
				Reason: "test double: contact detail in the claim"})
		}
		if containsAny(c.Text, manipWords) {
			rep.Issues = append(rep.Issues, reportIssue{Category: "manipulation", ClaimID: c.ID, SpanIDs: []string{},
				Reason: "test double: exclusivity or dependency wording"})
		}
	}
	for _, s := range in.Spans {
		if containsAny(s.Text, crisisWords) {
			rep.Issues = append(rep.Issues, reportIssue{Category: "safety_signal", ClaimID: "", SpanIDs: []string{s.ID},
				Reason: "test double: risk language in a span"})
		}
	}
	return rep
}

func longDigitRun(s string) bool {
	run := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			run++
			if run >= 8 {
				return true
			}
		} else if r != ' ' && r != '-' {
			run = 0
		}
	}
	return false
}

// aiInput is the internal/ai generation input.
type aiInput struct {
	Task     string `json:"task"`
	Schema   string `json:"schema"`
	Language string `json:"language"`
	Spans    []struct {
		ID   string `json:"id"`
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"spans"`
}

type aiItem struct {
	Text    string   `json:"text"`
	SpanIDs []string `json:"span_ids"`
}

type aiDraft struct {
	Declined bool     `json:"declined"`
	Risk     string   `json:"risk"`
	Items    []aiItem `json:"items"`
}

func parseAIInput(text string) (aiInput, bool) {
	body, ok := extract(text, InputOpenMarker, InputCloseMarker)
	if !ok {
		return aiInput{}, false
	}
	var in aiInput
	if json.Unmarshal([]byte(body), &in) != nil || in.Schema != AIGenerationSchema {
		return aiInput{}, false
	}
	return in, true
}

// aiGenerate answers an internal/ai generation by quoting the spans
// verbatim (a fixed neutral line for a listening reply), with the Meta
// content faults applied.
func aiGenerate(in aiInput, fault Fault) aiDraft {
	d := aiDraft{Risk: "none", Items: []aiItem{}}
	var user []int
	for i, s := range in.Spans {
		if s.Role == "user" || s.Role == "item" || s.Role == "memory" {
			user = append(user, i)
			switch {
			case strings.Contains(s.Text, MarkerDecline):
				d.Declined = true
			case strings.Contains(s.Text, MarkerRiskCrisis):
				d.Risk = "crisis"
			case strings.Contains(s.Text, MarkerRiskElevated) && d.Risk == "none":
				d.Risk = "elevated"
			}
		}
	}
	if d.Declined {
		return d
	}
	quote := func(i, max int) aiItem {
		return aiItem{Text: clipWords(strings.TrimSpace(in.Spans[i].Text), max), SpanIDs: []string{in.Spans[i].ID}}
	}
	switch in.Task {
	case "listening_reply":
		text := "Terima kasih sudah cerita. Aku mendengarkan."
		if in.Language == "en" {
			text = "Thank you for telling me. I am listening."
		}
		it := aiItem{Text: text, SpanIDs: []string{}}
		if len(user) > 0 {
			it.SpanIDs = []string{in.Spans[user[len(user)-1]].ID}
		}
		d.Items = append(d.Items, it)
	case "reflection":
		if len(user) > 0 {
			d.Items = append(d.Items, quote(user[len(user)-1], 600))
		}
	case "memory_proposal":
		for _, i := range user {
			if len(d.Items) == 5 {
				break
			}
			d.Items = append(d.Items, quote(i, 300))
		}
	case "summary_draft":
		for _, i := range user {
			d.Items = append(d.Items, quote(i, 800))
		}
	case "opening_lines":
		for _, i := range user {
			if len(d.Items) == 3 {
				break
			}
			d.Items = append(d.Items, quote(i, 240))
		}
	}
	if len(d.Items) > 0 {
		switch fault {
		case FaultFabricatedSource:
			d.Items[0].SpanIDs = []string{fabricatedID}
		case FaultNegationFlip:
			for i := range d.Items {
				d.Items[i].Text = flipNegation(d.Items[i].Text, in.Language)
			}
		case FaultDiagnosis, FaultInventedNumber, FaultPromptInjectionEcho:
			d.Items[0].Text = mutate(d.Items[0].Text, fault)
		}
	}
	return d
}

// providerGeneration answers a generation on the DeepSeek route.
func providerGeneration(user string, fault ProviderFault) string {
	in, ok := parseAIInput(user)
	if !ok {
		return unknownTask
	}
	d := aiGenerate(in, FaultNone)
	switch fault {
	case ProviderFaultPromptInjectionEcho:
		if len(d.Items) > 0 {
			d.Items[0].Text = injectionText
		}
	case ProviderFaultWrongEnumCase:
		d.Risk = strings.ToUpper(d.Risk)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return unknownTask
	}
	return string(raw)
}

package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/research"
	"github.com/YanIanZ/sela/internal/store"
)

// scripted is a TEST-ONLY provider; never Meta inference.
type scripted struct{}

func (scripted) Info() llm.Info {
	return llm.Info{Mode: llm.ModeTestDouble, ServedModel: "test-double-not-meta"}
}
func (scripted) Health(context.Context) llm.Health { return llm.Health{State: "ok"} }
func (scripted) Complete(ctx context.Context, req llm.Request, onDelta func(string) error) (llm.Result, error) {
	msg := req.Messages[len(req.Messages)-1].Content
	i, j := strings.Index(msg, journey.InputOpenMarker), strings.Index(msg, journey.InputCloseMarker)
	var in struct {
		Task     string `json:"task"`
		Segments []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"segments"`
	}
	_ = json.Unmarshal([]byte(msg[i+len(journey.InputOpenMarker):j]), &in)
	var claims []string
	for _, s := range in.Segments {
		q, _ := json.Marshal(s.Text)
		claims = append(claims, fmt.Sprintf(`{"segment_id":%q,"quote":%s,"kind":"experience","text":%s,"subject":"self","time":"unspecified","certainty":"stated","negated":false}`, s.ID, q, q))
	}
	text := `{"claims":[` + strings.Join(claims, ",") + `],"unknowns":[],"clarification":{"field":"none"}}`
	if onDelta != nil {
		for k := 0; k < len(text); k += 11 {
			end := min(k+11, len(text))
			if err := onDelta(text[k:end]); err != nil {
				return llm.Result{}, err
			}
		}
	}
	return llm.Result{Text: text, Model: "test-double-not-meta"}, nil
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

type env struct {
	t    *testing.T
	srv  *httptest.Server
	logs *syncBuffer
	svc  *journey.Service
	h    *httpapi.Server
}

func newEnv(t *testing.T, webDir string) *env {
	return newEnvWith(t, httpapi.Options{WebDir: webDir}, scripted{})
}

func newEnvWith(t *testing.T, opt httpapi.Options, provider llm.Provider) *env {
	t.Helper()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	cat := fixtures.MustLoad()
	svc := journey.New(journey.Config{Logger: logger, OpTimeout: 5 * time.Second}, store.NewMemory(store.Options{}), cat, provider, llm.NewAdmission(2))
	opt.Logger, opt.Version = logger, "test"
	h := httpapi.New(svc, opt)
	srv := httptest.NewServer(h)
	t.Cleanup(func() { svc.Wait(); srv.Close() })
	return &env{t: t, srv: srv, logs: logs, svc: svc, h: h}
}

type client struct {
	e  *env
	hc *http.Client
}

func (e *env) client() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, hc: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
}

func (c *client) do(method, path string, body any, hdr map[string]string) (*http.Response, map[string]any) {
	c.e.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("X-Sela-Request", "1")
		req.Header.Set("Origin", c.e.srv.URL)
	}
	for k, v := range hdr {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	res, err := c.hc.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func data(m map[string]any) map[string]any { d, _ := m["data"].(map[string]any); return d }
func errCode(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}
func version(m map[string]any) int {
	d := data(m)
	if s, ok := d["session"].(map[string]any); ok {
		d = s
	}
	v, _ := d["version"].(float64)
	return int(v)
}

func (c *client) start(lang string) map[string]any {
	c.e.t.Helper()
	cat := fixtures.MustLoad()
	res, body := c.do("POST", "/api/v2/sessions", map[string]any{"language": lang, "adultAttested": true, "privacyPolicy": cat.Policies["privacy"].Version, "identity": map[string]any{"mode": "anonymous"}}, nil)
	if res.StatusCode != 201 {
		c.e.t.Fatalf("create session: %d %v", res.StatusCode, body)
	}
	return body
}

func TestMutationGuardRejectsCrossSite(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	for name, hdr := range map[string]map[string]string{
		"missing custom header": {"X-Sela-Request": ""},
		"foreign origin":        {"Origin": "https://evil.example"},
		"cross-site fetch":      {"Sec-Fetch-Site": "cross-site"},
	} {
		res, body := c.do("POST", "/api/v2/sessions", map[string]any{"language": "id"}, hdr)
		if res.StatusCode != 403 || errCode(body) != "cross_origin" {
			t.Errorf("%s: got %d %v", name, res.StatusCode, body)
		}
	}
}

func TestSessionCookieAttributesAndStrictJSON(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	cat := fixtures.MustLoad()
	res, _ := c.do("POST", "/api/v2/sessions", map[string]any{"language": "id", "adultAttested": true, "privacyPolicy": cat.Policies["privacy"].Version}, nil)
	setCookie := res.Header.Get("Set-Cookie")
	for _, want := range []string{"sela_session=", "HttpOnly", "SameSite=Strict", "Path=/api/v2"} {
		if !strings.Contains(setCookie, want) {
			t.Errorf("cookie %q missing %q", setCookie, want)
		}
	}
	res, body := c.do("POST", "/api/v2/sessions", map[string]any{"language": "id", "adultAttested": true, "privacyPolicy": "x", "isAdmin": true}, nil)
	if res.StatusCode != 400 || errCode(body) != "invalid_input" {
		t.Errorf("unknown field accepted: %d %v", res.StatusCode, body)
	}
	big := strings.Repeat("a", 40<<10)
	res, body = c.do("PUT", "/api/v2/story", map[string]any{"expectedVersion": 1, "mode": "practice", "text": big}, nil)
	if res.StatusCode != 413 {
		t.Errorf("oversized body: %d %v", res.StatusCode, body)
	}
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/v2/sessions", strings.NewReader("language=id"))
	req.Header.Set("X-Sela-Request", "1")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2, _ := http.DefaultClient.Do(req)
	if r2.StatusCode != 415 {
		t.Errorf("form body: %d", r2.StatusCode)
	}
}

func TestSecurityHeadersAndUnknownAPIRoute(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	res, body := c.do("GET", "/api/v2/nope", nil, nil)
	if res.StatusCode != 404 || errCode(body) != "not_found" {
		t.Errorf("unknown route: %d %v", res.StatusCode, body)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy", "X-Frame-Options"} {
		if res.Header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Error("CSP lacks frame-ancestors")
	}
}

func TestLocalizedErrorsAndSessionMissing(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	res, body := c.do("GET", "/api/v2/session", nil, map[string]string{"X-Sela-Language": "en"})
	if res.StatusCode != 401 || errCode(body) != "session_missing" || !strings.Contains(fmt.Sprint(body), "demo session") {
		t.Errorf("en error: %d %v", res.StatusCode, body)
	}
	_, body = c.do("GET", "/api/v2/session", nil, map[string]string{"X-Sela-Language": "id"})
	if !strings.Contains(fmt.Sprint(body), "Sesi demo") {
		t.Errorf("id error: %v", body)
	}
}

// TestFullJourneyOverHTTP drives the whole owner + recipient flow through the
// real handler with a scripted (test-double) model, and checks that logs
// contain no story text, tokens or codes.
func TestFullJourneyOverHTTP(t *testing.T) {
	e := newEnv(t, "")
	owner := e.client()
	cat := fixtures.MustLoad()
	b := owner.start("id")
	v := version(b)
	_, b = owner.do("PUT", "/api/v2/consents/ai_processing", map[string]any{"expectedVersion": v, "granted": true, "policyVersion": cat.Policies["ai_processing"].Version}, nil)
	v = version(b)
	canary := "Tokoh fiktif CANARY-7f3a9 merasa lelah dengan pekerjaan barunya setiap hari."
	res, b := owner.do("PUT", "/api/v2/story", map[string]any{"expectedVersion": v, "mode": "practice", "text": canary}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("story: %d %v", res.StatusCode, b)
	}
	v = version(b)
	res, b = owner.do("POST", "/api/v2/ledger/draft", map[string]any{"expectedVersion": v}, nil)
	if res.StatusCode != 202 {
		t.Fatalf("draft: %d %v", res.StatusCode, b)
	}
	op := data(b)["operation"].(map[string]any)["id"].(string)

	// SSE: read until done.
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/v2/operations/"+op+"/events", nil)
	sse, err := owner.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if ct := sse.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q", ct)
	}
	sc := bufio.NewScanner(sse.Body)
	var events []string
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			events = append(events, strings.TrimPrefix(line, "event: "))
		}
		if line == "event: done" || line == "event: error" {
			break
		}
	}
	sse.Body.Close()
	if len(events) == 0 || events[len(events)-1] != "done" {
		t.Fatalf("events: %v", events)
	}
	// A second subscriber with Last-Event-ID 1 replays from event 2.
	req, _ = http.NewRequest("GET", e.srv.URL+"/api/v2/operations/"+op+"/events", nil)
	req.Header.Set("Last-Event-ID", "1")
	replay, _ := owner.hc.Do(req)
	raw, _ := io.ReadAll(replay.Body)
	replay.Body.Close()
	if strings.Contains(string(raw), "id: 1\n") || !strings.Contains(string(raw), "event: done") {
		t.Fatalf("replay did not honour Last-Event-ID: %s", raw)
	}
	// Another visitor cannot subscribe.
	other := e.client()
	other.start("en")
	res, _ = other.do("GET", "/api/v2/operations/"+op+"/events", nil, nil)
	if res.StatusCode != 404 {
		t.Fatalf("foreign subscriber: %d", res.StatusCode)
	}

	_, b = owner.do("GET", "/api/v2/session", nil, nil)
	sess := data(b)
	v = int(sess["version"].(float64))
	items := sess["ledger"].(map[string]any)["items"].([]any)
	for _, it := range items {
		id := it.(map[string]any)["id"].(string)
		_, b = owner.do("PATCH", "/api/v2/ledger/items/"+id, map[string]any{"expectedVersion": v, "action": "confirm"}, nil)
		v = version(b)
	}
	lv := int(data(b)["ledger"].(map[string]any)["version"].(float64))
	res, b = owner.do("POST", "/api/v2/ledger/approval", map[string]any{"expectedVersion": v, "ledgerVersion": lv}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("approve: %d %v", res.StatusCode, b)
	}
	v = version(b)
	share := map[string]any{"expectedVersion": v, "ledgerVersion": lv, "professionalId": "pro-rani", "slotId": "slot-rani-1", "fields": []string{"items", "quotes"},
		"durationHours": 24, "shareConsent": true, "sharingPolicy": cat.Policies["sharing"].Version}
	res, b = owner.do("POST", "/api/v2/handoffs/preview", map[string]any{"professionalId": "pro-rani", "slotId": "slot-rani-1", "fields": []string{"items", "quotes"}}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("preview: %d %v", res.StatusCode, b)
	}
	preview := data(b)
	res, b = owner.do("POST", "/api/v2/handoffs", share, nil)
	if res.StatusCode != 400 || errCode(b) != "idempotency_key_required" {
		t.Fatalf("missing key: %d %v", res.StatusCode, b)
	}
	key := map[string]string{"Idempotency-Key": "http-test-key-000001"}
	res, b = owner.do("POST", "/api/v2/handoffs", share, key)
	if res.StatusCode != 201 {
		t.Fatalf("handoff: %d %v", res.StatusCode, b)
	}
	created := data(b)
	token, code := created["token"].(string), created["code"].(string)
	proj, _ := json.Marshal(created["handoff"].(map[string]any)["projection"])
	prev, _ := json.Marshal(preview)
	if string(proj) != string(prev) {
		t.Fatalf("preview differs from stored projection:\n%s\n%s", prev, proj)
	}
	res, b = owner.do("POST", "/api/v2/handoffs", share, key)
	if res.StatusCode != 200 || data(b)["token"] != token || data(b)["replay"] != true {
		t.Fatalf("replay: %d %v", res.StatusCode, b)
	}

	recipient := e.client()
	res, b = recipient.do("GET", "/api/v2/recipient/handoff", nil, nil)
	if res.StatusCode != 403 || errCode(b) != "access_ended" {
		t.Fatalf("recipient without principal: %d %v", res.StatusCode, b)
	}
	res, b = recipient.do("POST", "/api/v2/recipient/sessions", map[string]any{"token": token, "code": "WRONG-CODE"}, nil)
	if res.StatusCode != 403 {
		t.Fatalf("wrong code: %d %v", res.StatusCode, b)
	}
	res, b = recipient.do("POST", "/api/v2/recipient/sessions", map[string]any{"token": token, "code": code}, nil)
	if res.StatusCode != 201 {
		t.Fatalf("recipient sign-in: %d %v", res.StatusCode, b)
	}
	if !strings.Contains(res.Header.Get("Set-Cookie"), "Path=/api/v2/recipient") {
		t.Errorf("recipient cookie path: %s", res.Header.Get("Set-Cookie"))
	}
	rh := data(b)
	if rh["synthetic"] != true {
		t.Fatal("recipient view not marked synthetic")
	}
	// The recipient cookie must not unlock the owner's session API.
	res, _ = recipient.do("GET", "/api/v2/session", nil, nil)
	if res.StatusCode != 401 {
		t.Fatalf("recipient reached owner API: %d", res.StatusCode)
	}
	rv := int(rh["version"].(float64))
	res, b = recipient.do("POST", "/api/v2/recipient/handoff/actions", map[string]any{"expectedVersion": rv, "action": "acknowledge"}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("ack: %d %v", res.StatusCode, b)
	}
	rv = int(data(b)["version"].(float64))
	res, b = recipient.do("POST", "/api/v2/recipient/handoff/actions", map[string]any{"expectedVersion": rv, "action": "complete", "planId": "plan-psychologist-followup"}, nil)
	if res.StatusCode != 200 || data(b)["status"] != "completed" {
		t.Fatalf("complete: %d %v", res.StatusCode, b)
	}

	_, b = owner.do("GET", "/api/v2/session", nil, nil)
	h := data(b)["handoffs"].([]any)[0].(map[string]any)
	if h["status"] != "completed" || h["careReceived"] != false || h["opened"] != true {
		t.Fatalf("owner receipt: %v", h)
	}
	v = version(b)
	res, b = owner.do("DELETE", "/api/v2/handoffs/"+h["id"].(string)+"/share", map[string]any{"expectedVersion": v}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("revoke: %d %v", res.StatusCode, b)
	}
	res, b = recipient.do("GET", "/api/v2/recipient/handoff", nil, nil)
	if res.StatusCode != 403 || errCode(b) != "access_ended" {
		t.Fatalf("after revoke: %d %v", res.StatusCode, b)
	}

	res, _ = owner.do("DELETE", "/api/v2/session", nil, nil)
	if res.StatusCode != 200 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	res, _ = owner.do("GET", "/api/v2/session", nil, nil)
	if res.StatusCode != 401 {
		t.Fatalf("after delete: %d", res.StatusCode)
	}

	e.svc.Wait()
	logs := e.logs.String()
	for _, secret := range []string{"CANARY-7f3a9", token, code, "merasa lelah"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("operational logs contain sensitive value %q", secret)
		}
	}
	if !strings.Contains(logs, `"route":"POST /api/v2/handoffs"`) {
		t.Fatalf("expected route-template logging, got: %.400s", logs)
	}
}

func TestStaticServingAndSPAFallback(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Sela</title>"), 0o644)
	os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644)
	e := newEnv(t, dir)
	for path, want := range map[string]int{"/": 200, "/ledger": 200, "/r": 200, "/assets/app.js": 200, "/assets/missing.js": 404, "/../../etc/passwd": 200} {
		res, err := http.Get(e.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
		if strings.Contains(string(body), "root:") {
			t.Errorf("%s: path traversal", path)
		}
	}
	res, _ := http.Get(e.srv.URL + "/assets/app.js")
	if !strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("asset caching: %s", res.Header.Get("Cache-Control"))
	}
	empty := newEnv(t, t.TempDir())
	res, _ = http.Get(empty.srv.URL + "/")
	if res.StatusCode != 503 {
		t.Errorf("frontend missing: %d", res.StatusCode)
	}
}

func TestResearchUnavailableWithoutIndexAndPublicEndpoints(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	res, b := c.do("GET", "/api/v2/research/search?q=chatbot", nil, nil)
	if res.StatusCode != 503 || errCode(b) != "research_unavailable" {
		t.Errorf("research: %d %v", res.StatusCode, b)
	}
	for _, p := range []string{"/api/v2/health", "/api/v2/ready", "/api/v2/catalog", "/api/v2/ai/status"} {
		res, b := c.do("GET", p, nil, nil)
		if res.StatusCode != 200 {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
		raw, _ := json.Marshal(b)
		for _, forbidden := range []string{"apiKey", "ModalSecret", "modalSecret", "Bearer"} {
			if strings.Contains(string(raw), forbidden) {
				t.Errorf("%s leaks %s", p, forbidden)
			}
		}
	}
}

func TestRateLimitOnSessionCreation(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	limited := false
	for i := 0; i < 40; i++ {
		res, b := c.do("POST", "/api/v2/sessions", map[string]any{"language": "id"}, nil)
		if res.StatusCode == 429 {
			limited = errCode(b) == "rate_limited" && res.Header.Get("Retry-After") != ""
			break
		}
	}
	if !limited {
		t.Fatal("session creation was never rate limited")
	}
}

// raw sends a request with an exact body and header lines.
func (c *client) raw(method, path, body string, hdr http.Header) (*http.Response, map[string]any, string) {
	c.e.t.Helper()
	req, _ := http.NewRequest(method, c.e.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sela-Request", "1")
	for k, vs := range hdr {
		req.Header.Del(k)
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	res, err := c.hc.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return res, out, string(b)
}

func sessionBody(extra string) string {
	cat := fixtures.MustLoad()
	return `{"language":"id","adultAttested":true,"privacyPolicy":"` + cat.Policies["privacy"].Version + `"` + extra + `}`
}

// Review finding: X-Forwarded-For was read with Header.Get, i.e. the first
// line only, so a client could prepend its own line and pick its key.
func TestTrustedProxyUsesLastHopAcrossHeaderLines(t *testing.T) {
	e := newEnvWith(t, httpapi.Options{TrustProxy: true}, scripted{})
	c := e.client()
	limited := 0
	for i := 0; i < 12; i++ {
		hdr := http.Header{"X-Forwarded-For": {fmt.Sprintf("10.9.0.%d", i), "203.0.113.7"}}
		if res, _, _ := c.raw("POST", "/api/v2/sessions", sessionBody(""), hdr); res.StatusCode == 429 {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("a client-supplied first X-Forwarded-For line chose the rate-limit key")
	}
	// Distinct last hops are distinct clients.
	for i := 0; i < 12; i++ {
		hdr := http.Header{"X-Forwarded-For": {"203.0.113.7", fmt.Sprintf("198.51.100.%d", i)}}
		if res, b, _ := c.raw("POST", "/api/v2/sessions", sessionBody(""), hdr); res.StatusCode != 201 {
			t.Fatalf("last hop %d: %d %v", i, res.StatusCode, b)
		}
	}
}

// Review finding: /metrics was public and showed safety_crisis deltas live.
func TestMetricsNeedOperatorToken(t *testing.T) {
	// Without a token the route must not exist for any method: a 405 with
	// Allow would announce it (verifier finding).
	off := newEnv(t, "").client()
	for _, m := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"} {
		res, b := off.do(m, "/api/v2/metrics", nil, nil)
		if res.StatusCode != 404 || res.Header.Get("Allow") != "" || (m != "HEAD" && errCode(b) != "not_found") {
			t.Errorf("%s metrics without a configured token: %d Allow=%q %v", m, res.StatusCode, res.Header.Get("Allow"), b)
		}
	}
	const token = "metrics-token-0123456789abcdef"
	e := newEnvWith(t, httpapi.Options{MetricsToken: token}, scripted{})
	c := e.client()
	res, b := c.do("POST", "/api/v2/metrics", nil, nil)
	if res.StatusCode != 405 || res.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("POST metrics with a configured token: %d Allow=%q %v", res.StatusCode, res.Header.Get("Allow"), b)
	}
	for name, auth := range map[string]string{
		"missing": "", "wrong": "Bearer metrics-token-0123456789abcdeX", "prefix": "Bearer " + token[:10],
		"longer": "Bearer " + token + "x", "scheme": "Basic " + token, "bare": token,
	} {
		res, b := c.do("GET", "/api/v2/metrics", nil, map[string]string{"Authorization": auth})
		if res.StatusCode != 401 || errCode(b) != "forbidden" || res.Header.Get("WWW-Authenticate") == "" {
			t.Errorf("%s token: %d %v", name, res.StatusCode, b)
		}
		if strings.Contains(fmt.Sprint(b), "counters") {
			t.Errorf("%s token: counters exposed", name)
		}
	}
	res, b = c.do("GET", "/api/v2/metrics", nil, map[string]string{"Authorization": "Bearer " + token})
	if res.StatusCode != 200 || data(b)["counters"] == nil {
		t.Fatalf("operator token: %d %v", res.StatusCode, b)
	}
	if strings.Contains(e.logs.String(), token) {
		t.Fatal("metrics token logged")
	}
}

// Review finding: a body whose JSON fits but whose total size does not (here
// trailing whitespace) was reported as 400 instead of 413.
func TestOversizedTrailingDataIsTooLarge(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	res, b, _ := c.raw("POST", "/api/v2/sessions", sessionBody("")+strings.Repeat(" ", 20_000), nil)
	if res.StatusCode != 413 || errCode(b) != "too_large" {
		t.Fatalf("trailing oversize: %d %v", res.StatusCode, b)
	}
	res, b, _ = c.raw("POST", "/api/v2/sessions", sessionBody("")+"   \n", nil)
	if res.StatusCode != 201 {
		t.Fatalf("small trailing whitespace: %d %v", res.StatusCode, b)
	}
}

// Review finding: case-variant and duplicate keys were silently accepted.
func TestStrictDecodingRejectsCaseVariantAndDuplicateKeys(t *testing.T) {
	e := newEnv(t, "")
	c := e.client()
	pol := fixtures.MustLoad().Policies["privacy"].Version
	for name, body := range map[string]string{
		"case-variant keys": `{"LANGUAGE":"id","AdultAttested":true,"PRIVACYPOLICY":"` + pol + `"}`,
		"one case variant":  `{"language":"id","adultAttested":true,"privacypolicy":"` + pol + `"}`,
		"duplicate key":     `{"language":"id","adultAttested":false,"adultAttested":true,"privacyPolicy":"` + pol + `"}`,
		"nested variant":    sessionBody(`,"identity":{"MODE":"anonymous"}`),
		"nested duplicate":  sessionBody(`,"identity":{"mode":"anonymous","mode":"anonymous"}`),
		"trailing value":    sessionBody("") + `{}`,
	} {
		res, b, _ := c.raw("POST", "/api/v2/sessions", body, nil)
		if res.StatusCode != 400 || errCode(b) != "invalid_input" {
			t.Errorf("%s: %d %v", name, res.StatusCode, b)
		}
	}
	res, b, _ := c.raw("POST", "/api/v2/sessions", sessionBody(`,"identity":{"mode":"anonymous"}`), nil)
	if res.StatusCode != 201 {
		t.Fatalf("exact keys: %d %v", res.StatusCode, b)
	}
}

// Review finding: wrong methods on real routes returned 404, and case-variant
// or encoded API paths were served the SPA.
func TestWrongMethodIs405AndAPILookalikesNeverGetTheSPA(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Sela</title>"), 0o644)
	e := newEnv(t, dir)
	c := e.client()
	for _, tc := range []struct{ method, path, allow string }{
		{"POST", "/api/v2/session", "GET, HEAD, PATCH, DELETE"},
		{"PUT", "/api/v2/session", "GET, HEAD, PATCH, DELETE"},
		{"POST", "/api/v2/health", "GET, HEAD"},
		{"DELETE", "/api/v2/recipient/handoff", "GET, HEAD"},
		{"GET", "/api/v2/handoffs/h1/share", "DELETE"},
		{"GET", "/api/v2/sessions", "POST"},
	} {
		res, b := c.do(tc.method, tc.path, nil, map[string]string{"X-Sela-Language": "en"})
		if res.StatusCode != 405 || errCode(b) != "method_not_allowed" || res.Header.Get("Allow") != tc.allow {
			t.Errorf("%s %s: %d Allow=%q %v", tc.method, tc.path, res.StatusCode, res.Header.Get("Allow"), b)
		}
		if msg := fmt.Sprint(b); !strings.Contains(msg, "not supported") {
			t.Errorf("%s %s: message not localized: %v", tc.method, tc.path, msg)
		}
	}
	res, b := c.do("POST", "/api/v2/health", nil, nil)
	if !strings.Contains(fmt.Sprint(b), "tidak didukung") {
		t.Errorf("id message: %d %v", res.StatusCode, b)
	}
	for _, p := range []string{"/api/v2/nope", "/API/v2/session", "/Api/v2/health", "/api%2fv2%2fsession", "/api", "/apix"} {
		res, err := http.Get(e.srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 404 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") || strings.Contains(string(body), "<title>") {
			t.Errorf("%s: %d %s %.60s", p, res.StatusCode, res.Header.Get("Content-Type"), body)
		}
	}
	res, _ = c.do("POST", "/ledger", nil, nil)
	if res.StatusCode != 405 || res.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("POST to the app: %d Allow=%q", res.StatusCode, res.Header.Get("Allow"))
	}
	if res, _ := c.do("GET", "/ledger", nil, nil); res.StatusCode != 200 {
		t.Errorf("SPA route: %d", res.StatusCode)
	}
}

// Contract J3 (handler side): the revoke body is optional, including an
// empty chunked body, and {} is accepted.
func TestRevokeBodyIsOptional(t *testing.T) {
	e := newEnv(t, "")
	owner := e.client()
	hid := shareOnce(t, owner)
	_, b := owner.do("GET", "/api/v2/session", nil, nil)
	res, b := owner.do("DELETE", "/api/v2/handoffs/"+hid+"/share", map[string]any{"expectedVersion": version(b)}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("revoke: %d %v", res.StatusCode, b)
	}
	// Repeat revokes of the ended share: no body, {}, and an empty chunked body.
	if res, b := owner.do("DELETE", "/api/v2/handoffs/"+hid+"/share", nil, nil); res.StatusCode != 200 {
		t.Errorf("no body: %d %v", res.StatusCode, b)
	}
	if res, b, _ := owner.raw("DELETE", "/api/v2/handoffs/"+hid+"/share", "{}", nil); res.StatusCode != 200 {
		t.Errorf("{}: %d %v", res.StatusCode, b)
	}
	u, _ := url.Parse(e.srv.URL + "/api/v2/")
	var cookie string
	for _, ck := range owner.hc.Jar.Cookies(u) {
		cookie += ck.Name + "=" + ck.Value
	}
	conn, err := net.Dial("tcp", e.srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "DELETE /api/v2/handoffs/%s/share HTTP/1.1\r\nHost: %s\r\nX-Sela-Request: 1\r\nCookie: %s\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n0\r\n\r\n", hid, e.srv.Listener.Addr(), cookie)
	cr, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(cr.Body)
	if cr.StatusCode != 200 {
		t.Errorf("empty chunked body: %d %s", cr.StatusCode, raw)
	}
	// Malformed bodies are still rejected.
	if res, b, _ := owner.raw("DELETE", "/api/v2/handoffs/"+hid+"/share", `{"expectedVersion":1,"expectedVersion":2}`, nil); res.StatusCode != 400 {
		t.Errorf("duplicate key: %d %v", res.StatusCode, b)
	}
}

// shareOnce drives a session to an active share and returns the handoff id.
func shareOnce(t *testing.T, owner *client) string {
	t.Helper()
	cat := fixtures.MustLoad()
	b := owner.start("id")
	res, b := owner.do("PUT", "/api/v2/story", map[string]any{"expectedVersion": version(b), "mode": "practice", "text": "Tokoh fiktif merasa lelah setiap hari karena pekerjaan baru."}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("story: %d %v", res.StatusCode, b)
	}
	seg := data(b)["session"].(map[string]any)["story"].(map[string]any)["segments"].([]any)[0].(map[string]any)["id"]
	res, b = owner.do("POST", "/api/v2/ledger/items", map[string]any{"expectedVersion": version(b), "segmentId": seg, "kind": "experience"}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("manual item: %d %v", res.StatusCode, b)
	}
	sess := data(b)
	led := sess["ledger"].(map[string]any)
	v := int(sess["version"].(float64))
	for _, it := range led["items"].([]any) {
		id := it.(map[string]any)["id"].(string)
		_, b = owner.do("PATCH", "/api/v2/ledger/items/"+id, map[string]any{"expectedVersion": v, "action": "confirm"}, nil)
		v = version(b)
	}
	lv := int(data(b)["ledger"].(map[string]any)["version"].(float64))
	res, b = owner.do("POST", "/api/v2/ledger/approval", map[string]any{"expectedVersion": v, "ledgerVersion": lv}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("approve: %d %v", res.StatusCode, b)
	}
	share := map[string]any{"expectedVersion": version(b), "ledgerVersion": lv, "professionalId": "pro-rani", "slotId": "slot-rani-1", "fields": []string{"items"},
		"durationHours": 24, "shareConsent": true, "sharingPolicy": cat.Policies["sharing"].Version}
	res, b = owner.do("POST", "/api/v2/handoffs", share, map[string]string{"Idempotency-Key": "http-test-key-revoke-01"})
	if res.StatusCode != 201 {
		t.Fatalf("handoff: %d %v", res.StatusCode, b)
	}
	return data(b)["handoff"].(map[string]any)["id"].(string)
}

// Review finding: research q was capped at 200 bytes, not 200 characters.
func TestResearchQueryLimitCountsCharacters(t *testing.T) {
	idx, err := research.Load(filepath.Join("..", "..", "data", "research", "corpus", "knowledge_base.jsonl"))
	if err != nil {
		t.Skipf("research corpus not available: %v", err)
	}
	e := newEnvWith(t, httpapi.Options{Research: idx}, scripted{})
	c := e.client()
	for q, want := range map[string]int{strings.Repeat("é", 150): 200, strings.Repeat("語", 200): 200, strings.Repeat("a", 201): 400, strings.Repeat("é", 201): 400} {
		res, b := c.do("GET", "/api/v2/research/search?q="+url.QueryEscape(q), nil, nil)
		if res.StatusCode != want {
			t.Errorf("q of %d runes / %d bytes: %d %v", len([]rune(q)), len(q), res.StatusCode, errCode(b))
		}
	}
}

// blocking is a TEST-ONLY provider whose completion never finishes on its own.
type blocking struct{ scripted }

func (blocking) Complete(ctx context.Context, _ llm.Request, _ func(string) error) (llm.Result, error) {
	<-ctx.Done()
	return llm.Result{}, ctx.Err()
}

// Review finding: an open stream held graceful shutdown for its whole
// timeout and was then cut without a terminal event.
func TestCloseStreamsEndsOpenStreamsWithAFinalEvent(t *testing.T) {
	e := newEnvWith(t, httpapi.Options{}, blocking{})
	owner := e.client()
	cat := fixtures.MustLoad()
	b := owner.start("id")
	_, b = owner.do("PUT", "/api/v2/consents/ai_processing", map[string]any{"expectedVersion": version(b), "granted": true, "policyVersion": cat.Policies["ai_processing"].Version}, nil)
	canary := "Tokoh fiktif CANARY-SHUTDOWN-41 merasa lelah setiap hari karena pekerjaan barunya."
	_, b = owner.do("PUT", "/api/v2/story", map[string]any{"expectedVersion": version(b), "mode": "practice", "text": canary}, nil)
	res, b := owner.do("POST", "/api/v2/ledger/draft", map[string]any{"expectedVersion": version(b)}, nil)
	if res.StatusCode != 202 {
		t.Fatalf("draft: %d %v", res.StatusCode, b)
	}
	op := data(b)["operation"].(map[string]any)["id"].(string)
	defer owner.do("DELETE", "/api/v2/operations/"+op, nil, nil)

	stream := func() (chan string, func()) {
		req, _ := http.NewRequest("GET", e.srv.URL+"/api/v2/operations/"+op+"/events", nil)
		sse, err := owner.hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		lines := make(chan string, 64)
		go func() {
			defer close(lines)
			sc := bufio.NewScanner(sse.Body)
			for sc.Scan() {
				lines <- sc.Text()
			}
		}()
		return lines, func() { sse.Body.Close() }
	}
	lines, closeBody := stream()
	defer closeBody()
	// Wait for the stream to be live.
	for l := range lines {
		if strings.HasPrefix(l, "event: status") {
			break
		}
	}
	t0 := time.Now()
	e.h.CloseStreams()
	var tail []string
	timeout := time.After(3 * time.Second)
collect:
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				break collect
			}
			tail = append(tail, l)
		case <-timeout:
			t.Fatalf("stream still open 3s after CloseStreams: %v", tail)
		}
	}
	joined := strings.Join(tail, "\n")
	if !strings.Contains(joined, "event: error\ndata: {\"code\":\"ai_unavailable\",\"reason\":\"server_stopping\"}") {
		t.Fatalf("no final event after %s: %q", time.Since(t0), joined)
	}
	if strings.Contains(joined, "CANARY") {
		t.Fatal("final event carries story text")
	}
	// A stream opened after CloseStreams ends at once with the same event.
	lines, closeBody2 := stream()
	defer closeBody2()
	var again []string
	timeout = time.After(3 * time.Second)
	for done := false; !done; {
		select {
		case l, ok := <-lines:
			if !ok {
				done = true
			}
			again = append(again, l)
		case <-timeout:
			t.Fatalf("late stream still open: %v", again)
		}
	}
	if !strings.Contains(strings.Join(again, "\n"), "server_stopping") {
		t.Fatalf("late stream: %v", again)
	}
	e.h.CloseStreams() // idempotent
}

package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/llm"
)

func TestModalRequestShapeAndAuthHeaders(t *testing.T) {
	var (
		mu      sync.Mutex
		header  http.Header
		path    string
		method  string
		rawBody []byte
	)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		header, path, method, rawBody = r.Header.Clone(), r.URL.Path, r.Method, b
		mu.Unlock()
		writeStream(w, "sela-glimmer-cec94fc", "Halo", ", dunia ", "ü")
	}))
	defer srv.Close()
	p, err := llm.New(llm.Config{
		Mode: llm.ModeModal, BaseURL: srv.URL + "/", APIKey: "api-key-1", ModalKey: "wk-modal", ModalSecret: "ws-secret",
		ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fcfd6512f2cfe708138c32533b346ea4f69",
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	seed := 7
	req := testRequest()
	req.Seed = &seed
	req.ReasoningStrength = llm.ReasoningLow
	res, err := p.Complete(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Text != "Halo, dunia ü" || res.Model != "sela-glimmer-cec94fc" || res.FinishReason != "stop" ||
		res.PromptTokens != 11 || res.CompletionTokens != 7 || res.Attempts != 1 {
		t.Fatalf("result = %+v", res)
	}

	mu.Lock()
	defer mu.Unlock()
	if method != http.MethodPost || path != "/v1/chat/completions" {
		t.Fatalf("%s %s", method, path)
	}
	for k, want := range map[string]string{
		"Authorization": "Bearer api-key-1", "Modal-Key": "wk-modal", "Modal-Secret": "ws-secret",
		"Content-Type": "application/json", "Accept": "text/event-stream",
	} {
		if got := header.Get(k); got != want {
			t.Fatalf("header %s = %q, want %q", k, got, want)
		}
	}
	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "sela-glimmer-cec94fc" || body["stream"] != true || body["max_tokens"] != float64(256) ||
		body["seed"] != float64(7) {
		t.Fatalf("body = %v", body)
	}
	if so, _ := body["stream_options"].(map[string]any); so["include_usage"] != true {
		t.Fatalf("stream_options = %v", body["stream_options"])
	}
	// No sampling override: the server's generation config (never greedy)
	// applies; reasoning strength travels as a chat-template kwarg.
	for _, k := range []string{"temperature", "top_p", "top_k", "reasoning_effort"} {
		if _, ok := body[k]; ok {
			t.Fatalf("body carries %s: %v", k, body)
		}
	}
	if kw, _ := body["chat_template_kwargs"].(map[string]any); kw["reasoning_strength"] != "low" {
		t.Fatalf("chat_template_kwargs = %v", body["chat_template_kwargs"])
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" || msgs[1].(map[string]any)["content"] != "PROMPT-CANARY user text" {
		t.Fatalf("messages = %v", msgs)
	}
	rf, _ := body["response_format"].(map[string]any)
	js, _ := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["name"] != "sela_test" || js["strict"] != true {
		t.Fatalf("response_format = %v", rf)
	}
	var wantSchema any
	_ = json.Unmarshal([]byte(req.JSONSchema), &wantSchema)
	if !reflect.DeepEqual(js["schema"], wantSchema) {
		t.Fatalf("schema = %v, want the raw schema object", js["schema"])
	}
	if p.Info().Mode != llm.ModeModal {
		t.Fatal("info mode")
	}
}

func TestRequestDefaults(t *testing.T) {
	var mu sync.Mutex
	var body map[string]any
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		body = m
		mu.Unlock()
		if r.Header.Get("Authorization") != "" || r.Header.Get("Modal-Key") != "" || r.Header.Get("Modal-Secret") != "" {
			t.Error("credential headers sent although none are configured")
		}
		writeStream(w, servedModel, "x")
	})
	p := newTestDouble(t, srv.URL, nil)
	req := testRequest()
	req.SchemaName, req.MaxTokens = "", 0
	if _, err := p.Complete(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	rf := body["response_format"].(map[string]any)
	if rf["json_schema"].(map[string]any)["name"] != "sela" || body["max_tokens"] != float64(llm.DefaultMaxTokens) {
		t.Fatalf("defaults not applied: %v", body)
	}
	if _, ok := body["seed"]; ok {
		t.Fatal("seed sent although unset")
	}
	mu.Unlock()

	req.JSONSchema = ""
	if _, err := p.Complete(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := body["response_format"]; ok {
		t.Fatal("response_format sent without a schema")
	}
}

func TestStreamingAssembly(t *testing.T) {
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.WriteHeader(200)
		s := func(v string) *string { return &v }
		// A keep-alive comment, an event field, CRLF line endings, a
		// reasoning delta (ignored) and a second choice (ignored).
		fmt.Fprint(w, ": keep-alive\n\nevent: message\r\n")
		fmt.Fprint(w, sseChunk(servedModel, s(""), "", false))
		fmt.Fprint(w, `data: {"model":"`+servedModel+`","choices":[{"index":0,"delta":{"reasoning":"thinking..."}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"model":"`+servedModel+`","choices":[{"index":1,"delta":{"content":"OTHER"}}]}`+"\n\n")
		for _, piece := range []string{`{"lines":`, `["Saya `, `ingin`, ` bercerita."]}`} {
			fmt.Fprint(w, strings.ReplaceAll(sseChunk(servedModel, s(piece), "", false), "\n", "\r\n"))
			w.(http.Flusher).Flush()
			time.Sleep(2 * time.Millisecond)
		}
		fmt.Fprint(w, sseChunk(servedModel, nil, "stop", false))
		fmt.Fprint(w, sseChunk(servedModel, nil, "", true))
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	var deltas []string
	p := newTestDouble(t, srv.URL, nil)
	res, err := p.Complete(context.Background(), testRequest(), func(d string) error {
		deltas = append(deltas, d)
		return nil
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Text != `{"lines":["Saya ingin bercerita."]}` || strings.Join(deltas, "") != res.Text || len(deltas) != 4 {
		t.Fatalf("text %q deltas %q", res.Text, deltas)
	}
	if res.FirstToken <= 0 || res.FirstToken > res.Latency {
		t.Fatalf("first token %v, latency %v", res.FirstToken, res.Latency)
	}
}

func TestMultiLineDataEvent(t *testing.T) {
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// One event whose JSON is split over two data lines.
		fmt.Fprint(w, "data: {\"model\":\""+servedModel+"\",\ndata: \"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]") // no trailing blank line
	})
	res, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
	if err != nil || res.Text != "ok" || res.FinishReason != "stop" {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestRetryOn503ThenSuccess(t *testing.T) {
	var calls counter
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.inc() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeStream(w, servedModel, "ok")
	})
	var ev events
	p := newTestDouble(t, srv.URL, func(c *llm.Config) { c.Observer = ev.observe })
	res, err := p.Complete(context.Background(), testRequest(), nil)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Attempts != 2 || calls.get() != 2 || res.Text != "ok" {
		t.Fatalf("attempts %d calls %d text %q", res.Attempts, calls.get(), res.Text)
	}
	if got := ev.list(); len(got) != 1 || got[0].Attempts != 2 || got[0].Outcome != "ok" || got[0].Status != 200 {
		t.Fatalf("events %+v", got)
	}
}

func TestRetryPolicyByStatus(t *testing.T) {
	cases := []struct {
		status     int
		maxRetries int
		kind       llm.Kind
		attempts   int
	}{
		{400, 0, llm.KindRejected, 1},
		{401, 0, llm.KindAuth, 1},
		{403, 0, llm.KindAuth, 1},
		{404, 0, llm.KindRejected, 1},
		{422, 0, llm.KindRejected, 1},
		{413, 0, llm.KindRejected, 1},
		{500, 0, llm.KindUnavailable, 2}, // retried once only
		{501, 0, llm.KindUnavailable, 1},
		{502, 1, llm.KindUnavailable, 2},
		{503, 0, llm.KindUnavailable, 3}, // default MaxRetries is 2
		{504, 1, llm.KindUnavailable, 2},
		{429, 1, llm.KindRateLimited, 2},
		{503, -1, llm.KindUnavailable, 1}, // retries disabled
		{302, 0, llm.KindProtocol, 1},
		{204, 0, llm.KindProtocol, 1},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d_retries%d", tc.status, tc.maxRetries), func(t *testing.T) {
			t.Parallel()
			var calls counter
			srv := server(t, func(w http.ResponseWriter, r *http.Request) {
				calls.inc()
				if tc.status == 302 {
					w.Header().Set("Location", "/elsewhere")
				}
				w.WriteHeader(tc.status)
			})
			p := newTestDouble(t, srv.URL, func(c *llm.Config) { c.MaxRetries = tc.maxRetries })
			res, err := p.Complete(context.Background(), testRequest(), nil)
			var e *llm.Error
			if !errors.As(err, &e) || e.Kind != tc.kind || e.Status != tc.status {
				t.Fatalf("err = %v, want kind %s status %d", err, tc.kind, tc.status)
			}
			if res.Attempts != tc.attempts || calls.get() != tc.attempts {
				t.Fatalf("attempts %d calls %d, want %d", res.Attempts, calls.get(), tc.attempts)
			}
		})
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var hits counter
	target := server(t, func(w http.ResponseWriter, r *http.Request) {
		hits.inc()
		writeStream(w, servedModel, "x")
	})
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/v1/chat/completions", http.StatusTemporaryRedirect)
	})
	_, err := newTestDouble(t, srv.URL, func(c *llm.Config) { c.APIKey = "k" }).Complete(context.Background(), testRequest(), nil)
	if llm.KindOf(err) != llm.KindProtocol || hits.get() != 0 {
		t.Fatalf("err %v, redirect target hit %d times", err, hits.get())
	}
}

func TestNoRetryAfterFirstDelta(t *testing.T) {
	var calls counter
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls.inc()
		w.Header().Set("Content-Type", "text/event-stream")
		s := "first"
		fmt.Fprint(w, sseChunk(servedModel, &s, "", false))
		w.(http.Flusher).Flush()
		// Break the connection mid-stream.
		conn, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			conn.Close()
		}
	})
	var deltas []string
	res, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), func(d string) error {
		deltas = append(deltas, d)
		return nil
	})
	if llm.KindOf(err) != llm.KindUnavailable {
		t.Fatalf("err = %v", err)
	}
	if res.Attempts != 1 || calls.get() != 1 || len(deltas) != 1 || res.Text != "" {
		t.Fatalf("attempts %d calls %d deltas %q text %q", res.Attempts, calls.get(), deltas, res.Text)
	}
}

func TestRetryWhenStreamBreaksBeforeContent(t *testing.T) {
	var calls counter
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.inc() == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			empty := ""
			fmt.Fprint(w, sseChunk(servedModel, &empty, "", false)) // role chunk only
			w.(http.Flusher).Flush()
			if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
				conn.Close()
			}
			return
		}
		writeStream(w, servedModel, "ok")
	})
	res, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
	if err != nil || res.Attempts != 2 || res.Text != "ok" {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestRetryAfterHonouredWithinDeadline(t *testing.T) {
	newServer := func(retryAfter string) (*httptest.Server, *counter) {
		var calls counter
		return server(t, func(w http.ResponseWriter, r *http.Request) {
			if calls.inc() == 1 {
				w.Header().Set("Retry-After", retryAfter)
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			writeStream(w, servedModel, "ok")
		}), &calls
	}

	t.Run("honoured", func(t *testing.T) {
		srv, calls := newServer("1")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		start := time.Now()
		res, err := newTestDouble(t, srv.URL, nil).Complete(ctx, testRequest(), nil)
		if err != nil || res.Attempts != 2 || calls.get() != 2 {
			t.Fatalf("res %+v err %v", res, err)
		}
		if el := time.Since(start); el < 950*time.Millisecond {
			t.Fatalf("retried after %v, want >= Retry-After 1s", el)
		}
	})
	t.Run("not past the deadline", func(t *testing.T) {
		srv, calls := newServer("1")
		ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
		defer cancel()
		start := time.Now()
		res, err := newTestDouble(t, srv.URL, nil).Complete(ctx, testRequest(), nil)
		if llm.KindOf(err) != llm.KindRateLimited || res.Attempts != 1 || calls.get() != 1 {
			t.Fatalf("res %+v err %v", res, err)
		}
		if el := time.Since(start); el > 500*time.Millisecond {
			t.Fatalf("gave up after %v; it should not wait for a retry it cannot make", el)
		}
	})
	t.Run("too long", func(t *testing.T) {
		srv, calls := newServer("30")
		start := time.Now()
		res, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
		if llm.KindOf(err) != llm.KindRateLimited || res.Attempts != 1 || calls.get() != 1 || time.Since(start) > 2*time.Second {
			t.Fatalf("res %+v err %v after %v", res, err, time.Since(start))
		}
	})
	t.Run("http date", func(t *testing.T) {
		srv, calls := newServer(time.Now().Add(-time.Second).UTC().Format(http.TimeFormat))
		res, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
		if err != nil || res.Attempts != 2 || calls.get() != 2 {
			t.Fatalf("res %+v err %v", res, err)
		}
	})
}

func TestCallerTimeout(t *testing.T) {
	srv := server(t, hang)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	res, err := newTestDouble(t, srv.URL, nil).Complete(ctx, testRequest(), nil)
	if llm.KindOf(err) != llm.KindTimeout || !errors.Is(err, context.DeadlineExceeded) || res.Attempts != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("returned after %v", time.Since(start))
	}
}

func TestAttemptTimeoutBeforeHeadersIsRetried(t *testing.T) {
	var calls counter
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls.inc()
		hang(w, r)
	})
	p := newTestDouble(t, srv.URL, func(c *llm.Config) { c.AttemptTimeout, c.MaxRetries = 150*time.Millisecond, 1 })
	res, err := p.Complete(context.Background(), testRequest(), nil)
	if llm.KindOf(err) != llm.KindTimeout || !errors.Is(err, context.DeadlineExceeded) || res.Attempts != 2 || calls.get() != 2 {
		t.Fatalf("res %+v calls %d err %v", res, calls.get(), err)
	}
}

// slowStream streams one chunk every 20 ms for up to 10 s and reports when
// the client went away.
func slowStream(t *testing.T) (*httptest.Server, <-chan time.Time) {
	gone := make(chan time.Time, 1)
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		defer func() { gone <- time.Now() }()
		for i := 0; i < 500; i++ {
			s := fmt.Sprintf("tok%d ", i)
			if _, err := fmt.Fprint(w, sseChunk(servedModel, &s, "", false)); err != nil {
				return
			}
			if http.NewResponseController(w).Flush() != nil {
				return
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	})
	return srv, gone
}

func TestCancelClosesBodyPromptly(t *testing.T) {
	srv, gone := slowStream(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var canceledAt time.Time
	var once sync.Once
	res, err := newTestDouble(t, srv.URL, nil).Complete(ctx, testRequest(), func(string) error {
		once.Do(func() {
			canceledAt = time.Now()
			cancel()
		})
		return nil
	})
	returned := time.Now()
	if llm.KindOf(err) != llm.KindCanceled || !errors.Is(err, context.Canceled) || res.Attempts != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if d := returned.Sub(canceledAt); d > time.Second {
		t.Fatalf("Complete returned %v after cancel", d)
	}
	select {
	case at := <-gone:
		if d := at.Sub(canceledAt); d > 2*time.Second {
			t.Fatalf("server saw the disconnect %v after cancel", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never saw the client go away: body not closed")
	}
}

func TestOnDeltaErrorAborts(t *testing.T) {
	srv, gone := slowStream(t)
	stop := errors.New("consumer stopped")
	var ev events
	n := 0
	res, err := newTestDouble(t, srv.URL, func(c *llm.Config) { c.Observer = ev.observe }).Complete(context.Background(), testRequest(), func(string) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	if err != stop || res.Attempts != 1 || n != 2 {
		t.Fatalf("err %v attempts %d deltas %d", err, res.Attempts, n)
	}
	select {
	case <-gone:
	case <-time.After(3 * time.Second):
		t.Fatal("stream not closed after callback error")
	}
	if got := ev.list(); len(got) != 1 || got[0].Outcome != string(llm.KindCanceled) {
		t.Fatalf("events %+v", got)
	}
}

func TestStreamFailures(t *testing.T) {
	s := func(v string) *string { return &v }
	big := strings.Repeat("x", 60)
	cases := []struct {
		name string
		body string
		ct   string
		max  int
		kind llm.Kind
	}{
		{"model mismatch", sseChunk("llama-impostor", s("hi"), "stop", false) + "data: [DONE]\n\n", "", 0, llm.KindModelMismatch},
		{"model mismatch on a later chunk", sseChunk(servedModel, s("hi"), "", false) + sseChunk("other", nil, "stop", false) + "data: [DONE]\n\n", "", 0, llm.KindModelMismatch},
		{"missing model", `data: {"choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":"stop"}]}` + "\n\ndata: [DONE]\n\n", "", 0, llm.KindModelMismatch},
		{"no [DONE]", sseChunk(servedModel, s("hi"), "", false) + sseChunk(servedModel, nil, "stop", false), "", 0, llm.KindProtocol},
		{"no finish reason", sseChunk(servedModel, s("hi"), "", false) + "data: [DONE]\n\n", "", 0, llm.KindProtocol},
		{"only [DONE]", "data: [DONE]\n\n", "", 0, llm.KindProtocol},
		{"empty body", "", "", 0, llm.KindProtocol},
		{"malformed chunk", "data: {\"model\":\n\n", "", 0, llm.KindProtocol},
		{"not an event stream", `{"choices":[]}`, "application/json", 0, llm.KindProtocol},
		{"output too long", sseChunk(servedModel, &big, "", false) + sseChunk(servedModel, &big, "stop", false) + "data: [DONE]\n\n", "", 100, llm.KindOutputTooLong},
		{"line too long", "data: " + strings.Repeat("a", 70<<10) + "\n\n", "", 100, llm.KindProtocol},
		{"error event", `data: {"error":{"message":"bad","code":400}}` + "\n\n", "", 0, llm.KindRejected},
		{"error object", `data: {"object":"error","message":"overloaded","code":503}` + "\n\n", "", 0, llm.KindUnavailable},
		{"error event 429", `data: {"error":{"message":"slow down","code":"429"}}` + "\n\n", "", 0, llm.KindRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls counter
			srv := server(t, func(w http.ResponseWriter, r *http.Request) {
				calls.inc()
				ct := tc.ct
				if ct == "" {
					ct = "text/event-stream"
				}
				w.Header().Set("Content-Type", ct)
				fmt.Fprint(w, tc.body)
			})
			got := 0
			res, err := newTestDouble(t, srv.URL, func(c *llm.Config) { c.MaxOutputBytes = tc.max }).Complete(context.Background(), testRequest(), func(d string) error {
				got += len(d)
				return nil
			})
			if llm.KindOf(err) != tc.kind {
				t.Fatalf("err = %v, want %s", err, tc.kind)
			}
			if res.Text != "" || res.Attempts != 1 || calls.get() != 1 {
				t.Fatalf("res %+v calls %d", res, calls.get())
			}
			if tc.max > 0 && got > tc.max {
				t.Fatalf("delivered %d bytes past the %d byte limit", got, tc.max)
			}
			if strings.Contains(err.Error(), "llama-impostor") {
				t.Fatal("error echoes the reported model name")
			}
		})
	}
}

func TestErrorsNeverContainServerBody(t *testing.T) {
	statuses := []int{400, 401, 403, 404, 422, 429, 500, 502, 503}
	for _, status := range statuses {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := server(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Debug", "sk-live-SECRET-9f8e7d")
				w.WriteHeader(status)
				fmt.Fprint(w, secretBody)
			})
			_, err := newTestDouble(t, srv.URL, func(c *llm.Config) { c.MaxRetries = -1 }).Complete(context.Background(), testRequest(), nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			msg := err.Error()
			for _, leak := range []string{"SECRET", "user said", "menyerah", "internal", "PROMPT-CANARY"} {
				if strings.Contains(msg, leak) {
					t.Fatalf("error %q leaks %q", msg, leak)
				}
			}
			if !strings.Contains(msg, fmt.Sprint(status)) {
				t.Fatalf("error %q lacks the status code", msg)
			}
		})
	}
	t.Run("stream error event", func(t *testing.T) {
		srv := server(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: "+secretBody+"\n\n")
		})
		_, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
		if err == nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "menyerah") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("finish reason without content", func(t *testing.T) {
		// The finish_reason is a server string; with no content it ends in
		// a protocol error whose message must not quote it.
		srv := server(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, sseChunk(servedModel, nil, "sk-live-SECRET-9f8e7d user said aku ingin menyerah", false))
			fmt.Fprint(w, "data: [DONE]\n\n")
		})
		_, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
		if llm.KindOf(err) != llm.KindProtocol {
			t.Fatalf("want a protocol error, got %v", err)
		}
		for _, leak := range []string{"SECRET", "user said", "menyerah"} {
			if strings.Contains(err.Error(), leak) {
				t.Fatalf("error %q leaks %q", err, leak)
			}
		}
		if !strings.Contains(err.Error(), "finish reason other") {
			t.Fatalf("error %q does not name the normalized reason", err)
		}
	})
}

func TestFinishReasonIsNormalized(t *testing.T) {
	cases := map[string]string{
		"stop":           "stop",
		"length":         "length",
		"abort":          "abort",
		"content_filter": "content_filter",
		"tool_calls":     "tool_calls",
		"STOP":           "other",
		"sk-live-SECRET-9f8e7d user said aku ingin menyerah": "other",
	}
	for sent, want := range cases {
		t.Run(want+"/"+sent[:min(len(sent), 8)], func(t *testing.T) {
			srv := server(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				ok := "ok"
				fmt.Fprint(w, sseChunk(servedModel, &ok, "", false))
				fmt.Fprint(w, sseChunk(servedModel, nil, sent, false))
				fmt.Fprint(w, "data: [DONE]\n\n")
			})
			res, err := newTestDouble(t, srv.URL, nil).Complete(context.Background(), testRequest(), nil)
			if err != nil || res.Text != "ok" || res.FinishReason != want {
				t.Fatalf("res %+v err %v, want finish reason %q", res, err, want)
			}
		})
	}
}

func TestConnectionFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	res, err := newTestDouble(t, "http://"+addr, func(c *llm.Config) { c.MaxRetries = 1 }).Complete(context.Background(), testRequest(), nil)
	var opErr *net.OpError
	if llm.KindOf(err) != llm.KindUnavailable || res.Attempts != 2 || !errors.As(err, &opErr) {
		t.Fatalf("res %+v err %v", res, err)
	}
	if strings.Contains(err.Error(), addr) || strings.Contains(err.Error(), "http://") {
		t.Fatalf("error %q exposes the endpoint", err)
	}
}

func TestCancelDuringBackoff(t *testing.T) {
	var calls counter
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls.inc()
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(100 * time.Millisecond) // the first backoff is at least 250 ms
		cancel()
	}()
	start := time.Now()
	res, err := newTestDouble(t, srv.URL, nil).Complete(ctx, testRequest(), nil)
	if llm.KindOf(err) != llm.KindCanceled || res.Attempts != 1 || calls.get() != 1 {
		t.Fatalf("res %+v calls %d err %v", res, calls.get(), err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("returned after %v", time.Since(start))
	}
}

func TestCertificateErrorIsNotRetried(t *testing.T) {
	var calls counter
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.inc() }))
	defer srv.Close()
	p, err := llm.New(llm.Config{
		Mode: llm.ModeModal, BaseURL: srv.URL, APIKey: "k", ModalKey: "mk", ModalSecret: "ms",
		ServedModel: "sela-glimmer-cec94fc", ModelID: "m/x", Revision: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Complete(context.Background(), testRequest(), nil)
	if llm.KindOf(err) != llm.KindUnavailable || res.Attempts != 1 || calls.get() != 0 {
		t.Fatalf("res %+v calls %d err %v", res, calls.get(), err)
	}
	if h := p.Health(context.Background()); h.State != "unavailable" {
		t.Fatalf("health %+v", h)
	}
}

func TestLocalValidationRejectsWithoutCalling(t *testing.T) {
	var calls counter
	srv := server(t, func(w http.ResponseWriter, r *http.Request) { calls.inc() })
	p := newTestDouble(t, srv.URL, nil)
	cases := map[string]func(*llm.Request){
		"no messages":       func(r *llm.Request) { r.Messages = nil },
		"assistant role":    func(r *llm.Request) { r.Messages = append(r.Messages, llm.Message{Role: "assistant", Content: "x"}) },
		"invalid schema":    func(r *llm.Request) { r.JSONSchema = `{"type":` },
		"bad schema name":   func(r *llm.Request) { r.SchemaName = "has space" },
		"temperature":       func(r *llm.Request) { r.Sampling = &llm.Sampling{Temperature: 2.5, TopP: 0.9} },
		"negative tokens":   func(r *llm.Request) { r.MaxTokens = -1 },
		"negative temp":     func(r *llm.Request) { r.Sampling = &llm.Sampling{Temperature: -0.1, TopP: 0.9} },
		"zero top_p":        func(r *llm.Request) { r.Sampling = &llm.Sampling{Temperature: 1, TopP: 0} },
		"negative top_k":    func(r *llm.Request) { r.Sampling = &llm.Sampling{Temperature: 1, TopP: 0.95, TopK: -1} },
		"bad reasoning":     func(r *llm.Request) { r.ReasoningStrength = "none" },
		"long schema name":  func(r *llm.Request) { r.SchemaName = strings.Repeat("a", 65) },
		"empty role":        func(r *llm.Request) { r.Messages[0].Role = "" },
		"tool role refused": func(r *llm.Request) { r.Messages[1].Role = "tool" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			req := testRequest()
			mut(&req)
			res, err := p.Complete(context.Background(), req, nil)
			if llm.KindOf(err) != llm.KindRejected || res.Attempts != 0 {
				t.Fatalf("res %+v err %v", res, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Complete(ctx, testRequest(), nil); llm.KindOf(err) != llm.KindCanceled {
		t.Fatalf("canceled context: %v", err)
	}
	if calls.get() != 0 {
		t.Fatalf("server called %d times", calls.get())
	}
}

func TestObserverReceivesMetadataOnly(t *testing.T) {
	const output = "OUTPUT-CANARY model words"
	var fail bool
	var mu sync.Mutex
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		f := fail
		mu.Unlock()
		if f {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, secretBody)
			return
		}
		writeStream(w, servedModel, output)
	})
	var ev events
	p := newTestDouble(t, srv.URL, func(c *llm.Config) { c.Observer = ev.observe; c.APIKey = "key-CANARY" })
	if _, err := p.Complete(context.Background(), testRequest(), nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	fail = true
	mu.Unlock()
	if _, err := p.Complete(context.Background(), testRequest(), nil); llm.KindOf(err) != llm.KindAuth {
		t.Fatal(err)
	}
	got := ev.list()
	if len(got) != 2 {
		t.Fatalf("got %d events, want one per Complete", len(got))
	}
	ok, bad := got[0], got[1]
	if ok.Operation != "ledger_draft" || ok.Outcome != "ok" || ok.Status != 200 || ok.Attempts != 1 ||
		ok.PromptTokens != 11 || ok.CompletionTokens != 7 || ok.LatencyMS < 0 || ok.FirstTokenMS > ok.LatencyMS {
		t.Fatalf("ok event %+v", ok)
	}
	if bad.Outcome != string(llm.KindAuth) || bad.Status != 401 || bad.Attempts != 1 {
		t.Fatalf("error event %+v", bad)
	}
	for _, e := range got {
		dump := fmt.Sprintf("%#v", e)
		for _, leak := range []string{"CANARY", "SECRET", "system prompt", "menyerah"} {
			if strings.Contains(dump, leak) {
				t.Fatalf("event %s leaks %q", dump, leak)
			}
		}
	}
	// Event has no field that could carry text beyond the two labels.
	et := reflect.TypeOf(llm.Event{})
	for i := 0; i < et.NumField(); i++ {
		if f := et.Field(i); f.Type.Kind() == reflect.String && f.Name != "Operation" && f.Name != "Outcome" {
			t.Fatalf("Event.%s is a string field", f.Name)
		}
	}
}

func TestConcurrentCompletes(t *testing.T) {
	srv := server(t, func(w http.ResponseWriter, r *http.Request) { writeStream(w, servedModel, "a", "b", "c") })
	var ev events
	p := newTestDouble(t, srv.URL, func(c *llm.Config) { c.Observer = ev.observe })
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := p.Complete(context.Background(), testRequest(), nil)
			if err == nil && res.Text != "abc" {
				err = fmt.Errorf("text %q", res.Text)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(ev.list()) != 16 {
		t.Fatalf("%d events", len(ev.list()))
	}
}

func TestInfoHasNoSecrets(t *testing.T) {
	p, err := llm.New(llm.Config{
		Mode: llm.ModeModal, BaseURL: "https://ws--sela-llm-serve.modal.run:8443/base", APIKey: "api-SECRET-1", ModalKey: "wk-SECRET-2",
		ModalSecret: "ws-SECRET-3", ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc",
	})
	if err != nil {
		t.Fatal(err)
	}
	info := p.Info()
	raw, _ := json.Marshal(info)
	dump := string(raw) + fmt.Sprintf("%#v", info)
	if strings.Contains(dump, "SECRET") || strings.Contains(dump, "https://") || strings.Contains(dump, "/base") {
		t.Fatalf("info leaks: %s", dump)
	}
	want := llm.Info{Mode: llm.ModeModal, ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc", Endpoint: "ws--sela-llm-serve.modal.run:8443"}
	if info != want {
		t.Fatalf("info = %+v, want %+v", info, want)
	}

	td := newTestDouble(t, "http://127.0.0.1:8090", nil).Info()
	if td.Mode != llm.ModeTestDouble || td.ModelID != llm.TestDoubleModelID || td.Revision != llm.TestDoubleRevision || td.Endpoint != "127.0.0.1:8090" {
		t.Fatalf("test-double info = %+v", td)
	}
}

func TestConfigValidation(t *testing.T) {
	modal := func(mut func(*llm.Config)) llm.Config {
		c := llm.Config{
			Mode: llm.ModeModal, BaseURL: "https://ws--sela-llm-serve.modal.run", APIKey: "api-SECRET", ModalKey: "wk-SECRET",
			ModalSecret: "ws-SECRET", ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc",
		}
		if mut != nil {
			mut(&c)
		}
		return c
	}
	double := func(base string, mut func(*llm.Config)) llm.Config {
		c := llm.Config{Mode: llm.ModeTestDouble, BaseURL: base, ServedModel: llm.TestDoubleServedModel}
		if mut != nil {
			mut(&c)
		}
		return c
	}
	cases := []struct {
		name    string
		cfg     llm.Config
		wantErr string // substring; "" means valid
	}{
		{"modal valid", modal(nil), ""},
		{"modal http rejected", modal(func(c *llm.Config) { c.BaseURL = "http://ws--sela-llm-serve.modal.run" }), "https"},
		{"modal http loopback rejected", modal(func(c *llm.Config) { c.BaseURL = "http://127.0.0.1:8000" }), "https"},
		{"modal missing APIKey", modal(func(c *llm.Config) { c.APIKey = "" }), "APIKey"},
		{"modal missing ModalKey", modal(func(c *llm.Config) { c.ModalKey = "" }), "ModalKey"},
		{"modal missing ModalSecret", modal(func(c *llm.Config) { c.ModalSecret = " " }), "ModalSecret"},
		{"modal missing ServedModel", modal(func(c *llm.Config) { c.ServedModel = "" }), "ServedModel"},
		{"modal missing ModelID", modal(func(c *llm.Config) { c.ModelID = "" }), "ModelID"},
		{"modal missing Revision", modal(func(c *llm.Config) { c.Revision = "" }), "Revision"},
		{"modal test-double model", modal(func(c *llm.Config) { c.ServedModel = llm.TestDoubleServedModel }), "test-double"},
		{"modal missing BaseURL", modal(func(c *llm.Config) { c.BaseURL = "" }), "BaseURL"},
		{"modal relative URL", modal(func(c *llm.Config) { c.BaseURL = "ws.modal.run" }), "BaseURL"},
		{"modal ftp URL", modal(func(c *llm.Config) { c.BaseURL = "ftp://ws.modal.run" }), "BaseURL"},
		{"credentials in URL", modal(func(c *llm.Config) { c.BaseURL = "https://user:pw@ws.modal.run" }), "credentials"},
		{"query in URL", modal(func(c *llm.Config) { c.BaseURL = "https://ws.modal.run?key=x" }), "query"},
		{"fragment in URL", modal(func(c *llm.Config) { c.BaseURL = "https://ws.modal.run#x" }), "fragment"},
		{"header injection", modal(func(c *llm.Config) { c.APIKey = "a\r\nX-Evil: 1" }), "line breaks"},
		{"negative timeout", modal(func(c *llm.Config) { c.AttemptTimeout = -time.Second }), "AttemptTimeout"},
		{"test double loopback", double("http://127.0.0.1:8090", nil), ""},
		{"test double localhost", double("http://localhost:8090", nil), ""},
		{"test double ipv6 loopback", double("http://[::1]:8090", nil), ""},
		{"test double https remote", double("https://fake.example.test", nil), ""},
		{"test double http remote", double("http://10.0.0.5:8090", nil), "loopback"},
		{"test double http hostname", double("http://fakellm.internal:8090", nil), "loopback"},
		{"test double wrong model", double("http://127.0.0.1:8090", func(c *llm.Config) { c.ServedModel = "sela-glimmer-cec94fc" }), "test-double-not-meta"},
		{"test double missing URL", double("", nil), "BaseURL"},
		{"unknown mode", llm.Config{Mode: "openai", BaseURL: "https://x"}, "mode"},
		{"empty mode", llm.Config{}, "mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := llm.New(tc.cfg)
			if tc.wantErr == "" {
				if err != nil || p == nil {
					t.Fatalf("New: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("New accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q lacks %q", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "pw@") {
				t.Fatalf("config error leaks a credential: %q", err)
			}
		})
	}
}

func TestDisabled(t *testing.T) {
	p, err := llm.New(llm.Config{Mode: llm.ModeDisabled, BaseURL: "http://ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Info().Mode != llm.ModeDisabled {
		t.Fatal("not disabled")
	}
	if _, err := p.Complete(context.Background(), testRequest(), nil); llm.KindOf(err) != llm.KindDisabled {
		t.Fatalf("err = %v", err)
	}
	if h := p.Health(context.Background()); h.State != "disabled" || h.CheckedAt.IsZero() {
		t.Fatalf("health = %+v", h)
	}
}

func TestHealth(t *testing.T) {
	const (
		served = "sela-glimmer-cec94fc"
		model  = "meta-models/Muse-Glimmer-30B"
		rev    = "cec94fcfd6512f2cfe708138c32533b346ea4f69"
	)
	identity := func(status, modelID, revision, servedModel string) string {
		raw, _ := json.Marshal(map[string]string{"status": status, "model_id": modelID, "revision": revision, "served_model": servedModel})
		return string(raw)
	}
	vllmRoot := "/root/.cache/huggingface/hub/models--meta-models--Muse-Glimmer-30B/snapshots/" + rev
	models := func(id, root string) string {
		raw, _ := json.Marshal(map[string]any{"object": "list", "data": []map[string]string{{"id": id, "root": root}}})
		return string(raw)
	}
	cases := []struct {
		name   string
		health string
		status int
		models string
		want   string
	}{
		{"identity match", identity("ok", model, rev, served), 200, "", "ok"},
		{"revision mismatch", identity("ok", model, "0000000", served), 200, "", "unavailable"},
		{"model id mismatch", identity("ok", "meta-llama/Other", rev, served), 200, "", "unavailable"},
		{"served model mismatch", identity("ok", model, rev, "other"), 200, "", "unavailable"},
		{"status not ok", identity("loading", model, rev, served), 200, "", "unavailable"},
		{"identity mismatch never falls back", identity("ok", model, "bad", served), 200, models(served, vllmRoot), "unavailable"},
		{"http 503", secretBody, 503, "", "unavailable"},
		{"vllm empty health, models match", "", 200, models(served, vllmRoot), "ok"},
		{"vllm empty health, wrong revision", "", 200, models(served, strings.Replace(vllmRoot, rev, "deadbeef", 1)), "unavailable"},
		{"vllm empty health, wrong model", "", 200, models("other", vllmRoot), "unavailable"},
		{"vllm empty health, wrong repo", "", 200, models(served, "/root/hub/models--meta-llama--Other/snapshots/"+rev), "unavailable"},
		{"vllm empty health, models fail", "", 200, "", "unavailable"},
		{"vllm empty health, trailing slash", "", 200, models(served, vllmRoot+"/"), "ok"},
		{"vllm empty health, model@revision", "", 200, models(served, model+"@"+rev), "ok"},
		{"vllm empty health, repo name extends model id", "", 200,
			models(served, "/root/.cache/huggingface/hub/models--meta-models--Muse-Glimmer-30B-Uncensored/snapshots/"+rev), "unavailable"},
		{"vllm empty health, revision nested in another snapshot", "", 200,
			models(served, "/root/.cache/huggingface/hub/models--meta-models--Muse-Glimmer-30B/snapshots/deadbeef/"+rev), "unavailable"},
		{"vllm empty health, revision only a substring", "", 200, models(served, vllmRoot+"0"), "unavailable"},
		{"vllm empty health, short revision in path", "", 200, models(served, strings.Replace(vllmRoot, rev, rev[:7], 1)), "unavailable"},
		{"vllm empty health, model@ with revision inside", "", 200, models(served, model+"@zz"+rev+"zz"), "unavailable"},
		{"vllm empty health, bare model id", "", 200, models(served, model), "unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var seen []http.Header
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				seen = append(seen, r.Header.Clone())
				mu.Unlock()
				switch r.URL.Path {
				case "/health":
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.health)
				case "/v1/models":
					if tc.models == "" {
						w.WriteHeader(500)
						return
					}
					fmt.Fprint(w, tc.models)
				default:
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()
			p, err := llm.New(llm.Config{
				Mode: llm.ModeModal, BaseURL: srv.URL, APIKey: "k1", ModalKey: "mk", ModalSecret: "ms",
				ServedModel: served, ModelID: model, Revision: rev, HTTPClient: srv.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			before := time.Now()
			h := p.Health(context.Background())
			if h.State != tc.want {
				t.Fatalf("state %q, want %q", h.State, tc.want)
			}
			if h.CheckedAt.Before(before.Add(-time.Second)) || h.Latency < 0 {
				t.Fatalf("health = %+v", h)
			}
			if tc.want == "ok" && (h.ModelID != model || h.Revision != rev) {
				t.Fatalf("health = %+v", h)
			}
			if tc.want != "ok" && (h.ModelID != "" || h.Revision != "") {
				t.Fatalf("unavailable health reports identity: %+v", h)
			}
			raw, _ := json.Marshal(h)
			if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "huggingface") {
				t.Fatalf("health leaks the body: %s", raw)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, hd := range seen {
				if hd.Get("Authorization") != "Bearer k1" || hd.Get("Modal-Key") != "mk" || hd.Get("Modal-Secret") != "ms" {
					t.Fatalf("health request without credentials: %v", hd)
				}
			}
		})
	}
}

func TestHealthHonoursContext(t *testing.T) {
	srv := server(t, hang)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	h := newTestDouble(t, srv.URL, nil).Health(ctx)
	if h.State != "unavailable" || time.Since(start) > 2*time.Second {
		t.Fatalf("health %+v after %v", h, time.Since(start))
	}
}

func TestKindOf(t *testing.T) {
	cases := map[llm.Kind]error{
		llm.KindAuth:        &llm.Error{Kind: llm.KindAuth, Status: 401},
		llm.KindCanceled:    fmt.Errorf("wrapped: %w", context.Canceled),
		llm.KindTimeout:     context.DeadlineExceeded,
		llm.KindUnavailable: errors.New("other"),
	}
	for want, err := range cases {
		if got := llm.KindOf(err); got != want {
			t.Fatalf("KindOf(%v) = %s, want %s", err, got, want)
		}
	}
}

func TestSamplingOverrideSendsAllThreeValues(t *testing.T) {
	var raw []byte
	var mu sync.Mutex
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		raw = b
		mu.Unlock()
		writeStream(w, servedModel, "ok")
	})
	p := newTestDouble(t, srv.URL, nil)
	req := testRequest()
	req.Sampling = &llm.Sampling{Temperature: 1, TopP: 0.95, TopK: 64}
	if _, err := p.Complete(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["temperature"] != float64(1) || body["top_p"] != 0.95 || body["top_k"] != float64(64) {
		t.Fatalf("sampling not sent: %v", body)
	}
}

func TestEmptyContentIsAFailure(t *testing.T) {
	// A reasoning model that exhausts max_tokens while thinking streams only
	// reasoning and ends with no content.
	for _, finish := range []string{"length", "stop"} {
		t.Run(finish, func(t *testing.T) {
			srv := server(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				blank := "  "
				fmt.Fprint(w, sseChunk(servedModel, &blank, "", false))
				fmt.Fprint(w, sseChunk(servedModel, nil, finish, false))
				fmt.Fprint(w, "data: [DONE]\n\n")
			})
			p := newTestDouble(t, srv.URL, nil)
			_, err := p.Complete(context.Background(), testRequest(), nil)
			if llm.KindOf(err) != llm.KindProtocol {
				t.Fatalf("want protocol error, got %v", err)
			}
		})
	}
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var listenRe = regexp.MustCompile(`listening on (http://\S+)`)

// start runs the command and returns its base URL and a stop function that
// reports run's error.
func start(t *testing.T, args ...string) (string, *syncBuffer, func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr syncBuffer
	errc := make(chan error, 1)
	go func() { errc <- run(ctx, args, &stdout, &stderr) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if m := listenRe.FindStringSubmatch(stdout.String()); m != nil {
			stop := func() error {
				cancel()
				select {
				case err := <-errc:
					return err
				case <-time.After(10 * time.Second):
					t.Fatal("run did not return after cancel")
					return nil
				}
			}
			return m[1], &stdout, stop
		}
		select {
		case err := <-errc:
			cancel()
			t.Fatalf("run exited early: %v (stderr %q)", err, stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("no startup line")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunServesTestDouble(t *testing.T) {
	base, stdout, stop := start(t, "-addr", "127.0.0.1:0", "-api-key", "secret-key-123", "-latency", "1ms")
	line := stdout.String()
	for _, want := range []string{"TEST DOUBLE", "not Meta", "model=test-double-not-meta", "auth=bearer"} {
		if !strings.Contains(line, want) {
			t.Fatalf("startup line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "secret-key-123") {
		t.Fatal("startup line prints the API key")
	}
	if strings.Contains(strings.ToLower(line), "llama") {
		t.Fatal("startup line mentions llama")
	}

	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	var health map[string]string
	err = json.NewDecoder(resp.Body).Decode(&health)
	resp.Body.Close()
	if err != nil || health["served_model"] != "test-double-not-meta" || resp.Header.Get("X-Sela-Test-Double") != "true" {
		t.Fatalf("health = %v, header %q, err %v", health, resp.Header.Get("X-Sela-Test-Double"), err)
	}

	body := `{"model":"test-double-not-meta","stream":false,"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", strings.NewReader(body))
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("without key: status %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, base+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-key-123")
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(b), `"model":"test-double-not-meta"`) {
		t.Fatalf("with key: status %d", resp.StatusCode)
	}
	if err := stop(); err != nil {
		t.Fatalf("run returned %v", err)
	}
}

func TestRunFaultFlag(t *testing.T) {
	base, stdout, stop := start(t, "-addr", "127.0.0.1:0", "-fault", "http_503")
	if !strings.Contains(stdout.String(), "fault=http_503") {
		t.Fatalf("startup line %q", stdout.String())
	}
	resp, err := http.Post(base+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test-double-not-meta","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestRunShutsDownWithStalledStream(t *testing.T) {
	base, _, stop := start(t, "-addr", "127.0.0.1:0", "-fault", "stall")
	resp, err := http.Post(base+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test-double-not-meta","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 64)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatal(err)
	}
	if err := stop(); err != nil {
		t.Fatalf("run returned %v", err)
	}
}

func TestRunProviderDoubles(t *testing.T) {
	base, stdout, stop := start(t, "-addr", "127.0.0.1:0", "-provider-fault", "refusal")
	line := stdout.String()
	for _, want := range []string{"claude_double=test-double-not-claude", "deepseek_double=test-double-not-deepseek",
		"provider_fault=refusal", "not Claude, not DeepSeek", "not Meta"} {
		if !strings.Contains(line, want) {
			t.Fatalf("startup line %q lacks %q", line, want)
		}
	}
	body := `{"model":"test-double-not-claude","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
	req.Header.Set("anthropic-version", "2026-10-21")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Sela-Test-Double") != "true" || !strings.Contains(string(b), `"stop_reason":"refusal"`) {
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
	resp, err = http.Post(base+"/chat/completions", "application/json",
		strings.NewReader(`{"model":"test-double-not-deepseek","messages":[{"role":"user","content":"json please"}],"response_format":{"type":"json_object"}}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(b), `"finish_reason":"content_filter"`) || !strings.Contains(string(b), "test-double-not-deepseek") {
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-provider-fault", "explode"},
		{"-slow-delay", "-1s"},
		{"-fault", "explode"},
		{"-latency", "-1s"},
		{"-addr", "127.0.0.1:0", "extra"},
		{"-unknown"},
	} {
		var out, errOut syncBuffer
		if err := run(context.Background(), args, &out, &errOut); err == nil {
			t.Fatalf("run(%q) succeeded", args)
		}
		if strings.Contains(out.String(), "listening") {
			t.Fatalf("run(%q) started listening", args)
		}
	}
}

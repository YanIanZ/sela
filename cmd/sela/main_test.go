package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/fixtures"
)

// TestMain lets the subprocess tests run the real main() from this binary;
// SELA_TEST_ARGS carries the subcommand and flags (space separated).
func TestMain(m *testing.M) {
	if os.Getenv("SELA_TEST_RUN_MAIN") == "1" {
		os.Args = append([]string{os.Args[0]}, strings.Fields(os.Getenv("SELA_TEST_ARGS"))...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
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

// A request still busy when the grace period ends is closed, and that is a
// clean stop: before, Shutdown's DeadlineExceeded made the process exit 1.
func TestServeTreatsGraceTimeoutAsCleanStop(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release // ignores shutdown, like a stalled upload
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stopped atomic.Bool
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, func() { stopped.Store(true); cancel() }, srv, ln, 200*time.Millisecond, slog.New(slog.NewJSONHandler(logs, nil)))
	}()
	clientErr := make(chan error, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/")
		if err == nil {
			res.Body.Close()
		}
		clientErr <- err
	}()
	<-entered
	t0 := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v after the grace period; want a clean stop", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not return after the grace period")
	}
	if el := time.Since(t0); el > 2*time.Second {
		t.Fatalf("stop took %s", el)
	}
	if !stopped.Load() {
		t.Fatal("signal handling was not restored, so a second signal would be swallowed")
	}
	if err := <-clientErr; err == nil {
		t.Fatal("the busy connection was not closed")
	}
	if !strings.Contains(logs.String(), "sela.stop_forced") {
		t.Fatalf("forced stop not logged: %s", logs.String())
	}
}

type proc struct {
	cmd    *exec.Cmd
	base   string
	stdout *syncBuffer
	stderr *syncBuffer
	exited chan struct{}
}

// startSela runs main() in a child process with a clean SELA_ environment.
func startSela(t *testing.T, extra ...string) *proc {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "SELA_") {
			env = append(env, kv)
		}
	}
	env = append(env, "SELA_TEST_RUN_MAIN=1", "SELA_ENV=development", "SELA_SECURE_COOKIES=false", "SELA_ADDR="+addr,
		"SELA_WEB_DIR="+t.TempDir(), "SELA_RESEARCH_KB="+t.TempDir()+"/none.jsonl")
	env = append(env, extra...)
	p := &proc{cmd: exec.Command(os.Args[0], "-test.run=^$"), base: "http://" + addr, stdout: &syncBuffer{}, stderr: &syncBuffer{}, exited: make(chan struct{})}
	p.cmd.Env, p.cmd.Stdout, p.cmd.Stderr = env, p.stdout, p.stderr
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = p.cmd.Wait(); close(p.exited) }()
	t.Cleanup(func() {
		select {
		case <-p.exited:
		default:
			_ = p.cmd.Process.Kill()
			<-p.exited
		}
	})
	for deadline := time.Now().Add(15 * time.Second); ; {
		if res, err := http.Get(p.base + "/api/v2/health"); err == nil {
			res.Body.Close()
			return p
		}
		select {
		case <-p.exited:
			t.Fatalf("sela exited at start: %s", p.stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("sela did not start: %s", p.stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// wait returns how long the process took to exit after since, or fails.
func (p *proc) wait(t *testing.T, since time.Time, limit time.Duration) time.Duration {
	t.Helper()
	select {
	case <-p.exited:
		return time.Since(since)
	case <-time.After(limit):
		t.Fatalf("sela still running %s after the signal; stderr: %s", limit, p.stderr.String())
	}
	return 0
}

func call(t *testing.T, hc *http.Client, method, url string, body any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sela-Request", "1")
	res, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %v", method, url, res.StatusCode, out)
	}
	d, _ := out["data"].(map[string]any)
	return d
}

func ver(d map[string]any) int {
	if s, ok := d["session"].(map[string]any); ok {
		d = s
	}
	v, _ := d["version"].(float64)
	return int(v)
}

// Review finding: SIGTERM with an open event stream waited the full 10s,
// cut the stream without a terminal event and exited 1.
func TestSIGTERMWithOpenStreamStopsPromptlyAndCleanly(t *testing.T) {
	// The model never answers, so the operation and its stream stay open.
	quit := make(chan struct{})
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-quit:
		}
	}))
	t.Cleanup(model.Close)
	t.Cleanup(func() { close(quit) })
	p := startSela(t, "SELA_LLM_MODE=test_double", "SELA_ALLOW_TEST_DOUBLE=true", "SELA_LLM_BASE_URL="+model.URL)

	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	cat := fixtures.MustLoad()
	d := call(t, hc, "POST", p.base+"/api/v2/sessions", map[string]any{"language": "id", "adultAttested": true, "privacyPolicy": cat.Policies["privacy"].Version})
	d = call(t, hc, "PUT", p.base+"/api/v2/consents/ai_processing", map[string]any{"expectedVersion": ver(d), "granted": true, "policyVersion": cat.Policies["ai_processing"].Version})
	canary := "Tokoh fiktif CANARY-SIGTERM-52 merasa lelah setiap hari karena pekerjaan barunya."
	d = call(t, hc, "PUT", p.base+"/api/v2/story", map[string]any{"expectedVersion": ver(d), "mode": "practice", "text": canary})
	d = call(t, hc, "POST", p.base+"/api/v2/ledger/draft", map[string]any{"expectedVersion": ver(d)})
	op := d["operation"].(map[string]any)["id"].(string)

	res, err := hc.Get(p.base + "/api/v2/operations/" + op + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	for l := range lines {
		if strings.HasPrefix(l, "event: status") {
			break
		}
	}
	t0 := time.Now()
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	el := p.wait(t, t0, 5*time.Second)
	if code := p.cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("exit code %d after %s; stderr: %s", code, el, p.stderr.String())
	}
	if el > 3*time.Second {
		t.Fatalf("shutdown took %s with one open stream", el)
	}
	var tail []string
	for l := range lines {
		tail = append(tail, l)
	}
	if got := strings.Join(tail, "\n"); !strings.Contains(got, "event: error\ndata: {\"code\":\"ai_unavailable\",\"reason\":\"server_stopping\"}") {
		t.Fatalf("stream ended without a final event: %q", got)
	}
	logs := p.stdout.String()
	if !strings.Contains(logs, `"msg":"sela.stop"`) {
		t.Fatalf("no stop log: %s", logs)
	}
	if strings.Contains(logs, "CANARY") || strings.Contains(logs, "merasa lelah") {
		t.Fatal("logs contain story text")
	}
}

// Review finding: signal.NotifyContext stayed registered during shutdown, so
// a second Ctrl-C was swallowed while a request held shutdown open.
func TestSecondSignalForcesExit(t *testing.T) {
	p := startSela(t)
	// A stalled upload keeps one request active through graceful shutdown.
	conn, err := net.Dial("tcp", strings.TrimPrefix(p.base, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /api/v2/sessions HTTP/1.1\r\nHost: %s\r\nX-Sela-Request: 1\r\nContent-Type: application/json\r\nContent-Length: 200\r\n\r\n{\"language\":", strings.TrimPrefix(p.base, "http://"))
	time.Sleep(300 * time.Millisecond)
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// Shutdown has begun once the listener refuses new connections.
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		c, err := net.DialTimeout("tcp", strings.TrimPrefix(p.base, "http://"), 200*time.Millisecond)
		if err != nil {
			break
		}
		c.Close()
		if time.Now().After(deadline) {
			t.Fatal("listener still open after SIGTERM")
		}
	}
	select {
	case <-p.exited:
		t.Fatal("graceful shutdown did not wait for the active request")
	default:
	}
	t0 := time.Now()
	if err := p.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	el := p.wait(t, t0, 3*time.Second)
	ws, _ := p.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		t.Fatalf("second signal did not terminate the process (status %v after %s)", p.cmd.ProcessState, el)
	}
}

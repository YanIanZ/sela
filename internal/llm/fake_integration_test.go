package llm_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

// fakeServer starts the cmd/fakellm handler with cfg.
func fakeServer(t testing.TB, cfg fakellm.Config) *httptest.Server {
	t.Helper()
	h, err := fakellm.NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// ledgerRequest wraps a ledger_draft input the way internal/journey does.
func ledgerRequest(t testing.TB) llm.Request {
	t.Helper()
	in, err := json.Marshal(map[string]any{
		"task": "ledger_draft", "schema": "sela-ledger-draft-v1", "language": "id",
		"segments": []map[string]string{
			{"id": "s1", "text": "Aku capek banget dengan skripsi."},
			{"id": "s2", "text": "Aku nggak bisa tidur sejak bulan lalu."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := testRequest()
	req.Messages[1].Content = fakellm.InputOpenMarker + "\n" + string(in) + "\n" + fakellm.InputCloseMarker
	return req
}

func TestFakeLLMHappyPath(t *testing.T) {
	srv := fakeServer(t, fakellm.Config{APIKey: "fake-key"})
	var ev events
	p := newTestDouble(t, srv.URL, func(c *llm.Config) { c.APIKey = "fake-key"; c.Observer = ev.observe })
	var deltas []string
	res, err := p.Complete(context.Background(), ledgerRequest(t), func(d string) error {
		deltas = append(deltas, d)
		return nil
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Model != fakellm.ServedModel || res.FinishReason != "stop" || res.Attempts != 1 || res.PromptTokens <= 0 || res.CompletionTokens <= 0 {
		t.Fatalf("result = %+v", res)
	}
	if len(deltas) < 5 || strings.Join(deltas, "") != res.Text {
		t.Fatalf("%d deltas do not assemble the text", len(deltas))
	}
	var d struct {
		Claims []struct {
			SegmentID string `json:"segment_id"`
			Quote     string `json:"quote"`
			Negated   bool   `json:"negated"`
		} `json:"claims"`
	}
	if err := json.Unmarshal([]byte(res.Text), &d); err != nil {
		t.Fatalf("text is not JSON: %v", err)
	}
	if len(d.Claims) != 2 || d.Claims[1].Quote != "Aku nggak bisa tidur sejak bulan lalu." || !d.Claims[1].Negated {
		t.Fatalf("draft = %+v", d)
	}
	if got := ev.list(); len(got) != 1 || got[0].Outcome != "ok" || got[0].CompletionTokens != res.CompletionTokens {
		t.Fatalf("events = %+v", got)
	}
	if h := p.Health(context.Background()); h.State != "ok" || h.ModelID != fakellm.ModelID || h.Revision != fakellm.Revision {
		t.Fatalf("health = %+v", h)
	}
	mismatch := newTestDouble(t, srv.URL, func(c *llm.Config) { c.ModelID = "meta-models/Muse-Glimmer-30B" })
	if h := mismatch.Health(context.Background()); h.State != "unavailable" {
		t.Fatalf("mismatched identity reported %q", h.State)
	}
}

func TestFakeLLMWrongAPIKey(t *testing.T) {
	srv := fakeServer(t, fakellm.Config{APIKey: "fake-key"})
	res, err := newTestDouble(t, srv.URL, func(c *llm.Config) { c.APIKey = "wrong" }).Complete(context.Background(), ledgerRequest(t), nil)
	if llm.KindOf(err) != llm.KindAuth || res.Attempts != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFakeLLMFaults(t *testing.T) {
	cases := []struct {
		fault    fakellm.Fault
		mut      func(*llm.Config)
		ctx      time.Duration
		kind     llm.Kind // "" means success
		attempts int
		deltas   int // minimum deltas delivered
	}{
		{fault: fakellm.FaultWrongModel, kind: llm.KindModelMismatch, attempts: 1},
		{fault: fakellm.FaultOversize, kind: llm.KindOutputTooLong, attempts: 1, deltas: 1},
		{fault: fakellm.FaultTruncatedStream, kind: llm.KindProtocol, attempts: 1, deltas: 1},
		{fault: fakellm.FaultHTTP401, kind: llm.KindAuth, attempts: 1},
		{fault: fakellm.FaultHTTP503, mut: func(c *llm.Config) { c.MaxRetries = 1 }, kind: llm.KindUnavailable, attempts: 2},
		// Retry-After: 1 does not fit in the deadline, so no retry.
		{fault: fakellm.FaultHTTP429, ctx: 700 * time.Millisecond, kind: llm.KindRateLimited, attempts: 1},
		{fault: fakellm.FaultHTTP429, mut: func(c *llm.Config) { c.MaxRetries = 1 }, kind: llm.KindRateLimited, attempts: 2},
		// One chunk, then silence: the attempt times out and is not retried
		// because content already started.
		{fault: fakellm.FaultStall, mut: func(c *llm.Config) { c.AttemptTimeout = 300 * time.Millisecond }, kind: llm.KindTimeout, attempts: 1, deltas: 1},
		{fault: fakellm.FaultSlow, ctx: 300 * time.Millisecond, kind: llm.KindTimeout, attempts: 1},
		// Content faults keep the stream valid; the caller's validator
		// handles the content.
		{fault: fakellm.FaultMalformedJSON, attempts: 1, deltas: 1},
		{fault: fakellm.FaultFabricatedSource, attempts: 1, deltas: 1},
		{fault: fakellm.FaultDiagnosis, attempts: 1, deltas: 1},
	}
	for _, tc := range cases {
		t.Run(string(tc.fault), func(t *testing.T) {
			t.Parallel()
			srv := fakeServer(t, fakellm.Config{Fault: tc.fault, SlowDelay: 3 * time.Second})
			ctx := context.Background()
			if tc.ctx > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.ctx)
				defer cancel()
			}
			n := 0
			res, err := newTestDouble(t, srv.URL, tc.mut).Complete(ctx, ledgerRequest(t), func(string) error {
				n++
				return nil
			})
			if tc.kind == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
			} else if llm.KindOf(err) != tc.kind {
				t.Fatalf("err = %v, want %s", err, tc.kind)
			}
			if res.Attempts != tc.attempts || n < tc.deltas {
				t.Fatalf("attempts %d (want %d), deltas %d (want >= %d)", res.Attempts, tc.attempts, n, tc.deltas)
			}
			if err != nil && strings.Contains(err.Error(), "test double:") {
				t.Fatalf("error %q contains the server body", err)
			}
		})
	}
}

func TestFakeLLMConcurrentWithAdmission(t *testing.T) {
	srv := fakeServer(t, fakellm.Config{Latency: time.Millisecond})
	p := newTestDouble(t, srv.URL, nil)
	adm := llm.NewAdmission(3)
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		active int
		peak   int
	)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := adm.Acquire(context.Background())
			if err != nil {
				errs <- err
				return
			}
			defer release()
			mu.Lock()
			active++
			peak = max(peak, active)
			mu.Unlock()
			_, err = p.Complete(context.Background(), ledgerRequest(t), nil)
			mu.Lock()
			active--
			mu.Unlock()
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
	if peak > 3 || adm.InUse() != 0 {
		t.Fatalf("peak %d, in use %d", peak, adm.InUse())
	}
}

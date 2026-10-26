package ai

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

type llmStub struct {
	mode llm.Mode
	fn   func(ctx context.Context, req llm.Request, onDelta func(string) error) (llm.Result, error)
	last llm.Request
}

func (s *llmStub) Info() llm.Info {
	return llm.Info{Mode: s.mode, ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc"}
}

func (s *llmStub) Complete(ctx context.Context, req llm.Request, onDelta func(string) error) (llm.Result, error) {
	s.last = req
	return s.fn(ctx, req, onDelta)
}

func (s *llmStub) Health(context.Context) llm.Health { return llm.Health{} }

func failWith(kind llm.Kind) func(context.Context, llm.Request, func(string) error) (llm.Result, error) {
	return func(context.Context, llm.Request, func(string) error) (llm.Result, error) {
		return llm.Result{Attempts: 1}, &llm.Error{Kind: kind, Status: 503}
	}
}

// TestMetaClassification maps every llm error kind to its ADR-0003 §4
// failure class and, for the upstream Meta availability failures only, to
// a fallback_reason (§10, §17 O-1): a credential problem on Sela's side is
// an availability failure with no fallback reason.
func TestMetaClassification(t *testing.T) {
	cases := []struct {
		kind     llm.Kind
		class    transport.Class
		fallback string
		cause    string
	}{
		{llm.KindUnavailable, transport.ClassAvailability, FallbackUnavailable, ""},
		{llm.KindRateLimited, transport.ClassAvailability, FallbackRateLimited, ""},
		{llm.KindAuth, transport.ClassAvailability, "", CauseAuth},
		{llm.KindTimeout, transport.ClassAvailability, FallbackTimeout, ""},
		{llm.KindModelMismatch, transport.ClassIntegrity, "", ""},
		{llm.KindProtocol, transport.ClassContent, "", ""},
		{llm.KindOutputTooLong, transport.ClassContent, "", ""},
		{llm.KindRejected, transport.ClassPolicy, "", ""},
		{llm.KindDisabled, transport.ClassPolicy, "", ""},
		{llm.KindCanceled, transport.ClassPolicy, "", ""},
	}
	for _, tc := range cases {
		g := newMetaGenerator(&llmStub{mode: llm.ModeModal, fn: failWith(tc.kind)}, nil, 10*time.Millisecond)
		_, err := g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u"})
		var ge *GenerateError
		if !errors.As(err, &ge) || ge.Class != tc.class || ge.FallbackReason != tc.fallback || ge.Cause != tc.cause || ge.ContentStarted {
			t.Errorf("%s: %+v", tc.kind, ge)
		}
	}
	// A certificate failure is not clearly transient: no fallback reason.
	g := newMetaGenerator(&llmStub{mode: llm.ModeModal, fn: func(context.Context, llm.Request, func(string) error) (llm.Result, error) {
		return llm.Result{Attempts: 1}, &llm.Error{Kind: llm.KindUnavailable, Err: &tls.CertificateVerificationError{Err: errors.New("x509")}}
	}}, nil, 10*time.Millisecond)
	_, err := g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u"})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.Class != transport.ClassAvailability || ge.FallbackReason != "" || ge.Cause != CauseTLS {
		t.Errorf("certificate failure: %+v", ge)
	}
	for _, fr := range FallbackReasons() {
		switch fr {
		case "cold_start_budget", "unavailable", "timeout", "rate_limited":
		default:
			t.Errorf("fallback_reason %q is not in ADR-0003 §10", fr)
		}
	}
	if len(FallbackReasons()) != 4 {
		t.Errorf("fallback reasons %v", FallbackReasons())
	}
}

// TestMetaBreakerOpenNoColdStartWait: with the RR-14 breaker open, Meta is
// still called, and Modal's 503 (no container ready) ends the call at once
// instead of a cold-start wait (§17 O-12).
func TestMetaBreakerOpenNoColdStartWait(t *testing.T) {
	var calls int
	stub := &llmStub{mode: llm.ModeModal, fn: func(context.Context, llm.Request, func(string) error) (llm.Result, error) {
		calls++
		return llm.Result{Attempts: 1}, &llm.Error{Kind: llm.KindUnavailable, Status: 503}
	}}
	g := newMetaGenerator(stub, nil, 10*time.Millisecond)
	_, err := g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u",
		FirstContentWait: time.Second, NoColdStartWait: true})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.FallbackReason != FallbackUnavailable || calls != 1 {
		t.Fatalf("calls %d %+v", calls, ge)
	}
	// Without it, the 503 is polled while the window lasts.
	calls = 0
	_, _ = g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u", FirstContentWait: 100 * time.Millisecond})
	if calls < 2 {
		t.Fatalf("cold start not polled: %d calls", calls)
	}
}

// TestMetaColdStartBeyondBudget: no content within the first-content wait
// is an availability failure with reason cold_start_budget.
func TestMetaColdStartBeyondBudget(t *testing.T) {
	stub := &llmStub{mode: llm.ModeModal, fn: func(ctx context.Context, _ llm.Request, _ func(string) error) (llm.Result, error) {
		<-ctx.Done()
		return llm.Result{}, &llm.Error{Kind: llm.KindCanceled, Err: context.Canceled}
	}}
	g := newMetaGenerator(stub, nil, 10*time.Millisecond)
	start := time.Now()
	_, err := g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u", FirstContentWait: 50 * time.Millisecond})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.Class != transport.ClassAvailability || ge.FallbackReason != FallbackColdStart || time.Since(start) > time.Second {
		t.Fatalf("%+v after %s", ge, time.Since(start))
	}
}

// TestMetaFailureAfterContent: any failure after the first delta is
// content-class and records that content started.
func TestMetaFailureAfterContent(t *testing.T) {
	stub := &llmStub{mode: llm.ModeModal, fn: func(ctx context.Context, _ llm.Request, onDelta func(string) error) (llm.Result, error) {
		if err := onDelta(`{"declined":`); err != nil {
			return llm.Result{}, err
		}
		return llm.Result{FirstToken: time.Millisecond}, &llm.Error{Kind: llm.KindUnavailable}
	}}
	g := newMetaGenerator(stub, nil, 10*time.Millisecond)
	_, err := g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u", FirstContentWait: time.Second})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.Class != transport.ClassContent || !ge.ContentStarted {
		t.Fatalf("%+v", ge)
	}
}

// TestMetaRequest: the adapter keeps the llm API: system and user
// messages only, the generation schema, low reasoning strength.
func TestMetaRequest(t *testing.T) {
	stub := &llmStub{mode: llm.ModeModal, fn: func(ctx context.Context, _ llm.Request, onDelta func(string) error) (llm.Result, error) {
		_ = onDelta("x")
		return llm.Result{Text: "x", Model: "sela-glimmer-cec94fc", PromptTokens: 5, CompletionTokens: 1, Attempts: 1}, nil
	}}
	g := newMetaGenerator(stub, nil, 10*time.Millisecond)
	res, err := g.Generate(context.Background(), GenerateRequest{Task: TaskReflection, System: "sys", User: "usr", Schema: GenerationSchemaJSON, MaxTokens: 99})
	if err != nil {
		t.Fatal(err)
	}
	r := stub.last
	if len(r.Messages) != 2 || r.Messages[0].Role != llm.RoleSystem || r.Messages[1].Role != llm.RoleUser || r.JSONSchema != GenerationSchemaJSON ||
		r.ReasoningStrength != llm.ReasoningLow || r.MaxTokens != 99 || r.Sampling != nil || r.Operation != "ai_reflection" {
		t.Fatalf("request %+v", r)
	}
	if res.ServedModel != "sela-glimmer-cec94fc" || res.ServedRevision != "cec94fc" || res.InputTokens != 5 {
		t.Fatalf("result %+v", res)
	}
	id := g.Identity()
	if id.Role != ProviderMeta || id.ModelID != "meta-models/Muse-Glimmer-30B" || id.TestDouble || id.Disabled {
		t.Fatalf("identity %+v", id)
	}
	if !newMetaGenerator(&llmStub{mode: llm.ModeDisabled}, nil, 10*time.Millisecond).Identity().Disabled {
		t.Fatal("disabled mode")
	}
	_, err = newMetaGenerator(&llmStub{mode: llm.ModeDisabled}, nil, 10*time.Millisecond).Generate(context.Background(), GenerateRequest{})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.Reason != transport.ReasonDisabled {
		t.Fatalf("disabled: %v", err)
	}
}

// TestMetaCapacity: a full admission queue past the wait is an
// availability failure on Sela's side (capacity), never a fallback reason
// (§17 O-1).
func TestMetaCapacity(t *testing.T) {
	adm := llm.NewAdmission(1)
	release, _ := adm.Acquire(context.Background())
	defer release()
	g := newMetaGenerator(&llmStub{mode: llm.ModeModal, fn: failWith(llm.KindUnavailable)}, adm, 10*time.Millisecond)
	_, err := g.Generate(context.Background(), GenerateRequest{FirstContentWait: 30 * time.Millisecond})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.FallbackReason != "" || ge.Cause != CauseCapacity || ge.Class != transport.ClassAvailability {
		t.Fatalf("%+v", ge)
	}
}

// TestMetaThroughLLMClient wraps the real internal/llm client against the
// Meta test double: a 503 is an availability failure, a stall after the
// first chunk is content-class.
func TestMetaThroughLLMClient(t *testing.T) {
	for _, tc := range []struct {
		fault   fakellm.Fault
		class   transport.Class
		started bool
	}{
		{fakellm.FaultHTTP503, transport.ClassAvailability, false},
		{fakellm.FaultHTTP401, transport.ClassAvailability, false},
		{fakellm.FaultStall, transport.ClassContent, true},
		{fakellm.FaultWrongModel, transport.ClassIntegrity, false},
		{fakellm.FaultTruncatedStream, transport.ClassContent, true},
	} {
		h, err := fakellm.NewHandler(fakellm.Config{Fault: tc.fault})
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(h)
		g, err := NewMetaGenerator(llm.Config{Mode: llm.ModeTestDouble, BaseURL: srv.URL, ServedModel: llm.TestDoubleServedModel,
			AttemptTimeout: 300 * time.Millisecond}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !g.Identity().TestDouble {
			t.Fatal("test double not labelled")
		}
		_, err = g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u", FirstContentWait: 2 * time.Second})
		srv.Close()
		var ge *GenerateError
		if !errors.As(err, &ge) || ge.Class != tc.class || ge.ContentStarted != tc.started {
			t.Errorf("%s: %+v", tc.fault, ge)
		}
	}
}

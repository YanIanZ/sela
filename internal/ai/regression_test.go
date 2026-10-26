package ai

// Regression tests for the AI-layer review findings (policy and privacy
// rounds, 2026-10-26). Each test names the rule it pins.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/anthropic"
	"github.com/YanIanZ/sela/internal/ai/deepseek"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

func thirteenItems(t *testing.T) (*harness, Input) {
	var items []string
	for i := 0; i < 13; i++ {
		items = append(items, "Aku mencatat hal nomor "+itoa(i)+" minggu ini.")
	}
	return summaryHarness(t, items...)
}

// VC-8, RR-7/CK-9, RR-10: a validated Claude report is never discarded.
// Claude stops at the first report that withholds the output, and a later
// Claude outage never hands the output to the alternative verifier.
func TestNoVerifierShoppingAcrossChunks(t *testing.T) {
	t.Run("safety signal in chunk 1 stops at once", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.claude.steps = []verStep{
			{report: func(r verifier.Request) verifier.Report {
				return withVerdicts(r, map[int]verifier.Verdict{0: verifier.Contradicted},
					verifier.Issue{Category: verifier.SafetySignal, ClaimID: "c1", SpanIDs: []string{"s1"}, Reason: "risk"})
			}},
			{err: verr(transport.ReasonUnavailable)},
		}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeSafeStop, ReasonSafetySignal)
		if !re.ShowHelp || h.claude.count() != 1 || h.alt.count() != 0 || re.Provenance.Guards.Risk != RiskElevated {
			t.Fatalf("help %v claude %d alt %d risk %s", re.ShowHelp, h.claude.count(), h.alt.count(), re.Provenance.Guards.Risk)
		}
		v := re.Provenance.Verification
		if v.Status != StatusChecked || v.Provider != ProviderAnthropic || len(v.Attempts) != 1 || v.Attempts[0].Outcome != "withheld" ||
			len(v.ClaimOutcomes) != 12 || v.ClaimOutcomes[0].Verdict != "contradicted" {
			t.Fatalf("verification %+v", v)
		}
	})
	t.Run("output-level issue in chunk 1: chunk 2 still goes to Claude for CK-9, never to the alternative", func(t *testing.T) {
		// Review round 3: the early stop also stopped CK-9 for the rest.
		// An outage on chunk 2 keeps the withhold and is recorded.
		h, in := thirteenItems(t)
		h.claude.steps = []verStep{
			{report: func(r verifier.Request) verifier.Report {
				return withVerdicts(r, nil, verifier.Issue{Category: verifier.InformationLeak, ClaimID: "", SpanIDs: []string{}, Reason: "x"})
			}},
			{err: verr(transport.ReasonUnavailable)},
		}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, ReasonWithheld)
		expect(t, re, OutcomeAbstain, ReasonSafetyPassIncomplete)
		v := re.Provenance.Verification
		if h.claude.count() != 2 || h.alt.count() != 0 || v.Status != StatusChecked || v.SafetyPassIncomplete != "unavailable" ||
			len(v.Attempts) != 1 || v.Attempts[0].Outcome != "withheld" || re.ShowHelp {
			t.Fatalf("claude %d alt %d verification %+v", h.claude.count(), h.alt.count(), v)
		}
	})
	t.Run("outage after a report ends without the alternative", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.claude.steps = []verStep{
			{report: func(r verifier.Request) verifier.Report {
				return withVerdicts(r, map[int]verifier.Verdict{0: verifier.Contradicted})
			}},
			{err: verr(transport.ReasonUnavailable)},
		}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, StatusVerificationUnavailable)
		expect(t, re, OutcomeAbstain, ReasonPartialReport)
		v := re.Provenance.Verification
		if h.claude.count() != 2 || h.alt.count() != 0 || v.Status != StatusVerificationUnavailable ||
			len(v.UnavailableReasons) != 2 || v.UnavailableReasons[0] != "unavailable" || v.UnavailableReasons[1] != ReasonPartialReport {
			t.Fatalf("claude %d alt %d verification %+v", h.claude.count(), h.alt.count(), v)
		}
	})
	t.Run("outage before any report may use the alternative", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
		out, re := h.run(TaskSummaryDraft, in)
		if re != nil || h.alt.count() != 2 || out.Provenance.Verification.Route != AlternativeVerifierRoute {
			t.Fatalf("err %v alt %d", re, h.alt.count())
		}
	})
}

// §10, D-029: one sub-record per verifier; the used report's provider,
// calls and tokens are its own, and Claude's failure reason is kept.
func TestAlternativeVerifierProvenanceIsSeparate(t *testing.T) {
	h, in := thirteenItems(t)
	h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
	out, re := h.run(TaskSummaryDraft, in)
	if re != nil {
		t.Fatal(re)
	}
	v := out.Provenance.Verification
	if v.Provider != ProviderDeepSeek || v.ModelID != "deepseek-flash" || v.Calls != 2 || v.InputTokens != 600 || v.OutputTokens != 120 ||
		len(v.UnavailableReasons) != 1 || v.UnavailableReasons[0] != "unavailable" || v.InferenceGeo != "" {
		t.Fatalf("verification %+v", v)
	}
	if len(v.Attempts) != 2 {
		t.Fatalf("attempts %+v", v.Attempts)
	}
	c, a := v.Attempts[0], v.Attempts[1]
	if c.Provider != ProviderAnthropic || c.Route != VerifierRoute || c.Calls != 1 || c.Reports != 0 || c.Outcome != "unavailable" ||
		c.InferenceGeo != "us" || a.Provider != ProviderDeepSeek || a.Route != AlternativeVerifierRoute || a.Calls != 2 || a.Reports != 2 ||
		a.Outcome != StatusChecked || a.InputTokens != 600 || a.Label != "alternative verifier (DeepSeek)" {
		t.Fatalf("attempts %+v", v.Attempts)
	}
	if out.Provenance.InputTokens != 100+600 {
		t.Fatalf("total tokens %d", out.Provenance.InputTokens)
	}
}

// RR-10: an error that is not a *transport.Error fails closed as invalid
// output and never reaches the alternative verifier.
func TestUnknownVerifierErrorFailsClosed(t *testing.T) {
	for _, err := range []error{verifier.ErrInvalidOutput, errors.New("custom verifier failure")} {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.claude.steps = []verStep{{err: err}}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, string(transport.ReasonInvalidOutput))
		if h.alt.count() != 0 {
			t.Fatalf("%v reached the alternative verifier", err)
		}
	}
	if verifierReason(context.DeadlineExceeded) != transport.ReasonTimeout || verifierReason(context.Canceled) != transport.ReasonCanceled ||
		verifierReason(verr(transport.ReasonRateLimited)) != transport.ReasonRateLimited {
		t.Fatal("verifierReason")
	}
}

// RR-3, RR-4(a): a budget too short for Meta's window and the reserves is
// the no-AI path or invalid input, never a Meta failure, and DeepSeek is
// never the primary.
func TestShortBudgetNeverSkipsMeta(t *testing.T) {
	for _, op := range []time.Duration{2 * time.Minute, 100 * time.Second, 150 * time.Second} {
		cfg := testConfig()
		cfg.OpTimeout = op
		if _, err := NewRouter(cfg, Deps{Meta: newGen(t, ProviderMeta, nil), Consent: allConsent()}); err == nil {
			t.Errorf("op timeout %s accepted", op)
		}
		if _, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "development", "SELA_LLM_OP_TIMEOUT": op.String()})); err == nil {
			t.Errorf("SELA_LLM_OP_TIMEOUT=%s accepted", op)
		}
	}
	if MinOpTimeout() != 60*time.Second+90*time.Second+MinMetaWindow+reserveMargin {
		t.Fatalf("MinOpTimeout %s", MinOpTimeout())
	}
	cfg := testConfig()
	cfg.OpTimeout = MinOpTimeout()
	if _, err := NewRouter(cfg, Deps{Meta: newGen(t, ProviderMeta, nil), Consent: allConsent()}); err != nil {
		t.Fatalf("MinOpTimeout refused: %v", err)
	}

	t.Run("caller budget below the reserves", func(t *testing.T) {
		for task, budget := range map[Task]time.Duration{TaskSummaryDraft: 140 * time.Second, TaskMemoryProposal: 149 * time.Second,
			TaskReflection: 39 * time.Second, TaskListeningReply: 19 * time.Second} {
			h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
			if task != TaskSummaryDraft {
				in = inReply()
			}
			in.Budget = budget
			_, re := h.run(task, in)
			expect(t, re, OutcomeInvalidInput, ReasonBudgetReserves)
			noCalls(t, h)
		}
	})
	t.Run("budget that holds the reserves but not the window", func(t *testing.T) {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		in := inReply()
		in.Budget = 21 * time.Second // AT-1: 20 s reserve, 20 s window, 2 s margin
		_, re := h.run(TaskListeningReply, in)
		expect(t, re, OutcomeNoAI, ReasonBudget)
		if h.meta.count() != 0 || h.ds.count() != 0 || re.Provenance.FallbackReason != "" || re.Provenance.Route != RouteNone {
			t.Fatalf("meta %d deepseek %d provenance %+v", h.meta.count(), h.ds.count(), re.Provenance)
		}
	})
	t.Run("short budget keeps Meta primary without the fallback reserve", func(t *testing.T) {
		h := newHarness(t)
		out, re := h.run(TaskReflection, func() Input { in := inReply(); in.Budget = 40 * time.Second; return in }())
		if re != nil || h.meta.count() != 1 || h.ds.count() != 0 || out.Provenance.Route != RoutePrimary {
			t.Fatalf("err %v meta %d deepseek %d", re, h.meta.count(), h.ds.count())
		}
		if w := h.meta.req(0).FirstContentWait; w < MinMetaWindow || w > 38*time.Second {
			t.Fatalf("window %s", w)
		}
	})
	t.Run("default budget keeps both reserves", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		if _, re := h.run(TaskSummaryDraft, in); re != nil {
			t.Fatal(re)
		}
		w := h.meta.req(0).FirstContentWait
		if w > DefaultOpTimeout-150*time.Second-reserveMargin || w < DefaultOpTimeout-150*time.Second-reserveMargin-time.Second {
			t.Fatalf("AT-4 window %s", w)
		}
		if r := h.meta.req(0).MaxRetries; r != 2 {
			t.Fatalf("AT-4 Meta retries %d", r)
		}
	})
}

// §3.2: the DeepSeek fallback is bounded by its reserve and never eats
// into the verification reserve.
func TestFallbackIsBounded(t *testing.T) {
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h.meta.steps = []genStep{unavailableErr()}
	h.ds.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya susah tidur sejak dua bulan.", "s1"})}}
	out, re := h.run(TaskSummaryDraft, in)
	if re != nil || out.Provenance.Route != RouteFallback {
		t.Fatalf("err %v", re)
	}
	if got := h.ds.req(0).AttemptTimeout; got <= 0 || got > 60*time.Second {
		t.Fatalf("fallback attempt bound %s", got)
	}
	h2 := newHarness(t)
	h2.meta.steps = []genStep{unavailableErr()}
	if _, re := h2.run(TaskListeningReply, inReply()); re != nil {
		t.Fatal(re)
	}
	if got := h2.ds.req(0).AttemptTimeout; got <= 0 || got > 20*time.Second {
		t.Fatalf("AT-1 fallback attempt bound %s", got)
	}
}

// modalServer is a Modal-like TLS server for the real internal/llm client
// in modal mode: it answers 503 (no container ready) for the first cold
// requests, then streams content as the pinned served model.
func modalServer(t *testing.T, cold int, content string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(n.Add(1)) <= cold {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		raw, _ := json.Marshal(content)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"model\":\"sela-glimmer-cec94fc\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%s},\"finish_reason\":null}]}\n\n", raw)
		fmt.Fprint(w, "data: {\"model\":\"sela-glimmer-cec94fc\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

// modalGenerator builds the modal-mode generator for a local TLS server.
// NewMetaGenerator refuses a local host in modal mode, so this calls the
// constructor behind its host checks.
func modalGenerator(t *testing.T, srv *httptest.Server, poll time.Duration) Generator {
	t.Helper()
	g, err := newMetaFromLLM(llm.Config{Mode: llm.ModeModal, BaseURL: srv.URL, APIKey: "k", ModalKey: "mk", ModalSecret: "ms",
		ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fcfd6512f2cfe708138c32533b346ea4f69",
		HTTPClient: srv.Client(), AttemptTimeout: 2 * time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.(*metaGenerator).poll = poll
	return g
}

// §3.2, RR-14: AT-3/AT-4 (and AT-1/2/5 within their windows) wait for a
// Modal cold start: a 503 before content is polled until the window ends,
// and DeepSeek is not called while Meta is starting.
func TestModalColdStartIsWaitedFor(t *testing.T) {
	srv, hits := modalServer(t, 4, genJSON("none", false, [2]string{"Saya susah tidur sejak dua bulan.", "s1"}))
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h.meta.consent = nil
	r, err := NewRouter(h.cfg, Deps{Meta: modalGenerator(t, srv, 30*time.Millisecond), DeepSeek: h.ds, Claude: h.claude,
		AltVerifier: h.alt, Consent: h.c, Screener: screenFake{}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Run(context.Background(), TaskSummaryDraft, in)
	if err != nil {
		t.Fatal(err)
	}
	p := out.Provenance
	if h.ds.count() != 0 || p.Route != RoutePrimary || p.FallbackReason != "" || hits.Load() != 5 || p.Generator.Attempts != 5 {
		t.Fatalf("deepseek %d route %s reason %q hits %d attempts %d", h.ds.count(), p.Route, p.FallbackReason, hits.Load(), p.Generator.Attempts)
	}
}

// A cold start that outlasts the window is cold_start_budget, and only
// then; a test double's 503 is an ordinary bounded retry.
func TestModalColdStartBeyondWindow(t *testing.T) {
	srv, hits := modalServer(t, 1<<30, "")
	g := modalGenerator(t, srv, 40*time.Millisecond)
	start := time.Now()
	_, err := g.Generate(context.Background(), GenerateRequest{Task: TaskSummaryDraft, System: "s", User: "u",
		FirstContentWait: 300 * time.Millisecond, MaxRetries: 2})
	var ge *GenerateError
	if !errors.As(err, &ge) || ge.Class != transport.ClassAvailability || ge.FallbackReason != FallbackColdStart || hits.Load() < 4 ||
		time.Since(start) > 2*time.Second {
		t.Fatalf("err %+v hits %d after %s", ge, hits.Load(), time.Since(start))
	}

	var n atomic.Int32
	td := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer td.Close()
	g2, err := NewMetaGenerator(llm.Config{Mode: llm.ModeTestDouble, BaseURL: td.URL, ServedModel: llm.TestDoubleServedModel}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = g2.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u",
		FirstContentWait: 20 * time.Second, MaxRetries: 1})
	if !errors.As(err, &ge) || ge.FallbackReason != FallbackUnavailable || n.Load() != 2 {
		t.Fatalf("test double 503: %+v attempts %d", ge, n.Load())
	}
}

// §3.2: Meta follows the task's retry rule; a Retry-After above the
// task's bound ends the call at once with rate_limited, not after a wait
// that turns it into cold_start_budget.
func TestMetaHonoursTaskRetryRule(t *testing.T) {
	for _, tc := range []struct {
		retryAfter string
		attempts   int32
	}{{"6", 1}, {"1", 2}} {
		var n atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n.Add(1)
			w.Header().Set("Retry-After", tc.retryAfter)
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		g, err := NewMetaGenerator(llm.Config{Mode: llm.ModeTestDouble, BaseURL: srv.URL, ServedModel: llm.TestDoubleServedModel,
			AttemptTimeout: 2 * time.Second}, nil)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		_, err = g.Generate(context.Background(), GenerateRequest{Task: TaskListeningReply, System: "s", User: "u",
			FirstContentWait: 20 * time.Second, MaxRetryAfter: 3 * time.Second, MaxRetries: 1})
		srv.Close()
		var ge *GenerateError
		if !errors.As(err, &ge) || ge.Reason != transport.ReasonRateLimited || ge.FallbackReason != FallbackUnavailable ||
			n.Load() != tc.attempts || time.Since(start) > 2500*time.Millisecond {
			t.Fatalf("Retry-After %s: %+v attempts %d after %s", tc.retryAfter, ge, n.Load(), time.Since(start))
		}
	}
	h := newHarness(t)
	if _, re := h.run(TaskListeningReply, inReply()); re != nil {
		t.Fatal(re)
	}
	if r := h.meta.req(0); r.MaxRetries != 1 || r.MaxRetryAfter != 3*time.Second || r.FirstContentWait != 20*time.Second || r.BeforeAttempt == nil {
		t.Fatalf("AT-1 Meta request %+v", r)
	}
}

// §8.3: the allowed bands are a set; enabling one minor band does not
// enable the other; DeepSeek never serves a minor band; production allows
// 18_plus only.
func TestAgeBandsArePerBand(t *testing.T) {
	cases := []struct {
		bands   string
		band    AgeBand
		allowed bool
	}{
		{"18_plus,unknown", Band15To17, false},
		{"18_plus,unknown", BandUnknown, true},
		{"18_plus,15_17", BandUnknown, false},
		{"18_plus,15_17", Band15To17, true},
		{"18_plus", Band15To17, false},
		{"18_plus", BandUnknown, false},
		{"15_17", Band18Plus, false},
		{"15_17,18_plus,unknown", Band15To17, true},
	}
	for _, tc := range cases {
		cfg, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "staging", "SELA_ALLOWED_AGE_BANDS": tc.bands}))
		if err != nil {
			t.Fatal(err)
		}
		h := newHarness(t)
		h.cfg = cfg
		h.c.band = tc.band
		_, re := h.run(TaskListeningReply, inReply())
		if got := h.meta.count() == 1; got != tc.allowed {
			t.Errorf("bands %s, user %s: Meta called %v (err %v)", tc.bands, tc.band, got, re)
		}
		if !tc.allowed {
			expect(t, re, OutcomeNoAI, ReasonAgeBand)
		}
		for _, role := range []ProviderName{ProviderMeta, ProviderAnthropic, ProviderDeepSeek} {
			want := tc.allowed && (tc.band.Adult() || role != ProviderDeepSeek)
			if got := providerAllowed(role, tc.band, cfg); got != want {
				t.Errorf("bands %s, %s, %s: %v want %v", tc.bands, tc.band, role, got, want)
			}
		}
	}
	if _, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "production", "SELA_ALLOWED_AGE_BANDS": "18_plus,unknown"})); err == nil {
		t.Fatal("production accepted a minor band")
	}
	prod := Config{Env: EnvProduction, AllowedBands: Bands(Band18Plus, Band15To17)}
	if providerAllowed(ProviderMeta, Band15To17, prod) || prod.validateCore() == nil {
		t.Fatal("production let a minor band through")
	}
}

// RR-15: the configured modes are authoritative, also for clients passed
// in Deps.
func TestDisabledModesDropInjectedClients(t *testing.T) {
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h.cfg.Anthropic.Mode, h.cfg.DeepSeek.Mode = ModeDisabled, ModeDisabled
	r, err := Build(h.cfg, llm.Config{}, Deps{Meta: h.meta, Consent: h.c, Claude: h.claude, DeepSeek: h.ds, AltVerifier: h.alt, Screener: screenFake{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Run(context.Background(), TaskSummaryDraft, in)
	var re *RunError
	if !errors.As(err, &re) {
		t.Fatalf("err %v", err)
	}
	expect(t, re, OutcomeNoAI, ReasonVerifierDisabled)
	if h.claude.count()+h.meta.count() != 0 {
		t.Fatalf("claude %d meta %d", h.claude.count(), h.meta.count())
	}

	h2 := newHarness(t)
	h2.meta.steps = []genStep{unavailableErr()}
	h2.cfg.DeepSeek.Mode = ModeDisabled
	h2.cfg.Anthropic.Mode = ModeDisabled
	r, err = Build(h2.cfg, llm.Config{}, Deps{Meta: h2.meta, Consent: h2.c, DeepSeek: h2.ds, Claude: h2.claude, Screener: screenFake{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Run(context.Background(), TaskListeningReply, inReply())
	if !errors.As(err, &re) || re.Outcome != OutcomeNoAI || h2.ds.count() != 0 {
		t.Fatalf("err %v deepseek %d", err, h2.ds.count())
	}
	expect(t, re, OutcomeNoAI, ReasonNotConfigured)

	// An empty mode is off too (fail closed).
	h3, in3 := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h3.cfg.Anthropic.Mode = ""
	_, re = h3.run(TaskSummaryDraft, in3)
	expect(t, re, OutcomeNoAI, ReasonVerifierDisabled)
	if h3.claude.count() != 0 {
		t.Fatal("Claude called with an empty mode")
	}
}

// RR-2, §8.2: consent is rechecked before every retry; a withdrawal
// during a Retry-After wait stops the retry for Claude, DeepSeek and Meta.
func TestConsentRecheckedBeforeEveryRetry(t *testing.T) {
	t.Run("claude", func(t *testing.T) {
		c := allConsent()
		fake, err := fakellm.NewHandler(fakellm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		var n, after atomic.Int32
		var withdrawn atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if withdrawn.Load() {
				after.Add(1)
			}
			if n.Add(1) == 1 {
				c.withdraw(PurposeAnthropic)
				withdrawn.Store(true)
				w.Header().Set(transport.HeaderTestDouble, "true")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`))
				return
			}
			fake.ServeHTTP(w, r)
		}))
		defer srv.Close()
		cl, err := anthropic.New(anthropic.Config{TestDouble: true, BaseURL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.meta.consent = nil
		r, err := NewRouter(h.cfg, Deps{Meta: h.meta, Claude: cl, Consent: c, Screener: screenFake{}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Run(context.Background(), TaskSummaryDraft, in)
		if OutcomeOf(err) != OutcomeStale || n.Load() != 1 || after.Load() != 0 {
			t.Fatalf("outcome %s requests %d after withdrawal %d", OutcomeOf(err), n.Load(), after.Load())
		}
	})
	t.Run("deepseek fallback", func(t *testing.T) {
		c := allConsent()
		fake, err := fakellm.NewHandler(fakellm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		var n atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n.Add(1) == 1 {
				c.withdraw(PurposeDeepSeek)
				w.Header().Set(transport.HeaderTestDouble, "true")
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			fake.ServeHTTP(w, r)
		}))
		defer srv.Close()
		ds, err := deepseek.New(deepseek.Config{TestDouble: true, BaseURL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		h := newHarness(t)
		h.meta.consent = nil
		h.meta.steps = []genStep{unavailableErr()}
		r, err := NewRouter(h.cfg, Deps{Meta: h.meta, DeepSeek: NewDeepSeekGenerator(ds), Consent: c, Screener: screenFake{}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Run(context.Background(), TaskListeningReply, inReply())
		if OutcomeOf(err) != OutcomeStale || n.Load() != 1 {
			t.Fatalf("outcome %s requests %d", OutcomeOf(err), n.Load())
		}
	})
	t.Run("meta", func(t *testing.T) {
		c := allConsent()
		var n atomic.Int32
		stub := &llmStub{mode: llm.ModeTestDouble, fn: func(ctx context.Context, _ llm.Request, onDelta func(string) error) (llm.Result, error) {
			if n.Add(1) == 1 {
				c.withdraw(PurposeAIProcessing)
				return llm.Result{Attempts: 1}, &llm.Error{Kind: llm.KindUnavailable, Status: 502}
			}
			_ = onDelta("x")
			return llm.Result{Text: reply("Terima kasih."), Model: llm.TestDoubleServedModel, Attempts: 1}, nil
		}}
		r, err := NewRouter(testConfig(), Deps{Meta: newMetaGenerator(stub, nil, time.Millisecond), Consent: c, Screener: screenFake{}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Run(context.Background(), TaskListeningReply, inReply())
		if OutcomeOf(err) != OutcomeStale || n.Load() != 1 {
			t.Fatalf("outcome %s Meta attempts %d", OutcomeOf(err), n.Load())
		}
	})
}

// RR-8, RR-11: every part of a flagged reply is checked; a letterless
// fragment is joined to a sentence, and a reply whose claims do not
// rebuild it is withheld.
func TestFlaggedReplyIsCheckedWhole(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{{text: reply("Kamu nggak sendirian. 80%.")}}
	out, re := h.run(TaskListeningReply, inReply("Aku capek banget sama skripsi."))
	if re != nil {
		t.Fatal(re)
	}
	var claims []string
	for _, q := range h.claude.reqs {
		for _, c := range q.Claims {
			claims = append(claims, c.Text)
		}
	}
	if len(claims) != 1 || claims[0] != "Kamu nggak sendirian. 80%." || !out.Items[0].Verified {
		t.Fatalf("claims %q", claims)
	}
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"80%. Kamu nggak sendirian.", []string{"80%. Kamu nggak sendirian."}},
		{"Satu. 2. Tiga!", []string{"Satu. 2.", "Tiga!"}},
		{"Halo… 1-2-3 ?", []string{"Halo… 1-2-3 ?"}},
		{"Kedengarannya berat.  Mau cerita lagi? Aku di sini…\nTerima kasih", []string{"Kedengarannya berat.", "Mau cerita lagi?", "Aku di sini…", "Terima kasih"}},
	} {
		got := sentences(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") || !covers(tc.in, got) {
			t.Errorf("sentences(%q) = %q", tc.in, got)
		}
	}
	// release refuses claims that do not rebuild the reply.
	h2 := newHarness(t)
	st := &run{r: h2.router(), task: TaskListeningReply, pol: policies[TaskListeningReply], in: inReply(), start: time.Now(),
		uses: map[Purpose]string{}, roles: map[ProviderName]bool{}}
	req := verifier.Request{Task: string(TaskListeningReply), Language: "id", Audience: "adult",
		Spans: []verifier.Span{{ID: "s1", Text: "Aku capek."}}, Claims: []verifier.Claim{{ID: "c1", Text: "Kamu capek.", SpanIDs: []string{"s1"}}}}
	draft := Draft{Items: []DraftItem{{Text: "Kamu capek. 80%.", SpanIDs: []string{"s1"}}}}
	dec := verifier.Decide(req, withVerdicts(req, nil), verifier.ReleaseOptions{WithholdOnAnyNotReleased: true})
	_, err := st.release(draft, []verifier.Request{req}, []claimRef{{req: 0, item: 0, claimID: "c1", spanIDs: []string{"s1"}}},
		[]verifier.Decision{dec})
	var rerr *RunError
	if !errors.As(err, &rerr) {
		t.Fatalf("err %v", err)
	}
	expect(t, rerr, OutcomeAbstain, ReasonClaimsCoverage)
}

// VC-6: generator output with duplicate or case-variant keys is invalid
// (the last duplicate would otherwise win).
func TestGeneratorDuplicateKeysAreInvalid(t *testing.T) {
	for _, text := range []string{
		`{"declined":true,"risk":"none","items":[],"Declined":false}`,
		`{"declined":false,"risk":"crisis","risk":"none","items":[{"text":"Halo.","span_ids":["s1"]}]}`,
		`{"declined":false,"risk":"none","items":[{"text":"Halo.","Text":"Kamu depresi.","span_ids":["s1"]}]}`,
	} {
		h := newHarness(t)
		h.meta.steps = []genStep{{text: text}}
		_, re := h.run(TaskReflection, inReply())
		expect(t, re, OutcomeAbstain, ReasonInvalidOutput)
	}
}

// §3.1: each task accepts only its own span roles; RoleSource only AT-6.
func TestSpanRolesPerTask(t *testing.T) {
	all := []Role{RoleUser, RoleEarlierReply, RoleMemory, RoleItem, RoleSource}
	allowed := map[Task][]Role{
		TaskListeningReply:      {RoleUser, RoleEarlierReply, RoleMemory},
		TaskReflection:          {RoleUser, RoleEarlierReply},
		TaskMemoryProposal:      {RoleUser, RoleMemory},
		TaskSummaryDraft:        {RoleItem},
		TaskOpeningLines:        {RoleItem},
		TaskReadingVerification: {RoleSource},
	}
	for task, roles := range allowed {
		ok := map[Role]bool{}
		for _, r := range roles {
			ok[r] = true
		}
		pol, _ := PolicyFor(task)
		for _, role := range all {
			if pol.allowsRole(role) != ok[role] {
				t.Errorf("%s accepts %s: %v", task, role, pol.allowsRole(role))
			}
			if ok[role] {
				continue
			}
			h := newHarness(t)
			in := Input{Subject: subject, Language: "id", Purpose: "first_consultation",
				Spans:  []Span{{Key: "k", Role: role, Text: "Aku mau bunuh diri malam ini."}},
				Claims: []ClaimInput{{Key: "c", Text: "Klaim.", SpanKeys: []string{"k"}}}}
			_, re := h.run(task, in)
			expect(t, re, OutcomeInvalidInput, ReasonSpanRole)
			noCalls(t, h)
		}
	}
	// AT-6 never sends user text to Claude, with or without consent.
	h := newHarness(t)
	h.c = newConsent()
	h.c.setBand(Band15To17)
	h.claude = newClaude(t, nil)
	in := Input{Subject: subject, Language: "id", Spans: []Span{{Key: "journal-42", Role: RoleUser, Text: "Aku sering nangis tiap malam."}},
		Claims: []ClaimInput{{Key: "c", Text: "Pengguna sering menangis.", SpanKeys: []string{"journal-42"}}}}
	_, re := h.run(TaskReadingVerification, in)
	expect(t, re, OutcomeInvalidInput, ReasonSpanRole)
	if h.claude.count() != 0 {
		t.Fatal("user text reached Claude through AT-6")
	}
	// A crisis sentence labelled as an earlier reply or source never
	// reaches Meta in AT-4.
	for _, role := range []Role{RoleSource, RoleEarlierReply} {
		h := newHarness(t)
		r, err := NewRouter(h.cfg, Deps{Meta: h.meta, Consent: h.c, Claude: h.claude})
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Run(context.Background(), TaskSummaryDraft, Input{Subject: subject, Language: "id", Purpose: "first_consultation",
			Spans: []Span{{Key: "k", Role: role, Text: "Aku mau bunuh diri malam ini."}}})
		if OutcomeOf(err) != OutcomeInvalidInput || h.meta.count() != 0 {
			t.Fatalf("%s: outcome %s meta %d", role, OutcomeOf(err), h.meta.count())
		}
	}
}

// DM-2 end to end: grouped NIK, addresses, self-introduced names, cue
// handles and normalised spellings never reach a provider.
func TestRedactionReachesEveryProvider(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"[NAMA-1] mencatat [KONTAK-1].", "s1"})}}
	in := inItems("Namaku Raka Putra, NIK 3273 2504 9001 0003, alamat Jl. Melati No. 12, RT 03/RW 05, ig raka.putra, WA 0812–3456–7890, teman Ｄｅｗｉ.")
	in.PrivateNames = []string{"Dewi"}
	if _, re := h.run(TaskSummaryDraft, in); re != nil {
		t.Fatal(re)
	}
	raw, _ := json.Marshal(h.claude.reqs)
	sent := h.meta.req(0).User + string(raw)
	for _, forbidden := range []string{"Raka", "3273", "2504", "Melati", "RT 03", "raka.putra", "0812", "3456", "Ｄｅｗｉ", "Dewi"} {
		if strings.Contains(sent, forbidden) {
			t.Fatalf("payload contains %q", forbidden)
		}
	}
}

// Secrets never print: config types, the router, the clients and Deps,
// for every fmt verb and JSON, also where fmt cannot call the types'
// methods: an application struct holding Config in an unexported field,
// and a Router printed by value (its Format has a pointer receiver).
func TestSecretsNeverPrint(t *testing.T) {
	const ak, dk, ws, tk = "sk-ant-SECRET-CANARY", "sk-ds-SECRET-CANARY", "wrkspc_SECRET_CANARY", "td-SECRET-CANARY"
	sec := transport.NewSecret
	cfg := Config{Env: EnvDevelopment, Profile: ProfileFull, TestDoubleAPIKey: sec(tk),
		Anthropic: AnthropicConfig{Mode: ModeLive, APIKey: sec(ak), WorkspaceID: sec(ws)}, DeepSeek: DeepSeekConfig{Mode: ModeLive, APIKey: sec(dk)}}
	r, err := Build(cfg, llm.Config{Mode: llm.ModeModal, BaseURL: "https://sela--glimmer.modal.run", APIKey: "vllm-SECRET-CANARY",
		ModalKey: "mk-SECRET-CANARY", ModalSecret: "ms-SECRET-CANARY", ServedModel: "sela-glimmer-cec94fc",
		ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc"}, Deps{Consent: allConsent()})
	if err != nil {
		t.Fatal(err)
	}
	deps := Deps{Meta: r.meta, DeepSeek: r.ds, Claude: r.claude, AltVerifier: r.alt, Consent: allConsent()}
	type app struct {
		name string
		ai   Config
	}
	type routerHolder struct{ R Router }
	type embedded struct {
		Router
		n int
	}
	raw, _ := json.Marshal(cfg)
	out := string(raw)
	for _, f := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%d"} {
		for _, v := range []any{cfg, &cfg, cfg.Anthropic, cfg.DeepSeek, r, r.claude, r.ds, r.alt, r.meta, deps, &deps,
			app{name: "sela", ai: cfg}, &app{name: "sela", ai: cfg}, routerHolder{R: *r}, &routerHolder{R: *r}, embedded{Router: *r},
			[]any{*r}, map[string]any{"r": *r}} {
			out += fmt.Sprintf(f, v)
		}
	}
	for _, canary := range []string{ak, dk, ws, tk, "vllm-SECRET-CANARY", "mk-SECRET-CANARY", "ms-SECRET-CANARY",
		fmt.Sprintf("%x", ak), fmt.Sprintf("%x", dk)} {
		if strings.Contains(out, canary) {
			t.Fatalf("printed %q", canary)
		}
	}
	if !strings.Contains(fmt.Sprint(r), "key=set") {
		t.Fatalf("router summary %v", r)
	}
}

// A test double never runs at a live provider host, never receives a live
// key, and is labelled with its own model name.
func TestTestDoubleNeverLive(t *testing.T) {
	for _, vars := range []map[string]string{
		{"SELA_ENV": "staging", "SELA_ALLOW_TEST_DOUBLE": "true", "SELA_VERIFIER_MODE": "test_double",
			"SELA_ANTHROPIC_API_KEY": "sk-ant-api03-LIVE", "SELA_ANTHROPIC_BASE_URL": "https://api.anthropic.com"},
		{"SELA_ENV": "staging", "SELA_ALLOW_TEST_DOUBLE": "true", "SELA_DEEPSEEK_MODE": "test_double",
			"SELA_DEEPSEEK_API_KEY": "sk-LIVE", "SELA_DEEPSEEK_BASE_URL": "https://api.deepseek.com"},
		{"SELA_ENV": "staging", "SELA_ALLOW_TEST_DOUBLE": "true", "SELA_DEEPSEEK_MODE": "test_double",
			"SELA_DEEPSEEK_BASE_URL": "https://deepseek-proxy.example-corp.com"},
	} {
		if _, err := LoadFromEnv(env(vars)); err == nil {
			t.Errorf("accepted %v", vars)
		}
	}
	c, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "development", "SELA_ALLOW_TEST_DOUBLE": "true",
		"SELA_VERIFIER_MODE": "test_double", "SELA_DEEPSEEK_MODE": "test_double", "SELA_TEST_DOUBLE_API_KEY": "td-key"}))
	if err != nil {
		t.Fatal(err)
	}
	s := c.String()
	if !strings.Contains(s, "model="+anthropic.TestDoubleModel) || !strings.Contains(s, "model="+deepseek.TestDoubleModel) ||
		strings.Contains(s, "model=claude-sonnet-5-5") || c.anthropicConfig().TestDoubleAPIKey.Reveal() != "td-key" {
		t.Fatalf("String = %s", s)
	}
}

// §8: Claude gets personal data only where the consent says (the US);
// production refuses another geo; AT-6 may use global.
func TestClaudeInferenceGeo(t *testing.T) {
	if _, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "production", "SELA_ANTHROPIC_API_KEY": "sk-ant-api03-LIVE",
		"SELA_VERIFIER_INFERENCE_GEO": "global"})); err == nil {
		t.Fatal("production accepted inference_geo global")
	}
	if _, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "production", "SELA_ANTHROPIC_API_KEY": "sk-ant-api03-LIVE",
		"SELA_VERIFIER_INFERENCE_GEO": "us"})); err != nil {
		t.Fatal(err)
	}
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h.claude.id.InferenceGeo = "global"
	_, re := h.run(TaskSummaryDraft, in)
	expect(t, re, OutcomeNoAI, ReasonInferenceGeo)
	noCalls(t, h)
	h2 := newHarness(t)
	h2.claude.consent = nil
	h2.claude.id.InferenceGeo = "global"
	_, re = h2.run(TaskReadingVerification, Input{Language: "id", Spans: []Span{{Key: "R-1#p1", Role: RoleSource, Text: "Menulis membantu."}},
		Claims: []ClaimInput{{Key: "d", Text: "Menulis membantu.", SpanKeys: []string{"R-1#p1"}}}})
	if re != nil || h2.claude.count() != 1 {
		t.Fatalf("AT-6 with global: %v", re)
	}
}

// Validate and NewRouter refuse an unknown environment and treat it as
// production; test doubles in Deps need AllowTestDouble.
func TestEnvironmentFailsClosed(t *testing.T) {
	for _, e := range []string{"", "Production", "prod", "dev"} {
		cfg := testConfig()
		cfg.Env = e
		meta := newGen(t, ProviderMeta, nil)
		meta.id.TestDouble = true
		if !cfg.Production() || cfg.Validate() == nil {
			t.Errorf("Env %q: Production %v", e, cfg.Production())
		}
		if _, err := NewRouter(cfg, Deps{Meta: meta, Consent: allConsent()}); err == nil {
			t.Errorf("Env %q accepted by NewRouter", e)
		}
	}
	cfg := testConfig()
	cfg.AllowTestDouble = false
	meta := newGen(t, ProviderMeta, nil)
	meta.id.TestDouble = true
	if _, err := NewRouter(cfg, Deps{Meta: meta, Consent: allConsent()}); err == nil {
		t.Fatal("test double accepted without AllowTestDouble")
	}
	claude := newClaude(t, nil)
	claude.id.TestDouble = true
	if _, err := NewRouter(cfg, Deps{Meta: newGen(t, ProviderMeta, nil), Claude: claude, Consent: allConsent()}); err == nil {
		t.Fatal("test-double verifier accepted without AllowTestDouble")
	}
}

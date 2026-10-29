package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm"
)

// Tests of the owner's fallback contract (docs/DEEPSEEK_FALLBACK.md, SPEC
// C13, V19-V21) as ADR-0003 §17 O-1..O-12, RR-2, RR-4 (i), RR-8 and RR-15
// apply it to the router.

func hasReason(re *RunError, reason string) bool {
	if re == nil {
		return false
	}
	for _, r := range re.Reasons {
		if r == reason {
			return true
		}
	}
	return false
}

// O-4, V20, §10: a DeepSeek fallback draft is live_deepseek_fallback; the
// alternative verifier keeps its own value live_deepseek_alternative.
func TestOwnerProvenanceValues(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{unavailableErr()}
	out, re := h.run(TaskReflection, inReply())
	if re != nil {
		t.Fatal(re)
	}
	p := out.Provenance
	if p.DraftProvenance != "live_deepseek_fallback" || p.Route != RouteFallback || p.FallbackReason != "unavailable" {
		t.Fatalf("fallback provenance %+v", p)
	}
	raw, _ := json.Marshal(p)
	if !strings.Contains(string(raw), `"draftProvenance":"live_deepseek_fallback"`) || strings.Contains(string(raw), `"live_deepseek"`) {
		t.Fatalf("provenance JSON %s", raw)
	}

	h2, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h2.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
	out2, re2 := h2.run(TaskSummaryDraft, in)
	if re2 != nil {
		t.Fatal(re2)
	}
	if v := out2.Provenance.Verification; v.Provenance != "live_deepseek_alternative" || out2.Provenance.DraftProvenance != DraftLiveMeta ||
		v.Route != AlternativeVerifierRoute {
		t.Fatalf("alternative verifier provenance %+v / %s", v, out2.Provenance.DraftProvenance)
	}
	if DraftLiveDeepSeekFallback == VerifiedLiveDeepSeekAlt {
		t.Fatal("the fallback draft and the alternative verdict share a label")
	}

	// A run that ends on the no-AI path carries the owner's "unavailable"
	// (AI drafting unavailable), never an off-contract value.
	h3 := newHarness(t)
	h3.cfg.Fallback = false
	h3.meta.steps = []genStep{unavailableErr()}
	_, re3 := h3.run(TaskReflection, inReply())
	expect(t, re3, OutcomeNoAI, "primary_unavailable")
	if re3.Provenance.DraftProvenance != "unavailable" || DraftUnavailable != "unavailable" {
		t.Fatalf("no-AI draft provenance %q", re3.Provenance.DraftProvenance)
	}
	raw3, _ := json.Marshal(re3.Provenance)
	if !strings.Contains(string(raw3), `"draftProvenance":"unavailable"`) {
		t.Fatalf("no-AI provenance JSON %s", raw3)
	}
	// Every draft value is the owner's (docs/DEEPSEEK_FALLBACK.md) or the
	// labelled test double of ADR-0003 §10.1.
	owner := map[string]bool{"live_meta": true, "live_deepseek_fallback": true, "manual": true, "recorded_demo": true,
		"unavailable": true, "test_double": true}
	for _, v := range []string{DraftLiveMeta, DraftLiveDeepSeekFallback, DraftTestDouble, DraftManual, DraftRecordedDemo,
		DraftUnavailable, draftProvenance(GeneratorIdentity{})} {
		if !owner[v] {
			t.Errorf("draft provenance %q is not an owner value", v)
		}
	}
}

// §10 fallback_reason: a Meta 429 after the retries is rate_limited, as
// the real Meta generator classifies it.
func TestOwnerFallbackReasonRateLimited(t *testing.T) {
	h := newHarness(t)
	meta := newMetaGenerator(&llmStub{mode: llm.ModeModal, fn: failWith(llm.KindRateLimited)}, nil, 10*time.Millisecond)
	r, err := NewRouter(h.cfg, Deps{Meta: meta, DeepSeek: h.ds, Claude: h.claude, Consent: h.c, Screener: screenFake{}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Run(context.Background(), TaskReflection, inReply())
	if err != nil || out.Provenance.FallbackReason != FallbackRateLimited || out.Provenance.Route != RouteFallback {
		t.Fatalf("err %v provenance %+v", err, out.Provenance)
	}
}

// O-1, V19: only upstream Meta availability errors trigger the fallback.
// Sela's admission queue, a credential problem on Sela's side, a Sela
// spend ceiling or billing error and a certificate failure end in the
// no-AI path with an operator alert; none is a fallback_reason.
func TestOwnerNoFallbackForSelaSideFailures(t *testing.T) {
	for _, tc := range []struct {
		cause  string
		reason transport.Reason
	}{{CauseCapacity, transport.ReasonUnavailable}, {CauseAuth, transport.ReasonAuth}, {CauseSpendCap, transport.ReasonSpendCap},
		{CauseTLS, transport.ReasonUnavailable}} {
		for _, task := range []Task{TaskListeningReply, TaskReflection, TaskOpeningLines} {
			h := newHarness(t)
			h.meta.steps = []genStep{{err: &GenerateError{Class: transport.ClassAvailability, Reason: tc.reason, Cause: tc.cause}}}
			in := inReply()
			if task == TaskOpeningLines {
				in = inItems("Aku susah tidur sejak dua bulan.")
			}
			_, re := h.run(task, in)
			expect(t, re, OutcomeNoAI, ReasonFallbackNotEligible)
			if !hasReason(re, "primary_"+tc.cause) || h.ds.count() != 0 || re.Provenance.FallbackReason != "" ||
				!h.ev.has(StageOperatorAlert) {
				t.Fatalf("%s %s: reasons %v deepseek %d", tc.cause, task, re.Reasons, h.ds.count())
			}
		}
	}
	// The real Meta generator: 401/403 is a credential problem on Sela's side.
	h := newHarness(t)
	meta := newMetaGenerator(&llmStub{mode: llm.ModeModal, fn: failWith(llm.KindAuth)}, nil, 10*time.Millisecond)
	r, err := NewRouter(h.cfg, Deps{Meta: meta, DeepSeek: h.ds, Claude: h.claude, Consent: h.c, Screener: screenFake{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Run(context.Background(), TaskReflection, inReply())
	var re *RunError
	if !errors.As(err, &re) || !hasReason(re, ReasonFallbackNotEligible) || h.ds.count() != 0 {
		t.Fatalf("auth: %v deepseek %d", err, h.ds.count())
	}
}

// C13, V19, RR-15, O-2: the fallback is off by default in every
// environment and profile, the hackathon (demo) and meta_only paths
// included, whatever DeepSeek keys are set; only an explicit server
// setting turns it on.
func TestOwnerFallbackOffByDefault(t *testing.T) {
	for _, e := range []string{EnvDevelopment, EnvTest, EnvDemo, EnvStaging, EnvProduction, ""} {
		for _, profile := range []string{"", "full", "meta_only"} {
			vars := map[string]string{"SELA_ENV": e, "SELA_AI_PROFILE": profile, "SELA_DEEPSEEK_API_KEY": deepseekKey,
				"SELA_ANTHROPIC_API_KEY": anthropicKey}
			c, err := LoadFromEnv(env(vars))
			if err != nil {
				t.Fatalf("%q/%q: %v", e, profile, err)
			}
			if c.Fallback || c.AltVerifier || c.deepseekData() != DeepSeekSyntheticOnly {
				t.Fatalf("%q/%q: fallback %v alt %v data %s by default", e, profile, c.Fallback, c.AltVerifier, c.deepseekData())
			}
		}
	}
	for _, v := range []string{"on", "deepseek", "true"} {
		c, err := LoadFromEnv(env(map[string]string{"SELA_ENV": EnvDemo, "SELA_AI_FALLBACK": v}))
		if err != nil || !c.Fallback {
			t.Fatalf("explicit %q: %v %v", v, c.Fallback, err)
		}
	}
	if (Config{}).Fallback {
		t.Fatal("zero config enables the fallback")
	}
	// The router: with the default (off), a Meta outage never reaches
	// DeepSeek, for any generative task; turned on, DeepSeek still needs
	// the user's DeepSeek consent.
	for _, task := range []Task{TaskListeningReply, TaskReflection, TaskMemoryProposal, TaskSummaryDraft, TaskOpeningLines} {
		for _, on := range []bool{false, true} {
			h := newHarness(t)
			h.cfg.Fallback = on
			if on {
				h.c.granted[PurposeDeepSeek] = false
			}
			h.meta.steps = []genStep{unavailableErr()}
			in := inReply("Aku kerja shift malam.")
			if task == TaskSummaryDraft || task == TaskOpeningLines {
				in = inItems("Aku susah tidur sejak dua bulan.")
			}
			_, re := h.run(task, in)
			want := ReasonFlagOff
			if on {
				want = ReasonNoConsent
			}
			expect(t, re, OutcomeNoAI, want)
			if h.ds.count() != 0 || h.meta.count() != 1 {
				t.Fatalf("%s on=%v: meta %d deepseek %d", task, on, h.meta.count(), h.ds.count())
			}
		}
	}
}

// V19: invalid or unsafe Meta output, a refusal, a raised risk flag, a
// crisis, a cancellation, missing consent, a policy refusal or block, an
// integrity failure and Sela's own limit never trigger the fallback.
func TestOwnerNeverFallsBack(t *testing.T) {
	cases := map[string]struct {
		setup func(h *harness, in *Input, cancel func())
		task  Task
		want  Outcome
	}{
		"invalid Meta output": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{text: "not json"}}
		}, TaskReflection, OutcomeAbstain},
		"empty Meta output": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{text: genJSON("none", false)}}
		}, TaskReflection, OutcomeAbstain},
		"unsafe Meta output (guard)": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{text: reply("Cuma aku yang ngerti kamu.")}}
		}, TaskListeningReply, OutcomeAbstain},
		"Meaning Lock rejection": {func(h *harness, in *Input, _ func()) {
			in.Validate = func(Task, []DraftItem, []minimize.Span) string { return "negation_flip" }
		}, TaskReflection, OutcomeAbstain},
		"refusal": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{text: genJSON("none", true)}}
		}, TaskListeningReply, OutcomeSafeStop},
		"risk crisis": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{text: genJSON("crisis", false, [2]string{"Terima kasih.", "s1"})}}
		}, TaskListeningReply, OutcomeSafeStop},
		"crisis screen": {func(h *harness, in *Input, _ func()) {
			in.Spans[0].Text = "CRISIS-WORDS"
		}, TaskListeningReply, OutcomeCrisisHold},
		"crisis paused": {func(h *harness, in *Input, _ func()) { in.CrisisPaused = true }, TaskListeningReply, OutcomeCrisisHold},
		"user cancels during a Meta outage": {func(h *harness, _ *Input, cancel func()) {
			h.meta.steps = []genStep{{hook: cancel, err: unavailableErr().err}}
		}, TaskListeningReply, OutcomeCanceled},
		"user cancels the Meta call": {func(h *harness, _ *Input, cancel func()) {
			h.meta.steps = []genStep{{hook: cancel, err: &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonCanceled}}}
		}, TaskListeningReply, OutcomeCanceled},
		"no ai_processing consent": {func(h *harness, _ *Input, _ func()) {
			h.c.granted[PurposeAIProcessing] = false
			h.meta.steps = []genStep{unavailableErr()}
		}, TaskListeningReply, OutcomeNoAI},
		"no DeepSeek consent": {func(h *harness, _ *Input, _ func()) {
			h.c.granted[PurposeDeepSeek] = false
			h.meta.steps = []genStep{unavailableErr()}
		}, TaskListeningReply, OutcomeNoAI},
		"policy refusal (4xx)": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{err: &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonRejected}}}
		}, TaskReflection, OutcomeNoAI},
		"failure after content": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{err: &GenerateError{Class: transport.ClassContent, Reason: transport.ReasonInvalidOutput, ContentStarted: true}}}
		}, TaskReflection, OutcomeAbstain},
		"model mismatch": {func(h *harness, _ *Input, _ func()) {
			h.meta.steps = []genStep{{err: &GenerateError{Class: transport.ClassIntegrity, Reason: transport.ReasonModelMismatch}}}
		}, TaskReflection, OutcomeAbstain},
		"kill switch": {func(h *harness, _ *Input, _ func()) { h.meta.id.Disabled = true }, TaskReflection, OutcomeNoAI},
		"Sela's per-user limit": {func(h *harness, _ *Input, _ func()) {
			h.lim = limiter{provider: ProviderMeta, err: ErrUserLimit}
		}, TaskReflection, OutcomeNoAI},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			in := inReply()
			tc.setup(h, &in, cancel)
			_, err := h.router().Run(ctx, tc.task, in)
			if OutcomeOf(err) != tc.want || h.ds.count() != 0 {
				t.Fatalf("outcome %s (%v), want %s; DeepSeek calls %d", OutcomeOf(err), err, tc.want, h.ds.count())
			}
		})
	}
}

// V21, O-3: until the operator enables real-user processing, DeepSeek
// receives only operations marked synthetic, never in production; the
// setting is recorded in provenance.
func TestOwnerDeepSeekSyntheticOnly(t *testing.T) {
	real := func() Input { in := inReply(); in.Synthetic = false; return in }
	t.Run("real-user text is not sent by default", func(t *testing.T) {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		_, re := h.run(TaskReflection, real())
		expect(t, re, OutcomeNoAI, ReasonSyntheticOnly)
		if h.ds.count() != 0 || re.Provenance.DeepSeekData != DeepSeekSyntheticOnly || re.Provenance.Synthetic {
			t.Fatalf("deepseek %d provenance %+v", h.ds.count(), re.Provenance)
		}
	})
	t.Run("synthetic data is", func(t *testing.T) {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		out, re := h.run(TaskReflection, inReply())
		if re != nil || h.ds.count() != 1 || out.Provenance.DeepSeekData != DeepSeekSyntheticOnly || !out.Provenance.Synthetic {
			t.Fatalf("err %v deepseek %d provenance %+v", re, h.ds.count(), out.Provenance)
		}
	})
	t.Run("real_user set by the operator", func(t *testing.T) {
		h := newHarness(t)
		h.cfg.DeepSeekData = DeepSeekRealUser
		h.meta.steps = []genStep{unavailableErr()}
		out, re := h.run(TaskReflection, real())
		if re != nil || h.ds.count() != 1 || out.Provenance.DeepSeekData != DeepSeekRealUser {
			t.Fatalf("err %v deepseek %d provenance %+v", re, h.ds.count(), out.Provenance)
		}
	})
	t.Run("production has no demo data", func(t *testing.T) {
		h := newHarness(t)
		h.cfg.Env, h.cfg.AllowTestDouble, h.cfg.AllowedBands = EnvProduction, false, Bands(Band18Plus)
		h.meta.steps = []genStep{unavailableErr()}
		_, re := h.run(TaskReflection, inReply())
		expect(t, re, OutcomeNoAI, ReasonSyntheticOnly)
		if h.ds.count() != 0 {
			t.Fatal("DeepSeek called in production on the synthetic-only setting")
		}
	})
	t.Run("production refuses real_user unless cleared", func(t *testing.T) {
		cfg := testConfig()
		cfg.Env, cfg.AllowTestDouble, cfg.AllowedBands, cfg.DeepSeekData = EnvProduction, false, Bands(Band18Plus), DeepSeekRealUser
		h := newHarness(t)
		if _, err := NewRouter(cfg, Deps{Meta: h.meta, Consent: h.c}); err == nil || !strings.Contains(err.Error(), "SELA_DEEPSEEK_REAL_USER_B10") {
			t.Fatalf("NewRouter: %v", err)
		}
		cfg.DeepSeekRealUserCleared = true
		if _, err := NewRouter(cfg, Deps{Meta: h.meta, Consent: h.c}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("the alternative verifier too", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		in.Synthetic = false
		h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, ReasonSyntheticOnly)
		if h.alt.count() != 0 || h.claude.count() != 1 {
			t.Fatalf("alternative %d claude %d", h.alt.count(), h.claude.count())
		}
	})
	t.Run("configuration", func(t *testing.T) {
		for vars, ok := range map[[3]string]bool{
			{EnvProduction, "real_user", ""}:        false,
			{EnvProduction, "real_user", "cleared"}: true,
			{EnvProduction, "synthetic_only", ""}:   true,
			{EnvStaging, "real_user", ""}:           true,
			{EnvDemo, "everything", ""}:             false,
			{EnvDemo, "", "yes"}:                    false,
		} {
			c, err := LoadFromEnv(env(map[string]string{"SELA_ENV": vars[0], "SELA_DEEPSEEK_DATA": vars[1], "SELA_DEEPSEEK_REAL_USER_B10": vars[2]}))
			if (err == nil) != ok {
				t.Fatalf("%v: %v", vars, err)
			}
			if ok && !strings.Contains(c.String(), "data="+string(c.deepseekData())) {
				t.Fatalf("String = %s", c.String())
			}
		}
	})
}

// RR-8, §17 O-6: every DeepSeek AT-1 reply is verified by Claude, also a
// reply the health-claim detector does not flag; a Claude failure after
// the DeepSeek call withholds it, and DeepSeek never verifies itself.
func TestOwnerClaudeVerifiesEveryDeepSeekReply(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{unavailableErr()}
	out, re := h.run(TaskListeningReply, inReply())
	if re != nil {
		t.Fatal(re)
	}
	v := out.Provenance.Verification
	if h.claude.count() != 1 || v.Status != StatusChecked || v.Provenance != VerifiedLiveAnthropic || v.Route != VerifierRoute ||
		!out.Items[0].Verified || out.Items[0].CheckLabel != "Claude" || out.Provenance.DraftProvenance != DraftLiveDeepSeekFallback {
		t.Fatalf("claude %d verification %+v items %+v", h.claude.count(), v, out.Items)
	}
	// AT-1 keeps the 15 s verifier attempt beside the 20 s DeepSeek
	// attempt (§3.2): Meta's window is 20 s, DeepSeek's bound at most 20 s.
	if w := h.meta.req(0).FirstContentWait; w != 20*time.Second {
		t.Fatalf("Meta window %s", w)
	}
	if d := h.ds.req(0).AttemptTimeout; d <= 0 || d > 20*time.Second {
		t.Fatalf("DeepSeek bound %s", d)
	}
	// A Meta reply that is not flagged needs no check.
	h2 := newHarness(t)
	if out, re := h2.run(TaskListeningReply, inReply()); re != nil || h2.claude.count() != 0 || out.Provenance.Verification.Status != StatusNotRequired {
		t.Fatalf("Meta reply: %v claude %d", re, h2.claude.count())
	}
	for name, step := range map[string]verStep{
		"Claude unavailable": {err: verr(transport.ReasonUnavailable)},
		"Claude refuses":     {err: verr(transport.ReasonRefused)},
		"Claude unsupported": {report: func(req verifier.Request) verifier.Report {
			return withVerdicts(req, map[int]verifier.Verdict{0: verifier.Unsupported})
		}},
	} {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		h.claude.steps = []verStep{step}
		_, re := h.run(TaskListeningReply, inReply())
		expect(t, re, OutcomeAbstain, "")
		if h.ds.count() != 1 || h.alt.count() != 0 {
			t.Fatalf("%s: deepseek %d alternative %d", name, h.ds.count(), h.alt.count())
		}
	}
}

// RR-4 (i), RR-2, §12 routing.deepseek_listening_calls_without_claude_route
// = 0: when the Claude route is not permitted at preflight (consent, age
// band, configuration, per-user verifier limit, Sela's verifier daily
// spend ceiling), a Meta outage on a listening reply makes zero DeepSeek
// calls and ends in the no-AI path.
func TestOwnerNoDeepSeekReplyWithoutClaudeRoute(t *testing.T) {
	cases := map[string]struct {
		setup  func(h *harness)
		reason string
	}{
		"no Anthropic consent":     {func(h *harness) { h.c.granted[PurposeAnthropic] = false }, ReasonNoConsent},
		"verifier disabled":        {func(h *harness) { h.cfg.Anthropic.Mode = ModeDisabled }, ReasonVerifierDisabled},
		"verifier not configured":  {func(h *harness) { h.noCl = true }, ReasonNotConfigured},
		"inference geo not us":     {func(h *harness) { h.cfg.Anthropic.InferenceGeo = "global" }, ReasonInferenceGeo},
		"per-user verifier limit":  {func(h *harness) { h.lim = limiter{provider: ProviderAnthropic, err: ErrUserLimit} }, ReasonUserLimit},
		"Limiter verifier ceiling": {func(h *harness) { h.lim = limiter{provider: ProviderAnthropic, err: ErrSpendCap} }, ReasonSpendCap},
		"meta_only profile":        {func(h *harness) { h.cfg.Profile = ProfileMetaOnly }, ReasonProfileMetaOnly},
		"minor band":               {func(h *harness) { h.c.band = Band15To17 }, ReasonAgeBand},
	}
	calls := 0
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.meta.steps = []genStep{unavailableErr()}
			tc.setup(h)
			_, re := h.run(TaskListeningReply, inReply())
			expect(t, re, OutcomeNoAI, tc.reason)
			calls += h.ds.count()
			if h.ds.count() != 0 || h.claude.count() != 0 || h.meta.count() != 1 {
				t.Fatalf("meta %d deepseek %d claude %d", h.meta.count(), h.ds.count(), h.claude.count())
			}
		})
	}
	t.Run("SELA_VERIFIER_DAILY_USD reached", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.cfg.VerifierDailyUSD = 0.001 // one fake Claude call costs about $0.0013
		r := h.router()
		if _, err := r.Run(context.Background(), TaskSummaryDraft, in); err != nil {
			t.Fatal(err)
		}
		h.meta.steps = []genStep{unavailableErr()}
		_, err := r.Run(context.Background(), TaskListeningReply, inReply())
		var re *RunError
		if !errors.As(err, &re) || re.Outcome != OutcomeNoAI || !hasReason(re, ReasonFallbackNeedsVerifier) || !hasReason(re, ReasonSpendCap) {
			t.Fatalf("err %v", err)
		}
		calls += h.ds.count()
		if h.ds.count() != 0 || h.claude.count() != 1 {
			t.Fatalf("deepseek %d claude %d", h.ds.count(), h.claude.count())
		}
	})
	if calls != 0 {
		t.Fatalf("routing.deepseek_listening_calls_without_claude_route = %d", calls)
	}
	// Limiter.Check asks without counting: Allow is asked once per call.
	h := newHarness(t)
	lim := &countLimiter{n: map[ProviderName]int{}}
	h.lim = lim
	h.meta.steps = []genStep{unavailableErr()}
	if _, re := h.run(TaskListeningReply, inReply()); re != nil {
		t.Fatal(re)
	}
	if lim.get(ProviderAnthropic) != h.claude.count() || lim.get(ProviderDeepSeek) != h.ds.count() {
		t.Fatalf("Allow anthropic %d (calls %d) deepseek %d (calls %d)", lim.get(ProviderAnthropic), h.claude.count(),
			lim.get(ProviderDeepSeek), h.ds.count())
	}
}

// §11 SELA_VERIFIER_DAILY_USD: once the router's estimated Claude spend
// for the UTC day reaches the ceiling, Claude is not called (spend_cap,
// an availability failure: the alternative verifier may check Meta
// output, RR-9); the next day it is called again.
func TestOwnerVerifierDailySpendCeiling(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 10, 26, 23, 0, 0, 0, time.UTC)
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h.cfg.VerifierDailyUSD = 0.001
	r, err := NewRouter(h.cfg, Deps{Meta: h.meta, DeepSeek: h.ds, Claude: h.claude, AltVerifier: h.alt, Consent: h.c,
		Screener: screenFake{}, Now: clock})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(context.Background(), TaskSummaryDraft, in); err != nil || h.claude.count() != 1 {
		t.Fatalf("first: %v claude %d", err, h.claude.count())
	}
	out, err := r.Run(context.Background(), TaskSummaryDraft, in)
	v := out.Provenance.Verification
	if err != nil || h.claude.count() != 1 || h.alt.count() != 1 || v.Route != AlternativeVerifierRoute || v.UnavailableReasons[0] != ReasonSpendCap {
		t.Fatalf("second: %v claude %d alternative %d verification %+v", err, h.claude.count(), h.alt.count(), v)
	}
	mu.Lock()
	now = now.Add(2 * time.Hour) // the next UTC day
	mu.Unlock()
	if _, err := r.Run(context.Background(), TaskSummaryDraft, in); err != nil || h.claude.count() != 2 {
		t.Fatalf("next day: %v claude %d", err, h.claude.count())
	}
	for v, ok := range map[string]bool{"5": true, "2.50": true, "0": false, "-1": false, "abc": false, "1e9": false} {
		c, err := LoadFromEnv(env(map[string]string{"SELA_VERIFIER_DAILY_USD": v}))
		if (err == nil) != ok || (ok && c.VerifierDailyUSD <= 0) {
			t.Fatalf("SELA_VERIFIER_DAILY_USD=%s: %v %v", v, c.VerifierDailyUSD, err)
		}
	}
}

// V21: after every fallback request the session, record and fixture
// versions and both provider consents are rechecked before anything is
// kept; a stale result is discarded. Without a version re-check the
// fallback is not attempted.
func TestOwnerV21RecheckBeforePersisting(t *testing.T) {
	t.Run("version changed during the DeepSeek call", func(t *testing.T) {
		h := newHarness(t)
		var changed atomic.Bool
		h.meta.steps = []genStep{unavailableErr()}
		h.ds.steps = []genStep{{text: reply("Terima kasih sudah cerita."), hook: func() { changed.Store(true) }}}
		in := inReply()
		in.Commit = func(context.Context) error {
			if changed.Load() {
				return errors.New("brief version changed")
			}
			return nil
		}
		_, re := h.run(TaskListeningReply, in)
		expect(t, re, OutcomeStale, ReasonStale)
		if h.ds.count() != 1 || h.claude.count() != 0 {
			t.Fatalf("deepseek %d claude %d: a stale fallback result was used", h.ds.count(), h.claude.count())
		}
	})
	t.Run("version rechecked after the fallback and again at commit", func(t *testing.T) {
		h := newHarness(t)
		var n atomic.Int32
		h.meta.steps = []genStep{unavailableErr()}
		in := inReply()
		in.Commit = func(context.Context) error { n.Add(1); return nil }
		if _, re := h.run(TaskReflection, in); re != nil || n.Load() != 2 {
			t.Fatalf("err %v commit checks %d", re, n.Load())
		}
		h2 := newHarness(t)
		n.Store(0)
		if _, re := h2.run(TaskReflection, in); re != nil || n.Load() != 1 {
			t.Fatalf("Meta: err %v commit checks %d", re, n.Load())
		}
	})
	t.Run("no version re-check", func(t *testing.T) {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		in := inReply()
		in.Commit = nil
		_, re := h.run(TaskReflection, in)
		expect(t, re, OutcomeNoAI, ReasonNoVersionCheck)
		if h.ds.count() != 0 {
			t.Fatal("DeepSeek called without a version re-check")
		}
	})
	for _, p := range []Purpose{PurposeAIProcessing, PurposeDeepSeek} {
		t.Run("consent "+string(p)+" withdrawn during the DeepSeek call", func(t *testing.T) {
			h := newHarness(t)
			h.meta.steps = []genStep{unavailableErr()}
			h.ds.steps = []genStep{{text: reply("Terima kasih sudah cerita."), hook: func() { h.c.withdraw(p) }}}
			_, re := h.run(TaskReflection, inReply())
			expect(t, re, OutcomeStale, ReasonConsentChanged)
		})
	}
	t.Run("version changed before commit", func(t *testing.T) {
		h := newHarness(t)
		var n atomic.Int32
		h.meta.steps = []genStep{unavailableErr()}
		in := inReply()
		in.Commit = func(context.Context) error {
			if n.Add(1) > 1 {
				return errors.New("session ended")
			}
			return nil
		}
		_, re := h.run(TaskListeningReply, in)
		expect(t, re, OutcomeStale, ReasonStale)
		if h.claude.count() != 1 {
			t.Fatalf("claude %d", h.claude.count())
		}
	})
}

// §3.1, RR-2: the personal_memory purpose stays: every call of a run that
// sends confirmed memory items, the fallback included, needs it.
func TestOwnerPersonalMemoryPurposeKept(t *testing.T) {
	if PurposePersonalMemory != "personal_memory" {
		t.Fatal(PurposePersonalMemory)
	}
	for _, role := range []ProviderName{ProviderMeta, ProviderDeepSeek, ProviderAnthropic} {
		ps := purposesFor(role, TaskListeningReply, true)
		if ps[len(ps)-1] != PurposePersonalMemory {
			t.Fatalf("%s: %v", role, ps)
		}
	}
	h := newHarness(t)
	h.c.granted[PurposePersonalMemory] = false
	h.meta.steps = []genStep{unavailableErr()}
	in := inReply("Aku capek banget minggu ini.")
	in.Spans = append([]Span{{Key: "mem-1", Role: RoleMemory, Text: "Aku kerja shift malam."}}, in.Spans...)
	_, re := h.run(TaskListeningReply, in)
	expect(t, re, OutcomeNoAI, ReasonNoConsent)
	noCalls(t, h)
}

// DM-2 through the router: a part of a multi-word private name and a
// phone number written with "·", "/" or "_" never reach a provider, and
// the output guard blocks a part that a generator writes alone.
func TestOwnerPrivacyGaps(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{unavailableErr()}
	in := inReply("Santoso marah lagi. WA dia 0812·3456·7890 atau 0812/3456/7890 atau 0812_3456_7890.")
	in.PrivateNames = []string{"Budi Santoso"}
	if _, re := h.run(TaskReflection, in); re != nil {
		t.Fatal(re)
	}
	for _, req := range []GenerateRequest{h.meta.req(0), h.ds.req(0)} {
		if strings.Contains(req.User, "Santoso") || strings.Contains(req.User, "3456") {
			t.Fatalf("sent: %s", req.User)
		}
	}
	h2 := newHarness(t)
	h2.meta.steps = []genStep{{text: reply("Mungkin Santoso bisa bantu.")}}
	in2 := inReply("Aku capek.")
	in2.PrivateNames = []string{"Budi Santoso"}
	_, re := h2.run(TaskListeningReply, in2)
	expect(t, re, OutcomeAbstain, ReasonGuardRejected)
	if !hasReason(re, "guard:private_name") || h2.ds.count() != 0 {
		t.Fatalf("reasons %v deepseek %d", re.Reasons, h2.ds.count())
	}
}

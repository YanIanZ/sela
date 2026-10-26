package ai

// Regression tests for the third review round of the AI layer (policy and
// privacy, 2026-10-26). Each test names the rule it pins and reproduces the
// reviewer's probe.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/anthropic"
	"github.com/YanIanZ/sela/internal/ai/deepseek"
	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

// RR-2, RR-13, §3.1: the age band read before the first call (whose
// audience class went into every prompt and VC-1 request) must hold until
// commit. Any change, also between two minor bands of one audience class,
// discards the run; output written and checked for "adult" is never
// released to a 15-17 user (probe TestProbeR3BandChangeKeepsAdultAudience).
func TestR3BandChangeIsStale(t *testing.T) {
	t.Run("during the Meta call of AT-4", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.meta.steps[0].hook = func() { h.c.setBand(Band15To17) }
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeStale, ReasonBandChanged)
		if h.claude.count() != 0 || !strings.Contains(h.meta.req(0).User, `"audience":"adult"`) {
			t.Fatalf("claude %d", h.claude.count())
		}
	})
	t.Run("during the Claude call", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.claude.steps = []verStep{{hook: func() { h.c.setBand(Band15To17) }}}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeStale, ReasonBandChanged)
	})
	t.Run("between two minor bands", func(t *testing.T) {
		h := newHarness(t)
		h.c.band = BandUnknown
		h.meta.steps = []genStep{{text: reply("Terima kasih sudah cerita."), hook: func() { h.c.setBand(Band15To17) }}}
		_, re := h.run(TaskListeningReply, inReply())
		expect(t, re, OutcomeStale, ReasonBandChanged)
		if !strings.Contains(h.meta.req(0).User, `"audience":"young_person"`) {
			t.Fatal("a minor band was not sent as young_person")
		}
	})
	t.Run("after the last call, at commit", func(t *testing.T) {
		h := newHarness(t)
		in := inReply()
		in.Validate = func(Task, []DraftItem, []minimize.Span) string { h.c.setBand(Band15To17); return "" }
		_, re := h.run(TaskListeningReply, in)
		expect(t, re, OutcomeStale, ReasonBandChanged)
	})
	t.Run("during the first call of a test-double alternative verifier", func(t *testing.T) {
		// Probe TestProbeR3AltTestDoubleBandChange: the alternative slot is
		// DeepSeek's row of §8.3 whatever the client reports.
		var items []string
		for i := 0; i < 24; i++ {
			items = append(items, fmt.Sprintf("Aku susah tidur sejak %d bulan.", i+1))
		}
		h, in := summaryHarness(t, items...)
		h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
		h.alt.id = verifier.Identity{Provider: deepseek.Provider, Model: deepseek.TestDoubleModel, TestDouble: true, Label: "TEST DOUBLE"}
		h.alt.consent = nil
		h.alt.steps = []verStep{{hook: func() { h.c.setBand(Band15To17) }}}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeStale, ReasonBandChanged)
		if h.alt.count() != 1 {
			t.Fatalf("alternative calls %d after the band changed", h.alt.count())
		}
	})
}

// AT-4 ("for each span with role item, write one summary item ... citing
// exactly that span ... do not merge items"), VC-8, §3 AT-4: a merged,
// left-out or repeated item is invalid output, so no selected item
// disappears and no quote is shown under another item's key (probe
// TestProbeR3SummaryMergesItems).
func TestR3SummaryItemsOneToOne(t *testing.T) {
	two := []string{"Aku susah tidur sejak dua bulan.", "Aku tidak pernah cerita ke ibu."}
	for name, draft := range map[string]string{
		"merged":   genJSON("none", false, [2]string{"Saya susah tidur dan sudah cerita ke ibu.", "s1,s2"}),
		"left out": genJSON("none", false, [2]string{"Saya susah tidur sejak dua bulan.", "s1"}),
		"repeated": genJSON("none", false, [2]string{"Saya susah tidur.", "s1"}, [2]string{"Saya sulit tidur.", "s1"}),
		"extra": genJSON("none", false, [2]string{"Saya susah tidur.", "s1"}, [2]string{"Saya tidak cerita ke ibu.", "s2"},
			[2]string{"Saya sedih.", "s2"}),
	} {
		t.Run(name, func(t *testing.T) {
			h, in := summaryHarness(t, two...)
			h.meta.steps = []genStep{{text: draft}}
			_, re := h.run(TaskSummaryDraft, in)
			expect(t, re, OutcomeAbstain, ReasonInvalidOutput)
			if h.claude.count() != 0 || h.ds.count() != 0 {
				t.Fatalf("claude %d deepseek %d", h.claude.count(), h.ds.count())
			}
		})
	}
	// One item per span, in any order: a contradicted item falls back to
	// its own quote under its own key.
	h, in := summaryHarness(t, two...)
	h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya tidak pernah cerita ke ibu.", "s2"},
		[2]string{"Saya sudah dua bulan tidak bisa tidur sama sekali.", "s1"})}}
	h.claude.steps = []verStep{{report: func(r verifier.Request) verifier.Report {
		return withVerdicts(r, map[int]verifier.Verdict{1: verifier.Contradicted})
	}}}
	out, re := h.run(TaskSummaryDraft, in)
	if re != nil {
		t.Fatal(re)
	}
	if len(out.Items) != 2 || out.Items[0].Source != SourceAIWording || strings.Join(out.Items[0].SpanKeys, ",") != "item-db-b" ||
		out.Items[1].Source != SourceUserQuote || out.Items[1].Text != two[0] || strings.Join(out.Items[1].SpanKeys, ",") != "item-db-a" {
		t.Fatalf("items %+v", out.Items)
	}
	// The DeepSeek fallback is held to the same rule.
	h2, in2 := summaryHarness(t, two...)
	h2.meta.steps = []genStep{unavailableErr()}
	h2.ds.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya susah tidur dan sudah cerita ke ibu.", "s1,s2"})}}
	_, re = h2.run(TaskSummaryDraft, in2)
	expect(t, re, OutcomeNoAI, ReasonInvalidOutput)
	if h2.claude.count() != 0 {
		t.Fatal("a merged fallback draft was verified")
	}
	// AT-3 may cite several spans; its quote fallback shows one quote
	// under that span's key only.
	h3 := newHarness(t)
	h3.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya perawat yang kerja shift malam.", "s1,s2"})}}
	h3.claude.steps = []verStep{{report: func(r verifier.Request) verifier.Report {
		return withVerdicts(r, map[int]verifier.Verdict{0: verifier.Unsupported})
	}}}
	out, re = h3.run(TaskMemoryProposal, inReply("Aku kerja shift malam.", "Aku perawat di rumah sakit."))
	if re != nil {
		t.Fatal(re)
	}
	if len(out.Items) != 1 || out.Items[0].Source != SourceUserQuote || out.Items[0].Text != "Aku kerja shift malam." ||
		strings.Join(out.Items[0].SpanKeys, ",") != "turn-db-a" {
		t.Fatalf("items %+v", out.Items)
	}
}

// limitAfter fails Allow for provider once it has allowed n calls.
type limitAfter struct {
	mu       sync.Mutex
	provider ProviderName
	n        int
}

func (l *limitAfter) Allow(_ context.Context, _ Subject, p ProviderName, _ Task) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if p != l.provider {
		return nil
	}
	if l.n == 0 {
		return ErrUserLimit
	}
	l.n--
	return nil
}

// VC-8 ("safety_signal raises the route"), D-035, CK-9: an output-level
// issue in one chunk withholds the output but does not stop CK-9 on the
// other chunks. They go to the same verifier only; a failure there keeps
// the withhold and never reaches the alternative verifier (probe
// TestProbeR3EarlyStopSkipsCK9).
func TestR3CK9RunsAfterOutputLevelIssue(t *testing.T) {
	medicalThenSafety := func(req verifier.Request, call int) verifier.Report {
		if call == 1 {
			return withVerdicts(req, nil, verifier.Issue{Category: verifier.MedicalLanguage, ClaimID: req.Claims[0].ID, SpanIDs: []string{}, Reason: "x"})
		}
		return withVerdicts(req, nil, verifier.Issue{Category: verifier.SafetySignal, ClaimID: "", SpanIDs: []string{}, Reason: "x"})
	}
	scripted := func(f func(verifier.Request, int) verifier.Report) []verStep {
		call := 0
		return []verStep{{report: func(r verifier.Request) verifier.Report { call++; return f(r, call) }}}
	}
	t.Run("a safety signal in chunk 2 raises the route", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.claude.steps = scripted(medicalThenSafety)
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeSafeStop, ReasonSafetySignal)
		if !re.ShowHelp || h.claude.count() != 2 || h.alt.count() != 0 || re.Provenance.Guards.Risk != RiskElevated {
			t.Fatalf("help %v claude %d alt %d", re.ShowHelp, h.claude.count(), h.alt.count())
		}
	})
	t.Run("chunk 2 refused: the withhold stands, no alternative", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.claude.steps = []verStep{scripted(medicalThenSafety)[0], {err: verr(transport.ReasonRefused)}}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, ReasonWithheld)
		expect(t, re, OutcomeAbstain, string(verifier.MedicalLanguage))
		expect(t, re, OutcomeAbstain, ReasonSafetyPassIncomplete)
		if v := re.Provenance.Verification; h.alt.count() != 0 || v.SafetyPassIncomplete != "refused" || v.Status != StatusChecked {
			t.Fatalf("alt %d verification %+v", h.alt.count(), v)
		}
	})
	t.Run("a limit before chunk 2: the withhold stands", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.lim = &limitAfter{provider: ProviderAnthropic, n: 1}
		h.claude.steps = scripted(medicalThenSafety)
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, ReasonWithheld)
		if v := re.Provenance.Verification; h.claude.count() != 1 || v.SafetyPassIncomplete != ReasonUserLimit {
			t.Fatalf("claude %d verification %+v", h.claude.count(), v)
		}
	})
	t.Run("a consent change during chunk 2 is still stale", func(t *testing.T) {
		h, in := thirteenItems(t)
		first := scripted(medicalThenSafety)[0]
		h.claude.steps = []verStep{first, {hook: func() { h.c.withdraw(PurposeAnthropic) }}}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeStale, ReasonConsentChanged)
	})
	t.Run("the alternative verifier follows the same rule on its own route", func(t *testing.T) {
		h, in := thirteenItems(t)
		h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
		h.alt.steps = scripted(medicalThenSafety)
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeSafeStop, ReasonSafetySignal)
		if h.claude.count() != 1 || h.alt.count() != 2 {
			t.Fatalf("claude %d alt %d", h.claude.count(), h.alt.count())
		}
	})
}

// ctxConsent honours its context like a database-backed checker.
type ctxConsent struct{ *consentFake }

func (c ctxConsent) Consent(ctx context.Context, s Subject, p Purpose) (Consent, error) {
	if err := ctx.Err(); err != nil {
		return Consent{}, err
	}
	return c.consentFake.Consent(ctx, s, p)
}

func (c ctxConsent) AgeBand(ctx context.Context, s Subject) (AgeBand, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return c.consentFake.AgeBand(ctx, s)
}

// RR-11 ("says ... why"), §10: a user cancel during a call is canceled,
// never a consent change, also with a context-aware consent store; when a
// verifier was called its status says so (probe
// TestProbeR3CancelLooksLikeConsentChange).
func TestR3CancelIsNotAConsentChange(t *testing.T) {
	for _, stage := range []string{"meta", "meta with content", "claude", "deepseek fallback", "alternative verifier"} {
		for _, aware := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/context-aware=%v", stage, aware), func(t *testing.T) {
				h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				canceled := &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonCanceled}
				verifierCalled := false
				switch stage {
				case "meta":
					h.meta.steps = []genStep{{hook: cancel, err: canceled}}
				case "meta with content":
					h.meta.steps[0].hook = cancel
				case "claude":
					h.claude.steps = []verStep{{hook: cancel, err: verr(transport.ReasonCanceled)}}
					verifierCalled = true
				case "deepseek fallback":
					h.meta.steps = []genStep{unavailableErr()}
					h.ds.steps = []genStep{{hook: cancel, err: canceled}}
				case "alternative verifier":
					h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
					h.alt.steps = []verStep{{hook: cancel, err: transport.NewError("deepseek", transport.ReasonCanceled, 0, "")}}
					verifierCalled = true
				}
				var c ConsentChecker = h.c
				if aware {
					c = ctxConsent{h.c}
				}
				r, err := NewRouter(h.cfg, Deps{Meta: h.meta, DeepSeek: h.ds, Claude: h.claude, AltVerifier: h.alt, Consent: c, Screener: screenFake{}})
				if err != nil {
					t.Fatal(err)
				}
				_, err = r.Run(ctx, TaskSummaryDraft, in)
				var re *RunError
				if !errors.As(err, &re) {
					t.Fatalf("err %v", err)
				}
				expect(t, re, OutcomeCanceled, ReasonCanceled)
				if !errors.Is(re, ErrCanceled) {
					t.Fatal("not ErrCanceled")
				}
				v := re.Provenance.Verification
				if verifierCalled && (v.Status != StatusVerificationUnavailable || !strings.Contains(strings.Join(v.UnavailableReasons, ","), ReasonCanceled)) {
					t.Fatalf("verification %+v", v)
				}
				if !verifierCalled && v.Status != StatusNotRequested {
					t.Fatalf("verification %+v", v)
				}
			})
		}
	}
}

// A consent-store failure is consent_check_failed, never a consent change
// (stale), before or after a call and at commit.
func TestR3ConsentStoreFailureIsNotAConsentChange(t *testing.T) {
	storeDown := func(h *harness) func() {
		return func() {
			h.c.mu.Lock()
			h.c.err = errors.New("store down")
			h.c.mu.Unlock()
		}
	}
	h := newHarness(t)
	h.meta.steps = []genStep{{text: reply("Terima kasih sudah cerita."), hook: storeDown(h)}}
	_, re := h.run(TaskListeningReply, inReply())
	expect(t, re, OutcomeNoAI, ReasonConsentCheck)

	h2, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h2.claude.steps = []verStep{{hook: storeDown(h2)}}
	_, re = h2.run(TaskSummaryDraft, in)
	expect(t, re, OutcomeAbstain, ReasonConsentCheck)
	if v := re.Provenance.Verification; v.Status != StatusVerificationUnavailable || h2.alt.count() != 0 {
		t.Fatalf("verification %+v alt %d", v, h2.alt.count())
	}

	h3 := newHarness(t)
	in3 := inReply()
	in3.Validate = func(Task, []DraftItem, []minimize.Span) string { storeDown(h3)(); return "" }
	_, re = h3.run(TaskListeningReply, in3)
	expect(t, re, OutcomeAbstain, ReasonConsentCheck)
}

// storeError tells the caller's cancellation and the operation deadline
// from a consent-store failure.
func TestR3StoreErrorReasons(t *testing.T) {
	live := context.Background()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel2()
	for _, tc := range []struct {
		parent, ctx context.Context
		want        string
	}{
		{live, live, ReasonConsentCheck},
		{canceled, canceled, ReasonCanceled},
		{expired, expired, ReasonCanceled}, // the caller's own deadline
		{live, expired, ReasonBudget},      // the operation deadline
	} {
		st := &run{parent: tc.parent}
		if got := st.storeError(tc.ctx); got != tc.want {
			t.Errorf("storeError = %s, want %s", got, tc.want)
		}
	}
}

// Each Deps slot holds its provider; role, consents, band row and labels
// come from the slot (probes TestProbeR3ClaudeSlotHoldsDeepSeek,
// AltSlotHoldsClaude, AltWithoutLabel, AltTestDoubleProviderName).
func TestR3SlotsHoldTheirProviders(t *testing.T) {
	c := allConsent()
	build := func(d Deps) error {
		d.Consent = c
		_, err := NewRouter(testConfig(), d)
		return err
	}
	meta, ds, claude, alt := newGen(t, ProviderMeta, c), newGen(t, ProviderDeepSeek, c), newClaude(t, c), newAlt(t, c)
	if err := build(Deps{Meta: meta, DeepSeek: ds, Claude: claude, AltVerifier: alt}); err != nil {
		t.Fatal(err)
	}
	testDouble := func(provider string) *verFake {
		return &verFake{t: t, id: verifier.Identity{Provider: provider, Model: "test-double", TestDouble: true, Label: "TEST DOUBLE"}}
	}
	for name, d := range map[string]Deps{
		"DeepSeek verifier in the Claude slot":        {Meta: meta, Claude: newAlt(t, c)},
		"Claude in the alternative slot":              {Meta: meta, AltVerifier: newClaude(t, c)},
		"test_double provider in the Claude slot":     {Meta: meta, Claude: testDouble("test_double")},
		"test_double provider in the alternative":     {Meta: meta, AltVerifier: testDouble("test_double")},
		"DeepSeek generator in the Meta slot":         {Meta: newGen(t, ProviderDeepSeek, c)},
		"Meta generator in the DeepSeek slot":         {Meta: meta, DeepSeek: newGen(t, ProviderMeta, c)},
		"test-double generator without a Meta role":   {Meta: &genFake{t: t, id: GeneratorIdentity{Role: ProviderTestDouble, TestDouble: true}}},
		"anthropic test double in the alternative":    {Meta: meta, AltVerifier: testDouble(anthropic.Provider)},
		"deepseek test double in the Claude slot":     {Meta: meta, Claude: testDouble(deepseek.Provider)},
		"alternative verifier with an empty provider": {Meta: meta, AltVerifier: testDouble("")},
	} {
		if err := build(d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// Labels come from the slot.
	for _, tc := range []struct {
		name  string
		setup func(h *harness)
		prov  string
		label string
	}{
		{"live Claude", func(h *harness) {}, VerifiedLiveAnthropic, "Claude"},
		{"alternative verifier without a label", func(h *harness) {
			h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
			h.alt.id.Label = ""
		}, VerifiedLiveDeepSeekAlt, deepseek.AlternativeLabel},
		{"alternative test double calling itself Claude", func(h *harness) {
			h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
			h.alt.id = verifier.Identity{Provider: deepseek.Provider, Model: deepseek.TestDoubleModel, TestDouble: true, Label: "Claude"}
		}, VerifiedTestDouble, deepseek.TestDoubleLabel + " (" + deepseek.AlternativeLabel + ")"},
		{"Claude test double without a label", func(h *harness) {
			h.claude.id = verifier.Identity{Provider: anthropic.Provider, Model: anthropic.TestDoubleModel, TestDouble: true}
		}, VerifiedTestDouble, anthropic.TestDoubleLabel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
			tc.setup(h)
			out, re := h.run(TaskSummaryDraft, in)
			if re != nil {
				t.Fatal(re)
			}
			if v := out.Provenance.Verification; v.Provenance != tc.prov || out.Items[0].CheckLabel != tc.label {
				t.Fatalf("provenance %s label %q", v.Provenance, out.Items[0].CheckLabel)
			}
		})
	}
}

// EV-7, RR-4, RR-5, RR-9, RR-10, §4: every transport.Reason at every
// router position. Only an availability failure reaches the alternative
// verifier or the fallback generator; a DeepSeek fallback failure ends in
// no-AI; an integrity failure raises the operator alert; nothing failed is
// released (probe TestProbeR3ReasonSweep).
func TestR3EveryReasonIsRouted(t *testing.T) {
	for _, r := range transport.Reasons() {
		availability, integrity := r.Class() == transport.ClassAvailability, r.Class() == transport.ClassIntegrity
		failed := OutcomeAbstain
		if r == transport.ReasonCanceled {
			failed = OutcomeCanceled
		}
		t.Run(string(r), func(t *testing.T) {
			// The Claude verifier fails.
			h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
			h.claude.steps = []verStep{{err: verr(r)}}
			out, re := h.run(TaskSummaryDraft, in)
			if (h.alt.count() == 1) != availability {
				t.Errorf("claude %s: alternative calls %d", r, h.alt.count())
			}
			if availability {
				if re != nil || out.Provenance.Verification.Route != AlternativeVerifierRoute {
					t.Errorf("claude %s: %v", r, re)
				}
			} else if re == nil || re.Outcome != failed {
				t.Errorf("claude %s: %v", r, re)
			}
			if h.ev.has(StageIntegrity) != integrity {
				t.Errorf("claude %s: integrity alert %v", r, h.ev.has(StageIntegrity))
			}

			// The alternative verifier fails after a Claude outage.
			h, in = summaryHarness(t, "Aku susah tidur sejak dua bulan.")
			h.claude.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
			h.alt.steps = []verStep{{err: transport.NewError("deepseek", r, 0, "")}}
			_, re = h.run(TaskSummaryDraft, in)
			if re == nil || re.Outcome != failed || h.claude.count() != 1 || h.alt.count() != 1 {
				t.Errorf("alternative %s: %v claude %d alt %d", r, re, h.claude.count(), h.alt.count())
			}
			if h.ev.has(StageIntegrity) != integrity {
				t.Errorf("alternative %s: integrity alert %v", r, h.ev.has(StageIntegrity))
			}

			// The DeepSeek fallback generator fails: no-AI, one switch.
			h = newHarness(t)
			h.meta.steps = []genStep{unavailableErr()}
			h.ds.steps = []genStep{{err: &GenerateError{Class: r.Class(), Reason: r}}}
			_, re = h.run(TaskListeningReply, inReply())
			// RR-6: a fallback failure ends in no-AI, whatever its reason
			// (the caller did not cancel here).
			if re == nil || re.Outcome != OutcomeNoAI || h.meta.count() != 1 || h.ds.count() != 1 || h.claude.count() != 0 {
				t.Errorf("fallback %s: %v meta %d deepseek %d", r, re, h.meta.count(), h.ds.count())
			}
			if integrity && (!h.ev.has(StageIntegrity) || !strings.Contains(strings.Join(re.Reasons, ","), ReasonIntegrity)) {
				t.Errorf("fallback %s: no integrity alert (%v)", r, re)
			}

			// Meta fails.
			h = newHarness(t)
			ge := &GenerateError{Class: r.Class(), Reason: r}
			if availability {
				ge.FallbackReason = FallbackUnavailable
			}
			h.meta.steps = []genStep{{err: ge}}
			out, re = h.run(TaskListeningReply, inReply())
			if (h.ds.count() == 1) != availability {
				t.Errorf("meta %s: deepseek calls %d", r, h.ds.count())
			}
			switch {
			case availability:
				if re != nil || out.Provenance.Route != RouteFallback {
					t.Errorf("meta %s: %v", r, re)
				}
			case integrity:
				expect(t, re, OutcomeAbstain, ReasonIntegrity)
			case r.Class() == transport.ClassContent:
				expect(t, re, OutcomeAbstain, string(r))
			case r == transport.ReasonCanceled:
				expect(t, re, OutcomeCanceled, string(r))
			case r == transport.ReasonDisabled:
				expect(t, re, OutcomeNoAI, ReasonKillSwitch)
			default:
				expect(t, re, OutcomeNoAI, string(r))
			}
			if h.ev.has(StageIntegrity) != integrity {
				t.Errorf("meta %s: integrity alert %v", r, h.ev.has(StageIntegrity))
			}
		})
	}
}

// sweepRouter wires the real Meta, Claude and DeepSeek clients to labelled
// fakellm doubles. Each request is bounded at 800 ms by the HTTP client,
// so a timeout or a stall fails fast inside the router's attempt bounds.
func sweepRouter(t *testing.T, e e2e) (*Router, *counting, *counting, *events) {
	t.Helper()
	metaSrv, _ := doubleServer(t, e.meta)
	claudeSrv, cc := doubleServer(t, e.claude)
	dsSrv, dc := doubleServer(t, e.deepseek)
	hc := &http.Client{Timeout: 800 * time.Millisecond}
	meta, err := NewMetaGenerator(llm.Config{Mode: llm.ModeTestDouble, BaseURL: metaSrv.URL, ServedModel: llm.TestDoubleServedModel,
		AttemptTimeout: 500 * time.Millisecond}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cl, err := anthropic.New(anthropic.Config{TestDouble: true, BaseURL: claudeSrv.URL, HTTPClient: hc})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := deepseek.New(deepseek.Config{TestDouble: true, BaseURL: dsSrv.URL, HTTPClient: hc})
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.Anthropic.Mode, cfg.DeepSeek.Mode = ModeTestDouble, ModeTestDouble
	ev := &events{}
	r, err := NewRouter(cfg, Deps{Meta: meta, DeepSeek: NewDeepSeekGenerator(ds), Claude: cl, AltVerifier: ds.AsVerifier(),
		Consent: allConsent(), Observer: ev})
	if err != nil {
		t.Fatal(err)
	}
	return r, cc, dc, ev
}

// EV-7 with the real clients: every fakellm.ProviderFault through the
// Claude verifier, the DeepSeek fallback generator and the DeepSeek
// alternative verifier.
func TestR3ProviderFaultsThroughTheRouter(t *testing.T) {
	availability := map[fakellm.ProviderFault]bool{fakellm.ProviderFaultHTTP429: true, fakellm.ProviderFaultHTTP5xx: true,
		fakellm.ProviderFaultSpendCap: true, fakellm.ProviderFaultHTTP401: true, fakellm.ProviderFaultTimeout: true}
	harmless := map[fakellm.ProviderFault]bool{fakellm.ProviderFaultNone: true, fakellm.ProviderFaultWrongEnumCase: true}
	for _, f := range fakellm.ProviderFaults() {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			integrity := f == fakellm.ProviderFaultWrongModel

			// Claude at fault: only an availability failure reaches the
			// alternative verifier.
			r, _, dc, ev := sweepRouter(t, e2e{claude: fakellm.Config{ProviderFault: f}})
			out, err := r.Run(ctx, TaskSummaryDraft, summaryInput())
			if alt := dc.chat.Load() > 0; alt != availability[f] {
				t.Errorf("claude %s: alternative verifier called %v", f, alt)
			}
			if (err == nil) != (availability[f] || harmless[f]) {
				t.Errorf("claude %s: err %v", f, err)
			}
			if err == nil && availability[f] && out.Provenance.Verification.Route != AlternativeVerifierRoute {
				t.Errorf("claude %s: route %s", f, out.Provenance.Verification.Route)
			}
			if ev.has(StageIntegrity) != integrity {
				t.Errorf("claude %s: integrity alert %v", f, ev.has(StageIntegrity))
			}

			// DeepSeek as fallback at fault (Meta down): one switch, never
			// released unless harmless, and only Claude verifies it.
			r, cc, _, ev := sweepRouter(t, e2e{meta: fakellm.Config{Fault: fakellm.FaultHTTP503}, deepseek: fakellm.Config{ProviderFault: f}})
			out, err = r.Run(ctx, TaskSummaryDraft, summaryInput())
			if (err == nil) != harmless[f] {
				t.Errorf("fallback %s: err %v", f, err)
			}
			if err == nil && (out.Provenance.Route != RouteFallback || out.Provenance.Verification.Route != VerifierRoute) {
				t.Errorf("fallback %s: provenance %+v", f, out.Provenance)
			}
			if err != nil && (cc.messages.Load() != 0 || OutcomeOf(err) == OutcomeReleased) {
				t.Errorf("fallback %s: a failed draft reached Claude (%v)", f, err)
			}
			if ev.has(StageIntegrity) != integrity {
				t.Errorf("fallback %s: integrity alert %v", f, ev.has(StageIntegrity))
			}

			// DeepSeek as alternative verifier at fault (Claude down):
			// released only when harmless; Claude is never asked again.
			r, cc, _, ev = sweepRouter(t, e2e{claude: fakellm.Config{ProviderFault: fakellm.ProviderFaultHTTP5xx}, deepseek: fakellm.Config{ProviderFault: f}})
			out, err = r.Run(ctx, TaskSummaryDraft, summaryInput())
			if (err == nil) != harmless[f] {
				t.Errorf("alternative %s: err %v", f, err)
			}
			if err == nil && out.Provenance.Verification.Route != AlternativeVerifierRoute {
				t.Errorf("alternative %s: route %s", f, out.Provenance.Verification.Route)
			}
			if cc.messages.Load() != 2 { // the first Claude attempt and its one retry
				t.Errorf("alternative %s: Claude requests %d", f, cc.messages.Load())
			}
			if ev.has(StageIntegrity) != integrity {
				t.Errorf("alternative %s: integrity alert %v", f, ev.has(StageIntegrity))
			}
		})
	}
}

// D-029, DM-2 end to end: a private name that a contact match covers only
// in part never reaches Meta, the DeepSeek fallback or Claude (probe
// TestProbeR3PrivateNameOverlapReachesProviders).
func TestR3PrivateNameOverlapNeverReachesProviders(t *testing.T) {
	for _, text := range []string{
		"Cari aja fbnya: Dewi Lestari, dia yang bikin aku sedih terus.",
		"Aku di-tag @Dewi Lestari di story itu dan aku malu banget.",
		"Profilnya di linktr.ee/Dewi Lestari, dia sibuk terus.",
		"Rumahnya di Gg. Mawar Indah 3 Dewi Lestari tinggal sendirian.",
	} {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		h.ds.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya sedih karena seseorang.", "s1"})}}
		in := inItems(text)
		in.PrivateNames = []string{"Dewi Lestari"}
		if _, re := h.run(TaskSummaryDraft, in); re != nil {
			t.Fatalf("%q: %v", text, re)
		}
		if h.meta.count() != 1 || h.ds.count() != 1 || h.claude.count() != 1 {
			t.Fatalf("calls: meta %d deepseek %d claude %d", h.meta.count(), h.ds.count(), h.claude.count())
		}
		if sent := sentText(t, h); strings.Contains(sent, "Lestari") || strings.Contains(sent, "Dewi") {
			t.Fatalf("%q: a provider received part of the private name", text)
		}
	}
}

// DM-2, CON-1: a phone number with spaced separators and a number written
// as a placeholder never reach a provider; the number in placeholder form
// is blocked in output (probes TestProbeR3SpacedDashPhone,
// TestProbeR3PlaceholderDigits).
func TestR3PhoneFormsNeverReachProviders(t *testing.T) {
	texts := []string{"Kalau mau ngobrol, WA aku aja di 0812 - 3456 - 7890 ya.", "WA: 0812 – 3456 – 7890", "WA +62 812 . 3456 . 7890",
		"Nomorku [KONTAK-6281234567890] kalau perlu."}
	h, in := summaryHarness(t, texts...)
	h.meta.steps = []genStep{unavailableErr()}
	var gen [][2]string
	for i := range texts {
		gen = append(gen, [2]string{"Saya bisa dihubungi lewat WhatsApp.", "s" + itoa(i+1)})
	}
	h.ds.steps = []genStep{{text: genJSON("none", false, gen...)}}
	if _, re := h.run(TaskSummaryDraft, in); re != nil {
		t.Fatal(re)
	}
	sent := sentText(t, h)
	for _, leak := range []string{"3456", "7890", "6281234567890"} {
		if strings.Contains(sent, leak) {
			t.Fatalf("a provider received %q", leak)
		}
	}
	h2 := newHarness(t)
	h2.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Hubungi saya di [KONTAK-6281234567890] kapan saja.", "s1"})}}
	_, re := h2.run(TaskSummaryDraft, inItems("Nomorku [KONTAK-6281234567890] kalau perlu."))
	expect(t, re, OutcomeAbstain, "guard:contact_detail")
	if strings.Contains(h2.meta.req(0).User, "6281234567890") {
		t.Fatal("Meta received the number")
	}
}

// Round 2 Finding 17 through mode=modal: Meta in modal (live) mode never
// runs at a local or test-double host, in any environment, as the live
// Claude and DeepSeek clients (probe TestProbeR3MetaModalAtTestDoubleHost).
func TestR3MetaModalNeverAtTestDoubleHost(t *testing.T) {
	modal := func(base string) llm.Config {
		return llm.Config{Mode: llm.ModeModal, BaseURL: base, APIKey: "LIVE-VLLM-KEY", ModalKey: "LIVE-MODAL-KEY", ModalSecret: "LIVE-MODAL-SECRET",
			ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc"}
	}
	prod := Config{Env: EnvProduction, Profile: ProfileFull}
	staging := testConfig()
	staging.Env = EnvStaging
	staging.Anthropic.Mode, staging.DeepSeek.Mode = ModeDisabled, ModeDisabled
	for _, base := range []string{"https://sela-fakellm.modal.run", "https://mock--sela.modal.run", "https://test-double--sela.modal.direct"} {
		if _, err := Build(prod, modal(base), Deps{Consent: allConsent()}); err == nil {
			t.Errorf("production accepted Meta in modal mode at %s", base)
		}
	}
	for _, base := range []string{"https://fakellm.internal", "https://127.0.0.1:8443", "https://localhost:8443", "https://sela-fakellm.modal.run",
		"https://glimmer.local"} {
		if _, err := Build(staging, modal(base), Deps{Consent: allConsent()}); err == nil {
			t.Errorf("staging accepted Meta in modal mode at %s", base)
		}
		if _, err := NewMetaGenerator(modal(base), nil); err == nil {
			t.Errorf("NewMetaGenerator accepted modal mode at %s", base)
		}
	}
	for _, base := range []string{"https://sela--glimmer.modal.run", "https://ws--sela.modal.direct"} {
		if _, err := Build(staging, modal(base), Deps{Consent: allConsent()}); err != nil {
			t.Errorf("staging refused Meta at %s: %v", base, err)
		}
	}
}

// §3.1, API_SPEC_V3 §3: memory text reaches a provider only with
// personal_memory, checked before and after every call and at commit
// (probe TestProbeR3PersonalMemoryNotChecked).
func TestR3PersonalMemoryConsent(t *testing.T) {
	withMemory := Input{Subject: subject, Language: "id", Spans: []Span{
		{Key: "mem-1", Role: RoleMemory, Text: "Aku kerja shift malam di rumah sakit."},
		{Key: "u-1", Role: RoleUser, Text: "Aku capek banget minggu ini."},
	}}
	noMemory := func() *harness {
		h := newHarness(t)
		h.c = newConsent(PurposeAIProcessing, PurposeAnthropic, PurposeDeepSeek)
		h.meta.consent, h.ds.consent, h.claude.consent, h.alt.consent = h.c, h.c, h.c, h.c
		return h
	}
	t.Run("AT-1 with memory items needs personal_memory", func(t *testing.T) {
		h := noMemory()
		h.meta.steps = []genStep{unavailableErr()}
		_, re := h.run(TaskListeningReply, withMemory)
		expect(t, re, OutcomeNoAI, ReasonNoConsent)
		noCalls(t, h)
	})
	t.Run("AT-1 without memory items does not", func(t *testing.T) {
		h := noMemory()
		if _, re := h.run(TaskListeningReply, inReply()); re != nil {
			t.Fatal(re)
		}
	})
	t.Run("AT-3 always needs it, before generating", func(t *testing.T) {
		h := noMemory()
		_, re := h.run(TaskMemoryProposal, inReply("Aku kerja shift malam."))
		expect(t, re, OutcomeNoAI, ReasonNoConsent)
		expect(t, re, OutcomeNoAI, "manual_path")
		noCalls(t, h)
	})
	t.Run("granted: recorded and sent", func(t *testing.T) {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		out, re := h.run(TaskListeningReply, withMemory)
		if re != nil {
			t.Fatal(re)
		}
		var purposes []string
		for _, u := range out.Provenance.Consent {
			purposes = append(purposes, string(u.Purpose))
		}
		if got := strings.Join(purposes, ","); got != "ai_processing,personal_memory,ai_provider_deepseek" {
			t.Fatalf("consents used %s", got)
		}
	})
	for name, setup := range map[string]func(h *harness) (Task, Input){
		"withdrawn during the Meta call": func(h *harness) (Task, Input) {
			h.meta.steps = []genStep{{text: reply("Terima kasih sudah cerita."), hook: func() { h.c.withdraw(PurposePersonalMemory) }}}
			return TaskListeningReply, withMemory
		},
		"withdrawn during the DeepSeek fallback": func(h *harness) (Task, Input) {
			h.meta.steps = []genStep{unavailableErr()}
			h.ds.steps = []genStep{{text: reply("Terima kasih sudah cerita."), hook: func() { h.c.withdraw(PurposePersonalMemory) }}}
			return TaskListeningReply, withMemory
		},
		"withdrawn during the Claude check of AT-3": func(h *harness) (Task, Input) {
			h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya kerja shift malam.", "s1"})}}
			h.claude.steps = []verStep{{hook: func() { h.c.withdraw(PurposePersonalMemory) }}}
			return TaskMemoryProposal, inReply("Aku kerja shift malam.")
		},
		"withdrawn before commit": func(h *harness) (Task, Input) {
			in := withMemory
			in.Validate = func(Task, []DraftItem, []minimize.Span) string { h.c.withdraw(PurposePersonalMemory); return "" }
			return TaskListeningReply, in
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			task, in := setup(h)
			_, re := h.run(task, in)
			expect(t, re, OutcomeStale, ReasonConsentChanged)
		})
	}
}

// RunError carries the support information an elevated risk asks for,
// also when nothing is released (RR-7).
func TestR3SupportInfoOnAbstain(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{{text: genJSON("elevated", false, [2]string{"Kamu sepertinya mengalami depresi.", "s1"})}}
	_, re := h.run(TaskListeningReply, inReply())
	expect(t, re, OutcomeAbstain, ReasonRiskElevated)
	raw, _ := json.Marshal(re.Provenance)
	if !re.SupportInfo || !re.Provenance.SupportInfo || strings.Contains(string(raw), "depresi") {
		t.Fatalf("support %v provenance %s", re.SupportInfo, raw)
	}
}

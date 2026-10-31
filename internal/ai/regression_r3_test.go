package ai

// Regression tests for the Meta-only verifier routing (D-065). Each test
// names the rule it pins.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
)

// ctxConsent passes the call context to the consent store.
type ctxConsent struct{ *consentFake }

func (c ctxConsent) Consent(ctx context.Context, s Subject, p Purpose) (Consent, error) {
	if err := ctx.Err(); err != nil {
		return Consent{}, err
	}
	return c.consentFake.Consent(ctx, s, p)
}

// RR-2, RR-13: the age band read before the first call must hold until
// commit; any change discards the run.
func TestR3BandChangeIsStale(t *testing.T) {
	t.Run("during the Meta call of AT-4", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.meta.steps[0].hook = func() { h.c.setBand(Band15To17) }
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeStale, ReasonBandChanged)
		if h.verifier.count() != 0 || !strings.Contains(h.meta.req(0).User, `"audience":"adult"`) {
			t.Fatalf("verifier %d", h.verifier.count())
		}
	})
	t.Run("during the verifier call", func(t *testing.T) {
		h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
		h.verifier.steps = []verStep{{hook: func() { h.c.setBand(Band15To17) }}}
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
}

// AT-4: a merged, left-out or repeated item is invalid output, so no
// selected item disappears.
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
			if h.verifier.count() != 0 {
				t.Fatalf("verifier %d", h.verifier.count())
			}
		})
	}
}

// A caller cancellation is canceled, never a consent change (stale).
func TestR3CancelIsNotAConsentChange(t *testing.T) {
	for _, stage := range []string{"meta", "meta with content", "verifier"} {
		for _, aware := range []bool{true, false} {
			t.Run(stage, func(t *testing.T) {
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
				case "verifier":
					h.verifier.steps = []verStep{{hook: cancel, err: verr(transport.ReasonCanceled)}}
					verifierCalled = true
				}
				var c ConsentChecker = h.c
				if aware {
					c = ctxConsent{h.c}
				}
				r, err := NewRouter(h.cfg, Deps{Meta: h.meta, Verifier: h.verifier, Consent: c, Screener: screenFake{}})
				if err != nil {
					t.Fatal(err)
				}
				_, err = r.Run(ctx, TaskSummaryDraft, in)
				var re *RunError
				if !errors.As(err, &re) {
					t.Fatalf("err %v", err)
				}
				expect(t, re, OutcomeCanceled, ReasonCanceled)
				v := re.Provenance.Verification
				if verifierCalled && (v.Status != StatusVerificationUnavailable || !strings.Contains(strings.Join(v.UnavailableReasons, ","), ReasonCanceled)) {
					t.Fatalf("verification %+v", v)
				}
			})
		}
	}
}

// A consent-store failure is consent_check_failed, never a consent change.
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
	h2.verifier.steps = []verStep{{hook: storeDown(h2)}}
	_, re = h2.run(TaskSummaryDraft, in)
	expect(t, re, OutcomeAbstain, ReasonConsentCheck)
	if v := re.Provenance.Verification; v.Status != StatusVerificationUnavailable {
		t.Fatalf("verification %+v", v)
	}
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

// Each Deps slot holds its provider (SPEC V36).
func TestR3SlotsHoldTheirProviders(t *testing.T) {
	c := allConsent()
	build := func(d Deps) error {
		d.Consent = c
		_, err := NewRouter(testConfig(), d)
		return err
	}
	meta, ver := newGen(t, ProviderMeta, c), newVerifier(t, c)
	if err := build(Deps{Meta: meta, Verifier: ver}); err != nil {
		t.Fatal(err)
	}
	testDouble := func(provider string) *verFake {
		return &verFake{t: t, id: verifier.Identity{Provider: provider, Model: "test-double", TestDouble: true, Label: "TEST DOUBLE"}}
	}
	for name, d := range map[string]Deps{
		"wrong verifier provider":            {Meta: meta, Verifier: testDouble("test_double")},
		"empty verifier provider":            {Meta: meta, Verifier: testDouble("")},
		"test-double generator without role": {Meta: &genFake{t: t, id: GeneratorIdentity{Role: ProviderTestDouble, TestDouble: true}}},
	} {
		if err := build(d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	out, re := h.run(TaskSummaryDraft, in)
	if re != nil {
		t.Fatal(re)
	}
	if v := out.Provenance.Verification; v.Provenance != VerifiedLiveMetaVerifier || out.Items[0].CheckLabel != "Meta verifier" {
		t.Fatalf("provenance %s label %q", v.Provenance, out.Items[0].CheckLabel)
	}
}

// TestR3VerifierTestDoubleLabel: a test double always says so, whatever
// label it reports otherwise.
func TestR3VerifierTestDoubleLabel(t *testing.T) {
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	h.verifier.id = verifier.Identity{Provider: "meta_verifier", Model: "test-double", TestDouble: true, Label: "Claude"}
	out, re := h.run(TaskSummaryDraft, in)
	if re != nil {
		t.Fatal(re)
	}
	if v := out.Provenance.Verification; v.Provenance != VerifiedTestDouble || out.Items[0].CheckLabel != "TEST DOUBLE — not the Meta verifier" {
		t.Fatalf("provenance %s label %q", v.Provenance, out.Items[0].CheckLabel)
	}
}

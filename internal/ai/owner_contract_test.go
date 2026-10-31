package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/ai/transport"
)

// Tests of the owner's provider contract after the Meta-only verifier
// decision (D-065): one Meta generator, one different Meta verifier, no
// fallback and no second verifier.

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

func TestOwnerProvenanceValues(t *testing.T) {
	values := []string{DraftLiveMeta, DraftTestDouble, DraftManual, DraftRecordedDemo, DraftUnavailable,
		VerifiedLiveMetaVerifier, VerifiedTestDouble, VerifiedUnavailable, VerifiedNotRequired, VerifiedNotRequested}
	seen := map[string]bool{}
	for _, v := range values {
		if v == "" {
			t.Fatalf("provenance value %q is empty", v)
		}
		if seen[v] && v != "test_double" && v != "unavailable" { // shared labels by design
			t.Fatalf("provenance value %q is duplicated", v)
		}
		seen[v] = true
	}
	if DraftLiveMeta == VerifiedLiveMetaVerifier {
		t.Fatal("draft and verification provenance share a label")
	}
}

func TestOwnerVerifierOffByDefault(t *testing.T) {
	var c Config
	if c.VerifierEnabled() {
		t.Fatal("verifier enabled by default")
	}
	c.MetaVerifier.Mode = ModeDisabled
	if c.VerifierEnabled() {
		t.Fatal("disabled mode enabled the verifier")
	}
}

// TestOwnerNoFallbackOnOutage: a Meta availability failure ends on the
// no-AI path; no second generator and no verifier call.
func TestOwnerNoFallbackOnOutage(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{{err: &GenerateError{Class: transport.ClassAvailability,
		Reason: transport.ReasonUnavailable, FallbackReason: "unavailable"}}}
	_, re := h.run(TaskListeningReply, inReply())
	expect(t, re, OutcomeNoAI, "primary_unavailable")
	if h.verifier.count() != 0 || h.meta.count() != 1 {
		t.Fatalf("meta %d verifier %d", h.meta.count(), h.verifier.count())
	}
}

// TestOwnerVerifierUnavailableWithholds: a verifier-gated task abstains
// when the verifier cannot check the output; nothing is released.
func TestOwnerVerifierUnavailableWithholds(t *testing.T) {
	h := newHarness(t)
	h.verifier.steps = []verStep{{err: verr(transport.ReasonUnavailable)}}
	_, re := h.run(TaskSummaryDraft, inItems("Aku susah tidur."))
	expect(t, re, OutcomeAbstain, StatusVerificationUnavailable)
	if h.verifier.count() != 1 {
		t.Fatalf("verifier calls %d", h.verifier.count())
	}
}

// TestOwnerSelfVerificationBlocked: the verifier must be a different Meta
// model from the generator (SPEC V36).
func TestOwnerSelfVerificationBlocked(t *testing.T) {
	h := newHarness(t)
	h.verifier.id.Model = h.meta.Identity().ModelID
	_, re := h.run(TaskSummaryDraft, inItems("Aku susah tidur."))
	expect(t, re, OutcomeAbstain, ReasonSelfVerification)
	if h.verifier.count() != 0 {
		t.Fatal("a model verified its own output")
	}
}

// TestOwnerVerifierDailySpendCeiling: once Sela's verifier ceiling is
// reached, verifier-gated output abstains instead of being released.
func TestOwnerVerifierDailySpendCeiling(t *testing.T) {
	h := newHarness(t)
	h.cfg.VerifierDailyUSD = 0.0005
	r := h.router()
	if _, err := r.Run(context.Background(), TaskSummaryDraft, inItems("Aku susah tidur.")); err != nil {
		t.Fatalf("first run: %v", err)
	}
	_, err := r.Run(context.Background(), TaskSummaryDraft, inItems("Aku susah tidur."))
	re, _ := err.(*RunError)
	expect(t, re, OutcomeAbstain, ReasonSpendCap)
}

// TestOwnerPersonalMemoryPurpose: a memory run needs personal_memory as
// well as ai_processing.
func TestOwnerPersonalMemoryPurpose(t *testing.T) {
	h := newHarness(t)
	h.c.granted[PurposePersonalMemory] = false
	_, re := h.run(TaskMemoryProposal, inReply("Aku suka menulis."))
	expect(t, re, OutcomeNoAI, ReasonNoConsent)
}

// TestOwnerProvenanceIsContentFree: no prompt, span, output, key or reason
// text reaches provenance.
func TestOwnerProvenanceIsContentFree(t *testing.T) {
	h := newHarness(t)
	out, re := h.run(TaskListeningReply, inReply("CANARY-USER-WORDS"))
	if re != nil {
		t.Fatalf("run: %v", re)
	}
	raw, err := json.Marshal(out.Provenance)
	if err != nil {
		t.Fatal(err)
	}
	for _, canary := range []string{"CANARY-USER-WORDS", "subject", strings.Repeat("x", 40)} {
		if strings.Contains(string(raw), canary) {
			t.Fatalf("provenance leaks %q", canary)
		}
	}
}

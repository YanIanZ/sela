package ai

import (
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/ai/verifier"
)

// RR-4 (i), §17 O-6 (review round 4): an AT-1 current turn that needs more
// than verifier.MaxCitedSpans VC-1 pieces can never be verified (claims
// cites it whole), and Sela knows that before the call, so no text is sent
// to DeepSeek only for its reply to abstain: the run ends on the no-AI path
// with fallback_needs_verifier and verification_input_too_large.
func TestFallbackSkippedWhenAT1VerifierInputTooLarge(t *testing.T) {
	long := strings.Repeat("ابتث ", 900) // 4,500 runes, about 8,100 bytes: 6 pieces
	if n := len(splitSpan(long, verifier.MaxSpanBytes)); n <= verifier.MaxCitedSpans {
		t.Fatalf("probe turn is %d pieces, want more than %d", n, verifier.MaxCitedSpans)
	}
	h := newHarness(t)
	h.meta.steps = []genStep{unavailableErr()}
	_, re := h.run(TaskListeningReply, inReply(long))
	expect(t, re, OutcomeNoAI, ReasonFallbackNeedsVerifier)
	if !hasReason(re, ReasonInputTooLarge) || !hasReason(re, "primary_unavailable") {
		t.Fatalf("reasons %v", re.Reasons)
	}
	if h.ds.count() != 0 || h.claude.count() != 0 {
		t.Fatalf("deepseek calls %d, claude calls %d, want 0 and 0", h.ds.count(), h.claude.count())
	}
	if re.Provenance.DraftProvenance != DraftUnavailable || re.Provenance.FallbackReason != "" {
		t.Fatalf("provenance %+v", re.Provenance)
	}

	// Only the current turn decides: long earlier turns are left out of the
	// claim (claims adds them only while they fit), so the fallback runs.
	fits := strings.Repeat("ابتث ", 300) // about 2,700 bytes: 2 pieces
	h2 := newHarness(t)
	h2.meta.steps = []genStep{unavailableErr()}
	if _, re := h2.run(TaskListeningReply, inReply(long, fits)); re != nil {
		t.Fatalf("a current turn that fits: %s %v", re.Outcome, re.Reasons)
	}
	if h2.ds.count() != 1 || h2.claude.count() != 1 {
		t.Fatalf("deepseek calls %d, claude calls %d, want 1 and 1", h2.ds.count(), h2.claude.count())
	}

	// Meta, the primary, is still called for an oversized turn: the check
	// applies to the DeepSeek fallback only.
	h3 := newHarness(t)
	h3.run(TaskListeningReply, inReply(long))
	if h3.meta.count() != 1 || h3.ds.count() != 0 {
		t.Fatalf("meta calls %d, deepseek calls %d", h3.meta.count(), h3.ds.count())
	}
}

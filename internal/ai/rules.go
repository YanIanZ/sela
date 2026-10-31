package ai

import (
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/ai/guard"
)

// Reason codes used in RunError.Reasons and provenance (content-free).
const (
	ReasonNoConsent        = "no_consent"
	ReasonConsentCheck     = "consent_check_failed"
	ReasonAgeBand          = "age_band"
	ReasonUserLimit        = "user_limit"
	ReasonSpendCap         = "spend_cap"
	ReasonFlagOff          = "flag_off"
	ReasonProfileMetaOnly  = "profile_meta_only"
	ReasonNotConfigured    = "not_configured"
	ReasonVerifierDisabled = "verifier_disabled"
	ReasonKillSwitch       = "kill_switch"
	ReasonBudget           = "budget"
	ReasonTaskDisallows    = "task_disallows_fallback"
	ReasonNotAvailability  = "not_availability_failure"
	ReasonContentStarted   = "content_started"
	ReasonOneSwitch        = "one_switch_used"
	ReasonSelfVerification = "self_verification_blocked"
	ReasonCircuitOpen      = "circuit_open"
	ReasonDeclined         = "declined"
	ReasonRiskCrisis       = "risk_crisis"
	ReasonSafetySignal     = "safety_signal"
	ReasonCrisisScreen     = "crisis_screen"
	ReasonCrisisPaused     = "crisis_paused"
	ReasonInvalidOutput    = "invalid_generation"
	ReasonGuardRejected    = "guard_rejected"
	ReasonMeaningLock      = "meaning_lock_rejected"
	ReasonWithheld         = "withheld_by_verifier"
	ReasonNothingReleased  = "nothing_released"
	ReasonStale            = "stale_result"
	ReasonConsentChanged   = "consent_changed"
	ReasonIntegrity        = "integrity_alert"
	ReasonSpanRole         = "span_role"
	ReasonBudgetReserves   = "budget_below_reserves"
	ReasonPartialReport    = "claude_report_received"
	ReasonInferenceGeo     = "inference_geo_not_us"
	ReasonClaimsCoverage   = "claims_do_not_cover_output"
	// ReasonNoUserEvidence: a claim that needs a check cites only Sela's
	// earlier replies, so there is nothing of the user's to check it
	// against; it is withheld without a verifier call (CK-1, VC-3).
	ReasonNoUserEvidence = "no_user_evidence"
	// ReasonInputTooLarge: a claim or its evidence does not fit VC-1
	// whole; partial evidence is never sent.
	ReasonInputTooLarge = "verification_input_too_large"
	// ReasonRoleLimit: more spans or characters of one role than the task
	// may send (ADR-0003 §3.1: AT-1 at most 5 memory items and 600
	// characters, AT-3 at most 10 memory items); nothing is sent.
	ReasonRoleLimit = "span_role_limit"
	// ReasonBandChanged: the age band changed after it was read for this
	// operation. The audience class already went into the prompts and the
	// VC-1 requests, so the run is stale (RR-13), like a consent change.
	ReasonBandChanged = "age_band_changed"
	// ReasonCanceled: the caller canceled the operation (or its own
	// deadline passed) while a provider call or a consent check ran.
	ReasonCanceled = "canceled"
	// ReasonRiskElevated: the generator raised its risk flag on a draft
	// the guards or Meaning Lock rejected; RR-5 allows no regeneration
	// after a raised risk flag.
	ReasonRiskElevated = "risk_elevated"
	// ReasonSafetyPassIncomplete: a report already withheld the output and
	// the remaining requests, sent only so that CK-9 could still raise the
	// route, could not all be checked. The output stays withheld.
	ReasonSafetyPassIncomplete = "safety_pass_incomplete"
	// ReasonVerifierUnavailable: a verifier-gated output could not be
	// checked because the Meta verifier route is unavailable. Verifier-gated
	// outputs abstain; nothing unverified is released.
	ReasonVerifierUnavailable = "verifier_unavailable"
)

// isStale reports a reason that discards the run as stale (RR-13): a
// consent or age-band change since it was first read.
func isStale(reason string) bool {
	return reason == ReasonConsentChanged || reason == ReasonBandChanged
}

// providerAllowed is the age-band matrix of ADR-0003 §8.3, checked per
// band: a band reaches a provider only when it is in the configured set
// (SELA_ALLOWED_AGE_BANDS). Meta and the Meta verifier serve 15_17 and
// unknown only where that band is enabled, and never in production (B-10).
func providerAllowed(role ProviderName, band AgeBand, cfg Config) bool {
	if !band.Valid() || !cfg.bandAllowed(band) {
		return false
	}
	if band.Adult() {
		return true
	}
	if cfg.Production() {
		return false
	}
	switch role {
	case ProviderMeta, ProviderMetaAPI, ProviderMetaVerifier:
		return true
	}
	return false
}

// purposesFor lists the consents a provider call needs. ai_processing is
// the base AI consent, shared by the Meta generator and the Meta verifier:
// both are Meta processing. memory marks a run of the memory feature (AT-3)
// or one whose payload holds confirmed memory items: every call of it also
// needs personal_memory (§3.1).
func purposesFor(_ ProviderName, task Task, memory bool) []Purpose {
	if task == TaskReadingVerification {
		return nil // AT-6 carries no personal data
	}
	out := []Purpose{PurposeAIProcessing}
	if memory {
		out = append(out, PurposePersonalMemory)
	}
	return out
}

// needsVerification applies RR-8: always for AT-3, AT-4 and AT-6; for
// AT-1, AT-2 and AT-5 when the health-claim detector flags the output.
// Guards can only add a requirement, never remove one.
func needsVerification(pol Policy, g guard.Result) bool {
	switch {
	case pol.Verification == VerifyAlways:
		return true
	case pol.Verification == VerifyWhenFlagged:
		return g.Flagged()
	}
	return false
}

// breaker is the RR-14 circuit breaker: after 3 consecutive upstream Meta
// availability failures within 2 minutes, interactive tasks stop waiting
// for a Meta cold start for 60 s. Meta is still called first (§17 O-12):
// its immediate 503 is the availability failure, and only then does the
// RR-4 check run.
type breaker struct {
	mu        sync.Mutex
	failures  []time.Time
	openUntil time.Time
}

const (
	breakerThreshold = 3
	breakerWindow    = 2 * time.Minute
	breakerOpen      = 60 * time.Second
)

func (b *breaker) isOpen(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return now.Before(b.openUntil)
}

func (b *breaker) failure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	kept := b.failures[:0]
	for _, t := range b.failures {
		if now.Sub(t) <= breakerWindow {
			kept = append(kept, t)
		}
	}
	b.failures = append(kept, now)
	if len(b.failures) >= breakerThreshold {
		b.openUntil = now.Add(breakerOpen)
		b.failures = b.failures[:0]
	}
}

func (b *breaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = b.failures[:0]
}

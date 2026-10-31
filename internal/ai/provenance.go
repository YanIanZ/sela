package ai

import (
	"errors"
	"strings"
	"time"
)

// PriceTableVersion names the price table used for cost estimates
// (observed list prices, ADR-0003 §11; estimates, not measurements).
const PriceTableVersion = "ai-prices-2026-10-30"

// Verification statuses (ADR-0003 §10.1).
const (
	StatusChecked                 = "checked"
	StatusNotRequired             = "not_required"
	StatusVerificationUnavailable = "verification_unavailable"
	StatusAbstained               = "abstained"
	StatusNotRequested            = "not_requested"
)

// V35 draft and verification provenance values (ADR-0003 §10). The
// verifier is always a Meta model, recorded as live_meta_verifier. A run
// that produced no AI draft (the no-AI path, or an abstention, stop or
// stale discard before any draft passed the guards) carries the owner's
// "unavailable".
const (
	DraftLiveMeta     = "live_meta"
	DraftLiveMetaAPI  = "live_meta_api"
	DraftTestDouble   = "test_double"
	DraftManual       = "manual"
	DraftRecordedDemo = "recorded_demo"
	DraftUnavailable  = "unavailable"

	VerifiedLiveMetaVerifier = "live_meta_verifier"
	VerifiedTestDouble       = "test_double"
	VerifiedUnavailable      = "unavailable"
	VerifiedNotRequired      = "not_required"
	VerifiedNotRequested     = "not_requested"
)

// Provenance is the per-output record of ADR-0003 §10. It holds enums,
// ids of rules, counts, versions, latencies and token counts only: no
// prompt, span, output, verdict reason, key, subject or chain-of-thought.
type Provenance struct {
	Task            Task   `json:"task"`
	TaskID          string `json:"taskId"`
	ProvenanceClass string `json:"provenanceClass"`
	ReviewStatus    string `json:"reviewStatus"`
	PromptVersion   string `json:"promptVersion,omitempty"`
	SchemaVersion   string `json:"schemaVersion,omitempty"`
	// Route is the generator route of the released output.
	Route       Route `json:"route"`
	CircuitOpen bool  `json:"circuitOpen,omitempty"`
	// Synthetic says whether the caller marked this operation's data as
	// synthetic demo or test data.
	Synthetic bool `json:"synthetic,omitempty"`
	// DraftProvenance is the V35 draft value.
	DraftProvenance string `json:"draftProvenance"`
	// Generator is the generator whose output is used (nil when none).
	Generator *GeneratorRecord `json:"generator,omitempty"`
	// Calls lists every generator call in order, including failed ones.
	Calls        []GeneratorRecord  `json:"generatorCalls,omitempty"`
	Verification VerificationRecord `json:"verification"`
	Guards       GuardRecord        `json:"guards"`
	// Consent lists the revision of every purpose used, rechecked at
	// commit.
	Consent []ConsentUse `json:"consent,omitempty"`
	// Outcome is the run outcome; Reasons are content-free codes in order
	// of occurrence.
	Outcome      Outcome  `json:"outcome"`
	Reasons      []string `json:"reasons,omitempty"`
	SupportInfo  bool     `json:"supportInfo,omitempty"`
	LatencyMS    int64    `json:"latencyMs"`
	InputTokens  int      `json:"inputTokens"`
	OutputTokens int      `json:"outputTokens"`
	// EstimatedCostUSD covers API tokens only (Meta GPU time is not
	// attributed here).
	EstimatedCostUSD float64        `json:"estimatedCostUsd"`
	PriceTable       string         `json:"priceTable"`
	SpansSent        int            `json:"spansSent"`
	SpansDropped     int            `json:"spansDropped,omitempty"`
	Redactions       map[string]int `json:"redactions,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
}

// GeneratorRecord describes one generator call.
type GeneratorRecord struct {
	Provider ProviderName `json:"provider"`
	// ModelID is the configured model (Meta: the pinned HF repository).
	ModelID string `json:"modelId"`
	// ServedModel is the model reported by the response (Meta: served
	// name; DeepSeek: the response "model" field), ServedRevision the
	// pinned revision (Meta).
	ServedModel    string `json:"servedModel,omitempty"`
	ServedRevision string `json:"servedRevision,omitempty"`
	TestDouble     bool   `json:"testDouble,omitempty"`
	Route          Route  `json:"route"`
	Regeneration   bool   `json:"regeneration,omitempty"`
	Outcome        string `json:"outcome"`
	Reason         string `json:"reason,omitempty"`
	Class          string `json:"class,omitempty"`
	FirstContentMS int64  `json:"firstContentMs,omitempty"`
	LatencyMS      int64  `json:"latencyMs"`
	InputTokens    int    `json:"inputTokens"`
	OutputTokens   int    `json:"outputTokens"`
	Attempts       int    `json:"attempts"`
}

// VerificationRecord describes the verification of one output. The
// provider, model, calls, latency and token fields describe the verifier
// whose report was used (or, when none was, the last one tried); Attempts
// keeps one sub-record per verifier, in order, so an alternative-verifier
// check never credits Claude's calls or tokens to DeepSeek.
type VerificationRecord struct {
	Status     string            `json:"status"`
	Provenance string            `json:"provenance"`
	Route      VerificationRoute `json:"route"`
	Provider   ProviderName      `json:"provider,omitempty"`
	ModelID    string            `json:"modelId,omitempty"`
	// ServedModel is the response model.
	ServedModel string `json:"servedModel,omitempty"`
	APIVersion  string `json:"apiVersion,omitempty"`
	// Label marks test-double verdicts.
	Label  string `json:"label,omitempty"`
	Calls  int    `json:"calls,omitempty"`
	Claims int    `json:"claims,omitempty"`
	// Verdicts and Issues count per enum value.
	Verdicts map[string]int `json:"verdicts,omitempty"`
	Issues   map[string]int `json:"issues,omitempty"`
	// Checks lists the CK-n checks the reports expressed.
	Checks []string `json:"checks,omitempty"`
	// Per-claim outcomes, in claim order, without reasons.
	ClaimOutcomes []ClaimOutcome `json:"claimOutcomes,omitempty"`
	Released      int            `json:"released"`
	QuoteFallback int            `json:"quoteFallback"`
	Dropped       int            `json:"dropped"`
	WithheldAll   bool           `json:"withheldAll,omitempty"`
	// UnavailableReasons are in order of occurrence (§10).
	UnavailableReasons []string `json:"unavailableReasons,omitempty"`
	// SafetyPassIncomplete is the reason ("timeout", "user_limit", ...)
	// the requests after a withholding report could not all be checked.
	// They are still sent after an output-level issue, so CK-9 can raise
	// the route (VC-8); a failure there keeps the output withheld and never
	// reaches another verifier. Empty when every request was checked.
	SafetyPassIncomplete string `json:"safetyPassIncomplete,omitempty"`
	RefusalCategory      string `json:"refusalCategory,omitempty"`
	LatencyMS            int64  `json:"latencyMs,omitempty"`
	InputTokens          int    `json:"inputTokens,omitempty"`
	OutputTokens         int    `json:"outputTokens,omitempty"`
	// Attempts lists every verifier route tried, in order.
	Attempts []VerifierAttempt `json:"attempts,omitempty"`
}

// VerifierAttempt is one verifier's part in the verification of an
// output: its own calls, reports, served model, latency, tokens and
// outcome ("checked", "withheld" for a report that withheld the output
// early, or the failure reason).
type VerifierAttempt struct {
	Route        VerificationRoute `json:"route"`
	Provider     ProviderName      `json:"provider"`
	ModelID      string            `json:"modelId,omitempty"`
	ServedModel  string            `json:"servedModel,omitempty"`
	APIVersion   string            `json:"apiVersion,omitempty"`
	Label        string            `json:"label,omitempty"`
	Calls        int               `json:"calls"`
	Reports      int               `json:"reports"`
	Outcome      string            `json:"outcome"`
	LatencyMS    int64             `json:"latencyMs"`
	InputTokens  int               `json:"inputTokens"`
	OutputTokens int               `json:"outputTokens"`
}

// ClaimOutcome is the content-free outcome of one claim.
type ClaimOutcome struct {
	Verdict     string   `json:"verdict"`
	Disposition string   `json:"disposition"`
	Issues      []string `json:"issues,omitempty"`
	SpansCited  int      `json:"spansCited"`
}

// GuardRecord describes the deterministic checks.
type GuardRecord struct {
	Version string `json:"version"`
	// Blocked lists hard guards that matched in any draft (including a
	// rejected first draft); Flags the health-claim flags of the used
	// draft; Rules the matched rule ids.
	Blocked             []string `json:"blocked,omitempty"`
	Flags               []string `json:"flags,omitempty"`
	Rules               []string `json:"rules,omitempty"`
	MeaningLockRejected int      `json:"meaningLockRejected,omitempty"`
	GuardRejected       int      `json:"guardRejected,omitempty"`
	Regenerations       int      `json:"regenerations,omitempty"`
	// Risk is the highest risk seen (input screen, generator flag,
	// verifier safety signal).
	Risk Risk `json:"risk"`
}

// ConsentUse records the revision of one purpose used.
type ConsentUse struct {
	Purpose  Purpose `json:"purpose"`
	Revision string  `json:"revision"`
}

// Outcome is a run outcome.
type Outcome string

// Outcomes.
const (
	OutcomeReleased     Outcome = "released"
	OutcomeModelFree    Outcome = "model_free"    // AT-0: never a model
	OutcomeCrisisHold   Outcome = "crisis_hold"   // RR-1: help shown, zero calls
	OutcomeNoAI         Outcome = "no_ai"         // the honest no-AI path
	OutcomeAbstain      Outcome = "abstained"     // manual path, AI wording withheld
	OutcomeSafeStop     Outcome = "safe_stop"     // declined, crisis risk or safety signal
	OutcomeStale        Outcome = "stale_result"  // RR-13: discarded at commit
	OutcomeInvalidInput Outcome = "invalid_input" // the request was not run
	OutcomeCanceled     Outcome = "canceled"
)

// Sentinel errors; errors.Is(err, ErrAbstain) holds for a RunError with
// that outcome.
var (
	ErrModelFree    = errors.New("ai: crisis support is model-free")
	ErrCrisisHold   = errors.New("ai: crisis screen matched; no model call")
	ErrNoAI         = errors.New("ai: no AI available for this operation")
	ErrAbstain      = errors.New("ai: abstained")
	ErrSafeStop     = errors.New("ai: safe stop")
	ErrStale        = errors.New("ai: stale result discarded")
	ErrInvalidInput = errors.New("ai: invalid input")
	ErrCanceled     = errors.New("ai: canceled")
)

var outcomeErrors = map[Outcome]error{
	OutcomeModelFree: ErrModelFree, OutcomeCrisisHold: ErrCrisisHold, OutcomeNoAI: ErrNoAI, OutcomeAbstain: ErrAbstain,
	OutcomeSafeStop: ErrSafeStop, OutcomeStale: ErrStale, OutcomeInvalidInput: ErrInvalidInput, OutcomeCanceled: ErrCanceled,
}

// RunError is the typed abstain / unavailable error of Router.Run. It
// carries the provenance of what happened (no content), the reason codes
// the UI explains ("no_consent", "verification_unavailable:refused", ...)
// and whether help or support information must be shown.
type RunError struct {
	Outcome  Outcome
	Reasons  []string
	ShowHelp bool
	// SupportInfo: show support information beside the notice (the input
	// screen or the generator raised the risk to elevated, RR-7), also
	// when nothing is released.
	SupportInfo bool
	Provenance  Provenance
}

func (e *RunError) Error() string {
	msg := "ai: " + string(e.Outcome)
	if len(e.Reasons) > 0 {
		msg += " (" + strings.Join(e.Reasons, ", ") + ")"
	}
	return msg
}

// Is matches the sentinel of e's outcome.
func (e *RunError) Is(target error) bool { return outcomeErrors[e.Outcome] == target }

// OutcomeOf returns the outcome of a Run error (OutcomeReleased for nil).
func OutcomeOf(err error) Outcome {
	if err == nil {
		return OutcomeReleased
	}
	var re *RunError
	if errors.As(err, &re) {
		return re.Outcome
	}
	return OutcomeNoAI
}

// estimateCost returns an API cost estimate in USD from token counts
// (cache misses assumed). Self-hosted Meta GPU time and test doubles cost 0
// here; Meta Model API tokens (generator and verifier) use the
// Standard-mode list prices observed on 2026-10-30.
func estimateCost(role ProviderName, testDouble bool, in, out int) float64 {
	if testDouble {
		return 0
	}
	var inPrice, outPrice float64
	switch role {
	case ProviderMetaAPI, ProviderMetaVerifier:
		inPrice, outPrice = 1.25, 4.25
	default:
		return 0
	}
	return (float64(in)*inPrice + float64(out)*outPrice) / 1e6
}

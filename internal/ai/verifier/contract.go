// Package verifier is the provider-independent verifier contract of
// ADR-0003 §6 and §7: the request a verifier receives (VC-1), the output
// schema (VC-2), the verdict meanings (VC-3), the fail-closed validation
// of a report (VC-6) and the release rules (VC-8).
//
// A verifier only returns verdicts, cited span ids, issue categories and
// one short reason per finding. It never writes user-facing wording, and
// no chain-of-thought is requested or kept (VC-7). Agreement between
// models is not clinical evidence.
package verifier

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// Versions recorded in provenance.
const (
	// SchemaVersion names the VC-2 output schema.
	SchemaVersion = "sela-verifier-v1"
	// InstructionsVersion names the system instructions in prompt.go.
	InstructionsVersion = "sela-verifier-instructions-2026.10.26-1"
)

// Verdict is a per-claim verdict (VC-3).
type Verdict string

// Verdicts.
const (
	// Supported: every important detail follows from the cited spans, with
	// no change of negation, speaker, time, certainty or quantity and no
	// added content.
	Supported Verdict = "supported"
	// Unsupported: the claim adds something no supplied span states,
	// although nothing in the spans conflicts with it.
	Unsupported Verdict = "unsupported"
	// Contradicted: a supplied span conflicts with the claim.
	Contradicted Verdict = "contradicted"
	// InsufficientEvidence: the spans are too short, ambiguous or
	// off-topic to decide.
	InsufficientEvidence Verdict = "insufficient_evidence"
)

// Verdicts lists every verdict in schema order.
func Verdicts() []Verdict {
	return []Verdict{Supported, Unsupported, Contradicted, InsufficientEvidence}
}

// Category is an issue category (VC-2).
type Category string

// Issue categories.
const (
	LostContext      Category = "lost_context"
	CitationInvalid  Category = "citation_invalid"
	MedicalLanguage  Category = "medical_language"
	Manipulation     Category = "manipulation"
	AgeInappropriate Category = "age_inappropriate"
	InformationLeak  Category = "information_leak"
	SafetySignal     Category = "safety_signal"
)

// Categories lists every issue category in schema order.
func Categories() []Category {
	return []Category{LostContext, CitationInvalid, MedicalLanguage, Manipulation, AgeInappropriate, InformationLeak, SafetySignal}
}

// Check is one of the nine verifier checks of ADR-0003 §7.
type Check string

// Checks (brief §8 plus the missed-safety-signal safeguard).
const (
	CK1Consistency  Check = "CK-1" // consistency with the user's words: per-claim verdict
	CK2Unsupported  Check = "CK-2" // unsupported claims: verdict unsupported
	CK3Distortion   Check = "CK-3" // distortion or lost context: contradicted, issue lost_context
	CK4Citation     Check = "CK-4" // citation relevance and validity: issue citation_invalid
	CK5Medical      Check = "CK-5" // inappropriate medical language
	CK6Manipulation Check = "CK-6" // manipulation
	CK7Age          Check = "CK-7" // age appropriateness
	CK8Leakage      Check = "CK-8" // information leakage
	CK9SafetySignal Check = "CK-9" // missed safety signal (can only raise the route)
)

// Checks lists the nine checks in order.
func Checks() []Check {
	return []Check{CK1Consistency, CK2Unsupported, CK3Distortion, CK4Citation, CK5Medical, CK6Manipulation, CK7Age, CK8Leakage, CK9SafetySignal}
}

// CheckOfCategory maps an issue category to its check.
func CheckOfCategory(c Category) Check {
	switch c {
	case LostContext:
		return CK3Distortion
	case CitationInvalid:
		return CK4Citation
	case MedicalLanguage:
		return CK5Medical
	case Manipulation:
		return CK6Manipulation
	case AgeInappropriate:
		return CK7Age
	case InformationLeak:
		return CK8Leakage
	case SafetySignal:
		return CK9SafetySignal
	}
	return ""
}

// ChecksOfVerdict maps a verdict to the checks it expresses.
func ChecksOfVerdict(v Verdict) []Check {
	switch v {
	case Supported, InsufficientEvidence:
		return []Check{CK1Consistency}
	case Unsupported:
		return []Check{CK1Consistency, CK2Unsupported}
	case Contradicted:
		return []Check{CK1Consistency, CK3Distortion}
	}
	return nil
}

// Audience classes sent to the verifier and the generator (never the age).
const (
	AudienceAdult       = "adult"
	AudienceYoungPerson = "young_person"
)

// Limits of one verifier call (VC-1).
const (
	MaxClaims        = 12
	MaxSpans         = 16
	MaxSpanBytes     = 1600
	MaxClaimBytes    = 800
	MaxCitedSpans    = 5
	MaxReasonRunes   = 300
	MaxIDBytes       = 16
	MaxResponseBytes = 64 << 10
)

// Span is one source span with a per-request id ("s1", "s2", ...).
type Span struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Claim is one claim with a per-request id ("c1", ...) and its cited spans.
type Claim struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	SpanIDs []string `json:"span_ids"`
}

// Request is the VC-1 input. It is built only by Go from owned, current
// records; ids are opaque and per request, and Go keeps the mapping to
// database ids. It carries no account, user, session or database id.
type Request struct {
	Task     string  `json:"task"`
	Language string  `json:"language"`
	Audience string  `json:"audience"`
	Spans    []Span  `json:"spans"`
	Claims   []Claim `json:"claims"`
}

// ErrInvalidRequest marks a request that violates VC-1; it is never sent.
var ErrInvalidRequest = errors.New("verifier: invalid request")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequest, fmt.Sprintf(format, args...))
}

// Validate checks the VC-1 limits. Messages never contain span or claim
// text.
func (r Request) Validate() error {
	if r.Task == "" || len(r.Task) > 40 {
		return invalid("task is required")
	}
	if r.Language != "id" && r.Language != "en" {
		return invalid("language must be id or en")
	}
	if r.Audience != AudienceAdult && r.Audience != AudienceYoungPerson {
		return invalid("audience must be adult or young_person")
	}
	if len(r.Claims) == 0 || len(r.Claims) > MaxClaims {
		return invalid("between 1 and %d claims are required", MaxClaims)
	}
	if len(r.Spans) == 0 || len(r.Spans) > MaxSpans {
		return invalid("between 1 and %d spans are required", MaxSpans)
	}
	spans := make(map[string]bool, len(r.Spans))
	for i, s := range r.Spans {
		if !validID(s.ID) || spans[s.ID] {
			return invalid("span %d has an invalid or duplicate id", i)
		}
		if s.Text == "" || len(s.Text) > MaxSpanBytes || !utf8.ValidString(s.Text) {
			return invalid("span %d text must be 1..%d bytes of UTF-8", i, MaxSpanBytes)
		}
		spans[s.ID] = true
	}
	claims := make(map[string]bool, len(r.Claims))
	for i, c := range r.Claims {
		if !validID(c.ID) || claims[c.ID] || spans[c.ID] {
			return invalid("claim %d has an invalid or duplicate id", i)
		}
		if c.Text == "" || len(c.Text) > MaxClaimBytes || !utf8.ValidString(c.Text) {
			return invalid("claim %d text must be 1..%d bytes of UTF-8", i, MaxClaimBytes)
		}
		if len(c.SpanIDs) == 0 || len(c.SpanIDs) > MaxCitedSpans {
			return invalid("claim %d must cite 1..%d spans", i, MaxCitedSpans)
		}
		seen := map[string]bool{}
		for _, id := range c.SpanIDs {
			if !spans[id] || seen[id] {
				return invalid("claim %d cites an unknown or repeated span", i)
			}
			seen[id] = true
		}
		claims[c.ID] = true
	}
	return nil
}

// validID accepts short opaque ids of letters and digits.
func validID(id string) bool {
	if id == "" || len(id) > MaxIDBytes {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// ClaimVerdict is one validated per-claim finding.
type ClaimVerdict struct {
	ClaimID string
	Verdict Verdict
	SpanIDs []string
	Reason  string
}

// Issue is one validated issue. ClaimID "" concerns the whole output.
type Issue struct {
	Category Category
	ClaimID  string
	SpanIDs  []string
	Reason   string
}

// Report is a validated verifier report: one verdict per requested claim,
// in request order.
type Report struct {
	Claims []ClaimVerdict
	Issues []Issue
}

// Counts returns the number of claims per verdict.
func (r Report) Counts() map[Verdict]int {
	out := map[Verdict]int{}
	for _, c := range r.Claims {
		out[c.Verdict]++
	}
	return out
}

// Checks lists the checks the report expresses, in check order.
func (r Report) Checks() []Check {
	seen := map[Check]bool{}
	for _, c := range r.Claims {
		for _, ck := range ChecksOfVerdict(c.Verdict) {
			seen[ck] = true
		}
	}
	for _, is := range r.Issues {
		seen[CheckOfCategory(is.Category)] = true
	}
	var out []Check
	for _, ck := range Checks() {
		if seen[ck] {
			out = append(out, ck)
		}
	}
	return out
}

// Identity describes a configured verifier without secrets.
type Identity struct {
	// Provider names the verifier backend: metaverifier.Provider
	// ("meta_verifier") since D-065. "anthropic" and "deepseek" are historical
	// values that stored records may still carry; they are never produced.
	Provider string
	// Model is the configured model id.
	Model string
	// TestDouble is true for a labelled test double.
	TestDouble bool
	// Label is shown with every verdict of a test double or of the
	// alternative verifier.
	Label string
	// InferenceGeo is where the provider processes the request ("us" or
	// "global" for Claude; "" when the verifier does not report it).
	InferenceGeo string
}

// CallOptions tune one call within the task's budget (ADR-0003 §3.2).
type CallOptions struct {
	// MaxRetryAfter: a longer server Retry-After ends the retries. Zero
	// keeps the client default.
	MaxRetryAfter time.Duration
	// AttemptTimeout bounds each attempt. Zero keeps the client default.
	AttemptTimeout time.Duration
	// BeforeAttempt, when set, runs right before every retry; an error
	// stops the call (the router's consent and age-band check, RR-2).
	BeforeAttempt func(ctx context.Context) error
}

// Result is a verifier call's outcome. On failure Report is empty and the
// metadata fields are still set.
type Result struct {
	Report Report
	Identity
	// ServedModel is the model the response reported.
	ServedModel string
	// APIVersion is a provider API version header when one is sent; it is
	// empty for the Meta verifier and kept for historical records.
	APIVersion string
	// RefusalCategory is a refusal's stop_details category when it is a
	// known value, "other" otherwise, "" without a refusal.
	RefusalCategory string
	InputTokens     int
	OutputTokens    int
	Attempts        int
	Status          int
	Latency         time.Duration
}

// Verifier checks claims against spans. Implementations are safe for
// concurrent use and fail closed: any refusal, truncation, schema or enum
// violation, model mismatch or transport failure is an error whose
// transport.Reason says which.
type Verifier interface {
	Identity() Identity
	Verify(ctx context.Context, req Request, opts CallOptions) (Result, error)
}

// Package ai is Sela's AI provider layer (ADR-0003): the task taxonomy
// AT-0..AT-6, the routing state machine and its rules RR-1..RR-16, the
// verifier contract wiring (VC-1..VC-9), deterministic guards, data
// minimisation, provenance and configuration.
//
// Providers:
//   - Meta is the generator for every generative task: Muse Spark on the
//     Meta Model API in development/demo, or Muse Glimmer on Modal in
//     production, through the existing internal/llm client.
//   - A different Meta model (default Muse Wonder) verifies; it never
//     generates. There is no fallback generator and no second verifier:
//     an unavailable generator ends on the honest no-AI path, and an
//     unavailable verifier withholds verifier-gated output.
//
// Crisis support (AT-0) never calls a model. The package never logs or
// stores user content, prompts, model output, keys or chain-of-thought:
// observers and provenance carry metadata only.
package ai

import "time"

// Task is a task of the ADR-0003 §3 taxonomy. Its value is the provenance
// task name.
type Task string

// Tasks.
const (
	TaskCrisisSupport       Task = "crisis_support"       // AT-0
	TaskListeningReply      Task = "listening_reply"      // AT-1
	TaskReflection          Task = "reflection"           // AT-2
	TaskMemoryProposal      Task = "memory_proposal"      // AT-3
	TaskSummaryDraft        Task = "summary_draft"        // AT-4
	TaskOpeningLines        Task = "opening_lines"        // AT-5
	TaskReadingVerification Task = "reading_verification" // AT-6
)

// Tasks lists every task in taxonomy order.
func Tasks() []Task {
	return []Task{TaskCrisisSupport, TaskListeningReply, TaskReflection, TaskMemoryProposal, TaskSummaryDraft,
		TaskOpeningLines, TaskReadingVerification}
}

// Verification says when a task's output must be verified (RR-8).
type Verification string

// Verification rules.
const (
	VerifyNever       Verification = "never"
	VerifyWhenFlagged Verification = "when_flagged"
	VerifyAlways      Verification = "always"
)

// Policy is a task's routing policy: the PM-decided structure of D-029
// and the engineering-default budgets of ADR-0003 §3.2.
type Policy struct {
	ID   string // "AT-1"
	Task Task
	// Generative is false for AT-0 (no model) and AT-6 (a human author).
	Generative bool
	// ModelFree: no provider is ever called (AT-0).
	ModelFree    bool
	Verification Verification
	// Regenerate: one same-provider regeneration after a Meaning Lock or
	// output-guard rejection (RR-5), if RegenerateMinRemaining remains.
	Regenerate             bool
	RegenerateMinRemaining time.Duration
	// Interactive tasks use the circuit breaker (RR-14).
	Interactive bool
	// Budget is the total operation budget; zero means the configured
	// background budget (SELA_LLM_OP_TIMEOUT).
	Budget time.Duration
	// FirstContentWait bounds Meta's wait for first content including a
	// cold start; zero means "until the remaining budget equals the
	// reserves".
	FirstContentWait    time.Duration
	VerifierAttempt     time.Duration
	VerificationReserve time.Duration
	// MaxRetryAfter bounds a provider Retry-After that is still waited for.
	MaxRetryAfter time.Duration
	// MetaMaxRetries is the number of Meta retries before content for a
	// connection failure, 429 or 5xx (§3.2: AT-1, AT-2, AT-5 one; AT-3,
	// AT-4 the v2 two). Modal cold-start polls are not retries.
	MetaMaxRetries int
	// Roles are the span roles the task accepts (§3.1); any other role is
	// invalid input and nothing is sent. RoleSource exists only for AT-6.
	Roles []Role
	// MaxVerifierCalls bounds the verifier calls of one output (AT-4: 3
	// calls of at most 12 claims).
	MaxVerifierCalls int
	// Items bounds the generated items.
	MinItems, MaxItems int
	MaxItemRunes       int
	// RequireCitations: each item must cite at least one span.
	RequireCitations bool
	// OneItemPerSpan: the draft has exactly one item per selected span,
	// each citing exactly that span (AT-4: "for each span with role item,
	// write one summary item ... citing exactly that span ... do not merge
	// items"). Anything else is invalid output, so a selected item never
	// disappears from a summary and every item sits next to its own quote
	// (§3 AT-4, VC-8).
	OneItemPerSpan bool
	// QuoteFallback: a claim that is not released falls back to the user's
	// own quote (VC-8); otherwise it is dropped.
	QuoteFallback bool
	// WithholdOnAnyNotReleased: one non-released claim withholds the whole
	// output (AT-1).
	WithholdOnAnyNotReleased bool
	// Spans bounds the payload (ADR-0003 §3.1).
	MaxSpans      int
	MaxSpanRunes  int
	MaxTotalRunes int
	DropOldest    bool
	// RoleLimits bounds the spans of one role (§3.1: AT-1 at most 5
	// confirmed memory items of at most 600 characters together, AT-3 at
	// most 10 memory items). More is invalid input; nothing is sent.
	RoleLimits map[Role]RoleLimit
	// MaxOutputTokens bounds generation.
	MaxOutputTokens int
	// ProvenanceClass is the D-039 class of the result.
	ProvenanceClass string
}

// Provenance classes (D-039).
const (
	ClassSelaMessage               = "sela_message"
	ClassUnconfirmedInterpretation = "unconfirmed_interpretation"
	ClassAISummary                 = "ai_summary"
	ClassStaticHelp                = "static_verified_help"
	ClassReadingItem               = "public_reading_item"
)

var policies = map[Task]Policy{
	TaskCrisisSupport: {ID: "AT-0", Task: TaskCrisisSupport, ModelFree: true, Verification: VerifyNever,
		ProvenanceClass: ClassStaticHelp},
	TaskListeningReply: {ID: "AT-1", Task: TaskListeningReply, Generative: true, Verification: VerifyWhenFlagged,
		Regenerate: true, RegenerateMinRemaining: 30 * time.Second, Interactive: true,
		Budget: 60 * time.Second, FirstContentWait: 20 * time.Second,
		VerifierAttempt: 15 * time.Second, MaxRetryAfter: 3 * time.Second, MetaMaxRetries: 1, MaxVerifierCalls: 1,
		Roles:    []Role{RoleUser, RoleEarlierReply, RoleMemory},
		MinItems: 1, MaxItems: 1, MaxItemRunes: 1200, WithholdOnAnyNotReleased: true,
		MaxSpans: 7, MaxSpanRunes: 6000, MaxTotalRunes: 6000, DropOldest: true, MaxOutputTokens: 1024,
		RoleLimits:      map[Role]RoleLimit{RoleMemory: {MaxSpans: 5, MaxRunes: 600}},
		ProvenanceClass: ClassSelaMessage},
	TaskReflection: {ID: "AT-2", Task: TaskReflection, Generative: true, Verification: VerifyWhenFlagged,
		Regenerate: true, RegenerateMinRemaining: 30 * time.Second,
		Budget: 120 * time.Second, VerifierAttempt: 20 * time.Second,
		MaxRetryAfter: 10 * time.Second, MetaMaxRetries: 1, MaxVerifierCalls: 1, MinItems: 1, MaxItems: 1, MaxItemRunes: 600,
		Roles:            []Role{RoleUser, RoleEarlierReply},
		RequireCitations: true, MaxSpans: 16, MaxSpanRunes: 1600, MaxTotalRunes: 12000, MaxOutputTokens: 1024,
		ProvenanceClass: ClassUnconfirmedInterpretation},
	TaskMemoryProposal: {ID: "AT-3", Task: TaskMemoryProposal, Generative: true, Verification: VerifyAlways,
		VerifierAttempt:     30 * time.Second,
		VerificationReserve: 90 * time.Second, MaxRetryAfter: 10 * time.Second, MetaMaxRetries: 2, MaxVerifierCalls: 1,
		MinItems: 0, MaxItems: 5, MaxItemRunes: 300, RequireCitations: true, QuoteFallback: true,
		MaxSpans: 16, MaxSpanRunes: 1600, MaxTotalRunes: 16000, MaxOutputTokens: 1536,
		Roles:           []Role{RoleUser, RoleMemory},
		RoleLimits:      map[Role]RoleLimit{RoleMemory: {MaxSpans: 10}},
		ProvenanceClass: ClassUnconfirmedInterpretation},
	TaskSummaryDraft: {ID: "AT-4", Task: TaskSummaryDraft, Generative: true, Verification: VerifyAlways,
		VerifierAttempt:     30 * time.Second,
		VerificationReserve: 90 * time.Second, MaxRetryAfter: 10 * time.Second, MetaMaxRetries: 2, MaxVerifierCalls: 3,
		MinItems: 1, MaxItems: 36, MaxItemRunes: 800, RequireCitations: true, OneItemPerSpan: true, QuoteFallback: true,
		MaxSpans: 36, MaxSpanRunes: 1600, MaxTotalRunes: 40000, MaxOutputTokens: 4096,
		Roles:           []Role{RoleItem},
		ProvenanceClass: ClassAISummary},
	TaskOpeningLines: {ID: "AT-5", Task: TaskOpeningLines, Generative: true, Verification: VerifyWhenFlagged,
		Regenerate: true, RegenerateMinRemaining: 30 * time.Second,
		Budget: 120 * time.Second, VerifierAttempt: 20 * time.Second,
		MaxRetryAfter: 10 * time.Second, MetaMaxRetries: 1, MaxVerifierCalls: 1, MinItems: 1, MaxItems: 3, MaxItemRunes: 240,
		RequireCitations: true, MaxSpans: 12, MaxSpanRunes: 1600, MaxTotalRunes: 9600, MaxOutputTokens: 1024,
		Roles:           []Role{RoleItem},
		ProvenanceClass: ClassAISummary},
	TaskReadingVerification: {ID: "AT-6", Task: TaskReadingVerification, Verification: VerifyAlways,
		Budget: 10 * time.Minute, VerifierAttempt: 60 * time.Second, MaxRetryAfter: 10 * time.Second,
		MaxVerifierCalls: 1, MaxSpans: 16, MaxSpanRunes: 1600, MaxTotalRunes: 25600, Roles: []Role{RoleSource},
		ProvenanceClass: ClassReadingItem},
}

// PolicyFor returns the policy of t.
func PolicyFor(t Task) (Policy, bool) {
	p, ok := policies[t]
	return p, ok
}

// MinMetaWindow is the shortest first-content window Meta is called with
// (the AT-1 wait). With less, Meta is not called and the run ends on the
// no-AI path; a short budget is never treated as a Meta failure.
const MinMetaWindow = 20 * time.Second

// reserveMargin is kept on top of the verification reserve, so the
// reserve is still whole when Meta's window ends.
const reserveMargin = 2 * time.Second

// reserves is the time the steps after generation need: the verification
// reserve.
func (p Policy) reserves() time.Duration { return p.VerificationReserve }

// allowsRole reports whether the task accepts a span of role r.
func (p Policy) allowsRole(r Role) bool {
	for _, x := range p.Roles {
		if x == r {
			return true
		}
	}
	return false
}

// ProviderName names a provider in provenance.
type ProviderName string

// Providers.
const (
	ProviderMeta         ProviderName = "meta_modal"
	ProviderMetaAPI      ProviderName = "meta_api"
	ProviderMetaVerifier ProviderName = "meta_verifier"
	ProviderTestDouble   ProviderName = "test_double"
	ProviderNone         ProviderName = "none"
)

// Purpose is a consent purpose reserved in API_SPEC_V3 §3.
type Purpose string

// Consent purposes (ADR-0003 §8, API_SPEC_V3 §3).
const (
	PurposeAIProcessing Purpose = "ai_processing"
	// PurposePersonalMemory is the memory feature's consent (reserved in
	// API_SPEC_V3 §3 for P3). AT-3 is the memory feature itself, and
	// §3.1 lets AT-1 send confirmed memory items "only when
	// personal_memory is on": every provider call of such a run needs it,
	// checked like the other purposes before each call, after it and at
	// commit (RR-2, RR-13).
	PurposePersonalMemory Purpose = "personal_memory"
)

// AgeBand is the self-declared band of ADR-0002.
type AgeBand string

// Age bands.
const (
	Band18Plus  AgeBand = "18_plus"
	Band15To17  AgeBand = "15_17"
	BandUnknown AgeBand = "unknown"
)

// Adult reports whether the band is 18+ (unknown is treated as not 18+).
func (b AgeBand) Adult() bool { return b == Band18Plus }

// Valid reports whether b is a known band.
func (b AgeBand) Valid() bool { return b == Band18Plus || b == Band15To17 || b == BandUnknown }

// Audience returns the audience class sent to providers (never the age).
func (b AgeBand) Audience() string {
	if b.Adult() {
		return "adult"
	}
	return "young_person"
}

// Route is the generator route of an output (ADR-0003 §10).
type Route string

// Routes.
const (
	RoutePrimary Route = "primary"
	RouteNone    Route = "none"
)

// VerificationRoute is the verification route of an output.
type VerificationRoute string

// Verification routes.
const (
	VerifierRoute   VerificationRoute = "verifier"
	NoVerifierRoute VerificationRoute = "none"
)

// Risk is the generator's structured risk flag (RR-7). It can only raise
// the route.
type Risk string

// Risk levels.
const (
	RiskNone     Risk = "none"
	RiskElevated Risk = "elevated"
	RiskCrisis   Risk = "crisis"
)

func riskRank(r Risk) int {
	switch r {
	case RiskElevated:
		return 1
	case RiskCrisis:
		return 2
	}
	return 0
}

// raise returns the more severe of a and b.
func raise(a, b Risk) Risk {
	if riskRank(b) > riskRank(a) {
		return b
	}
	if a == "" {
		return RiskNone
	}
	return a
}

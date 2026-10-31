package ai

import (
	"context"
	"errors"
	"time"

	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/safety"
)

// Subject is an opaque reference to the person an operation is for. The
// router passes it only to ConsentChecker and Limiter; it is never sent to
// a provider, logged, observed or put in provenance.
type Subject string

// Consent is one purpose's state at the moment of the check.
type Consent struct {
	// Granted is true only when the purpose is granted on its current
	// policy version (an outdated grant is not granted).
	Granted bool
	// Revision is an opaque revision that changes on every grant or
	// withdrawal; the router records it and rechecks it at commit.
	Revision string
}

// ConsentChecker reads consent and the age band from the store at call
// time. The router calls it right before every provider call and again
// before a result is accepted, so a withdrawal stops future calls at once
// and discards in-flight results (RR-2, RR-13). It must not cache.
type ConsentChecker interface {
	Consent(ctx context.Context, subject Subject, purpose Purpose) (Consent, error)
	AgeBand(ctx context.Context, subject Subject) (AgeBand, error)
}

// Limit errors returned by a Limiter.
var (
	// ErrUserLimit: Sela's own per-user limit. Never treated as an outage,
	// so it never triggers a fallback (D-022).
	ErrUserLimit = errors.New("ai: per-user limit reached")
	// ErrSpendCap: a Sela daily spend ceiling. An availability failure for
	// that provider (ADR-0003 §4).
	ErrSpendCap = errors.New("ai: daily spend ceiling reached")
)

// Limiter enforces Sela's per-user limits and daily spend ceilings. Allow
// is called right before every provider call and counts it. Check reports
// what Allow would return now without counting a call: the router asks it
// before a call that depends on a later one (RR-4 i: DeepSeek writes an
// AT-1 reply only when the Claude check it needs is still allowed). A nil
// Limiter allows all.
type Limiter interface {
	Allow(ctx context.Context, subject Subject, provider ProviderName, task Task) error
	Check(ctx context.Context, subject Subject, provider ProviderName, task Task) error
}

// Screener is the deterministic crisis screen of RR-1, run on every
// user-authored span before any call.
type Screener interface {
	Screen(text string) Risk
}

// SafetyScreener screens with internal/safety (the lexicon ruleset).
type SafetyScreener struct{}

// Screen maps the safety assessment to a risk level.
func (SafetyScreener) Screen(text string) Risk {
	switch safety.Assess(text).Level {
	case safety.LevelCrisis:
		return RiskCrisis
	case safety.LevelElevated:
		return RiskElevated
	}
	return RiskNone
}

// Stage names a step in observer events.
type Stage string

// Stages.
const (
	StageScreen    Stage = "screen"
	StagePreflight Stage = "preflight"
	StageGenerate  Stage = "generate"
	StageGuard     Stage = "guard"
	StageVerify    Stage = "verify"
	StageAltVerify Stage = "alt_verify"
	StageCommit    Stage = "commit"
	StageIntegrity Stage = "integrity_alert"
	// StageOperatorAlert: Meta failed for a reason on Sela's side (its
	// admission queue, credentials, a spend ceiling or billing, a
	// certificate failure). Not an outage: no fallback (§17 O-1).
	StageOperatorAlert Stage = "operator_alert"
	StageResult        Stage = "result"
)

// Event is a metadata-only observation (DM-5): provider, task, route,
// outcome class, latency and token counts. It never carries prompts,
// spans, outputs, verdict reasons, payloads or the subject.
type Event struct {
	Task         Task         `json:"task"`
	Stage        Stage        `json:"stage"`
	Provider     ProviderName `json:"provider,omitempty"`
	Model        string       `json:"model,omitempty"`
	Route        string       `json:"route,omitempty"`
	Outcome      string       `json:"outcome"`
	Reason       string       `json:"reason,omitempty"`
	Status       int          `json:"status,omitempty"`
	Attempts     int          `json:"attempts,omitempty"`
	LatencyMS    int64        `json:"latencyMs,omitempty"`
	InputTokens  int          `json:"inputTokens,omitempty"`
	OutputTokens int          `json:"outputTokens,omitempty"`
	TestDouble   bool         `json:"testDouble,omitempty"`
}

// Observer receives metadata-only events. It is called synchronously and
// possibly concurrently, and must not block.
type Observer interface {
	Observe(Event)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(Event)

// Observe calls f.
func (f ObserverFunc) Observe(e Event) { f(e) }

// Span is one span the caller selected explicitly (DM-1): Key is the
// caller's own reference and is never sent.
type Span = minimize.Selection

// Role is a span role (ADR-0003 §3.1).
type Role = minimize.Role

// RoleLimit bounds the spans of one role in a payload (Policy.RoleLimits).
type RoleLimit = minimize.RoleLimit

// Span roles.
const (
	RoleUser         = minimize.RoleUser
	RoleEarlierReply = minimize.RoleEarlierReply
	RoleMemory       = minimize.RoleMemory
	RoleItem         = minimize.RoleItem
	RoleSource       = minimize.RoleSource
)

// DraftItem is one generated item as parsed: text and the per-request ids
// of the spans it cites.
type DraftItem struct {
	Text    string
	SpanIDs []string
}

// Validator is the task's deterministic Meaning Lock check (quote
// grounding, negation, subject, time, certainty; internal/ledger). It sees
// the parsed items and the spans as sent (redacted) and returns a
// content-free rejection code, or "" to accept. A rejection is terminal
// for the provider chain (RR-5), apart from one same-provider
// regeneration where the task allows it.
type Validator func(task Task, items []DraftItem, spans []minimize.Span) string

// ClaimInput is a human-authored claim for AT-6 (reading-content
// verification): its text and the keys of the source spans it cites.
type ClaimInput struct {
	Key      string
	Text     string
	SpanKeys []string
}

// Input is one operation's input. Only the fields a task uses are read.
type Input struct {
	Subject Subject
	// Language is "id" or "en".
	Language string
	// Spans are the explicitly selected spans, oldest first.
	Spans []Span
	// Claims are AT-6 claims.
	Claims []ClaimInput
	// PrivateNames are names the user marked private; they are redacted
	// before any call and blocked in any output.
	PrivateNames []string
	// Intent is the optional conversation intent (AT-1, D-034).
	Intent string
	// Purpose is the consultation purpose (AT-4), a short token from the
	// list owned by the summary feature.
	Purpose string
	// CrisisPaused: generation is paused for this conversation until the
	// user continues after seeing help (RR-1).
	CrisisPaused bool
	// Validate is the task's Meaning Lock check.
	Validate Validator
	// Commit rechecks the entry or conversation version (and, for
	// synthetic data, the fixture or scenario version), the crisis state
	// and the account session right before the result is accepted
	// (RR-13); any error discards the result as stale. A fallback result
	// is rechecked right after the DeepSeek call and again at commit
	// (V21), so the fallback is never attempted without it.
	Commit func(ctx context.Context) error
	// Synthetic marks the operation's data as synthetic, versioned demo or
	// test data (a fixture or demo scenario), set by server code from the
	// validated fixture, never from a browser field. While DeepSeek is
	// limited to synthetic data (Config.DeepSeekData, V21, §17 O-3) it is
	// called only for such operations, and never in production.
	Synthetic bool
	// Budget, when positive and shorter, replaces the task budget.
	Budget time.Duration
}

// Intents (D-034, D-067). The room offers two modes: "listen" and the normal
// "discuss"; the older values stay accepted and are answered in normal mode.
var intents = map[string]bool{"": true, "listen": true, "discuss": true, "sort_thoughts": true, "understand_patterns": true, "prepare_consultation": true}

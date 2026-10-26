// Package journey owns Sela v2's application rules: sessions, consent,
// deterministic safety routing, the Story Ledger and its versions, Next
// Step, Opening Practice, share grants, recipient access, the simulated
// handoff and follow-up. The model never makes these decisions (SPEC V3–V8,
// V11). HTTP concerns live in internal/httpapi.
package journey

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/safety"
)

// Error is an application error with a stable code. Message text is
// localised by the HTTP layer; Error never carries user content.
type Error struct {
	Code   string
	Status int
	Field  string
}

func (e *Error) Error() string {
	if e.Field != "" {
		return "journey: " + e.Code + " (" + e.Field + ")"
	}
	return "journey: " + e.Code
}

// Stable error codes.
const (
	CodeInvalid           = "invalid_input"
	CodeSessionMissing    = "session_missing"
	CodeStale             = "stale_version"
	CodeConsentRequired   = "consent_required"
	CodeCrisis            = "crisis_interrupted"
	CodeApprovalRequired  = "approval_required"
	CodeReviewIncomplete  = "review_incomplete"
	CodeNotFound          = "not_found"
	CodeForbidden         = "forbidden"
	CodeRateLimited       = "rate_limited"
	CodeAIDisabled        = "ai_disabled"
	CodeAIUnavailable     = "ai_unavailable"
	CodeOpInProgress      = "operation_in_progress"
	CodeInvalidTransition = "invalid_transition"
	CodeIdempotency       = "idempotency_conflict"
	CodeIdempotencyKey    = "idempotency_key_required"
	CodeCapacity          = "capacity"
	CodeContactDetails    = "contact_details_detected"
	CodePolicyOutdated    = "policy_outdated"
	CodeAccessEnded       = "access_ended"
	CodeHandoffExists     = "handoff_exists"
	CodeSlotUnavailable   = "slot_unavailable"
	CodeNoStory           = "story_required"
	CodeAdultRequired     = "adult_attestation_required"
)

func errf(code string, status int, field string) *Error {
	return &Error{Code: code, Status: status, Field: field}
}

var (
	errSession = errf(CodeSessionMissing, 401, "")
	errStale   = errf(CodeStale, 409, "expectedVersion")
	errCrisis  = errf(CodeCrisis, 409, "")
)

// Config tunes the service. Zero values get safe defaults.
type Config struct {
	// OpTimeout bounds one AI operation including queueing and cold start.
	OpTimeout time.Duration
	// MaxOpsPerWindow limits AI operations per session per OpWindow.
	MaxOpsPerWindow int
	OpWindow        time.Duration
	// MaxOutputTokens bounds model output, including the hidden reasoning of
	// a reasoning model.
	MaxOutputTokens int
	// WarmWindow: time since the last successful live call during which the
	// model is assumed warm (matches the Modal scaledown window).
	WarmWindow time.Duration
	// ColdStartEstimate is shown while a cold model is starting. Modal
	// answers 503 immediately while no container is ready, so a cold
	// operation keeps retrying for at most twice this estimate (and never
	// beyond OpTimeout).
	ColdStartEstimate time.Duration
	// WarmPollInterval is the wait between cold-start retries.
	WarmPollInterval time.Duration
	// RecipientTTL bounds a recipient principal's lifetime.
	RecipientTTL time.Duration
	// MaxCodeAttempts locks a share after this many wrong access codes.
	MaxCodeAttempts int
	Now             func() time.Time
	Logger          *slog.Logger
}

func (c *Config) defaults() {
	if c.OpTimeout <= 0 {
		c.OpTimeout = 180 * time.Second
	}
	if c.MaxOpsPerWindow <= 0 {
		c.MaxOpsPerWindow = 8
	}
	if c.OpWindow <= 0 {
		c.OpWindow = 10 * time.Minute
	}
	if c.MaxOutputTokens <= 0 {
		c.MaxOutputTokens = 3072
	}
	if c.WarmWindow <= 0 {
		c.WarmWindow = 5 * time.Minute
	}
	if c.ColdStartEstimate <= 0 {
		c.ColdStartEstimate = 3 * time.Minute
	}
	if c.WarmPollInterval <= 0 {
		c.WarmPollInterval = 10 * time.Second
	}
	if c.RecipientTTL <= 0 {
		c.RecipientTTL = 2 * time.Hour
	}
	if c.MaxCodeAttempts <= 0 {
		c.MaxCodeAttempts = 5
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
}

// Service implements the journey use cases.
type Service struct {
	cfg       Config
	store     Store
	catalog   *fixtures.Catalog
	provider  llm.Provider
	admission *llm.Admission
	log       *slog.Logger

	opsMu sync.Mutex
	ops   map[string]*opRuntime

	warmMu      sync.Mutex
	lastLiveOK  time.Time
	opsInFlight sync.WaitGroup

	metrics counters
}

// counters are process-wide, content-free event counts.
type counters struct {
	mu sync.Mutex
	m  map[string]int64
}

func (c *counters) add(name string) {
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]int64{}
	}
	c.m[name]++
	c.mu.Unlock()
}

// Metrics returns a copy of the content-free counters.
func (s *Service) Metrics() map[string]int64 {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()
	out := make(map[string]int64, len(s.metrics.m))
	for k, v := range s.metrics.m {
		out[k] = v
	}
	return out
}

// New wires a service.
func New(cfg Config, st Store, cat *fixtures.Catalog, p llm.Provider, adm *llm.Admission) *Service {
	cfg.defaults()
	if p == nil {
		p = llm.Disabled()
	}
	if adm == nil {
		adm = llm.NewAdmission(2)
	}
	return &Service{cfg: cfg, store: st, catalog: cat, provider: p, admission: adm, log: cfg.Logger, ops: map[string]*opRuntime{}}
}

// Catalog returns the fixture catalogue.
func (s *Service) Catalog() *fixtures.Catalog { return s.catalog }

// Provider returns the configured model provider's public info.
func (s *Service) ProviderInfo() llm.Info { return s.provider.Info() }

func (s *Service) now() time.Time { return s.cfg.Now() }

// Wait blocks until in-flight AI operations finish (tests and shutdown).
func (s *Service) Wait() { s.opsInFlight.Wait() }

// randomToken returns n random bytes, base64url without padding.
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("journey: crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// randomCode returns an 8-character access code like "K7Q2-M9XD" from an
// unambiguous alphabet (no 0/O/1/I/L/U), about 39 bits of entropy.
func randomCode() string {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("journey: crypto/rand failed: %v", err))
	}
	out := make([]byte, 0, 9)
	for i, v := range b {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, alphabet[int(v)%len(alphabet)])
	}
	return string(out)
}

// normalizeCode uppercases and strips separators/spaces for comparison.
func normalizeCode(c string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(c) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// HashToken returns the hex SHA-256 used to index share tokens.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func codesEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(normalizeCode(a)))
	hb := sha256.Sum256([]byte(normalizeCode(b)))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

func hashJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// update loads the session, rejects expired ones, touches LastSeen and runs fn.
func (s *Service) update(sessionID string, fn func(*Session) error) error {
	if sessionID == "" {
		return errSession
	}
	err := s.store.Update(sessionID, func(sess *Session) error {
		sess.LastSeen = s.now()
		return fn(sess)
	})
	if errors.Is(err, ErrNotFound) {
		return errSession
	}
	return err
}

func (s *Service) view(sessionID string, fn func(*Session) error) error {
	return s.update(sessionID, fn)
}

func checkVersion(sess *Session, expected int) error {
	if expected != sess.Version {
		return errStale
	}
	return nil
}

// bump increments the session version after a successful mutation.
func bump(sess *Session) { sess.Version++ }

// assessText runs the deterministic safety gate over user text and records
// the routing in the session. It returns errCrisis when the journey must stop.
func (s *Service) assessText(sess *Session, texts ...string) safety.Assessment {
	var a safety.Assessment
	a.Level = safety.LevelNone
	a.Ruleset = safety.RulesetVersion
	for _, t := range texts {
		a = safety.Max(a, safety.Assess(t))
	}
	if a.Level != safety.LevelNone {
		sess.Safety.Level = maxLevel(sess.Safety.Level, a.Level)
		sess.Safety.RuleIDs = a.RuleIDs
		sess.Safety.At = s.now()
		sess.Safety.ThirdParty = a.ThirdParty
	}
	if a.Interrupts() {
		// Crisis route (SPEC V3): stop the ordinary journey for this session.
		// The text that triggered it is not kept, nor is the draft built from
		// earlier text; existing shares stay revocable but no transition,
		// model call or plan can follow.
		sess.Safety.Interrupted = true
		sess.Story, sess.Ledger, sess.Opening = nil, nil, nil
		s.cancelSessionOps(sess, "crisis")
	}
	// Routing outcomes are health-related metadata: count them, never log
	// them per request (no session id, IP or timing correlation in logs).
	s.metrics.add("safety_" + string(a.Level))
	return a
}

func maxLevel(a, b safety.Level) safety.Level {
	rank := map[safety.Level]int{"": 0, safety.LevelNone: 0, safety.LevelElevated: 1, safety.LevelCrisis: 2}
	if rank[b] > rank[a] {
		return b
	}
	if a == "" {
		return safety.LevelNone
	}
	return a
}

func (s *Service) policy(key string) string { return s.catalog.Policies[key].Version }

// StoreStats returns content-free store counts.
func (s *Service) StoreStats() Stats { return s.store.Stats() }

package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/YanIanZ/sela/internal/ai/guard"
	"github.com/YanIanZ/sela/internal/ai/metaverifier"
	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm"
)

// Deps are the router's collaborators. The configuration is
// authoritative: a client given here is used only when the configuration
// enables its provider (SELA_VERIFIER_MODE=disabled drops the verifier;
// RR-15), and a test double given here needs AllowTestDouble outside
// production. Each slot must hold the provider it is named for (NewRouter
// refuses any other): the router takes a call's consents, age-band row,
// labels and provenance from the slot.
type Deps struct {
	// Meta is the generator (required for NewRouter; Build builds it with
	// NewMetaGenerator when unset).
	Meta Generator
	// Verifier is the Meta verifier (optional; without it verifier-gated
	// tasks abstain).
	Verifier verifier.Verifier
	// Consent is required.
	Consent ConsentChecker
	// Limits is optional (nil allows all).
	Limits Limiter
	// Screener defaults to SafetyScreener.
	Screener Screener
	// Observer is optional.
	Observer Observer
	// MetaAdmission optionally bounds concurrent Meta calls (Build only).
	MetaAdmission Admission
	// Now defaults to time.Now.
	Now func() time.Time
}

// Router routes AI tasks (ADR-0003 §5). It is safe for concurrent use.
type Router struct {
	cfg      Config
	meta     Generator
	verifier verifier.Verifier
	consent  ConsentChecker
	limits   Limiter
	screen   Screener
	obs      Observer
	now      func() time.Time
	brk      *breaker
	// spend is the verifier's estimated spend today, for SELA_VERIFIER_DAILY_USD.
	spend *spendMeter
}

// spendMeter sums a provider's estimated API cost per UTC day (ADR-0003
// §11). It counts this router's calls only; a Limiter can enforce a
// ceiling shared between instances.
type spendMeter struct {
	mu  sync.Mutex
	day string
	usd float64
}

func (m *spendMeter) add(usd float64, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d := now.UTC().Format(time.DateOnly); d != m.day {
		m.day, m.usd = d, 0
	}
	m.usd += usd
}

func (m *spendMeter) total(now time.Time) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.UTC().Format(time.DateOnly) != m.day {
		return 0
	}
	return m.usd
}

// verifierSpendReached reports whether Sela's verifier daily spend ceiling
// (SELA_VERIFIER_DAILY_USD) is reached: an availability failure of the
// verifier route (spend_cap, §4, §10.1).
func (r *Router) verifierSpendReached() bool {
	return r.cfg.VerifierDailyUSD > 0 && r.spend.total(r.now()) >= r.cfg.VerifierDailyUSD
}

// NewRouter returns a router. It checks the configuration (a known
// environment, an operation budget that holds Meta's window and the
// reserves, the age-band set; see Config.Validate), drops the clients of
// providers the configuration disables, and refuses test doubles in
// production and, elsewhere, unless AllowTestDouble is set.
func NewRouter(cfg Config, d Deps) (*Router, error) {
	if d.Meta == nil || d.Consent == nil {
		return nil, errors.New("ai: router needs a Meta generator and a consent checker")
	}
	if cfg.Profile == "" {
		cfg.Profile = ProfileFull
	}
	if cfg.OpTimeout <= 0 {
		cfg.OpTimeout = DefaultOpTimeout
	}
	if err := cfg.validateCore(); err != nil {
		return nil, err
	}
	// Each slot holds the provider it is named for. The router derives a
	// call's consents, age-band row, labels and provenance from the slot,
	// so a client in the wrong slot would be verified, consented and
	// labelled as another provider.
	if err := checkSlots(d); err != nil {
		return nil, err
	}
	// RR-15: the configured modes are authoritative, also for clients
	// passed in Deps (Build keeps those).
	if !cfg.VerifierEnabled() {
		d.Verifier = nil
	}
	var doubles []string
	if d.Meta.Identity().TestDouble {
		doubles = append(doubles, "Meta")
	}
	if d.Verifier != nil && d.Verifier.Identity().TestDouble {
		doubles = append(doubles, "Meta verifier")
	}
	if len(doubles) > 0 {
		if cfg.Production() {
			return nil, errors.New("ai: production refuses test doubles (" + strings.Join(doubles, ", ") + ")")
		}
		if !cfg.AllowTestDouble {
			return nil, errors.New("ai: test doubles (" + strings.Join(doubles, ", ") + ") require SELA_ALLOW_TEST_DOUBLE=true")
		}
	}
	r := &Router{cfg: cfg, meta: d.Meta, verifier: d.Verifier, consent: d.Consent,
		limits: d.Limits, screen: d.Screener, obs: d.Observer, now: d.Now, brk: &breaker{}, spend: &spendMeter{}}
	if r.screen == nil {
		r.screen = SafetyScreener{}
	}
	if r.now == nil {
		r.now = time.Now
	}
	return r, nil
}

// checkSlots refuses a Deps client that is not the provider of its slot:
// Meta must have the role meta_modal and the verifier the provider
// meta_verifier (a test double reports the provider it stands in for).
func checkSlots(d Deps) error {
	var bad []string
	if r := d.Meta.Identity().Role; r != ProviderMeta {
		bad = append(bad, "Meta has role "+string(r))
	}
	if d.Verifier != nil {
		if p := d.Verifier.Identity().Provider; p != metaverifier.Provider {
			bad = append(bad, "Verifier has provider "+p)
		}
	}
	if len(bad) > 0 {
		return errors.New("ai: a provider is in the wrong slot (" + strings.Join(bad, ", ") + ")")
	}
	return nil
}

// Build wires a router from configuration: Meta through the existing
// internal/llm client built from meta (NewMetaGenerator: the generator owns
// the retries and the cold-start wait), and the Meta verifier client as
// configured. Deps fields that are already set are kept, subject to the
// configured modes (see Deps). A Meta test double receives
// Config.TestDoubleAPIKey only, never the live vLLM key or the Modal
// pair in meta; production requires Meta at an https Modal host.
func Build(cfg Config, meta llm.Config, d Deps) (*Router, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if d.Meta == nil {
		g, err := buildMetaGenerator(meta, d.MetaAdmission, cfg.TestDoubleAPIKey.Reveal(), cfg.Production())
		if err != nil {
			return nil, fmt.Errorf("ai: Meta provider: %w", err)
		}
		d.Meta = g
	}
	if d.Verifier == nil && cfg.VerifierEnabled() {
		c, err := metaverifier.New(cfg.metaVerifierConfig())
		if err != nil {
			return nil, err
		}
		d.Verifier = c
	}
	return NewRouter(cfg, d)
}

// Format prints the router without secrets for every fmt verb: the
// configuration summary of Config.String, never the keys.
func (r *Router) Format(f fmt.State, _ rune) {
	if r == nil {
		_, _ = io.WriteString(f, "ai.Router(nil)")
		return
	}
	_, _ = io.WriteString(f, "ai.Router{"+r.cfg.String()+"}")
}

// ItemSource says where an output item's text comes from.
type ItemSource string

// Item sources.
const (
	SourceAIWording     ItemSource = "ai_wording"
	SourceUserQuote     ItemSource = "user_quote"
	SourceAuthoredClaim ItemSource = "authored_claim"
)

// Item is one released item.
type Item struct {
	// Text is the released text: AI wording, or the user's own quote (as
	// sent, with placeholders; see Output.Restore).
	Text   string
	Source ItemSource
	// SpanKeys are the caller's keys of the cited spans.
	SpanKeys []string
	// Verified is true when a verifier checked this item.
	Verified bool
	Verdict  verifier.Verdict
	Issues   []verifier.Category
	// Explanation is Sela's fixed explanation code (VC-9).
	Explanation string
	// CheckReason is the verifier's short reason, for "Check details"
	// only, labelled with CheckLabel. It is never advice.
	CheckReason string
	CheckLabel  string
}

// Output is a released result.
type Output struct {
	Task  Task
	Items []Item
	// SupportInfo: show support information beside the output (elevated
	// risk).
	SupportInfo bool
	// Restore maps placeholders to the original values, for the owner's
	// own view only. Never send it to a provider or another recipient,
	// never log it.
	Restore    map[string]string
	Provenance Provenance
}

// run is the state of one operation.
type run struct {
	r        *Router
	task     Task
	pol      Policy
	in       Input
	start    time.Time
	deadline time.Time
	prov     Provenance
	uses     map[Purpose]string
	roles    map[ProviderName]bool
	support  bool
	guards   guard.Result
	genID    GeneratorIdentity
	payload  minimize.Payload
	// names are the names the output guard blocks: the private names and
	// the self-introduced names of this request (minimize.Session.Names),
	// and parts the parts of private names that are everyday words, which
	// it blocks only capitalised outside a fixed phrase
	// (minimize.Session.CommonNameParts).
	names, parts []string
	// band is the age band read before the first provider call and aud
	// its audience class, which goes into every prompt and VC-1 request.
	// The band is rechecked before and after every call and at commit:
	// any change makes the run stale (ReasonBandChanged), so output
	// written and checked for one audience is never released to another.
	band AgeBand
	aud  string
	// memory: the run is the memory feature (AT-3) or its payload holds
	// confirmed memory items, so every call also needs personal_memory.
	memory bool
	// parent is the caller's context (before the operation deadline), to
	// tell a caller cancellation from the operation's own deadline.
	parent context.Context
}

var purposeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// Run executes one task (ADR-0003 §5). It returns the output with its
// provenance, or a *RunError whose Outcome says what the UI shows:
// crisis help (OutcomeCrisisHold, OutcomeSafeStop), the honest no-AI path
// (OutcomeNoAI), the manual path with the reason the check was unavailable
// (OutcomeAbstain) or a discarded stale result (OutcomeStale). AT-0 is
// never routed to a model (OutcomeModelFree).
func (r *Router) Run(ctx context.Context, task Task, in Input) (Output, error) {
	pol, ok := PolicyFor(task)
	st := &run{r: r, task: task, pol: pol, in: in, start: r.now(), uses: map[Purpose]string{}, roles: map[ProviderName]bool{},
		parent: ctx}
	st.prov = Provenance{Task: task, TaskID: pol.ID, ProvenanceClass: pol.ProvenanceClass, ReviewStatus: "unreviewed",
		Route: RouteNone, DraftProvenance: DraftUnavailable, PriceTable: PriceTableVersion, CreatedAt: st.start.UTC(),
		Guards:       GuardRecord{Version: guard.Version, Risk: RiskNone},
		Verification: VerificationRecord{Status: StatusNotRequested, Provenance: VerifiedNotRequested, Route: NoVerifierRoute}}
	st.prov.Synthetic = in.Synthetic
	if !ok {
		return Output{}, st.fail(OutcomeInvalidInput, "unknown_task")
	}
	if pol.ModelFree {
		// AT-0: static verified help, never a model, never waits (D-035).
		return Output{}, st.fail(OutcomeModelFree, "model_free")
	}
	budget := pol.Budget
	if budget == 0 {
		budget = r.cfg.OpTimeout
	}
	if in.Budget > 0 && in.Budget < budget {
		budget = in.Budget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	st.deadline, _ = ctx.Deadline()
	if in.Language != "id" && in.Language != "en" {
		return Output{}, st.fail(OutcomeInvalidInput, "language")
	}
	if !intents[in.Intent] || (in.Purpose != "" && !purposeRe.MatchString(in.Purpose)) {
		return Output{}, st.fail(OutcomeInvalidInput, "intent_or_purpose")
	}
	// §3.1: a task receives only its own span roles. A mislabelled span
	// would skip the crisis screen (RR-1 screens user-authored roles) or,
	// for AT-6, the consent and age-band checks; nothing is sent.
	for _, s := range in.Spans {
		if !pol.allowsRole(s.Role) {
			return Output{}, st.fail(OutcomeInvalidInput, ReasonSpanRole)
		}
	}
	// §3.2: a caller's budget must hold the task's reserves.
	if pol.Generative && in.Budget > 0 && in.Budget < pol.reserves() {
		return Output{}, st.fail(OutcomeInvalidInput, ReasonBudgetReserves)
	}

	// RR-1: deterministic crisis screen of every user-authored span before
	// any call. A crisis match makes zero provider calls.
	if task != TaskReadingVerification {
		if in.CrisisPaused {
			return Output{}, st.failHelp(OutcomeCrisisHold, ReasonCrisisPaused)
		}
		risk := RiskNone
		for _, s := range in.Spans {
			if s.Role.UserAuthored() {
				risk = raise(risk, r.screen.Screen(s.Text))
			}
		}
		st.prov.Guards.Risk = risk
		st.observe(Event{Stage: StageScreen, Outcome: string(risk)})
		if risk == RiskCrisis {
			return Output{}, st.failHelp(OutcomeCrisisHold, ReasonCrisisScreen)
		}
		st.support = risk == RiskElevated
	}

	// DM-1..DM-3: payload from the explicit selection only, redacted.
	sess := minimize.NewSession(in.PrivateNames)
	payload, err := minimize.Build(in.Spans, minimize.Limits{MaxSpans: pol.MaxSpans, MaxSpanRunes: pol.MaxSpanRunes,
		MaxTotalRunes: pol.MaxTotalRunes, DropOldest: pol.DropOldest, PerRole: pol.RoleLimits}, sess)
	if err != nil {
		code := "invalid_spans"
		switch {
		case errors.Is(err, minimize.ErrRoleLimit):
			code = ReasonRoleLimit
		case errors.Is(err, minimize.ErrTooLarge):
			code = "payload_too_large"
		}
		return Output{}, st.fail(OutcomeInvalidInput, code)
	}
	st.payload = payload
	st.memory = task == TaskMemoryProposal
	for _, s := range payload.Spans {
		st.memory = st.memory || s.Role == RoleMemory
	}
	// NAME-1 blocks the private names and the names the user introduced
	// themselves with in this request (learned by Build).
	st.names, st.parts = sess.Names(), sess.CommonNameParts()
	st.prov.SpansSent, st.prov.SpansDropped = len(payload.Spans), payload.Dropped
	for k, n := range sess.Counts() {
		if st.prov.Redactions == nil {
			st.prov.Redactions = map[string]int{}
		}
		st.prov.Redactions[string(k)] = n
	}

	if task == TaskReadingVerification {
		st.aud = verifier.AudienceAdult // public reading content
		return st.authoring(ctx, sess)
	}
	// The band whose audience class goes into every prompt and VC-1
	// request, read once before any call and then held to (see run.band).
	band, err := r.consent.AgeBand(ctx, in.Subject)
	if err != nil {
		return Output{}, st.halt(st.storeError(ctx), OutcomeNoAI)
	}
	st.band, st.aud = band, band.Audience()

	// RR-15: the operator kill switch stops all generation; it is not an
	// outage and never triggers the fallback.
	if r.meta.Identity().Disabled {
		return Output{}, st.fail(OutcomeNoAI, ReasonKillSwitch)
	}
	// RR-2: a verifier-gated task starts only if the Claude route is
	// permitted (Sela does not generate what it could not release).
	if pol.Verification == VerifyAlways {
		if reason := st.verifierRoute(ctx); reason != "" {
			return Output{}, st.halt(reason, OutcomeNoAI, "manual_path")
		}
	}

	draft, err := st.generate(ctx)
	if err != nil {
		return Output{}, err
	}

	var decisions []verifier.Decision
	var reqs []verifier.Request
	var refs []claimRef
	if needsVerification(pol, st.guards) && len(draft.Items) > 0 {
		reqs, refs, err = st.claims(draft)
		if err != nil {
			code := ReasonInputTooLarge
			switch {
			case errors.Is(err, errClaimsCoverage):
				code = ReasonClaimsCoverage
			case errors.Is(err, errNoUserEvidence):
				code = ReasonNoUserEvidence
			}
			st.prov.Verification.Status, st.prov.Verification.Provenance = StatusAbstained, VerifiedUnavailable
			st.prov.Verification.UnavailableReasons = []string{code}
			return Output{}, st.fail(OutcomeAbstain, code)
		}
		decisions, err = st.verify(ctx, reqs)
		if err != nil {
			return Output{}, err
		}
	} else if len(draft.Items) > 0 {
		st.prov.Verification.Status, st.prov.Verification.Provenance = StatusNotRequired, VerifiedNotRequired
	}

	out, err := st.release(draft, reqs, refs, decisions)
	if err != nil {
		return Output{}, err
	}
	// RR-13: commit re-check.
	if reason := st.commit(ctx); reason != "" {
		return Output{}, st.commitFailed(reason)
	}
	out.Restore = sess.Restore()
	out.SupportInfo = st.support
	st.finish(OutcomeReleased)
	out.Provenance = st.prov
	return out, nil
}

// fail ends the run with a typed error.
func (st *run) fail(o Outcome, reasons ...string) error {
	st.prov.Reasons = append(st.prov.Reasons, reasons...)
	st.finish(o)
	return &RunError{Outcome: o, Reasons: append([]string(nil), st.prov.Reasons...), SupportInfo: st.support, Provenance: st.prov}
}

// halt ends the run for a reason that a preflight, recheck or commit
// returned: a consent or band change is stale (RR-13), a caller
// cancellation is canceled, and any other reason ends with o, after
// prefix (a consent-store failure is consent_check_failed, never a
// consent change).
func (st *run) halt(reason string, o Outcome, prefix ...string) error {
	switch {
	case isStale(reason):
		return st.fail(OutcomeStale, reason)
	case reason == ReasonCanceled:
		return st.fail(OutcomeCanceled, reason)
	}
	return st.fail(o, append(prefix, reason)...)
}

// storeError is the reason for a failed consent or age-band read: the
// caller's cancellation (or its own deadline), the operation deadline
// (budget), or else a consent-store failure. A context-aware checker
// fails once the context ends, and that is not a consent change.
func (st *run) storeError(ctx context.Context) string {
	switch {
	case st.parent != nil && st.parent.Err() != nil:
		return ReasonCanceled
	case ctx.Err() != nil:
		return ReasonBudget
	}
	return ReasonConsentCheck
}

// commitFailed ends the run for a failed commit re-check: the output is
// discarded (stale), or not released because the consent store cannot
// confirm it, or the caller canceled.
func (st *run) commitFailed(reason string) error {
	switch reason {
	case ReasonCanceled:
		return st.fail(OutcomeCanceled, reason)
	case ReasonConsentCheck:
		return st.fail(OutcomeAbstain, reason)
	case ReasonBudget:
		return st.fail(OutcomeStale, ReasonStale)
	}
	return st.fail(OutcomeStale, reason)
}

// failHelp ends the run and asks the UI to show the inline help card.
func (st *run) failHelp(o Outcome, reasons ...string) error {
	err := st.fail(o, reasons...)
	err.(*RunError).ShowHelp = true
	return err
}

func (st *run) finish(o Outcome) {
	st.prov.Outcome = o
	st.prov.SupportInfo = st.support
	st.prov.LatencyMS = st.r.now().Sub(st.start).Milliseconds()
	st.observe(Event{Stage: StageResult, Outcome: string(o), Route: string(st.prov.Route), LatencyMS: st.prov.LatencyMS,
		InputTokens: st.prov.InputTokens, OutputTokens: st.prov.OutputTokens})
}

func (st *run) observe(e Event) {
	if st.r.obs == nil {
		return
	}
	e.Task = st.task
	st.r.obs.Observe(e)
}

func (st *run) remaining() time.Duration { return time.Until(st.deadline) }

// purposes lists the consents a call to role needs in this run.
func (st *run) purposes(role ProviderName) []Purpose { return purposesFor(role, st.task, st.memory) }

// preflight checks, right before one provider call, everything RR-2 lists:
// each consent the call needs on its current policy version, the age band
// for that provider, Sela's limits (when withLimits) and the kill
// switches. It returns "" when the call may start. A purpose granted
// earlier in this run whose revision changed or that was withdrawn
// returns ReasonConsentChanged, and a band other than the one read at the
// start ReasonBandChanged: the run is then stale. A failed read returns
// storeError's reason. No call starts once the caller canceled or the
// operation deadline passed, whatever the consent store does with its
// context.
func (st *run) preflight(ctx context.Context, role ProviderName, withLimits bool) string {
	if ctx.Err() != nil {
		return st.storeError(ctx)
	}
	r := st.r
	for _, p := range st.purposes(role) {
		c, err := r.consent.Consent(ctx, st.in.Subject, p)
		prev, seen := st.uses[p]
		switch {
		case err != nil:
			return st.storeError(ctx)
		case !c.Granted || (seen && prev != c.Revision):
			if seen {
				return ReasonConsentChanged
			}
			return ReasonNoConsent
		}
		if !seen {
			st.uses[p] = c.Revision
			st.prov.Consent = append(st.prov.Consent, ConsentUse{Purpose: p, Revision: c.Revision})
		}
	}
	if reason := st.checkBand(ctx, role); reason != "" {
		return reason
	}
	if withLimits && role == ProviderMetaVerifier && r.verifierSpendReached() {
		return ReasonSpendCap
	}
	if withLimits && r.limits != nil {
		if err := r.limits.Allow(ctx, st.in.Subject, role, st.task); err != nil {
			if errors.Is(err, ErrSpendCap) {
				return ReasonSpendCap
			}
			return ReasonUserLimit
		}
	}
	st.observe(Event{Stage: StagePreflight, Provider: role, Outcome: "ok"})
	return ""
}

// checkBand reads the age band: it must be the band read at the start of
// the run (the audience class in every prompt and request) and allow the
// provider. AT-6 carries no personal data and has no band.
func (st *run) checkBand(ctx context.Context, role ProviderName) string {
	if st.task == TaskReadingVerification {
		return ""
	}
	band, err := st.r.consent.AgeBand(ctx, st.in.Subject)
	switch {
	case err != nil:
		return st.storeError(ctx)
	case band != st.band:
		return ReasonBandChanged
	case !providerAllowed(role, band, st.r.cfg):
		return ReasonAgeBand
	}
	return ""
}

// recheck reports whether every consent a call to role uses is still
// granted on the revision recorded before the call, and the band is
// unchanged and still allows the provider ("" when all hold). A canceled
// or expired run context ends the run first (storeError), so a result
// that arrives after the caller canceled is never used.
func (st *run) recheck(ctx context.Context, role ProviderName) string {
	if ctx.Err() != nil {
		return st.storeError(ctx)
	}
	r := st.r
	for _, p := range st.purposes(role) {
		c, err := r.consent.Consent(ctx, st.in.Subject, p)
		switch {
		case err != nil:
			return st.storeError(ctx)
		case !c.Granted || c.Revision != st.uses[p]:
			return ReasonConsentChanged
		}
	}
	return st.checkBand(ctx, role)
}

// reconfirm runs after a call returns and before its result is accepted:
// every consent the call used must still be granted on the same revision,
// and the band must be unchanged and still allow the provider. It returns
// recheck's reason ("" when the result may be used).
func (st *run) reconfirm(ctx context.Context, role ProviderName) string {
	if reason := st.recheck(ctx, role); reason != "" {
		return reason
	}
	st.roles[role] = true
	return ""
}

// retryCheck returns the check that every retry of a call to role passes
// first (transport.Do for Claude and DeepSeek, the Meta generator's own
// loop): RR-2 checks "immediately before each call", and a withdrawal
// takes effect for every call not yet started (§8.2), including a retry
// after a backoff or Retry-After wait. The reason a retry was stopped is
// returned through the pointer once the call has returned. The check
// reads the store under the run's context, not the attempt's, so the end
// of one attempt's window is never taken for a consent problem.
func (st *run) retryCheck(ctx context.Context, role ProviderName) (func(context.Context) error, *string) {
	stopped := new(string)
	return func(context.Context) error {
		if reason := st.recheck(ctx, role); reason != "" {
			*stopped = reason
			return transport.NewError(string(role), transport.ReasonCanceled, 0, "retry stopped: consent or age band changed")
		}
		return nil
	}, stopped
}

// afterCall applies the post-call checks of a generator call: the
// consents and band it used are unchanged (RR-13), and a retry that the
// pre-retry check stopped ends the run. A change is stale, a caller
// cancellation canceled, and any other reason (the operation deadline, a
// consent-store failure) ends with o and that reason.
func (st *run) afterCall(ctx context.Context, role ProviderName, stopped string, o Outcome) error {
	if reason := st.reconfirm(ctx, role); reason != "" {
		return st.halt(reason, o)
	}
	if stopped != "" {
		return st.halt(stopped, o)
	}
	return nil
}

// verifierRoute reports whether the Meta verifier route is permitted for
// this user and configuration ("" when it is). SELA_VERIFIER_MODE=disabled
// always abstains (RR-15), whatever client was passed in.
func (st *run) verifierRoute(ctx context.Context) string {
	r := st.r
	switch {
	case !r.cfg.MetaVerifier.Mode.on():
		return ReasonVerifierDisabled
	case r.verifier == nil:
		return ReasonNotConfigured
	}
	return st.preflight(ctx, ProviderMetaVerifier, false)
}

// metaWindow is how long Meta may take to produce first content,
// including a cold start (§3.2): the task's fixed wait (AT-1: 20 s) or the
// time until the remaining budget equals the verification reserve, less
// reserveMargin so the reserve is whole when the window ends. ok is false
// when less than MinMetaWindow is left: Meta is then not called at all,
// and the run ends on the no-AI path. A short budget is never a Meta
// failure.
func (st *run) metaWindow() (wait time.Duration, ok bool) {
	avail := st.remaining() - reserveMargin - st.pol.VerificationReserve
	wait = avail
	if st.pol.FirstContentWait > 0 && st.pol.FirstContentWait < wait {
		wait = st.pol.FirstContentWait
	}
	return wait, wait >= MinMetaWindow
}

// genCall bounds one generator call.
type genCall struct {
	// wait is Meta's first-content window.
	wait time.Duration
	// maxRetries is the task's retry rule for Meta (§3.2).
	maxRetries int
	// timeout bounds a generator call as a whole and per attempt (zero
	// keeps the client's own attempt timeout).
	timeout time.Duration
	// noColdStartWait: the RR-14 breaker is open (Meta).
	noColdStartWait bool
}

// slotIdentity is g's identity with the role of its slot.
func slotIdentity(g Generator, _ Route) GeneratorIdentity {
	id := g.Identity()
	id.Role = ProviderMeta
	return id
}

// callGen performs one generator call and records it. It returns the
// reason the pre-retry check stopped a retry ("" if it did not).
func (st *run) callGen(ctx context.Context, g Generator, route Route, regen bool, constraints []string, c genCall) (GenerateResult, *GenerateError, string) {
	id := slotIdentity(g, route)
	system, user, err := prompt(st.task, st.in.Language, st.aud, st.in.Intent, st.in.Purpose, constraints, st.payload.Spans)
	if userJailbreak(st.payload.Spans) && !slices.Contains(st.prov.InputFlags, "jailbreak_attempt") {
		st.prov.InputFlags = append(st.prov.InputFlags, "jailbreak_attempt")
	}
	if err != nil {
		return GenerateResult{}, &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonRejected}, ""
	}
	check, stopped := st.retryCheck(ctx, id.Role)
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	res, gerr := g.Generate(ctx, GenerateRequest{Task: st.task, System: system, User: user, Schema: GenerationSchemaJSON,
		MaxTokens: st.pol.MaxOutputTokens, FirstContentWait: c.wait, MaxRetries: c.maxRetries, MaxRetryAfter: st.pol.MaxRetryAfter,
		AttemptTimeout: c.timeout, NoColdStartWait: c.noColdStartWait, BeforeAttempt: check})
	rec := GeneratorRecord{Provider: id.Label(), ModelID: id.ModelID, ServedModel: res.ServedModel,
		ServedRevision: res.ServedRevision, TestDouble: id.TestDouble, Route: route, Regeneration: regen, Outcome: "ok",
		FirstContentMS: res.FirstContent.Milliseconds(), LatencyMS: res.Latency.Milliseconds(),
		InputTokens: res.InputTokens, OutputTokens: res.OutputTokens, Attempts: res.Attempts}
	var ge *GenerateError
	if gerr != nil {
		if !errors.As(gerr, &ge) {
			ge = &GenerateError{Class: transport.ClassContent, Reason: transport.ReasonInvalidOutput}
		}
		rec.Outcome, rec.Reason, rec.Class = "failed", string(ge.Reason), string(ge.Class)
		res.Text = ""
	}
	st.prov.Calls = append(st.prov.Calls, rec)
	st.prov.InputTokens += res.InputTokens
	st.prov.OutputTokens += res.OutputTokens
	st.prov.EstimatedCostUSD += estimateCost(id.Label(), id.TestDouble, res.InputTokens, res.OutputTokens)
	st.observe(Event{Stage: StageGenerate, Provider: id.Label(), Model: id.ModelID, Route: string(route), Outcome: rec.Outcome,
		Reason: rec.Reason, Attempts: res.Attempts, LatencyMS: rec.LatencyMS, InputTokens: res.InputTokens,
		OutputTokens: res.OutputTokens, TestDouble: id.TestDouble})
	if ge != nil && ge.Class == transport.ClassIntegrity {
		st.observe(Event{Stage: StageIntegrity, Provider: id.Label(), Model: id.ModelID, Outcome: string(ge.Reason)})
	}
	return res, ge, *stopped
}

// generate runs Primary, the RR-4 check and at most one Fallback, then
// Guards (with at most one same-provider regeneration).
func (st *run) generate(ctx context.Context) (Draft, error) {
	r := st.r
	var failure *GenerateError
	// RR-14, §17 O-12: an open breaker never skips Meta. Meta is still
	// called first; only the cold-start wait is dropped, so Modal's
	// immediate 503 is the availability failure RR-4 needs.
	circuitOpen := st.pol.Interactive && r.brk.isOpen(r.now())
	if circuitOpen {
		st.prov.CircuitOpen = true
		st.prov.Reasons = append(st.prov.Reasons, ReasonCircuitOpen)
	}
	wait, ok := st.metaWindow()
	if !ok {
		// The budget cannot hold Meta's window and the reserves: the
		// honest no-AI path. Meta is not called, so this is not a Meta
		// failure and never a reason to fall back (RR-3, RR-4 a).
		return Draft{}, st.fail(OutcomeNoAI, ReasonBudget)
	}
	switch reason := st.preflight(ctx, ProviderMeta, true); reason {
	case "":
		res, ge, stopped := st.callGen(ctx, r.meta, RoutePrimary, false, nil,
			genCall{wait: wait, maxRetries: st.pol.MetaMaxRetries, noColdStartWait: circuitOpen})
		if err := st.afterCall(ctx, ProviderMeta, stopped, OutcomeNoAI); err != nil {
			return Draft{}, err
		}
		if ge == nil {
			r.brk.success()
			return st.accept(ctx, res, r.meta, RoutePrimary)
		}
		if ge.Class == transport.ClassAvailability && ge.FallbackReason != "" {
			r.brk.failure(r.now()) // an upstream Meta outage
		} else {
			r.brk.success()
		}
		failure = ge
	case ReasonSpendCap:
		// A Sela daily ceiling is an availability failure (§4), but not an
		// upstream Meta outage: no fallback (§17 O-1).
		failure = &GenerateError{Class: transport.ClassAvailability, Reason: transport.ReasonSpendCap, Cause: CauseSpendCap}
	default:
		return Draft{}, st.halt(reason, OutcomeNoAI)
	}
	switch failure.Class {
	case transport.ClassIntegrity:
		return Draft{}, st.fail(OutcomeAbstain, ReasonIntegrity, string(failure.Reason))
	case transport.ClassPolicy:
		if failure.Reason == transport.ReasonCanceled {
			return Draft{}, st.fail(OutcomeCanceled, string(failure.Reason))
		}
		if failure.Reason == transport.ReasonDisabled {
			return Draft{}, st.fail(OutcomeNoAI, ReasonKillSwitch)
		}
		return Draft{}, st.fail(OutcomeNoAI, string(failure.Reason))
	case transport.ClassContent:
		// RR-5: content failures are terminal for the provider chain.
		return Draft{}, st.fail(OutcomeAbstain, string(failure.Reason))
	}
	// Availability failures end on the honest no-AI path: there is no
	// fallback generator. An upstream outage raises an operator alert.
	primary := failure.FallbackReason
	if primary == "" {
		primary = orDefault(failure.Cause, string(failure.Reason))
	}
	if failure.FallbackReason != "" {
		st.observe(Event{Stage: StageOperatorAlert, Provider: ProviderMeta, Outcome: primary})
	}
	return Draft{}, st.fail(OutcomeNoAI, "primary_"+primary)
}

// accept parses a generation, applies the risk flag, the guards and the
// Meaning Lock, and allows one same-provider regeneration (RR-5, RR-7):
// never after a decline or a raised risk flag.
func (st *run) accept(ctx context.Context, res GenerateResult, g Generator, route Route) (Draft, error) {
	id := slotIdentity(g, route)
	failOutcome := OutcomeAbstain
	for attempt := 0; ; attempt++ {
		draft, err := parseDraft(res.Text, st.pol, st.payload)
		if err != nil {
			return Draft{}, st.fail(failOutcome, ReasonInvalidOutput)
		}
		st.prov.Guards.Risk = raise(st.prov.Guards.Risk, draft.Risk)
		if draft.Declined {
			return Draft{}, st.failHelp(OutcomeSafeStop, ReasonDeclined)
		}
		if draft.Risk == RiskCrisis {
			// As RR-1: help shown, generation paused, this output not shown.
			return Draft{}, st.failHelp(OutcomeSafeStop, ReasonRiskCrisis)
		}
		if draft.Risk == RiskElevated {
			st.support = true
		}
		var gr guard.Result
		for _, it := range draft.Items {
			gr = gr.Merge(guard.Check(it.Text, guard.Options{PrivateNames: st.names, CommonNameParts: st.parts}))
		}
		st.recordGuards(gr)
		ml := ""
		if st.in.Validate != nil {
			ml = st.in.Validate(st.task, draft.Items, st.payload.Spans)
		}
		st.observe(Event{Stage: StageGuard, Provider: id.Label(), Outcome: guardOutcome(gr, ml)})
		if !gr.Blocked() && ml == "" {
			st.guards = gr
			st.genID = id
			st.prov.Route = route
			st.prov.PromptVersion, st.prov.SchemaVersion = PromptVersion, GenerationSchemaVersion
			st.prov.DraftProvenance = draftProvenance(id)
			last := st.prov.Calls[len(st.prov.Calls)-1]
			st.prov.Generator = &last
			st.prov.Guards.Flags = flagNames(gr)
			return draft, nil
		}
		var codes []string
		if gr.Blocked() {
			st.prov.Guards.GuardRejected++
			for _, b := range gr.BlockedBy() {
				codes = append(codes, "guard:"+string(b))
			}
		}
		if ml != "" {
			st.prov.Guards.MeaningLockRejected++
			codes = append(codes, "meaning_lock:"+ml)
		}
		reason := ReasonGuardRejected
		if !gr.Blocked() {
			reason = ReasonMeaningLock
		}
		if draft.Risk == RiskElevated {
			// RR-5: a raised risk flag is terminal for the provider chain and
			// no regeneration follows it. The user's words are kept, and the
			// support information is shown with the notice (RR-7).
			return Draft{}, st.fail(OutcomeAbstain, append(append([]string{reason}, codes...), ReasonRiskElevated)...)
		}
		if !st.pol.Regenerate || attempt > 0 || st.remaining() < st.pol.RegenerateMinRemaining {
			return Draft{}, st.fail(OutcomeAbstain, append([]string{reason}, codes...)...)
		}
		// One regeneration on the same provider, with the rejection reason
		// added as a constraint. Never another provider: there is no
		// fallback generator.
		wait, ok := st.metaWindow()
		if !ok {
			return Draft{}, st.fail(OutcomeAbstain, reason, ReasonBudget)
		}
		call := genCall{wait: wait, maxRetries: st.pol.MetaMaxRetries}
		if pre := st.preflight(ctx, id.Role, true); pre != "" {
			return Draft{}, st.halt(pre, OutcomeAbstain, reason)
		}
		st.prov.Guards.Regenerations++
		next, ge, stopped := st.callGen(ctx, g, route, true, codes, call)
		if err := st.afterCall(ctx, id.Role, stopped, OutcomeAbstain); err != nil {
			return Draft{}, err
		}
		if ge != nil {
			return Draft{}, st.fail(OutcomeAbstain, reason, "regeneration_"+string(ge.Reason))
		}
		res = next
	}
}

func guardOutcome(g guard.Result, ml string) string {
	switch {
	case g.Blocked():
		return "blocked"
	case ml != "":
		return "meaning_lock_rejected"
	case g.Flagged():
		return "flagged"
	}
	return "passed"
}

func (st *run) recordGuards(g guard.Result) {
	have := map[string]bool{}
	for _, b := range st.prov.Guards.Blocked {
		have["b:"+b] = true
	}
	for _, x := range st.prov.Guards.Rules {
		have["r:"+x] = true
	}
	for _, b := range g.BlockedBy() {
		if !have["b:"+string(b)] {
			st.prov.Guards.Blocked = append(st.prov.Guards.Blocked, string(b))
		}
	}
	for _, h := range g.Hits {
		if !have["r:"+h.Rule] {
			have["r:"+h.Rule] = true
			st.prov.Guards.Rules = append(st.prov.Guards.Rules, h.Rule)
		}
	}
}

func flagNames(g guard.Result) []string {
	var out []string
	for _, x := range g.Guards() {
		if x.IsFlag() {
			out = append(out, string(x))
		}
	}
	return out
}

func draftProvenance(id GeneratorIdentity) string {
	switch {
	case id.TestDouble:
		return DraftTestDouble
	case id.Role == ProviderMeta:
		return DraftLiveMeta
	}
	return DraftUnavailable
}

// claimRef maps a claim to the draft item it checks.
type claimRef struct {
	req, item int
	claimID   string
	spanIDs   []string
}

// errClaimsCoverage: the claims would not cover the whole reply, so part
// of it would be released unchecked (RR-8, RR-11).
var errClaimsCoverage = errors.New("ai: the claims do not rebuild the reply")

// errNoUserEvidence: a claim cites no span the user (or, for AT-6, a
// licensed source) authored, only Sela's own earlier, unverified replies.
// Such a claim is never sent for verification: a model's earlier output
// is not evidence for its new output (CK-1, VC-3, RR-9 b), and the
// verifier could not tell it from the user's words.
var errNoUserEvidence = errors.New("ai: a claim has no user-authored evidence")

// errClaimTooLarge: a claim's evidence does not fit one VC-1 claim
// (MaxCitedSpans pieces); partial evidence is never sent.
var errClaimTooLarge = errors.New("ai: claim outside VC-1 limits")

// evidence reports whether a span of role r may be sent to a verifier as
// evidence: the user's own words (user turns, confirmed memory, summary
// items) and, for AT-6 only, a licensed source passage. Sela's earlier
// replies (RoleEarlierReply) never are.
func (st *run) evidence(r Role) bool {
	return r.UserAuthored() || (st.task == TaskReadingVerification && r == RoleSource)
}

// covers reports whether parts, joined, rebuild text exactly, ignoring
// whitespace.
func covers(text string, parts []string) bool {
	strip := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, s)
	}
	return strip(text) == strip(strings.Join(parts, ""))
}

// claims builds the VC-1 requests: one claim per item (AT-1: one claim per
// reply sentence, citing the user's turns, and the sentences must rebuild
// the whole reply), each citing its evidence; requests hold at most 12
// claims and 16 spans, and at most MaxVerifierCalls requests are made.
//
// Evidence is never cut: a claim's cited spans are sent whole or the run
// abstains (verification_input_too_large), because a verifier that sees
// part of the evidence may miss the span that contradicts the claim. Only
// user-authored spans are evidence (see evidence): a claim that rests on
// Sela's earlier replies alone is not verified (errNoUserEvidence). For
// AT-1 the reply answers the current (latest) user turn, which is always
// sent whole; earlier turns are added, newest first and whole, while they
// fit beside it in MaxCitedSpans pieces.
func (st *run) claims(d Draft) ([]verifier.Request, []claimRef, error) {
	type pending struct {
		item  int
		text  string
		spans []string // evidence span ids, oldest first
	}
	// Split long spans into verifier-sized pieces (VC-1: 1,600 bytes).
	pieces := map[string][]verifier.Span{}
	role := map[string]Role{}
	for _, s := range st.payload.Spans {
		role[s.ID] = s.Role
		for j, part := range splitSpan(s.Text, verifier.MaxSpanBytes) {
			id := s.ID
			if j > 0 {
				id = s.ID + "_" + itoa(j+1)
			}
			pieces[s.ID] = append(pieces[s.ID], verifier.Span{ID: id, Text: part})
		}
	}
	var ps []pending
	if st.task == TaskListeningReply {
		var user []string
		for _, s := range st.payload.Spans {
			if s.Role == RoleUser {
				user = append(user, s.ID)
			}
		}
		if len(user) == 0 {
			return nil, nil, errors.New("no user spans")
		}
		// The current turn first, whole; then earlier turns while whole
		// turns fit.
		from, n := len(user)-1, len(pieces[user[len(user)-1]])
		if n > verifier.MaxCitedSpans {
			return nil, nil, errClaimTooLarge
		}
		for from > 0 && n+len(pieces[user[from-1]]) <= verifier.MaxCitedSpans {
			from--
			n += len(pieces[user[from]])
		}
		user = user[from:]
		segs := sentences(d.Items[0].Text)
		if len(segs) == 0 || !covers(d.Items[0].Text, segs) {
			return nil, nil, errClaimsCoverage
		}
		for _, s := range segs {
			ps = append(ps, pending{item: 0, text: s, spans: user})
		}
	} else {
		for i, it := range d.Items {
			var ids []string
			for _, id := range it.SpanIDs {
				if st.evidence(role[id]) {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				return nil, nil, errNoUserEvidence
			}
			ps = append(ps, pending{item: i, text: it.Text, spans: ids})
		}
	}
	var reqs []verifier.Request
	var refs []claimRef
	cur := verifier.Request{Task: string(st.task), Language: st.in.Language, Audience: st.aud}
	used := map[string]bool{}
	flush := func() {
		if len(cur.Claims) > 0 {
			reqs = append(reqs, cur)
		}
		cur = verifier.Request{Task: string(st.task), Language: st.in.Language, Audience: st.aud}
		used = map[string]bool{}
	}
	for _, p := range ps {
		var cited []verifier.Span
		for _, sid := range p.spans {
			cited = append(cited, pieces[sid]...)
		}
		if len(cited) > verifier.MaxCitedSpans || len([]byte(p.text)) > verifier.MaxClaimBytes || len(cited) == 0 {
			// Never a partial claim or partial evidence.
			return nil, nil, errClaimTooLarge
		}
		newSpans := 0
		for _, c := range cited {
			if !used[c.ID] {
				newSpans++
			}
		}
		if len(cur.Claims) == verifier.MaxClaims || len(cur.Spans)+newSpans > verifier.MaxSpans {
			flush()
		}
		cid := "c" + itoa(len(refs)+1)
		var ids []string
		for _, c := range cited {
			if !used[c.ID] {
				used[c.ID] = true
				cur.Spans = append(cur.Spans, c)
			}
			ids = append(ids, c.ID)
		}
		cur.Claims = append(cur.Claims, verifier.Claim{ID: cid, Text: p.text, SpanIDs: ids})
		refs = append(refs, claimRef{req: len(reqs), item: p.item, claimID: cid, spanIDs: p.spans})
	}
	flush()
	if len(reqs) == 0 || len(reqs) > st.pol.MaxVerifierCalls {
		return nil, nil, errors.New("too many verifier calls")
	}
	for _, q := range reqs {
		if err := q.Validate(); err != nil {
			return nil, nil, err
		}
	}
	return reqs, refs, nil
}

// splitSpan cuts text into pieces of at most max bytes at spaces (a word
// longer than max is cut at a rune boundary).
func splitSpan(text string, max int) []string {
	if len(text) <= max {
		return []string{text}
	}
	var out []string
	for len(text) > max {
		cut := strings.LastIndex(text[:max], " ")
		if cut <= 0 {
			cut = max
			for cut > 0 && !utf8RuneStart(text[cut]) {
				cut--
			}
		}
		out = append(out, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// verify runs the verification route: Claude, and only under RR-9 the
// alternative verifier. It returns one decision per request sent or a
// RunError (abstained or verification_unavailable).
//
// A validated Claude report is never discarded: once Claude has returned
// any report for this output, a later Claude availability failure ends in
// verification_unavailable without the alternative verifier (RR-10's
// intent: no verifier shopping past a content verdict). A report that
// withholds the output stops nothing but the release: runVerifier still
// sends the remaining requests to the same verifier, so CK-9 can raise the
// route (see runVerifier).
func (st *run) verify(ctx context.Context, reqs []verifier.Request) ([]verifier.Decision, error) {
	r := st.r
	vr := &st.prov.Verification
	unavailable := func(reasons ...string) error {
		vr.Status, vr.Provenance = StatusVerificationUnavailable, VerifiedUnavailable
		vr.UnavailableReasons = append(vr.UnavailableReasons, reasons...)
		return st.fail(OutcomeAbstain, append([]string{StatusVerificationUnavailable}, vr.UnavailableReasons...)...)
	}
	abstained := func(reason string) error {
		vr.Status, vr.Provenance = StatusAbstained, VerifiedUnavailable
		vr.UnavailableReasons = append(vr.UnavailableReasons, reason)
		return st.fail(OutcomeAbstain, StatusAbstained, reason)
	}
	// notOutage: Sela's own limits, a missing route or a consent-store
	// failure (§10.1). abstained while no verifier was called for this
	// output, verification_unavailable once one was.
	notOutage := func(reason string) error {
		if st.verifierCalled() {
			return unavailable(reason)
		}
		return abstained(reason)
	}
	if reason := st.verifierRoute(ctx); reason != "" {
		if err := st.verifierEnded(reason); err != nil {
			return nil, err
		}
		return nil, abstained(reason)
	}
	if st.remaining() < st.pol.VerifierAttempt {
		return nil, abstained(ReasonBudget)
	}
	decs, reports, reason := st.runVerifier(ctx, r.verifier, VerifierRoute, reqs)
	switch reason {
	case "":
		return decs, nil
	case ReasonUserLimit, ReasonBudget, ReasonNoConsent, ReasonAgeBand, ReasonConsentCheck:
		// Sela's own limits or a missing route are not outages (§10.1).
		return nil, notOutage(reason)
	}
	if err := st.verifierEnded(reason); err != nil {
		return nil, err
	}
	verifierFailure := transport.Reason(reason) // includes spend_cap
	vr.UnavailableReasons = append(vr.UnavailableReasons, reason)
	if reports > 0 && verifierFailure.Class() == transport.ClassAvailability {
		// The verifier already checked part of this output: its findings
		// stand, and the rest is withheld. There is no second verifier.
		return nil, unavailable(ReasonPartialReport)
	}
	return nil, unavailable()
}

// verifierCalled reports whether any verifier was called for this output.
func (st *run) verifierCalled() bool {
	for _, a := range st.prov.Verification.Attempts {
		if a.Calls > 0 {
			return true
		}
	}
	return false
}

// verifierEnded ends the run for a stale result or a caller cancellation
// during verification (nil for any other reason). A canceled check still
// records its status: verification_unavailable when a verifier was
// called, abstained otherwise, with the reason "canceled".
func (st *run) verifierEnded(reason string) error {
	switch {
	case isStale(reason):
		return st.fail(OutcomeStale, reason)
	case reason == ReasonCanceled:
		vr := &st.prov.Verification
		status := StatusAbstained
		if st.verifierCalled() {
			status = StatusVerificationUnavailable
		}
		vr.Status, vr.Provenance = status, VerifiedUnavailable
		vr.UnavailableReasons = append(vr.UnavailableReasons, reason)
		return st.fail(OutcomeCanceled, status, reason)
	}
	return nil
}

// verifierLabel is the label shown with a verifier's verdicts: a test
// double always says so, whatever label it reports otherwise; a live Meta
// verifier needs none (the items say "Meta verifier", see checkLabel).
func verifierLabel(id verifier.Identity) string {
	if id.TestDouble {
		if strings.HasPrefix(id.Label, "TEST DOUBLE") {
			return id.Label
		}
		return metaverifier.TestDoubleLabel
	}
	return ""
}

// checkLabel is the CheckLabel of released items: the recorded label, or
// "Meta verifier" for the live route.
func checkLabel(vr *VerificationRecord) string {
	if vr.Label != "" {
		return vr.Label
	}
	return "Meta verifier"
}

// verifierReason classifies a verifier error. An error that is not a
// *transport.Error or a context error fails closed as invalid output (a
// content failure), so it never reaches the alternative verifier (RR-10).
func verifierReason(err error) transport.Reason {
	var te *transport.Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &te):
		return te.Reason
	case errors.Is(err, context.Canceled):
		return transport.ReasonCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return transport.ReasonTimeout
	}
	return transport.ReasonInvalidOutput
}

// runVerifier runs the requests on one verifier, in order, and records one
// VerifierAttempt. The verifier's role, consents, age-band row and label
// come from its slot (route), never from its Identity: the router checks
// the slots once (NewRouter).
//
// A safety_signal stops it at once: the route is raised and nothing more
// is sent. Any other report that withholds the whole output (an
// output-level issue, VC-8) does not stop it: the remaining requests still
// go to this verifier, and only to it, so CK-9 (the second safeguard,
// D-035) can raise the route for spans the lexicon missed. A failure on
// those requests keeps the withhold, never reaches another verifier, and
// is recorded as SafetyPassIncomplete; only a consent or band change (the
// run is stale) or a caller cancellation ends the run instead. Before any
// withholding report, any failure fails this verifier's verification
// (partial reports are never released); reports counts the validated
// reports received before it. It returns the decisions, the report count
// and "" or a reason code.
func (st *run) runVerifier(ctx context.Context, v verifier.Verifier, route VerificationRoute, reqs []verifier.Request) ([]verifier.Decision, int, string) {
	id := v.Identity()
	role, stage := ProviderMetaVerifier, StageVerify
	if id.Model == st.r.meta.Identity().ModelID {
		// A model never verifies its own output (SPEC V36): the verifier
		// slot must be configured with a different Meta model.
		return nil, 0, ReasonSelfVerification
	}
	vr := &st.prov.Verification
	label := ProviderName(role)
	if id.TestDouble {
		label = ProviderTestDouble
	}
	att := VerifierAttempt{Route: route, Provider: label, ModelID: id.Model, Label: verifierLabel(id)}
	// finish records the attempt; the top-level fields describe the
	// latest verifier, so a used alternative report never carries
	// Claude's calls or tokens.
	finish := func(outcome string) {
		att.Outcome = outcome
		vr.Attempts = append(vr.Attempts, att)
		vr.Route, vr.Provider, vr.ModelID, vr.Label = att.Route, att.Provider, att.ModelID, att.Label
		vr.ServedModel, vr.APIVersion, vr.Calls = att.ServedModel, att.APIVersion, att.Calls
		vr.LatencyMS, vr.InputTokens, vr.OutputTokens = att.LatencyMS, att.InputTokens, att.OutputTokens
	}
	var decs []verifier.Decision
	verdicts, issues := map[string]int{}, map[string]int{}
	checks := map[verifier.Check]bool{}
	// withholding: a report already withheld the output; the requests
	// left are sent for CK-9 only. incomplete is why not all of them were
	// checked.
	withholding, incomplete := false, ""
	// keepWithhold reports whether a failure only ends the CK-9 pass
	// (the withhold stands), rather than the verification.
	keepWithhold := func(reason string) bool {
		return withholding && !isStale(reason) && reason != ReasonCanceled
	}
	for _, req := range reqs {
		if st.remaining() < st.pol.VerifierAttempt {
			if keepWithhold(ReasonBudget) {
				incomplete = ReasonBudget
				break
			}
			finish(ReasonBudget)
			return nil, len(decs), ReasonBudget
		}
		if reason := st.preflight(ctx, role, true); reason != "" {
			if keepWithhold(reason) {
				incomplete = reason
				break
			}
			finish(reason)
			return nil, len(decs), reason
		}
		attempt := min(st.pol.VerifierAttempt, st.remaining())
		check, stopped := st.retryCheck(ctx, role)
		res, err := v.Verify(ctx, req, verifier.CallOptions{MaxRetryAfter: st.pol.MaxRetryAfter, AttemptTimeout: attempt, BeforeAttempt: check})
		att.Calls++
		if res.ServedModel != "" {
			att.ServedModel = res.ServedModel
		}
		if res.APIVersion != "" {
			att.APIVersion = res.APIVersion
		}
		att.LatencyMS += res.Latency.Milliseconds()
		att.InputTokens += res.InputTokens
		att.OutputTokens += res.OutputTokens
		st.prov.InputTokens += res.InputTokens
		st.prov.OutputTokens += res.OutputTokens
		cost := estimateCost(role, id.TestDouble, res.InputTokens, res.OutputTokens)
		st.prov.EstimatedCostUSD += cost
		if role == ProviderMetaVerifier {
			st.r.spend.add(cost, st.r.now()) // SELA_VERIFIER_DAILY_USD
		}
		outcome := "ok"
		if err != nil {
			outcome = string(verifierReason(err))
		}
		st.observe(Event{Stage: stage, Provider: label, Model: id.Model, Route: string(route), Outcome: outcome,
			Status: res.Status, Attempts: res.Attempts, LatencyMS: res.Latency.Milliseconds(), InputTokens: res.InputTokens,
			OutputTokens: res.OutputTokens, TestDouble: id.TestDouble})
		// The post-call recheck (RR-13) and a retry the pre-retry check
		// stopped come first: a change is stale whatever the call
		// returned. The operation deadline passing during the call is the
		// call's timeout, not a consent problem.
		fail := st.reconfirm(ctx, role)
		if fail == "" {
			fail = *stopped
		}
		if fail == ReasonBudget {
			fail = string(transport.ReasonTimeout)
		}
		if fail == "" && err != nil {
			fail = outcome
			if res.RefusalCategory != "" {
				vr.RefusalCategory = res.RefusalCategory
			}
			if transport.Reason(outcome).Class() == transport.ClassIntegrity {
				st.observe(Event{Stage: StageIntegrity, Provider: label, Model: id.Model, Outcome: outcome})
			}
		}
		if fail != "" {
			if keepWithhold(fail) {
				incomplete = fail
				break
			}
			finish(fail)
			return nil, len(decs), fail
		}
		for v, n := range res.Report.Counts() {
			verdicts[string(v)] += n
		}
		for _, is := range res.Report.Issues {
			issues[string(is.Category)]++
		}
		for _, ck := range res.Report.Checks() {
			checks[ck] = true
		}
		dec := st.decide(req, res.Report)
		decs = append(decs, dec)
		att.Reports++
		if dec.SafetySignal {
			// CK-9 raises the route at once: nothing more is sent.
			withholding = true
			break
		}
		if dec.WithholdAll {
			withholding = true
		}
	}
	vr.Status = StatusChecked
	switch {
	case id.TestDouble:
		vr.Provenance = VerifiedTestDouble
	default:
		vr.Provenance = VerifiedLiveMetaVerifier
	}
	vr.Verdicts, vr.Issues = verdicts, issues
	for _, ck := range verifier.Checks() {
		if checks[ck] {
			vr.Checks = append(vr.Checks, string(ck))
		}
	}
	vr.SafetyPassIncomplete = incomplete
	outcome := StatusChecked
	if withholding {
		outcome = "withheld"
	}
	finish(outcome)
	return decs, len(decs), ""
}

// decide applies VC-8 for one request.
func (st *run) decide(req verifier.Request, rep verifier.Report) verifier.Decision {
	quote := map[string]bool{}
	for _, c := range req.Claims {
		_, ok := st.quoteFor(c.SpanIDs)
		quote[c.ID] = st.pol.QuoteFallback && ok
	}
	return verifier.Decide(req, rep, verifier.ReleaseOptions{
		WithholdOnAnyNotReleased: st.pol.WithholdOnAnyNotReleased,
		HasQuote:                 func(id string) bool { return quote[id] },
	})
}

// quoteFor returns the user's own words (as sent) of the first cited
// user-authored span and that span's id. A quote fallback shows that one
// span under that span's key only, never under the keys of other spans
// the claim cited (AT-4 cites exactly one span per item; AT-3 may cite
// several).
func (st *run) quoteFor(spanIDs []string) (minimize.Span, bool) {
	for _, id := range spanIDs {
		base, _, _ := strings.Cut(id, "_")
		for _, s := range st.payload.Spans {
			if s.ID == base && s.Role.UserAuthored() {
				return s, true
			}
		}
	}
	return minimize.Span{}, false
}

// release builds the output from the draft and the decisions (VC-8).
func (st *run) release(d Draft, reqs []verifier.Request, refs []claimRef, decs []verifier.Decision) (Output, error) {
	out := Output{Task: st.task}
	keys := func(ids []string) []string {
		var ks []string
		for _, id := range ids {
			if k, ok := st.payload.Key(id); ok {
				ks = append(ks, k)
			}
		}
		return ks
	}
	if decs == nil {
		for _, it := range d.Items {
			out.Items = append(out.Items, Item{Text: it.Text, Source: SourceAIWording, SpanKeys: keys(it.SpanIDs)})
		}
		return out, nil
	}
	vr := &st.prov.Verification
	label := checkLabel(vr)
	byClaim := map[string]verifier.ClaimDecision{}
	withhold, safety := false, false
	var withheld []string
	for i, dec := range decs {
		if dec.SafetySignal {
			safety = true
		}
		if dec.WithholdAll {
			withhold = true
			for _, c := range dec.Withheld {
				withheld = append(withheld, string(c))
			}
		}
		for _, c := range dec.Claims {
			byClaim[itoa(i)+"/"+c.ClaimID] = c
		}
	}
	vr.Claims = len(refs)
	for _, ref := range refs {
		if ref.req >= len(decs) {
			continue // never sent: an earlier report withheld the output
		}
		cd := byClaim[itoa(ref.req)+"/"+ref.claimID]
		var iss []string
		for _, c := range cd.Issues {
			iss = append(iss, string(c))
		}
		vr.ClaimOutcomes = append(vr.ClaimOutcomes, ClaimOutcome{Verdict: string(cd.Verdict), Disposition: string(cd.Disposition),
			Issues: iss, SpansCited: len(ref.spanIDs)})
	}
	if safety {
		// CK-9 raises the route: help card shown, output withheld.
		st.prov.Guards.Risk = raise(st.prov.Guards.Risk, RiskElevated)
		st.support = true
		vr.WithheldAll = true
		return Output{}, st.failHelp(OutcomeSafeStop, ReasonSafetySignal)
	}
	if withhold {
		vr.WithheldAll = true
		if vr.SafetyPassIncomplete != "" {
			withheld = append(withheld, ReasonSafetyPassIncomplete)
		}
		return Output{}, st.fail(OutcomeAbstain, append([]string{ReasonWithheld}, withheld...)...)
	}
	if len(decs) != len(reqs) {
		// Only a withholding report stops verification early; anything
		// else unchecked is never released (RR-11).
		vr.WithheldAll = true
		return Output{}, st.fail(OutcomeAbstain, ReasonClaimsCoverage)
	}
	if st.task == TaskListeningReply {
		// Every sentence was supported (otherwise the reply is withheld),
		// and the checked sentences must rebuild the whole reply: nothing
		// is released that was not checked.
		it := d.Items[0]
		var checked []string
		for _, ref := range refs {
			for _, c := range reqs[ref.req].Claims {
				if c.ID == ref.claimID {
					checked = append(checked, c.Text)
				}
			}
		}
		if !covers(it.Text, checked) {
			vr.WithheldAll = true
			return Output{}, st.fail(OutcomeAbstain, ReasonClaimsCoverage)
		}
		vr.Released = len(refs)
		out.Items = append(out.Items, Item{Text: it.Text, Source: SourceAIWording, SpanKeys: keys(it.SpanIDs), Verified: true,
			Verdict: verifier.Supported, Explanation: verifier.ExplainSupported, CheckLabel: label})
		return out, nil
	}
	for _, ref := range refs {
		cd := byClaim[itoa(ref.req)+"/"+ref.claimID]
		it := d.Items[ref.item]
		item := Item{SpanKeys: keys(ref.spanIDs), Verified: true, Verdict: cd.Verdict, Issues: cd.Issues,
			Explanation: cd.Explanation, CheckReason: cd.Reason, CheckLabel: label}
		switch cd.Disposition {
		case verifier.Release:
			vr.Released++
			item.Text, item.Source = it.Text, SourceAIWording
		case verifier.QuoteFallback:
			vr.QuoteFallback++
			q, _ := st.quoteFor(ref.spanIDs)
			item.Text, item.Source, item.SpanKeys = q.Text, SourceUserQuote, keys([]string{q.ID})
		default:
			vr.Dropped++
			continue
		}
		out.Items = append(out.Items, item)
	}
	if len(out.Items) == 0 {
		return Output{}, st.fail(OutcomeAbstain, ReasonNothingReleased)
	}
	return out, nil
}

// commit is the RR-13 re-check right before the result is stored or
// shown: the caller's cancellation, the operation deadline, every consent
// revision used, the band (unchanged since the start, and allowing every
// provider used) and the caller's version, crisis and session checks.
// See commitFailed for the outcome of each reason.
func (st *run) commit(ctx context.Context) string {
	if st.parent != nil && st.parent.Err() != nil {
		return ReasonCanceled
	}
	if ctx.Err() != nil {
		return ReasonStale
	}
	r := st.r
	for p, rev := range st.uses {
		c, err := r.consent.Consent(ctx, st.in.Subject, p)
		switch {
		case err != nil:
			return st.storeError(ctx)
		case !c.Granted || c.Revision != rev:
			return ReasonConsentChanged
		}
	}
	if st.task != TaskReadingVerification && len(st.roles) > 0 {
		band, err := r.consent.AgeBand(ctx, st.in.Subject)
		switch {
		case err != nil:
			return st.storeError(ctx)
		case band != st.band:
			return ReasonBandChanged
		}
		for role := range st.roles {
			if !providerAllowed(role, band, r.cfg) {
				return ReasonAgeBand
			}
		}
	}
	if st.in.Commit != nil {
		if err := st.in.Commit(ctx); err != nil {
			return ReasonStale
		}
	}
	st.observe(Event{Stage: StageCommit, Outcome: "ok"})
	return ""
}

// authoring runs AT-6: a human author wrote the claims, the Meta verifier
// checks each against the licensed source passages, and nothing is
// published without a check.
func (st *run) authoring(ctx context.Context, sess *minimize.Session) (Output, error) {
	if len(st.in.Claims) == 0 || len(st.in.Claims) > verifier.MaxClaims*st.pol.MaxVerifierCalls {
		return Output{}, st.fail(OutcomeInvalidInput, "claims")
	}
	if reason := st.verifierRoute(ctx); reason != "" {
		if err := st.verifierEnded(reason); err != nil {
			return Output{}, err
		}
		st.prov.Verification.Status, st.prov.Verification.Provenance = StatusAbstained, VerifiedUnavailable
		st.prov.Verification.UnavailableReasons = []string{reason}
		return Output{}, st.fail(OutcomeAbstain, StatusAbstained, reason)
	}
	byKey := map[string]string{}
	for _, s := range st.payload.Spans {
		if k, ok := st.payload.Key(s.ID); ok {
			byKey[k] = s.ID
		}
	}
	d := Draft{Risk: RiskNone}
	for _, c := range st.in.Claims {
		var ids []string
		for _, k := range c.SpanKeys {
			id, ok := byKey[k]
			if !ok {
				return Output{}, st.fail(OutcomeInvalidInput, "claim_cites_unknown_span")
			}
			ids = append(ids, id)
		}
		text := strings.TrimSpace(sess.Redact(c.Text))
		if text == "" || len(ids) == 0 {
			return Output{}, st.fail(OutcomeInvalidInput, "claims")
		}
		d.Items = append(d.Items, DraftItem{Text: text, SpanIDs: ids})
	}
	st.genID = GeneratorIdentity{Role: ProviderNone}
	reqs, refs, err := st.claims(d)
	if err != nil {
		return Output{}, st.fail(OutcomeInvalidInput, "claims")
	}
	decs, _, reason := st.runVerifier(ctx, st.r.verifier, VerifierRoute, reqs)
	if err := st.verifierEnded(reason); err != nil {
		return Output{}, err
	}
	if reason != "" {
		vr := &st.prov.Verification
		switch reason {
		case ReasonBudget, ReasonUserLimit:
			// §10.1: Sela's own operation or per-user limit stopped the
			// call; it is not an outage and no verifier was called.
			vr.Status, vr.Provenance = StatusAbstained, VerifiedUnavailable
			vr.UnavailableReasons = []string{reason}
		default:
			vr.Status, vr.Provenance = StatusVerificationUnavailable, VerifiedUnavailable
			vr.UnavailableReasons = []string{reason, "alternative_not_permitted_human_authored"}
		}
		return Output{}, st.fail(OutcomeAbstain, vr.Status, reason)
	}
	vr := &st.prov.Verification
	vr.Claims = len(refs)
	out := Output{Task: st.task}
	byClaim := map[string]verifier.ClaimDecision{}
	withhold, safety := false, false
	var withheld []string
	for i, dec := range decs {
		safety = safety || dec.SafetySignal
		if dec.WithholdAll {
			withhold = true
			for _, c := range dec.Withheld {
				withheld = append(withheld, string(c))
			}
		}
		for _, c := range dec.Claims {
			byClaim[itoa(i)+"/"+c.ClaimID] = c
		}
	}
	for _, ref := range refs {
		if ref.req >= len(decs) {
			continue // never sent: an earlier report withheld the item
		}
		cd := byClaim[itoa(ref.req)+"/"+ref.claimID]
		var iss []string
		for _, c := range cd.Issues {
			iss = append(iss, string(c))
		}
		vr.ClaimOutcomes = append(vr.ClaimOutcomes, ClaimOutcome{Verdict: string(cd.Verdict), Disposition: string(cd.Disposition),
			Issues: iss, SpansCited: len(ref.spanIDs)})
	}
	// VC-8, §3 AT-6: an output-level issue or a safety signal withholds
	// the whole reading item, which stays unpublished; so does a report
	// that stopped the check early.
	if safety || withhold || len(decs) != len(reqs) {
		vr.WithheldAll = true
		reasons := []string{ReasonWithheld}
		if safety {
			reasons = append(reasons, ReasonSafetySignal)
		}
		if vr.SafetyPassIncomplete != "" {
			withheld = append(withheld, ReasonSafetyPassIncomplete)
		}
		return Output{}, st.fail(OutcomeAbstain, append(reasons, withheld...)...)
	}
	for _, ref := range refs {
		cd := byClaim[itoa(ref.req)+"/"+ref.claimID]
		if cd.Disposition == verifier.Release {
			vr.Released++
		} else {
			vr.Dropped++
		}
		var ks []string
		for _, id := range ref.spanIDs {
			if k, ok := st.payload.Key(id); ok {
				ks = append(ks, k)
			}
		}
		out.Items = append(out.Items, Item{Text: d.Items[ref.item].Text, Source: SourceAuthoredClaim, SpanKeys: ks, Verified: true,
			Verdict: cd.Verdict, Issues: cd.Issues, Explanation: cd.Explanation, CheckReason: cd.Reason,
			CheckLabel: checkLabel(vr)})
	}
	if reason := st.commit(ctx); reason != "" {
		return Output{}, st.commitFailed(reason)
	}
	st.prov.DraftProvenance = DraftManual
	st.finish(OutcomeReleased)
	out.Provenance = st.prov
	return out, nil
}

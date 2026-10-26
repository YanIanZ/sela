package ai

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YanIanZ/sela/internal/ai/deepseek"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/llm"
)

// GeneratorIdentity describes a generator without secrets.
type GeneratorIdentity struct {
	// Role is ProviderMeta or ProviderDeepSeek, whatever serves it.
	Role ProviderName
	// ModelID and Revision are the configured model.
	ModelID  string
	Revision string
	// TestDouble marks a labelled test double.
	TestDouble bool
	// Disabled is the operator kill switch (SELA_LLM_MODE=disabled): not
	// an outage, so it never triggers the fallback (RR-15).
	Disabled bool
}

// Label returns the provenance provider name.
func (g GeneratorIdentity) Label() ProviderName {
	if g.TestDouble {
		return ProviderTestDouble
	}
	return g.Role
}

// GenerateRequest is one generation call.
type GenerateRequest struct {
	Task      Task
	System    string
	User      string
	Schema    string
	MaxTokens int
	// FirstContentWait bounds the wait for the first content, including a
	// cold start (Meta; zero means no separate bound).
	FirstContentWait time.Duration
	// MaxRetries is the number of retries after the first attempt for a
	// connection failure, 429 or 5xx before any content (Meta: the task's
	// rule, ADR-0003 §3.2). Modal cold-start polls do not count. Zero
	// means none; DeepSeek keeps its client default.
	MaxRetries int
	// MaxRetryAfter bounds a provider Retry-After still waited for; a
	// longer one ends the retries.
	MaxRetryAfter time.Duration
	// AttemptTimeout bounds each attempt (DeepSeek; zero keeps the client
	// default).
	AttemptTimeout time.Duration
	// BeforeAttempt runs right before every attempt that may start after
	// a wait: Meta runs it before every attempt, the first included (an
	// admission-queue wait can last the whole first-content window), and
	// before every retry or cold-start poll; DeepSeek before every retry.
	// An error stops the call before anything is sent (the router's
	// consent and age-band check, RR-2 and §8.2).
	BeforeAttempt func(ctx context.Context) error
}

// GenerateResult is a completed generation.
type GenerateResult struct {
	Text           string
	ServedModel    string
	ServedRevision string
	FirstContent   time.Duration
	Latency        time.Duration
	InputTokens    int
	OutputTokens   int
	Attempts       int
}

// Fallback reasons (ADR-0003 §10).
const (
	FallbackColdStart   = "cold_start_budget"
	FallbackUnavailable = "unavailable"
	FallbackTimeout     = "timeout"
	FallbackCapacity    = "capacity"
	FallbackAuth        = "auth"
)

// GenerateError classifies a failed generation (ADR-0003 §4).
type GenerateError struct {
	Class  transport.Class
	Reason transport.Reason
	// FallbackReason is set for availability failures.
	FallbackReason string
	// ContentStarted: content had been received; nothing after it may fall
	// back.
	ContentStarted bool
	Status         int
}

func (e *GenerateError) Error() string {
	return "ai: generation failed: " + string(e.Class) + "/" + string(e.Reason)
}

// Generator produces one generation. Implementations are safe for
// concurrent use; errors are *GenerateError.
type Generator interface {
	Identity() GeneratorIdentity
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
}

// Admission bounds concurrent Meta calls (llm.Admission satisfies it).
type Admission interface {
	Acquire(ctx context.Context) (release func(), err error)
}

// DefaultWarmPollInterval is how often a Modal "no container ready" 503
// is retried during a cold start (v2 journey.generate, WarmPollInterval).
const DefaultWarmPollInterval = 10 * time.Second

// defaultMetaMaxRetryAfter is internal/llm's own bound on a Retry-After.
const defaultMetaMaxRetryAfter = 10 * time.Second

// NewMetaGenerator builds the Meta generator from the internal/llm
// configuration (Meta on Modal, or its labelled test double) without
// changing the llm API. The generator owns the retries: the llm client is
// built with its own retries off, so every attempt (the first one
// included, after any admission wait), retry and cold-start poll
//   - first passes the router's consent and age-band check (RR-2: right
//     before each call; §8.2: a withdrawal stops every call not yet
//     started);
//   - follows the task's rule (§3.2): AT-1 one retry and a Retry-After of
//     at most 3 s, AT-2/AT-5 one retry and 10 s, AT-3/AT-4 the v2 two
//     retries; a longer Retry-After ends the call with its own reason;
//   - in modal mode, polls a 503 that arrives before content (Modal: no
//     container ready) every 10 s, as v2 journey.generate does, until the
//     first-content window ends, and only then reports
//     availability/cold_start_budget.
//
// It also bounds the wait for first content, classifies llm error kinds
// into failure classes and records whether content had started. The
// HTTP client is wrapped only to read each response's status and
// Retry-After; requests and bodies are not touched or logged.
//
// A test double (llm.ModeTestDouble) never receives a live credential and
// never runs at a live host, as for Claude and DeepSeek: its base URL
// must be a loopback, local or test-double host and never a Modal host,
// and the vLLM key and the Modal-Key/Modal-Secret pair are cleared
// whatever the caller filled in (v2 cmd/sela fills them from the
// environment in every mode). Build sends it Config.TestDoubleAPIKey
// only.
func NewMetaGenerator(cfg llm.Config, adm Admission) (Generator, error) {
	return buildMetaGenerator(cfg, adm, "", false)
}

// modalHostSuffixes are the hosts Modal serves Sela's Meta deployment at:
// web endpoints (*.modal.run) and Modal Servers (*.modal.direct, see
// internal/llm.Config.BaseURL).
var modalHostSuffixes = []string{".modal.run", ".modal.direct"}

func modalHost(u *url.URL) bool {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	for _, s := range modalHostSuffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

// buildMetaGenerator is NewMetaGenerator with the key a test double may
// receive (Config.TestDoubleAPIKey) and, for production, the requirement
// of an https Modal host in modal mode.
//
// Modal (live) mode never runs at a local or test-double host, in any
// environment, as the live Claude and DeepSeek clients
// (transport.ParseBaseURL): the live vLLM key, the Modal-Key/Modal-Secret
// pair and user content would reach that host before any served-model
// check could fail (review round 3; round 2 Finding 17 through
// mode=modal).
func buildMetaGenerator(cfg llm.Config, adm Admission, testDoubleKey string, production bool) (Generator, error) {
	switch cfg.Mode {
	case llm.ModeTestDouble:
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || u.Host == "" || !transport.TestDoubleLike(u) || modalHost(u) {
			return nil, errors.New("ai: a Meta test double must run at a loopback, local or test-double host, never at a Modal or other live host")
		}
		// Never a live credential to a test double.
		cfg.APIKey, cfg.ModalKey, cfg.ModalSecret = testDoubleKey, "", ""
	case llm.ModeModal:
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || u.Host == "" || transport.TestDoubleLike(u) {
			return nil, errors.New("ai: Meta in modal mode must not run at a local or test-double host")
		}
		if production && (u.Scheme != "https" || !modalHost(u) || (u.Port() != "" && u.Port() != "443")) {
			return nil, errors.New("ai: production requires Meta at an https Modal host (*.modal.run or *.modal.direct)")
		}
	}
	return newMetaFromLLM(cfg, adm)
}

// newMetaFromLLM builds the generator from an llm configuration that has
// passed buildMetaGenerator's host and credential checks (tests in this
// package call it directly for a local Modal-like TLS server).
func newMetaFromLLM(cfg llm.Config, adm Admission) (Generator, error) {
	cfg.MaxRetries = -1
	cfg.HTTPClient = observingClient(cfg.HTTPClient)
	p, err := llm.New(cfg)
	if err != nil {
		return nil, err
	}
	return newMetaGenerator(p, adm, DefaultWarmPollInterval), nil
}

// newMetaGenerator wraps a provider whose own retries are off.
func newMetaGenerator(p llm.Provider, adm Admission, poll time.Duration) *metaGenerator {
	return &metaGenerator{p: p, adm: adm, poll: poll}
}

type metaGenerator struct {
	p    llm.Provider
	adm  Admission
	poll time.Duration
}

func (m *metaGenerator) Identity() GeneratorIdentity {
	info := m.p.Info()
	return GeneratorIdentity{
		Role: ProviderMeta, ModelID: info.ModelID, Revision: info.Revision,
		TestDouble: info.Mode == llm.ModeTestDouble, Disabled: info.Mode == llm.ModeDisabled,
	}
}

// Format prints the generator without the llm client's credentials, for
// every fmt verb.
func (m *metaGenerator) Format(f fmt.State, _ rune) {
	id := m.Identity()
	_, _ = fmt.Fprintf(f, "ai.metaGenerator{model=%s testDouble=%t disabled=%t}", id.ModelID, id.TestDouble, id.Disabled)
}

// attemptNote receives the status and Retry-After of the last HTTP
// response of one Complete call (observingTransport).
type attemptNote struct {
	mu         sync.Mutex
	status     int
	retryAfter time.Duration
}

func (n *attemptNote) set(status int, retryAfter time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status, n.retryAfter = status, retryAfter
}

func (n *attemptNote) get() (int, time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status, n.retryAfter
}

type attemptNoteKey struct{}

func withAttemptNote(ctx context.Context, n *attemptNote) context.Context {
	return context.WithValue(ctx, attemptNoteKey{}, n)
}

// observingTransport records each response's status and Retry-After into
// the request context's attemptNote. It never changes the request or the
// response and reads no body.
type observingTransport struct{ base http.RoundTripper }

func (t observingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if resp != nil {
		if n, ok := req.Context().Value(attemptNoteKey{}).(*attemptNote); ok {
			n.set(resp.StatusCode, transport.ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()))
		}
	}
	return resp, err
}

func observingClient(hc *http.Client) *http.Client {
	var c http.Client
	if hc != nil {
		c = *hc
	}
	base := c.Transport
	if base == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConnsPerHost = 16
		base = t
	}
	c.Transport = observingTransport{base: base}
	return &c
}

var errFirstContentBudget = errors.New("ai: no content within the first-content budget")

// first-content states.
const (
	waiting int32 = iota
	started
	expired
)

func (m *metaGenerator) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	start := time.Now()
	id := m.Identity()
	if id.Disabled {
		return GenerateResult{}, &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonDisabled}
	}
	wctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var state atomic.Int32
	var windowEnd time.Time
	if req.FirstContentWait > 0 {
		windowEnd = start.Add(req.FirstContentWait)
		t := time.AfterFunc(req.FirstContentWait, func() {
			if state.CompareAndSwap(waiting, expired) {
				cancel(errFirstContentBudget)
			}
		})
		defer t.Stop()
	}
	if m.adm != nil {
		release, err := m.adm.Acquire(wctx)
		if err != nil {
			if state.Load() == expired {
				return GenerateResult{Latency: time.Since(start)}, &GenerateError{Class: transport.ClassAvailability,
					Reason: transport.ReasonUnavailable, FallbackReason: FallbackCapacity}
			}
			return GenerateResult{Latency: time.Since(start)}, ctxGenerateError(ctx)
		}
		defer release()
	}
	llmReq := llm.Request{
		Operation:         "ai_" + string(req.Task),
		Messages:          []llm.Message{{Role: llm.RoleSystem, Content: req.System}, {Role: llm.RoleUser, Content: req.User}},
		JSONSchema:        req.Schema,
		SchemaName:        "sela_ai_generation",
		MaxTokens:         req.MaxTokens,
		ReasoningStrength: llm.ReasoningLow,
	}
	onDelta := func(string) error {
		if state.CompareAndSwap(waiting, started) || state.Load() == started {
			return nil
		}
		return errFirstContentBudget
	}
	maxAfter := req.MaxRetryAfter
	if maxAfter <= 0 {
		maxAfter = defaultMetaMaxRetryAfter
	}
	modal := m.p.Info().Mode == llm.ModeModal
	deadline, hasDeadline := ctx.Deadline()
	// fits reports whether a wait of d ends inside the first-content
	// window and the operation deadline.
	fits := func(d time.Duration) bool {
		end := time.Now().Add(d)
		if !windowEnd.IsZero() && !end.Before(windowEnd) {
			return false
		}
		return !hasDeadline || end.Before(deadline)
	}
	// A cold start is waited for only inside a bound (the window or the
	// operation deadline); without one a 503 is an ordinary failure.
	bounded := !windowEnd.IsZero() || hasDeadline
	var meta GenerateResult
	attempts, retries, retried500 := 0, 0, false
	for {
		// RR-2, §8.2: the check runs right before every attempt, the first
		// one included. The router's preflight ran before Generate, but the
		// admission queue above may have held the call for most of the
		// first-content window: a withdrawal or band change during that
		// wait must stop a call that has not started.
		if req.BeforeAttempt != nil {
			if err := req.BeforeAttempt(wctx); err != nil {
				meta.Latency = time.Since(start)
				return meta, &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonCanceled}
			}
		}
		note := &attemptNote{retryAfter: -1}
		res, err := m.p.Complete(withAttemptNote(wctx, note), llmReq, onDelta)
		attempts += max(res.Attempts, 1)
		meta = GenerateResult{Latency: time.Since(start), FirstContent: res.FirstToken, Attempts: attempts,
			InputTokens: res.PromptTokens, OutputTokens: res.CompletionTokens}
		if err == nil {
			meta.Text = res.Text
			meta.ServedModel = res.Model
			meta.ServedRevision = id.Revision
			return meta, nil
		}
		ge := classifyMeta(ctx, err, state.Load())
		if state.Load() != waiting || ge.Class != transport.ClassAvailability || ctx.Err() != nil {
			return meta, ge
		}
		status, retryAfter := note.get()
		if ge.Status != 0 {
			status = ge.Status
		}
		var delay time.Duration
		switch {
		case modal && bounded && llm.KindOf(err) == llm.KindUnavailable && status == http.StatusServiceUnavailable:
			// Modal: no container ready yet. Wait for the cold start
			// while the window lasts (v2 journey.generate); not a retry.
			delay = m.poll
			if !fits(delay) {
				return meta, &GenerateError{Class: transport.ClassAvailability, Reason: transport.ReasonTimeout,
					FallbackReason: FallbackColdStart, Status: status}
			}
		case retryableMeta(err, status, &retried500) && retries < req.MaxRetries:
			if retryAfter > maxAfter {
				return meta, ge // the server asks for longer than the task allows
			}
			delay = transport.Backoff(retries + 1)
			if retryAfter >= 0 {
				delay = retryAfter
			}
			if !fits(delay) {
				return meta, ge
			}
			retries++
		default:
			return meta, ge
		}
		t := time.NewTimer(delay)
		select {
		case <-wctx.Done():
			t.Stop()
			meta.Latency = time.Since(start)
			return meta, classifyMeta(ctx, context.Cause(wctx), state.Load())
		case <-t.C:
		}
	}
}

// retryableMeta mirrors internal/llm's retry rule before content: a
// connection failure or attempt timeout (never a certificate failure),
// 429, 502, 503, 504, and 500 once.
func retryableMeta(err error, status int, retried500 *bool) bool {
	switch llm.KindOf(err) {
	case llm.KindRateLimited:
		return true
	case llm.KindTimeout:
		return status == 0
	case llm.KindUnavailable:
		switch status {
		case 0:
			var certErr *tls.CertificateVerificationError
			return !errors.As(err, &certErr)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		case http.StatusInternalServerError:
			if *retried500 {
				return false
			}
			*retried500 = true
			return true
		}
	}
	return false
}

// classifyMeta maps an llm error to a failure class (ADR-0003 §4).
func classifyMeta(ctx context.Context, err error, state int32) *GenerateError {
	status := 0
	var le *llm.Error
	if errors.As(err, &le) {
		status = le.Status
	}
	kind := llm.KindOf(err)
	if kind == llm.KindModelMismatch {
		return &GenerateError{Class: transport.ClassIntegrity, Reason: transport.ReasonModelMismatch, Status: status,
			ContentStarted: state == started}
	}
	if state == expired {
		// Meta produced no content before the remaining budget reached
		// the reserves: "cold start beyond budget".
		return &GenerateError{Class: transport.ClassAvailability, Reason: transport.ReasonTimeout,
			FallbackReason: FallbackColdStart, Status: status}
	}
	if ctx.Err() != nil {
		e := ctxGenerateError(ctx)
		e.ContentStarted = state == started
		return e
	}
	if state == started {
		// Any failure after the first content delta is content-class.
		return &GenerateError{Class: transport.ClassContent, Reason: transport.ReasonInvalidOutput, ContentStarted: true, Status: status}
	}
	ge := &GenerateError{Status: status}
	switch kind {
	case llm.KindUnavailable:
		ge.Class, ge.Reason, ge.FallbackReason = transport.ClassAvailability, transport.ReasonUnavailable, FallbackUnavailable
	case llm.KindRateLimited:
		ge.Class, ge.Reason, ge.FallbackReason = transport.ClassAvailability, transport.ReasonRateLimited, FallbackUnavailable
	case llm.KindAuth:
		ge.Class, ge.Reason, ge.FallbackReason = transport.ClassAvailability, transport.ReasonAuth, FallbackAuth
	case llm.KindTimeout:
		ge.Class, ge.Reason, ge.FallbackReason = transport.ClassAvailability, transport.ReasonTimeout, FallbackTimeout
	case llm.KindDisabled:
		ge.Class, ge.Reason = transport.ClassPolicy, transport.ReasonDisabled
	case llm.KindRejected:
		ge.Class, ge.Reason = transport.ClassPolicy, transport.ReasonRejected
	case llm.KindCanceled:
		ge.Class, ge.Reason = transport.ClassPolicy, transport.ReasonCanceled
	case llm.KindOutputTooLong:
		ge.Class, ge.Reason = transport.ClassContent, transport.ReasonTruncated
	default:
		// KindProtocol (malformed or empty response) fails closed: content
		// class, never a fallback.
		ge.Class, ge.Reason = transport.ClassContent, transport.ReasonInvalidOutput
	}
	return ge
}

func ctxGenerateError(ctx context.Context) *GenerateError {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &GenerateError{Class: transport.ClassAvailability, Reason: transport.ReasonTimeout, FallbackReason: FallbackTimeout}
	}
	return &GenerateError{Class: transport.ClassPolicy, Reason: transport.ReasonCanceled}
}

// NewDeepSeekGenerator wraps a DeepSeek client as the fallback generator.
func NewDeepSeekGenerator(c *deepseek.Client) Generator { return &deepseekGenerator{c: c} }

type deepseekGenerator struct{ c *deepseek.Client }

func (g *deepseekGenerator) Identity() GeneratorIdentity {
	return GeneratorIdentity{Role: ProviderDeepSeek, ModelID: g.c.Model(), TestDouble: g.c.TestDouble()}
}

// Format prints the generator without the client's secrets.
func (g *deepseekGenerator) Format(f fmt.State, verb rune) {
	_, _ = io.WriteString(f, "ai.deepseekGenerator{")
	g.c.Format(f, verb)
	_, _ = io.WriteString(f, "}")
}

func (g *deepseekGenerator) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	comp, err := g.c.Complete(ctx, deepseek.Request{System: req.System, User: req.User, MaxTokens: req.MaxTokens,
		MaxRetryAfter: req.MaxRetryAfter, AttemptTimeout: req.AttemptTimeout, BeforeAttempt: req.BeforeAttempt})
	res := GenerateResult{Latency: comp.Latency, InputTokens: comp.InputTokens, OutputTokens: comp.OutputTokens,
		Attempts: comp.Attempts, ServedModel: comp.ServedModel}
	if err != nil {
		reason := transport.ReasonOf(err)
		ge := &GenerateError{Class: reason.Class(), Reason: reason, Status: comp.Status}
		if ge.Class == transport.ClassAvailability {
			ge.FallbackReason = FallbackUnavailable
		}
		return res, ge
	}
	res.Text = comp.Text
	res.FirstContent = comp.Latency
	return res, nil
}

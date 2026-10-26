package ai

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
)

// consentFake is an in-memory ConsentChecker that records which purposes
// were checked since the last provider call.
type consentFake struct {
	mu      sync.Mutex
	granted map[Purpose]bool
	rev     map[Purpose]string
	band    AgeBand
	calls   int
	checked map[Purpose]bool
	err     error
}

func newConsent(purposes ...Purpose) *consentFake {
	c := &consentFake{granted: map[Purpose]bool{}, rev: map[Purpose]string{}, band: Band18Plus, checked: map[Purpose]bool{}}
	for _, p := range purposes {
		c.granted[p] = true
		c.rev[p] = "rev-1-" + string(p)
	}
	return c
}

func allConsent() *consentFake {
	return newConsent(PurposeAIProcessing, PurposeAnthropic, PurposeDeepSeek, PurposePersonalMemory)
}

func (c *consentFake) Consent(_ context.Context, _ Subject, p Purpose) (Consent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.checked[p] = true
	if c.err != nil {
		return Consent{}, c.err
	}
	return Consent{Granted: c.granted[p], Revision: c.rev[p]}, nil
}

func (c *consentFake) AgeBand(context.Context, Subject) (AgeBand, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.band, nil
}

func (c *consentFake) withdraw(p Purpose) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.granted[p] = false
	c.rev[p] = "rev-2-" + string(p)
}

func (c *consentFake) setBand(b AgeBand) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.band = b
}

// reset forgets the checks made so far (called when a provider call
// ends, so a post-call reconfirmation never counts for the next call).
func (c *consentFake) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checked = map[Purpose]bool{}
}

// takeChecked reports whether every purpose was checked since the last
// call, and resets.
func (c *consentFake) takeChecked(ps []Purpose) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ok := true
	for _, p := range ps {
		ok = ok && c.checked[p]
	}
	c.checked = map[Purpose]bool{}
	return ok
}

// genStep is one scripted generator answer.
type genStep struct {
	text  string
	err   *GenerateError
	hook  func()
	delay time.Duration
}

// genFake is a scripted Generator. The last step repeats.
type genFake struct {
	t       *testing.T
	id      GeneratorIdentity
	consent *consentFake
	mu      sync.Mutex
	steps   []genStep
	reqs    []GenerateRequest
}

func newGen(t *testing.T, role ProviderName, c *consentFake, steps ...genStep) *genFake {
	model := "meta-models/Muse-Glimmer-30B"
	if role == ProviderDeepSeek {
		model = "deepseek-flash"
	}
	return &genFake{t: t, id: GeneratorIdentity{Role: role, ModelID: model, Revision: "cec94fc"}, consent: c, steps: steps}
}

func (g *genFake) Identity() GeneratorIdentity { return g.id }

func (g *genFake) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	g.mu.Lock()
	g.reqs = append(g.reqs, req)
	i := len(g.reqs) - 1
	if i >= len(g.steps) {
		i = len(g.steps) - 1
	}
	s := g.steps[i]
	g.mu.Unlock()
	if g.consent != nil {
		// A run of the memory feature, or one that sends memory items,
		// also needs personal_memory.
		memory := req.Task == TaskMemoryProposal || strings.Contains(req.User, `"role":"memory"`)
		if !g.consent.takeChecked(purposesFor(g.id.Role, req.Task, memory)) {
			g.t.Errorf("%s called without a fresh consent check", g.id.Role)
		}
		defer g.consent.reset()
	}
	if s.hook != nil {
		s.hook()
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
		}
	}
	served := "sela-glimmer-cec94fc"
	if g.id.Role == ProviderDeepSeek {
		served = "DeepSeek-V4.1-Flash-1017"
	}
	if s.err != nil {
		return GenerateResult{Latency: time.Millisecond, Attempts: 1}, s.err
	}
	return GenerateResult{Text: s.text, ServedModel: served, ServedRevision: "cec94fc", Latency: 2 * time.Millisecond,
		FirstContent: time.Millisecond, InputTokens: 100, OutputTokens: 20, Attempts: 1}, nil
}

func (g *genFake) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.reqs)
}

func (g *genFake) req(i int) GenerateRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.reqs[i]
}

// verStep is one scripted verifier answer.
type verStep struct {
	report func(verifier.Request) verifier.Report
	err    error
	hook   func()
}

type verFake struct {
	t       *testing.T
	id      verifier.Identity
	role    ProviderName
	consent *consentFake
	mu      sync.Mutex
	steps   []verStep
	reqs    []verifier.Request
}

func newClaude(t *testing.T, c *consentFake, steps ...verStep) *verFake {
	return &verFake{t: t, id: verifier.Identity{Provider: "anthropic", Model: "claude-sonnet-5-5"}, role: ProviderAnthropic, consent: c, steps: steps}
}

func newAlt(t *testing.T, c *consentFake, steps ...verStep) *verFake {
	return &verFake{t: t, id: verifier.Identity{Provider: "deepseek", Model: "deepseek-flash", Label: "alternative verifier (DeepSeek)"},
		role: ProviderDeepSeek, consent: c, steps: steps}
}

func (v *verFake) Identity() verifier.Identity { return v.id }

func (v *verFake) Verify(_ context.Context, req verifier.Request, _ verifier.CallOptions) (verifier.Result, error) {
	if err := req.Validate(); err != nil {
		v.t.Errorf("router sent an invalid VC-1 request: %v", err)
	}
	v.mu.Lock()
	v.reqs = append(v.reqs, req)
	i := len(v.reqs) - 1
	var s verStep
	if len(v.steps) > 0 {
		if i >= len(v.steps) {
			i = len(v.steps) - 1
		}
		s = v.steps[i]
	}
	v.mu.Unlock()
	if v.consent != nil {
		if !v.consent.takeChecked(purposesFor(v.role, TaskSummaryDraft, Task(req.Task) == TaskMemoryProposal)) {
			v.t.Errorf("%s verifier called without a fresh consent check", v.role)
		}
		defer v.consent.reset()
	}
	if s.hook != nil {
		s.hook()
	}
	if s.err != nil {
		return verifier.Result{Identity: v.id, Attempts: 1}, s.err
	}
	rep := allSupported(req)
	if s.report != nil {
		rep = s.report(req)
	}
	return v.result(rep), nil
}

func (v *verFake) result(rep verifier.Report) verifier.Result {
	return verifier.Result{Report: rep, Identity: v.id, ServedModel: v.id.Model, APIVersion: "2026-10-21", InputTokens: 300,
		OutputTokens: 60, Attempts: 1, Status: 200, Latency: 3 * time.Millisecond}
}

func (v *verFake) count() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.reqs)
}

func allSupported(req verifier.Request) verifier.Report {
	return withVerdicts(req, nil)
}

// withVerdicts marks claim i with verdicts[i] (default supported).
func withVerdicts(req verifier.Request, verdicts map[int]verifier.Verdict, issues ...verifier.Issue) verifier.Report {
	rep := verifier.Report{Issues: append([]verifier.Issue(nil), issues...)}
	for i, c := range req.Claims {
		v := verifier.Supported
		if x, ok := verdicts[i]; ok {
			v = x
		}
		rep.Claims = append(rep.Claims, verifier.ClaimVerdict{ClaimID: c.ID, Verdict: v, SpanIDs: c.SpanIDs, Reason: "REASON-CANARY checked"})
	}
	return rep
}

func verr(r transport.Reason) error { return transport.NewError("anthropic", r, 0, "") }

// genJSON builds a generation text.
func genJSON(risk string, declined bool, items ...[2]string) string {
	type it struct {
		Text    string   `json:"text"`
		SpanIDs []string `json:"span_ids"`
	}
	out := struct {
		Declined bool   `json:"declined"`
		Risk     string `json:"risk"`
		Items    []it   `json:"items"`
	}{Declined: declined, Risk: risk, Items: []it{}}
	for _, x := range items {
		ids := []string{}
		if x[1] != "" {
			ids = strings.Split(x[1], ",")
		}
		out.Items = append(out.Items, it{Text: x[0], SpanIDs: ids})
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}

func reply(text string) string { return genJSON("none", false, [2]string{text, "s1"}) }

// events collects observer events.
type events struct {
	mu  sync.Mutex
	all []Event
}

func (e *events) Observe(ev Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.all = append(e.all, ev)
}

func (e *events) has(stage Stage) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.all {
		if ev.Stage == stage {
			return true
		}
	}
	return false
}

func (e *events) json(t *testing.T) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	raw, err := json.Marshal(e.all)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// screenFake maps marker words to risks.
type screenFake struct{}

func (screenFake) Screen(text string) Risk {
	switch {
	case strings.Contains(text, "CRISIS-WORDS"):
		return RiskCrisis
	case strings.Contains(text, "ELEVATED-WORDS"):
		return RiskElevated
	}
	return RiskNone
}

// limiter returns err for the named provider.
type limiter struct {
	provider ProviderName
	err      error
}

func (l limiter) Allow(_ context.Context, _ Subject, p ProviderName, _ Task) error {
	if p == l.provider {
		return l.err
	}
	return nil
}

func testConfig() Config {
	return Config{Env: EnvTest, AllowTestDouble: true, Profile: ProfileFull, Fallback: true, AltVerifier: true,
		AllowedBands: Bands(Band18Plus, Band15To17, BandUnknown),
		OpTimeout:    DefaultOpTimeout, Anthropic: AnthropicConfig{Mode: ModeLive}, DeepSeek: DeepSeekConfig{Mode: ModeLive}}
}

// harness wires a router with fakes.
type harness struct {
	t      *testing.T
	cfg    Config
	c      *consentFake
	meta   *genFake
	ds     *genFake
	claude *verFake
	alt    *verFake
	ev     *events
	lim    Limiter
	noDS   bool
	noCl   bool
	noAlt  bool
}

func newHarness(t *testing.T) *harness {
	c := allConsent()
	return &harness{t: t, cfg: testConfig(), c: c,
		meta:   newGen(t, ProviderMeta, c, genStep{text: reply("Terima kasih sudah cerita.")}),
		ds:     newGen(t, ProviderDeepSeek, c, genStep{text: reply("Terima kasih sudah cerita.")}),
		claude: newClaude(t, c), alt: newAlt(t, c), ev: &events{}}
}

func (h *harness) router() *Router {
	d := Deps{Meta: h.meta, Consent: h.c, Screener: screenFake{}, Observer: h.ev, Limits: h.lim}
	if !h.noDS {
		d.DeepSeek = h.ds
	}
	if !h.noCl {
		d.Claude = h.claude
	}
	if !h.noAlt {
		d.AltVerifier = h.alt
	}
	r, err := NewRouter(h.cfg, d)
	if err != nil {
		h.t.Fatal(err)
	}
	return r
}

func (h *harness) run(task Task, in Input) (Output, *RunError) {
	h.t.Helper()
	out, err := h.router().Run(context.Background(), task, in)
	if err == nil {
		return out, nil
	}
	re, ok := err.(*RunError)
	if !ok {
		h.t.Fatalf("Run returned %T %v", err, err)
	}
	return out, re
}

const subject = Subject("acct-7f3a-SUBJECT-CANARY")

func inReply(texts ...string) Input {
	if len(texts) == 0 {
		texts = []string{"Aku capek banget sama skripsi."}
	}
	in := Input{Subject: subject, Language: "id"}
	for i, t := range texts {
		in.Spans = append(in.Spans, Span{Key: "turn-db-" + string(rune('a'+i)), Role: RoleUser, Text: t})
	}
	return in
}

func inItems(texts ...string) Input {
	in := Input{Subject: subject, Language: "id", Purpose: "first_consultation"}
	for i, t := range texts {
		in.Spans = append(in.Spans, Span{Key: "item-db-" + string(rune('a'+i)), Role: RoleItem, Text: t})
	}
	return in
}

func expect(t *testing.T, re *RunError, o Outcome, reason string) {
	t.Helper()
	if re == nil {
		t.Fatalf("Run succeeded, want %s", o)
	}
	if re.Outcome != o {
		t.Fatalf("outcome %s (%v), want %s", re.Outcome, re.Reasons, o)
	}
	if reason == "" {
		return
	}
	for _, r := range re.Reasons {
		if r == reason {
			return
		}
	}
	t.Fatalf("reasons %v lack %q", re.Reasons, reason)
}

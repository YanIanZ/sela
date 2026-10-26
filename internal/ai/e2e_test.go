package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/anthropic"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

// counting wraps a handler and counts requests by path.
type counting struct {
	h        http.Handler
	messages atomic.Int32
	chat     atomic.Int32
}

func (c *counting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/messages":
		c.messages.Add(1)
	case "/chat/completions":
		c.chat.Add(1)
	}
	c.h.ServeHTTP(w, r)
}

func doubleServer(t *testing.T, cfg fakellm.Config) (*httptest.Server, *counting) {
	t.Helper()
	cfg.SlowDelay = 2 * time.Second
	h, err := fakellm.NewHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c := &counting{h: h}
	srv := httptest.NewServer(c)
	t.Cleanup(srv.Close)
	return srv, c
}

type e2e struct {
	meta, claude, deepseek fakellm.Config
	cfg                    func(*Config)
}

func (e e2e) router(t *testing.T) (*Router, *counting, *counting) {
	t.Helper()
	metaSrv, _ := doubleServer(t, e.meta)
	claudeSrv, cc := doubleServer(t, e.claude)
	dsSrv, dc := doubleServer(t, e.deepseek)
	p := llm.Config{Mode: llm.ModeTestDouble, BaseURL: metaSrv.URL, ServedModel: llm.TestDoubleServedModel,
		AttemptTimeout: 500 * time.Millisecond}
	cfg := Config{Env: EnvTest, AllowTestDouble: true, Profile: ProfileFull, Fallback: true, AltVerifier: true,
		AllowedBands: Bands(Band18Plus, Band15To17, BandUnknown),
		OpTimeout:    DefaultOpTimeout,
		Anthropic:    AnthropicConfig{Mode: ModeTestDouble, BaseURL: claudeSrv.URL, AttemptTimeout: time.Second},
		DeepSeek:     DeepSeekConfig{Mode: ModeTestDouble, BaseURL: dsSrv.URL, AttemptTimeout: time.Second}}
	if e.cfg != nil {
		e.cfg(&cfg)
	}
	r, err := Build(cfg, p, Deps{Consent: allConsent()})
	if err != nil {
		t.Fatal(err)
	}
	return r, cc, dc
}

func summaryInput() Input {
	return inItems("Aku susah tidur sejak dua bulan.", "Aku belum pernah cerita ke siapa pun soal ini.")
}

func TestE2ESummaryWithTestDoubles(t *testing.T) {
	r, cc, _ := e2e{}.router(t)
	out, err := r.Run(context.Background(), TaskSummaryDraft, summaryInput())
	if err != nil {
		t.Fatal(err)
	}
	p := out.Provenance
	if len(out.Items) != 2 || out.Items[0].Source != SourceAIWording || out.Items[0].CheckLabel != anthropic.TestDoubleLabel ||
		p.DraftProvenance != DraftTestDouble || p.Verification.Provenance != VerifiedTestDouble || p.Generator.Provider != ProviderTestDouble ||
		p.Generator.ServedModel != llm.TestDoubleServedModel || p.Verification.ServedModel != anthropic.TestDoubleModel || cc.messages.Load() != 1 {
		raw, _ := json.Marshal(out)
		t.Fatalf("out %s", raw)
	}
}

func TestE2EMetaDownFallsBackToDeepSeek(t *testing.T) {
	r, cc, dc := e2e{meta: fakellm.Config{Fault: fakellm.FaultHTTP503}}.router(t)
	out, err := r.Run(context.Background(), TaskSummaryDraft, summaryInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Provenance.Route != RouteFallback || out.Provenance.FallbackReason != FallbackUnavailable || dc.chat.Load() != 1 ||
		cc.messages.Load() != 1 || out.Provenance.Verification.Route != VerifierRoute {
		t.Fatalf("provenance %+v", out.Provenance)
	}
}

func TestE2EClaudeDownUsesAlternativeVerifier(t *testing.T) {
	r, cc, dc := e2e{claude: fakellm.Config{ProviderFault: fakellm.ProviderFaultHTTP5xx}}.router(t)
	out, err := r.Run(context.Background(), TaskSummaryDraft, summaryInput())
	if err != nil {
		t.Fatal(err)
	}
	v := out.Provenance.Verification
	if v.Route != AlternativeVerifierRoute || !strings.Contains(v.Label, "alternative verifier") || !strings.Contains(v.Label, "TEST DOUBLE") ||
		cc.messages.Load() != 2 || dc.chat.Load() != 1 {
		t.Fatalf("verification %+v claude %d deepseek %d", v, cc.messages.Load(), dc.chat.Load())
	}
}

func TestE2EClaudeRefusalNeverReachesAlternative(t *testing.T) {
	r, _, dc := e2e{claude: fakellm.Config{ProviderFault: fakellm.ProviderFaultRefusal}}.router(t)
	_, err := r.Run(context.Background(), TaskSummaryDraft, summaryInput())
	re, ok := err.(*RunError)
	if !ok || re.Outcome != OutcomeAbstain || re.Provenance.Verification.UnavailableReasons[0] != "refused" ||
		re.Provenance.Verification.RefusalCategory != "general_harms" || dc.chat.Load() != 0 {
		t.Fatalf("err %v deepseek %d", err, dc.chat.Load())
	}
}

func TestE2EGuardsOnMetaDiagnosis(t *testing.T) {
	r, _, dc := e2e{meta: fakellm.Config{Fault: fakellm.FaultDiagnosis}}.router(t)
	_, err := r.Run(context.Background(), TaskListeningReply, inReply())
	re, ok := err.(*RunError)
	if !ok || re.Outcome != OutcomeAbstain || re.Provenance.Guards.Regenerations != 1 || dc.chat.Load() != 0 {
		t.Fatalf("err %v", err)
	}
}

func TestE2EMetaStallAfterContentNeverFallsBack(t *testing.T) {
	r, _, dc := e2e{meta: fakellm.Config{Fault: fakellm.FaultStall}}.router(t)
	_, err := r.Run(context.Background(), TaskReflection, inReply())
	if OutcomeOf(err) != OutcomeAbstain || dc.chat.Load() != 0 {
		t.Fatalf("err %v deepseek %d", err, dc.chat.Load())
	}
}

func TestE2EDeepSeekReasoningLeak(t *testing.T) {
	r, _, dc := e2e{meta: fakellm.Config{Fault: fakellm.FaultHTTP503}, deepseek: fakellm.Config{ProviderFault: fakellm.ProviderFaultReasoningLeak}}.router(t)
	_, err := r.Run(context.Background(), TaskListeningReply, inReply())
	re, ok := err.(*RunError)
	if !ok || re.Outcome != OutcomeNoAI || dc.chat.Load() != 1 {
		t.Fatalf("err %v", err)
	}
	raw, _ := json.Marshal(re.Provenance)
	if strings.Contains(string(raw)+re.Error(), fakellm.ReasoningCanary) {
		t.Fatal("reasoning leaked into provenance or the error")
	}
}

func TestE2EGeneratorDecline(t *testing.T) {
	r, _, dc := e2e{}.router(t)
	_, err := r.Run(context.Background(), TaskReflection, inReply("Aku capek. "+fakellm.MarkerDecline))
	if OutcomeOf(err) != OutcomeSafeStop || dc.chat.Load() != 0 {
		t.Fatalf("err %v", err)
	}
}

func TestBuildRefusesProductionTestDoubles(t *testing.T) {
	p := llm.Config{Mode: llm.ModeTestDouble, BaseURL: "http://127.0.0.1:1", ServedModel: llm.TestDoubleServedModel}
	cfg := Config{Env: EnvProduction, Profile: ProfileFull, OpTimeout: DefaultOpTimeout}
	if _, err := Build(cfg, p, Deps{Consent: allConsent()}); err == nil {
		t.Fatal("production accepted a Meta test double")
	}
	cfg.Anthropic = AnthropicConfig{Mode: ModeTestDouble, BaseURL: "http://127.0.0.1:1"}
	if _, err := Build(cfg, llm.Config{Mode: llm.ModeDisabled}, Deps{Consent: allConsent()}); err == nil {
		t.Fatal("production accepted a Claude test double")
	}
}

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

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

// counting wraps a handler and counts Chat Completions requests.
type counting struct {
	h    http.Handler
	chat atomic.Int32
}

func (c *counting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/chat/completions") {
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
	meta fakellm.Config
	cfg  func(*Config)
	ver  func(*testing.T, *consentFake) *verFake
}

func (e e2e) router(t *testing.T) (*Router, *counting) {
	t.Helper()
	metaSrv, mc := doubleServer(t, e.meta)
	p := llm.Config{Mode: llm.ModeTestDouble, BaseURL: metaSrv.URL, ServedModel: llm.TestDoubleServedModel,
		AttemptTimeout: 500 * time.Millisecond}
	c := allConsent()
	cfg := Config{Env: EnvTest, AllowTestDouble: true, Profile: ProfileFull,
		AllowedBands: Bands(Band18Plus, Band15To17, BandUnknown),
		OpTimeout:    DefaultOpTimeout,
		MetaVerifier: MetaVerifierConfig{Mode: ModeTestDouble}}
	if e.cfg != nil {
		e.cfg(&cfg)
	}
	ver := newVerifier(t, c)
	if e.ver != nil {
		ver = e.ver(t, c)
	}
	r, err := Build(cfg, p, Deps{Consent: c, Verifier: ver})
	if err != nil {
		t.Fatal(err)
	}
	return r, mc
}

func summaryInput() Input {
	return inItems("Aku susah tidur sejak dua bulan.", "Aku belum pernah cerita ke siapa pun soal ini.")
}

func TestE2ESummaryWithTestDoubles(t *testing.T) {
	r, mc := e2e{}.router(t)
	out, err := r.Run(context.Background(), TaskSummaryDraft, summaryInput())
	if err != nil {
		t.Fatal(err)
	}
	p := out.Provenance
	if len(out.Items) != 2 || out.Items[0].Source != SourceAIWording || out.Items[0].CheckLabel != "Meta verifier" ||
		p.DraftProvenance != DraftTestDouble || p.Verification.Provenance != VerifiedLiveMetaVerifier || p.Generator.Provider != ProviderTestDouble ||
		p.Generator.ServedModel != llm.TestDoubleServedModel || mc.chat.Load() != 1 {
		raw, _ := json.Marshal(out)
		t.Fatalf("out %s", raw)
	}
}

func TestE2EVerifierUnavailableWithholds(t *testing.T) {
	r, _ := e2e{ver: func(t *testing.T, c *consentFake) *verFake {
		return newVerifier(t, c, verStep{err: transport.NewError("meta_verifier", transport.ReasonUnavailable, 503, "")})
	}}.router(t)
	_, err := r.Run(context.Background(), TaskSummaryDraft, summaryInput())
	re, ok := err.(*RunError)
	if !ok || re.Outcome != OutcomeAbstain || re.Provenance.Verification.Status != StatusVerificationUnavailable {
		t.Fatalf("err %v", err)
	}
}

func TestE2EGuardsOnMetaDiagnosis(t *testing.T) {
	r, mc := e2e{meta: fakellm.Config{Fault: fakellm.FaultDiagnosis}}.router(t)
	_, err := r.Run(context.Background(), TaskListeningReply, inReply())
	re, ok := err.(*RunError)
	if !ok || re.Outcome != OutcomeAbstain || mc.chat.Load() < 1 {
		t.Fatalf("err %v", err)
	}
}

func TestE2EMetaStallAfterContentNeverFallsBack(t *testing.T) {
	r, mc := e2e{meta: fakellm.Config{Fault: fakellm.FaultStall}}.router(t)
	_, err := r.Run(context.Background(), TaskReflection, inReply())
	if OutcomeOf(err) != OutcomeAbstain && OutcomeOf(err) != OutcomeNoAI {
		t.Fatalf("err %v calls %d", err, mc.chat.Load())
	}
}

func TestE2EGeneratorDecline(t *testing.T) {
	r, mc := e2e{}.router(t)
	_, err := r.Run(context.Background(), TaskReflection, inReply("Aku capek. "+fakellm.MarkerDecline))
	if OutcomeOf(err) != OutcomeSafeStop || mc.chat.Load() != 1 {
		t.Fatalf("err %v", err)
	}
}

func TestBuildRefusesProductionTestDoubles(t *testing.T) {
	p := llm.Config{Mode: llm.ModeTestDouble, BaseURL: "http://127.0.0.1:1", ServedModel: llm.TestDoubleServedModel}
	cfg := Config{Env: EnvProduction, Profile: ProfileFull, OpTimeout: DefaultOpTimeout}
	if _, err := Build(cfg, p, Deps{Consent: allConsent()}); err == nil {
		t.Fatal("production accepted a Meta test double")
	}
	cfg.MetaVerifier = MetaVerifierConfig{Mode: ModeTestDouble, BaseURL: "http://127.0.0.1:1"}
	if _, err := Build(cfg, llm.Config{Mode: llm.ModeDisabled}, Deps{Consent: allConsent()}); err == nil {
		t.Fatal("production accepted a verifier test double")
	}
}

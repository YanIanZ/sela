package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
)

// allowAI grants every AI purpose to the synthetic test subject.
type allowAI struct{}

func (allowAI) Consent(context.Context, ai.Subject, ai.Purpose) (ai.Consent, error) {
	return ai.Consent{Granted: true, Revision: "test-revision"}, nil
}
func (allowAI) AgeBand(context.Context, ai.Subject) (ai.AgeBand, error) { return ai.Band18Plus, nil }

// countingHandler counts provider requests; a denied operation must leave
// both servers untouched.
type countingHandler struct {
	h     http.Handler
	calls atomic.Int64
}

func (c *countingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		c.calls.Add(1)
	}
	c.h.ServeHTTP(w, r)
}

// fakeGate is the narrow BudgetGate the HTTP layer sees.
type fakeGate struct {
	enabled  bool
	admitErr error
	mu       sync.Mutex
	admitted int
	settled  int
	last     budget.Settlement
	summary  budget.Summary
}

func (f *fakeGate) Enabled() bool { return f.enabled }
func (f *fakeGate) Admit(context.Context, string, bool) (budget.Reservation, error) {
	if f.admitErr != nil {
		return budget.Reservation{}, f.admitErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.admitted++
	return budget.Reservation{ID: [16]byte{1}, ReservedUSD: 0.25}, nil
}
func (f *fakeGate) Settle(_ context.Context, _ budget.Reservation, s budget.Settlement) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settled++
	f.last = s
	return nil
}
func (f *fakeGate) Summary(context.Context) (budget.Summary, error) { return f.summary, nil }
func (f *fakeGate) ByTask(context.Context, int) ([]budget.TaskSpend, error) {
	return nil, nil
}

func newBudgetEnv(t *testing.T, gate httpapi.BudgetGate) (*v3env, *countingHandler) {
	t.Helper()
	posts := &countingHandler{h: mustDouble(t)}
	metaSrv := httptest.NewServer(posts)
	t.Cleanup(metaSrv.Close)
	router, err := ai.Build(ai.Config{
		Env: ai.EnvTest, AllowTestDouble: true, Profile: ai.ProfileFull,
		AllowedBands: ai.Bands(ai.Band18Plus), OpTimeout: ai.DefaultOpTimeout,
		MetaVerifier: ai.MetaVerifierConfig{Mode: ai.ModeDisabled},
	}, llm.Config{Mode: llm.ModeTestDouble, BaseURL: metaSrv.URL, ServedModel: llm.TestDoubleServedModel,
		AttemptTimeout: time.Second}, ai.Deps{Consent: allowAI{}, MetaAdmission: llm.NewAdmission(2)})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			kinds := []string{account.KindGuest, account.KindAccount}
			c.Purposes = append(c.Purposes,
				account.Purpose{Name: "ai_processing", Kinds: kinds},
				account.Purpose{Name: "ai_processing", Kinds: kinds})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) {
			o.AIRouter = router
			o.Budget = gate
		},
	})
	return e, posts
}

func mustDouble(t *testing.T) http.Handler {
	t.Helper()
	h, err := fakellm.NewHandler(fakellm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestBudgetDeniesBeforeAnyProviderCall(t *testing.T) {
	gate := &fakeGate{enabled: true, admitErr: budget.ErrExceeded}
	e, posts := newBudgetEnv(t, gate)
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	id := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)
	room := c.must(200, "POST", "/api/v3/conversations/"+id+"/turns",
		map[string]any{"text": "Aku ingin mengurai hari ini.", "expectedVersion": 1}).data()
	if room["aiState"] != "budget_limited" {
		t.Fatalf("aiState = %v, want budget_limited", room["aiState"])
	}
	if got := posts.calls.Load(); got != 0 {
		t.Fatalf("provider received %d calls after a budget refusal", got)
	}
	if gate.settled != 0 {
		t.Fatalf("denied operation settled %d reservations", gate.settled)
	}
}

func TestBudgetSettlesOneAdmittedOperation(t *testing.T) {
	gate := &fakeGate{enabled: true}
	e, posts := newBudgetEnv(t, gate)
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	id := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)
	room := c.must(200, "POST", "/api/v3/conversations/"+id+"/turns",
		map[string]any{"text": "Aku ingin mengurai hari ini.", "expectedVersion": 1}).data()
	if room["aiState"] != "ready" {
		t.Fatalf("aiState = %v, want ready", room["aiState"])
	}
	if posts.calls.Load() == 0 {
		t.Fatal("admitted operation never reached the provider")
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.admitted != 1 || gate.settled != 1 {
		t.Fatalf("admitted=%d settled=%d", gate.admitted, gate.settled)
	}
	if gate.last.Provider == "" || gate.last.Outcome == "" {
		t.Fatalf("settlement = %+v", gate.last)
	}
}

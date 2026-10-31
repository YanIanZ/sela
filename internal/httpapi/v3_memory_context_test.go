package httpapi_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/httpapi"
)

// memoryCapture records the generator payload of every call, so the test
// can inspect which spans actually reached the router. It never stores a
// key or logs content.
type memoryCapture struct {
	mu    sync.Mutex
	users []string
}

func (g *memoryCapture) Identity() ai.GeneratorIdentity {
	return ai.GeneratorIdentity{Role: ai.ProviderMeta, ModelID: "test-generator", Revision: "test"}
}

func (g *memoryCapture) Generate(_ context.Context, req ai.GenerateRequest) (ai.GenerateResult, error) {
	g.mu.Lock()
	g.users = append(g.users, req.User)
	g.mu.Unlock()
	return ai.GenerateResult{
		Text:           `{"declined":false,"risk":"none","items":[{"text":"Aku mendengar.","span_ids":["s1"]}]}`,
		ServedModel:    "test-generator",
		ServedRevision: "test",
		Latency:        time.Millisecond,
		FirstContent:   time.Millisecond,
		InputTokens:    10,
		OutputTokens:   10,
		Attempts:       1,
	}, nil
}

func (g *memoryCapture) snapshot() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.users...)
}

func TestListeningMemoryContextOnlyWithConsent(t *testing.T) {
	gen := &memoryCapture{}
	router, err := ai.NewRouter(ai.Config{
		Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus), OpTimeout: ai.DefaultOpTimeout,
	}, ai.Deps{Meta: gen, Consent: allowAI{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes,
				account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})

	grantMemoryConsent(t, c)
	for i := 0; i < 6; i++ {
		c.must(201, "POST", "/api/v3/memory", map[string]any{
			"title":          fmt.Sprintf("Synthetic memory %d", i),
			"text":           fmt.Sprintf("Synthetic memory context %d, chosen by the owner.", i),
			"storageGranted": true,
		})
	}
	// Withdrawal must stop use at once, even though the rows remain.
	c.must(200, "PUT", "/api/v3/me/consents/personal_memory", map[string]any{
		"granted": false, "policyVersion": policyVersion("personal_memory"),
	})

	room := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	id := room["id"].(string)
	c.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{
		"text": "Aku ingin mengurai hari ini.", "expectedVersion": 1,
	})
	afterOff := gen.snapshot()
	if len(afterOff) != 1 {
		t.Fatalf("generator calls without memory consent: %d", len(afterOff))
	}
	if strings.Contains(afterOff[0], "Synthetic memory context") || strings.Contains(afterOff[0], `"role":"memory"`) {
		t.Fatalf("memory reached the router without consent: %s", afterOff[0])
	}

	grantMemoryConsent(t, c)
	room = c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	id = room["id"].(string)
	c.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{
		"text": "Aku ingin mengurai hari ini lagi.", "expectedVersion": 1,
	})
	afterOn := gen.snapshot()
	if len(afterOn) != 2 {
		t.Fatalf("generator calls with memory consent: %d", len(afterOn))
	}
	if !strings.Contains(afterOn[1], "Synthetic memory context") {
		t.Fatalf("memory did not reach the router with consent: %s", afterOn[1])
	}
	// At most five items are selected, so AT-1's five-item ceiling holds
	// at the router boundary.
	if got := strings.Count(afterOn[1], `"role":"memory"`); got != 5 {
		t.Fatalf("memory spans sent = %d, want 5: %s", got, afterOn[1])
	}
}

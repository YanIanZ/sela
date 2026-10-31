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

func TestV3PrivateNames(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	a.register("daftar.nama")
	b := e.client()
	b.register("pemilik.lain")

	empty := a.must(200, "GET", "/api/v3/me/private-names", nil).data()
	if names, ok := empty["names"].([]any); !ok || len(names) != 0 {
		t.Fatalf("empty list: %v", empty)
	}
	created := a.must(201, "POST", "/api/v3/me/private-names", map[string]any{"name": " Raka "}).data()
	id, _ := created["id"].(string)
	if id == "" || created["name"] != "Raka" {
		t.Fatalf("created: %v", created)
	}
	a.must(201, "POST", "/api/v3/me/private-names", map[string]any{"name": "Dewi"})
	list := a.must(200, "GET", "/api/v3/me/private-names", nil).data()["names"].([]any)
	seen := map[string]bool{}
	for _, n := range list {
		seen[n.(map[string]any)["name"].(string)] = true
	}
	if len(list) != 2 || !seen["Raka"] || !seen["Dewi"] {
		t.Fatalf("list: %v", list)
	}

	// Case-insensitive duplicate, empty and over-long names, and the limit.
	if r := a.call("POST", "/api/v3/me/private-names", map[string]any{"name": "raka"}); r.status != 409 || r.code() != "name_taken" || r.field() != "name" {
		t.Fatalf("duplicate: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("a", account.MaxPrivateNameRunes+1)} {
		if r := a.call("POST", "/api/v3/me/private-names", map[string]any{"name": bad}); r.status != 400 || r.code() != "invalid_input" || r.field() != "name" {
			t.Fatalf("invalid %q: %d %s", bad, r.status, r.raw)
		}
	}
	for i := 2; i < account.MaxPrivateNames; i++ {
		a.must(201, "POST", "/api/v3/me/private-names", map[string]any{"name": fmt.Sprintf("Nama %02d", i)})
	}
	if r := a.call("POST", "/api/v3/me/private-names", map[string]any{"name": "Nama terakhir"}); r.status != 409 || r.code() != "name_limit_reached" {
		t.Fatalf("limit: %d %s", r.status, r.raw)
	}

	// Another owner neither sees nor deletes the list.
	if other := b.must(200, "GET", "/api/v3/me/private-names", nil).data()["names"].([]any); len(other) != 0 {
		t.Fatalf("other owner: %v", other)
	}
	for _, id := range []string{id, "6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", "not-a-uuid"} {
		if r := b.call("DELETE", "/api/v3/me/private-names/"+id, nil); r.status != 404 || r.code() != "not_found" {
			t.Fatalf("cross-owner delete %s: %d %s", id, r.status, r.raw)
		}
	}
	if n := a.must(200, "GET", "/api/v3/me/private-names", nil).data()["names"].([]any); len(n) != account.MaxPrivateNames {
		t.Fatalf("list changed by another owner: %d", len(n))
	}

	// The mutation guard applies: no origin/CSRF header means refusal.
	if r := a.call("DELETE", "/api/v3/me/private-names/"+id, nil, "X-Sela-Request", ""); r.status != 403 || r.code() != "cross_origin" {
		t.Fatalf("mutation guard: %d %s", r.status, r.raw)
	}
	if r := a.must(200, "DELETE", "/api/v3/me/private-names/"+id, nil); r.data()["deleted"] != true {
		t.Fatalf("delete: %s", r.raw)
	}
	if r := a.call("DELETE", "/api/v3/me/private-names/"+id, nil); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("repeat delete: %d %s", r.status, r.raw)
	}
	// The owner's export includes the list, decrypted.
	ex := a.must(200, "GET", "/api/v3/me/export?format=json", nil)
	if !strings.Contains(string(ex.raw), `"privateNames"`) || !strings.Contains(string(ex.raw), `"name": "Dewi"`) {
		t.Fatalf("export: %s", ex.raw)
	}
}

// nameCapture records the serialized provider payload of every call.
type nameCapture struct {
	mu   sync.Mutex
	seen []string
}

func (g *nameCapture) Identity() ai.GeneratorIdentity {
	return ai.GeneratorIdentity{Role: ai.ProviderMeta, ModelID: "test-generator", Revision: "test"}
}

func (g *nameCapture) Generate(_ context.Context, req ai.GenerateRequest) (ai.GenerateResult, error) {
	text := `{"declined":false,"risk":"none","items":[{"text":"Aku mendengar.","span_ids":["s1"]}]}`
	if req.Task == ai.TaskReflection {
		text = `{"declined":false,"risk":"none","items":[{"text":"Kamu bercerita tentang kemarin.","span_ids":["s1"]}]}`
	}
	g.mu.Lock()
	g.seen = append(g.seen, req.User)
	g.mu.Unlock()
	return ai.GenerateResult{Text: text, ServedModel: "test-generator", ServedRevision: "test",
		Latency: time.Millisecond, FirstContent: time.Millisecond, InputTokens: 10, OutputTokens: 10, Attempts: 1}, nil
}

func (g *nameCapture) last() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.seen) == 0 {
		return ""
	}
	return g.seen[len(g.seen)-1]
}

// TestV3PrivateNamesRedactedBeforeProvider proves a saved name never reaches
// the generator, and that an unreadable list never blocks the reply.
func TestV3PrivateNamesRedactedBeforeProvider(t *testing.T) {
	t.Parallel()
	gen := &nameCapture{}
	router, err := ai.NewRouter(ai.Config{Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus), OpTimeout: ai.DefaultOpTimeout},
		ai.Deps{Meta: gen, Consent: allowAI{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	c.must(201, "POST", "/api/v3/me/private-names", map[string]any{"name": "Raka"})

	room := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true}).data()
	id := room["id"].(string)
	sent := c.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{
		"text": "Aku cerita ke Raka kemarin.", "expectedVersion": 1,
	}).data()
	if payload := gen.last(); strings.Contains(payload, "Raka") || !strings.Contains(payload, "[NAMA-1]") {
		t.Fatalf("provider payload not redacted: %s", payload)
	}

	// The reflection path redacts too.
	version := int(sent["version"].(float64))
	c.must(200, "POST", "/api/v3/conversations/"+id+"/reflections", map[string]any{"expectedVersion": version})
	if payload := gen.last(); strings.Contains(payload, "Raka") || !strings.Contains(payload, "[NAMA-1]") {
		t.Fatalf("reflection payload not redacted: %s", payload)
	}

	// A list that cannot be read is a content-free warning, not a refusal:
	// the reply still runs and only the deterministic redaction applies.
	if _, err := e.db.Pool.Exec(bg, `DROP TABLE private_names`); err != nil {
		t.Fatal(err)
	}
	room = c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	id = room["id"].(string)
	after := c.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{
		"text": "Aku cerita ke Raka kemarin lagi.", "expectedVersion": 1,
	})
	if turns, _ := after.data()["turns"].([]any); len(turns) != 2 {
		t.Fatalf("reply blocked by an unreadable name list: %s", after.raw)
	}
	logs := e.logs.String()
	if !strings.Contains(logs, "ai_private_names_failed") || strings.Contains(logs, "Raka") {
		t.Fatalf("warning missing or it leaked a name: %s", logs)
	}
}

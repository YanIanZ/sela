package httpapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/httpapi"
)

type reflectionConsent struct{}

func (reflectionConsent) Consent(context.Context, ai.Subject, ai.Purpose) (ai.Consent, error) {
	return ai.Consent{Granted: true, Revision: "rev-1"}, nil
}
func (reflectionConsent) AgeBand(context.Context, ai.Subject) (ai.AgeBand, error) {
	return ai.Band18Plus, nil
}

// reflectionGenerator answers AT-1 with a plain listening reply and AT-2
// with one quote-backed interpretation.
type reflectionGenerator struct{}

func (reflectionGenerator) Identity() ai.GeneratorIdentity {
	return ai.GeneratorIdentity{Role: ai.ProviderMeta, ModelID: "test-generator", Revision: "test"}
}

func (reflectionGenerator) Generate(_ context.Context, req ai.GenerateRequest) (ai.GenerateResult, error) {
	text := `{"declined":false,"risk":"none","items":[{"text":"Terima kasih sudah cerita.","span_ids":["s1"]}]}`
	if req.Task == ai.TaskReflection {
		text = `{"declined":false,"risk":"none","items":[{"text":"Kamu merasa lelah dengan kuliahmu.","span_ids":["s1"]}]}`
	}
	return ai.GenerateResult{Text: text, ServedModel: "test-generator", ServedRevision: "test",
		Latency: time.Millisecond, FirstContent: time.Millisecond, InputTokens: 10, OutputTokens: 10, Attempts: 1}, nil
}

func TestV3ReflectionCardsReview(t *testing.T) {
	router, err := ai.NewRouter(ai.Config{Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus),
		OpTimeout: ai.DefaultOpTimeout}, ai.Deps{Meta: reflectionGenerator{}, Consent: reflectionConsent{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")
	a.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	room := a.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true}).data()
	id := room["id"].(string)
	sent := a.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{
		"text": "Aku capek banget sama kuliahku.", "expectedVersion": 1,
	}).data()
	version := int(sent["version"].(float64))
	if version != 3 {
		t.Fatalf("turn versions: %v logs=%s", sent, e.logs.String())
	}

	cards := a.must(200, "POST", "/api/v3/conversations/"+id+"/reflections", map[string]any{"expectedVersion": version}).data()
	list, _ := cards["reflections"].([]any)
	if len(list) != 1 {
		t.Fatalf("reflections: %v", cards)
	}
	card := list[0].(map[string]any)
	reflectionID := card["id"].(string)
	if card["quote"] != "Aku capek banget sama kuliahku." || card["reviewStatus"] != "unreviewed" ||
		card["provenance"] != "unconfirmed_interpretation" {
		t.Fatalf("card: %v", card)
	}
	version = int(cards["version"].(float64))

	review := a.must(200, "PATCH", "/api/v3/conversations/"+id+"/reflections/"+reflectionID, map[string]any{
		"expectedVersion": version, "action": "correct", "text": "Aku lelah dengan kuliah.",
	}).data()
	card = review["reflections"].([]any)[0].(map[string]any)
	if card["reviewStatus"] != "corrected" || card["provenance"] != "user_correction" || card["text"] != "Aku lelah dengan kuliah." {
		t.Fatalf("corrected: %v", card)
	}
	version = int(review["version"].(float64))
	confirm := a.must(200, "PATCH", "/api/v3/conversations/"+id+"/reflections/"+reflectionID, map[string]any{
		"expectedVersion": version, "action": "confirm",
	}).data()
	card = confirm["reflections"].([]any)[0].(map[string]any)
	if card["reviewStatus"] != "confirmed" {
		t.Fatalf("confirmed: %v", card)
	}
	version = int(confirm["version"].(float64))
	reject := a.must(200, "PATCH", "/api/v3/conversations/"+id+"/reflections/"+reflectionID, map[string]any{
		"expectedVersion": version, "action": "reject",
	}).data()
	card = reject["reflections"].([]any)[0].(map[string]any)
	if card["reviewStatus"] != "rejected" {
		t.Fatalf("rejected: %v", card)
	}
	version = int(reject["version"].(float64))

	if r := a.call("PATCH", "/api/v3/conversations/"+id+"/reflections/"+reflectionID, map[string]any{
		"expectedVersion": version - 1, "action": "confirm",
	}); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("stale review: %d %s", r.status, r.raw)
	}
	if r := b.call("PATCH", "/api/v3/conversations/"+id+"/reflections/"+reflectionID, map[string]any{
		"expectedVersion": version, "action": "confirm",
	}); r.status != 404 {
		t.Fatalf("foreign review: %d %s", r.status, r.raw)
	}
	if r := b.call("POST", "/api/v3/conversations/"+id+"/reflections", map[string]any{"expectedVersion": version}); r.status != 404 {
		t.Fatalf("foreign create: %d %s", r.status, r.raw)
	}
}

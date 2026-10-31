package httpapi_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/ai/metaverifier"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/httpapi"
)

// proposalGenerator answers AT-1 with a listening reply and AT-3 with one
// quote-backed proposal. It records the payloads of AT-1 calls only, so a
// test can assert what memory reached the provider.
type proposalGenerator struct {
	mu    sync.Mutex
	users []string
}

func (g *proposalGenerator) Identity() ai.GeneratorIdentity {
	return ai.GeneratorIdentity{Role: ai.ProviderMeta, ModelID: "test-generator", Revision: "test"}
}

func (g *proposalGenerator) Generate(_ context.Context, req ai.GenerateRequest) (ai.GenerateResult, error) {
	if req.Task == ai.TaskListeningReply {
		g.mu.Lock()
		g.users = append(g.users, req.User)
		g.mu.Unlock()
	}
	text := `{"declined":false,"risk":"none","items":[{"text":"Terima kasih sudah cerita.","span_ids":["s1"]}]}`
	if req.Task == ai.TaskMemoryProposal {
		text = `{"declined":false,"risk":"none","items":[{"text":"Kamu suka menulis di pagi hari.","span_ids":["s1"]}]}`
	}
	return ai.GenerateResult{Text: text, ServedModel: "test-generator", ServedRevision: "test",
		Latency: time.Millisecond, FirstContent: time.Millisecond, InputTokens: 10, OutputTokens: 10, Attempts: 1}, nil
}

func (g *proposalGenerator) snapshot() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.users...)
}

// proposalVerifier supports every claim. Other tests substitute a
// withholding verifier.
type proposalVerifier struct{ withhold bool }

func (v proposalVerifier) Identity() verifier.Identity {
	return verifier.Identity{Provider: metaverifier.Provider, Model: "test-verifier"}
}

func (v proposalVerifier) Verify(_ context.Context, req verifier.Request, _ verifier.CallOptions) (verifier.Result, error) {
	report := verifier.Report{Claims: make([]verifier.ClaimVerdict, 0, len(req.Claims))}
	for _, c := range req.Claims {
		report.Claims = append(report.Claims, verifier.ClaimVerdict{
			ClaimID: c.ID, Verdict: verifier.Supported, SpanIDs: append([]string(nil), c.SpanIDs[:1]...), Reason: "supported",
		})
	}
	if v.withhold {
		report.Issues = []verifier.Issue{{Category: verifier.MedicalLanguage, Reason: "medical language"}}
	}
	return verifier.Result{Report: report, Identity: v.Identity(), ServedModel: "test-verifier", Attempts: 1,
		Latency: time.Millisecond, InputTokens: 10, OutputTokens: 10}, nil
}

func proposalRouter(t *testing.T, gen *proposalGenerator, v proposalVerifier) *ai.Router {
	t.Helper()
	router, err := ai.NewRouter(ai.Config{
		Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus), OpTimeout: ai.DefaultOpTimeout,
		MetaVerifier: ai.MetaVerifierConfig{Mode: ai.ModeLive},
	}, ai.Deps{Meta: gen, Verifier: v, Consent: reflectionConsent{}})
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func proposalV3(t *testing.T, router *ai.Router) *v3env {
	t.Helper()
	return newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes,
				account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
}

func grantProposalConsents(t *testing.T, c *v3client) {
	t.Helper()
	grantMemoryConsent(t, c)
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
}

// savedConversation creates a saved room with one user turn and returns
// the room version and the conversation record.
func savedConversation(t *testing.T, c *v3client) (string, int, map[string]any) {
	t.Helper()
	room := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true}).data()
	id := room["id"].(string)
	sent := c.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{
		"text": "Aku suka menulis di pagi hari.", "expectedVersion": 1,
	}).data()
	version := int(sent["version"].(float64))
	records, _ := c.must(200, "GET", "/api/v3/records", nil).data()["records"].([]any)
	for _, raw := range records {
		r := raw.(map[string]any)
		if r["kind"] == "conversation" {
			return id, version, r
		}
	}
	t.Fatalf("no saved conversation record: %v", records)
	return "", 0, nil
}

func TestV3MemoryProposalsCreateReviewAndReplyContext(t *testing.T) {
	gen := &proposalGenerator{}
	e := proposalV3(t, proposalRouter(t, gen, proposalVerifier{}))
	a := e.client()
	a.guest("18_plus")
	grantProposalConsents(t, a)
	roomID, roomVersion, conv := savedConversation(t, a)
	convID, convVersion := conv["id"].(string), int(conv["version"].(float64))

	resp := a.must(200, "POST", "/api/v3/memory/proposals", map[string]any{
		"conversationId": convID, "expectedVersion": convVersion,
	}).data()
	if resp["aiState"] != "released" {
		t.Fatalf("aiState: %v", resp)
	}
	proposals, _ := resp["proposals"].([]any)
	if len(proposals) != 1 {
		t.Fatalf("proposals: %v", resp)
	}
	p := proposals[0].(map[string]any)
	id := p["id"].(string)
	if p["kind"] != "memory" || p["reviewStatus"] != "unreviewed" || p["provenance"] != "unconfirmed_interpretation" ||
		p["text"] != "Kamu suka menulis di pagi hari." || p["version"].(float64) != 1 {
		t.Fatalf("proposal: %v", p)
	}
	refs, _ := p["sourceRefs"].([]any)
	if len(refs) != 1 {
		t.Fatalf("source refs: %v", p)
	}
	ref := refs[0].(map[string]any)
	if ref["recordId"] != convID || ref["quote"] != "Aku suka menulis di pagi hari." || ref["turnKey"] != "turn-0" {
		t.Fatalf("source ref: %v", ref)
	}
	detail := a.must(200, "GET", "/api/v3/memory/"+id, nil).data()
	if detail["reviewStatus"] != "unreviewed" || detail["sourceRefs"].([]any)[0].(map[string]any)["quote"] != "Aku suka menulis di pagi hari." {
		t.Fatalf("detail: %v", detail)
	}

	// An unreviewed proposal is never sent as AT-1 memory context.
	next := a.must(200, "POST", "/api/v3/conversations/"+roomID+"/turns", map[string]any{
		"text": "Aku ingin bercerita lagi.", "expectedVersion": roomVersion,
	}).data()
	roomVersion = int(next["version"].(float64))
	before := gen.snapshot()
	if len(before) < 2 || strings.Contains(before[len(before)-1], "Kamu suka menulis di pagi hari.") {
		t.Fatalf("unreviewed proposal reached AT-1: %v", before)
	}

	// Confirm keeps the AI wording, then it becomes usable reply context.
	confirmed := a.must(200, "PATCH", "/api/v3/memory/"+id+"/review", map[string]any{
		"expectedVersion": 1, "action": "confirm",
	}).data()
	if confirmed["reviewStatus"] != "confirmed" || confirmed["version"].(float64) != 2 ||
		confirmed["text"] != "Kamu suka menulis di pagi hari." || confirmed["provenance"] != "unconfirmed_interpretation" {
		t.Fatalf("confirmed: %v", confirmed)
	}
	a.must(200, "POST", "/api/v3/conversations/"+roomID+"/turns", map[string]any{
		"text": "Aku ingin bercerita sekali lagi.", "expectedVersion": roomVersion,
	})
	after := gen.snapshot()
	if len(after) < 3 || !strings.Contains(after[len(after)-1], "Kamu suka menulis di pagi hari.") {
		t.Fatalf("confirmed proposal missing from AT-1: %v", after)
	}

	// Correct replaces the text and adds a revision.
	corrected := a.must(200, "PATCH", "/api/v3/memory/"+id+"/review", map[string]any{
		"expectedVersion": 2, "action": "correct", "text": "Menulis di pagi hari membantumu.",
	}).data()
	if corrected["reviewStatus"] != "corrected" || corrected["provenance"] != "user_correction" ||
		corrected["version"].(float64) != 3 || corrected["text"] != "Menulis di pagi hari membantumu." {
		t.Fatalf("corrected: %v", corrected)
	}
	if r := a.call("PATCH", "/api/v3/memory/"+id+"/review", map[string]any{
		"expectedVersion": 1, "action": "confirm",
	}); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("stale review: %d %s", r.status, r.raw)
	}
	// Reject, then reopen.
	rejected := a.must(200, "PATCH", "/api/v3/memory/"+id+"/review", map[string]any{
		"expectedVersion": 3, "action": "reject",
	}).data()
	if rejected["reviewStatus"] != "rejected" || rejected["version"].(float64) != 4 {
		t.Fatalf("rejected: %v", rejected)
	}
	reopened := a.must(200, "PATCH", "/api/v3/memory/"+id+"/review", map[string]any{
		"expectedVersion": 4, "action": "confirm",
	}).data()
	if reopened["reviewStatus"] != "confirmed" || reopened["version"].(float64) != 5 {
		t.Fatalf("reopened: %v", reopened)
	}
	if r := a.call("PATCH", "/api/v3/memory/"+id+"/review", map[string]any{
		"expectedVersion": 5, "action": "undo",
	}); r.status != 400 || r.code() != "invalid_input" || r.field() != "action" {
		t.Fatalf("invalid action: %d %s", r.status, r.raw)
	}
	if r := a.call("POST", "/api/v3/memory/proposals", map[string]any{
		"conversationId": convID, "expectedVersion": convVersion,
	}); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("stale conversation: %d %s", r.status, r.raw)
	}
}

func TestV3MemoryProposalGatesAndIsolation(t *testing.T) {
	gen := &proposalGenerator{}
	e := proposalV3(t, proposalRouter(t, gen, proposalVerifier{}))
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")
	_, _, conv := savedConversation(t, a)
	convID := conv["id"].(string)

	// Consent gates: memory first, then AI processing.
	if r := a.call("POST", "/api/v3/memory/proposals", map[string]any{"conversationId": convID}); r.status != 403 ||
		r.code() != "consent_required" || r.field() != "personalMemory" {
		t.Fatalf("without memory consent: %d %s", r.status, r.raw)
	}
	grantMemoryConsent(t, a)
	if r := a.call("POST", "/api/v3/memory/proposals", map[string]any{"conversationId": convID}); r.status != 403 ||
		r.code() != "consent_required" || r.field() != "aiProcessing" {
		t.Fatalf("without AI consent: %d %s", r.status, r.raw)
	}
	a.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	created := a.must(200, "POST", "/api/v3/memory/proposals", map[string]any{"conversationId": convID}).data()
	proposals, _ := created["proposals"].([]any)
	if len(proposals) != 1 {
		t.Fatalf("proposals: %v", created)
	}
	id := proposals[0].(map[string]any)["id"].(string)

	// Another owner sees neither the conversation nor the proposal.
	if r := b.call("POST", "/api/v3/memory/proposals", map[string]any{"conversationId": convID}); r.status != 404 {
		t.Fatalf("foreign conversation: %d %s", r.status, r.raw)
	}
	if r := b.call("PATCH", "/api/v3/memory/"+id+"/review", map[string]any{"expectedVersion": 1, "action": "confirm"}); r.status != 404 ||
		r.code() != "not_found" {
		t.Fatalf("foreign review: %d %s", r.status, r.raw)
	}
	// A non-conversation record and an unsaved room are not proposal
	// sources.
	note := a.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "text": "Synthetic note.", "storageGranted": true,
	}).data()
	if r := a.call("POST", "/api/v3/memory/proposals", map[string]any{"conversationId": note["id"]}); r.status != 404 {
		t.Fatalf("note source: %d %s", r.status, r.raw)
	}
	unsaved := a.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	if r := a.call("POST", "/api/v3/memory/proposals", map[string]any{"conversationId": unsaved["id"]}); r.status != 404 {
		t.Fatalf("unsaved room: %d %s", r.status, r.raw)
	}
	empty := a.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true}).data()
	quiet := a.must(200, "POST", "/api/v3/memory/proposals", map[string]any{"conversationId": empty["recordId"]}).data()
	if quiet["aiState"] != "no_input" || len(quiet["proposals"].([]any)) != 0 {
		t.Fatalf("empty conversation: %v", quiet)
	}
	// A saved crisis pause is never a proposal source until the owner
	// explicitly continues.
	paused := a.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true}).data()
	pausedTurn := a.must(200, "POST", "/api/v3/conversations/"+paused["id"].(string)+"/turns", map[string]any{
		"text": "Aku mau bunuh diri malam ini.", "expectedVersion": 1,
	}).data()
	if r := a.call("POST", "/api/v3/memory/proposals", map[string]any{"conversationId": pausedTurn["recordId"]}); r.status != 409 ||
		r.code() != "invalid_transition" {
		t.Fatalf("paused conversation: %d %s", r.status, r.raw)
	}
	// Withdrawal blocks review; deletion remains available.
	a.must(200, "PUT", "/api/v3/me/consents/personal_memory", map[string]any{
		"granted": false, "policyVersion": policyVersion("personal_memory"),
	})
	if r := a.call("PATCH", "/api/v3/memory/"+id+"/review", map[string]any{"expectedVersion": 1, "action": "confirm"}); r.status != 403 ||
		r.code() != "consent_required" {
		t.Fatalf("review after withdrawal: %d %s", r.status, r.raw)
	}
	a.must(200, "DELETE", "/api/v3/memory/"+id, map[string]any{"expectedVersion": 1})
}

func TestV3MemoryProposalWithheldIsNeverStored(t *testing.T) {
	gen := &proposalGenerator{}
	e := proposalV3(t, proposalRouter(t, gen, proposalVerifier{withhold: true}))
	a := e.client()
	a.guest("18_plus")
	grantProposalConsents(t, a)
	_, _, conv := savedConversation(t, a)
	convID := conv["id"].(string)
	resp := a.must(200, "POST", "/api/v3/memory/proposals", map[string]any{"conversationId": convID}).data()
	if resp["aiState"] != "unavailable" || len(resp["proposals"].([]any)) != 0 {
		t.Fatalf("withheld response: %v", resp)
	}
	if n := e.count("SELECT count(*) FROM personal_records WHERE kind='memory'"); n != 0 {
		t.Fatalf("%d withheld proposals were stored", n)
	}
}

package httpapi_test

import (
	"testing"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/httpapi"
)

// guardianGateV3 adds the AI and sharing purposes this test needs and a
// router whose generator answers both AT-1 and AT-2.
func guardianGateV3(t *testing.T) *v3env {
	t.Helper()
	router, err := ai.NewRouter(ai.Config{Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus),
		OpTimeout: ai.DefaultOpTimeout}, ai.Deps{Meta: reflectionGenerator{}, Consent: reflectionConsent{}})
	if err != nil {
		t.Fatal(err)
	}
	return newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes,
				account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}},
				account.Purpose{Name: "sharing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
}

func grantGuardianGateConsents(t *testing.T, c *v3client) {
	t.Helper()
	for _, purpose := range []string{"personal_memory", "ai_processing", "sharing"} {
		c.must(200, "PUT", "/api/v3/me/consents/"+purpose, map[string]any{
			"granted": true, "policyVersion": policyVersion(purpose),
		})
	}
}

func recordGuardianConsent(t *testing.T, c *v3client) {
	t.Helper()
	c.must(200, "PUT", "/api/v3/me/guardian-consent", map[string]any{
		"relationship": "parent", "method": "guardian_present_attestation",
		"policyVersion": policyVersion("guardian_information"),
	})
}

// exerciseGatedWrites performs one successful write on every gated route.
// A failure means an adult or a consenting 15_17 owner was blocked or the
// route stopped working.
func exerciseGatedWrites(t *testing.T, c *v3client) {
	t.Helper()
	// Records: create, update and delete.
	note := c.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic gate note", "text": "Synthetic gate note text.", "storageGranted": true,
	}).data()
	noteID, noteVersion := note["id"].(string), int(note["version"].(float64))
	note = c.must(200, "PATCH", "/api/v3/records/"+noteID, map[string]any{
		"kind": "note", "title": "Synthetic gate note", "text": "Synthetic corrected gate note.", "expectedVersion": noteVersion, "storageGranted": true,
	}).data()
	noteVersion = int(note["version"].(float64))
	drop := c.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic drop note", "text": "Synthetic drop note text.", "storageGranted": true,
	}).data()
	c.must(200, "DELETE", "/api/v3/records/"+drop["id"].(string), map[string]any{"expectedVersion": 1})

	// Memory: create, update and delete.
	memory := c.must(201, "POST", "/api/v3/memory", map[string]any{
		"title": "Synthetic memory", "text": "Synthetic memory text.", "storageGranted": true,
	}).data()
	memoryID := memory["id"].(string)
	memory = c.must(200, "PATCH", "/api/v3/memory/"+memoryID, map[string]any{
		"title": "Synthetic memory", "text": "Synthetic corrected memory text.", "expectedVersion": 1, "storageGranted": true,
	}).data()
	c.must(200, "DELETE", "/api/v3/memory/"+memoryID, map[string]any{"expectedVersion": int(memory["version"].(float64))})

	// Consultation summary: create, update, approve, follow-up and delete.
	summary := c.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic summary", "text": "Synthetic summary text.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": noteVersion}},
	}).data()
	summaryID := summary["id"].(string)
	summary = c.must(200, "PATCH", "/api/v3/consultation-summaries/"+summaryID, map[string]any{
		"title": "Synthetic summary", "text": "Synthetic corrected summary.", "expectedVersion": 1, "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": noteVersion}},
	}).data()
	summaryVersion := int(summary["version"].(float64))
	summary = c.must(200, "POST", "/api/v3/consultation-summaries/"+summaryID+"/approve", map[string]any{
		"expectedVersion": summaryVersion,
	}).data()
	summaryVersion = int(summary["version"].(float64))
	followup := c.must(201, "POST", "/api/v3/consultation-summaries/"+summaryID+"/followups", map[string]any{
		"title": "Synthetic follow-up", "text": "Synthetic follow-up text.", "storageGranted": true,
		"sections": map[string]any{"feeling": "Synthetic feeling.", "remember": "", "agreed": "", "askNext": ""},
	}).data()
	c.must(200, "PATCH", "/api/v3/consultation-summaries/"+summaryID+"/followups/"+followup["id"].(string), map[string]any{
		"title": "Synthetic follow-up", "text": "Synthetic corrected follow-up text.", "expectedVersion": 1, "storageGranted": true,
		"sections": map[string]any{"feeling": "Synthetic corrected feeling.", "remember": "", "agreed": "", "askNext": ""},
	})

	// Share: create and revoke.
	created := c.must(201, "POST", "/api/v3/shares", map[string]any{
		"summaryId": summaryID, "label": "Synthetic recipient", "purpose": "consultation", "durationHours": 24,
	}).data()
	share := created["share"].(map[string]any)
	c.must(200, "POST", "/api/v3/shares/"+share["id"].(string)+"/revoke", map[string]any{"expectedVersion": 1})
	c.must(200, "DELETE", "/api/v3/consultation-summaries/"+summaryID, map[string]any{"expectedVersion": summaryVersion})

	// Conversations: create, turn, reflection create/review, storage and resume.
	room := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	roomID := room["id"].(string)
	sent := c.must(200, "POST", "/api/v3/conversations/"+roomID+"/turns", map[string]any{
		"text": "Aku capek banget sama kuliahku.", "expectedVersion": 1,
	}).data()
	version := int(sent["version"].(float64))
	cards := c.must(200, "POST", "/api/v3/conversations/"+roomID+"/reflections", map[string]any{
		"expectedVersion": version,
	}).data()
	reflections, _ := cards["reflections"].([]any)
	if len(reflections) == 0 {
		t.Fatalf("reflection cards: %v", cards)
	}
	reflectionID := reflections[0].(map[string]any)["id"].(string)
	reviewed := c.must(200, "PATCH", "/api/v3/conversations/"+roomID+"/reflections/"+reflectionID, map[string]any{
		"expectedVersion": int(cards["version"].(float64)), "action": "confirm",
	}).data()
	c.must(200, "POST", "/api/v3/conversations/"+roomID+"/resume", map[string]any{
		"expectedVersion": int(reviewed["version"].(float64)),
	})
	stored := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	c.must(200, "PATCH", "/api/v3/conversations/"+stored["id"].(string)+"/storage", map[string]any{
		"save": true, "expectedVersion": 1,
	})
}

// TestV3GuardianGateAllowsAdultAndConsentingMinor covers every gated write
// for an adult and for a 15_17 owner with an active guardian consent.
func TestV3GuardianGateAllowsAdultAndConsentingMinor(t *testing.T) {
	t.Parallel()
	e := guardianGateV3(t)
	for _, tc := range []struct {
		name  string
		band  string
		minor bool
	}{
		{"adult", "18_plus", false},
		{"consenting 15_17", "15_17", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := e.client()
			c.guest(tc.band)
			if tc.minor {
				recordGuardianConsent(t, c)
			}
			grantGuardianGateConsents(t, c)
			exerciseGatedWrites(t, c)
		})
	}
}

var gatedRouteBodies = []struct {
	method, path string
	body         any
}{
	{"POST", "/api/v3/records", map[string]any{"kind": "note", "text": "Synthetic.", "storageGranted": true}},
	{"PATCH", "/api/v3/records/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", map[string]any{"kind": "note", "text": "Synthetic.", "storageGranted": true, "expectedVersion": 1}},
	{"DELETE", "/api/v3/records/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", map[string]any{"expectedVersion": 1}},
	{"POST", "/api/v3/memory", map[string]any{"title": "Synthetic.", "text": "Synthetic.", "storageGranted": true}},
	{"PATCH", "/api/v3/memory/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", map[string]any{"title": "Synthetic.", "text": "Synthetic.", "storageGranted": true, "expectedVersion": 1}},
	{"DELETE", "/api/v3/memory/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", map[string]any{"expectedVersion": 1}},
	{"POST", "/api/v3/consultation-summaries", map[string]any{"title": "Synthetic.", "text": "Synthetic.", "storageGranted": true}},
	{"PATCH", "/api/v3/consultation-summaries/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", map[string]any{"title": "Synthetic.", "text": "Synthetic.", "storageGranted": true, "expectedVersion": 1}},
	{"DELETE", "/api/v3/consultation-summaries/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", map[string]any{"expectedVersion": 1}},
	{"POST", "/api/v3/consultation-summaries/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/approve", map[string]any{"expectedVersion": 1}},
	{"POST", "/api/v3/consultation-summaries/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/followups", map[string]any{"title": "Synthetic.", "text": "Synthetic.", "storageGranted": true}},
	{"PATCH", "/api/v3/consultation-summaries/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/followups/0b9c2d3e-4f50-4a61-8b72-9c8d7e6f5a40", map[string]any{"title": "Synthetic.", "text": "Synthetic.", "storageGranted": true, "expectedVersion": 1}},
	{"POST", "/api/v3/shares", map[string]any{"summaryId": "6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", "label": "Synthetic.", "purpose": "consultation", "durationHours": 24}},
	{"POST", "/api/v3/shares/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/revoke", map[string]any{"expectedVersion": 1}},
	{"POST", "/api/v3/conversations", map[string]any{"save": false}},
	{"POST", "/api/v3/conversations/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/turns", map[string]any{"text": "Synthetic.", "expectedVersion": 1}},
	{"POST", "/api/v3/conversations/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/resume", map[string]any{"expectedVersion": 1}},
	{"PATCH", "/api/v3/conversations/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/storage", map[string]any{"save": true, "expectedVersion": 1}},
	{"POST", "/api/v3/conversations/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/reflections", map[string]any{"expectedVersion": 1}},
	{"PATCH", "/api/v3/conversations/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11/reflections/00000000-0000-4000-8000-000000000000", map[string]any{"expectedVersion": 1, "action": "confirm"}},
}

// TestV3GuardianGateRefusesEveryGatedWrite: a 15_17 owner with no guardian
// consent gets 403 consent_required / guardianConsent before any handler
// reads a body or touches storage.
func TestV3GuardianGateRefusesEveryGatedWrite(t *testing.T) {
	t.Parallel()
	e := guardianGateV3(t)
	c := e.client()
	c.guest("15_17")
	for _, rt := range gatedRouteBodies {
		r := c.call(rt.method, rt.path, rt.body)
		if r.status != 403 || r.code() != "consent_required" || r.field() != "guardianConsent" {
			t.Errorf("%s %s: %d %s", rt.method, rt.path, r.status, r.raw)
		}
	}
	if n := e.count(`SELECT count(*) FROM personal_records`); n != 0 {
		t.Fatalf("%d records written behind the gate", n)
	}
	if n := e.count(`SELECT count(*) FROM shares`); n != 0 {
		t.Fatalf("%d shares written behind the gate", n)
	}
}

// TestV3GuardianGateWithdrawalIsImmediate: an active record opens the gate,
// and withdrawing it closes the gate on the next write.
func TestV3GuardianGateWithdrawalIsImmediate(t *testing.T) {
	t.Parallel()
	e := guardianGateV3(t)
	c := e.client()
	c.guest("15_17")
	recordGuardianConsent(t, c)
	c.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic", "text": "Synthetic before withdrawal.", "storageGranted": true,
	})
	c.must(200, "DELETE", "/api/v3/me/guardian-consent", nil)
	if r := c.call("POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic", "text": "Synthetic after withdrawal.", "storageGranted": true,
	}); r.status != 403 || r.code() != "consent_required" || r.field() != "guardianConsent" {
		t.Fatalf("after withdrawal: %d %s", r.status, r.raw)
	}
}

// TestV3GuardianGateLeavesUnknownOpen: unknown keeps its existing
// protective defaults; the gate adds nothing for it.
func TestV3GuardianGateLeavesUnknownOpen(t *testing.T) {
	t.Parallel()
	e := guardianGateV3(t)
	c := e.client()
	c.guest("unknown")
	c.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic", "text": "Synthetic unknown note.", "storageGranted": true,
	})
	c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false})
}

package httpapi_test

import (
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

func TestV3ShareLifecycle(t *testing.T) {
	e := newV3(t, v3opt{mutateC: func(c *account.Config) {
		c.Purposes = append(c.Purposes, account.Purpose{Name: "sharing", Kinds: []string{account.KindGuest, account.KindAccount}})
	}})
	owner := e.client()
	owner.guest("18_plus")
	owner.must(200, "PUT", "/api/v3/me/consents/sharing", map[string]any{
		"granted": true, "policyVersion": policyVersion("sharing"),
	})
	note := owner.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic source", "text": "Synthetic source text.", "storageGranted": true,
	}).data()
	summary := owner.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic summary", "text": "Synthetic summary for a recipient.", "storageGranted": true,
		"prepared":   map[string]any{"goal": "Synthetic goal"},
		"sourceRefs": []map[string]any{{"recordId": note["id"], "version": note["version"]}},
	}).data()
	id := summary["id"].(string)
	owner.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 1})

	created := owner.must(201, "POST", "/api/v3/shares", map[string]any{
		"summaryId": id, "label": "Dr. Synthetic", "purpose": "consultation", "durationHours": 24,
	}).data()
	share := created["share"].(map[string]any)
	token, code := created["token"].(string), created["code"].(string)
	if share["state"] != "active" || token == "" || len(code) != 8 {
		t.Fatalf("share create: %v", created)
	}
	list := owner.must(200, "GET", "/api/v3/shares", nil).data()["shares"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["label"] != "Dr. Synthetic" {
		t.Fatalf("shares: %v", list)
	}
	if _, leaked := list[0].(map[string]any)["token"]; leaked {
		t.Fatal("list leaks the token")
	}

	recipient := e.client()
	wrong := recipient.call("POST", "/api/v3/recipient/session", map[string]any{"token": token, "code": "WRONG123"})
	if wrong.status != 401 || wrong.code() != "invalid_share" {
		t.Fatalf("wrong code: %d %s", wrong.status, wrong.raw)
	}
	opened := recipient.must(200, "POST", "/api/v3/recipient/session", map[string]any{"token": token, "code": code}).data()
	projection := opened["share"].(map[string]any)
	if projection["text"] != "Synthetic summary for a recipient." || projection["label"] != "Dr. Synthetic" ||
		projection["prepared"].(map[string]any)["goal"] != "Synthetic goal" {
		t.Fatalf("projection: %v", projection)
	}
	if _, leaked := projection["summaryId"]; leaked {
		t.Fatal("projection leaks the owner summary id")
	}
	again := recipient.must(200, "GET", "/api/v3/recipient/share", nil).data()["share"].(map[string]any)
	if again["text"] != projection["text"] {
		t.Fatalf("recipient read: %v", again)
	}
	recipient.must(200, "POST", "/api/v3/recipient/acknowledge", nil)
	ack := recipient.must(200, "GET", "/api/v3/recipient/share", nil).data()["share"].(map[string]any)
	if ack["acknowledged"] != true {
		t.Fatalf("acknowledge: %v", ack)
	}

	owner.must(200, "POST", "/api/v3/shares/"+share["id"].(string)+"/revoke", map[string]any{"expectedVersion": 1})
	if r := recipient.call("GET", "/api/v3/recipient/share", nil); r.status != 401 || r.code() != "invalid_share" {
		t.Fatalf("recipient after revoke: %d %s", r.status, r.raw)
	}
}

func TestV3ShareNeedsConsentAndApproval(t *testing.T) {
	e := newV3(t, v3opt{mutateC: func(c *account.Config) {
		c.Purposes = append(c.Purposes, account.Purpose{Name: "sharing", Kinds: []string{account.KindGuest, account.KindAccount}})
	}})
	c := e.client()
	c.guest("18_plus")
	if r := c.call("POST", "/api/v3/shares", map[string]any{"summaryId": "00000000-0000-4000-8000-000000000000",
		"label": "Synthetic", "purpose": "consultation", "durationHours": 24}); r.status != 403 || r.code() != "consent_required" {
		t.Fatalf("without consent: %d %s", r.status, r.raw)
	}
	c.must(200, "PUT", "/api/v3/me/consents/sharing", map[string]any{
		"granted": true, "policyVersion": policyVersion("sharing"),
	})
	summary := c.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Draft summary", "text": "Synthetic draft.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": mustNote(t, c), "version": 1}},
	}).data()
	if r := c.call("POST", "/api/v3/shares", map[string]any{"summaryId": summary["id"],
		"label": "Synthetic", "purpose": "consultation", "durationHours": 24}); r.status != 409 || r.code() != "invalid_transition" {
		t.Fatalf("unapproved share: %d %s", r.status, r.raw)
	}
}

func mustNote(t *testing.T, c *v3client) string {
	t.Helper()
	n := c.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic", "text": "Synthetic source.", "storageGranted": true,
	}).data()
	return n["id"].(string)
}

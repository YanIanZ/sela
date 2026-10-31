package httpapi_test

import (
	"bytes"
	"testing"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/reading"
)

func newReadingEnv(t *testing.T) (*v3env, *admin.Store, *reading.ReviewStore) {
	t.Helper()
	key := bytes.Repeat([]byte{17}, 32)
	var adminStore *admin.Store
	var reviewStore *reading.ReviewStore
	e := newV3(t, v3opt{mutateO: func(e *v3env, o *httpapi.Options) {
		s, err := admin.New(e.db.Pool, admin.Options{Hasher: e.store.Hasher(), MFAKey: key, Now: e.clock.Now})
		if err != nil {
			t.Fatal(err)
		}
		r, err := reading.NewReviewStore(e.db.Pool, e.clock.Now)
		if err != nil {
			t.Fatal(err)
		}
		adminStore, reviewStore = s, r
		o.Admin, o.ReadingReviews = s, r
	}})
	return e, adminStore, reviewStore
}

func TestAdminReadingApprovalPublishesAndRevocationStopsIt(t *testing.T) {
	e, st, _ := newReadingEnv(t)
	owner := e.client()
	adminLogin(t, e, st, owner, "reading.owner", adminPassword, admin.RoleOwner)

	list := owner.must(200, "GET", "/api/admin/v1/reading", nil).data()
	items, _ := list["items"].([]any)
	if len(items) != 38 {
		t.Fatalf("admin reading items = %d, want all 38 embedded items", len(items))
	}
	first, _ := items[0].(map[string]any)
	detail, _ := first["detail"].(map[string]any)
	slug, _ := detail["id"].(string)
	hash, _ := first["contentHash"].(string)
	if slug == "" || len(hash) != 64 {
		t.Fatalf("first admin item = %+v", first)
	}
	if first["published"] != false || first["approval"] != nil {
		t.Fatalf("unapproved item reported as published: %+v", first)
	}
	embedded, _ := first["embeddedReview"].(map[string]any)
	if embedded["status"] != "machine_drafted_unreviewed" || embedded["verifierStatus"] != "not_run" {
		t.Fatalf("embedded review = %+v", embedded)
	}

	public := e.client().must(200, "GET", "/api/v3/reading", nil).data()
	if got := public["items"].([]any); len(got) != 0 {
		t.Fatalf("public list is not empty before approval: %v", got)
	}

	body := func(over map[string]any) map[string]any {
		b := map[string]any{
			"reviewerName":   "Editor Uji Manusia",
			"reviewedOn":     "2026-11-01",
			"note":           "read the exact current text",
			"verifierStatus": "supported",
			"contentHash":    hash,
			"confirm":        true,
		}
		for k, v := range over {
			b[k] = v
		}
		return b
	}
	path := "/api/admin/v1/reading/" + slug + "/approve"

	if r := owner.call("POST", path, body(map[string]any{"confirm": false})); r.status != 400 || r.code() != "confirmation_required" {
		t.Fatalf("missing confirmation: %d %s", r.status, r.raw)
	}
	if r := owner.call("POST", path, body(map[string]any{"contentHash": "deadbeef"})); r.status != 409 || r.code() != "reading_content_changed" {
		t.Fatalf("stale content hash: %d %s", r.status, r.raw)
	}
	if r := owner.call("POST", path, body(map[string]any{"verifierStatus": "not_run"})); r.status != 400 {
		t.Fatalf("unsupported verifier: %d %s", r.status, r.raw)
	}
	if r := owner.call("POST", "/api/admin/v1/reading/no-such-item/approve", body(nil)); r.status != 404 {
		t.Fatalf("unknown slug: %d %s", r.status, r.raw)
	}
	if got := e.client().must(200, "GET", "/api/v3/reading", nil).data()["items"].([]any); len(got) != 0 {
		t.Fatalf("a refused approval published the item: %v", got)
	}

	approved := owner.must(200, "POST", path, body(nil)).data()
	approval, _ := approved["approval"].(map[string]any)
	if approval["reviewerName"] != "Editor Uji Manusia" || approval["reviewedOn"] != "2026-11-01" ||
		approval["verifierStatus"] != "supported" || approval["contentHash"] != hash ||
		approval["approvedBy"] == "" || approval["revokedAt"] != nil {
		t.Fatalf("approval = %+v", approval)
	}

	public = e.client().must(200, "GET", "/api/v3/reading", nil).data()
	published, _ := public["items"].([]any)
	if len(published) != 1 || published[0].(map[string]any)["id"] != slug {
		t.Fatalf("public list after approval = %v", published)
	}
	if published[0].(map[string]any)["reviewedOn"] != "2026-11-01" {
		t.Fatalf("public review date = %v", published[0])
	}
	itemResp := e.client().must(200, "GET", "/api/v3/reading/"+slug, nil)
	review, _ := itemResp.data()["review"].(map[string]any)
	if review["status"] != "human_approved" || review["reviewedOn"] != "2026-11-01" || review["verifier"] != "supported" {
		t.Fatalf("public review = %+v", review)
	}
	if bytes.Contains(itemResp.raw, []byte("Editor Uji Manusia")) || bytes.Contains(itemResp.raw, []byte("read the exact current text")) {
		t.Fatal("public detail leaked the reviewer name or note")
	}
	if owner.must(200, "GET", "/api/admin/v1/reading", nil).data()["items"].([]any)[0].(map[string]any)["published"] != true {
		t.Fatal("admin list does not report the approved item as published")
	}

	owner.must(200, "POST", "/api/admin/v1/reading/"+slug+"/revoke", map[string]any{"reason": "withdrawn"})
	if got := e.client().must(200, "GET", "/api/v3/reading", nil).data()["items"].([]any); len(got) != 0 {
		t.Fatalf("revoked item stayed public: %v", got)
	}
	if r := e.client().call("GET", "/api/v3/reading/"+slug, nil); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("revoked detail: %d %s", r.status, r.raw)
	}
	// Revocation is idempotent.
	owner.must(200, "POST", "/api/admin/v1/reading/"+slug+"/revoke", map[string]any{})

	events := owner.must(200, "GET", "/api/admin/v1/reading/audit", nil).data()["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want one approve and one revoke: %v", len(events), events)
	}
	seen := map[string]bool{}
	for _, ev := range events {
		seen[ev.(map[string]any)["action"].(string)] = true
	}
	if !seen["reading.approve"] || !seen["reading.revoke"] {
		t.Fatalf("audit actions = %v", seen)
	}

	ops := e.client()
	adminLogin(t, e, st, ops, "reading.ops", adminPassword, admin.RoleOps)
	ops.must(403, "GET", "/api/admin/v1/reading", nil)
	ops.must(403, "POST", "/api/admin/v1/reading/"+slug+"/revoke", map[string]any{})
}

func TestAdminReadingReviewerRolesCanRead(t *testing.T) {
	e, st, _ := newReadingEnv(t)
	research := e.client()
	adminLogin(t, e, st, research, "reading.research", adminPassword, admin.RoleResearchEditor)
	research.must(200, "GET", "/api/admin/v1/reading", nil)

	safety := e.client()
	adminLogin(t, e, st, safety, "reading.safety", adminPassword, admin.RoleSafetyReviewer)
	safety.must(200, "GET", "/api/admin/v1/reading", nil)
}

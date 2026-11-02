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

	// The first embedded item fails only overridable readability checks,
	// so its approval needs the full checklist and an override reason.
	a11y, _ := first["accessibility"].(map[string]any)
	if a11y["version"] != reading.A11yChecksVersion || a11y["result"] != reading.A11yResultNeedsOverride {
		t.Fatalf("first item accessibility = %+v", a11y)
	}
	body := func(over map[string]any) map[string]any {
		b := map[string]any{
			"reviewerName":   "Editor Uji Manusia",
			"reviewedOn":     "2026-11-01",
			"note":           "read the exact current text",
			"verifierStatus": "supported",
			"contentHash":    hash,
			"confirm":        true,
			"accessibility":  a11yBody(overrideReason),
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
	if len(events) != 3 {
		t.Fatalf("audit events = %d, want approve, accessibility override and revoke: %v", len(events), events)
	}
	seen := map[string]bool{}
	for _, ev := range events {
		seen[ev.(map[string]any)["action"].(string)] = true
	}
	if !seen["reading.approve"] || !seen["reading.revoke"] || !seen["reading.a11y_override"] {
		t.Fatalf("audit actions = %v", seen)
	}

	ops := e.client()
	adminLogin(t, e, st, ops, "reading.ops", adminPassword, admin.RoleOps)
	ops.must(403, "GET", "/api/admin/v1/reading", nil)
	ops.must(403, "POST", "/api/admin/v1/reading/"+slug+"/revoke", map[string]any{})
}

const overrideReason = "Kalimat bantuan darurat dibiarkan utuh agar nomor 119 tidak terpisah."

func a11yBody(reason string) map[string]any {
	checks := map[string]any{}
	for _, id := range reading.ManualCheckIDs() {
		checks[id] = true
	}
	return map[string]any{
		"checksVersion":  reading.A11yChecksVersion,
		"manualVersion":  reading.A11yManualVersion,
		"manualChecks":   checks,
		"overrideReason": reason,
	}
}

func readingItemBySlug(t *testing.T, items []any, slug string) map[string]any {
	t.Helper()
	for _, raw := range items {
		it := raw.(map[string]any)
		if it["slug"] == slug {
			return it
		}
	}
	t.Fatalf("no admin reading item %q", slug)
	return nil
}

func TestAdminReadingAccessibilityGate(t *testing.T) {
	e, st, _ := newReadingEnv(t)
	owner := e.client()
	adminLogin(t, e, st, owner, "reading.a11y", adminPassword, admin.RoleOwner)

	list := owner.must(200, "GET", "/api/admin/v1/reading", nil).data()
	meta, _ := list["accessibility"].(map[string]any)
	checklist, _ := meta["manualChecklist"].([]any)
	if meta["checksVersion"] != reading.A11yChecksVersion || meta["manualVersion"] != reading.A11yManualVersion ||
		len(checklist) != len(reading.ManualChecklist) {
		t.Fatalf("accessibility meta = %+v", meta)
	}
	items := list["items"].([]any)
	results := map[string]int{}
	for _, raw := range items {
		a := raw.(map[string]any)["accessibility"].(map[string]any)
		results[a["result"].(string)]++
		if checks, _ := a["checks"].([]any); len(checks) != 9 {
			t.Fatalf("checks = %v", a["checks"])
		}
	}
	// No embedded item is blocked today; 10 pass and 28 need a reasoned
	// readability override (matching sela-plain-language-v1).
	if results[reading.A11yResultPass] != 10 || results[reading.A11yResultNeedsOverride] != 28 || results[reading.A11yResultBlocked] != 0 {
		t.Fatalf("results = %v", results)
	}

	clean := readingItemBySlug(t, items, "grounding-satu-menit-lima-indra")
	if clean["accessibility"].(map[string]any)["result"] != reading.A11yResultPass {
		t.Fatalf("clean item = %+v", clean["accessibility"])
	}
	needs := readingItemBySlug(t, items, "grounding-saat-badai-emosi")
	body := func(it map[string]any, a11y any) map[string]any {
		return map[string]any{
			"reviewerName": "Editor Uji Manusia", "reviewedOn": "2026-11-01", "note": "",
			"verifierStatus": "supported", "contentHash": it["contentHash"], "confirm": true,
			"accessibility": a11y,
		}
	}
	path := func(it map[string]any) string { return "/api/admin/v1/reading/" + it["slug"].(string) + "/approve" }
	expect := func(r resp, status int, code, field string) {
		t.Helper()
		if r.status != status || r.code() != code {
			t.Fatalf("got %d %s, want %d %s", r.status, r.raw, status, code)
		}
		if field != "" && !bytes.Contains(r.raw, []byte(`"field":"`+field+`"`)) {
			t.Fatalf("field: %s, want %s", r.raw, field)
		}
	}

	expect(owner.call("POST", path(clean), body(clean, nil)), 400, "accessibility_checklist_incomplete", "accessibility")
	stale := a11yBody("")
	stale["checksVersion"] = "sela-reading-a11y-v0"
	expect(owner.call("POST", path(clean), body(clean, stale)), 409, "accessibility_checks_changed", "accessibility.checksVersion")
	partial := a11yBody("")
	partial["manualChecks"].(map[string]any)["meaning_without_visuals"] = false
	expect(owner.call("POST", path(clean), body(clean, partial)), 400, "accessibility_checklist_incomplete", "accessibility.manualChecks.meaning_without_visuals")
	expect(owner.call("POST", path(needs), body(needs, a11yBody(""))), 422, "accessibility_override_required", "accessibility.overrideReason")
	expect(owner.call("POST", path(needs), body(needs, a11yBody("terlalu pendek"))), 422, "accessibility_override_required", "accessibility.overrideReason")
	expect(owner.call("POST", path(clean), body(clean, a11yBody(overrideReason))), 400, "invalid_input", "accessibility.overrideReason")

	if got := e.client().must(200, "GET", "/api/v3/reading", nil).data()["items"].([]any); len(got) != 0 {
		t.Fatalf("a refused approval published an item: %v", got)
	}
	if events := owner.must(200, "GET", "/api/admin/v1/reading/audit", nil).data()["events"].([]any); len(events) != 0 {
		t.Fatalf("refusals were audited as approvals: %v", events)
	}

	ok := owner.must(200, "POST", path(clean), body(clean, a11yBody(""))).data()["approval"].(map[string]any)
	rec := ok["accessibility"].(map[string]any)
	if rec["result"] != reading.A11yApprovedPass || rec["checksVersion"] != reading.A11yChecksVersion || rec["overrideReason"] != nil {
		t.Fatalf("pass record = %+v", rec)
	}
	ov := owner.must(200, "POST", path(needs), body(needs, a11yBody(overrideReason))).data()["approval"].(map[string]any)
	rec = ov["accessibility"].(map[string]any)
	if rec["result"] != reading.A11yApprovedOverridden || rec["overrideReason"] != overrideReason {
		t.Fatalf("override record = %+v", rec)
	}
	actions := map[string]int{}
	for _, ev := range owner.must(200, "GET", "/api/admin/v1/reading/audit", nil).data()["events"].([]any) {
		actions[ev.(map[string]any)["action"].(string)]++
	}
	if actions["reading.approve"] != 2 || actions["reading.a11y_override"] != 1 {
		t.Fatalf("audit actions = %v", actions)
	}
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

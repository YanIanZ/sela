package httpapi_test

import (
	"strings"
	"testing"
)

// continuityV3 uses the shared v3 test configuration, where the optional
// personal_memory purpose is active but not granted.
func continuityV3(t *testing.T) *v3env {
	t.Helper()
	return newV3(t, v3opt{})
}

func grantMemoryConsent(t *testing.T, c *v3client) {
	t.Helper()
	c.must(200, "PUT", "/api/v3/me/consents/personal_memory", map[string]any{
		"granted": true, "policyVersion": policyVersion("personal_memory"),
	})
}

func TestV3MemoryConsentOwnerAndAtRest(t *testing.T) {
	e := continuityV3(t)
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")

	view := a.must(200, "GET", "/api/v3/memory", nil).data()
	items, ok := view["items"].([]any)
	if view["enabled"] != false || !ok || len(items) != 0 {
		t.Fatalf("memory before consent: %v", view)
	}
	body := map[string]any{"title": "Synthetic memory", "text": "Synthetic memory body for at-rest check.", "storageGranted": true}
	if r := a.call("POST", "/api/v3/memory", body); r.status != 403 || r.code() != "consent_required" {
		t.Fatalf("memory write without consent: %d %s", r.status, r.raw)
	}
	noStorage := map[string]any{"title": "Synthetic memory", "text": "Synthetic memory body.", "storageGranted": false}
	if r := a.call("POST", "/api/v3/memory", noStorage); r.status != 403 || r.code() != "storage_required" {
		t.Fatalf("memory without storage choice: %d %s", r.status, r.raw)
	}

	grantMemoryConsent(t, a)
	created := a.must(201, "POST", "/api/v3/memory", body).data()
	id, _ := created["id"].(string)
	if id == "" || created["kind"] != "memory" || created["version"].(float64) != 1 || created["provenance"] != "user_original" {
		t.Fatalf("memory record: %v", created)
	}
	if e.count(`SELECT count(*) FROM personal_records WHERE id=$1 AND position(convert_to($2,'UTF8') in content_ct)=0`, id, body["text"]) != 1 ||
		e.count(`SELECT count(*) FROM personal_record_versions WHERE record_id=$1 AND position(convert_to($2,'UTF8') in content_ct)=0`, id, body["text"]) != 1 {
		t.Fatal("memory plaintext at rest")
	}

	view = a.must(200, "GET", "/api/v3/memory", nil).data()
	if view["enabled"] != true || len(view["items"].([]any)) != 1 {
		t.Fatalf("memory list: %v", view)
	}
	if b.must(200, "GET", "/api/v3/memory", nil).data()["enabled"] != false {
		t.Fatal("memory visible without consent")
	}
	patch := map[string]any{"title": "Synthetic memory", "text": "Synthetic corrected memory.", "expectedVersion": 1, "storageGranted": true}
	changed := a.must(200, "PATCH", "/api/v3/memory/"+id, patch).data()
	if changed["version"].(float64) != 2 || changed["provenance"] != "user_correction" {
		t.Fatalf("memory edit: %v", changed)
	}
	if r := a.call("PATCH", "/api/v3/memory/"+id, patch); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("stale memory edit: %d %s", r.status, r.raw)
	}
	if r := b.call("PATCH", "/api/v3/memory/"+id, patch); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("foreign memory edit: %d %s", r.status, r.raw)
	}
	if r := b.call("DELETE", "/api/v3/memory/"+id, map[string]any{"expectedVersion": 2}); r.status != 404 {
		t.Fatalf("foreign memory delete: %d %s", r.status, r.raw)
	}
	if r := a.call("DELETE", "/api/v3/memory/"+id, map[string]any{"expectedVersion": 1}); r.status != 409 {
		t.Fatalf("stale memory delete: %d %s", r.status, r.raw)
	}
	a.must(200, "DELETE", "/api/v3/memory/"+id, map[string]any{"expectedVersion": 2})
	a.must(404, "GET", "/api/v3/records/"+id, nil)

	// Revocation stops reads and writes at once; it does not destroy rows.
	a.must(201, "POST", "/api/v3/memory", body)
	a.must(200, "PUT", "/api/v3/me/consents/personal_memory", map[string]any{
		"granted": false, "policyVersion": policyVersion("personal_memory"),
	})
	view = a.must(200, "GET", "/api/v3/memory", nil).data()
	if view["enabled"] != false || len(view["items"].([]any)) != 0 {
		t.Fatalf("memory after revocation: %v", view)
	}
	if r := a.call("POST", "/api/v3/memory", body); r.status != 403 || r.code() != "consent_required" {
		t.Fatalf("memory write after revocation: %d %s", r.status, r.raw)
	}
}

func TestV3ConsultationSummaryApprovalFollowupsAndCascade(t *testing.T) {
	e := continuityV3(t)
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")

	note := a.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic source", "text": "Synthetic source text about a busy week.", "storageGranted": true,
	}).data()
	noteID := note["id"].(string)
	noteVersion := int(note["version"].(float64))
	base := map[string]any{
		"title": "Synthetic summary", "text": "Synthetic summary in my own words.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": noteVersion}},
		"prepared":   map[string]any{"goal": "Synthetic goal", "questions": "Synthetic question", "bring": "Synthetic list"},
	}
	created := a.must(201, "POST", "/api/v3/consultation-summaries", base).data()
	id := created["id"].(string)
	if created["kind"] != "summary" || created["summaryState"] != "draft" || created["provenance"] != "manual" ||
		created["prepared"].(map[string]any)["goal"] != "Synthetic goal" {
		t.Fatalf("summary create: %v", created)
	}
	if e.count(`SELECT count(*) FROM personal_records WHERE id=$1 AND position(convert_to($2,'UTF8') in content_ct)=0`, id, base["text"]) != 1 {
		t.Fatal("summary plaintext at rest")
	}

	approved := a.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 1}).data()
	if approved["summaryState"] != "approved" || approved["approvedVersion"].(float64) != 1 {
		t.Fatalf("approval: %v", approved)
	}
	if r := a.call("POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 2}); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("approval with wrong version: %d %s", r.status, r.raw)
	}

	// Any text edit revokes approval and creates a revision.
	patch := map[string]any{
		"title": "Synthetic summary", "text": "Synthetic summary corrected.", "expectedVersion": 1, "storageGranted": true,
		"sourceRefs": base["sourceRefs"], "prepared": base["prepared"],
	}
	draft := a.must(200, "PATCH", "/api/v3/consultation-summaries/"+id, patch).data()
	if draft["version"].(float64) != 2 || draft["summaryState"] != "draft" {
		t.Fatalf("edit must revoke approval: %v", draft)
	}
	if r := a.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=html", nil); r.status != 409 || r.code() != "invalid_transition" {
		t.Fatalf("draft export: %d %s", r.status, r.raw)
	}
	a.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 2})

	// A source edit invalidates the stored approval and blocks reapproval
	// until the summary pins the new revision.
	a.must(200, "PATCH", "/api/v3/records/"+noteID, map[string]any{
		"kind": "note", "title": "Synthetic source", "text": "Synthetic changed source text.", "expectedVersion": noteVersion, "storageGranted": true,
	})
	stale := a.must(200, "GET", "/api/v3/consultation-summaries/"+id, nil).data()
	if stale["summaryState"] != "draft" {
		t.Fatalf("stale source did not demote approval: %v", stale)
	}
	if r := a.call("POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 2}); r.status != 409 || r.code() != "stale_version" || r.field() != "sourceRefs" {
		t.Fatalf("approval with stale source: %d %s", r.status, r.raw)
	}
	repin := map[string]any{
		"title": "Synthetic summary", "text": "Synthetic summary final.", "expectedVersion": 2, "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": noteVersion + 1}},
	}
	a.must(200, "PATCH", "/api/v3/consultation-summaries/"+id, repin)
	a.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 3})

	// The approved summary exports as HTML or Markdown, with the owner's
	// words, the prepared sections and the pinned source label.
	htmlExport := a.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=html", nil)
	htmlBody := string(htmlExport.raw)
	if htmlExport.status != 200 || !strings.Contains(htmlBody, "Synthetic summary final.") ||
		!strings.Contains(htmlBody, "Synthetic goal") || !strings.Contains(htmlBody, "Synthetic source") ||
		!strings.Contains(htmlExport.header.Get("Content-Type"), "text/html") {
		t.Fatalf("html export: %d %s", htmlExport.status, htmlBody)
	}
	mdExport := a.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=md&lang=en", nil)
	mdBody := string(mdExport.raw)
	if mdExport.status != 200 || !strings.Contains(mdBody, "# Synthetic summary") ||
		!strings.Contains(mdBody, "## Chosen sources") || !strings.Contains(mdExport.header.Get("Content-Type"), "text/markdown") {
		t.Fatalf("markdown export: %d %s", mdExport.status, mdBody)
	}

	// Sources and summaries stay owner-isolated.
	foreign := map[string]any{
		"title": "Synthetic foreign summary", "text": "Synthetic foreign.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": noteVersion + 1}},
	}
	if r := b.call("POST", "/api/v3/consultation-summaries", foreign); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("foreign source accepted: %d %s", r.status, r.raw)
	}
	b.must(404, "GET", "/api/v3/consultation-summaries/"+id, nil)
	if r := b.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=html", nil); r.status != 404 {
		t.Fatalf("foreign export: %d %s", r.status, r.raw)
	}

	// The generic record surface stays note/check-in only.
	if r := a.call("POST", "/api/v3/records", map[string]any{"kind": "memory", "text": "Synthetic", "storageGranted": true}); r.status != 400 || r.field() != "kind" {
		t.Fatalf("generic memory create: %d %s", r.status, r.raw)
	}
	if r := a.call("PATCH", "/api/v3/records/"+id, map[string]any{"kind": "note", "text": "Synthetic", "expectedVersion": 3, "storageGranted": true}); r.status != 400 || r.field() != "kind" {
		t.Fatalf("generic summary patch: %d %s", r.status, r.raw)
	}
	if r := a.call("POST", "/api/v3/records", map[string]any{"kind": "followup", "text": "Synthetic", "storageGranted": true}); r.status != 400 || r.field() != "kind" {
		t.Fatalf("generic follow-up create: %d %s", r.status, r.raw)
	}

	// Follow-ups belong to one summary and are owner-isolated.
	followup := a.must(201, "POST", "/api/v3/consultation-summaries/"+id+"/followups", map[string]any{
		"title": "Synthetic follow-up", "text": "Synthetic after-consultation note.", "storageGranted": true,
	}).data()
	if followup["kind"] != "followup" || followup["parentId"] != id {
		t.Fatalf("follow-up: %v", followup)
	}
	list := a.must(200, "GET", "/api/v3/consultation-summaries/"+id+"/followups", nil).data()["followups"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["id"] != followup["id"] {
		t.Fatalf("follow-up list: %v", list)
	}
	b.must(404, "GET", "/api/v3/consultation-summaries/"+id+"/followups", nil)
	if r := b.call("POST", "/api/v3/consultation-summaries/"+id+"/followups", map[string]any{"text": "Synthetic", "storageGranted": true}); r.status != 404 {
		t.Fatalf("foreign follow-up create: %d %s", r.status, r.raw)
	}
	followupID := followup["id"].(string)

	// Deleting the summary cascades to the follow-up.
	a.must(200, "DELETE", "/api/v3/consultation-summaries/"+id, map[string]any{"expectedVersion": 3})
	a.must(404, "GET", "/api/v3/consultation-summaries/"+id, nil)
	a.must(404, "GET", "/api/v3/records/"+followupID, nil)
	summaries := a.must(200, "GET", "/api/v3/consultation-summaries", nil).data()["summaries"].([]any)
	if len(summaries) != 0 {
		t.Fatalf("deleted summary listed: %v", summaries)
	}
}

package httpapi_test

import (
	"testing"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/admin"
)

const scenarioBSource = "Kadang aku ingin sendiri, tapi bukan berarti membenci teman."

// TestMeaningCorrectionPropagates is scenario B: a deliberately wrong AI
// interpretation (a test fixture, not a live Meta response) is confirmed,
// cited by a summary, shared with a recipient and sent to a clinic; the
// owner then corrects it. Every descendant must stop being current in the
// same transaction and no new read may succeed; a re-approved new version
// can be shared again; a later source edit invalidates the new chain.
func TestMeaningCorrectionPropagates(t *testing.T) {
	e, st := newNetworkEnv(t, false)

	// A simulated clinic with a practitioner who can read summaries.
	ops := e.client()
	adminLogin(t, e, st, ops, "owner.mie", adminPassword, admin.RoleOwner)
	org := ops.must(201, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "clinic", "name": "Klinik Simulasi MIE", "modes": []string{"online"}, "synthetic": true,
	}).data()["org"].(map[string]any)
	orgID := org["id"].(string)
	ops.must(200, "POST", "/api/admin/v1/network/orgs/"+orgID+"/status", map[string]any{"status": "verified"})
	doc := e.client()
	partnerLogin(t, e, st, doc, admin.PartnerInput{Username: "psy.mie", Password: adminPassword, Role: admin.RolePractitioner,
		CareOrgID: orgID, DisplayName: "Psikolog MIE", ProfessionalType: "psychologist"})

	owner := e.client()
	uid := userID(t, owner.guest("18_plus"))
	for _, purpose := range []string{"sharing", "personal_memory"} {
		owner.must(200, "PUT", "/api/v3/me/consents/"+purpose, map[string]any{"granted": true, "policyVersion": policyVersion(purpose)})
	}
	note := owner.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Minggu ini", "text": "Akhir-akhir ini capek. " + scenarioBSource, "storageGranted": true,
	}).data()
	noteID := note["id"].(string)

	// The deliberately wrong interpretation C@v1, as AT-3 would store it.
	props, err := e.store.CreateMemoryProposals(bg, uid, []account.MemoryProposalInput{{
		Text:       "Kamu menghindari semua teman.",
		SourceRefs: []account.SourceRef{{RecordID: noteID, Version: 1, Quote: scenarioBSource, TurnKey: ""}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	memID := props[0].ID
	// Meaning Diff flags the drift before anyone confirms it.
	if m := props[0].Meaning; m == nil || !m.Critical {
		t.Fatalf("meaning check missed the drift: %+v", props[0].Meaning)
	}
	if got := owner.must(200, "GET", "/api/v3/memory/"+memID, nil).data()["meaning"].(map[string]any); got["critical"] != true {
		t.Fatalf("meaning not returned: %v", got)
	}
	owner.must(200, "PATCH", "/api/v3/memory/"+memID+"/review", map[string]any{"expectedVersion": 1, "action": "confirm"})

	// A statement whose quote is not in the source is refused (MIE-I02).
	if r := owner.call("POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Ringkasan", "text": "Teks.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": 1}},
		"items":      []map[string]any{{"text": "Pernyataan.", "quote": "Kalimat yang tidak pernah ditulis.", "sourceRef": map[string]any{"recordId": noteID, "version": 1}}},
	}); r.status != 400 || r.code() != "quote_not_in_source" {
		t.Fatalf("foreign quote: %d %s", r.status, r.raw)
	}
	// A claim that is not confirmed cannot be cited.
	if r := owner.call("POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Ringkasan", "text": "Teks.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": 1}},
		"items": []map[string]any{{"text": "Pernyataan.", "quote": scenarioBSource, "sourceRef": map[string]any{"recordId": noteID, "version": 1},
			"claimRef": map[string]any{"recordId": memID, "version": 1}}},
	}); r.status != 409 || r.code() != "claim_not_usable" {
		t.Fatalf("old claim version: %d %s", r.status, r.raw)
	}

	summary := owner.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Untuk konsultasi", "text": "Ringkasan sintetis.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": 1}},
		"items": []map[string]any{{"text": "Aku menghindari semua teman.", "quote": scenarioBSource,
			"sourceRef": map[string]any{"recordId": noteID, "version": 1}, "claimRef": map[string]any{"recordId": memID, "version": 2}}},
	}).data()
	sumID := summary["id"].(string)
	owner.must(200, "POST", "/api/v3/consultation-summaries/"+sumID+"/approve", map[string]any{"expectedVersion": 1})

	created := owner.must(201, "POST", "/api/v3/shares", map[string]any{"summaryId": sumID, "label": "Konselor", "purpose": "consultation", "durationHours": 24}).data()
	token, code := created["token"].(string), created["code"].(string)
	recipient := e.client()
	recipient.must(200, "POST", "/api/v3/recipient/session", map[string]any{"token": token, "code": code})
	recipient.must(200, "GET", "/api/v3/recipient/share", nil)

	careReq := owner.must(201, "POST", "/api/v3/care-requests", map[string]any{"summaryId": sumID, "orgId": orgID, "mode": "online"}).data()["request"].(map[string]any)
	careID := careReq["id"].(string)
	if v := doc.must(200, "GET", "/api/admin/v1/partner/requests/"+careID, nil).data()["request"].(map[string]any); v["summary"] == nil {
		t.Fatal("partner cannot read the summary before the correction")
	}

	// The preview counts what the correction will reach, and writes nothing.
	impact := owner.must(200, "GET", "/api/v3/meaning/impact?id="+memID, nil).data()["impact"].(map[string]any)
	if impact["summaries"].(float64) != 1 || impact["shares"].(float64) != 2 || impact["careRequests"].(float64) != 1 {
		t.Fatalf("impact preview: %v", impact)
	}
	if e.count(`SELECT count(*) FROM mie_events`) != 0 {
		t.Fatal("the preview wrote an event")
	}

	// The owner corrects the interpretation.
	owner.must(200, "PATCH", "/api/v3/memory/"+memID+"/review", map[string]any{
		"expectedVersion": 2, "action": "correct", "text": "Kamu sesekali butuh waktu sendiri dan tetap suka bertemu teman.",
	})

	// No new read of the old projection succeeds after the commit.
	if r := recipient.call("GET", "/api/v3/recipient/share", nil); r.status != 401 || r.code() != "invalid_share" {
		t.Fatalf("recipient after correction: %d %s", r.status, r.raw)
	}
	if r := e.client().call("POST", "/api/v3/recipient/session", map[string]any{"token": token, "code": code}); r.status != 401 {
		t.Fatalf("reopen after correction: %d %s", r.status, r.raw)
	}
	partnerView := doc.must(200, "GET", "/api/admin/v1/partner/requests/"+careID, nil).data()["request"].(map[string]any)
	if partnerView["state"] != "cancelled" || partnerView["contentAvailable"] != false || partnerView["summary"] != nil {
		t.Fatalf("partner after correction: %v", partnerView)
	}
	stale := owner.must(200, "GET", "/api/v3/consultation-summaries/"+sumID, nil).data()
	if stale["summaryState"] != "draft" || stale["stale"] == nil || stale["stale"].(map[string]any)["reason"] != "claim_corrected" {
		t.Fatalf("summary after correction: %v", stale)
	}
	if r := owner.call("POST", "/api/v3/consultation-summaries/"+sumID+"/approve", map[string]any{"expectedVersion": 1}); r.status != 409 {
		t.Fatalf("re-approve stale version: %d %s", r.status, r.raw)
	}
	if r := owner.call("POST", "/api/v3/shares", map[string]any{"summaryId": sumID, "label": "Lagi", "purpose": "consultation", "durationHours": 24}); r.status != 409 {
		t.Fatalf("share stale summary: %d %s", r.status, r.raw)
	}
	if r := owner.call("GET", "/api/v3/consultation-summaries/"+sumID+"/export?format=md", nil); r.status != 409 {
		t.Fatalf("export stale summary: %d %s", r.status, r.raw)
	}
	if e.count(`SELECT count(*) FROM shares WHERE end_reason='meaning_invalidated'`) != 2 {
		t.Fatal("both shares were not revoked as meaning_invalidated")
	}
	events := owner.must(200, "GET", "/api/v3/meaning/events", nil).data()["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["reason"] != "claim_corrected" {
		t.Fatalf("events: %v", events)
	}
	// The corrected claim itself is current and clean; its source is current.
	lineage := owner.must(200, "GET", "/api/v3/meaning/lineage?id="+memID, nil).data()["lineage"].(map[string]any)
	if lineage["stale"] != nil || len(lineage["parents"].([]any)) != 1 || lineage["parents"].([]any)[0].(map[string]any)["current"] != true {
		t.Fatalf("lineage: %v", lineage)
	}

	// Re-approval: a new version citing the corrected claim can be shared.
	owner.must(200, "PATCH", "/api/v3/consultation-summaries/"+sumID, map[string]any{
		"expectedVersion": 1, "title": "Untuk konsultasi", "text": "Ringkasan sintetis.", "storageGranted": true,
		"items": []map[string]any{{"text": "Aku sesekali butuh waktu sendiri dan tetap suka bertemu teman.", "quote": scenarioBSource,
			"sourceRef": map[string]any{"recordId": noteID, "version": 1}, "claimRef": map[string]any{"recordId": memID, "version": 3}}},
	})
	owner.must(200, "POST", "/api/v3/consultation-summaries/"+sumID+"/approve", map[string]any{"expectedVersion": 2})
	again := owner.must(201, "POST", "/api/v3/shares", map[string]any{"summaryId": sumID, "label": "Konselor", "purpose": "consultation", "durationHours": 24}).data()
	second := e.client()
	second.must(200, "POST", "/api/v3/recipient/session", map[string]any{"token": again["token"], "code": again["code"]})

	// A source edit invalidates the new chain: claim, summary and share.
	preview := owner.must(200, "GET", "/api/v3/meaning/impact?id="+noteID, nil).data()["impact"].(map[string]any)
	if preview["memories"].(float64) != 1 || preview["summaries"].(float64) != 1 || preview["shares"].(float64) != 1 {
		t.Fatalf("source impact preview: %v", preview)
	}
	owner.must(200, "PATCH", "/api/v3/records/"+noteID, map[string]any{
		"kind": "note", "title": "Minggu ini", "text": "Aku sudah tidak capek lagi.", "storageGranted": true, "expectedVersion": 1,
	})
	if r := second.call("GET", "/api/v3/recipient/share", nil); r.status != 401 {
		t.Fatalf("recipient after source edit: %d %s", r.status, r.raw)
	}
	mem := owner.must(200, "GET", "/api/v3/memory/"+memID, nil).data()
	if mem["stale"] == nil || mem["stale"].(map[string]any)["reason"] != "source_edited" {
		t.Fatalf("memory after source edit: %v", mem)
	}
	if e.count(`SELECT count(*) FROM mie_events WHERE reason='source_edited'`) != 1 {
		t.Fatal("source edit event missing")
	}
}

func TestMeaningSourceDeletionKeepsHistoryButStopsShares(t *testing.T) {
	e, _ := newNetworkEnv(t, false)
	owner, summaryID := approvedSummary(t, e, true)
	created := owner.must(201, "POST", "/api/v3/shares", map[string]any{"summaryId": summaryID, "label": "Konselor", "purpose": "consultation", "durationHours": 24}).data()
	recipient := e.client()
	recipient.must(200, "POST", "/api/v3/recipient/session", map[string]any{"token": created["token"], "code": created["code"]})
	src := owner.must(200, "GET", "/api/v3/consultation-summaries/"+summaryID, nil).data()["sourceRefs"].([]any)[0].(map[string]any)
	owner.must(200, "DELETE", "/api/v3/records/"+src["recordId"].(string)+"?derived=keep", map[string]any{"expectedVersion": 1})
	if r := recipient.call("GET", "/api/v3/recipient/share", nil); r.status != 401 {
		t.Fatalf("recipient after source deletion: %d %s", r.status, r.raw)
	}
	kept := owner.must(200, "GET", "/api/v3/consultation-summaries/"+summaryID, nil).data()
	if kept["summaryState"] != "draft" || kept["stale"].(map[string]any)["reason"] != "source_deleted" {
		t.Fatalf("kept summary: %v", kept)
	}
}

package httpapi_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/admin"
)

// Synthetic Care Passport fixtures; none is real data.
const (
	passportSourceText   = "Minggu ini aku capek setelah kuliah. Kadang aku ingin sendiri dulu. Wulan Prameswari bilang itu wajar."
	passportPrivateText  = "RAHASIA-B catatan yang tidak dipilih untuk dibagikan."
	passportPrivateTitle = "Catatan B tidak dipilih"
	passportPrivateName  = "Wulan Prameswari"
)

// recipientFields keeps the content fields a recipient receives, so a
// frozen share projection and the live preview can be compared exactly.
func recipientFields(m map[string]any) string {
	out := map[string]any{}
	for _, k := range []string{"summaryVersion", "title", "text", "prepared", "items", "sources"} {
		out[k] = m[k]
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}

// TestV3CarePassport covers BW-005/006/009 through the public routes: the
// preview is the recipient projection (CP-01, BW-T07), private names are
// masked, a passport can stay private without the sharing consent
// (BW-T13), the export preset escapes and refuses non-approved versions,
// foreign ids are indistinguishable (CP-T05), a source edit makes it
// needs_review and ends the share and the synthetic care request (CP-03,
// BW-T05/T17), and reapproval yields a fresh version while the old share
// stays ended (BW-T06).
func TestV3CarePassport(t *testing.T) {
	e, st := newNetworkEnv(t, false)

	ops := e.client()
	adminLogin(t, e, st, ops, "owner.passport", adminPassword, admin.RoleOwner)
	org := ops.must(201, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "clinic", "name": "Klinik Paspor Simulasi", "modes": []string{"online"}, "synthetic": true,
	}).data()["org"].(map[string]any)
	orgID := org["id"].(string)
	ops.must(200, "POST", "/api/admin/v1/network/orgs/"+orgID+"/status", map[string]any{"status": "verified"})
	doc := e.client()
	partnerLogin(t, e, st, doc, admin.PartnerInput{Username: "psy.passport", Password: adminPassword, Role: admin.RolePractitioner,
		CareOrgID: orgID, DisplayName: "Psikolog Simulasi", ProfessionalType: "psychologist"})
	desk := e.client()
	partnerLogin(t, e, st, desk, admin.PartnerInput{Username: "desk.passport", Password: adminPassword, Role: admin.RolePartnerStaff,
		CareOrgID: orgID, DisplayName: "Meja depan simulasi"})

	owner := e.client()
	owner.guest("18_plus")
	other := e.client()
	other.guest("18_plus")
	owner.must(201, "POST", "/api/v3/me/private-names", map[string]any{"name": passportPrivateName})

	noteA := owner.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Catatan A", "text": passportSourceText, "storageGranted": true,
	}).data()
	noteB := owner.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": passportPrivateTitle, "text": passportPrivateText, "storageGranted": true,
	}).data()
	aID, bID := noteA["id"].(string), noteB["id"].(string)
	srcA := map[string]any{"recordId": aID, "version": 1}
	summary := owner.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Persiapan sintetis", "text": "", "storageGranted": true,
		"sourceRefs": []map[string]any{srcA},
		"prepared": map[string]any{
			"goal":      "Aku ingin bisa menjelaskan rasa capekku.",
			"questions": "Apa yang sebaiknya dibahas dulu?\nBagaimana cerita ke Wulan Prameswari?",
			"bring":     "",
		},
		"items": []map[string]any{{
			"text": "Aku butuh waktu tenang setelah kuliah.", "quote": "Kadang aku ingin sendiri dulu.", "sourceRef": srcA,
		}, {
			"text": "Temanku menganggapnya wajar.", "quote": "Wulan Prameswari bilang itu wajar.", "sourceRef": srcA,
		}},
	}).data()
	sumID := summary["id"].(string)
	base := "/api/v3/consultation-summaries/" + sumID

	// A draft previews but is neither shareable nor exportable as approved.
	draft := owner.must(200, "GET", base+"/passport", nil).data()["passport"].(map[string]any)
	if draft["state"] != "draft" || draft["shareable"] != false || draft["summaryVersion"] != float64(1) {
		t.Fatalf("draft passport: %v", draft)
	}
	if r := owner.call("GET", base+"/export?format=html&preset=passport", nil); r.status != 409 || r.code() != "invalid_transition" {
		t.Fatalf("draft passport export: %d %s", r.status, r.raw)
	}
	if r := owner.call("GET", base+"/export?format=html&preset=clinical", nil); r.status != 400 || r.field() != "preset" {
		t.Fatalf("unknown preset: %d %s", r.status, r.raw)
	}

	owner.must(200, "POST", base+"/approve", map[string]any{"expectedVersion": 1})
	previewResp := owner.must(200, "GET", base+"/passport", nil)
	preview := previewResp.data()["passport"].(map[string]any)
	if preview["state"] != "approved" || preview["shareable"] != true || preview["approvedVersion"] != float64(1) {
		t.Fatalf("approved passport: %v", preview)
	}
	sources := preview["sources"].([]any)
	if len(sources) != 1 || sources[0].(map[string]any)["recordId"] != aID || sources[0].(map[string]any)["current"] != true ||
		sources[0].(map[string]any)["label"] != "Catatan A" {
		t.Fatalf("passport sources: %v", sources)
	}
	recipient := preview["recipient"].(map[string]any)
	// CP-01 / BW-T07: the unselected source never reaches the preview.
	for _, leak := range []string{bID, passportPrivateTitle, "RAHASIA-B", passportPrivateName, "Wulan"} {
		if strings.Contains(recipientFields(recipient), leak) {
			t.Fatalf("recipient preview leaks %q: %s", leak, recipientFields(recipient))
		}
	}
	if strings.Contains(string(previewResp.raw), bID) || strings.Contains(string(previewResp.raw), "RAHASIA-B") {
		t.Fatalf("passport response leaks the unselected source: %s", previewResp.raw)
	}
	if q := recipient["prepared"].(map[string]any)["questions"].(string); !strings.Contains(q, "[NAMA-1]") {
		t.Fatalf("private name not masked in the recipient preview: %q", q)
	}
	items := recipient["items"].([]any)
	if len(items) != 2 || items[1].(map[string]any)["quote"] != "[NAMA-1] bilang itu wajar." ||
		items[0].(map[string]any)["sourceLabel"] != "Catatan A · v1" {
		t.Fatalf("recipient items: %v", items)
	}
	if _, leaked := recipient["summaryId"]; leaked {
		t.Fatal("recipient preview carries the owner summary id")
	}

	// The passport preset renders the same projection, escaped.
	for _, format := range []string{"html", "md"} {
		r := owner.call("GET", base+"/export?format="+format+"&preset=passport", nil)
		body := string(r.raw)
		if r.status != 200 || !strings.Contains(r.header.Get("Content-Disposition"), "sela-persiapan-konsultasi."+format) ||
			!strings.Contains(body, "Ringkasan untuk dibawa") || !strings.Contains(body, "bukan paspor medis") ||
			!strings.Contains(body, "NAMA-1") {
			t.Fatalf("passport %s export: %d %s", format, r.status, body)
		}
		for _, leak := range []string{bID, passportPrivateTitle, "RAHASIA-B", passportPrivateName} {
			if strings.Contains(body, leak) {
				t.Fatalf("passport %s export leaks %q", format, leak)
			}
		}
		if format == "html" && r.header.Get("Content-Security-Policy") == "" {
			t.Fatal("passport html export without CSP")
		}
	}

	// BW-T13: without the sharing consent the passport stays private; the
	// share is refused and no recipient token exists.
	if r := owner.call("POST", "/api/v3/shares", map[string]any{"summaryId": sumID, "label": "Konselor", "purpose": "consultation", "durationHours": 24}); r.status != 403 || r.code() != "consent_required" {
		t.Fatalf("share without consent: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM shares WHERE summary_id::text=$1`, sumID); n != 0 {
		t.Fatalf("a share row exists without consent: %d", n)
	}
	owner.must(200, "PUT", "/api/v3/me/consents/sharing", map[string]any{"granted": true, "policyVersion": policyVersion("sharing")})

	// The recipient receives exactly the previewed fields.
	created := owner.must(201, "POST", "/api/v3/shares", map[string]any{"summaryId": sumID, "label": "Konselor", "purpose": "consultation", "durationHours": 24}).data()
	reader := e.client()
	opened := reader.must(200, "POST", "/api/v3/recipient/session", map[string]any{"token": created["token"], "code": created["code"]}).data()["share"].(map[string]any)
	if got, want := recipientFields(opened), recipientFields(recipient); got != want {
		t.Fatalf("recipient differs from the preview:\n got %s\nwant %s", got, want)
	}

	// The synthetic care partner reads the same projection; staff never do.
	careReq := owner.must(201, "POST", "/api/v3/care-requests", map[string]any{"summaryId": sumID, "orgId": orgID, "mode": "online"}).data()["request"].(map[string]any)
	careID := careReq["id"].(string)
	if careReq["orgSynthetic"] != true {
		t.Fatalf("care request to a synthetic org not labelled: %v", careReq)
	}
	partnerView := doc.must(200, "GET", "/api/admin/v1/partner/requests/"+careID, nil).data()["request"].(map[string]any)
	if partnerView["synthetic"] != true {
		t.Fatalf("partner view not synthetic: %v", partnerView)
	}
	if got, want := recipientFields(partnerView["summary"].(map[string]any)), recipientFields(recipient); got != want {
		t.Fatalf("partner projection differs from the preview:\n got %s\nwant %s", got, want)
	}
	if _, ok := desk.must(200, "GET", "/api/admin/v1/partner/requests/"+careID, nil).data()["request"].(map[string]any)["summary"]; ok {
		t.Fatal("partner staff reads the passport content")
	}

	// CP-T05: another owner's id is indistinguishable from an unknown one.
	foreign := other.call("GET", base+"/passport", nil)
	unknown := other.call("GET", "/api/v3/consultation-summaries/00000000-0000-4000-8000-000000000000/passport", nil)
	malformed := other.call("GET", "/api/v3/consultation-summaries/nope/passport", nil)
	for _, r := range []resp{foreign, unknown, malformed} {
		if r.status != 404 || r.code() != "not_found" || r.field() != foreign.field() {
			t.Fatalf("foreign/unknown passport: %d %s", r.status, r.raw)
		}
	}
	if r := other.call("GET", base+"/export?format=md&preset=passport", nil); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("foreign passport export: %d %s", r.status, r.raw)
	}

	// CP-03: editing the selected source makes the passport needs_review;
	// export, new shares and every later recipient read are refused, and
	// the synthetic care request is cancelled.
	owner.must(200, "PATCH", "/api/v3/records/"+aID, map[string]any{
		"kind": "note", "title": "Catatan A", "text": passportSourceText + " Sekarang sedikit lebih baik.", "storageGranted": true, "expectedVersion": 1,
	})
	stale := owner.must(200, "GET", base+"/passport", nil).data()["passport"].(map[string]any)
	if stale["state"] != "needs_review" || stale["shareable"] != false || stale["stale"] == nil ||
		stale["sources"].([]any)[0].(map[string]any)["current"] != false {
		t.Fatalf("passport after source edit: %v", stale)
	}
	for _, format := range []string{"html", "md"} {
		if r := owner.call("GET", base+"/export?format="+format+"&preset=passport", nil); r.status != 409 || r.code() != "invalid_transition" {
			t.Fatalf("stale passport %s export: %d %s", format, r.status, r.raw)
		}
	}
	if r := owner.call("POST", "/api/v3/shares", map[string]any{"summaryId": sumID, "label": "Lagi", "purpose": "consultation", "durationHours": 24}); r.status != 409 {
		t.Fatalf("share stale passport: %d %s", r.status, r.raw)
	}
	if r := reader.call("GET", "/api/v3/recipient/share", nil); r.status != 401 || r.code() != "invalid_share" {
		t.Fatalf("recipient after source edit: %d %s", r.status, r.raw)
	}
	cancelled := doc.must(200, "GET", "/api/admin/v1/partner/requests/"+careID, nil).data()["request"].(map[string]any)
	if cancelled["state"] != "cancelled" || cancelled["summary"] != nil {
		t.Fatalf("care request after source edit: %v", cancelled)
	}
	if r := owner.call("POST", base+"/approve", map[string]any{"expectedVersion": 1}); r.status != 409 {
		t.Fatalf("re-approve stale passport: %d %s", r.status, r.raw)
	}

	// BW-T06: re-pinning the corrected source and approving gives a fresh
	// version; the old share stays ended.
	srcA2 := map[string]any{"recordId": aID, "version": 2}
	owner.must(200, "PATCH", base, map[string]any{
		"title": "Persiapan sintetis", "text": "", "storageGranted": true, "expectedVersion": 1,
		"sourceRefs": []map[string]any{srcA2},
		"items": []map[string]any{{
			"text": "Aku butuh waktu tenang setelah kuliah.", "quote": "Kadang aku ingin sendiri dulu.", "sourceRef": srcA2,
		}},
	})
	owner.must(200, "POST", base+"/approve", map[string]any{"expectedVersion": 2})
	fresh := owner.must(200, "GET", base+"/passport", nil).data()["passport"].(map[string]any)
	if fresh["state"] != "approved" || fresh["summaryVersion"] != float64(2) || fresh["stale"] != nil ||
		fresh["recipient"].(map[string]any)["summaryVersion"] != float64(2) {
		t.Fatalf("reapproved passport: %v", fresh)
	}
	shares := owner.must(200, "GET", "/api/v3/shares", nil).data()["shares"].([]any)
	for _, sh := range shares {
		m := sh.(map[string]any)
		if m["summaryVersion"] == float64(1) && (m["state"] == "active" || m["endReason"] != "meaning_invalidated") {
			t.Fatalf("old share not ended after reapproval: %v", m)
		}
	}
	if r := reader.call("GET", "/api/v3/recipient/share", nil); r.status != 401 {
		t.Fatalf("old recipient after reapproval: %d %s", r.status, r.raw)
	}
	owner.must(200, "GET", base+"/export?format=html&preset=passport", nil)
}

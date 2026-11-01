package httpapi_test

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/carenet"
	"github.com/YanIanZ/sela/internal/httpapi"
)

func newNetworkEnv(t *testing.T, allowReal bool) (*v3env, *admin.Store) {
	t.Helper()
	key := bytes.Repeat([]byte{9}, 32)
	var st *admin.Store
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: "sharing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(e *v3env, o *httpapi.Options) {
			s, err := admin.New(e.db.Pool, admin.Options{Hasher: e.store.Hasher(), MFAKey: key, Now: e.clock.Now})
			if err != nil {
				t.Fatal(err)
			}
			st = s
			o.Admin = s
			n, err := carenet.New(e.db.Pool, e.clock.Now, allowReal)
			if err != nil {
				t.Fatal(err)
			}
			o.Network = n
		},
	})
	return e, st
}

func partnerLogin(t *testing.T, e *v3env, st *admin.Store, c *v3client, in admin.PartnerInput) {
	t.Helper()
	_, secretText, err := st.CreatePartner(bg, in)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := admin.DecodeTOTPSecret(secretText)
	if err != nil {
		t.Fatal(err)
	}
	c.must(200, "POST", "/api/admin/v1/sessions", map[string]any{
		"username": in.Username, "password": in.Password, "totp": admin.CodeAt(secret, e.clock.Now()),
	})
}

// approvedSummary creates a guest with sharing consent and one approved
// consultation summary, and returns the client and the summary id.
func approvedSummary(t *testing.T, e *v3env, consent bool) (*v3client, string) {
	t.Helper()
	owner := e.client()
	owner.guest("18_plus")
	if consent {
		owner.must(200, "PUT", "/api/v3/me/consents/sharing", map[string]any{"granted": true, "policyVersion": policyVersion("sharing")})
	}
	note := owner.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic source", "text": "Synthetic source text.", "storageGranted": true,
	}).data()
	summary := owner.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic summary for a clinic", "text": "Synthetic summary text.", "storageGranted": true,
		"prepared":   map[string]any{"goal": "Synthetic goal"},
		"sourceRefs": []map[string]any{{"recordId": note["id"], "version": note["version"]}},
	}).data()
	id := summary["id"].(string)
	owner.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 1})
	return owner, id
}

func TestCareNetworkEndToEnd(t *testing.T) {
	e, st := newNetworkEnv(t, false)
	ops := e.client()
	adminLogin(t, e, st, ops, "owner.net", adminPassword, admin.RoleOwner)

	// Real partners are refused while the deployment allows simulated only.
	ops.must(403, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "clinic", "name": "Real Clinic", "modes": []string{"online"}, "synthetic": false,
	})
	org := ops.must(201, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "clinic", "name": "Klinik Teduh (simulasi)", "city": "Bandung", "modes": []string{"online", "in_person"},
		"stages": []string{"attention", "support"}, "synthetic": true,
	}).data()["org"].(map[string]any)
	orgID := org["id"].(string)
	if org["status"] != "pending" {
		t.Fatalf("new org status = %v", org["status"])
	}
	// A synthetic organisation may be verified by its creator.
	ops.must(200, "POST", "/api/admin/v1/network/orgs/"+orgID+"/status", map[string]any{"status": "verified"})

	other := ops.must(201, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "practice", "name": "Praktik Lain (simulasi)", "modes": []string{"online"}, "synthetic": true,
	}).data()["org"].(map[string]any)
	ops.must(200, "POST", "/api/admin/v1/network/orgs/"+other["id"].(string)+"/status", map[string]any{"status": "verified"})

	doc := e.client()
	partnerLogin(t, e, st, doc, admin.PartnerInput{Username: "psy.demo", Password: adminPassword, Role: admin.RolePractitioner,
		CareOrgID: orgID, DisplayName: "Psikolog Demo", ProfessionalType: "psychologist", LicenseNo: "STR-SIM-0001"})
	desk := e.client()
	partnerLogin(t, e, st, desk, admin.PartnerInput{Username: "desk.demo", Password: adminPassword, Role: admin.RolePartnerStaff,
		CareOrgID: orgID, DisplayName: "Front desk"})
	outsider := e.client()
	partnerLogin(t, e, st, outsider, admin.PartnerInput{Username: "psy.other", Password: adminPassword, Role: admin.RolePractitioner,
		CareOrgID: other["id"].(string), DisplayName: "Psikolog Lain", ProfessionalType: "psychologist"})

	// Internal roles cannot be created as partners and vice versa.
	if _, _, err := st.Create(bg, "bad.partner", adminPassword, admin.RolePractitioner, ""); err == nil {
		t.Fatal("Create accepted a partner role without an organisation")
	}

	// Partners cannot reach internal routes; internal staff cannot act as a partner.
	doc.must(403, "GET", "/api/admin/v1/network/orgs", nil)
	doc.must(403, "GET", "/api/admin/v1/overview", nil)
	doc.must(403, "POST", "/api/admin/v1/network/members/psy.demo/verify-license", nil)
	ops.must(403, "GET", "/api/admin/v1/partner/requests", nil)

	// Licence verification by a different operator.
	reviewer := e.client()
	adminLogin(t, e, st, reviewer, "reviewer.net", adminPassword, admin.RoleDirectoryReviewer)
	reviewer.must(200, "POST", "/api/admin/v1/network/members/psy.demo/verify-license", nil)
	reviewer.must(409, "POST", "/api/admin/v1/network/members/psy.demo/verify-license", nil)

	// The person: no sharing consent means no request.
	noConsent, sum0 := approvedSummary(t, e, false)
	noConsent.must(403, "POST", "/api/v3/care-requests", map[string]any{"summaryId": sum0, "orgId": orgID, "mode": "online"})

	owner, summaryID := approvedSummary(t, e, true)
	network := owner.must(200, "GET", "/api/v3/care-network", nil).data()
	orgs := network["orgs"].([]any)
	if len(orgs) != 2 {
		t.Fatalf("care network lists %d orgs, want 2", len(orgs))
	}
	var practitionerID string
	for _, o := range orgs {
		m := o.(map[string]any)
		if m["id"] == orgID {
			pr := m["practitioners"].([]any)
			if len(pr) != 1 || pr[0].(map[string]any)["licenseVerified"] != true {
				t.Fatalf("practitioners = %v", pr)
			}
			if _, leaked := pr[0].(map[string]any)["licenseNo"]; leaked {
				t.Fatal("public practitioner exposes the licence number")
			}
			practitionerID = pr[0].(map[string]any)["id"].(string)
		}
	}
	owner.must(400, "POST", "/api/v3/care-requests", map[string]any{"summaryId": summaryID, "orgId": orgID, "mode": "teleport"})
	created := owner.must(201, "POST", "/api/v3/care-requests", map[string]any{
		"summaryId": summaryID, "orgId": orgID, "practitionerId": practitionerID, "mode": "online",
	}).data()["request"].(map[string]any)
	reqID := created["id"].(string)
	if created["state"] != "sent" || !regexp.MustCompile(`^SR-[A-Z0-9]{6}$`).MatchString(created["reference"].(string)) {
		t.Fatalf("created = %v", created)
	}

	// Staff see the request but never the summary.
	list := desk.must(200, "GET", "/api/admin/v1/partner/requests", nil).data()["requests"].([]any)
	if len(list) != 1 {
		t.Fatalf("partner list = %d", len(list))
	}
	staffView := desk.must(200, "GET", "/api/admin/v1/partner/requests/"+reqID, nil).data()
	if _, ok := staffView["request"].(map[string]any)["summary"]; ok || staffView["canReadContent"] != false {
		t.Fatalf("staff view leaks the summary: %v", staffView)
	}
	desk.must(403, "POST", "/api/admin/v1/partner/requests/"+reqID+"/respond", map[string]any{"decision": "decline", "declineReason": "other", "expectedVersion": 1})

	// Another organisation cannot see it.
	outsider.must(404, "GET", "/api/admin/v1/partner/requests/"+reqID, nil)
	if n := len(outsider.must(200, "GET", "/api/admin/v1/partner/requests", nil).data()["requests"].([]any)); n != 0 {
		t.Fatalf("outsider sees %d requests", n)
	}

	// The practitioner opens the summary; the owner's access history shows it.
	view := doc.must(200, "GET", "/api/admin/v1/partner/requests/"+reqID, nil).data()["request"].(map[string]any)
	summary, ok := view["summary"].(map[string]any)
	if !ok || summary["title"] != "Synthetic summary for a clinic" {
		t.Fatalf("practitioner summary = %v", view["summary"])
	}
	if e.count(`SELECT count(*) FROM share_events se JOIN care_requests r ON r.share_id = se.share_id WHERE r.id::text=$1 AND se.kind='opened'`, reqID) != 1 {
		t.Fatal("opening was not recorded for the owner")
	}
	if e.count(`SELECT count(*) FROM admin_audit WHERE action='care_request.open' AND object_id=$1`, reqID) != 1 {
		t.Fatal("opening was not audited")
	}

	slot1 := e.clock.Now().Add(48 * time.Hour).Truncate(time.Minute)
	slot2 := e.clock.Now().Add(72 * time.Hour).Truncate(time.Minute)
	doc.must(400, "POST", "/api/admin/v1/partner/requests/"+reqID+"/respond", map[string]any{
		"decision": "propose", "expectedVersion": 1,
		"proposal": map[string]any{"slots": []string{e.clock.Now().Add(-time.Hour).Format(time.RFC3339)}, "mode": "online", "contact": "WhatsApp klinik"},
	})
	proposed := doc.must(200, "POST", "/api/admin/v1/partner/requests/"+reqID+"/respond", map[string]any{
		"decision": "propose", "expectedVersion": 1,
		"proposal": map[string]any{"slots": []string{slot1.Format(time.RFC3339), slot2.Format(time.RFC3339)}, "mode": "online",
			"contact": "WhatsApp klinik simulasi, sebut kode permintaan", "note": "Sesi pertama 45 menit."},
	}).data()["request"].(map[string]any)
	if proposed["state"] != "proposed" {
		t.Fatalf("after propose: %v", proposed["state"])
	}

	mine := owner.must(200, "GET", "/api/v3/care-requests/"+reqID, nil).data()["request"].(map[string]any)
	prop := mine["proposal"].(map[string]any)
	if len(prop["slots"].([]any)) != 2 || prop["contact"] == "" {
		t.Fatalf("owner proposal = %v", prop)
	}
	version := int(mine["version"].(float64))
	owner.must(400, "POST", "/api/v3/care-requests/"+reqID+"/confirm", map[string]any{"slot": e.clock.Now().Add(96 * time.Hour).Format(time.RFC3339), "expectedVersion": version})
	confirmed := owner.must(200, "POST", "/api/v3/care-requests/"+reqID+"/confirm", map[string]any{"slot": slot1.Format(time.RFC3339), "expectedVersion": version}).data()["request"].(map[string]any)
	if confirmed["state"] != "confirmed" {
		t.Fatalf("after confirm: %v", confirmed["state"])
	}
	done := doc.must(200, "POST", "/api/admin/v1/partner/requests/"+reqID+"/complete", map[string]any{"expectedVersion": int(confirmed["version"].(float64))}).data()["request"].(map[string]any)
	if done["state"] != "completed" {
		t.Fatalf("after complete: %v", done["state"])
	}

	// Cancelling revokes the share: the organisation can no longer read it.
	second := owner.must(201, "POST", "/api/v3/care-requests", map[string]any{"summaryId": summaryID, "orgId": orgID, "mode": "either"}).data()["request"].(map[string]any)
	secondID := second["id"].(string)
	owner.must(200, "POST", "/api/v3/care-requests/"+secondID+"/cancel", map[string]any{"expectedVersion": 1})
	after := doc.must(200, "GET", "/api/admin/v1/partner/requests/"+secondID, nil).data()["request"].(map[string]any)
	if after["state"] != "cancelled" || after["contentAvailable"] != false || after["summary"] != nil {
		t.Fatalf("after cancel: %v", after)
	}

	// Revoking the share from the share list also ends an open request.
	third := owner.must(201, "POST", "/api/v3/care-requests", map[string]any{"summaryId": summaryID, "orgId": orgID, "mode": "online"}).data()["request"].(map[string]any)
	owner.must(200, "POST", "/api/v3/shares/"+third["shareId"].(string)+"/revoke", map[string]any{"expectedVersion": 1})
	if got := owner.must(200, "GET", "/api/v3/care-requests/"+third["id"].(string), nil).data()["request"].(map[string]any)["state"]; got != "cancelled" {
		t.Fatalf("after share revoke: %v", got)
	}

	// A suspended organisation takes no new requests.
	ops.must(200, "POST", "/api/admin/v1/network/orgs/"+orgID+"/status", map[string]any{"status": "suspended"})
	if code := owner.must(409, "POST", "/api/v3/care-requests", map[string]any{"summaryId": summaryID, "orgId": orgID, "mode": "online"}).code(); code != "care_org_unavailable" {
		t.Fatalf("suspended org code = %s", code)
	}
}

func TestCareNetworkRealPartnersNeedAcknowledgement(t *testing.T) {
	e, st := newNetworkEnv(t, true)
	creator := e.client()
	adminLogin(t, e, st, creator, "owner.real", adminPassword, admin.RoleOwner)
	org := creator.must(201, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "clinic", "name": "Klinik Nyata", "modes": []string{"in_person"}, "synthetic": false,
	}).data()["org"].(map[string]any)
	// The creator of a real organisation never verifies it.
	creator.must(403, "POST", "/api/admin/v1/network/orgs/"+org["id"].(string)+"/status", map[string]any{"status": "verified"})
	reviewer := e.client()
	adminLogin(t, e, st, reviewer, "reviewer.real", adminPassword, admin.RoleDirectoryReviewer)
	reviewer.must(200, "POST", "/api/admin/v1/network/orgs/"+org["id"].(string)+"/status", map[string]any{"status": "verified"})
}

func TestCommunityDecisionsAreAudited(t *testing.T) {
	e, st := newNetworkEnv(t, false)
	for _, action := range []string{"community.restore", "community.remove"} {
		if err := st.Audit(bg, nil, action, "community_post", "00000000-0000-4000-8000-000000000000", "held", "published", ""); err != nil {
			t.Fatalf("audit %s refused: %v", action, err)
		}
	}
	if e.count(`SELECT count(*) FROM admin_audit WHERE action LIKE 'community.%'`) != 2 {
		t.Fatal("community decisions were not stored")
	}
}

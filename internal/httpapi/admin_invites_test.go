package httpapi_test

import (
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/admin"
)

// TestPartnerInviteActivation: an owner invites a practitioner from the
// console; the invitee activates with their own password and authenticator,
// lands signed in on the partner workspace, and the link cannot be reused.
func TestPartnerInviteActivation(t *testing.T) {
	e, st := newNetworkEnv(t, false)
	ops := e.client()
	adminLogin(t, e, st, ops, "owner.inv", adminPassword, admin.RoleOwner)

	// Empty operator lists are arrays, never null (the console waited on
	// "Loading…" forever when the directory was empty).
	if raw := string(ops.must(200, "GET", "/api/admin/v1/directory", nil).raw); !strings.Contains(raw, `"entries":[]`) {
		t.Fatalf("empty directory: %s", raw)
	}

	org := ops.must(201, "POST", "/api/admin/v1/network/orgs", map[string]any{
		"kind": "clinic", "name": "Klinik Undangan (simulasi)", "modes": []string{"online"}, "synthetic": true,
	}).data()["org"].(map[string]any)
	orgID := org["id"].(string)
	ops.must(200, "POST", "/api/admin/v1/network/orgs/"+orgID+"/status", map[string]any{"status": "verified"})

	ops.must(400, "POST", "/api/admin/v1/network/invites", map[string]any{"username": "psy.inv", "role": "practitioner", "careOrgId": orgID})
	created := ops.must(201, "POST", "/api/admin/v1/network/invites", map[string]any{
		"username": "psy.inv", "role": "practitioner", "careOrgId": orgID, "displayName": "Psikolog Undangan",
		"professionalType": "psychologist", "licenseNo": "STR-SIM-9",
	}).data()
	link := created["activationUrl"].(string)
	if !strings.Contains(link, "/operator/activate#") {
		t.Fatalf("activation url: %s", link)
	}
	token := link[strings.Index(link, "#")+1:]
	ops.must(409, "POST", "/api/admin/v1/network/invites", map[string]any{
		"username": "psy.inv", "role": "partner_staff", "careOrgId": orgID,
	})
	if n := len(ops.must(200, "GET", "/api/admin/v1/network/orgs/"+orgID+"/invites", nil).data()["invites"].([]any)); n != 1 {
		t.Fatalf("invites listed: %d", n)
	}
	tasks := ops.must(200, "GET", "/api/admin/v1/tasks", nil).data()["tasks"].(map[string]any)
	if tasks["network"].(map[string]any)["openInvites"].(float64) != 1 {
		t.Fatalf("tasks: %v", tasks)
	}

	invitee := e.client()
	invitee.must(410, "POST", "/api/admin/v1/activation", map[string]any{"token": "not-a-token"})
	act := invitee.must(200, "POST", "/api/admin/v1/activation", map[string]any{"token": token}).data()["activation"].(map[string]any)
	if act["username"] != "psy.inv" || act["orgName"] != "Klinik Undangan (simulasi)" || act["secret"] == "" {
		t.Fatalf("activation: %v", act)
	}
	// A reload returns the same secret.
	again := invitee.must(200, "POST", "/api/admin/v1/activation", map[string]any{"token": token}).data()["activation"].(map[string]any)
	if again["secret"] != act["secret"] {
		t.Fatal("secret changed on reload")
	}
	secret, err := admin.DecodeTOTPSecret(act["secret"].(string))
	if err != nil {
		t.Fatal(err)
	}
	code := admin.CodeAt(secret, e.clock.Now())
	invitee.must(400, "POST", "/api/admin/v1/activation/complete", map[string]any{"token": token, "password": "short", "totp": code})
	invitee.must(401, "POST", "/api/admin/v1/activation/complete", map[string]any{"token": token, "password": adminPassword, "totp": "000000"})
	signed := invitee.must(200, "POST", "/api/admin/v1/activation/complete", map[string]any{"token": token, "password": adminPassword, "totp": code}).data()["admin"].(map[string]any)
	if signed["partner"] != true || signed["careOrgId"] != orgID {
		t.Fatalf("signed in as: %v", signed)
	}
	invitee.must(200, "GET", "/api/admin/v1/partner/org", nil)
	members := invitee.must(200, "GET", "/api/admin/v1/partner/members", nil).data()["members"].([]any)
	if len(members) != 1 || members[0].(map[string]any)["licenseNo"] != nil {
		t.Fatalf("practitioner sees: %v", members)
	}
	// The link is single use.
	e.client().must(410, "POST", "/api/admin/v1/activation", map[string]any{"token": token})
	if e.count(`SELECT count(*) FROM admin_audit WHERE action IN ('network.invite_create','network.invite_accept')`) != 2 {
		t.Fatal("invitation not audited")
	}

	// Revoked invitations cannot be activated.
	staff := ops.must(201, "POST", "/api/admin/v1/network/invites", map[string]any{"username": "desk.inv", "role": "partner_staff", "careOrgId": orgID}).data()
	ops.must(200, "POST", "/api/admin/v1/network/invites/"+staff["invite"].(map[string]any)["id"].(string)+"/revoke", nil)
	tok := staff["activationUrl"].(string)
	e.client().must(410, "POST", "/api/admin/v1/activation", map[string]any{"token": tok[strings.Index(tok, "#")+1:]})

	// Partners never manage invitations.
	invitee.must(403, "POST", "/api/admin/v1/network/invites", map[string]any{"username": "x.inv", "role": "partner_staff", "careOrgId": orgID})
	invitee.must(403, "GET", "/api/admin/v1/tasks", nil)
}

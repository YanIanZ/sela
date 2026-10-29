package account_test

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

// snapshot is every row one user owns (and their key row), as text.
func snapshot(t *testing.T, f *fixture, id account.UserID) string {
	t.Helper()
	var lines []string
	for _, tbl := range append(dbtest.OwnedTables(t, f.db.Pool), "users") {
		col := "user_id"
		if tbl == "users" {
			col = "id"
		}
		rows, err := f.db.Pool.Query(ctx, "SELECT t::text FROM "+db.QuoteIdent(tbl)+" t WHERE "+col+" = $1", id)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, tbl+" "+s)
		}
		rows.Close()
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestIsolation runs every owner-facing method as user B and checks that
// user A's rows are byte-for-byte unchanged, that B never sees A's data,
// and that A's resources answer not found to B.
func TestIsolation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, err := f.store.Register(ctx, account.RegisterInput{Username: "pemilik.a", Password: markerPassword, Locale: "id",
		AgeBand: account.AgeBand15to17, TermsPolicyVersion: termsV})
	if err != nil {
		t.Fatal(err)
	}
	ap := f.principal(a.Token)
	name := markerGuardian
	if _, err := f.store.SetGuardianConsent(ctx, ap.UserID, ap.SessionID, account.GuardianInput{Relationship: "parent",
		Method: "guardian_present_attestation", GuardianName: &name, PolicyVersion: guardianV}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.SetConsent(ctx, ap.UserID, ap.SessionID, "test_optional", true, optionalV); err != nil {
		t.Fatal(err)
	}
	second, err := f.store.Login(ctx, account.LoginInput{Username: "pemilik.a", Password: markerPassword})
	if err != nil {
		t.Fatal(err)
	}
	aSecond := f.principal(second.Token)
	before := snapshot(t, f, ap.UserID)

	b := f.register("pengguna.b")
	bp := f.principal(b.Token)
	B, cur := bp.UserID, bp.SessionID

	// A's session id is not found for B, and stays live.
	if _, err := f.store.RevokeSession(ctx, B, cur, aSecond.SessionID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("revoke another user's session: %v", err)
	}
	// Every other owner method, run as B, touches only B.
	if p, err := f.store.Profile(ctx, B); err != nil || p.ID == ap.UserID.String() {
		t.Fatalf("profile: %v", err)
	}
	en := "en"
	if _, err := f.store.UpdateProfile(ctx, B, cur, account.ProfileUpdate{ExpectedVersion: 1, Locale: &en}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AcknowledgeRecoveryCodes(ctx, B, cur); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Reauthenticate(ctx, B, cur, markerPassword); err != nil {
		t.Fatal(err)
	}
	sessions, err := f.store.Sessions(ctx, B, cur)
	if err != nil || len(sessions) != 1 || sessions[0].ID != cur.String() {
		t.Fatalf("sessions: %+v %v", sessions, err)
	}
	if _, _, err := f.store.SetConsent(ctx, B, cur, "test_optional", true, optionalV); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.SetConsent(ctx, B, cur, "test_optional", false, optionalV); err != nil {
		t.Fatal(err)
	}
	if cs, err := f.store.Consents(ctx, B); err != nil || len(cs) != 3 {
		t.Fatalf("consents: %+v %v", cs, err)
	}
	hist, _, err := f.store.ConsentHistory(ctx, B, "", account.Page{})
	if err != nil || len(hist) != 3 {
		t.Fatalf("history: %d %v", len(hist), err)
	}
	if _, err := f.store.SetGuardianConsent(ctx, B, cur, account.GuardianInput{Relationship: "parent", Method: "guardian_present_attestation",
		PolicyVersion: guardianV}); !errors.Is(err, account.ErrInvalidTransition) {
		t.Fatalf("guardian for 18_plus: %v", err)
	}
	if err := f.store.WithdrawGuardianConsent(ctx, B, cur); err != nil {
		t.Fatal(err)
	}
	if g, err := f.store.GuardianConsent(ctx, B); err != nil || g.Active != nil || len(g.History) != 0 || g.Status != "not_applicable" {
		t.Fatalf("guardian view: %+v %v", g, err)
	}
	events, _, err := f.store.AuditEvents(ctx, B, cur, account.Page{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var aEvents int
	_ = f.db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE user_id = $1`, B).Scan(&aEvents)
	if len(events) != aEvents {
		t.Fatalf("audit list %d, own events %d", len(events), aEvents)
	}
	exp, err := f.store.Export(ctx, B, cur, "json", "en")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(exp.Body), ap.UserID.String()) || strings.Contains(string(exp.Body), "pemilik.a") ||
		strings.Contains(string(exp.Body), aSecond.SessionID.String()) || strings.Contains(string(exp.Body), markerGuardian) {
		t.Fatal("B's export contains A's data")
	}
	codes, _, err := f.store.RegenerateRecoveryCodes(ctx, B, cur, markerPassword)
	if err != nil || len(codes) != 10 {
		t.Fatal(err)
	}
	cp, err := f.store.ChangePassword(ctx, B, cur, markerPassword, newPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	cur = f.principal(cp.Token).SessionID
	if _, err := f.store.LogoutAll(ctx, B, cur, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.UpgradeGuest(ctx, B, cur, account.UpgradeInput{Username: "x.y.z", Password: newPassword}); !errors.Is(err, account.ErrNotGuest) {
		t.Fatalf("upgrade an account: %v", err)
	}
	if _, err := f.store.Reauthenticate(ctx, B, cur, newPassword); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Logout(ctx, B, aSecond.SessionID); err != nil {
		t.Fatal(err) // scoped to B: A's session id simply matches nothing
	}
	if _, err := f.store.DeleteAccount(ctx, B, cur); err != nil {
		t.Fatal(err)
	}

	if after := snapshot(t, f, ap.UserID); after != before {
		t.Fatalf("user A changed:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if _, err := f.store.Authenticate(ctx, second.Token); err != nil {
		t.Fatalf("A's second session ended: %v", err)
	}

	// An owner id that does not exist reads and writes nothing.
	ghost := account.NewUserID()
	if _, err := f.store.Profile(ctx, ghost); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("ghost profile: %v", err)
	}
	if _, err := f.store.RevokeSession(ctx, ghost, cur, ap.SessionID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("ghost revoke: %v", err)
	}
	if _, _, err := f.store.SetConsent(ctx, ghost, cur, "test_optional", true, optionalV); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("ghost consent: %v", err)
	}
	if _, err := f.store.DeleteAccount(ctx, ghost, ap.SessionID); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("ghost delete with A's session id: %v", err)
	}
	if after := snapshot(t, f, ap.UserID); after != before {
		t.Fatal("user A changed after ghost calls")
	}
}

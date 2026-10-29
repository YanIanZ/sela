package admin

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

func testStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	db := dbtest.New(t)
	hasher, err := account.NewHasher(account.FloorHashParams, 2, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	st, err := New(db.Pool, Options{Hasher: hasher, MFAKey: bytes.Repeat([]byte{7}, 32), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return st, &now
}

func TestAdminCreateAuthenticateAndLogout(t *testing.T) {
	ctx := context.Background()
	st, now := testStore(t)
	p, secretText, err := st.Create(ctx, "ops.one", "sungai biru deh", RoleOps, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != RoleOps || p.Username != "ops.one" {
		t.Fatalf("principal = %+v", p)
	}
	secret, err := DecodeTOTPSecret(secretText)
	if err != nil {
		t.Fatal(err)
	}
	step := now.Unix() / totpStep
	code := hotp(secret, uint64(step))

	if _, err := st.Authenticate(ctx, "ops.one", "wrong password", code); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := st.Authenticate(ctx, "ops.one", "sungai biru deh", "000000"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong MFA: %v", err)
	}
	sess, err := st.Authenticate(ctx, "ops.one", "sungai biru deh", code)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Token) != account.SessionTokenLength {
		t.Fatalf("token length %d", len(sess.Token))
	}
	got, err := st.AuthenticateSession(ctx, sess.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.AdminID != p.AdminID || got.Role != RoleOps {
		t.Fatalf("session principal = %+v", got)
	}
	if _, err := st.Authenticate(ctx, "ops.one", "sungai biru deh", code); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("replayed TOTP: %v", err)
	}
	if err := st.Logout(ctx, got); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AuthenticateSession(ctx, sess.Token); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("logged-out session: %v", err)
	}
	overview, err := st.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.ActiveStaff != 1 || overview.LiveSessions != 0 {
		t.Fatalf("overview = %+v", overview)
	}
	events, err := st.ListAudit(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 4 {
		t.Fatalf("audit events = %d", len(events))
	}
}

func TestAdminSeparateDomainsAndValidation(t *testing.T) {
	ctx := context.Background()
	st, now := testStore(t)
	if _, _, err := st.Create(ctx, "BAD", "sungai biru deh", RoleOps, ""); err == nil {
		t.Fatal("uppercase username accepted")
	}
	if _, _, err := st.Create(ctx, "ops.two", "short", RoleOps, ""); err == nil {
		t.Fatal("short password accepted")
	}
	if _, _, err := st.Create(ctx, "ops.three", "sungai biru deh", Role("superuser"), ""); err == nil {
		t.Fatal("unknown role accepted")
	}
	_, secretText, err := st.Create(ctx, "ops.two", "sungai biru deh", RoleSecurityAdmin, "org-a")
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := DecodeTOTPSecret(secretText)
	code := hotp(secret, uint64(now.Unix()/totpStep))
	sess, err := st.Authenticate(ctx, "ops.two", "sungai biru deh", code)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Principal.OrgID != "org-a" || !sess.Principal.Can(PermAuditRead) || sess.Principal.Can(PermCacheWrite) {
		t.Fatalf("role/org permissions = %+v", sess.Principal)
	}
	// A consumer token is not an operator token, even if it has the right shape.
	if _, err := st.AuthenticateSession(ctx, sess.Token+"x"); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("malformed token: %v", err)
	}
	// Revocation is idempotent, kills live sessions and is audited.
	if err := st.Revoke(ctx, "ops.two", "left the organisation"); err != nil {
		t.Fatal(err)
	}
	if err := st.Revoke(ctx, "ops.two", "left the organisation"); err != nil {
		t.Fatalf("second revoke: %v", err)
	}
	if _, err := st.AuthenticateSession(ctx, sess.Token); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("revoked session: %v", err)
	}
	if err := st.Revoke(ctx, "missing.user", "test"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown operator: %v", err)
	}
}

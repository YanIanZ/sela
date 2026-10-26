package account_test

import (
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/vault"
)

func TestSessionTokenHashedAtRest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("token.at.rest")
	if len(res.Token) != 43 {
		t.Fatalf("token length %d", len(res.Token))
	}
	hash, _ := account.HashSessionToken(res.Token)
	var stored []byte
	if err := f.db.Pool.QueryRow(ctx, `SELECT token_hash FROM sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stored, hash) {
		t.Fatal("stored value is not SHA-256(token)")
	}
	dump := dbtest.DumpText(t, f.db.Pool)
	if strings.Contains(dump, res.Token) {
		t.Fatal("raw token stored")
	}
	// Session lookup works by hash only; a token of another shape is refused.
	if p := f.principal(res.Token); p.UserID != mustID(t, res.Profile.ID) {
		t.Fatal("principal mismatch")
	}
	if _, err := f.store.Authenticate(ctx, hex.EncodeToString(hash)); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("hash used as a token: %v", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// Account, untrusted: idle 2h, absolute 12h.
	res := f.register("sesi.biasa")
	if res.CookieMaxAge != 0 {
		t.Fatalf("untrusted session cookie has Max-Age %s", res.CookieMaxAge)
	}
	tok := res.Token
	var seen0 time.Time
	_ = f.db.Pool.QueryRow(ctx, `SELECT last_seen_at FROM sessions`).Scan(&seen0)
	f.clock.Advance(30 * time.Second)
	f.principal(tok)
	var seen time.Time
	_ = f.db.Pool.QueryRow(ctx, `SELECT last_seen_at FROM sessions`).Scan(&seen)
	if !seen.Equal(seen0) {
		t.Fatal("last_seen_at written within a minute")
	}
	f.clock.Advance(31 * time.Second)
	f.principal(tok)
	_ = f.db.Pool.QueryRow(ctx, `SELECT last_seen_at FROM sessions`).Scan(&seen)
	if !seen.After(seen0) {
		t.Fatal("last_seen_at not written after a minute")
	}
	var active time.Time
	_ = f.db.Pool.QueryRow(ctx, `SELECT last_active_at FROM users`).Scan(&active)
	if !active.Equal(seen) {
		t.Fatalf("users.last_active_at %s, last_seen_at %s", active, seen)
	}
	// Activity every 1h59m keeps it alive until the absolute limit.
	for i := 0; i < 6; i++ {
		f.clock.Advance(time.Hour + 59*time.Minute)
		f.principal(tok)
	}
	f.clock.Advance(6 * time.Minute) // 12h01m after creation, 6 minutes idle
	if _, err := f.store.Authenticate(ctx, tok); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("absolute expiry not enforced: %v", err)
	}
	// Idle expiry: 2h without activity.
	res = f.register("sesi.diam")
	f.clock.Advance(2*time.Hour - time.Second)
	f.principal(res.Token)
	f.clock.Advance(2*time.Hour + time.Second)
	if _, err := f.store.Authenticate(ctx, res.Token); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("idle expiry not enforced: %v", err)
	}
	// Trusted device: 7 days idle, 30 days absolute, Max-Age to absolute.
	trusted, err := f.store.Login(ctx, account.LoginInput{Username: "sesi.diam", Password: markerPassword, RememberDevice: true})
	if err != nil {
		t.Fatal(err)
	}
	if trusted.CookieMaxAge != 30*24*time.Hour || !trusted.Session.TrustedDevice {
		t.Fatalf("trusted cookie age %s", trusted.CookieMaxAge)
	}
	f.clock.Advance(7*24*time.Hour - time.Minute)
	f.principal(trusted.Token)
	for i := 0; i < 3; i++ {
		f.clock.Advance(7*24*time.Hour - time.Minute)
		f.principal(trusted.Token)
	}
	f.clock.Advance(3 * 24 * time.Hour) // past 30 days
	if _, err := f.store.Authenticate(ctx, trusted.Token); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("trusted absolute expiry: %v", err)
	}
	// Guest: idle = inactive TTL (7d), absolute = maximum age (30d).
	g := f.guest(account.AgeBand18Plus)
	if g.CookieMaxAge != 30*24*time.Hour {
		t.Fatalf("guest cookie age %s", g.CookieMaxAge)
	}
	f.clock.Advance(7*24*time.Hour + time.Second)
	if _, err := f.store.Authenticate(ctx, g.Token); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("guest idle expiry: %v", err)
	}
	g = f.guest(account.AgeBand18Plus)
	for i := 0; i < 4; i++ {
		f.clock.Advance(7*24*time.Hour - time.Hour)
		f.principal(g.Token)
	}
	f.clock.Advance(3 * 24 * time.Hour)
	if _, err := f.store.Authenticate(ctx, g.Token); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("guest maximum age: %v", err)
	}
	// Revoked sessions are refused at once.
	r := f.register("sesi.dicabut")
	p := f.principal(r.Token)
	if err := f.store.Logout(ctx, p.UserID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Authenticate(ctx, r.Token); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("revoked: %v", err)
	}
}

func TestLoginBackoff(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("coba.lagi")
	wrong := account.LoginInput{Username: "coba.lagi", Password: "salah salah salah salah"}
	unknown := account.LoginInput{Username: "tidak.ada", Password: "salah salah salah salah"}
	for i := 1; i <= 5; i++ {
		for _, in := range []account.LoginInput{wrong, unknown} {
			if _, err := f.store.Login(ctx, in); !errors.Is(err, account.ErrInvalidCredentials) {
				t.Fatalf("attempt %d (%s): %v", i, in.Username, err)
			}
		}
	}
	// After 5 failures the next attempt must wait 2^0 s, for both the
	// known and the unknown username, and the response is the same.
	for _, in := range []account.LoginInput{wrong, unknown, {Username: "coba.lagi", Password: markerPassword}} {
		_, err := f.store.Login(ctx, in)
		var e *account.Error
		if !errors.As(err, &e) || e.Code != "rate_limited" || e.RetryAfter != time.Second {
			t.Fatalf("%s: want rate_limited 1s, got %v", in.Username, err)
		}
	}
	f.clock.Advance(time.Second)
	if _, err := f.store.Login(ctx, wrong); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatal(err)
	}
	var e *account.Error
	if _, err := f.store.Login(ctx, wrong); !errors.As(err, &e) || e.RetryAfter != 2*time.Second {
		t.Fatalf("6 failures: %v", err)
	}
	// Persisted per username key, for the account and for the unknown name
	// alike; the account's own counter is kept for the 100-failure disable.
	key, ghost := f.store.UsernameKey("coba.lagi"), f.store.UsernameKey("tidak.ada")
	var failures int
	var last time.Time
	if err := f.db.Pool.QueryRow(ctx, `SELECT failures, last_failed_at FROM login_throttle WHERE key_hash = $1`, key[:]).Scan(&failures, &last); err != nil ||
		failures != 6 || !last.Equal(f.clock.Now()) {
		t.Fatalf("persisted throttle: %d %v %v", failures, last, err)
	}
	if n := f.count(`SELECT failures FROM login_throttle WHERE key_hash = $1`, ghost[:]); n != 5 {
		t.Fatalf("unknown username throttle: %d failures", n)
	}
	if n := f.count(`SELECT failed_count FROM credentials`); n != 6 {
		t.Fatalf("failed_count %d", n)
	}
	// Persisted counters survive a new server instance.
	s2 := f.restart()
	if _, err := s2.Login(ctx, account.LoginInput{Username: "coba.lagi", Password: markerPassword}); !errors.As(err, &e) || e.Code != "rate_limited" || e.RetryAfter != 2*time.Second {
		t.Fatalf("restart forgot the backoff: %v", err)
	}
	f.clock.Advance(2 * time.Second)
	if _, err := s2.Login(ctx, account.LoginInput{Username: "coba.lagi", Password: markerPassword}); err != nil {
		t.Fatalf("login after the wait: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM login_throttle WHERE key_hash = $1`, key[:]); n != 0 || f.count(`SELECT failed_count FROM credentials`) != 0 {
		t.Fatal("success did not reset the counters")
	}
	// Failed logins are coalesced: one auth.login_failed per account per hour.
	if n := f.count(`SELECT count(*) FROM audit_events WHERE type = 'auth.login_failed'`); n != 1 {
		t.Fatalf("%d auth.login_failed events within an hour", n)
	}
	// 100 consecutive failures disable password sign-in until recovery.
	if _, err := f.db.Pool.Exec(ctx, `UPDATE credentials SET failed_count = 99`); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(2 * time.Hour)
	s3 := f.restart()
	if _, err := s3.Login(ctx, wrong); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatal(err)
	}
	f.clock.Advance(16 * time.Minute)
	if _, err := s3.Login(ctx, account.LoginInput{Username: "coba.lagi", Password: markerPassword}); !errors.Is(err, account.ErrPasswordLoginDisabled) {
		t.Fatalf("after 100 failures: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM audit_events WHERE type = 'password_login.disabled'`); n != 1 {
		t.Fatalf("password_login.disabled events: %d", n)
	}
	if n := f.count(`SELECT count(*) FROM audit_events WHERE type = 'auth.login_failed'`); n != 2 {
		t.Fatalf("auth.login_failed after an hour: %d", n)
	}
}

func TestRecoveryCodesSingleUse(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("pulih.kode")
	if len(res.RecoveryCodes) != 10 {
		t.Fatalf("%d codes", len(res.RecoveryCodes))
	}
	user := mustID(t, res.Profile.ID)
	// Stored as user-bound SHA-256 of the normalised code.
	for _, c := range res.RecoveryCodes {
		n, _ := account.NormalizeRecoveryCode(c)
		if f.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND code_hash = $2`, user, account.RecoveryCodeHash(user, n)) != 1 {
			t.Fatal("code hash not stored")
		}
	}
	other := f.register("pulih.lain")
	// Two concurrent uses of one code: exactly one succeeds.
	code := strings.ToLower(strings.ReplaceAll(res.RecoveryCodes[0], "-", " "))
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.store.Recover(ctx, account.RecoveryInput{Username: "pulih.kode", RecoveryCode: code, NewPassword: newPassword + string(rune('a'+i))})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	ok, failed := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, account.ErrRecoveryFailed):
			failed++
		default:
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 || failed != 1 {
		t.Fatalf("concurrent use: %d succeeded, %d failed", ok, failed)
	}
	// Reuse, another user's code, a wrong code and an unknown user all fail alike.
	for name, in := range map[string]account.RecoveryInput{
		"reused":       {Username: "pulih.kode", RecoveryCode: res.RecoveryCodes[0], NewPassword: newPassword},
		"other user's": {Username: "pulih.kode", RecoveryCode: other.RecoveryCodes[0], NewPassword: newPassword},
		"wrong":        {Username: "pulih.kode", RecoveryCode: "0000-0000-0000-0000", NewPassword: newPassword},
		"malformed":    {Username: "pulih.kode", RecoveryCode: "xyz", NewPassword: newPassword},
		"unknown user": {Username: "tidak.ada", RecoveryCode: res.RecoveryCodes[1], NewPassword: newPassword},
	} {
		if _, err := f.store.Recover(ctx, in); !errors.Is(err, account.ErrRecoveryFailed) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A weak new password is refused before any code is consumed.
	if _, err := f.store.Recover(ctx, account.RecoveryInput{Username: "pulih.kode", RecoveryCode: res.RecoveryCodes[1], NewPassword: "pendek"}); !errors.Is(err, account.ErrPasswordTooShort) {
		t.Fatalf("short password: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND used_at IS NULL`, user); n != 9 {
		t.Fatalf("%d unused codes, want 9", n)
	}
}

func TestCryptoShred(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	g := f.guest(account.AgeBand15to17)
	p := f.principal(g.Token)
	name := markerGuardian
	rec, err := f.store.SetGuardianConsent(ctx, p.UserID, p.SessionID, account.GuardianInput{Relationship: "parent",
		Method: "guardian_present_attestation", GuardianName: &name, PolicyVersion: guardianV})
	if err != nil {
		t.Fatal(err)
	}
	// Simulated content backup: copy the ciphertext before deletion.
	var ct []byte
	if err := f.db.Pool.QueryRow(ctx, `SELECT guardian_name_ct FROM guardian_consents WHERE user_id = $1`, p.UserID).Scan(&ct); err != nil {
		t.Fatal(err)
	}
	if !f.vault.Cached(p.UserID) {
		t.Fatal("DEK not cached")
	}
	if _, err := f.store.DeleteAccount(ctx, p.UserID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	if f.vault.Cached(p.UserID) {
		t.Fatal("DEK cache entry survived deletion")
	}
	var wrappedNull, kekNull, destroyed bool
	if err := f.db.Pool.QueryRow(ctx, `SELECT wrapped_dek IS NULL, kek_id IS NULL, destroyed_at IS NOT NULL FROM user_data_keys WHERE user_id = $1`, p.UserID).Scan(&wrappedNull, &kekNull, &destroyed); err != nil {
		t.Fatal(err)
	}
	if !wrappedNull || !kekNull || !destroyed {
		t.Fatal("key row is not a tombstone")
	}
	ref := vault.Ref{Kind: "guardian_consent", RecordID: rec.ID, Field: "guardian_name"}
	// A restarted instance (empty cache) with the live key table cannot open the copy.
	s2 := f.restart()
	if _, err := s2.Vault().Open(ctx, f.db.Pool, p.UserID, ref, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("backup copy opened after deletion: %v", err)
	}
	if _, err := f.vault.Open(ctx, f.db.Pool, p.UserID, ref, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("backup copy opened by the deleting instance: %v", err)
	}
}

func TestRestoreReconcile(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	keep := f.register("tetap.ada")
	gone := f.register("sudah.hapus")
	lost := f.register("kunci.hilang")
	goneID, lostID := mustID(t, gone.Profile.ID), mustID(t, lost.Profile.ID)
	// "Content backup" taken before the deletion: copy the rows.
	conn, err := f.db.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE bk_users AS SELECT * FROM users WHERE id = $1`, goneID); err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []string{"credentials", "consents", "audit_events"} {
		if _, err := conn.Exec(ctx, "CREATE TEMP TABLE bk_"+tbl+" AS SELECT * FROM "+tbl+" WHERE user_id = $1", goneID); err != nil {
			t.Fatal(err)
		}
	}
	gp := f.principal(gone.Token)
	if _, err := f.store.DeleteAccount(ctx, gp.UserID, gp.SessionID); err != nil {
		t.Fatal(err)
	}
	// "Restore": the content rows come back, the current key table stays.
	for _, tbl := range []string{"users", "credentials", "consents", "audit_events"} {
		if _, err := conn.Exec(ctx, "INSERT INTO "+tbl+" SELECT * FROM bk_"+tbl); err != nil {
			t.Fatal(err)
		}
	}
	// And one user whose key row is missing entirely (key table lost).
	if _, err := f.db.Pool.Exec(ctx, `DELETE FROM user_data_keys WHERE user_id = $1`, lostID); err != nil {
		t.Fatal(err)
	}
	rep, err := f.store.ReconcileRestore(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DestroyedKey != 1 || rep.MissingKey != 1 || rep.Deleted != 0 {
		t.Fatalf("dry run: %+v", rep)
	}
	if n := f.count(`SELECT count(*) FROM users`); n != 3 {
		t.Fatalf("dry run deleted users: %d left", n)
	}
	rep, err = f.store.ReconcileRestore(ctx, false)
	if err != nil || rep.Deleted != 2 {
		t.Fatalf("reconcile: %+v %v", rep, err)
	}
	for _, id := range []account.UserID{goneID, lostID} {
		for tbl, n := range dbtest.CountOwned(t, f.db.Pool, id) {
			if n != 0 && tbl != "user_data_keys" {
				t.Errorf("%s: %d rows left", tbl, n)
			}
		}
		if f.count(`SELECT count(*) FROM user_data_keys WHERE user_id = $1 AND destroyed_at IS NOT NULL`, id) != 1 {
			t.Error("no tombstone after reconcile")
		}
	}
	if _, err := f.store.Profile(ctx, mustID(t, keep.Profile.ID)); err != nil {
		t.Fatalf("unaffected user: %v", err)
	}
	if rep, _ := f.store.ReconcileRestore(ctx, true); rep.DestroyedKey+rep.MissingKey != 0 {
		t.Fatalf("second dry run: %+v", rep)
	}
}

func TestNoSecretsOrPlaintextAtRest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("rahasia.aman")
	p := f.principal(res.Token)
	g := f.guest(account.AgeBand15to17)
	gp := f.principal(g.Token)
	name := markerGuardian
	if _, err := f.store.SetGuardianConsent(ctx, gp.UserID, gp.SessionID, account.GuardianInput{Relationship: "legal_guardian",
		Method: "guardian_present_attestation", GuardianName: &name, PolicyVersion: guardianV}); err != nil {
		t.Fatal(err)
	}
	codes, _, err := f.store.RegenerateRecoveryCodes(ctx, p.UserID, p.SessionID, markerPassword)
	if err != nil {
		t.Fatal(err)
	}
	cp, err := f.store.ChangePassword(ctx, p.UserID, p.SessionID, markerPassword, newPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	dump := dbtest.DumpText(t, f.db.Pool)
	if len(dump) < 1000 {
		t.Fatalf("dump too small to be meaningful: %d bytes", len(dump))
	}
	hexOf := func(s string) string { return hex.EncodeToString([]byte(s)) }
	secrets := map[string]string{"guardian name": markerGuardian, "password": markerPassword, "new password": newPassword,
		"token": res.Token, "guest token": g.Token, "rotated token": cp.Token}
	for i, c := range append(res.RecoveryCodes, codes...) {
		n, _ := account.NormalizeRecoveryCode(c)
		secrets["recovery code "+string(rune('a'+i))] = c
		secrets["normalised code "+string(rune('a'+i))] = n
	}
	for what, v := range secrets {
		for _, form := range []string{v, strings.ToLower(v), hexOf(v)} {
			if strings.Contains(dump, form) {
				t.Errorf("%s found at rest", what)
			}
		}
	}
	// The key itself is only ever stored wrapped.
	if strings.Contains(dump, hex.EncodeToString(f.kek.Secret)) {
		t.Error("KEK at rest")
	}
}

func TestGuestPurge(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	idle := f.guest(account.AgeBand18Plus)
	active := f.guest(account.AgeBand18Plus)
	old := f.guest(account.AgeBand18Plus)
	acct := f.register("akun.tetap")
	idleID, activeID, oldID := mustID(t, idle.Profile.ID), mustID(t, active.Profile.ID), mustID(t, old.Profile.ID)
	// The profile states both purge dates from configuration.
	if idle.Profile.Guest == nil || idle.Profile.Guest.InactiveTTLHours != 168 || idle.Profile.Guest.MaxAgeDays != 30 ||
		time.Time(idle.Profile.Guest.MaxAgePurgeAt).Sub(time.Time(idle.Profile.CreatedAt)) != 30*24*time.Hour {
		t.Fatalf("guest info: %+v", idle.Profile.Guest)
	}
	// "active" and "old" stay active for 24 days; then 6 days pass. "idle"
	// was never active (inactive TTL), "old" passes its maximum age, and
	// "active" is touched again with a later maximum age.
	for i := 0; i < 4; i++ {
		f.clock.Advance(6 * 24 * time.Hour)
		f.principal(active.Token)
		f.principal(old.Token)
	}
	f.clock.Advance(6*24*time.Hour + time.Minute)
	if _, err := f.db.Pool.Exec(ctx, `UPDATE users SET last_active_at = $2, guest_expires_at = $3 WHERE id = $1`,
		activeID, f.clock.Now(), f.clock.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Another instance holds the job lock: nothing happens.
	release, err := db.HoldJobLock(ctx, f.db.Pool, account.JobGuestPurge)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := f.store.PurgeGuests(ctx); !errors.Is(err, db.ErrJobLocked) || n != 0 {
		t.Fatalf("purge ran under another instance's lock: %d %v", n, err)
	}
	release()
	n, err := f.store.PurgeGuests(ctx)
	if err != nil || n != 2 {
		t.Fatalf("purged %d (%v), want 2", n, err)
	}
	for _, id := range []account.UserID{idleID, oldID} {
		for tbl, c := range dbtest.CountOwned(t, f.db.Pool, id) {
			if c != 0 && tbl != "user_data_keys" {
				t.Errorf("%s: %d rows left", tbl, c)
			}
		}
		if f.count(`SELECT count(*) FROM user_data_keys WHERE user_id = $1 AND destroyed_at IS NOT NULL AND wrapped_dek IS NULL`, id) != 1 {
			t.Error("purge did not crypto-shred")
		}
	}
	if _, err := f.store.Profile(ctx, activeID); err != nil {
		t.Fatalf("active guest purged: %v", err)
	}
	if _, err := f.store.Profile(ctx, mustID(t, acct.Profile.ID)); err != nil {
		t.Fatalf("account purged: %v", err)
	}
	// Two instances at once never delete twice.
	f.clock.Advance(8 * 24 * time.Hour)
	s2 := f.restart()
	var wg sync.WaitGroup
	counts := make(chan int, 2)
	for _, s := range []*account.Store{f.store, s2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := s.PurgeGuests(ctx)
			if err != nil && !errors.Is(err, db.ErrJobLocked) {
				t.Error(err)
			}
			counts <- n
		}()
	}
	wg.Wait()
	close(counts)
	total := 0
	for n := range counts {
		total += n
	}
	if total != 1 {
		t.Fatalf("concurrent purge deleted %d, want 1", total)
	}
}

// TestStoreMethodsTakeOwner: every owner-facing method takes the context
// and then the owner; every other exported method is an allow-listed
// system or accessor method.
func TestStoreMethodsTakeOwner(t *testing.T) {
	t.Parallel()
	owner := reflect.TypeOf((*account.OwnerStore)(nil)).Elem()
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	userType := reflect.TypeOf(account.UserID{})
	for i := 0; i < owner.NumMethod(); i++ {
		m := owner.Method(i)
		if m.Type.NumIn() < 2 || m.Type.In(0) != ctxType || m.Type.In(1) != userType {
			t.Errorf("OwnerStore.%s does not take (context.Context, account.UserID, ...)", m.Name)
		}
	}
	system := reflect.TypeOf((*account.SystemStore)(nil)).Elem()
	accessors := map[string]bool{"Config": true, "Hasher": true, "Vault": true, "RegisterExporter": true, "Exporters": true, "UsernameKey": true,
		"DeadlockRetries": true}
	st := reflect.TypeOf(&account.Store{})
	for i := 0; i < st.NumMethod(); i++ {
		name := st.Method(i).Name
		_, inOwner := owner.MethodByName(name)
		_, inSystem := system.MethodByName(name)
		if !inOwner && !inSystem && !accessors[name] {
			t.Errorf("Store.%s is neither owner-scoped nor allow-listed", name)
		}
	}
}

func TestExportRegistryCoversOwnedTables(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	claimed := map[string]string{}
	for _, e := range f.store.Exporters() {
		for _, tbl := range e.Tables() {
			claimed[tbl] = e.Section()
		}
	}
	for _, tbl := range dbtest.OwnedTables(t, f.db.Pool) {
		if _, ok := claimed[tbl]; !ok && !slices.Contains(account.SecurityOnlyTables, tbl) {
			t.Errorf("table %s has a user_id column but no exporter and is not security-only", tbl)
		}
	}
	if err := f.store.RegisterExporter(f.store.Exporters()[0]); err == nil {
		t.Fatal("duplicate section accepted")
	}
}

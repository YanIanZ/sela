package account_test

// Regression tests for the PR #9 review findings: F1 (a password proof
// verified outside any lock committed after a password change or a
// recovery had replaced the password) and F2 (background jobs waited
// forever for a second connection at the smallest pool size).

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/vault"
)

// otherPassword is a synthetic password someone else sets.
const otherPassword = "gunung hijau senja tanpa awan"

// held starts op with the next verification of a stored hash held after
// its argon2id comparison (Hasher.HoldNextVerify) and returns once op is
// held there, between its verification and its success transaction.
// resume lets op finish and returns its error. The timeouts only bound a
// broken run; the order of events is fixed by the hold.
func held(t *testing.T, h *account.Hasher, op func() error) (resume func() error) {
	t.Helper()
	reached, release := h.HoldNextVerify()
	t.Cleanup(release)
	done := make(chan error, 1)
	go func() { done <- op() }()
	select {
	case <-reached:
	case err := <-done:
		t.Fatalf("the operation ended before its verification was held: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("the operation never reached its verification")
	}
	return func() error {
		t.Helper()
		release()
		select {
		case err := <-done:
			return err
		case <-time.After(30 * time.Second):
			t.Fatal("the held operation did not finish")
			return nil
		}
	}
}

// storedHash reads an account's stored password hash.
func storedHash(t *testing.T, f *fixture, user account.UserID) account.PasswordHash {
	t.Helper()
	var ph account.PasswordHash
	var mem, iter int32
	var par int16
	if err := f.db.Pool.QueryRow(ctx, `SELECT password_hash, password_salt, argon2_memory_kib, argon2_iterations, argon2_parallelism
		FROM credentials WHERE user_id = $1`, user).Scan(&ph.Tag, &ph.Salt, &mem, &iter, &par); err != nil {
		t.Fatal(err)
	}
	ph.Params = account.HashParams{MemoryKiB: uint32(mem), Iterations: uint32(iter), Parallelism: uint8(par)}
	return ph
}

// hashOf reports whether the stored hash is that of password.
func hashOf(t *testing.T, ph account.PasswordHash, password string) bool {
	t.Helper()
	h, err := account.NewHasher(account.FloorHashParams, 1, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := h.Verify(ctx, password, &ph)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// signIn reports the outcome of a sign-in with password.
func signIn(f *fixture, username, password string) error {
	_, err := f.store.Login(ctx, account.LoginInput{Username: username, Password: password})
	return err
}

// F1: a sign-in whose password was verified while the owner changed it or
// recovered the account is refused and commits nothing: no session (so
// none survives the change's revocation of every session), no re-hash (so
// the new hash is not overwritten with one of the old password), no
// counter reset; it ends as a failed attempt (auth.login_failed). The new
// password signs in, the old one does not.
func TestPR9StaleLoginProofCommitsNothing(t *testing.T) {
	t.Parallel()
	stronger := account.HashParams{MemoryKiB: 20480, Iterations: 3, Parallelism: 1}
	for _, tc := range []struct {
		name             string
		recovery, rehash bool
	}{
		{"password change", false, false},
		{"password change, re-hash due", false, true},
		{"recovery", true, false},
		{"recovery, re-hash due", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			const username = "lama.basi"
			res := f.register(username)
			user := mustID(t, res.Profile.ID)
			p := f.principal(res.Token)
			login := f.store
			if tc.rehash {
				// The stored hash's parameters differ from this instance's,
				// so its successful sign-in re-hashes the password.
				login = f.restart(func(c *account.Config) { c.Hash = stronger })
			}
			hashes := login.Hasher().Hashes()
			var stale account.AuthResult
			resume := held(t, login.Hasher(), func() error {
				var err error
				stale, err = login.Login(ctx, account.LoginInput{Username: username, Password: markerPassword})
				return err
			})
			// The owner replaces the password while the old one is proven.
			var err error
			if tc.recovery {
				_, err = f.store.Recover(ctx, account.RecoveryInput{Username: username, RecoveryCode: res.RecoveryCodes[0], NewPassword: newPassword})
			} else {
				_, err = f.store.ChangePassword(ctx, p.UserID, p.SessionID, markerPassword, newPassword, "")
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := resume(); !errors.Is(err, account.ErrInvalidCredentials) || stale.Token != "" {
				t.Fatalf("the old password after the change: %v (token issued: %t), want invalid_credentials", err, stale.Token != "")
			}
			if tc.rehash && login.Hasher().Hashes() != hashes+1 {
				t.Fatalf("%d hashes computed, want the one re-hash", login.Hasher().Hashes()-hashes)
			}
			ph := storedHash(t, f, user)
			if !hashOf(t, ph, newPassword) || hashOf(t, ph, markerPassword) {
				t.Fatal("the stored hash is not the new password's")
			}
			// Register's session and the change's (rotated) or the recovery's
			// new one: the refused sign-in left none.
			if n := f.count(`SELECT count(*) FROM sessions WHERE user_id = $1`, user); n != 2 {
				t.Fatalf("%d sessions, want 2", n)
			}
			if n := f.count(`SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, user); n != 1 {
				t.Fatalf("%d live sessions, want 1", n)
			}
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'auth.login_succeeded'`, user); n != 0 {
				t.Fatalf("%d auth.login_succeeded events", n)
			}
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'auth.login_failed'`, user); n != 1 {
				t.Fatalf("%d auth.login_failed events, want 1: the refused proof is a failed attempt", n)
			}
			if err := signIn(f, username, newPassword); err != nil {
				t.Fatalf("the new password: %v", err)
			}
			if err := signIn(f, username, markerPassword); !errors.Is(err, account.ErrInvalidCredentials) {
				t.Fatalf("the old password: %v", err)
			}
		})
	}
}

// F1 sweep: a code regeneration whose password was verified while the
// owner recovered the account or changed the password from another
// session is refused (its transaction does not check that the session is
// live): no new batch, the owner's codes unchanged, the attempt counted as
// failed. Before, the old password's holder got ten fresh codes, enough to
// recover the account again.
func TestPR9StaleRegenerationProofCommitsNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		recovery bool
	}{{"recovery", true}, {"password change from another session", false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			const username = "kode.basi"
			res := f.register(username)
			user := mustID(t, res.Profile.ID)
			p := f.principal(res.Token)
			var owner account.Principal
			if !tc.recovery {
				in, err := f.store.Login(ctx, account.LoginInput{Username: username, Password: markerPassword})
				if err != nil {
					t.Fatal(err)
				}
				owner = f.principal(in.Token)
			}
			var codes []string
			resume := held(t, f.store.Hasher(), func() error {
				var err error
				codes, _, err = f.store.RegenerateRecoveryCodes(ctx, p.UserID, p.SessionID, markerPassword)
				return err
			})
			var err error
			if tc.recovery {
				_, err = f.store.Recover(ctx, account.RecoveryInput{Username: username, RecoveryCode: res.RecoveryCodes[0], NewPassword: newPassword})
			} else {
				_, err = f.store.ChangePassword(ctx, owner.UserID, owner.SessionID, markerPassword, newPassword, "")
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := resume(); !errors.Is(err, account.ErrCurrentPasswordIncorrect) || codes != nil {
				t.Fatalf("regeneration with the old password: %v (%d codes), want current_password_incorrect", err, len(codes))
			}
			if n := f.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1`, user); n != account.RecoveryCodeCount {
				t.Fatalf("%d code rows, want the original %d", n, account.RecoveryCodeCount)
			}
			for i, c := range res.RecoveryCodes {
				norm, _ := account.NormalizeRecoveryCode(c)
				want := 1
				if tc.recovery && i == 0 {
					want = 0 // the code the recovery used
				}
				if n := f.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`,
					user, account.RecoveryCodeHash(user, norm)); n != want {
					t.Fatalf("original code %d: %d unused rows, want %d", i, n, want)
				}
			}
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'recovery.regenerated'`, user); n != 0 {
				t.Fatalf("%d recovery.regenerated events", n)
			}
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'auth.login_failed'`, user); n != 1 {
				t.Fatalf("%d auth.login_failed events, want 1: the refused proof is a failed attempt", n)
			}
			if err := signIn(f, username, newPassword); err != nil {
				t.Fatalf("the new password: %v", err)
			}
		})
	}
}

// F1, the accepted residual (ADR-0002 §9): the stale check compares the
// stored hash, not the password, so a proof of the right, current
// password that overlaps another sign-in which re-hashes the stored hash
// is refused once, whether or not the refused proof would re-hash itself.
// Here the other sign-in runs on an instance configured with other
// parameters (a rolling deploy after a parameter change). A sign-in gets
// invalid_credentials and a code regeneration current_password_incorrect;
// neither commits anything, each writes one auth.login_failed, and neither
// raises a counter (the other sign-in reset them after this proof's
// reservation); the other sign-in's hash stays, and the same proof
// succeeds when sent again.
func TestPR9RehashOverlapRefusesCurrentProofOnce(t *testing.T) {
	t.Parallel()
	stronger := account.HashParams{MemoryKiB: 20480, Iterations: 3, Parallelism: 1}
	for _, tc := range []struct {
		name  string
		regen bool
	}{{"sign-in", false}, {"code regeneration", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			const username = "hash.ulang"
			res := f.register(username)
			user := mustID(t, res.Profile.ID)
			p := f.principal(res.Token)
			other := f.restart(func(c *account.Config) { c.Hash = stronger })
			// The stored hash has the held instance's parameters: the held
			// proof would not re-hash.
			if storedHash(t, f, user).Params != f.store.Hasher().Params() {
				t.Fatal("the stored hash does not have the held instance's parameters")
			}
			proof := func() error {
				if tc.regen {
					codes, _, err := f.store.RegenerateRecoveryCodes(ctx, p.UserID, p.SessionID, markerPassword)
					if err == nil && len(codes) != account.RecoveryCodeCount {
						t.Errorf("%d codes", len(codes))
					}
					return err
				}
				_, err := f.store.Login(ctx, account.LoginInput{Username: username, Password: markerPassword})
				return err
			}
			resume := held(t, f.store.Hasher(), proof)
			if _, err := other.Login(ctx, account.LoginInput{Username: username, Password: markerPassword}); err != nil {
				t.Fatalf("the re-hashing sign-in: %v", err)
			}
			rehashed := storedHash(t, f, user)
			if rehashed.Params != stronger || !hashOf(t, rehashed, markerPassword) {
				t.Fatal("the other sign-in did not re-hash the password")
			}
			want := account.ErrInvalidCredentials
			if tc.regen {
				want = account.ErrCurrentPasswordIncorrect
			}
			if err := resume(); !errors.Is(err, want) {
				t.Fatalf("the right password overlapping a re-hash: %v, want %v (refused once)", err, want)
			}
			// Nothing committed: the other sign-in's hash stays, no session
			// beyond Register's and the other sign-in's, the codes unchanged.
			if ph := storedHash(t, f, user); ph.Params != stronger || !bytes.Equal(ph.Tag, rehashed.Tag) {
				t.Fatal("the refused proof replaced the stored hash")
			}
			if n := f.count(`SELECT count(*) FROM sessions WHERE user_id = $1`, user); n != 2 {
				t.Fatalf("%d sessions, want 2", n)
			}
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type IN ('auth.login_succeeded', 'recovery.regenerated')`, user); n != 1 {
				t.Fatalf("%d success events, want the other sign-in's", n)
			}
			for i, c := range res.RecoveryCodes {
				norm, _ := account.NormalizeRecoveryCode(c)
				if n := f.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`,
					user, account.RecoveryCodeHash(user, norm)); n != 1 {
					t.Fatalf("original code %d: %d unused rows, want 1", i, n)
				}
			}
			// A failed attempt in the audit trail only: no counter raised.
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'auth.login_failed'`, user); n != 1 {
				t.Fatalf("%d auth.login_failed events, want 1", n)
			}
			if n := f.count(`SELECT failed_count FROM credentials WHERE user_id = $1`, user); n != 0 {
				t.Fatalf("failed_count %d, want 0", n)
			}
			key := f.store.UsernameKey(username)
			if n := f.count(`SELECT count(*) FROM login_throttle WHERE key_hash = $1`, key[:]); n != 0 {
				t.Fatalf("%d throttle rows, want none", n)
			}
			// Refused once: the same proof, sent again, succeeds.
			if err := proof(); err != nil {
				t.Fatalf("the same proof again: %v", err)
			}
		})
	}
}

// F1 sweep, unchanged paths: reauthentication and a password change
// commit only while the current session is live, and a recovery (like a
// password change) revokes every session of the user in the transaction
// that replaces the hash, so a proof of the old password that ran during a
// recovery is refused (not_signed_in) and commits nothing. These pin that
// the session check keeps covering them.
func TestPR9StaleReauthAndChangeRefusedBySessionRevocation(t *testing.T) {
	t.Parallel()
	for name, op := range map[string]func(*fixture, account.Principal) error{
		"reauth": func(f *fixture, p account.Principal) error {
			_, err := f.store.Reauthenticate(ctx, p.UserID, p.SessionID, markerPassword)
			return err
		},
		"password change": func(f *fixture, p account.Principal) error {
			_, err := f.store.ChangePassword(ctx, p.UserID, p.SessionID, markerPassword, otherPassword, "")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			const username = "sesi.basi"
			res := f.register(username)
			user := mustID(t, res.Profile.ID)
			p := f.principal(res.Token)
			resume := held(t, f.store.Hasher(), func() error { return op(f, p) })
			if _, err := f.store.Recover(ctx, account.RecoveryInput{Username: username, RecoveryCode: res.RecoveryCodes[0], NewPassword: newPassword}); err != nil {
				t.Fatal(err)
			}
			if err := resume(); !errors.Is(err, account.ErrNotSignedIn) {
				t.Fatalf("%s with the old password after a recovery: %v, want not_signed_in", name, err)
			}
			if n := f.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type IN ('auth.reauthenticated', 'password.changed')`, user); n != 0 {
				t.Fatalf("%d reauthentication or password change events", n)
			}
			if n := f.count(`SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, user); n != 1 {
				t.Fatalf("%d live sessions, want the recovery's", n)
			}
			if !hashOf(t, storedHash(t, f, user), newPassword) {
				t.Fatal("the stored hash is not the recovered password's")
			}
			if err := signIn(f, username, otherPassword); !errors.Is(err, account.ErrInvalidCredentials) {
				t.Fatalf("the refused change's password: %v", err)
			}
		})
	}
}

// F1 sweep: a deletion checks its recent password proof under the
// credentials row lock, so a recovery in flight (it holds that lock and
// has revoked every session, not yet committed) is waited for and the
// deletion is then refused, instead of passing the check on the session
// still committed as live and deleting the account the recovery just
// secured.
func TestPR9DeletionWaitsForConcurrentRecovery(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("hapus.basi")
	user := mustID(t, res.Profile.ID)
	p := f.principal(res.Token)
	if _, err := f.store.Reauthenticate(ctx, p.UserID, p.SessionID, markerPassword); err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM credentials WHERE user_id = $1 FOR UPDATE`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $2, revoke_reason = 'recovery' WHERE user_id = $1 AND revoked_at IS NULL`,
		user, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.store.DeleteAccount(ctx, p.UserID, p.SessionID)
		done <- err
	}()
	// The deletion is blocked by the recovery's transaction (with or
	// without the fix it reaches the credentials row lock).
	deadline := time.Now().Add(30 * time.Second)
	for f.count(`SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))`, pid) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the deletion never waited for the recovery's lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("deletion by a session the recovery revoked: %v, want not_signed_in", err)
	}
	if n := f.count(`SELECT count(*) FROM users WHERE id = $1`, user); n != 1 {
		t.Fatal("the account was deleted")
	}
}

// F2: the guest purge and the retention job finish at the smallest pool
// size the configuration accepts. Each runs on the connection that holds
// its advisory lock; before, each queried through the pool while that
// connection held the lock, and with one connection it waited forever
// (here: until the deadline), blocking every request.
func TestPR9JobsRunOnSmallestPool(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("kolam.kecil")
	if err := signIn(f, "kolam.kecil", wrongPassword); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatal(err)
	}
	f.guest(account.AgeBand18Plus)
	f.clock.Advance(40 * 24 * time.Hour)
	pool := f.db.OpenPool(t, int32(config.MinDBMaxConns))
	v, err := vault.New(vault.Keyring{Current: f.kek}, vault.Options{Now: f.clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	s, err := account.New(pool, v, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	jctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if n, err := s.PurgeGuests(jctx); err != nil || n != 1 {
		t.Fatalf("purge at pool size %d: %d purged, %v", config.MinDBMaxConns, n, err)
	}
	rep, err := s.Retention(jctx)
	if err != nil || rep.Sessions != 1 || rep.Throttle != 1 {
		t.Fatalf("retention at pool size %d: %+v, %v", config.MinDBMaxConns, rep, err)
	}
	// The lock and the connection were given back: the job runs again and
	// a request is served.
	if n, err := s.PurgeGuests(jctx); err != nil || n != 0 {
		t.Fatalf("second purge: %d, %v", n, err)
	}
	if _, err := s.Login(jctx, account.LoginInput{Username: "kolam.kecil", Password: markerPassword}); err != nil {
		t.Fatalf("sign-in after the jobs: %v", err)
	}
}

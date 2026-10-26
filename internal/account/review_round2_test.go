package account_test

// Regression tests for the findings of review round 2 (auth R2-A to R2-D,
// data R2-2, R2-3 and R2-5).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/vault"
)

func medianOf(x []time.Duration) time.Duration {
	y := slices.Clone(x)
	slices.Sort(y)
	return y[len(y)/2]
}

// R2-A: unknown usernames are verified with the parameters the stored
// credentials use, not with the configured ones, so a failed login costs
// the same for an existing and an unknown username after the operator
// raises or lowers SELA_ARGON2_*. Accounts are registered at one set; the
// store restarts with a higher and then a lower set.
//
// The property is checked deterministically: every verification, known or
// unknown, ran with the stored parameters (Hasher.VerificationsAt). The
// latency ratio is a measurement: it is logged, and asserted only with
// timingAssertions (SELA_TIMING_TESTS=1, no -race), with more samples,
// because under -race and parallel packages single logins vary by several
// times and a fixed band failed about one run in ten (review round 3, R3-5).
func TestUnknownUsernameCostFollowsStoredParams(t *testing.T) {
	registered := account.HashParams{MemoryKiB: 40960, Iterations: 2, Parallelism: 1}
	higher := account.HashParams{MemoryKiB: 98304, Iterations: 2, Parallelism: 1}
	lower := account.FloorHashParams
	f := newFixture(t, func(c *account.Config) { c.Hash = registered })
	const n = 10
	for i := 0; i < n; i++ {
		f.register(fmt.Sprintf("ada.akun%02d", i))
	}
	samples := n
	if timingAssertions() {
		samples = 40
	}
	for round, p := range []account.HashParams{higher, lower} {
		s := f.restart(func(c *account.Config) { c.Hash = p })
		if err := s.RefreshHashDistribution(ctx); err != nil {
			t.Fatal(err)
		}
		if d := s.Hasher().Distribution(); len(d) != 1 || d[0].Params != registered || d[0].Count != n {
			t.Fatalf("distribution after the restart at %+v: %+v", p, d)
		}
		h := s.Hasher()
		var known, unknown []time.Duration
		for i := 0; i < samples; i++ {
			// Interleaved, so load from elsewhere affects both sides alike.
			// The clock moves on an hour between pairs, so the repeated
			// usernames never reach a backoff.
			f.clock.Advance(account.ThrottleQuiet + time.Second)
			for _, k := range []bool{i%2 == 0, i%2 != 0} {
				u := fmt.Sprintf("tak.ada%d.%02d", round, i)
				if k {
					u = fmt.Sprintf("ada.akun%02d", i%n)
				}
				start := time.Now()
				_, err := s.Login(ctx, account.LoginInput{Username: u, Password: "salah salah salah salah"})
				d := time.Since(start)
				if !errors.Is(err, account.ErrInvalidCredentials) {
					t.Fatalf("%s: %v", u, err)
				}
				if k {
					known = append(known, d)
				} else {
					unknown = append(unknown, d)
				}
			}
		}
		// Every verification, known or unknown, ran with the stored
		// parameters; none with the newly configured ones.
		if got := h.VerificationsAt(registered); got != uint64(2*samples) || h.VerificationsAt(p) != 0 {
			t.Fatalf("restart at %+v: %d verifications at the stored parameters, %d at the configured ones", p, got, h.VerificationsAt(p))
		}
		mk, mu := medianOf(known), medianOf(unknown)
		ratio := float64(mu) / float64(mk)
		t.Logf("restart at %d KiB: known median %v, unknown median %v (ratio %.2f; measurement, asserted only with SELA_TIMING_TESTS=1 without -race)",
			p.MemoryKiB, mk, mu, ratio)
		// Before the fix the unknown side cost what the configured set costs
		// (about 2.4x higher, 0.5x lower).
		if timingAssertions() && (ratio < 0.8 || ratio > 1.25) {
			t.Errorf("restart at %+v: unknown/known failure latency ratio %.2f (known %v, unknown %v)", p, ratio, mk, mu)
		}
	}
}

// R2-A: the distribution ignores tuples below the floor, and a dummy
// verification is counted at the parameters it ran with. (The proportional
// and stable mapping is TestDummyForIsProportionalAndStable.)
func TestUnknownUsernameDistributionShape(t *testing.T) {
	t.Parallel()
	a := account.FloorHashParams
	b := account.HashParams{MemoryKiB: 19456, Iterations: 3, Parallelism: 1}
	h, err := account.NewHasher(a, 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if d := h.Distribution(); len(d) != 1 || d[0].Params != a {
		t.Fatalf("default distribution: %+v", d)
	}
	h.SetDistribution([]account.ParamCount{{Params: b, Count: 2}, {Params: account.HashParams{MemoryKiB: 1024, Iterations: 1, Parallelism: 1}, Count: 50}, {Params: a, Count: 0}})
	if d := h.Distribution(); len(d) != 1 || d[0].Params != b || d[0].Count != 2 {
		t.Fatalf("distribution: %+v", d)
	}
	if ok, err := h.VerifyUnknown(ctx, "apa saja", [32]byte{1}); ok || err != nil {
		t.Fatalf("dummy verification: %t %v", ok, err)
	}
	if h.VerificationsAt(b) != 1 || h.VerificationsAt(a) != 0 || h.Verifications() != 1 {
		t.Fatalf("counted at b %d, at a %d", h.VerificationsAt(b), h.VerificationsAt(a))
	}
	h.SetDistribution(nil)
	if d := h.Distribution(); len(d) != 1 || d[0].Params != a {
		t.Fatalf("empty distribution: %+v", d)
	}
}

// R2-A: any parameter difference re-hashes on the next successful login,
// so lowered parameters converge too (before, only higher ones did).
func TestRehashOnLoginAfterLowering(t *testing.T) {
	t.Parallel()
	high := account.HashParams{MemoryKiB: 20480, Iterations: 3, Parallelism: 1}
	f := newFixture(t, func(c *account.Config) { c.Hash = high })
	f.register("turun.parameter")
	s := f.restart(func(c *account.Config) { c.Hash = account.FloorHashParams })
	if _, err := s.Login(ctx, account.LoginInput{Username: "turun.parameter", Password: markerPassword}); err != nil {
		t.Fatal(err)
	}
	var mem, iter int
	_ = f.db.Pool.QueryRow(ctx, `SELECT argon2_memory_kib, argon2_iterations FROM credentials`).Scan(&mem, &iter)
	if mem != 19456 || iter != 2 {
		t.Fatalf("not re-hashed after lowering: m=%d t=%d", mem, iter)
	}
}

// R2-B: the throttle key is an HMAC under a secret derived from the KEK,
// never a plain hash of what was typed; input that is not a valid username
// (a passphrase typed into the username field) is not persisted at all.
func TestThrottleKeyIsKeyedAndInvalidInputNotStored(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	typed := []string{"pelangi.ada", "Tiga kata panjang untuk diingat"}
	for _, u := range typed {
		if _, err := f.store.Login(ctx, account.LoginInput{Username: u, Password: "salah salah salah salah"}); !errors.Is(err, account.ErrInvalidCredentials) {
			t.Fatal(err)
		}
	}
	rows, err := f.db.Pool.Query(ctx, `SELECT key_hash FROM login_throttle`)
	if err != nil {
		t.Fatal(err)
	}
	var keys [][]byte
	for rows.Next() {
		var k []byte
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	if len(keys) != 1 {
		t.Fatalf("%d throttle rows, want 1 (the passphrase typed as a username must not be stored)", len(keys))
	}
	for _, guess := range []string{"u:pelangi.ada", "r:tiga kata panjang untuk diingat", "pelangi.ada"} {
		h := sha256.Sum256([]byte(guess))
		if string(keys[0]) == string(h[:]) {
			t.Fatalf("login_throttle holds a plain SHA-256 of %q", guess)
		}
	}
	secret, err := f.vault.DeriveKey(account.ThrottleKeyPurpose)
	if err != nil {
		t.Fatal(err)
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("u:pelangi.ada"))
	if want := m.Sum(nil); string(keys[0]) != string(want) {
		t.Fatal("the throttle key is not the HMAC under the derived secret")
	}
	// Another KEK (another deployment, or after rotation) keys differently.
	other := f.restart()
	v2, _ := vault.New(vault.Keyring{Current: vault.Key{ID: "test-other", Secret: make([]byte, 32)}}, vault.Options{})
	s2, err := account.New(f.db.Pool, v2, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if k1, k2 := other.UsernameKey("pelangi.ada"), s2.UsernameKey("pelangi.ada"); k1 == k2 {
		t.Fatal("two KEKs give the same username key")
	}
}

// R2-C: a recovery failure for an unknown username and for a known one
// with a wrong code sends the same statements.
func TestRecoveryFailureSameStatements(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("ada.akun")
	rec := &sqlRecorder{}
	s := f.tracedStore(rec)
	attempt := func(u, code string) []string {
		t.Helper()
		_, _ = rec.take()
		if _, err := s.Recover(ctx, account.RecoveryInput{Username: u, RecoveryCode: code, NewPassword: newPassword}); !errors.Is(err, account.ErrRecoveryFailed) {
			t.Fatalf("%s: %v", u, err)
		}
		sqls, errs := rec.take()
		if len(errs) != 0 {
			t.Fatalf("%s: statements failed: %v", u, errs)
		}
		return sqls
	}
	wrong := attempt("ada.akun", "AAAA-BBBB-CCCC-DDDD")
	unknown := attempt("tak.ada", "AAAA-BBBB-CCCC-DDDD")
	if !slices.Equal(wrong, unknown) {
		t.Fatalf("different statements:\nknown   %q\nunknown %q", wrong, unknown)
	}
	if len(wrong) != 4 {
		t.Fatalf("expected begin, credentials lock, code update, rollback: %q", wrong)
	}
	// A used code sends the same statements as well.
	if _, err := f.store.Recover(ctx, account.RecoveryInput{Username: "ada.akun", RecoveryCode: res.RecoveryCodes[0], NewPassword: newPassword}); err != nil {
		t.Fatal(err)
	}
	if used := attempt("ada.akun", res.RecoveryCodes[0]); !slices.Equal(used, unknown) {
		t.Fatalf("used code: %q", used)
	}
}

// R2-5: two overlapping regenerations leave exactly one valid batch.
func TestConcurrentRegenerationKeepsOneBatch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("dua.batch")
	p := f.principal(res.Token)
	for round := 0; round < 10; round++ {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := f.store.RegenerateRecoveryCodes(ctx, p.UserID, p.SessionID, markerPassword)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if n := f.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND used_at IS NULL`, p.UserID); n != account.RecoveryCodeCount {
			t.Fatalf("round %d: %d unused codes after two overlapping regenerations, want %d", round, n, account.RecoveryCodeCount)
		}
		if prof, _ := f.store.Profile(ctx, p.UserID); prof.Recovery.Remaining != account.RecoveryCodeCount {
			t.Fatalf("round %d: profile reports %d remaining", round, prof.Recovery.Remaining)
		}
	}
}

// R2-D: a well-formed cursor whose time PostgreSQL cannot represent, or
// lies in the future, or whose id is not a version-4 uuid, is 400
// invalid_input (cursor) and never reaches the database.
func TestCraftedCursorRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("kursor.uji")
	p := f.principal(res.Token)
	rec := &sqlRecorder{}
	s := f.tracedStore(rec)
	for name, cur := range map[string]string{
		"year -10000":       craftCursor(-377_000_000_000_000_000, true),
		"before 2000":       craftCursor(time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC).UnixMicro(), true),
		"two days ahead":    craftCursor(f.clock.Now().Add(48*time.Hour).UnixMicro(), true),
		"zero id":           craftCursor(f.clock.Now().UnixMicro(), false),
		"far future (2^62)": craftCursor(1<<62, true),
	} {
		_, _ = rec.take()
		_, _, err := s.AuditEvents(ctx, p.UserID, p.SessionID, account.Page{Cursor: cur})
		var e *account.Error
		if !errors.As(err, &e) || e.Code != "invalid_input" || e.Field != "cursor" {
			t.Errorf("audit %s: %v", name, err)
		}
		_, _, err = s.ConsentHistory(ctx, p.UserID, "", account.Page{Cursor: cur})
		if !errors.As(err, &e) || e.Code != "invalid_input" || e.Field != "cursor" {
			t.Errorf("consent history %s: %v", name, err)
		}
		if sqls, errs := rec.take(); len(sqls) != 0 || len(errs) != 0 {
			t.Errorf("%s: the cursor reached the database: %q %v", name, sqls, errs)
		}
	}
	// A cursor the store produced still pages.
	for i := 0; i < 3; i++ {
		f.clock.Advance(time.Second)
		if _, err := f.store.Login(ctx, account.LoginInput{Username: "kursor.uji", Password: markerPassword}); err != nil {
			t.Fatal(err)
		}
	}
	page1, next, err := f.store.AuditEvents(ctx, p.UserID, p.SessionID, account.Page{Limit: 2})
	if err != nil || len(page1) != 2 || next == "" {
		t.Fatalf("page 1: %d %q %v", len(page1), next, err)
	}
	if page2, _, err := f.store.AuditEvents(ctx, p.UserID, p.SessionID, account.Page{Limit: 2, Cursor: next}); err != nil || len(page2) == 0 {
		t.Fatalf("page 2: %d %v", len(page2), err)
	}
}

// craftCursor encodes a cursor as encodeCursor does: 8 big-endian bytes of
// Unix microseconds and a 16-byte id (version 4 when v4, else zero).
func craftCursor(micros int64, v4 bool) string {
	b := binary.BigEndian.AppendUint64(nil, uint64(micros))
	id := make([]byte, 16)
	if v4 {
		id[6], id[8] = 0x40, 0x80
	}
	return base64.RawURLEncoding.EncodeToString(append(b, id...))
}

// R2-2, R2-3: reconcile revokes every session restored from the content
// dump (one revoked after the dump comes back live) and destroys the live
// key of a user created after the dump, keeping the canary's key; the dry
// run reports both counts and changes nothing.
func TestRestoreReconcileEndsSessionsAndOrphanKeys(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("korban.cookie")
	stolen := res.Token
	// "Content dump": the live session row as it was.
	if _, err := f.db.Pool.Exec(ctx, `CREATE TABLE bk_sessions AS SELECT * FROM sessions`); err != nil {
		t.Fatal(err)
	}
	// After the dump: the session is revoked, and a new account is created.
	p := f.principal(stolen)
	if err := f.store.Logout(ctx, p.UserID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	later := f.register("setelah.dump")
	laterID := mustID(t, later.Profile.ID)
	ref := vault.Ref{Kind: "guardian_consent", RecordID: "r2", Field: "guardian_name"}
	ct, err := f.vault.Seal(ctx, f.db.Pool, laterID, ref, []byte(markerGuardian))
	if err != nil {
		t.Fatal(err)
	}
	canary := account.NewUserID()
	if err := f.vault.CreateUserKey(ctx, f.db.Pool, canary); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Pool.Exec(ctx, `INSERT INTO ops_canary (singleton, key_id, sealed, digest, created_at) VALUES (true, $1, '\x00', $2, now())`,
		canary, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	// "Restore": content rows as in the dump (the new account is absent,
	// the stolen session live), the newer key table as it is.
	if _, err := f.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, laterID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Pool.Exec(ctx, `DELETE FROM sessions; INSERT INTO sessions SELECT * FROM bk_sessions`); err != nil {
		t.Fatal(err)
	}
	r := f.restart()
	if _, err := r.Authenticate(ctx, stolen); err != nil {
		t.Fatalf("precondition: the restored session should be live before reconcile: %v", err)
	}
	rep, err := r.ReconcileRestore(ctx, true)
	if err != nil || rep.OrphanKeys != 1 || rep.LiveSessions != 1 || rep.KeysDestroyed+rep.SessionsRevoked+rep.Deleted != 0 {
		t.Fatalf("dry run: %+v %v", rep, err)
	}
	if _, err := r.Authenticate(ctx, stolen); err != nil {
		t.Fatalf("the dry run changed a session: %v", err)
	}
	rep, err = r.ReconcileRestore(ctx, false)
	if err != nil || rep.KeysDestroyed != 1 || rep.SessionsRevoked != 1 {
		t.Fatalf("reconcile: %+v %v", rep, err)
	}
	if _, err := r.Authenticate(ctx, stolen); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("the session revoked after the dump is live again after reconcile: %v", err)
	}
	if f.count(`SELECT count(*) FROM sessions WHERE revoke_reason = 'restored'`) != 1 {
		t.Fatal("the restored session is not marked 'restored'")
	}
	if _, err := r.Vault().Open(ctx, f.db.Pool, laterID, ref, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("the ciphertext of a user who no longer exists still opens: %v", err)
	}
	if f.count(`SELECT count(*) FROM user_data_keys WHERE user_id = $1 AND destroyed_at IS NULL`, canary) != 1 {
		t.Fatal("the canary's key was destroyed")
	}
	// The account restored from the dump signs in again normally.
	if _, err := r.Login(ctx, account.LoginInput{Username: "korban.cookie", Password: markerPassword}); err != nil {
		t.Fatal(err)
	}
	if rep, _ := r.ReconcileRestore(ctx, true); rep.OrphanKeys != 0 || rep.LiveSessions != 1 {
		t.Fatalf("second dry run: %+v", rep)
	}
}

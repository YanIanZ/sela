package account_test

// Regression tests for the findings of auth review round 3 (R3-1 to R3-3,
// R3-5). The HTTP-level ones (R3-3, R3-4, R3-6) are in
// internal/httpapi/v3_review_round3_test.go.

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/YanIanZ/sela/internal/account"
)

const wrongPassword = "salah salah salah salah"

// R3-1: after a parameter change, a change in the number of credentials
// (here one registration) and the next distribution refresh move only the
// unknown usernames near the boundary between the two tuples, as many as
// the boundary moved; before, about half of them were re-drawn, while
// existing accounts kept their stored tuple, so a username whose cost
// changed between two refresh periods was certainly unknown.
func TestR3UnknownUsernameTupleStableWhenCountChanges(t *testing.T) {
	t.Parallel()
	a := account.FloorHashParams                                             // the old parameters
	b := account.HashParams{MemoryKiB: 19456, Iterations: 3, Parallelism: 1} // the new ones
	f := newFixture(t, func(c *account.Config) { c.Hash = a })
	const perSide = 8
	for i := 0; i < perSide; i++ {
		f.register(fmt.Sprintf("lama.akun%02d", i)) // stored at a, never sign in again
	}
	s := f.restart(func(c *account.Config) { c.Hash = b })
	f.store = s
	for i := 0; i < perSide; i++ {
		f.register(fmt.Sprintf("baru.akun%02d", i)) // stored at b
	}
	if err := s.RefreshHashDistribution(ctx); err != nil {
		t.Fatal(err)
	}
	h := s.Hasher()
	// tupleOf runs one failed login and reports which parameters the
	// verification ran with (what its latency shows a client).
	tupleOf := func(u string) account.HashParams {
		t.Helper()
		ba, bb := h.VerificationsAt(a), h.VerificationsAt(b)
		if _, err := s.Login(ctx, account.LoginInput{Username: u, Password: wrongPassword}); !errors.Is(err, account.ErrInvalidCredentials) {
			t.Fatalf("%s: %v", u, err)
		}
		switch {
		case h.VerificationsAt(a) == ba+1 && h.VerificationsAt(b) == bb:
			return a
		case h.VerificationsAt(b) == bb+1 && h.VerificationsAt(a) == ba:
			return b
		}
		t.Fatalf("%s: not exactly one verification", u)
		return account.HashParams{}
	}
	// A few real logins tie UnknownParams to what Login does.
	const sampled = 6
	for i := 0; i < sampled; i++ {
		u := fmt.Sprintf("tak.ada.%02d", i)
		if got, want := tupleOf(u), h.UnknownParams(s.UsernameKey(u)); got != want {
			t.Fatalf("%s verified at %+v, UnknownParams says %+v", u, got, want)
		}
	}
	known := map[string]account.HashParams{}
	for i := 0; i < perSide; i++ {
		u := fmt.Sprintf("lama.akun%02d", i)
		known[u] = tupleOf(u)
	}
	const n = 4000
	before := make([]account.HashParams, n)
	for i := range before {
		before[i] = h.UnknownParams(s.UsernameKey(fmt.Sprintf("tak.ada.%04d", i)))
	}
	// Ten minutes later: one person registered, and the job refreshed.
	f.register("orang.baru")
	if err := s.RefreshHashDistribution(ctx); err != nil {
		t.Fatal(err)
	}
	moved := 0
	for i := range before {
		if h.UnknownParams(s.UsernameKey(fmt.Sprintf("tak.ada.%04d", i))) != before[i] {
			moved++
		}
	}
	for u, p := range known {
		if got := tupleOf(u); got != p {
			t.Fatalf("existing account %s changed tuple without signing in: %+v -> %+v", u, p, got)
		}
	}
	for i := 0; i < sampled; i++ {
		u := fmt.Sprintf("tak.ada.%02d", i)
		if got, want := tupleOf(u), h.UnknownParams(s.UsernameKey(u)); got != want {
			t.Fatalf("after the refresh %s verified at %+v, UnknownParams says %+v", u, got, want)
		}
	}
	// The a/b boundary moved from 8/16 to 8/17 of the key space: about 2.9%
	// of the unknown usernames (about 118 of 4000) change tuple; mapping by
	// remainder moved about half of them.
	t.Logf("after one registration and a refresh: %d of %d unknown usernames changed tuple (boundary shift %.4f)", moved, n, 0.5-8.0/17)
	if moved > n*6/100 {
		t.Errorf("%d of %d unknown usernames changed tuple after one registration; only those between the old and the new boundary may", moved, n)
	}
}

// R3-2: a failed proof locks the credentials row before the throttle row,
// as every success path does. Deterministic: a transaction that holds the
// credentials row (as a successful login does between its two updates)
// must still be able to clear the throttle row while a failed login of
// the same account waits. Before the fix the failure statement held the
// throttle row and waited for the credentials row: a deadlock.
func TestR3FailedProofLocksCredentialsBeforeThrottle(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("urutan.kunci")
	key := f.store.UsernameKey("urutan.kunci")
	// An earlier typo: the throttle row exists, so a failure statement that
	// upserted it first would hold its lock while waiting.
	if _, err := f.store.Login(ctx, account.LoginInput{Username: "urutan.kunci", Password: wrongPassword}); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatal(err)
	}
	tx, err := f.db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE credentials SET failed_count = 0 WHERE username = 'urutan.kunci'`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.store.Login(ctx, account.LoginInput{Username: "urutan.kunci", Password: wrongPassword})
		done <- err
	}()
	waitForLockWait(t, f, 1)
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '3s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM login_throttle WHERE key_hash = $1`, key[:]); err != nil {
		t.Fatalf("the success side could not take the throttle row while a failed proof waited (lock order inverted): %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatalf("failed login: %v", err)
	}
	if f.store.DeadlockRetries() != 0 {
		t.Fatalf("%d deadlock retries", f.store.DeadlockRetries())
	}
}

// R3-2 (second inversion): a password change locks the credentials row
// before the current session row, as a recovery does before it revokes
// every session of the user. Deterministic: while a recovery-like
// transaction holds the credentials row, a password change must not hold
// any session row of the user.
func TestR3PasswordChangeLocksCredentialsBeforeSessions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("ganti.urut")
	p := f.principal(res.Token)
	tx, err := f.db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM credentials WHERE user_id = $1 FOR UPDATE`, p.UserID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.store.ChangePassword(ctx, p.UserID, p.SessionID, markerPassword, newPassword, "")
		done <- err
	}()
	waitForLockWait(t, f, 1)
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '3s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET last_seen_at = last_seen_at WHERE user_id = $1`, p.UserID); err != nil {
		t.Fatalf("a password change held a session row while waiting for the credentials row (lock order inverted): %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("password change: %v", err)
	}
}

// waitForLockWait waits until n other backends of this test's schema wait
// for a row lock.
func waitForLockWait(t *testing.T, f *fixture, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := f.db.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock'
			AND datname = current_database() AND pid <> pg_backend_pid() AND query LIKE '%credentials%'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the store call never waited for the held row lock")
}

// R3-2 (stress, the reviewer's probe): concurrent successful and failed
// logins of one account, through the real Store.Login, never deadlock;
// before the fix PostgreSQL aborted one side with 40P01 (HTTP 500) about
// every five seconds. The retry backstop must not have been needed either.
func TestR3FailedAndSuccessfulLoginsDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("kunci.mati")
	var deadlocks, other, ok atomic.Int64
	stop := time.Now().Add(8 * time.Second)
	var wg sync.WaitGroup
	run := func(password string) {
		defer wg.Done()
		for time.Now().Before(stop) {
			_, err := f.store.Login(ctx, account.LoginInput{Username: "kunci.mati", Password: password})
			var pg *pgconn.PgError
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, account.ErrInvalidCredentials), errors.Is(err, account.ErrRateLimited):
			case errors.As(err, &pg) && pg.Code == "40P01":
				deadlocks.Add(1)
			default:
				other.Add(1)
				t.Errorf("unexpected error: %v", err)
			}
		}
	}
	for i := 0; i < 3; i++ {
		wg.Add(2)
		go run(markerPassword)
		go run(wrongPassword)
	}
	wg.Wait()
	t.Logf("successful logins %d; deadlocks %d; retries %d; other errors %d", ok.Load(), deadlocks.Load(), f.store.DeadlockRetries(), other.Load())
	if deadlocks.Load() > 0 || f.store.DeadlockRetries() > 0 {
		t.Errorf("concurrent failed and successful logins deadlocked: %d surfaced, %d retried", deadlocks.Load(), f.store.DeadlockRetries())
	}
	if ok.Load() == 0 {
		t.Error("no login succeeded")
	}
}

// R3-3: concurrent attempts cannot all pass one read of the backoff: each
// is reserved in one statement before its argon2id work, so 20 concurrent
// wrong logins verify exactly the 5 the schedule allows before a wait.
func TestR3ConcurrentAttemptsCannotBypassBackoff(t *testing.T) {
	t.Parallel()
	for _, u := range []string{"serbu.serentak", "tidak.terdaftar"} {
		t.Run(u, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.register("serbu.serentak")
			const n = 20
			var wg sync.WaitGroup
			var mu sync.Mutex
			got := map[string]int{}
			before := f.store.Hasher().Verifications()
			start := make(chan struct{})
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, err := f.store.Login(ctx, account.LoginInput{Username: u, Password: wrongPassword})
					mu.Lock()
					got[account.CodeOf(err)]++
					mu.Unlock()
				}()
			}
			close(start)
			wg.Wait()
			verified := f.store.Hasher().Verifications() - before
			key := f.store.UsernameKey(u)
			failures := f.count(`SELECT failures FROM login_throttle WHERE key_hash = $1`, key[:])
			t.Logf("results %v; verified %d; throttle failures %d", got, verified, failures)
			if verified != 5 || got["invalid_credentials"] != 5 || got["rate_limited"] != n-5 || failures != 5 {
				t.Fatalf("%d concurrent guesses: %v, %d verified, throttle %d; want 5 verified, then rate_limited", n, got, verified, failures)
			}
		})
	}
}

// R3-3: the right password that started just before the 100th failure was
// recorded never signs in after that failure disabled password sign-in.
// The reviewer's probe: 99 earlier failures, then 3 wrong attempts and
// the right one together.
func TestR3SignInAfterDisableInSameBurst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("hampir.kunci")
	for round := 0; round < 20; round++ {
		if _, err := f.db.Pool.Exec(ctx, `UPDATE credentials SET failed_count = 99, password_login_disabled_at = NULL`); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Pool.Exec(ctx, `DELETE FROM login_throttle`); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var okErr error
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = f.store.Login(ctx, account.LoginInput{Username: "hampir.kunci", Password: wrongPassword})
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(round%5) * time.Millisecond)
			_, okErr = f.store.Login(ctx, account.LoginInput{Username: "hampir.kunci", Password: markerPassword})
		}()
		wg.Wait()
		disabled := f.count(`SELECT count(*) FROM credentials WHERE password_login_disabled_at IS NOT NULL`)
		if okErr == nil && disabled != 0 {
			t.Fatalf("round %d: a password sign-in succeeded although the 100th attempt disabled password sign-in", round)
		}
		if okErr != nil && !errors.Is(okErr, account.ErrPasswordLoginDisabled) && !errors.Is(okErr, account.ErrRateLimited) {
			t.Fatalf("round %d: %v", round, okErr)
		}
		if n := f.count(`SELECT failed_count FROM credentials`); n > account.DisableAfter {
			t.Fatalf("round %d: %d attempts counted, more than %d verified", round, n, account.DisableAfter)
		}
	}
}

// R3-3, deterministic: a proof that passed the reservation and is still
// being verified when password sign-in is disabled (another attempt
// reached the ceiling) is refused by the success transaction, for every
// password proof.
func TestR3SuccessRefusedWhenDisabledDuringVerification(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("dikunci.tengah")
	p := f.principal(res.Token)
	proofs := map[string]func() error{
		"login": func() error {
			_, err := f.store.Login(ctx, account.LoginInput{Username: "dikunci.tengah", Password: markerPassword})
			return err
		},
		"reauth": func() error {
			_, err := f.store.Reauthenticate(ctx, p.UserID, p.SessionID, markerPassword)
			return err
		},
		"password change": func() error {
			_, err := f.store.ChangePassword(ctx, p.UserID, p.SessionID, markerPassword, newPassword, "")
			return err
		},
		"code regeneration": func() error {
			_, _, err := f.store.RegenerateRecoveryCodes(ctx, p.UserID, p.SessionID, markerPassword)
			return err
		},
	}
	for name, proof := range proofs {
		if _, err := f.db.Pool.Exec(ctx, `UPDATE credentials SET failed_count = 0, password_login_disabled_at = NULL`); err != nil {
			t.Fatal(err)
		}
		release := f.store.Hasher().Occupy() // the proof reserves, then waits for a hashing slot
		done := make(chan error, 1)
		go func() { done <- proof() }()
		deadline := time.Now().Add(5 * time.Second)
		for f.count(`SELECT failed_count FROM credentials`) == 0 {
			if time.Now().After(deadline) {
				release()
				t.Fatalf("%s: the attempt was never reserved", name)
			}
			time.Sleep(5 * time.Millisecond)
		}
		if _, err := f.db.Pool.Exec(ctx, `UPDATE credentials SET password_login_disabled_at = $1`, f.clock.Now()); err != nil {
			t.Fatal(err)
		}
		release()
		if err := <-done; !errors.Is(err, account.ErrPasswordLoginDisabled) {
			t.Fatalf("%s: the right password after the disable: %v, want password_login_disabled", name, err)
		}
	}
	if n := f.count(`SELECT count(*) FROM sessions WHERE revoked_at IS NULL`); n != 1 {
		t.Fatalf("%d live sessions; a refused proof created or rotated one", n)
	}
}

// R3-3: at most DisableAfter attempts are verified after the last
// success. With the ceiling reached and an attempt in flight, a further
// attempt waits; one that never reported back counts as failed after a
// minute and disables password sign-in.
func TestR3CeilingCountsAttemptsInFlight(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("di.batas")
	now := f.clock.Now()
	if _, err := f.db.Pool.Exec(ctx, `UPDATE credentials SET failed_count = $1, last_failed_at = $2`, account.DisableAfter, now); err != nil {
		t.Fatal(err)
	}
	before := f.store.Hasher().Verifications()
	_, err := f.store.Login(ctx, account.LoginInput{Username: "di.batas", Password: markerPassword})
	if account.CodeOf(err) != "rate_limited" || f.store.Hasher().Verifications() != before {
		t.Fatalf("with the 100th attempt in flight: %v (verified %d)", err, f.store.Hasher().Verifications()-before)
	}
	f.clock.Advance(2 * time.Minute)
	_, err = f.store.Login(ctx, account.LoginInput{Username: "di.batas", Password: markerPassword})
	if !errors.Is(err, account.ErrPasswordLoginDisabled) || f.store.Hasher().Verifications() != before {
		t.Fatalf("after the attempt in flight timed out: %v", err)
	}
	if f.count(`SELECT count(*) FROM credentials WHERE password_login_disabled_at IS NOT NULL`) != 1 ||
		f.count(`SELECT count(*) FROM audit_events WHERE type = 'password_login.disabled'`) != 1 {
		t.Fatal("the timed-out attempt did not disable password sign-in")
	}
}

// R3-3: a reservation whose proof never ran (the hashing slots stayed
// busy) is given back, so a busy server does not count failures.
func TestR3ReservationReleasedWhenHashingBusy(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(c *account.Config) { c.HashWait = 50 * time.Millisecond })
	f.register("sibuk.sekali")
	for _, u := range []string{"sibuk.sekali", "tak.dikenal"} {
		release := f.store.Hasher().Occupy()
		_, err := f.store.Login(ctx, account.LoginInput{Username: u, Password: wrongPassword})
		release()
		if !errors.Is(err, account.ErrServerBusy) {
			t.Fatalf("%s: %v", u, err)
		}
	}
	if n := f.count(`SELECT count(*) FROM login_throttle`); n != 0 {
		t.Fatalf("%d throttle rows after busy answers", n)
	}
	if n := f.count(`SELECT failed_count FROM credentials`); n != 0 {
		t.Fatalf("failed_count %d after a busy answer", n)
	}
}

// R3-2/R3-3: a deletion takes the credentials row before the throttle row
// too, so it never deadlocks with a failed proof of the same account.
func TestR3DeletionLocksCredentialsFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("hapus.urut")
	p := f.principal(res.Token)
	if _, err := f.store.Reauthenticate(ctx, p.UserID, p.SessionID, markerPassword); err != nil {
		t.Fatal(err)
	}
	key := f.store.UsernameKey("hapus.urut")
	// A throttle row exists (an earlier typo), so a deletion that cleared
	// it before locking the credentials row would hold its lock.
	if _, err := f.db.Pool.Exec(ctx, `INSERT INTO login_throttle (key_hash, failures, last_failed_at) VALUES ($1, 1, $2)`, key[:], f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	// A failed-proof-like transaction holds the credentials row and then
	// wants the throttle row; the deletion must wait for the credentials
	// row without holding the throttle row.
	tx, err := f.db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM credentials WHERE user_id = $1 FOR NO KEY UPDATE`, p.UserID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.store.DeleteAccount(ctx, p.UserID, p.SessionID)
		done <- err
	}()
	waitForLockWait(t, f, 1)
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '3s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO login_throttle (key_hash, failures, last_failed_at) VALUES ($1, 1, $2)
		ON CONFLICT (key_hash) DO UPDATE SET failures = login_throttle.failures + 1`, key[:], f.clock.Now()); err != nil {
		t.Fatalf("the deletion held the throttle row while waiting for the credentials row: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

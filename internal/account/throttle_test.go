package account_test

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

// outcome is the observable answer of one login attempt.
func outcome(err error) string {
	var e *account.Error
	if errors.As(err, &e) {
		return fmt.Sprintf("%s/%s", e.Code, e.RetryAfter)
	}
	return fmt.Sprint(err)
}

// S1: after a restart a known and an unknown username answer the same,
// Retry-After included, at every point of the backoff schedule: the
// throttle is one persisted table keyed by username for both.
func TestLoginBackoffUniformAfterRestart(t *testing.T) {
	t.Parallel()
	for name, gap := range map[string]time.Duration{
		"restart within the backoff":     0,
		"restart ten minutes later":      10 * time.Minute,
		"restart after the quiet period": account.ThrottleQuiet + time.Minute,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.register("korban.lupa")
			wrong := func(u string) account.LoginInput {
				return account.LoginInput{Username: u, Password: "salah salah salah salah"}
			}
			for _, u := range []string{"korban.lupa", "tidak.pernah.ada"} {
				for i := 0; i < 5; i++ {
					if _, err := f.store.Login(ctx, wrong(u)); !errors.Is(err, account.ErrInvalidCredentials) {
						t.Fatalf("setup %s %d: %v", u, i, err)
					}
				}
			}
			f.clock.Advance(gap)
			// A deploy: a new process on the same database.
			s2 := f.restart()
			seq := func(u string) []string {
				var out []string
				for i := 0; i < 3; i++ {
					_, err := s2.Login(ctx, wrong(u))
					out = append(out, outcome(err))
				}
				return out
			}
			known, unknown := seq("korban.lupa"), seq("tidak.pernah.ada")
			if !slices.Equal(known, unknown) {
				t.Fatalf("known and unknown usernames differ after a restart: %v vs %v", known, unknown)
			}
			want := map[time.Duration][]string{
				0:                                   {"rate_limited/1s", "rate_limited/1s", "rate_limited/1s"},
				10 * time.Minute:                    {"invalid_credentials/0s", "rate_limited/2s", "rate_limited/2s"},
				account.ThrottleQuiet + time.Minute: {"invalid_credentials/0s", "invalid_credentials/0s", "invalid_credentials/0s"},
			}[gap]
			if !slices.Equal(known, want) {
				t.Fatalf("schedule after restart: %v, want %v", known, want)
			}
		})
	}
}

// S2: a wrong password for an existing account and an unknown username
// send the same statements, in the same order, so neither the number of
// round trips nor a commit tells them apart. Since review round 3 they are
// the reservation (credentials lookup and lock, throttle, attempt
// counted) and the failure statement.
func TestLoginFailureSameStatements(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("ada.akun")
	rec := &sqlRecorder{}
	s := f.tracedStore(rec)
	attempt := func(u string) []string {
		t.Helper()
		_, _ = rec.take()
		if _, err := s.Login(ctx, account.LoginInput{Username: u, Password: "salah salah salah salah"}); !errors.Is(err, account.ErrInvalidCredentials) {
			t.Fatalf("%s: %v", u, err)
		}
		sqls, errs := rec.take()
		if len(errs) != 0 {
			t.Fatalf("%s: statements failed: %v", u, errs)
		}
		return sqls
	}
	known, unknown := attempt("ada.akun"), attempt("tak.ada")
	if !slices.Equal(known, unknown) {
		t.Fatalf("different statements:\nknown   %q\nunknown %q", known, unknown)
	}
	if len(known) != 2 || !strings.HasPrefix(known[0], "WITH cred AS (") || !strings.HasPrefix(known[1], "WITH account AS (") {
		t.Fatalf("expected the reservation and one failure statement, got %q", known)
	}
	// The failure statement wrote for both: the account counts it, and both
	// usernames have a throttle row.
	if n := f.count(`SELECT failed_count FROM credentials`); n != 1 {
		t.Fatalf("failed_count %d", n)
	}
	if n := f.count(`SELECT count(*) FROM login_throttle`); n != 2 {
		t.Fatalf("%d throttle rows", n)
	}
	// Throttled answers send the same statements too.
	for i := 0; i < 4; i++ {
		attempt("ada.akun")
		attempt("tak.ada")
	}
	throttled := func(u string) []string {
		_, _ = rec.take()
		_, err := s.Login(ctx, account.LoginInput{Username: u, Password: "salah salah salah salah"})
		if account.CodeOf(err) != "rate_limited" {
			t.Fatalf("%s: %v", u, err)
		}
		sqls, _ := rec.take()
		return sqls
	}
	if k, u := throttled("ada.akun"), throttled("tak.ada"); !slices.Equal(k, u) {
		t.Fatalf("throttled answers differ:\n%q\n%q", k, u)
	}
}

// S3: the live-guest cap holds when guests start concurrently.
func TestGuestCapConcurrent(t *testing.T) {
	t.Parallel()
	const maxGuests, n = 2, 12
	f := newFixture(t, func(c *account.Config) { c.MaxGuests = maxGuests })
	// A second instance on the same schema competes as well.
	other := f.restart()
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, busy := 0, 0
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			s := f.store
			if i%2 == 1 {
				s = other
			}
			_, err := s.StartGuest(ctx, account.GuestInput{Locale: "en", AgeBand: account.AgeBand18Plus, TermsPolicyVersion: termsV, GuestNoticeVersion: guestV})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				created++
			case errors.Is(err, account.ErrServerBusy):
				busy++
			default:
				t.Error(err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if live := f.count(`SELECT count(*) FROM users WHERE kind = 'guest'`); created != maxGuests || busy != n-maxGuests || live != maxGuests {
		t.Fatalf("MaxGuests=%d, %d concurrent starts: %d created, %d busy, %d live", maxGuests, n, created, busy, live)
	}
}

// R1-5: a taken username is answered without a failing statement, so
// PostgreSQL logs no error (whose DETAIL would name the username).
func TestUsernameTakenLeavesNoServerError(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	rec := &sqlRecorder{}
	s := f.tracedStore(rec)
	const name = "penanda.log.unik"
	in := account.RegisterInput{Username: name, Password: markerPassword, Locale: "id", AgeBand: account.AgeBand18Plus, TermsPolicyVersion: termsV}
	if _, err := s.Register(ctx, in); err != nil {
		t.Fatal(err)
	}
	_, _ = rec.take()
	in.Username = "Penanda.Log.Unik" // case-insensitive uniqueness
	if _, err := s.Register(ctx, in); !errors.Is(err, account.ErrUsernameTaken) {
		t.Fatalf("second register: %v", err)
	}
	g, err := s.StartGuest(ctx, account.GuestInput{Locale: "id", AgeBand: account.AgeBand18Plus, TermsPolicyVersion: termsV, GuestNoticeVersion: guestV})
	if err != nil {
		t.Fatal(err)
	}
	p := f.principal(g.Token)
	if _, err := s.UpgradeGuest(ctx, p.UserID, p.SessionID, account.UpgradeInput{Username: name, Password: newPassword}); !errors.Is(err, account.ErrUsernameTaken) {
		t.Fatalf("upgrade to a taken username: %v", err)
	}
	if _, errs := rec.take(); len(errs) != 0 {
		t.Fatalf("statements failed on the server (each would be logged with its DETAIL): %v", errs)
	}
	if n := f.count(`SELECT count(*) FROM users`); n != 2 {
		t.Fatalf("%d users; the refused registration left rows", n)
	}
	// Optional: when the test server's log file is readable, check it.
	if path := os.Getenv("SELA_TEST_PG_LOG"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("SELA_TEST_PG_LOG: %v", err)
		}
		if strings.Contains(string(raw), name) {
			t.Fatal("the PostgreSQL server log contains the username")
		}
	}
}

// R1-7: a withdrawal never stores the version text the client sent; free
// text is refused, and the version recorded is the one last agreed to.
func TestConsentWithdrawalStoresNoClientText(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	g := f.guest(account.AgeBand18Plus)
	p := f.principal(g.Token)
	const text = "aku depresi, hubungi 0812 3456 7890"
	if _, _, err := f.store.SetConsent(ctx, p.UserID, p.SessionID, "test_optional", false, text); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("free-text version accepted: %v", err)
	}
	// Never decided: the withdrawal records the current version.
	v, changed, err := f.store.SetConsent(ctx, p.UserID, p.SessionID, "test_optional", false, "made-up.9")
	if err != nil || !changed || v.PolicyVersion == nil || *v.PolicyVersion != optionalV {
		t.Fatalf("first withdrawal: %+v %t %v", v, changed, err)
	}
	f.clock.Advance(time.Second)
	if _, _, err := f.store.SetConsent(ctx, p.UserID, p.SessionID, "test_optional", true, optionalV); err != nil {
		t.Fatal(err)
	}
	// After a grant, a withdrawal on any well-formed version records the
	// granted one; repeating it changes nothing.
	f.clock.Advance(time.Second)
	v, changed, err = f.store.SetConsent(ctx, p.UserID, p.SessionID, "test_optional", false, "old-version.0")
	if err != nil || !changed || *v.PolicyVersion != optionalV || v.Granted {
		t.Fatalf("withdrawal: %+v %t %v", v, changed, err)
	}
	if _, changed, err := f.store.SetConsent(ctx, p.UserID, p.SessionID, "test_optional", false, "another.1"); err != nil || changed {
		t.Fatalf("repeated withdrawal changed: %t %v", changed, err)
	}
	dump := dbtest.DumpText(t, f.db.Pool)
	for _, s := range []string{"0812", "depresi", "made-up.9", "old-version.0", "another.1"} {
		if strings.Contains(dump, s) {
			t.Errorf("client text %q stored", s)
		}
	}
}

// Throttle rows go with the quiet period (retention) and with the account.
func TestLoginThrottleRetentionAndDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	res := f.register("hapus.lambat")
	for _, u := range []string{"hapus.lambat", "nama.asing"} {
		if _, err := f.store.Login(ctx, account.LoginInput{Username: u, Password: "salah salah salah salah"}); !errors.Is(err, account.ErrInvalidCredentials) {
			t.Fatal(err)
		}
	}
	if n := f.count(`SELECT count(*) FROM login_throttle`); n != 2 {
		t.Fatalf("%d throttle rows", n)
	}
	p := f.principal(res.Token)
	if _, err := f.store.DeleteAccount(ctx, p.UserID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	key := f.store.UsernameKey("hapus.lambat")
	if n := f.count(`SELECT count(*) FROM login_throttle WHERE key_hash = $1`, key[:]); n != 0 {
		t.Fatal("deletion kept the account's throttle row")
	}
	f.clock.Advance(account.ThrottleQuiet + time.Second)
	rep, err := f.store.Retention(ctx)
	if err != nil || rep.Throttle != 1 || f.count(`SELECT count(*) FROM login_throttle`) != 0 {
		t.Fatalf("retention: %+v %v", rep, err)
	}
}

package account_test

import (
	"context"
	"crypto/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/vault"
)

var ctx = context.Background()

// timingAssertions reports whether latency assertions run: only when
// SELA_TIMING_TESTS=1 and not under -race (review round 3, R3-5). In the
// default suite (go test -race ./..., CI) a latency comparison is logged
// as a measurement, and the deterministic checks (which parameters each
// verification ran with, Hasher.VerificationsAt) carry the property.
func timingAssertions() bool { return os.Getenv("SELA_TIMING_TESTS") == "1" && !raceEnabled }

// Synthetic markers used across tests; none is real data.
const (
	markerPassword = "Lembah-Kabut-Sintetis 7 marker"
	newPassword    = "sungai biru pagi hari tanpa hujan"
	markerGuardian = "Ibu Penanda Sintetis Kelana"
	termsV         = "terms-privacy-test.1"
	guestV         = "guest-storage-test.1"
	guardianV      = "guardian-information-test.1"
	optionalV      = "optional-test.1"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock { return &clock{now: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }

func (c *clock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type fixture struct {
	t     *testing.T
	db    *dbtest.DB
	store *account.Store
	vault *vault.Vault
	kek   vault.Key
	clock *clock
	cfg   account.Config
}

func testConfig(c *clock) account.Config {
	cfg := account.DefaultConfig("test")
	cfg.Hash = account.FloorHashParams
	cfg.Now = c.Now
	cfg.PolicyVersions = map[string]string{account.PolicyTerms: termsV, account.PolicyGuest: guestV, account.PolicyGuardian: guardianV, "test_optional": optionalV}
	cfg.Purposes = append(append([]account.Purpose(nil), account.FoundationPurposes...),
		account.Purpose{Name: "test_optional", Kinds: []string{account.KindGuest, account.KindAccount}})
	return cfg
}

func newFixture(t *testing.T, mutate ...func(*account.Config)) *fixture {
	t.Helper()
	d := dbtest.New(t)
	c := newClock()
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	kek := vault.Key{ID: "test-unit", Secret: secret}
	v, err := vault.New(vault.Keyring{Current: kek}, vault.Options{Now: c.Now})
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(c)
	for _, m := range mutate {
		m(&cfg)
	}
	s, err := account.New(d.Pool, v, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, db: d, store: s, vault: v, kek: kek, clock: c, cfg: cfg}
}

// another store instance on the same schema (a restarted server).
func (f *fixture) restart(mutate ...func(*account.Config)) *account.Store {
	f.t.Helper()
	v, err := vault.New(vault.Keyring{Current: f.kek}, vault.Options{Now: f.clock.Now})
	if err != nil {
		f.t.Fatal(err)
	}
	cfg := f.cfg
	for _, m := range mutate {
		m(&cfg)
	}
	s, err := account.New(f.db.Pool, v, cfg)
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) register(username string) account.AuthResult {
	f.t.Helper()
	res, err := f.store.Register(ctx, account.RegisterInput{Username: username, Password: markerPassword, Locale: "id",
		AgeBand: account.AgeBand18Plus, TermsPolicyVersion: termsV, UserAgent: "Mozilla/5.0 (Android 14) Firefox/131.0"})
	if err != nil {
		f.t.Fatalf("register %s: %v", username, err)
	}
	return res
}

func (f *fixture) guest(band string) account.AuthResult {
	f.t.Helper()
	res, err := f.store.StartGuest(ctx, account.GuestInput{Locale: "en", AgeBand: band, TermsPolicyVersion: termsV, GuestNoticeVersion: guestV})
	if err != nil {
		f.t.Fatalf("guest: %v", err)
	}
	return res
}

func (f *fixture) principal(token string) account.Principal {
	f.t.Helper()
	p, err := f.store.Authenticate(ctx, token)
	if err != nil {
		f.t.Fatalf("authenticate: %v", err)
	}
	return p
}

func mustID(t *testing.T, s string) account.UserID {
	t.Helper()
	u, err := account.ParseUUID(s)
	if err != nil {
		t.Fatal(err)
	}
	return account.UserID(u)
}

func mustSession(t *testing.T, s string) account.SessionID {
	t.Helper()
	u, err := account.ParseUUID(s)
	if err != nil {
		t.Fatal(err)
	}
	return account.SessionID(u)
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.db.Pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// sqlRecorder is a pgx tracer that records every statement a store sends
// and every statement that failed on the server.
type sqlRecorder struct {
	mu   sync.Mutex
	sqls []string
	errs []error
}

func (r *sqlRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	r.sqls = append(r.sqls, data.SQL)
	r.mu.Unlock()
	return ctx
}

func (r *sqlRecorder) TraceQueryEnd(_ context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		r.mu.Lock()
		r.errs = append(r.errs, data.Err)
		r.mu.Unlock()
	}
}

// take returns and clears what was recorded.
func (r *sqlRecorder) take() ([]string, []error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sqls, errs := r.sqls, r.errs
	r.sqls, r.errs = nil, nil
	return sqls, errs
}

// tracedStore is another store instance on the fixture's schema whose
// pool reports every statement to rec.
func (f *fixture) tracedStore(rec *sqlRecorder, mutate ...func(*account.Config)) *account.Store {
	f.t.Helper()
	pc, err := pgxpool.ParseConfig(dbtest.URL(f.t))
	if err != nil {
		f.t.Fatal(err)
	}
	pc.ConnConfig.RuntimeParams["search_path"] = f.db.Schema
	pc.ConnConfig.Tracer = rec
	pc.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(pool.Close)
	v, err := vault.New(vault.Keyring{Current: f.kek}, vault.Options{Now: f.clock.Now})
	if err != nil {
		f.t.Fatal(err)
	}
	cfg := f.cfg
	for _, m := range mutate {
		m(&cfg)
	}
	s, err := account.New(pool, v, cfg)
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

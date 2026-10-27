// Package db owns Sela's PostgreSQL plumbing: the connection pool and its
// session settings, a transaction helper, the embedded migration runner
// (migrate.go), the environment marker in instance_meta and the advisory
// lock that lets exactly one instance run a background job.
//
// It holds no product rules and never logs content, database URLs or
// error texts that could carry values; callers get errors without the URL.
package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is what pools, connections and transactions have in common. Store
// code takes it so the same query runs inside or outside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Beginner starts transactions (a pool or a connection).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Options configures Open.
type Options struct {
	// URL is a postgres:// URL. It is secret: never logged or returned in
	// an error.
	URL string
	// StatementTimeout bounds one statement (default 30s).
	StatementTimeout time.Duration
	// IdleInTxTimeout ends sessions that idle inside a transaction
	// (default 60s).
	IdleInTxTimeout time.Duration
	// MaxConns bounds the pool (default 10).
	MaxConns int32
	// SearchPath, when set, is the session search_path (tests use one
	// schema each). Otherwise the server default (or a search_path URL
	// parameter) applies.
	SearchPath string
	// ConnectTimeout bounds the initial ping (default 10s).
	ConnectTimeout time.Duration
}

// ErrInvalidURL is returned for a URL pgx cannot parse.
var ErrInvalidURL = errors.New("db: SELA_DATABASE_URL is not a valid PostgreSQL URL")

// Open builds a pool with Sela's session settings and checks that the
// database answers. Errors never contain the URL.
func Open(ctx context.Context, o Options) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(o.URL)
	if err != nil {
		return nil, ErrInvalidURL
	}
	if o.StatementTimeout <= 0 {
		o.StatementTimeout = 30 * time.Second
	}
	if o.IdleInTxTimeout <= 0 {
		o.IdleInTxTimeout = 60 * time.Second
	}
	if o.MaxConns <= 0 {
		o.MaxConns = 10
	}
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 10 * time.Second
	}
	rp := cfg.ConnConfig.RuntimeParams
	rp["statement_timeout"] = fmt.Sprint(o.StatementTimeout.Milliseconds())
	rp["idle_in_transaction_session_timeout"] = fmt.Sprint(o.IdleInTxTimeout.Milliseconds())
	rp["application_name"] = "sela"
	rp["timezone"] = "UTC"
	if o.SearchPath != "" {
		rp["search_path"] = o.SearchPath
	}
	cfg.MaxConns = o.MaxConns
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: cannot create pool (%s)", Classify(err))
	}
	pctx, cancel := context.WithTimeout(ctx, o.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: cannot connect to the database (%s)", Classify(err))
	}
	return pool, nil
}

// Classify reduces an error to a fixed, value-free category for logs and
// messages: a SQLSTATE, or one of canceled, timeout, auth, network, other.
// Error texts are never used because they can carry values (usernames in
// constraint details, hosts in connection errors).
func Classify(err error) string {
	if err == nil {
		return "ok"
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "28P01", "28000":
			return "auth"
		case "3D000":
			return "database_missing"
		}
		return "sqlstate_" + pg.Code
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var ne net.Error
	if errors.As(err, &ne) {
		if ne.Timeout() {
			return "timeout"
		}
		return "network"
	}
	var ce *pgconn.ConnectError
	if errors.As(err, &ce) {
		return "connect"
	}
	return "other"
}

// Unavailable reports an error that means the database cannot serve the
// request right now, as opposed to a statement that failed: a refused or
// lost connection, a network error or timeout, a closed pool, a server
// that is shutting down, starting or out of connections (SQLSTATE class
// 08, 57P01-57P03, 53300), and a statement cut off by statement_timeout
// (57014). The HTTP layer answers these with 503 store_unavailable and
// Retry-After instead of 500 (API_SPEC_V3 §2).
func Unavailable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "57P01", "57P02", "57P03", "53300", "57014":
			return true
		}
		return strings.HasPrefix(pg.Code, "08")
	}
	switch Classify(err) {
	case "timeout", "network", "connect":
		return true
	}
	// pgxpool returns the pool library's ErrClosedPool, whose text is
	// fixed ("closed pool"); matching it avoids a direct dependency on
	// github.com/jackc/puddle (TestUnavailableClassifiesClosedPool pins it).
	for e := err; e != nil; e = errors.Unwrap(e) {
		if e.Error() == "closed pool" {
			return true
		}
	}
	return false
}

// IsDeadlock reports a deadlock_detected (40P01) or serialization_failure
// (40001): PostgreSQL aborted the transaction to let another one through,
// and running it again is safe.
func IsDeadlock(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001")
}

// RetryAttempts bounds Retry.
const RetryAttempts = 3

// Retry runs fn and, while it fails with IsDeadlock, runs it again after a
// short jittered pause, at most RetryAttempts times in all. fn must build
// its results from scratch on every call (a transaction body, or one
// statement). onRetry, when set, is called before each repeat (a metric).
// It is a backstop: the stores take their row locks in one order
// (ADR-0002 §9), so a deadlock is not expected.
func Retry(ctx context.Context, onRetry func(), fn func() error) error {
	var err error
	for i := 0; i < RetryAttempts; i++ {
		if err = fn(); err == nil || !IsDeadlock(err) || i == RetryAttempts-1 {
			return err
		}
		if onRetry != nil {
			onRetry()
		}
		var b [1]byte
		_, _ = rand.Read(b[:])
		t := time.NewTimer(time.Duration(5+int(b[0])%20) * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return err
		case <-t.C:
		}
	}
	return err
}

// IsUniqueViolation reports a unique_violation (23505), optionally on one
// named constraint.
func IsUniqueViolation(err error, constraint string) bool {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23505" {
		return false
	}
	return constraint == "" || pg.ConstraintName == constraint
}

// InTx runs fn in a transaction: commit when fn returns nil, roll back
// otherwise (and on panic).
func InTx(ctx context.Context, b Beginner, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, b, fn)
}

// InTxOpts is InTx with transaction options (isolation, read only).
func InTxOpts(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, pool, opts, fn)
}

// ErrJobLocked means another instance holds a background job's lock.
var ErrJobLocked = errors.New("db: job lock held by another instance")

// jobLockSQL keys the job lock by job name and schema, like the migration
// lock, so parallel test schemas never contend.
const jobLockKey = `hashtextextended('sela.job:' || $1::text || ':' || current_schema(), 0)`

// JobConn is the connection a background job runs on (WithJobLock): it
// sends statements and starts transactions.
type JobConn interface {
	DBTX
	Beginner
}

// WithJobLock runs fn while holding the session-level advisory lock of a
// background job on a dedicated connection, and hands fn that connection:
// the job runs its statements and transactions on it and takes no other
// connection from the pool, so it never waits for one and every pool size
// works, one included (a job that queried through the pool while this
// connection held the lock waited forever at SELA_DB_MAX_CONNS=1). When
// another instance holds the lock, fn does not run and ErrJobLocked is
// returned.
func WithJobLock(ctx context.Context, pool *pgxpool.Pool, job string, fn func(ctx context.Context, conn JobConn) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+jobLockKey+`)`, job).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrJobLocked
	}
	defer func() {
		uctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(uctx, `SELECT pg_advisory_unlock(`+jobLockKey+`)`, job); err != nil {
			// Ending the session releases the lock.
			_ = conn.Conn().Close(uctx)
		}
	}()
	return fn(ctx, conn)
}

// HoldJobLock takes a job lock and keeps it until release is called; tests
// use it to simulate another instance running the job.
func HoldJobLock(ctx context.Context, pool *pgxpool.Pool, job string) (release func(), err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(`+jobLockKey+`)`, job).Scan(&ok); err != nil || !ok {
		conn.Release()
		if err == nil {
			err = ErrJobLocked
		}
		return nil, err
	}
	return func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(`+jobLockKey+`)`, job)
		conn.Release()
	}, nil
}

// Ping reports whether the database answers within d.
func Ping(ctx context.Context, pool *pgxpool.Pool, d time.Duration) error {
	pctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return pool.Ping(pctx)
}

// QuoteIdent quotes an SQL identifier (schema names in tests and tools).
func QuoteIdent(s string) string { return pgx.Identifier{s}.Sanitize() }

// IsLoopbackHost reports whether a database host is local: a unix socket
// directory, localhost or a loopback address.
func IsLoopbackHost(host string) bool {
	if host == "" || strings.HasPrefix(host, "/") || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

package db_test

// Regression tests for data review round 3 (DOWN-CHECKSUM-UNVERIFIED) and
// for the helpers the auth round 3 fixes rely on (Unavailable for R3-6,
// Retry for the R3-2 backstop).

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

// DOWN-CHECKSUM-UNVERIFIED: down-last compares the down file with the
// down_checksum recorded when the migration was applied, and refuses a
// drifted one (exit 4) instead of running it, deleting the row and
// reporting success while the schema stays. status names the drift without
// blocking up or the server. A row applied without a down file (NULL) has
// nothing to compare and is accepted.
func TestDownLastRefusesDriftedDownFile(t *testing.T) {
	t.Parallel()
	const up = "-- sela:kind schema\n-- sela:requires 0001\nCREATE TABLE drift (id int);\nREVOKE ALL ON drift FROM PUBLIC;"
	applied := embeddedPlus(t, map[string]string{"0100_drift.up.sql": up, "0100_drift.down.sql": "DROP TABLE drift;"})
	drifted := embeddedPlus(t, map[string]string{"0100_drift.up.sql": up, "0100_drift.down.sql": "SELECT 1;"})
	d := dbtest.New(t)
	if _, err := d.Migrator(applied).Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	m := d.Migrator(drifted)
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range st.Entries {
		if e.Version == 100 && (e.State != db.StateApplied || e.Detail == "") {
			t.Fatalf("status of the drifted down file: %+v", e)
		}
	}
	if st.ExitCode() != 0 {
		t.Fatalf("a drifted down file blocked status: exit %d", st.ExitCode())
	}
	if _, err := m.DownLast(ctx, 100, ""); !errors.Is(err, db.ErrDownChecksumMismatch) {
		t.Fatalf("down-last ran a down file that differs from the recorded one: %v", err)
	}
	var tableLeft, rowLeft bool
	if err := d.Pool.QueryRow(ctx, `SELECT to_regclass('drift') IS NOT NULL, EXISTS (SELECT 1 FROM schema_migrations WHERE version = 100)`).Scan(&tableLeft, &rowLeft); err != nil {
		t.Fatal(err)
	}
	if !tableLeft || !rowLeft {
		t.Fatalf("a refused down-last changed the schema (table %t, row %t)", tableLeft, rowLeft)
	}
	// The recorded down file reverts as before.
	if _, err := d.Migrator(applied).DownLast(ctx, 100, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.Pool.QueryRow(ctx, `SELECT to_regclass('drift') IS NOT NULL`).Scan(&tableLeft); err != nil || tableLeft {
		t.Fatalf("down-last with the recorded down file: table left %t, %v", tableLeft, err)
	}

	// A row recorded without a down checksum (by a runner that did not
	// record one) has nothing to compare against and is accepted.
	if _, err := d.Migrator(applied).Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE schema_migrations SET down_checksum = NULL WHERE version = 100`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Migrator(applied).DownLast(ctx, 100, ""); err != nil {
		t.Fatalf("down-last of a row without a recorded down checksum: %v", err)
	}
}

// R3-6: Unavailable tells a database that cannot be reached from a
// statement that failed.
func TestUnavailableClassifiesErrors(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]bool{
		"08006": true, "08001": true, "57P01": true, "57P02": true, "57P03": true, "53300": true, "57014": true,
		"23505": false, "40P01": false, "42P01": false, "28P01": false,
	} {
		if got := db.Unavailable(fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: code})); got != want {
			t.Errorf("SQLSTATE %s: Unavailable %t, want %t", code, got, want)
		}
	}
	for err, want := range map[error]bool{
		nil: false, context.Canceled: false, context.DeadlineExceeded: true, errors.New("something else"): false,
	} {
		if got := db.Unavailable(err); got != want {
			t.Errorf("%v: Unavailable %t, want %t", err, got, want)
		}
	}
	// A refused connection, and the pool after Close (pgxpool returns the
	// pool library's ErrClosedPool; Unavailable matches its fixed text).
	pool, err := pgxpool.New(ctx, "postgres://sela:x@127.0.0.1:1/none?sslmode=disable&connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `SELECT 1`)
	if !db.Unavailable(err) {
		t.Errorf("refused connection (%s) not unavailable", db.Classify(err))
	}
	pool.Close()
	_, err = pool.Exec(ctx, `SELECT 1`)
	if err == nil || !db.Unavailable(err) {
		t.Errorf("closed pool (%v) not unavailable", err)
	}
}

// R3-2 backstop: Retry repeats only a deadlock or serialization failure,
// at most RetryAttempts times in all.
func TestRetryRepeatsOnlyDeadlocks(t *testing.T) {
	t.Parallel()
	deadlock := fmt.Errorf("tx: %w", &pgconn.PgError{Code: "40P01"})
	calls, retries := 0, 0
	err := db.Retry(ctx, func() { retries++ }, func() error {
		calls++
		if calls < 3 {
			return deadlock
		}
		return nil
	})
	if err != nil || calls != 3 || retries != 2 {
		t.Fatalf("two deadlocks then success: %v, %d calls, %d retries", err, calls, retries)
	}
	calls = 0
	if err := db.Retry(ctx, nil, func() error { calls++; return deadlock }); !db.IsDeadlock(err) || calls != db.RetryAttempts {
		t.Fatalf("persistent deadlock: %v after %d calls", err, calls)
	}
	calls = 0
	other := errors.New("not a deadlock")
	if err := db.Retry(ctx, nil, func() error { calls++; return other }); err != other || calls != 1 {
		t.Fatalf("other error: %v after %d calls", err, calls)
	}
}

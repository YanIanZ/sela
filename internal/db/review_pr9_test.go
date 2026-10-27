package db_test

// Regression test for the PR #9 review finding F2: a background job runs
// on the connection that holds its advisory lock, so it needs no second
// connection and works with a pool of one.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

func TestWithJobLockRunsOnItsConnection(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	pool := d.OpenPool(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ran := false
	err := db.WithJobLock(ctx, pool, "pr9_job", func(ctx context.Context, conn db.JobConn) error {
		ran = true
		// The connection handed to the job is the one holding the lock.
		var held int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND granted AND pid = pg_backend_pid()`).Scan(&held); err != nil {
			return err
		}
		if held != 1 {
			return fmt.Errorf("the job's connection holds %d advisory locks, want 1", held)
		}
		// Another instance cannot take the lock meanwhile.
		if release, err := db.HoldJobLock(ctx, d.Pool, "pr9_job"); err == nil {
			release()
			return errors.New("another instance took the job lock while the job held it")
		} else if !errors.Is(err, db.ErrJobLocked) {
			return err
		}
		// Statements and transactions run on it while the pool's only
		// connection is in use.
		return db.InTx(ctx, conn, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `CREATE TEMP TABLE pr9_job (id int)`)
			return err
		})
	})
	if err != nil || !ran {
		t.Fatalf("job with a pool of one: ran=%t, %v", ran, err)
	}
	// The lock and the connection are given back.
	release, err := db.HoldJobLock(ctx, pool, "pr9_job")
	if err != nil {
		t.Fatalf("job lock after the job: %v", err)
	}
	release()
}

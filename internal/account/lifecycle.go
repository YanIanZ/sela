package account

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/db"
)

// Retention periods (ADR-0002 §8, §11).
const (
	// KeyBackupRetention is how long key dumps are kept: after it no backup
	// holds a usable key for a deleted user.
	KeyBackupRetention = 7 * 24 * time.Hour
	// ContentBackupRetention is how long content dumps are kept.
	ContentBackupRetention = 30 * 24 * time.Hour
	// EndedSessionRetention keeps ended session rows for the owner's view.
	EndedSessionRetention = 30 * 24 * time.Hour
	// TombstoneRetention keeps destroyed-key tombstones for the content
	// backup retention plus a week, so a restore can be reconciled.
	TombstoneRetention = ContentBackupRetention + 7*24*time.Hour
	// PurgeBatch bounds one guest purge run.
	PurgeBatch = 100
)

// Background job lock names.
const (
	JobGuestPurge = "guest_purge"
	JobRetention  = "retention"
)

// DeletionReceipt is DELETE /me.
type DeletionReceipt struct {
	Deleted             bool      `json:"deleted"`
	KeysUnrecoverableBy Timestamp `json:"keysUnrecoverableBy"`
	BackupsExpireBy     Timestamp `json:"backupsExpireBy"`
}

// deleteUser is the single deletion path: destroy the DEK (tombstone),
// forget the login throttle of the account's username and delete the
// users row, so every owned row in every range goes by ON DELETE CASCADE,
// in the caller's transaction. The credentials row is locked before the
// throttle row, the order every sign-in path uses (throttle.go).
func (s *Store) deleteUser(ctx context.Context, tx pgx.Tx, id UserID) (bool, error) {
	if err := s.vault.Destroy(ctx, tx, id); err != nil {
		return false, err
	}
	var username string
	err := tx.QueryRow(ctx, `SELECT username FROM credentials WHERE user_id = $1 FOR UPDATE`, id).Scan(&username)
	switch {
	case err == nil:
		if err := clearThrottle(ctx, tx, s.UsernameKey(username)); err != nil {
			return false, err
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// requireRecentProof enforces the reauthentication window for accounts;
// a guest's cookie is its only credential and proof.
func (s *Store) requireRecentProof(ctx context.Context, q db.DBTX, owner UserID, current SessionID) error {
	var kind string
	var at time.Time
	err := q.QueryRow(ctx, `SELECT s.kind, s.authenticated_at FROM sessions s WHERE s.id = $1 AND s.user_id = $2 AND s.revoked_at IS NULL`,
		current, owner).Scan(&kind, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotSignedIn
	}
	if err != nil {
		return err
	}
	if kind == KindGuest {
		return nil
	}
	if s.clock().Sub(at) > s.cfg.ReauthWindow {
		return ErrReauthRequired
	}
	return nil
}

// DeleteAccount deletes the owner and everything they own, crypto-shreds
// their key and drops its cached copy. Accounts need a recent password
// proof. The credentials row (none for a guest) is locked before the proof
// is checked: a password change or a recovery revokes every session of
// the user under that lock, so it either commits before the check, which
// then refuses, or waits until the deletion has committed; checked without
// the lock, a change that committed between the check and the delete left
// a deletion by a session it had just revoked.
func (s *Store) DeleteAccount(ctx context.Context, owner UserID, current SessionID) (DeletionReceipt, error) {
	now := s.clock()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM credentials WHERE user_id = $1 FOR UPDATE`, owner); err != nil {
			return err
		}
		if err := s.requireRecentProof(ctx, tx, owner, current); err != nil {
			return err
		}
		ok, err := s.deleteUser(ctx, tx, owner)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	})
	s.vault.Evict(owner)
	if err != nil {
		return DeletionReceipt{}, err
	}
	return DeletionReceipt{Deleted: true, KeysUnrecoverableBy: ts(now.Add(KeyBackupRetention)), BackupsExpireBy: ts(now.Add(ContentBackupRetention))}, nil
}

// PurgeGuests deletes, under the job advisory lock, up to PurgeBatch
// guests inactive beyond the inactive TTL or past their maximum age,
// through the deletion path. It returns db.ErrJobLocked when another
// instance is running it. Every statement runs on the lock's connection,
// so the job needs no other one (db.WithJobLock).
func (s *Store) PurgeGuests(ctx context.Context) (int, error) {
	n := 0
	err := db.WithJobLock(ctx, s.pool, JobGuestPurge, func(ctx context.Context, conn db.JobConn) error {
		now := s.clock()
		rows, err := conn.Query(ctx, `SELECT id FROM users WHERE kind = 'guest' AND (last_active_at < $1 OR guest_expires_at < $2)
			ORDER BY last_active_at LIMIT $3`, now.Add(-s.cfg.GuestInactiveTTL), now, PurgeBatch)
		if err != nil {
			return err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[UserID])
		if err != nil {
			return err
		}
		for _, id := range ids {
			var deleted bool
			err := s.txOn(ctx, conn, func(tx pgx.Tx) error {
				// Re-check under the row lock: activity since the scan keeps the guest.
				var still UserID
				err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 AND kind = 'guest'
					AND (last_active_at < $2 OR guest_expires_at < $3) FOR UPDATE`, id, now.Add(-s.cfg.GuestInactiveTTL), now).Scan(&still)
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
				if err != nil {
					return err
				}
				deleted, err = s.deleteUser(ctx, tx, id)
				return err
			})
			s.vault.Evict(id)
			if err != nil {
				return err
			}
			if deleted {
				n++
			}
		}
		return nil
	})
	if n > 0 || err == nil {
		s.log.Info("account.guest_purge", "purged", n)
	}
	return n, err
}

// RetentionReport counts what the retention job deleted.
type RetentionReport struct {
	Sessions   int
	Tombstones int
	// Throttle counts login_throttle rows quiet for ThrottleQuiet.
	Throttle int
}

// Retention deletes ended sessions older than EndedSessionRetention, key
// tombstones older than TombstoneRetention and login throttle rows without
// a failure for ThrottleQuiet, under the job lock, on the lock's connection.
func (s *Store) Retention(ctx context.Context) (RetentionReport, error) {
	var rep RetentionReport
	err := db.WithJobLock(ctx, s.pool, JobRetention, func(ctx context.Context, conn db.JobConn) error {
		cut := s.clock().Add(-EndedSessionRetention)
		tag, err := conn.Exec(ctx, `DELETE FROM sessions WHERE revoked_at < $1 OR absolute_expires_at < $1
			OR last_seen_at + make_interval(secs => idle_timeout_seconds) < $1`, cut)
		if err != nil {
			return err
		}
		rep.Sessions = int(tag.RowsAffected())
		tag, err = conn.Exec(ctx, `DELETE FROM user_data_keys WHERE destroyed_at < $1`, s.clock().Add(-TombstoneRetention))
		if err != nil {
			return err
		}
		rep.Tombstones = int(tag.RowsAffected())
		tag, err = conn.Exec(ctx, `DELETE FROM login_throttle WHERE last_failed_at < $1`, s.clock().Add(-ThrottleQuiet))
		if err != nil {
			return err
		}
		rep.Throttle = int(tag.RowsAffected())
		return nil
	})
	if err == nil {
		s.log.Info("account.retention", "sessions", rep.Sessions, "tombstones", rep.Tombstones, "throttle", rep.Throttle)
	}
	return rep, err
}

// ReconcileReport counts what a restore reconcile found and, unless it was
// a dry run, changed.
type ReconcileReport struct {
	// DestroyedKey and MissingKey count users of the restored content dump
	// whose key row is destroyed (deleted after the dump was taken) or
	// missing from the restored key table.
	DestroyedKey int
	MissingKey   int
	// OrphanKeys counts live key rows that belong to no user: accounts and
	// guests created after the content dump, whose key came back with the
	// newer key dump (the operator canary's key is not counted).
	OrphanKeys int
	// LiveSessions counts sessions without a revocation in the restored
	// database: every one that was live when the content dump was taken,
	// including those revoked since (logout, password change, recovery).
	LiveSessions int
	// Deleted, KeysDestroyed and SessionsRevoked are what a run changed.
	Deleted         int
	KeysDestroyed   int
	SessionsRevoked int
}

// ReconcileRestore brings a database restored from a content dump and the
// newest key dump back in line with what happened after the content dump,
// as far as the key dump shows it. Unless dryRun, it:
//
//  1. deletes, through the deletion path, every user whose key row is
//     destroyed (deleted after the content dump) or missing;
//  2. destroys (tombstones) every live key row without a users row, so the
//     ciphertext of an account created after the content dump, which no
//     longer exists anywhere, cannot be opened from any copy (the canary's
//     key is kept);
//  3. revokes every session with the reason 'restored', because the dump
//     cannot tell which of them were revoked after it was taken: everyone
//     signs in again.
//
// What it cannot undo (RUNBOOK §8): a deletion after the newest key dump
// brings that account back, and password changes, recoveries and code
// regenerations made after the content dump are lost (the old password and
// the old codes work again), so users are told to repeat them. The dry run
// shows the counts first, so losing the key table itself (a large missing
// count) or the content dump (a large orphan count) is noticed before
// anything changes.
func (s *Store) ReconcileRestore(ctx context.Context, dryRun bool) (ReconcileReport, error) {
	var rep ReconcileReport
	rows, err := s.pool.Query(ctx, `SELECT u.id, k.user_id IS NULL FROM users u LEFT JOIN user_data_keys k ON k.user_id = u.id
		WHERE k.user_id IS NULL OR k.destroyed_at IS NOT NULL ORDER BY u.id`)
	if err != nil {
		return rep, err
	}
	type row struct {
		id      UserID
		missing bool
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		err := r.Scan(&x.id, &x.missing)
		return x, err
	})
	if err != nil {
		return rep, err
	}
	for _, x := range list {
		if x.missing {
			rep.MissingKey++
		} else {
			rep.DestroyedKey++
		}
	}
	orphanSQL, err := s.orphanKeysSQL(ctx)
	if err != nil {
		return rep, err
	}
	rows, err = s.pool.Query(ctx, `SELECT k.user_id FROM user_data_keys k WHERE `+orphanSQL+` ORDER BY k.user_id`)
	if err != nil {
		return rep, err
	}
	orphans, err := pgx.CollectRows(rows, pgx.RowTo[UserID])
	if err != nil {
		return rep, err
	}
	rep.OrphanKeys = len(orphans)
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&rep.LiveSessions); err != nil {
		return rep, err
	}
	if dryRun {
		return rep, nil
	}
	for _, x := range list {
		var ok bool
		err := s.tx(ctx, func(tx pgx.Tx) error {
			var err error
			ok, err = s.deleteUser(ctx, tx, x.id)
			return err
		})
		s.vault.Evict(x.id)
		if err != nil {
			return rep, err
		}
		if ok {
			rep.Deleted++
		}
	}
	for _, id := range orphans {
		destroyed := false
		err := s.tx(ctx, func(tx pgx.Tx) error {
			destroyed = false // the body may run again (Store.tx)
			// Re-check under the row lock: only a key that is still live and
			// still belongs to no user is destroyed.
			var still UserID
			err := tx.QueryRow(ctx, `SELECT k.user_id FROM user_data_keys k WHERE k.user_id = $1 AND `+orphanSQL+` FOR UPDATE`, id).Scan(&still)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			destroyed = true
			return s.vault.Destroy(ctx, tx, id)
		})
		s.vault.Evict(id)
		if err != nil {
			return rep, err
		}
		if destroyed {
			rep.KeysDestroyed++
		}
	}
	tag, err := s.pool.Exec(ctx, `UPDATE sessions SET revoked_at = $1, revoke_reason = 'restored' WHERE revoked_at IS NULL`, s.clock())
	if err != nil {
		return rep, err
	}
	rep.SessionsRevoked = int(tag.RowsAffected())
	s.log.Info("account.restore_reconcile", "destroyed_key", rep.DestroyedKey, "missing_key", rep.MissingKey, "deleted", rep.Deleted,
		"orphan_keys", rep.OrphanKeys, "keys_destroyed", rep.KeysDestroyed, "sessions_revoked", rep.SessionsRevoked)
	return rep, nil
}

// orphanKeysSQL is the condition (on user_data_keys k) of a live key row
// without a users row, excluding the operator canary's key when the
// canary table exists (migration 0900).
func (s *Store) orphanKeysSQL(ctx context.Context) (string, error) {
	cond := `k.destroyed_at IS NULL AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = k.user_id)`
	var canary bool
	if err := s.pool.QueryRow(ctx, `SELECT to_regclass('ops_canary') IS NOT NULL`).Scan(&canary); err != nil {
		return "", err
	}
	if canary {
		cond += ` AND NOT EXISTS (SELECT 1 FROM ops_canary c WHERE c.key_id = k.user_id)`
	}
	return cond, nil
}

package account

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/db"
)

// Login backoff (ADR-0002 §9). One table, login_throttle, holds the
// failure count of every valid username's key (the keyed UsernameKey),
// whether or not an account exists, so a known and an unknown username get
// the same answers, the same Retry-After and the same database work,
// before and after a restart. Input that is not a valid username is never
// written (Login). credentials.failed_count is kept only for the
// 100-attempt disable.
//
// Every password proof is reserved before its argon2id work, in one
// statement (reserveSQL), and counted as a failure until it succeeds: a
// gate that only read the counts let concurrent attempts pass the same
// read, so a burst of guesses ran inside one backoff step, and an attempt
// that passed before the 100th failure was recorded still signed in
// afterwards (review round 3, R3-3). A success undoes the count in its own
// transaction, after checking under the credentials row lock that
// password sign-in is still enabled and, for a sign-in and a code
// regeneration, that the hash it proved is still the stored one
// (lockProof; PR #9 review, F1).
//
// Lock order (review round 3, R3-2): every statement and transaction that
// touches both locks the account's credentials row before the username
// key's login_throttle row, and the credentials row before the user's
// session rows; deletion and recovery included. db.Retry is a backstop
// for a deadlock that would still occur.

// ThrottleQuiet is the observation window: how long a username key keeps
// its failure count without a new failure. After it the count starts
// again at one and the retention job deletes the row. It is longer than
// the longest wait (15 min), so the decay never shortens a backoff, and
// short enough that a few typos by an account's owner stop showing as a
// backoff within the hour. Total guessing per account stays bounded by
// the per-username rate limit and by credentials.failed_count, which does
// not decay and disables password sign-in at DisableAfter.
const ThrottleQuiet = time.Hour

// Backoff schedule: from the backoffFrom-th consecutive failure on, each
// further attempt waits min(2^(n-backoffFrom) s, backoffMax).
const (
	backoffFrom = 5
	backoffMax  = 15 * time.Minute
)

// BackoffDelay is the wait after n consecutive failures:
// min(2^(n-5) s, 15 min) from the fifth failure on, else 0.
func BackoffDelay(n int) time.Duration {
	if n < backoffFrom {
		return 0
	}
	if n-backoffFrom >= 30 {
		return backoffMax
	}
	return min(time.Duration(1<<(n-backoffFrom))*time.Second, backoffMax)
}

// backoffSQL is BackoffDelay of the integer expression col, as an SQL
// interval (TestBackoffSQLMatchesBackoffDelay compares the two).
func backoffSQL(col string) string {
	return fmt.Sprintf(`(CASE WHEN %[1]s < %[2]d THEN interval '0' ELSE make_interval(secs => least(power(2, least(%[1]s - %[2]d, 30)), %[3]d)) END)`,
		col, backoffFrom, int(backoffMax/time.Second))
}

// DisableAfter is the consecutive-attempt ceiling after which password
// sign-in is disabled until recovery (NIST SP 800-63B-4: at most 100
// consecutive failed attempts). An attempt counts from its reservation, so
// at most DisableAfter proofs are ever verified after the last success.
const DisableAfter = 100

// attemptTimeout bounds how long a reserved attempt can be in flight
// (hashing wait, argon2id, the database). At the ceiling a further attempt
// waits for the one in flight; one that never reported back (a crash) is
// treated as failed after this long.
const attemptTimeout = time.Minute

// reserveSQL reserves one password proof for a username key. In order:
// it locks the account's credentials row (none for an unknown username);
// when that account is neither disabled nor at the ceiling, it upserts the
// username key's throttle row unless a backoff applies (the count starts
// again after ThrottleQuiet); when that reserved the attempt, it adds one
// to credentials.failed_count; otherwise it locks the throttle row and
// reads its latest state for the Retry-After. Known and unknown usernames
// send this same statement; the CTEs are chained (each reads the one
// before) so the locks are always taken credentials first.
//
// $1 key, $2 now, $3 quiet cut-off (now - ThrottleQuiet), $4 normalised
// username, $5 DisableAfter.
var reserveSQL = `WITH cred AS (
	SELECT ` + credentialColumns + `, last_failed_at
	FROM credentials WHERE username = $4::text FOR NO KEY UPDATE
), open AS (
	SELECT NOT EXISTS (SELECT 1 FROM cred WHERE password_login_disabled_at IS NOT NULL OR failed_count >= $5::integer) AS ok
), throttle AS (
	INSERT INTO login_throttle AS t (key_hash, failures, last_failed_at)
	SELECT $1::bytea, 1, $2::timestamptz FROM open WHERE open.ok
	ON CONFLICT (key_hash) DO UPDATE SET
	  failures = CASE WHEN t.last_failed_at < $3::timestamptz THEN 1 ELSE t.failures + 1 END,
	  last_failed_at = EXCLUDED.last_failed_at
	WHERE t.last_failed_at < $3::timestamptz OR t.last_failed_at + ` + backoffSQL("t.failures") + ` <= $2::timestamptz
	RETURNING failures
), account AS (
	UPDATE credentials SET failed_count = failed_count + 1, last_failed_at = $2::timestamptz
	WHERE username = $4::text AND EXISTS (SELECT 1 FROM throttle)
	RETURNING failed_count
), cur AS (
	SELECT failures, last_failed_at FROM login_throttle
	WHERE key_hash = $1::bytea AND NOT EXISTS (SELECT 1 FROM throttle) FOR UPDATE
)
SELECT EXISTS (SELECT 1 FROM throttle) AS reserved, (SELECT count(*) FROM account) AS counted,
  (SELECT last_failed_at + ` + backoffSQL("failures") + ` FROM cur WHERE last_failed_at >= $3::timestamptz) AS until,
  c.*
FROM (SELECT 1) AS one LEFT JOIN cred c ON true`

// reserveRetries bounds the re-runs of reserveSQL when its snapshot did not
// see a throttle row written concurrently (neither reserved nor a wait).
const reserveRetries = 3

// reserve counts one password proof for the username key, and for the
// account when one has the username, before the argon2id work, and returns
// the account's credentials (nil for an unknown username). It refuses with
// rate_limited and the remaining wait while a backoff applies, with
// password_login_disabled for a disabled account, and with a short
// rate_limited while DisableAfter attempts since the last success are
// already counted and one of them is still in flight.
func (s *Store) reserve(ctx context.Context, key [32]byte, username string, sid *SessionID) (*credential, error) {
	for range reserveRetries {
		var (
			now      time.Time
			reserved bool
			counted  int
			until    *time.Time
			c        *credential
		)
		err := db.Retry(ctx, s.onDeadlock, func() error {
			var err error
			now = s.clock()
			c, err = scanReservation(s.pool.QueryRow(ctx, reserveSQL, key[:], now, now.Add(-ThrottleQuiet), username, DisableAfter),
				&reserved, &counted, &until)
			return err
		})
		if err != nil {
			return nil, err
		}
		switch {
		case reserved:
			return c, nil
		case until != nil && until.After(now):
			return nil, throttled(until.Sub(now))
		case c != nil && c.disabled != nil:
			return nil, ErrPasswordLoginDisabled
		case c != nil && c.failed >= DisableAfter:
			if c.lastFailed != nil && now.Sub(*c.lastFailed) >= attemptTimeout {
				// The attempt that reached the ceiling never reported back:
				// count it as failed, which disables password sign-in.
				if err := s.recordFailure(ctx, username, sid); err != nil {
					return nil, err
				}
				return nil, ErrPasswordLoginDisabled
			}
			return nil, throttled(time.Second)
		}
		// Neither reserved nor refused: the throttle row was written after
		// this statement's snapshot was taken. Run it again.
	}
	return nil, throttled(time.Second)
}

// scanReservation reads reserveSQL's row: the decision and the credential
// columns, all NULL for an unknown username.
func scanReservation(row pgx.Row, reserved *bool, counted *int, until **time.Time) (*credential, error) {
	var (
		user               pgtype.UUID
		username           *string
		tag, salt          []byte
		mem, iter, failed  *int32
		par                *int16
		disabled, ack, gen *time.Time
		lastFailed         *time.Time
	)
	if err := row.Scan(reserved, counted, until, &user, &username, &tag, &salt, &mem, &iter, &par, &failed, &disabled, &ack, &gen,
		&lastFailed); err != nil {
		return nil, err
	}
	if !user.Valid || username == nil || mem == nil || iter == nil || par == nil || failed == nil || gen == nil {
		return nil, nil
	}
	c := &credential{user: UserID(user.Bytes), username: *username, failed: int(*failed), disabled: disabled, ackAt: ack,
		generated: *gen, lastFailed: lastFailed}
	c.hash = PasswordHash{Tag: tag, Salt: salt, Params: HashParams{MemoryKiB: uint32(*mem), Iterations: uint32(*iter), Parallelism: uint8(*par)}}
	return c, nil
}

// Audit events written by recordFailureSQL; checked against the
// allow-lists in TestFailureAuditEventsAreAllowListed.
const (
	auditLoginFailed     = "auth.login_failed"
	auditLoginDisabled   = "password_login.disabled"
	auditFailureCountKey = "count"
)

// recordFailureSQL is one statement for every failed password proof,
// known username or not (the attempt is already counted by reserveSQL):
// when the username belongs to an account that reached DisableAfter
// counted attempts, it disables password sign-in and writes
// password_login.disabled, and it writes auth.login_failed (at most one per
// account per hour). For an unknown username nothing matches. The audit
// inserts read the disable update first, so the credentials row is locked
// before the users and sessions rows their foreign keys share-lock.
//
// $1 normalised username, $2 now, $3 DisableAfter, $4/$5 event ids, $6
// session id or NULL, $7 coalescing cut-off (now - 1 h).
const recordFailureSQL = `WITH account AS (
	SELECT user_id, failed_count FROM credentials WHERE username = $1::text
), disabled AS (
	UPDATE credentials SET password_login_disabled_at = $2::timestamptz
	WHERE username = $1::text AND password_login_disabled_at IS NULL AND failed_count >= $3::integer
	RETURNING user_id, failed_count
), failed_event AS (
	INSERT INTO audit_events (id, user_id, at, type, actor, session_id, detail)
	SELECT $4::uuid, a.user_id, $2::timestamptz, '` + auditLoginFailed + `', '` + actorUser + `', $6::uuid,
	  jsonb_build_object('` + auditFailureCountKey + `', a.failed_count)
	FROM account a
	WHERE (SELECT count(*) FROM disabled) >= 0
	  AND NOT EXISTS (SELECT 1 FROM audit_events e WHERE e.user_id = a.user_id AND e.type = '` + auditLoginFailed + `'
	  AND e.at > $7::timestamptz)
), disabled_event AS (
	INSERT INTO audit_events (id, user_id, at, type, actor, session_id, detail)
	SELECT $5::uuid, d.user_id, $2::timestamptz, '` + auditLoginDisabled + `', '` + actorSystem + `', $6::uuid,
	  jsonb_build_object('` + auditFailureCountKey + `', d.failed_count)
	FROM disabled d
)
SELECT count(*) FROM account`

// recordFailure finishes a failed password proof (see recordFailureSQL).
// It runs even when the request was cancelled after the verification, so
// the disable cannot be skipped by hanging up.
func (s *Store) recordFailure(ctx context.Context, username string, sid *SessionID) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var session any
	if sid != nil {
		session = *sid
	}
	return db.Retry(ctx, s.onDeadlock, func() error {
		now := s.clock()
		var n int
		return s.pool.QueryRow(ctx, recordFailureSQL, username, now, DisableAfter, newID(), newID(), session, now.Add(-time.Hour)).Scan(&n)
	})
}

// releaseSQL gives back a reservation whose proof was never verified (the
// hashing slots stayed busy, or the request ended while waiting for one):
// one attempt less for the account and for the username key, credentials
// first. Known and unknown usernames send the same statement.
const releaseSQL = `WITH account AS (
	UPDATE credentials SET failed_count = failed_count - 1
	WHERE username = $2::text AND failed_count > 0 AND password_login_disabled_at IS NULL
	RETURNING 1
), lowered AS (
	UPDATE login_throttle SET failures = failures - 1
	WHERE key_hash = $1::bytea AND failures > 1 AND (SELECT count(*) FROM account) >= 0
	RETURNING 1
), dropped AS (
	DELETE FROM login_throttle WHERE key_hash = $1::bytea AND failures = 1 AND (SELECT count(*) FROM account) >= 0
	RETURNING 1
)
SELECT (SELECT count(*) FROM lowered) + (SELECT count(*) FROM dropped)`

// release undoes reserve after a verification that did not run.
func (s *Store) release(ctx context.Context, key [32]byte, username string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := db.Retry(ctx, s.onDeadlock, func() error {
		var n int
		return s.pool.QueryRow(ctx, releaseSQL, key[:], username).Scan(&n)
	})
	if err != nil {
		s.log.Warn("account.release_failed", "class", db.Classify(err))
	}
}

// clearThrottle forgets a username key's failures after a successful
// password or recovery-code proof, in the caller's transaction, after the
// credentials row is locked.
func clearThrottle(ctx context.Context, q db.DBTX, key [32]byte) error {
	_, err := q.Exec(ctx, `DELETE FROM login_throttle WHERE key_hash = $1`, key[:])
	return err
}

// resetFailures ends a successful proof in the caller's transaction: it
// clears the account's attempt count under the credentials row lock,
// refusing when password sign-in was disabled meanwhile (an attempt
// counted before this one failed as the DisableAfter-th), and then clears
// the username key's throttle.
func (s *Store) resetFailures(ctx context.Context, q db.DBTX, owner UserID, username string) error {
	tag, err := q.Exec(ctx, `UPDATE credentials SET failed_count = 0, last_failed_at = NULL
		WHERE user_id = $1 AND password_login_disabled_at IS NULL`, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrPasswordLoginDisabled
	}
	return clearThrottle(ctx, q, s.UsernameKey(username))
}

// errStaleProof means a password proof was verified against a hash that
// the account no longer has (lockProof).
var errStaleProof = errors.New("account: the proven password is no longer current")

// lockProof is the first statement of a password proof's success
// transaction: it locks the account's credentials row (credentials first,
// as above) and checks that the row still holds the hash the proof was
// verified against (c.hash, read by reserveSQL, so its tag and salt). The
// argon2id work runs outside any lock; a password change or a recovery
// that commits meanwhile replaces the hash, and the proof is then for a
// password that is no longer current: errStaleProof, and the caller
// commits nothing (no session, no re-hash, no reset) and ends the attempt
// as a failed one (recordFailure; its reservation is not undone), so the
// old password neither signs in past the change's session revocation nor
// overwrites the new hash. A deleted account is stale too. With the hash
// current, it refuses with ErrPasswordLoginDisabled when password sign-in
// was disabled meanwhile.
//
// Residual (ADR-0002 §9): the check compares hashes, not passwords, so a
// re-hash looks like a replacement. A proof of the right, current
// password that overlaps a sign-in which re-hashes the stored hash (after
// a parameter change, or while instances run with different parameters)
// is refused once, whether or not it would re-hash itself: a sign-in gets
// ErrInvalidCredentials and a code regeneration
// ErrCurrentPasswordIncorrect. Each writes the coalesced auth.login_failed
// (recordFailure) although the password was right; no counter is raised,
// because the re-hashing sign-in reset them after this proof's
// reservation. Sent again, the proof succeeds
// (TestPR9RehashOverlapRefusesCurrentProofOnce). Telling the two apart
// would need a password version: password_changed_at is a time, which a
// fixed clock (or a change within the same microsecond) leaves equal.
func lockProof(ctx context.Context, q db.DBTX, c *credential) error {
	var tag, salt []byte
	var disabled bool
	err := q.QueryRow(ctx, `SELECT password_hash, password_salt, password_login_disabled_at IS NOT NULL FROM credentials
		WHERE user_id = $1 FOR UPDATE`, c.user).Scan(&tag, &salt, &disabled)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errStaleProof
	case err != nil:
		return err
	case !bytes.Equal(tag, c.hash.Tag) || !bytes.Equal(salt, c.hash.Salt):
		return errStaleProof
	case disabled:
		return ErrPasswordLoginDisabled
	}
	return nil
}

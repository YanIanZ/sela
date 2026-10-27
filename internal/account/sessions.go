package account

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// touchEvery bounds how often last_seen_at and users.last_active_at are
// written for one session.
const touchEvery = time.Minute

// Authenticate resolves a session cookie to its principal. A malformed,
// unknown, revoked, idle-expired or absolutely expired token is
// ErrNotSignedIn. Activity is recorded at most once a minute.
func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	hash, ok := HashSessionToken(token)
	if !ok {
		return Principal{}, ErrNotSignedIn
	}
	var p Principal
	var lastSeen time.Time
	var idle int
	var revoked *time.Time
	err := s.pool.QueryRow(ctx, `SELECT id, user_id, kind, trusted_device, last_seen_at, idle_timeout_seconds, absolute_expires_at,
		authenticated_at, revoked_at FROM sessions WHERE token_hash = $1`, hash).Scan(
		&p.SessionID, &p.UserID, &p.Kind, &p.TrustedDevice, &lastSeen, &idle, &p.AbsoluteExpiresAt, &p.AuthenticatedAt, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNotSignedIn
	}
	if err != nil {
		return Principal{}, err
	}
	now := s.clock()
	idleFor := time.Duration(idle) * time.Second
	if revoked != nil || !now.Before(p.AbsoluteExpiresAt) || !now.Before(lastSeen.Add(idleFor)) {
		return Principal{}, ErrNotSignedIn
	}
	if now.Sub(lastSeen) >= touchEvery {
		threshold := now.Add(-touchEvery)
		if _, err := s.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = $2 WHERE id = $1 AND last_seen_at <= $3`, p.SessionID, now, threshold); err != nil {
			return Principal{}, err
		}
		if _, err := s.pool.Exec(ctx, `UPDATE users SET last_active_at = $2 WHERE id = $1 AND last_active_at <= $3`, p.UserID, now, threshold); err != nil {
			return Principal{}, err
		}
		lastSeen = now
	}
	p.IdleExpiresAt = lastSeen.Add(idleFor)
	if p.IdleExpiresAt.After(p.AbsoluteExpiresAt) {
		p.IdleExpiresAt = p.AbsoluteExpiresAt
	}
	return p, nil
}

// RevokeSession revokes one of the owner's sessions. Another user's or an
// unknown id is ErrNotFound and changes nothing; an own session that has
// already ended is fine (idempotent). It reports whether the revoked
// session is the current one.
func (s *Store) RevokeSession(ctx context.Context, owner UserID, current SessionID, id SessionID) (bool, error) {
	err := s.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $3, revoke_reason = 'revoked_by_user'
			WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, owner, s.clock())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sessions WHERE id = $1 AND user_id = $2)`, id, owner).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
			return nil
		}
		return s.audit(ctx, tx, owner, &current, "session.revoked", actorUser, map[string]any{"reason": "revoked_by_user"})
	})
	return id == current, err
}

// Logout revokes the current session.
func (s *Store) Logout(ctx context.Context, owner UserID, current SessionID) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $3, revoke_reason = 'logout' WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
			current, owner, s.clock())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return s.audit(ctx, tx, owner, &current, "auth.logout", actorUser, nil)
	})
}

// LogoutAll revokes every live session of the owner, or every other one
// when keepCurrent is set, and reports how many it revoked.
func (s *Store) LogoutAll(ctx context.Context, owner UserID, current SessionID, keepCurrent bool) (int, error) {
	n := 0
	err := s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $3, revoke_reason = 'logout_all'
			WHERE user_id = $1 AND revoked_at IS NULL AND NOT ($4 AND id = $2)
			  AND absolute_expires_at > $3 AND last_seen_at + make_interval(secs => idle_timeout_seconds) > $3`, owner, current, now, keepCurrent)
		if err != nil {
			return err
		}
		n = int(tag.RowsAffected())
		return s.audit(ctx, tx, owner, &current, "auth.logout_all", actorUser, map[string]any{"revoked": n})
	})
	return n, err
}

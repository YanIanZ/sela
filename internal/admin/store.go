package admin

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/account"
)

// Options configures the operator store.
type Options struct {
	Hasher *account.Hasher
	// MFAKey is SELA_ADMIN_MFA_KEY: exactly 32 bytes.
	MFAKey []byte
	Now    func() time.Time
	// Idle and Absolute bound staff sessions.
	Idle     time.Duration
	Absolute time.Duration
}

// Store owns staff identities, sessions and the immutable audit.
type Store struct {
	pool     *pgxpool.Pool
	hasher   *account.Hasher
	mfaKey   []byte
	now      func() time.Time
	idle     time.Duration
	absolute time.Duration
}

// New validates the operator configuration.
func New(pool *pgxpool.Pool, o Options) (*Store, error) {
	if pool == nil {
		return nil, errors.New("admin: nil pool")
	}
	if o.Hasher == nil {
		return nil, errors.New("admin: nil hasher")
	}
	if len(o.MFAKey) != 32 {
		return nil, errors.New("admin: SELA_ADMIN_MFA_KEY must be 32 bytes")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Idle <= 0 {
		o.Idle = 30 * time.Minute
	}
	if o.Absolute <= 0 {
		o.Absolute = 8 * time.Hour
	}
	if o.Absolute < o.Idle {
		o.Absolute = o.Idle
	}
	return &Store{pool: pool, hasher: o.Hasher, mfaKey: append([]byte(nil), o.MFAKey...), now: o.Now, idle: o.Idle, absolute: o.Absolute}, nil
}

// Session is the token plus the authenticated principal.
type Session struct {
	Token     string
	Principal Principal
}

// Create provisions a staff identity out of band and returns the TOTP
// secret exactly once. It never stores the plaintext secret.
func (s *Store) Create(ctx context.Context, username, password string, role Role, orgID string) (Principal, string, error) {
	if err := ValidateUsername(username); err != nil {
		return Principal{}, "", err
	}
	if !ValidRole(role) {
		return Principal{}, "", ErrInvalid
	}
	if len(orgID) > 80 {
		return Principal{}, "", ErrInvalid
	}
	normalized, err := account.CheckPassword(password, username, "password")
	if err != nil {
		return Principal{}, "", ErrInvalid
	}
	hash, err := s.hasher.Hash(ctx, normalized)
	if err != nil {
		return Principal{}, "", err
	}
	secretText, err := NewTOTPSecret()
	if err != nil {
		return Principal{}, "", err
	}
	secret, err := DecodeTOTPSecret(secretText)
	if err != nil {
		return Principal{}, "", err
	}
	id := newAdminID()
	sealed, err := sealMFA(s.mfaKey, id, secret)
	if err != nil {
		return Principal{}, "", err
	}
	now := s.now()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO admin_users
			(id, username, password_hash, password_salt, argon2_version, argon2_memory_kib, argon2_iterations, argon2_parallelism,
			 mfa_secret_ct, role, org_id, status, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),'active',$12)`,
			id, username, hash.Tag, hash.Salt, 19, hash.Params.MemoryKiB, hash.Params.Iterations, hash.Params.Parallelism,
			sealed, role, orgID, now)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, nil, "admin.create", "admin_user", username, "", "active", "provisioned out of band")
	})
	if err != nil {
		return Principal{}, "", err
	}
	return Principal{AdminID: id, Username: username, Role: role, OrgID: orgID}, secretText, nil
}

// Authenticate verifies username, password and TOTP and creates a session.
func (s *Store) Authenticate(ctx context.Context, username, password, code string) (Session, error) {
	var (
		id        AdminID
		hash      account.PasswordHash
		sealed    []byte
		role      Role
		orgID     string
		status    string
		failed    int
		next      *time.Time
		lastStep  int64
		lastLogin *time.Time
	)
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash, password_salt, argon2_memory_kib, argon2_iterations, argon2_parallelism,
		mfa_secret_ct, role, COALESCE(org_id,''), status, failed_count, next_attempt_at, last_totp_step, last_login_at
		FROM admin_users WHERE username = $1`, username).Scan(
		&id, &hash.Tag, &hash.Salt, &hash.Params.MemoryKiB, &hash.Params.Iterations, &hash.Params.Parallelism,
		&sealed, &role, &orgID, &status, &failed, &next, &lastStep, &lastLogin)
	if errors.Is(err, pgx.ErrNoRows) {
		_, _ = s.hasher.VerifyUnknown(ctx, password, HashUsername(username))
		_ = s.Audit(ctx, nil, "admin.login_failed", "admin_user", username, "", "", "unknown")
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	if status != "active" {
		_, _ = s.hasher.Verify(ctx, password, &hash)
		return Session{}, ErrInvalidCredentials
	}
	if next != nil && now.Before(*next) {
		return Session{}, ErrThrottled
	}
	ok, err := s.hasher.Verify(ctx, password, &hash)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		s.failed(ctx, id, failed, now)
		_ = s.Audit(ctx, nil, "admin.login_failed", "admin_user", username, "", "", "password")
		return Session{}, ErrInvalidCredentials
	}
	secret, err := openMFA(s.mfaKey, id, sealed)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	step, ok := VerifyTOTP(secret, code, now, lastStep)
	if !ok {
		s.failed(ctx, id, failed, now)
		_ = s.Audit(ctx, nil, "admin.login_failed", "admin_user", username, "", "", "mfa")
		return Session{}, ErrInvalidCredentials
	}
	token, tokenHash, err := account.NewSessionToken()
	if err != nil {
		return Session{}, err
	}
	sessionID := newSessionID()
	absolute := now.Add(s.absolute)
	err = s.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE admin_users SET failed_count = 0, next_attempt_at = NULL, last_totp_step = $2, last_login_at = $3
			WHERE id = $1 AND status = 'active' AND last_totp_step < $2`, id, step, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrInvalidCredentials
		}
		_, err = tx.Exec(ctx, `INSERT INTO admin_sessions
			(id, admin_id, token_hash, created_at, last_seen_at, idle_timeout_seconds, absolute_expires_at)
			VALUES ($1,$2,$3,$4,$4,$5,$6)`, sessionID, id, tokenHash, now, int(s.idle/time.Second), absolute)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, &id, "admin.login", "admin_session", sessionID.String(), "", "active", "")
	})
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, Principal: Principal{AdminID: id, SessionID: sessionID, Username: username, Role: role, OrgID: orgID, IdleExpiresAt: now.Add(s.idle), AbsoluteExpiresAt: absolute}}, nil
}

func (s *Store) failed(ctx context.Context, id AdminID, current int, now time.Time) {
	failed := current + 1
	var next *time.Time
	if failed >= 5 {
		t := now.Add(time.Minute)
		next = &t
	}
	_, _ = s.pool.Exec(ctx, `UPDATE admin_users SET failed_count = $2, next_attempt_at = $3 WHERE id = $1`, id, failed, next)
}

// AuthenticateSession resolves the separate operator cookie. It never
// accepts a consumer session.
func (s *Store) AuthenticateSession(ctx context.Context, token string) (Principal, error) {
	hash, ok := account.HashSessionToken(token)
	if !ok {
		return Principal{}, ErrNotSignedIn
	}
	var (
		p        Principal
		lastSeen time.Time
		idle     int
		revoked  *time.Time
		status   string
	)
	err := s.pool.QueryRow(ctx, `SELECT s.id, s.admin_id, a.username, a.role, COALESCE(a.org_id,''), s.last_seen_at,
		s.idle_timeout_seconds, s.absolute_expires_at, s.revoked_at, a.status
		FROM admin_sessions s JOIN admin_users a ON a.id = s.admin_id WHERE s.token_hash = $1`, hash).Scan(
		&p.SessionID, &p.AdminID, &p.Username, &p.Role, &p.OrgID, &lastSeen, &idle, &p.AbsoluteExpiresAt, &revoked, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNotSignedIn
	}
	if err != nil {
		return Principal{}, err
	}
	now := s.now()
	if status != "active" || revoked != nil || !now.Before(p.AbsoluteExpiresAt) || !now.Before(lastSeen.Add(time.Duration(idle)*time.Second)) {
		return Principal{}, ErrNotSignedIn
	}
	if now.Sub(lastSeen) >= time.Minute {
		_, _ = s.pool.Exec(ctx, `UPDATE admin_sessions SET last_seen_at = $2 WHERE id = $1 AND revoked_at IS NULL`, p.SessionID, now)
	}
	p.IdleExpiresAt = lastSeen.Add(time.Duration(idle) * time.Second)
	return p, nil
}

// Logout revokes the current staff session and audits it.
func (s *Store) Logout(ctx context.Context, p Principal) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE admin_sessions SET revoked_at = $3, revoke_reason = 'logout'
			WHERE id = $1 AND admin_id = $2 AND revoked_at IS NULL`, p.SessionID, p.AdminID, s.now())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return s.audit(ctx, tx, &p.AdminID, "admin.logout", "admin_session", p.SessionID.String(), "active", "revoked", "")
	})
}

// Revoke disables a staff identity and every live session it owns. It is
// idempotent: revoking an already-revoked operator is not an error.
func (s *Store) Revoke(ctx context.Context, username, reason string) error {
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if len(reason) > 200 {
		reason = reason[:200]
	}
	return s.tx(ctx, func(tx pgx.Tx) error {
		var id AdminID
		var status string
		err := tx.QueryRow(ctx, `SELECT id, status FROM admin_users WHERE username = $1 FOR UPDATE`, username).Scan(&id, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalid
		}
		if err != nil {
			return err
		}
		if status == "revoked" {
			return nil
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `UPDATE admin_users SET status = 'revoked', failed_count = 0, next_attempt_at = NULL WHERE id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_sessions SET revoked_at = $2, revoke_reason = 'revoked'
			WHERE admin_id = $1 AND revoked_at IS NULL`, id, now); err != nil {
			return err
		}
		return s.audit(ctx, tx, nil, "admin.revoke", "admin_user", username, "active", "revoked", reason)
	})
}

// Overview returns only privacy-safe aggregate counts.
func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var o Overview
	now := s.now()
	o.GeneratedAt = now
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admin_users WHERE status = 'active'`).Scan(&o.ActiveStaff); err != nil {
		return o, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admin_sessions
		WHERE revoked_at IS NULL AND absolute_expires_at > $1
		  AND last_seen_at + make_interval(secs => idle_timeout_seconds) > $1`, now).Scan(&o.LiveSessions); err != nil {
		return o, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit WHERE created_at > $1`, now.Add(-24*time.Hour)).Scan(&o.AuditLast24h); err != nil {
		return o, err
	}
	return o, nil
}

// Audit appends one immutable operator event.
func (s *Store) Audit(ctx context.Context, actor *AdminID, action, kind, id, oldStatus, newStatus, reason string) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		return s.audit(ctx, tx, actor, action, kind, id, oldStatus, newStatus, reason)
	})
}

func (s *Store) audit(ctx context.Context, tx pgx.Tx, actor *AdminID, action, kind, id, oldStatus, newStatus, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO admin_audit (id, actor_id, action, object_kind, object_id, old_status, new_status, reason, created_at)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9)`,
		newAdminID(), actor, action, kind, id, oldStatus, newStatus, reason, s.now())
	return err
}

// ListAudit returns the newest operator events, limited to 200.
func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT a.id::text, COALESCE(u.username,''), a.action, COALESCE(a.object_kind,''), COALESCE(a.object_id,''),
		COALESCE(a.old_status,''), COALESCE(a.new_status,''), COALESCE(a.reason,''), a.created_at
		FROM admin_audit a LEFT JOIN admin_users u ON u.id = a.actor_id ORDER BY a.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.ObjectKind, &e.ObjectID, &e.OldStatus, &e.NewStatus, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

package account

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"github.com/YanIanZ/sela/internal/db"
)

// RegisterInput is POST /auth/register.
type RegisterInput struct {
	Username, Password, Locale, AgeBand, TermsPolicyVersion string
	RememberDevice                                          bool
	// PriorToken is a session cookie already present; it is revoked.
	PriorToken string
	UserAgent  string
}

// LoginInput is POST /auth/login.
type LoginInput struct {
	Username, Password    string
	RememberDevice        bool
	PriorToken, UserAgent string
}

// GuestInput is POST /auth/guest.
type GuestInput struct {
	Locale, AgeBand, TermsPolicyVersion, GuestNoticeVersion string
	PriorToken, UserAgent                                   string
}

// UpgradeInput is POST /auth/guest/upgrade.
type UpgradeInput struct {
	Username, Password string
	RememberDevice     bool
	UserAgent          string
}

// RecoveryInput is POST /auth/recovery.
type RecoveryInput struct {
	Username, RecoveryCode, NewPassword string
	RememberDevice                      bool
	PriorToken, UserAgent               string
}

// newSession describes a session row to create.
type newSession struct {
	user            UserID
	kind            string
	trusted         bool
	device          Device
	idle            time.Duration
	absolute        time.Time
	rotatedFrom     *SessionID
	authenticatedAt time.Time
}

func (s *Store) accountSession(user UserID, trusted bool, ua string, now time.Time, from *SessionID) newSession {
	idle, abs := s.cfg.SessionIdle, s.cfg.SessionAbsolute
	if trusted {
		idle, abs = s.cfg.TrustedIdle, s.cfg.TrustedAbsolute
	}
	return newSession{user: user, kind: KindAccount, trusted: trusted, device: DeviceFromUserAgent(ua), idle: idle,
		absolute: now.Add(abs), rotatedFrom: from, authenticatedAt: now}
}

func (s *Store) insertSession(ctx context.Context, q db.DBTX, ns newSession) (SessionView, SessionID, string, error) {
	id := newSessionID()
	tok, hash, err := NewSessionToken()
	if err != nil {
		return SessionView{}, id, "", err
	}
	now := s.clock()
	_, err = q.Exec(ctx, `INSERT INTO sessions (id, user_id, token_hash, kind, trusted_device, device_browser, device_os,
		created_at, last_seen_at, idle_timeout_seconds, absolute_expires_at, authenticated_at, rotated_from)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9, $10, $11, $12)`,
		id, ns.user, hash, ns.kind, ns.trusted, ns.device.Browser, ns.device.OS, now, int(ns.idle/time.Second), ns.absolute,
		ns.authenticatedAt, ns.rotatedFrom)
	if err != nil {
		return SessionView{}, id, "", err
	}
	return sessionView(id, true, ns.kind, ns.trusted, ns.device, now, now, ns.idle, ns.absolute), id, tok, nil
}

// cookieAge is the cookie lifetime: none for an untrusted account session,
// up to the absolute expiry for trusted-device and guest sessions.
func (s *Store) cookieAge(ns newSession) time.Duration {
	if ns.kind == KindAccount && !ns.trusted {
		return 0
	}
	return ns.absolute.Sub(s.clock())
}

// revokeToken revokes the session a cookie names (any user), so a session
// planted before sign-in can never be promoted (no fixation).
func (s *Store) revokeToken(ctx context.Context, q db.DBTX, token, reason string) error {
	if token == "" {
		return nil
	}
	hash, ok := HashSessionToken(token)
	if !ok {
		return nil
	}
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = $2, revoke_reason = $3 WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash, s.clock(), reason)
	return err
}

// newRecoveryCodes generates the codes and their user-bound hashes.
func newRecoveryCodes(user UserID) ([]string, [][]byte, error) {
	codes := make([]string, 0, RecoveryCodeCount)
	hashes := make([][]byte, 0, RecoveryCodeCount)
	for len(codes) < RecoveryCodeCount {
		c, err := GenerateRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		n, _ := NormalizeRecoveryCode(c)
		codes = append(codes, c)
		hashes = append(hashes, RecoveryCodeHash(user, n))
	}
	return codes, hashes, nil
}

func (s *Store) insertRecoveryCodes(ctx context.Context, q db.DBTX, user UserID, hashes [][]byte, now time.Time) error {
	for _, h := range hashes {
		if _, err := q.Exec(ctx, `INSERT INTO recovery_codes (user_id, code_hash, created_at) VALUES ($1, $2, $3)`, user, h, now); err != nil {
			return err
		}
	}
	return nil
}

// insertCredentials creates the credentials row, or returns
// ErrUsernameTaken. A taken username is detected with ON CONFLICT DO
// NOTHING rather than a failed INSERT: a unique violation would make
// PostgreSQL write the username into its server log (the DETAIL line of
// the error), and usernames are never logged (ADR-0002 §13).
func (s *Store) insertCredentials(ctx context.Context, q db.DBTX, user UserID, username string, ph PasswordHash, now time.Time) error {
	tag, err := q.Exec(ctx, `INSERT INTO credentials (user_id, username, password_hash, password_salt, argon2_version, argon2_memory_kib,
		argon2_iterations, argon2_parallelism, password_changed_at, recovery_generated_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $9)
		ON CONFLICT (username) DO NOTHING`, user, username, ph.Tag, ph.Salt, argon2Version,
		int32(ph.Params.MemoryKiB), int32(ph.Params.Iterations), int16(ph.Params.Parallelism), now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUsernameTaken
	}
	return nil
}

func (s *Store) recordConsent(ctx context.Context, q db.DBTX, user UserID, purpose string, granted bool, version, source string, now time.Time) (int, error) {
	var rev int
	err := q.QueryRow(ctx, `INSERT INTO consents (user_id, purpose, granted, policy_version, decided_at, revision)
		VALUES ($1, $2, $3, $4, $5, 1)
		ON CONFLICT (user_id, purpose) DO UPDATE SET granted = EXCLUDED.granted, policy_version = EXCLUDED.policy_version,
		  decided_at = EXCLUDED.decided_at, revision = consents.revision + 1
		RETURNING revision`, user, purpose, granted, version, now).Scan(&rev)
	if err != nil {
		return 0, err
	}
	decision := "granted"
	if !granted {
		decision = "withdrawn"
	}
	_, err = q.Exec(ctx, `INSERT INTO consent_events (id, user_id, purpose, decision, policy_version, source, decided_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, newID(), user, purpose, decision, version, source, now)
	return rev, err
}

func (s *Store) checkNewUser(locale, band string) error {
	if err := s.checkBand(band); err != nil {
		return err
	}
	if !validLocale(locale) {
		return invalid("locale")
	}
	return nil
}

// Register creates an account: user, credentials, DEK, ten recovery
// codes, the terms consent, a session and account.created, in one
// transaction. The codes are returned only here.
func (s *Store) Register(ctx context.Context, in RegisterInput) (AuthResult, error) {
	if err := s.checkNewUser(in.Locale, in.AgeBand); err != nil {
		return AuthResult{}, err
	}
	username, err := NormalizeUsername(in.Username)
	if err != nil {
		return AuthResult{}, err
	}
	pw, err := CheckPassword(in.Password, username, "password")
	if err != nil {
		return AuthResult{}, err
	}
	if in.TermsPolicyVersion != s.cfg.PolicyVersions[PolicyTerms] {
		return AuthResult{}, withField(ErrPolicyOutdated, "termsPolicyVersion")
	}
	ph, err := s.hasher.Hash(ctx, pw)
	if err != nil {
		return AuthResult{}, err
	}
	id := NewUserID()
	codes, hashes, err := newRecoveryCodes(id)
	if err != nil {
		return AuthResult{}, err
	}
	var res AuthResult
	err = s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		if err := s.revokeToken(ctx, tx, in.PriorToken, "rotated"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO users (id, kind, age_band, age_band_source, age_band_confirmed_at, locale, created_at, last_active_at)
			VALUES ($1, 'account', $2, 'self_declared', $3, $4, $3, $3)`, id, in.AgeBand, now, in.Locale); err != nil {
			return err
		}
		if err := s.vault.CreateUserKey(ctx, tx, id); err != nil {
			return err
		}
		if err := s.insertCredentials(ctx, tx, id, username, ph, now); err != nil {
			return err
		}
		if err := s.insertRecoveryCodes(ctx, tx, id, hashes, now); err != nil {
			return err
		}
		if _, err := s.recordConsent(ctx, tx, id, PolicyTerms, true, in.TermsPolicyVersion, "onboarding", now); err != nil {
			return err
		}
		ns := s.accountSession(id, in.RememberDevice, in.UserAgent, now, nil)
		view, ssid, tok, err := s.insertSession(ctx, tx, ns)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, id, &ssid, "account.created", actorUser, map[string]any{"kind": KindAccount, "ageBand": in.AgeBand}); err != nil {
			return err
		}
		res = AuthResult{Session: view, Token: tok, CookieMaxAge: s.cookieAge(ns), RecoveryCodes: codes, RemainingRecoveryCodes: len(codes)}
		res.Profile, err = s.profile(ctx, tx, id)
		return err
	})
	if err != nil {
		s.vault.Evict(id)
		return AuthResult{}, err
	}
	return res, nil
}

// StartGuest creates a guest: a users row without credentials, a DEK, the
// terms and guest-storage consents and a guest session whose cookie is the
// only key to the data.
func (s *Store) StartGuest(ctx context.Context, in GuestInput) (AuthResult, error) {
	if err := s.checkNewUser(in.Locale, in.AgeBand); err != nil {
		return AuthResult{}, err
	}
	if in.TermsPolicyVersion != s.cfg.PolicyVersions[PolicyTerms] {
		return AuthResult{}, withField(ErrPolicyOutdated, "termsPolicyVersion")
	}
	if in.GuestNoticeVersion != s.cfg.PolicyVersions[PolicyGuest] {
		return AuthResult{}, withField(ErrPolicyOutdated, "guestNoticeVersion")
	}
	id := NewUserID()
	var res AuthResult
	err := s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		// The cap check and the insert must act as one: under READ
		// COMMITTED, concurrent starts would all count the same number and
		// all pass. The transaction-scoped advisory lock (keyed by schema,
		// like the job locks) serialises guest starts until commit.
		if _, err := tx.Exec(ctx, guestCapLock); err != nil {
			return err
		}
		var live int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE kind = 'guest'`).Scan(&live); err != nil {
			return err
		}
		if live >= s.cfg.MaxGuests {
			return ErrServerBusy
		}
		if err := s.revokeToken(ctx, tx, in.PriorToken, "rotated"); err != nil {
			return err
		}
		expires := now.Add(s.cfg.GuestMaxAge)
		if _, err := tx.Exec(ctx, `INSERT INTO users (id, kind, age_band, age_band_source, age_band_confirmed_at, locale, created_at, last_active_at, guest_expires_at)
			VALUES ($1, 'guest', $2, 'self_declared', $3, $4, $3, $3, $5)`, id, in.AgeBand, now, in.Locale, expires); err != nil {
			return err
		}
		if err := s.vault.CreateUserKey(ctx, tx, id); err != nil {
			return err
		}
		if _, err := s.recordConsent(ctx, tx, id, PolicyTerms, true, in.TermsPolicyVersion, "onboarding", now); err != nil {
			return err
		}
		if _, err := s.recordConsent(ctx, tx, id, PolicyGuest, true, in.GuestNoticeVersion, "onboarding", now); err != nil {
			return err
		}
		ns := newSession{user: id, kind: KindGuest, device: DeviceFromUserAgent(in.UserAgent), idle: s.cfg.GuestInactiveTTL,
			absolute: expires, authenticatedAt: now}
		view, ssid, tok, err := s.insertSession(ctx, tx, ns)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, id, &ssid, "account.created", actorUser, map[string]any{"kind": KindGuest, "ageBand": in.AgeBand}); err != nil {
			return err
		}
		res = AuthResult{Session: view, Token: tok, CookieMaxAge: s.cookieAge(ns)}
		res.Profile, err = s.profile(ctx, tx, id)
		return err
	})
	if err != nil {
		s.vault.Evict(id)
		return AuthResult{}, err
	}
	return res, nil
}

// guestCapLock serialises guest starts in one schema (S3: the live-guest
// cap must hold under concurrency).
const guestCapLock = `SELECT pg_advisory_xact_lock(hashtextextended('sela.guest_cap:' || current_schema(), 0))`

type credential struct {
	user      UserID
	username  string
	hash      PasswordHash
	failed    int
	disabled  *time.Time
	ackAt     *time.Time
	generated time.Time
	// lastFailed is the time of the last counted attempt (reserveSQL only).
	lastFailed *time.Time
}

const credentialColumns = `user_id, username, password_hash, password_salt, argon2_memory_kib, argon2_iterations, argon2_parallelism,
	failed_count, password_login_disabled_at, recovery_acknowledged_at, recovery_generated_at`

func scanCredential(row pgx.Row) (*credential, error) {
	var c credential
	var mem, iter int32
	var par int16
	err := row.Scan(&c.user, &c.username, &c.hash.Tag, &c.hash.Salt, &mem, &iter, &par, &c.failed, &c.disabled, &c.ackAt, &c.generated)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.hash.Params = HashParams{MemoryKiB: uint32(mem), Iterations: uint32(iter), Parallelism: uint8(par)}
	return &c, nil
}

func (s *Store) credentialByUser(ctx context.Context, q db.DBTX, owner UserID, lock bool) (*credential, error) {
	sql := `SELECT ` + credentialColumns + ` FROM credentials WHERE user_id = $1`
	if lock {
		sql += ` FOR UPDATE`
	}
	return scanCredential(q.QueryRow(ctx, sql, owner))
}

// Login verifies a username and password. An unknown username and a wrong
// password do the same work, in the same order: one reservation statement
// (reserveSQL: the credentials lookup and lock, the throttle check and
// the attempt counted), one argon2id verification (an unknown username
// against the dummy hash its key selects from the stored parameter
// distribution, Hasher.VerifyUnknown) and one failure statement
// (recordFailureSQL); both return the same ErrInvalidCredentials, and a
// backoff answers the same rate_limited with the same Retry-After. Input
// that is not a valid username cannot name an account: it gets the same
// argon2id work and answer, but touches no table, so nothing derived from
// it (a passphrase typed into the username field, an email address) is
// stored. On success the session rotates (any cookie already present is
// revoked), counters reset (refused if password sign-in was disabled while
// the proof ran) and a hash whose parameters differ from the configured
// ones, lower or higher, is replaced.
func (s *Store) Login(ctx context.Context, in LoginInput) (AuthResult, error) {
	key, username, valid := usernameKey(s.throttleSecret, in.Username)
	pw := norm.NFKC.String(in.Password)
	if !valid {
		if _, err := s.hasher.VerifyUnknown(ctx, pw, key); err != nil {
			return AuthResult{}, err
		}
		return AuthResult{}, ErrInvalidCredentials
	}
	c, err := s.reserve(ctx, key, username, nil)
	if err != nil {
		return AuthResult{}, err
	}
	var ok bool
	if c != nil {
		ok, err = s.hasher.Verify(ctx, pw, &c.hash)
	} else {
		ok, err = s.hasher.VerifyUnknown(ctx, pw, key)
	}
	if err != nil {
		s.release(ctx, key, username)
		return AuthResult{}, err
	}
	if !ok {
		if err := s.recordFailure(ctx, username, nil); err != nil {
			return AuthResult{}, err
		}
		return AuthResult{}, ErrInvalidCredentials
	}
	var rehash *PasswordHash
	if c.hash.Params != s.hasher.Params() {
		ph, err := s.hasher.Hash(ctx, pw)
		if err != nil {
			return AuthResult{}, err
		}
		rehash = &ph
	}
	var res AuthResult
	err = s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		// Credentials first, then the throttle row (lock order, throttle.go).
		if err := s.resetFailures(ctx, tx, c.user, c.username); err != nil {
			return err
		}
		if rehash != nil {
			if _, err := tx.Exec(ctx, `UPDATE credentials SET password_hash = $2, password_salt = $3, argon2_memory_kib = $4,
				argon2_iterations = $5, argon2_parallelism = $6 WHERE user_id = $1`, c.user, rehash.Tag, rehash.Salt,
				int32(rehash.Params.MemoryKiB), int32(rehash.Params.Iterations), int16(rehash.Params.Parallelism)); err != nil {
				return err
			}
		}
		if err := s.revokeToken(ctx, tx, in.PriorToken, "rotated"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET last_active_at = $2 WHERE id = $1`, c.user, now); err != nil {
			return err
		}
		ns := s.accountSession(c.user, in.RememberDevice, in.UserAgent, now, nil)
		view, ssid, tok, err := s.insertSession(ctx, tx, ns)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, c.user, &ssid, "auth.login_succeeded", actorUser, map[string]any{"trustedDevice": in.RememberDevice}); err != nil {
			return err
		}
		res = AuthResult{Session: view, Token: tok, CookieMaxAge: s.cookieAge(ns)}
		res.Profile, err = s.profile(ctx, tx, c.user)
		return err
	})
	if err != nil {
		return AuthResult{}, err
	}
	return res, nil
}

// Recover replaces the password with one recovery code: the code is
// consumed (single use), sign-in is re-enabled and counters cleared, every
// session is revoked and a new one issued. Every failure is the same
// ErrRecoveryFailed, and every attempt costs one argon2id hash. An unknown
// username and a known one with a wrong or used code send the same
// statements (begin, the credentials lock, the code update, rollback): for
// an unknown username the update runs against a random user id and
// matches nothing.
func (s *Store) Recover(ctx context.Context, in RecoveryInput) (AuthResult, error) {
	username, uerr := NormalizeUsername(in.Username)
	pwUser := username
	if uerr != nil {
		pwUser = strings.ToLower(strings.TrimSpace(in.Username))
	}
	pw, err := CheckPassword(in.NewPassword, pwUser, "newPassword")
	if err != nil {
		return AuthResult{}, err
	}
	code, codeOK := NormalizeRecoveryCode(in.RecoveryCode)
	ph, err := s.hasher.Hash(ctx, pw)
	if err != nil {
		return AuthResult{}, err
	}
	if uerr != nil || !codeOK {
		return AuthResult{}, ErrRecoveryFailed
	}
	var res AuthResult
	err = s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		// The credentials row is locked first, as in RegenerateRecoveryCodes,
		// so recoveries and regenerations of one account serialise.
		var user UserID
		known := true
		err := tx.QueryRow(ctx, `SELECT user_id FROM credentials WHERE username = $1 FOR UPDATE`, username).Scan(&user)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			user, known = NewUserID(), false
		case err != nil:
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE recovery_codes SET used_at = $3 WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`,
			user, RecoveryCodeHash(user, code), now)
		if err != nil {
			return err
		}
		if !known || tag.RowsAffected() != 1 {
			return ErrRecoveryFailed
		}
		if _, err := tx.Exec(ctx, `UPDATE credentials SET password_hash = $2, password_salt = $3, argon2_memory_kib = $4,
			argon2_iterations = $5, argon2_parallelism = $6, password_changed_at = $7, failed_count = 0, last_failed_at = NULL,
			password_login_disabled_at = NULL WHERE user_id = $1`, user, ph.Tag, ph.Salt,
			int32(ph.Params.MemoryKiB), int32(ph.Params.Iterations), int16(ph.Params.Parallelism), now); err != nil {
			return err
		}
		if err := clearThrottle(ctx, tx, s.UsernameKey(username)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $2, revoke_reason = 'recovery' WHERE user_id = $1 AND revoked_at IS NULL`, user, now); err != nil {
			return err
		}
		if err := s.revokeToken(ctx, tx, in.PriorToken, "rotated"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET last_active_at = $2 WHERE id = $1`, user, now); err != nil {
			return err
		}
		var remaining int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND used_at IS NULL`, user).Scan(&remaining); err != nil {
			return err
		}
		ns := s.accountSession(user, in.RememberDevice, in.UserAgent, now, nil)
		view, ssid, tok, err := s.insertSession(ctx, tx, ns)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, user, &ssid, "recovery.used", actorUser, map[string]any{"remaining": remaining}); err != nil {
			return err
		}
		res = AuthResult{Session: view, Token: tok, CookieMaxAge: s.cookieAge(ns), RemainingRecoveryCodes: remaining}
		res.Profile, err = s.profile(ctx, tx, user)
		return err
	})
	if err != nil {
		return AuthResult{}, err
	}
	return res, nil
}

// requireAccount loads the owner's credentials, or ErrAccountRequired for
// a guest.
func (s *Store) requireAccount(ctx context.Context, q db.DBTX, owner UserID) (*credential, error) {
	c, err := s.credentialByUser(ctx, q, owner, false)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrAccountRequired
	}
	return c, nil
}

// proveCurrent verifies the owner's current password. The attempt is
// reserved like a login (reserveSQL), so a wrong one counts towards the
// login backoff and the 100-attempt disable, and concurrent proofs cannot
// pass one backoff step together. The caller's success transaction must
// call resetFailures (or check the disable itself) under the credentials
// row lock.
func (s *Store) proveCurrent(ctx context.Context, owner UserID, current SessionID, password string) (*credential, string, error) {
	acct, err := s.requireAccount(ctx, s.pool, owner)
	if err != nil {
		return nil, "", err
	}
	key := s.UsernameKey(acct.username)
	c, err := s.reserve(ctx, key, acct.username, &current)
	if err != nil {
		return nil, "", err
	}
	if c == nil || c.user != owner {
		// The account was deleted meanwhile.
		return nil, "", ErrNotSignedIn
	}
	pw := norm.NFKC.String(password)
	ok, err := s.hasher.Verify(ctx, pw, &c.hash)
	if err != nil {
		s.release(ctx, key, c.username)
		return nil, "", err
	}
	if !ok {
		if err := s.recordFailure(ctx, c.username, &current); err != nil {
			return nil, "", err
		}
		return nil, "", ErrCurrentPasswordIncorrect
	}
	return c, pw, nil
}

// Reauthenticate records a fresh password proof on the current session,
// which export and deletion require for accounts.
func (s *Store) Reauthenticate(ctx context.Context, owner UserID, current SessionID, password string) (time.Time, error) {
	c, _, err := s.proveCurrent(ctx, owner, current, password)
	if err != nil {
		return time.Time{}, err
	}
	now := s.clock()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		// Credentials and throttle first, then the session row (lock order).
		if err := s.resetFailures(ctx, tx, owner, c.username); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE sessions SET authenticated_at = $3 WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, current, owner, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrNotSignedIn
		}
		return s.audit(ctx, tx, owner, &current, "auth.reauthenticated", actorUser, nil)
	})
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(s.cfg.ReauthWindow), nil
}

// rotate revokes the current session with reason and creates its
// successor (same kind; trust as given) in the caller's transaction.
func (s *Store) rotate(ctx context.Context, tx pgx.Tx, owner UserID, current SessionID, reason string, ns newSession) (SessionView, SessionID, string, error) {
	tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = $3, revoke_reason = $4 WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		current, owner, s.clock(), reason)
	if err != nil {
		return SessionView{}, SessionID{}, "", err
	}
	if tag.RowsAffected() != 1 {
		return SessionView{}, SessionID{}, "", ErrNotSignedIn
	}
	ns.rotatedFrom = &current
	return s.insertSession(ctx, tx, ns)
}

// ChangePassword needs the current password; the current session is
// rotated and every other session revoked.
func (s *Store) ChangePassword(ctx context.Context, owner UserID, current SessionID, currentPassword, newPassword, userAgent string) (AuthResult, error) {
	c, err := s.requireAccount(ctx, s.pool, owner)
	if err != nil {
		return AuthResult{}, err
	}
	next, err := CheckPassword(newPassword, c.username, "newPassword")
	if err != nil {
		return AuthResult{}, err
	}
	if _, _, err := s.proveCurrent(ctx, owner, current, currentPassword); err != nil {
		return AuthResult{}, err
	}
	ph, err := s.hasher.Hash(ctx, next)
	if err != nil {
		return AuthResult{}, err
	}
	var res AuthResult
	err = s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		// The credentials row is locked first, as in Recover (which then
		// revokes every session of the user), and only then the session
		// rows; locking the session first deadlocked against a recovery
		// (review round 3, R3-2). Refused if password sign-in was disabled
		// while the current password was being verified.
		tag, err := tx.Exec(ctx, `UPDATE credentials SET password_hash = $2, password_salt = $3, argon2_memory_kib = $4,
			argon2_iterations = $5, argon2_parallelism = $6, password_changed_at = $7, failed_count = 0, last_failed_at = NULL
			WHERE user_id = $1 AND password_login_disabled_at IS NULL`, owner, ph.Tag, ph.Salt, int32(ph.Params.MemoryKiB),
			int32(ph.Params.Iterations), int16(ph.Params.Parallelism), now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrPasswordLoginDisabled
		}
		if err := clearThrottle(ctx, tx, s.UsernameKey(c.username)); err != nil {
			return err
		}
		var trusted bool
		err = tx.QueryRow(ctx, `SELECT trusted_device FROM sessions WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL FOR UPDATE`, current, owner).Scan(&trusted)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotSignedIn
		}
		if err != nil {
			return err
		}
		tag, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at = $3, revoke_reason = 'password_changed'
			WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`, owner, current, now)
		if err != nil {
			return err
		}
		revoked := int(tag.RowsAffected())
		ns := s.accountSession(owner, trusted, userAgent, now, nil)
		view, ssid, tok, err := s.rotate(ctx, tx, owner, current, "rotated", ns)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, owner, &ssid, "password.changed", actorUser, map[string]any{"revoked": revoked}); err != nil {
			return err
		}
		res = AuthResult{Session: view, Token: tok, CookieMaxAge: s.cookieAge(ns), RevokedSessions: revoked}
		res.Profile, err = s.profile(ctx, tx, owner)
		return err
	})
	if err != nil {
		return AuthResult{}, err
	}
	return res, nil
}

// RegenerateRecoveryCodes needs the current password; every previous code
// becomes invalid and the acknowledgement is reset. The credentials row is
// locked first, so overlapping regenerations (and a recovery) of one
// account serialise: under READ COMMITTED the second DELETE would
// otherwise not see the first one's new codes, and both batches would
// stay valid.
func (s *Store) RegenerateRecoveryCodes(ctx context.Context, owner UserID, current SessionID, password string) ([]string, time.Time, error) {
	c, _, err := s.proveCurrent(ctx, owner, current, password)
	if err != nil {
		return nil, time.Time{}, err
	}
	codes, hashes, err := newRecoveryCodes(owner)
	if err != nil {
		return nil, time.Time{}, err
	}
	now := s.clock()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var disabled bool
		err := tx.QueryRow(ctx, `SELECT password_login_disabled_at IS NOT NULL FROM credentials WHERE user_id = $1 FOR UPDATE`, owner).Scan(&disabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAccountRequired
		}
		if err != nil {
			return err
		}
		if disabled {
			// Password sign-in was disabled while the password was verified.
			return ErrPasswordLoginDisabled
		}
		if _, err := tx.Exec(ctx, `DELETE FROM recovery_codes WHERE user_id = $1`, owner); err != nil {
			return err
		}
		if err := s.insertRecoveryCodes(ctx, tx, owner, hashes, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE credentials SET recovery_generated_at = $2, recovery_acknowledged_at = NULL,
			failed_count = 0, last_failed_at = NULL WHERE user_id = $1`, owner, now); err != nil {
			return err
		}
		if err := clearThrottle(ctx, tx, s.UsernameKey(c.username)); err != nil {
			return err
		}
		return s.audit(ctx, tx, owner, &current, "recovery.regenerated", actorUser, map[string]any{"count": len(codes)})
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	return codes, now, nil
}

// AcknowledgeRecoveryCodes records "I have saved them"; idempotent.
func (s *Store) AcknowledgeRecoveryCodes(ctx context.Context, owner UserID, current SessionID) (Profile, error) {
	var p Profile
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := s.requireAccount(ctx, tx, owner); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE credentials SET recovery_acknowledged_at = $2 WHERE user_id = $1 AND recovery_acknowledged_at IS NULL`, owner, s.clock())
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			if err := s.audit(ctx, tx, owner, &current, "recovery.acknowledged", actorUser, nil); err != nil {
				return err
			}
		}
		p, err = s.profile(ctx, tx, owner)
		return err
	})
	return p, err
}

// UpgradeGuest turns the owner's guest into an account under the same
// user id, so every owned row stays: credentials and codes are created,
// the guest expiry cleared, the session rotated and account.upgraded
// written, in one transaction.
func (s *Store) UpgradeGuest(ctx context.Context, owner UserID, current SessionID, in UpgradeInput) (AuthResult, error) {
	var kind string
	if err := s.pool.QueryRow(ctx, `SELECT kind FROM users WHERE id = $1`, owner).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, ErrNotSignedIn
		}
		return AuthResult{}, err
	}
	if kind != KindGuest {
		return AuthResult{}, ErrNotGuest
	}
	username, err := NormalizeUsername(in.Username)
	if err != nil {
		return AuthResult{}, err
	}
	pw, err := CheckPassword(in.Password, username, "password")
	if err != nil {
		return AuthResult{}, err
	}
	consents, err := loadConsents(ctx, s.pool, owner)
	if err != nil {
		return AuthResult{}, err
	}
	if c, ok := consents[PolicyTerms]; !ok || !c.granted || c.version != s.cfg.PolicyVersions[PolicyTerms] {
		return AuthResult{}, ErrPolicyOutdated
	}
	ph, err := s.hasher.Hash(ctx, pw)
	if err != nil {
		return AuthResult{}, err
	}
	codes, hashes, err := newRecoveryCodes(owner)
	if err != nil {
		return AuthResult{}, err
	}
	var res AuthResult
	err = s.tx(ctx, func(tx pgx.Tx) error {
		now := s.clock()
		if err := tx.QueryRow(ctx, `SELECT kind FROM users WHERE id = $1 FOR UPDATE`, owner).Scan(&kind); err != nil {
			return err
		}
		if kind != KindGuest {
			return ErrNotGuest
		}
		if err := s.insertCredentials(ctx, tx, owner, username, ph, now); err != nil {
			return err
		}
		if err := s.insertRecoveryCodes(ctx, tx, owner, hashes, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET kind = 'account', upgraded_at = $2, guest_expires_at = NULL,
			last_active_at = $2, version = version + 1 WHERE id = $1`, owner, now); err != nil {
			return err
		}
		ns := s.accountSession(owner, in.RememberDevice, in.UserAgent, now, nil)
		view, ssid, tok, err := s.rotate(ctx, tx, owner, current, "upgraded", ns)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, owner, &ssid, "account.upgraded", actorUser, map[string]any{"from": KindGuest, "to": KindAccount}); err != nil {
			return err
		}
		res = AuthResult{Session: view, Token: tok, CookieMaxAge: s.cookieAge(ns), RecoveryCodes: codes, RemainingRecoveryCodes: len(codes)}
		res.Profile, err = s.profile(ctx, tx, owner)
		return err
	})
	return res, err
}

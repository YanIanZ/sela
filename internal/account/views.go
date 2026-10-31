package account

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/db"
)

// Profile is the signed-in user (API_SPEC_V3 §3).
type Profile struct {
	ID                 string          `json:"id"`
	Kind               string          `json:"kind"`
	Username           *string         `json:"username"`
	Locale             string          `json:"locale"`
	AgeBand            string          `json:"ageBand"`
	AgeBandConfirmedAt Timestamp       `json:"ageBandConfirmedAt"`
	Version            int             `json:"version"`
	CreatedAt          Timestamp       `json:"createdAt"`
	LastActiveAt       Timestamp       `json:"lastActiveAt"`
	Guest              *GuestInfo      `json:"guest"`
	Recovery           *RecoveryInfo   `json:"recovery"`
	GuardianConsent    *GuardianStatus `json:"guardianConsent"`
	ActionsRequired    []string        `json:"actionsRequired"`
	Environment        string          `json:"environment"`
	Synthetic          bool            `json:"synthetic"`
}

// GuestInfo tells a guest where the data is and when it is deleted.
type GuestInfo struct {
	InactivePurgeAt  Timestamp    `json:"inactivePurgeAt"`
	MaxAgePurgeAt    Timestamp    `json:"maxAgePurgeAt"`
	InactiveTTLHours int          `json:"inactiveTtlHours"`
	MaxAgeDays       int          `json:"maxAgeDays"`
	Storage          GuestStorage `json:"storage"`
}

// GuestStorage describes guest storage honestly: a server database,
// encrypted per user, reachable only through this browser's cookie.
type GuestStorage struct {
	Location  string  `json:"location"`
	Region    *string `json:"region"`
	Encrypted bool    `json:"encrypted"`
	BoundTo   string  `json:"boundTo"`
}

// RecoveryInfo summarises an account's recovery codes (never the codes).
type RecoveryInfo struct {
	Remaining    int       `json:"remaining"`
	GeneratedAt  Timestamp `json:"generatedAt"`
	Acknowledged bool      `json:"acknowledged"`
}

// GuardianStatus is reported for the 15_17 band only.
type GuardianStatus struct {
	Required bool   `json:"required"`
	Status   string `json:"status"`
}

// SessionView is a session as the owner sees it.
type SessionView struct {
	ID                string    `json:"id"`
	Current           bool      `json:"current"`
	Kind              string    `json:"kind"`
	TrustedDevice     bool      `json:"trustedDevice"`
	Device            Device    `json:"device"`
	CreatedAt         Timestamp `json:"createdAt"`
	LastSeenAt        Timestamp `json:"lastSeenAt"`
	IdleExpiresAt     Timestamp `json:"idleExpiresAt"`
	AbsoluteExpiresAt Timestamp `json:"absoluteExpiresAt"`
}

// AuthResult is what a sign-in path returns. Token and RecoveryCodes are
// one-time secrets: never logged, never stored in plaintext.
type AuthResult struct {
	Profile Profile
	Session SessionView
	Token   string
	// CookieMaxAge is the cookie lifetime; 0 means a browser-session cookie
	// (untrusted device).
	CookieMaxAge           time.Duration
	RecoveryCodes          []string
	RemainingRecoveryCodes int
	RevokedSessions        int
}

// Page selects one page of a growing list (newest first).
type Page struct {
	Limit  int
	Cursor string
}

// Pagination bounds.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

func (p Page) limit() (int, error) {
	switch {
	case p.Limit == 0:
		return DefaultPageLimit, nil
	case p.Limit < 1 || p.Limit > MaxPageLimit:
		return 0, invalid("limit")
	}
	return p.Limit, nil
}

type cursor struct {
	at time.Time
	id [16]byte
}

func encodeCursor(at time.Time, id [16]byte) string {
	b := binary.BigEndian.AppendUint64(nil, uint64(at.UnixMicro()))
	return base64.RawURLEncoding.EncodeToString(append(b, id[:]...))
}

// cursorEarliest bounds the time of a cursor from below: no row of these
// lists is older than the product, and PostgreSQL refuses times before
// 4713 BC (SQLSTATE 22008, which would answer 500 and log an ERROR).
var cursorEarliest = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// cursor decodes ?cursor=. Only a value encodeCursor could have produced
// for a row is accepted: 24 base64url bytes, a time between 2000-01-01 and
// one day after now, and a version-4 RFC 4122 id (every id of these lists
// is one, newUUID); anything else is 400 invalid_input (field cursor) and
// never reaches the database.
func (p Page) cursor(now time.Time) (*cursor, error) {
	if p.Cursor == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(p.Cursor)
	if err != nil || len(b) != 24 {
		return nil, invalid("cursor")
	}
	c := &cursor{at: time.UnixMicro(int64(binary.BigEndian.Uint64(b[:8]))).UTC()}
	copy(c.id[:], b[8:])
	if c.at.Before(cursorEarliest) || c.at.After(now.Add(24*time.Hour)) || c.id[6]>>4 != 4 || c.id[8]>>6 != 2 {
		return nil, invalid("cursor")
	}
	return c, nil
}

type userRow struct {
	kind                 string
	ageBand              string
	ageBandConfirmedAt   time.Time
	locale               string
	version              int
	createdAt            time.Time
	lastActiveAt         time.Time
	guestExpiresAt       *time.Time
	username             *string
	recoveryGeneratedAt  *time.Time
	recoveryAcknowledged *time.Time
	passwordChangedAt    *time.Time
	remainingCodes       int
	guardianActive       bool
	guardianAny          bool
}

func loadUser(ctx context.Context, q db.DBTX, owner UserID) (userRow, error) {
	var u userRow
	err := q.QueryRow(ctx, `SELECT u.kind, u.age_band, u.age_band_confirmed_at, u.locale, u.version, u.created_at, u.last_active_at,
		u.guest_expires_at, c.username, c.recovery_generated_at, c.recovery_acknowledged_at, c.password_changed_at,
		(SELECT count(*) FROM recovery_codes r WHERE r.user_id = u.id AND r.used_at IS NULL),
		EXISTS (SELECT 1 FROM guardian_consents g WHERE g.user_id = u.id AND g.withdrawn_at IS NULL),
		EXISTS (SELECT 1 FROM guardian_consents g WHERE g.user_id = u.id)
		FROM users u LEFT JOIN credentials c ON c.user_id = u.id WHERE u.id = $1`, owner).Scan(
		&u.kind, &u.ageBand, &u.ageBandConfirmedAt, &u.locale, &u.version, &u.createdAt, &u.lastActiveAt, &u.guestExpiresAt,
		&u.username, &u.recoveryGeneratedAt, &u.recoveryAcknowledged, &u.passwordChangedAt, &u.remainingCodes, &u.guardianActive, &u.guardianAny)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

type consentRow struct {
	purpose   string
	granted   bool
	version   string
	decidedAt time.Time
	revision  int
}

func loadConsents(ctx context.Context, q db.DBTX, owner UserID) (map[string]consentRow, error) {
	rows, err := q.Query(ctx, `SELECT purpose, granted, policy_version, decided_at, revision FROM consents WHERE user_id = $1`, owner)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (consentRow, error) {
		var c consentRow
		err := r.Scan(&c.purpose, &c.granted, &c.version, &c.decidedAt, &c.revision)
		return c, err
	})
	out := map[string]consentRow{}
	for _, c := range list {
		out[c.purpose] = c
	}
	return out, err
}

// guardianStatus is missing, active or withdrawn for 15_17, else
// not_applicable.
func (s *Store) guardianStatus(u userRow) (bool, string) {
	if u.ageBand != AgeBand15to17 {
		return false, "not_applicable"
	}
	switch {
	case u.guardianActive:
		return s.cfg.GuardianConsentRequired, "active"
	case u.guardianAny:
		return s.cfg.GuardianConsentRequired, "withdrawn"
	}
	return s.cfg.GuardianConsentRequired, "missing"
}

func (s *Store) profile(ctx context.Context, q db.DBTX, owner UserID) (Profile, error) {
	u, err := loadUser(ctx, q, owner)
	if err != nil {
		return Profile{}, err
	}
	consents, err := loadConsents(ctx, q, owner)
	if err != nil {
		return Profile{}, err
	}
	now := s.clock()
	p := Profile{
		ID: owner.String(), Kind: u.kind, Locale: u.locale, AgeBand: u.ageBand, AgeBandConfirmedAt: ts(u.ageBandConfirmedAt),
		Version: u.version, CreatedAt: ts(u.createdAt), LastActiveAt: ts(u.lastActiveAt), ActionsRequired: []string{},
		Environment: s.cfg.Environment, Synthetic: s.cfg.Synthetic(),
	}
	for _, pur := range s.cfg.purposes() {
		if !pur.Required || !pur.appliesTo(u.kind) {
			continue
		}
		c, ok := consents[pur.Name]
		if !ok || !c.granted || c.version != s.cfg.PolicyVersions[pur.Name] {
			p.ActionsRequired = append(p.ActionsRequired, "accept_policy:"+pur.Name)
		}
	}
	if u.kind == KindGuest && u.guestExpiresAt != nil {
		var region *string
		if s.cfg.RegionLabel != "" {
			r := s.cfg.RegionLabel
			region = &r
		}
		p.Guest = &GuestInfo{
			InactivePurgeAt: ts(u.lastActiveAt.Add(s.cfg.GuestInactiveTTL)), MaxAgePurgeAt: ts(*u.guestExpiresAt),
			InactiveTTLHours: int(s.cfg.GuestInactiveTTL / time.Hour), MaxAgeDays: int(s.cfg.GuestMaxAge / (24 * time.Hour)),
			Storage: GuestStorage{Location: "server_database", Region: region, Encrypted: true, BoundTo: "this_browser_cookie"},
		}
	}
	if u.kind == KindAccount && u.username != nil {
		p.Username = u.username
		gen := time.Time{}
		if u.recoveryGeneratedAt != nil {
			gen = *u.recoveryGeneratedAt
		}
		p.Recovery = &RecoveryInfo{Remaining: u.remainingCodes, GeneratedAt: ts(gen), Acknowledged: u.recoveryAcknowledged != nil}
		if u.recoveryAcknowledged == nil || u.remainingCodes <= recoveryLowMark {
			p.ActionsRequired = append(p.ActionsRequired, "acknowledge_recovery_codes")
		}
	}
	if u.ageBand == AgeBand15to17 {
		req, status := s.guardianStatus(u)
		p.GuardianConsent = &GuardianStatus{Required: req, Status: status}
		if now.Sub(u.ageBandConfirmedAt) > 365*24*time.Hour {
			p.ActionsRequired = append(p.ActionsRequired, "confirm_age_band")
		}
		if req && status != "active" {
			p.ActionsRequired = append(p.ActionsRequired, "guardian_consent")
		}
	}
	return p, nil
}

// Profile returns the owner's profile.
func (s *Store) Profile(ctx context.Context, owner UserID) (Profile, error) {
	return s.profile(ctx, s.pool, owner)
}

// ProfileUpdate is PATCH /me.
type ProfileUpdate struct {
	ExpectedVersion int
	Locale          *string
	AgeBand         *string
}

func validLocale(l string) bool { return l == "id" || l == "en" }

// checkBand refuses under_15 and under_16 (nothing is stored), unknown
// values and bands this deployment does not enable.
func (s *Store) checkBand(b string) error {
	switch b {
	case AgeBandUnder15, AgeBandUnder16:
		return ErrAgeNotSupported
	case AgeBand15to17, AgeBand18Plus, AgeBandUnknown:
	default:
		return invalid("ageBand")
	}
	if !s.cfg.bandAllowed(b) {
		return ErrAgeBandNotEnabled
	}
	return nil
}

// UpdateProfile changes the locale and/or the age band under optimistic
// concurrency. Sending the current band again confirms it.
func (s *Store) UpdateProfile(ctx context.Context, owner UserID, current SessionID, in ProfileUpdate) (Profile, error) {
	if in.Locale == nil && in.AgeBand == nil {
		return Profile{}, invalid("body")
	}
	if in.Locale != nil && !validLocale(*in.Locale) {
		return Profile{}, invalid("locale")
	}
	if in.AgeBand != nil {
		if err := s.checkBand(*in.AgeBand); err != nil {
			return Profile{}, err
		}
	}
	var p Profile
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var version int
		var locale, band string
		err := tx.QueryRow(ctx, `SELECT version, locale, age_band FROM users WHERE id = $1 FOR UPDATE`, owner).Scan(&version, &locale, &band)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if in.ExpectedVersion != version {
			return ErrStaleVersion
		}
		now := s.clock()
		newLocale, newBand := locale, band
		if in.Locale != nil {
			newLocale = *in.Locale
		}
		if in.AgeBand != nil {
			newBand = *in.AgeBand
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET locale = $2, age_band = $3,
			age_band_confirmed_at = CASE WHEN $4 THEN $5 ELSE age_band_confirmed_at END, version = version + 1 WHERE id = $1`,
			owner, newLocale, newBand, in.AgeBand != nil, now); err != nil {
			return err
		}
		sid := &current
		if newLocale != locale {
			if err := s.audit(ctx, tx, owner, sid, "profile.locale_changed", actorUser, map[string]any{"from": locale, "to": newLocale}); err != nil {
				return err
			}
		}
		if in.AgeBand != nil {
			if newBand != band {
				err = s.audit(ctx, tx, owner, sid, "profile.age_band_changed", actorUser, map[string]any{"from": band, "to": newBand})
			} else {
				err = s.audit(ctx, tx, owner, sid, "profile.age_band_confirmed", actorUser, map[string]any{"ageBand": newBand})
			}
			if err != nil {
				return err
			}
		}
		p, err = s.profile(ctx, tx, owner)
		return err
	})
	return p, err
}

func sessionView(id SessionID, current bool, kind string, trusted bool, d Device, created, lastSeen time.Time, idle time.Duration, absolute time.Time) SessionView {
	idleAt := lastSeen.Add(idle)
	if idleAt.After(absolute) {
		idleAt = absolute
	}
	return SessionView{ID: id.String(), Current: current, Kind: kind, TrustedDevice: trusted, Device: d, CreatedAt: ts(created),
		LastSeenAt: ts(lastSeen), IdleExpiresAt: ts(idleAt), AbsoluteExpiresAt: ts(absolute)}
}

// Sessions lists the owner's live sessions, current first; last seen is
// rounded down to the hour.
func (s *Store) Sessions(ctx context.Context, owner UserID, current SessionID) ([]SessionView, error) {
	now := s.clock()
	rows, err := s.pool.Query(ctx, `SELECT id, kind, trusted_device, device_browser, device_os, created_at, last_seen_at,
		idle_timeout_seconds, absolute_expires_at FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND absolute_expires_at > $2
		  AND last_seen_at + make_interval(secs => idle_timeout_seconds) > $2
		ORDER BY created_at DESC, id DESC`, owner, now)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (SessionView, error) {
		var id SessionID
		var kind string
		var trusted bool
		var d Device
		var created, lastSeen, absolute time.Time
		var idle int
		if err := r.Scan(&id, &kind, &trusted, &d.Browser, &d.OS, &created, &lastSeen, &idle, &absolute); err != nil {
			return SessionView{}, err
		}
		v := sessionView(id, id == current, kind, trusted, d, created, lastSeen, time.Duration(idle)*time.Second, absolute)
		v.LastSeenAt = ts(lastSeen.Truncate(time.Hour))
		return v, nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(out, func(a, b SessionView) int {
		switch {
		case a.Current == b.Current:
			return 0
		case a.Current:
			return -1
		}
		return 1
	})
	return out, nil
}

package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/vault"
)

// ConsentView is the current decision for one purpose.
type ConsentView struct {
	Purpose              string     `json:"purpose"`
	Required             bool       `json:"required"`
	Granted              bool       `json:"granted"`
	PolicyVersion        *string    `json:"policyVersion"`
	CurrentPolicyVersion string     `json:"currentPolicyVersion"`
	Outdated             bool       `json:"outdated"`
	DecidedAt            *Timestamp `json:"decidedAt"`
	Revision             int        `json:"revision"`
}

// ConsentEvent is one history entry.
type ConsentEvent struct {
	ID            string    `json:"id"`
	Purpose       string    `json:"purpose"`
	Decision      string    `json:"decision"`
	PolicyVersion string    `json:"policyVersion"`
	Source        string    `json:"source"`
	DecidedAt     Timestamp `json:"decidedAt"`
}

func newID() pgtype.UUID { return pgtype.UUID{Bytes: newUUID(), Valid: true} }

func (s *Store) consentView(p Purpose, c *consentRow) ConsentView {
	cur := s.cfg.PolicyVersions[p.Name]
	v := ConsentView{Purpose: p.Name, Required: p.Required, CurrentPolicyVersion: cur}
	if c != nil {
		ver := c.version
		v.Granted, v.PolicyVersion, v.DecidedAt, v.Revision = c.granted, &ver, tsPtr(&c.decidedAt), c.revision
		v.Outdated = c.granted && c.version != cur
	}
	return v
}

func userKind(ctx context.Context, q db.DBTX, owner UserID, lock bool) (string, error) {
	sql := `SELECT kind FROM users WHERE id = $1`
	if lock {
		sql += ` FOR UPDATE`
	}
	var kind string
	err := q.QueryRow(ctx, sql, owner).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return kind, err
}

// Consents lists every active purpose that applies to the owner.
func (s *Store) Consents(ctx context.Context, owner UserID) ([]ConsentView, error) {
	kind, err := userKind(ctx, s.pool, owner, false)
	if err != nil {
		return nil, err
	}
	rows, err := loadConsents(ctx, s.pool, owner)
	if err != nil {
		return nil, err
	}
	out := []ConsentView{}
	for _, p := range s.cfg.purposes() {
		if !p.appliesTo(kind) {
			continue
		}
		var c *consentRow
		if r, ok := rows[p.Name]; ok {
			c = &r
		}
		out = append(out, s.consentView(p, c))
	}
	return out, nil
}

// policyVersionPattern is the shape of every policy version, configured
// or sent: lower-case letters, digits, dot, underscore and hyphen, at most
// 64 characters. Audit detail strings use it too, so no free text can
// reach the plaintext consent columns or the audit trail.
var policyVersionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// SetConsent records a decision. Granting needs the current policy
// version. Withdrawing is never blocked by a version mismatch, and the
// version sent with it is not stored: the withdrawal records the version
// of the user's last decision for that purpose (the text they agreed to),
// or the current version when there was none, so no client text reaches
// consents, consent_events or the audit trail. A required purpose cannot
// be withdrawn. The same decision changes nothing.
func (s *Store) SetConsent(ctx context.Context, owner UserID, current SessionID, purpose string, granted bool, policyVersion string) (ConsentView, bool, error) {
	p, ok := s.cfg.purpose(purpose)
	if !ok {
		return ConsentView{}, false, ErrNotFound
	}
	if !policyVersionPattern.MatchString(policyVersion) {
		return ConsentView{}, false, invalid("policyVersion")
	}
	var view ConsentView
	changed := false
	err := s.tx(ctx, func(tx pgx.Tx) error {
		changed = false // the body may run again (Store.tx)
		kind, err := userKind(ctx, tx, owner, true)
		if err != nil {
			return err
		}
		if !p.appliesTo(kind) {
			return ErrNotFound
		}
		if !granted && p.Required {
			return ErrRequiredConsent
		}
		if granted && policyVersion != s.cfg.PolicyVersions[p.Name] {
			return withField(ErrPolicyOutdated, "policyVersion")
		}
		rows, err := loadConsents(ctx, tx, owner)
		if err != nil {
			return err
		}
		version := policyVersion
		if !granted {
			version = s.cfg.PolicyVersions[p.Name]
			if c, ok := rows[p.Name]; ok {
				version = c.version
			}
		}
		if c, ok := rows[p.Name]; ok && c.granted == granted && c.version == version {
			view = s.consentView(p, &c)
			return nil
		}
		now := s.clock()
		if _, err := s.recordConsent(ctx, tx, owner, p.Name, granted, version, "settings", now); err != nil {
			return err
		}
		typ := "consent.granted"
		if !granted {
			typ = "consent.withdrawn"
		}
		if err := s.audit(ctx, tx, owner, &current, typ, actorUser, map[string]any{"purpose": p.Name, "policyVersion": version}); err != nil {
			return err
		}
		rows, err = loadConsents(ctx, tx, owner)
		if err != nil {
			return err
		}
		c := rows[p.Name]
		view, changed = s.consentView(p, &c), true
		return nil
	})
	return view, changed, err
}

var purposeName = regexp.MustCompile(`^[a-z][a-z0-9_]{1,47}$`)

// ConsentHistory pages the owner's consent events, newest first,
// optionally for one purpose.
func (s *Store) ConsentHistory(ctx context.Context, owner UserID, purpose string, page Page) ([]ConsentEvent, string, error) {
	limit, err := page.limit()
	if err != nil {
		return nil, "", err
	}
	cur, err := page.cursor(s.clock())
	if err != nil {
		return nil, "", err
	}
	if purpose != "" && !purposeName.MatchString(purpose) {
		return nil, "", invalid("purpose")
	}
	if _, err := userKind(ctx, s.pool, owner, false); err != nil {
		return nil, "", err
	}
	args := []any{owner, purpose, limit + 1}
	where := `user_id = $1 AND ($2 = '' OR purpose = $2)`
	if cur != nil {
		where += ` AND (decided_at, id) < ($4, $5)`
		args = append(args, cur.at, pgtype.UUID{Bytes: cur.id, Valid: true})
	}
	rows, err := s.pool.Query(ctx, `SELECT id, purpose, decision, policy_version, source, decided_at FROM consent_events
		WHERE `+where+` ORDER BY decided_at DESC, id DESC LIMIT $3`, args...)
	if err != nil {
		return nil, "", err
	}
	type row struct {
		id pgtype.UUID
		ev ConsentEvent
		at time.Time
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		err := r.Scan(&x.id, &x.ev.Purpose, &x.ev.Decision, &x.ev.PolicyVersion, &x.ev.Source, &x.at)
		x.ev.ID, x.ev.DecidedAt = x.id.String(), ts(x.at)
		return x, err
	})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(list) > limit {
		last := list[limit-1]
		next = encodeCursor(last.at, last.id.Bytes)
		list = list[:limit]
	}
	out := make([]ConsentEvent, len(list))
	for i, x := range list {
		out[i] = x.ev
	}
	return out, next, nil
}

// GuardianRecord is one guardian consent record (consent is not access).
type GuardianRecord struct {
	ID            string     `json:"id"`
	Relationship  string     `json:"relationship"`
	Method        string     `json:"method"`
	GuardianName  *string    `json:"guardianName"`
	PolicyVersion string     `json:"policyVersion"`
	GrantedAt     Timestamp  `json:"grantedAt"`
	WithdrawnAt   *Timestamp `json:"withdrawnAt"`
	WithdrawnBy   *string    `json:"withdrawnBy"`
}

// GuardianView is GET /me/guardian-consent.
type GuardianView struct {
	Required bool             `json:"required"`
	Status   string           `json:"status"`
	Active   *GuardianRecord  `json:"active"`
	History  []GuardianRecord `json:"history"`
}

// GuardianInput is PUT /me/guardian-consent.
type GuardianInput struct {
	Relationship  string
	Method        string
	GuardianName  *string
	PolicyVersion string
}

// GuardianNameMax bounds the optional guardian name.
const GuardianNameMax = 80

func guardianRef(id [16]byte) vault.Ref {
	return vault.Ref{Kind: "guardian_consent", RecordID: uuidText(id), Field: "guardian_name"}
}

func (s *Store) guardianRecords(ctx context.Context, q db.DBTX, owner UserID) ([]GuardianRecord, error) {
	rows, err := q.Query(ctx, `SELECT id, relationship, method, guardian_name_ct, policy_version, granted_at, withdrawn_at, withdrawn_by
		FROM guardian_consents WHERE user_id = $1 ORDER BY granted_at DESC, id DESC`, owner)
	if err != nil {
		return nil, err
	}
	type row struct {
		id  pgtype.UUID
		ct  []byte
		rec GuardianRecord
		at  time.Time
		wd  *time.Time
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		err := r.Scan(&x.id, &x.rec.Relationship, &x.rec.Method, &x.ct, &x.rec.PolicyVersion, &x.at, &x.wd, &x.rec.WithdrawnBy)
		return x, err
	})
	if err != nil {
		return nil, err
	}
	out := make([]GuardianRecord, 0, len(list))
	for _, x := range list {
		x.rec.ID, x.rec.GrantedAt, x.rec.WithdrawnAt = x.id.String(), ts(x.at), tsPtr(x.wd)
		if x.ct != nil {
			pt, err := s.vault.Open(ctx, q, owner, guardianRef(x.id.Bytes), x.ct)
			if err != nil {
				return nil, fmt.Errorf("account: guardian name: %w", err)
			}
			name := string(pt)
			x.rec.GuardianName = &name
		}
		out = append(out, x.rec)
	}
	return out, nil
}

// GuardianConsent reports whether a record is required, its status, the
// active record and the history.
func (s *Store) GuardianConsent(ctx context.Context, owner UserID) (GuardianView, error) {
	u, err := loadUser(ctx, s.pool, owner)
	if err != nil {
		return GuardianView{}, err
	}
	hist, err := s.guardianRecords(ctx, s.pool, owner)
	if err != nil {
		return GuardianView{}, err
	}
	req, status := s.guardianStatus(u)
	v := GuardianView{Required: req, Status: status, History: hist}
	for i := range hist {
		if hist[i].WithdrawnAt == nil {
			v.Active = &hist[i]
		}
	}
	return v, nil
}

func cleanGuardianName(in *string) (*string, error) {
	if in == nil {
		return nil, nil
	}
	n := strings.TrimSpace(*in)
	if n == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(n) > GuardianNameMax || containsContactDetails(n) {
		return nil, invalid("guardianName")
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return nil, invalid("guardianName")
		}
	}
	return &n, nil
}

// SetGuardianConsent records a guardian's agreement for a 15_17 user,
// withdrawing and replacing an active record in the same transaction. The
// optional name is encrypted; contact-like names are refused.
func (s *Store) SetGuardianConsent(ctx context.Context, owner UserID, current SessionID, in GuardianInput) (GuardianRecord, error) {
	if in.Relationship != "parent" && in.Relationship != "legal_guardian" {
		return GuardianRecord{}, invalid("relationship")
	}
	if in.Method != "guardian_present_attestation" {
		return GuardianRecord{}, invalid("method")
	}
	name, err := cleanGuardianName(in.GuardianName)
	if err != nil {
		return GuardianRecord{}, err
	}
	var rec GuardianRecord
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var band string
		err := tx.QueryRow(ctx, `SELECT age_band FROM users WHERE id = $1 FOR UPDATE`, owner).Scan(&band)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if band != AgeBand15to17 {
			return ErrInvalidTransition
		}
		if in.PolicyVersion != s.cfg.PolicyVersions[PolicyGuardian] {
			return withField(ErrPolicyOutdated, "policyVersion")
		}
		now := s.clock()
		tag, err := tx.Exec(ctx, `UPDATE guardian_consents SET withdrawn_at = $2, withdrawn_by = 'user' WHERE user_id = $1 AND withdrawn_at IS NULL`, owner, now)
		if err != nil {
			return err
		}
		id := newUUID()
		var ct []byte
		if name != nil {
			if ct, err = s.vault.Seal(ctx, tx, owner, guardianRef(id), []byte(*name)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO guardian_consents (id, user_id, relationship, method, guardian_name_ct, policy_version, granted_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, pgtype.UUID{Bytes: id, Valid: true}, owner, in.Relationship, in.Method, ct, in.PolicyVersion, now); err != nil {
			return err
		}
		detail := map[string]any{"relationship": in.Relationship, "method": in.Method, "policyVersion": in.PolicyVersion}
		if tag.RowsAffected() > 0 {
			detail["reason"] = "replaced"
		}
		if err := s.audit(ctx, tx, owner, &current, "guardian_consent.recorded", actorUser, detail); err != nil {
			return err
		}
		rec = GuardianRecord{ID: uuidText(id), Relationship: in.Relationship, Method: in.Method, GuardianName: name,
			PolicyVersion: in.PolicyVersion, GrantedAt: ts(now)}
		return nil
	})
	return rec, err
}

// WithdrawGuardianConsent withdraws the active record; idempotent.
func (s *Store) WithdrawGuardianConsent(ctx context.Context, owner UserID, current SessionID) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, true); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE guardian_consents SET withdrawn_at = $2, withdrawn_by = 'user' WHERE user_id = $1 AND withdrawn_at IS NULL`, owner, s.clock())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return s.audit(ctx, tx, owner, &current, "guardian_consent.withdrawn", actorUser, map[string]any{"reason": "withdrawn_by_user"})
	})
}

// AuditTypes are the foundation's audit event types.
var AuditTypes = []string{
	"account.created", "account.upgraded", "auth.login_succeeded", "auth.login_failed", "auth.reauthenticated", "auth.logout",
	"auth.logout_all", "session.revoked", "password.changed", "password_login.disabled", "recovery.used", "recovery.regenerated",
	"recovery.acknowledged", "profile.locale_changed", "profile.age_band_changed", "profile.age_band_confirmed", "consent.granted",
	"consent.withdrawn", "guardian_consent.recorded", "guardian_consent.withdrawn", "export.created",
}

// AuditDetailKeys are the only keys an audit detail may hold; none is
// content.
var AuditDetailKeys = []string{"kind", "ageBand", "from", "to", "trustedDevice", "count", "reason", "remaining", "purpose",
	"policyVersion", "relationship", "method", "format", "revoked"}

// AuditEvent is one entry of the owner's audit trail.
type AuditEvent struct {
	ID             string         `json:"id"`
	At             Timestamp      `json:"at"`
	Type           string         `json:"type"`
	Actor          string         `json:"actor"`
	CurrentSession *bool          `json:"currentSession"`
	Detail         map[string]any `json:"detail"`
}

// audit appends an event. Unknown types, detail keys outside the
// allow-list, non-scalar values and strings that are not short
// identifiers (policyVersionPattern) are refused, so the trail can only
// hold enums, versions and counts, never text from a client.
func (s *Store) audit(ctx context.Context, q db.DBTX, owner UserID, sid *SessionID, typ, actor string, detail map[string]any) error {
	if !slices.Contains(AuditTypes, typ) {
		return fmt.Errorf("account: unknown audit type %q", typ)
	}
	if detail == nil {
		detail = map[string]any{}
	}
	for k, v := range detail {
		if !slices.Contains(AuditDetailKeys, k) {
			return fmt.Errorf("account: audit detail key %q is not allow-listed", k)
		}
		switch x := v.(type) {
		case string:
			if !policyVersionPattern.MatchString(x) {
				return fmt.Errorf("account: audit detail %q is not an identifier", k)
			}
		case bool, int:
		default:
			return fmt.Errorf("account: audit detail %q must be a string, bool or int", k)
		}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	var session any
	if sid != nil {
		session = *sid
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_events (id, user_id, at, type, actor, session_id, detail) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		newID(), owner, s.clock(), typ, actor, session, raw)
	return err
}

// AuditEvents pages the owner's own events, newest first.
func (s *Store) AuditEvents(ctx context.Context, owner UserID, current SessionID, page Page) ([]AuditEvent, string, error) {
	limit, err := page.limit()
	if err != nil {
		return nil, "", err
	}
	cur, err := page.cursor(s.clock())
	if err != nil {
		return nil, "", err
	}
	args := []any{owner, limit + 1}
	where := `user_id = $1`
	if cur != nil {
		where += ` AND (at, id) < ($3, $4)`
		args = append(args, cur.at, pgtype.UUID{Bytes: cur.id, Valid: true})
	}
	list, err := s.auditRows(ctx, s.pool, current, `SELECT id, at, type, actor, session_id, detail FROM audit_events WHERE `+where+
		` ORDER BY at DESC, id DESC LIMIT $2`, args...)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(list) > limit {
		last := list[limit-1]
		next = encodeCursor(last.at, last.id)
		list = list[:limit]
	}
	out := make([]AuditEvent, len(list))
	for i, x := range list {
		out[i] = x.ev
	}
	return out, next, nil
}

type auditRow struct {
	ev AuditEvent
	at time.Time
	id [16]byte
}

func (s *Store) auditRows(ctx context.Context, q db.DBTX, current SessionID, sql string, args ...any) ([]auditRow, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (auditRow, error) {
		var x auditRow
		var id pgtype.UUID
		var sid *SessionID
		var detail []byte
		if err := r.Scan(&id, &x.at, &x.ev.Type, &x.ev.Actor, &sid, &detail); err != nil {
			return x, err
		}
		x.id, x.ev.ID, x.ev.At = id.Bytes, id.String(), ts(x.at)
		if sid != nil {
			c := *sid == current
			x.ev.CurrentSession = &c
		}
		x.ev.Detail = map[string]any{}
		if err := json.Unmarshal(detail, &x.ev.Detail); err != nil {
			return x, err
		}
		return x, nil
	})
}

// Package carenet is the professional care network (D-076): partner
// organisations such as clinics, practices, hospitals and puskesmas, the
// organisation-scoped operator accounts of their administrators,
// practitioners and staff, and the verification of organisations and
// practitioner licences. It holds no user data; care requests, which carry a
// person's sealed summary, live in package account.
//
// An organisation is synthetic (simulated) unless real partners are allowed
// for the deployment. The operator who creates an organisation never
// verifies it, unless it is synthetic, and nobody verifies their own
// licence. Every mutation is written to admin_audit in the same transaction.
package carenet

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the store.
var (
	ErrNotFound  = errors.New("carenet: not found")
	ErrInvalid   = errors.New("carenet: invalid input")
	ErrForbidden = errors.New("carenet: not allowed")
	ErrConflict  = errors.New("carenet: state conflict")
)

// Organisation kinds and statuses.
var (
	orgKinds = map[string]bool{"clinic": true, "hospital": true, "puskesmas": true, "practice": true, "organization": true}
	modes    = map[string]bool{"online": true, "in_person": true}
	stages   = map[string]bool{"everyday": true, "attention": true, "support": true, "urgent": true}
)

// Org is one partner organisation as operators see it.
type Org struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"`
	Name            string     `json:"name"`
	City            string     `json:"city,omitempty"`
	Description     string     `json:"description,omitempty"`
	Modes           []string   `json:"modes"`
	Stages          []string   `json:"stages"`
	Synthetic       bool       `json:"synthetic"`
	Status          string     `json:"status"`
	AcceptsRequests bool       `json:"acceptsRequests"`
	CreatedAt       time.Time  `json:"createdAt"`
	VerifiedAt      *time.Time `json:"verifiedAt,omitempty"`
	Members         int        `json:"members"`
}

// OrgInput creates an organisation.
type OrgInput struct {
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	City        string   `json:"city"`
	Description string   `json:"description"`
	Modes       []string `json:"modes"`
	Stages      []string `json:"stages"`
	Synthetic   bool     `json:"synthetic"`
}

// Member is one partner account of an organisation, without secrets.
type Member struct {
	Username          string     `json:"username"`
	Role              string     `json:"role"`
	DisplayName       string     `json:"displayName,omitempty"`
	ProfessionalType  string     `json:"professionalType,omitempty"`
	LicenseNo         string     `json:"licenseNo,omitempty"`
	LicenseVerified   bool       `json:"licenseVerified"`
	LicenseVerifiedAt *time.Time `json:"licenseVerifiedAt,omitempty"`
	Status            string     `json:"status"`
}

// PublicPractitioner is what a person choosing help may see about a
// practitioner: no username, licence number or account id.
type PublicPractitioner struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	ProfessionalType string `json:"professionalType"`
	LicenseVerified  bool   `json:"licenseVerified"`
}

// PublicOrg is a verified organisation that accepts care requests.
type PublicOrg struct {
	ID            string               `json:"id"`
	Kind          string               `json:"kind"`
	Name          string               `json:"name"`
	City          string               `json:"city,omitempty"`
	Description   string               `json:"description,omitempty"`
	Modes         []string             `json:"modes"`
	Stages        []string             `json:"stages"`
	Synthetic     bool                 `json:"synthetic"`
	Practitioners []PublicPractitioner `json:"practitioners"`
}

// Store reads and writes the care network.
type Store struct {
	pool      *pgxpool.Pool
	now       func() time.Time
	allowReal bool
}

// New returns a store. allowReal permits non-synthetic organisations
// (config.CareNetworkReal).
func New(pool *pgxpool.Pool, now func() time.Time, allowReal bool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("carenet: nil pool")
	}
	if now == nil {
		now = time.Now
	}
	return &Store{pool: pool, now: now, allowReal: allowReal}, nil
}

// AllowsReal reports whether real partner organisations are allowed.
func (s *Store) AllowsReal() bool { return s.allowReal }

func newID() string {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		panic(err)
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	const hex = "0123456789abcdef"
	b := make([]byte, 0, 36)
	for i, x := range u {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			b = append(b, '-')
		}
		b = append(b, hex[x>>4], hex[x&0x0f])
	}
	return string(b)
}

func cleanSet(in []string, allowed map[string]bool) ([]string, bool) {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if !allowed[v] {
			return nil, false
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, true
}

func audit(ctx context.Context, tx pgx.Tx, actor, action, kind, id, oldStatus, newStatus string, now time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO admin_audit (id, actor_id, action, object_kind, object_id, old_status, new_status, created_at)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, NULLIF($6,''), NULLIF($7,''), $8)`,
		newID(), actor, action, kind, id, oldStatus, newStatus, now)
	return err
}

// CreateOrg adds a pending organisation. A real (non-synthetic) one needs
// the deployment's acknowledgement.
func (s *Store) CreateOrg(ctx context.Context, actor string, in OrgInput) (Org, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.City = strings.TrimSpace(in.City)
	in.Description = strings.TrimSpace(in.Description)
	if !orgKinds[in.Kind] || len([]rune(in.Name)) < 2 || len([]rune(in.Name)) > 160 ||
		len([]rune(in.City)) > 120 || len([]rune(in.Description)) > 1000 {
		return Org{}, ErrInvalid
	}
	m, ok := cleanSet(in.Modes, modes)
	if !ok || len(m) == 0 {
		return Org{}, ErrInvalid
	}
	st, ok := cleanSet(in.Stages, stages)
	if !ok {
		return Org{}, ErrInvalid
	}
	if !in.Synthetic && !s.allowReal {
		return Org{}, ErrForbidden
	}
	now := s.now()
	o := Org{ID: newID(), Kind: in.Kind, Name: in.Name, City: in.City, Description: in.Description, Modes: m, Stages: st,
		Synthetic: in.Synthetic, Status: "pending", AcceptsRequests: true, CreatedAt: now}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO care_orgs (id, kind, name, city, description, modes, stages, synthetic, status,
			accepts_requests, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',true,NULLIF($9,'')::uuid,$10,$10)`,
			o.ID, o.Kind, o.Name, o.City, o.Description, o.Modes, o.Stages, o.Synthetic, actor, now); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "network.org_create", "care_org", o.ID, "", "pending", now)
	})
	if err != nil {
		return Org{}, err
	}
	return o, nil
}

// SetOrgStatus verifies or suspends an organisation. The creator may not
// verify a real organisation they created.
func (s *Store) SetOrgStatus(ctx context.Context, actor, id, status string) (Org, error) {
	if status != "verified" && status != "suspended" {
		return Org{}, ErrInvalid
	}
	now := s.now()
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var old, createdBy string
		var synthetic bool
		err := tx.QueryRow(ctx, `SELECT status, COALESCE(created_by::text,''), synthetic FROM care_orgs WHERE id::text=$1 FOR UPDATE`, id).
			Scan(&old, &createdBy, &synthetic)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if old == status {
			return ErrConflict
		}
		if status == "verified" {
			if !synthetic && createdBy != "" && createdBy == actor {
				return ErrForbidden
			}
			if !synthetic && !s.allowReal {
				return ErrForbidden
			}
			_, err = tx.Exec(ctx, `UPDATE care_orgs SET status='verified', verified_by=NULLIF($2,'')::uuid, verified_at=$3, updated_at=$3 WHERE id::text=$1`, id, actor, now)
		} else {
			_, err = tx.Exec(ctx, `UPDATE care_orgs SET status='suspended', updated_at=$2 WHERE id::text=$1`, id, now)
		}
		if err != nil {
			return err
		}
		action := "network.org_verify"
		if status == "suspended" {
			action = "network.org_suspend"
		}
		return audit(ctx, tx, actor, action, "care_org", id, old, status, now)
	})
	if err != nil {
		return Org{}, err
	}
	return s.Org(ctx, id)
}

// Org returns one organisation with its member count.
func (s *Store) Org(ctx context.Context, id string) (Org, error) {
	rows, err := s.pool.Query(ctx, orgSelect+` WHERE o.id::text=$1 GROUP BY o.id`, id)
	if err != nil {
		return Org{}, err
	}
	list, err := scanOrgs(rows)
	if err != nil {
		return Org{}, err
	}
	if len(list) == 0 {
		return Org{}, ErrNotFound
	}
	return list[0], nil
}

// Orgs lists every organisation, newest first.
func (s *Store) Orgs(ctx context.Context) ([]Org, error) {
	rows, err := s.pool.Query(ctx, orgSelect+` GROUP BY o.id ORDER BY o.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	return scanOrgs(rows)
}

const orgSelect = `SELECT o.id::text, o.kind, o.name, o.city, o.description, o.modes, o.stages, o.synthetic, o.status,
	o.accepts_requests, o.created_at, o.verified_at, count(u.id) FILTER (WHERE u.status = 'active')
	FROM care_orgs o LEFT JOIN admin_users u ON u.care_org_id = o.id`

func scanOrgs(rows pgx.Rows) ([]Org, error) {
	defer rows.Close()
	out := []Org{}
	for rows.Next() {
		var o Org
		var members int64
		if err := rows.Scan(&o.ID, &o.Kind, &o.Name, &o.City, &o.Description, &o.Modes, &o.Stages, &o.Synthetic, &o.Status,
			&o.AcceptsRequests, &o.CreatedAt, &o.VerifiedAt, &members); err != nil {
			return nil, err
		}
		o.Members = int(members)
		out = append(out, o)
	}
	return out, rows.Err()
}

// Members lists an organisation's partner accounts.
func (s *Store) Members(ctx context.Context, orgID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `SELECT username, role, COALESCE(display_name,''), COALESCE(professional_type,''),
		COALESCE(license_no,''), license_verified_at, status
		FROM admin_users WHERE care_org_id::text = $1 ORDER BY role, username`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.Username, &m.Role, &m.DisplayName, &m.ProfessionalType, &m.LicenseNo, &m.LicenseVerifiedAt, &m.Status); err != nil {
			return nil, err
		}
		m.LicenseVerified = m.LicenseVerifiedAt != nil
		out = append(out, m)
	}
	return out, rows.Err()
}

// VerifyLicense records that an operator checked a practitioner's licence
// number against its official register. Nobody verifies their own.
func (s *Store) VerifyLicense(ctx context.Context, actor, username string) error {
	now := s.now()
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var id, role, license string
		var verified *time.Time
		err := tx.QueryRow(ctx, `SELECT id::text, role, COALESCE(license_no,''), license_verified_at FROM admin_users
			WHERE username=$1 AND care_org_id IS NOT NULL FOR UPDATE`, username).Scan(&id, &role, &license, &verified)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if role != "practitioner" || license == "" {
			return ErrInvalid
		}
		if id == actor {
			return ErrForbidden
		}
		if verified != nil {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_users SET license_verified_by=NULLIF($2,'')::uuid, license_verified_at=$3 WHERE id::text=$1`, id, actor, now); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "network.license_verify", "admin_user", username, "unverified", "verified", now)
	})
}

// PublicOrgs lists verified organisations that accept requests, with
// their active practitioners. Real organisations appear only when the
// deployment allows them.
func (s *Store) PublicOrgs(ctx context.Context) ([]PublicOrg, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, kind, name, city, description, modes, stages, synthetic FROM care_orgs
		WHERE status='verified' AND accepts_requests AND (synthetic OR $1) ORDER BY synthetic, name LIMIT 100`, s.allowReal)
	if err != nil {
		return nil, err
	}
	out := []PublicOrg{}
	index := map[string]int{}
	for rows.Next() {
		var o PublicOrg
		if err := rows.Scan(&o.ID, &o.Kind, &o.Name, &o.City, &o.Description, &o.Modes, &o.Stages, &o.Synthetic); err != nil {
			rows.Close()
			return nil, err
		}
		o.Practitioners = []PublicPractitioner{}
		index[o.ID] = len(out)
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	prs, err := s.pool.Query(ctx, `SELECT id::text, care_org_id::text, display_name, professional_type, license_verified_at IS NOT NULL
		FROM admin_users WHERE role='practitioner' AND status='active' AND care_org_id IS NOT NULL ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer prs.Close()
	for prs.Next() {
		var p PublicPractitioner
		var org string
		if err := prs.Scan(&p.ID, &org, &p.DisplayName, &p.ProfessionalType, &p.LicenseVerified); err != nil {
			return nil, err
		}
		if i, ok := index[org]; ok {
			out[i].Practitioners = append(out[i].Practitioners, p)
		}
	}
	return out, prs.Err()
}

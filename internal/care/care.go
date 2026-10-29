// Package care owns the public planned-care directory and the isolated
// synthetic partner sandbox. It never stores a consumer identifier, a
// private story or an appointment: public entries are sourced metadata and
// sandbox rows are clearly fictional.
package care

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/admin"
)

// Kind is the public directory entity kind.
type Kind string

const (
	KindOrganization Kind = "organization"
	KindFacility     Kind = "facility"
	KindPractitioner Kind = "practitioner"
)

// Status is the editorial state of a directory entry.
type Status string

const (
	StatusDraft       Status = "draft"
	StatusPending     Status = "pending"
	StatusVerified    Status = "verified"
	StatusRejected    Status = "rejected"
	StatusStale       Status = "stale"
	StatusUnpublished Status = "unpublished"
)

// SandboxStatus is the fictional planned-support request state.
type SandboxStatus string

const (
	SandboxPrepared     SandboxStatus = "prepared"
	SandboxPending      SandboxStatus = "pending"
	SandboxReceived     SandboxStatus = "received"
	SandboxAcknowledged SandboxStatus = "acknowledged"
	SandboxRevoked      SandboxStatus = "revoked"
	SandboxClosed       SandboxStatus = "closed"
)

var (
	ErrNotFound  = errors.New("care: not found")
	ErrConflict  = errors.New("care: state or version conflict")
	ErrForbidden = errors.New("care: not allowed")
	ErrInvalid   = errors.New("care: invalid input")
)

// Entry is public directory metadata plus its editorial state. It holds no
// user data.
type Entry struct {
	ID               string     `json:"id"`
	Kind             Kind       `json:"kind"`
	Name             string     `json:"name"`
	City             string     `json:"city,omitempty"`
	Description      string     `json:"description,omitempty"`
	ContactURL       string     `json:"contactUrl,omitempty"`
	SourceURL        string     `json:"sourceUrl"`
	SourcePublisher  string     `json:"sourcePublisher"`
	SourceAccessedAt time.Time  `json:"sourceAccessedAt"`
	Status           Status     `json:"status"`
	Synthetic        bool       `json:"synthetic"`
	Version          int        `json:"version"`
	VerifiedAt       *time.Time `json:"verifiedAt,omitempty"`
	ReverifyAt       *time.Time `json:"reverifyAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// EntryInput is a draft create/update. Source fields are mandatory.
type EntryInput struct {
	Kind             Kind      `json:"kind"`
	Name             string    `json:"name"`
	City             string    `json:"city"`
	Description      string    `json:"description"`
	ContactURL       string    `json:"contactUrl"`
	SourceURL        string    `json:"sourceUrl"`
	SourcePublisher  string    `json:"sourcePublisher"`
	SourceAccessedAt time.Time `json:"sourceAccessedAt"`
	Synthetic        bool      `json:"synthetic"`
}

// SandboxRequest is a fictional partner workflow row.
type SandboxRequest struct {
	ID           string        `json:"id"`
	Organization string        `json:"organization"`
	Professional string        `json:"professional,omitempty"`
	Service      string        `json:"service,omitempty"`
	Status       SandboxStatus `json:"status"`
	Note         string        `json:"note,omitempty"`
	Synthetic    bool          `json:"synthetic"`
	Version      int           `json:"version"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// AuditEvent is one directory or sandbox action.
type AuditEvent struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor,omitempty"`
	Action     string    `json:"action"`
	ObjectKind string    `json:"objectKind"`
	ObjectID   string    `json:"objectId"`
	OldStatus  string    `json:"oldStatus,omitempty"`
	NewStatus  string    `json:"newStatus,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Store owns the directory tables.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// New opens the care store.
func New(pool *pgxpool.Pool, now func() time.Time) (*Store, error) {
	if pool == nil {
		return nil, errors.New("care: nil pool")
	}
	if now == nil {
		now = time.Now
	}
	return &Store{pool: pool, now: now}, nil
}

// Public returns verified entries for the anonymous care page. Synthetic
// rows are visible only when the server environment allowed them.
func (s *Store) Public(ctx context.Context, q, kind, city string, includeSynthetic bool) ([]Entry, error) {
	if kind != "" && !validKind(Kind(kind)) {
		return nil, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT id, kind, name, city, description, contact_url,
		source_url, source_publisher, source_accessed_at, status, synthetic, version,
		verified_at, reverify_at, created_at, updated_at
		FROM care_directory_entries
		WHERE status = 'verified'
		  AND (reverify_at IS NULL OR reverify_at >= $1)
		  AND ($2 OR NOT synthetic)
		  AND ($3 = '' OR kind = $3)
		  AND ($4 = '' OR city ILIKE '%' || $4 || '%')
		  AND ($5 = '' OR name ILIKE '%' || $5 || '%' OR description ILIKE '%' || $5 || '%'
		       OR source_publisher ILIKE '%' || $5 || '%')
		ORDER BY kind, name
		LIMIT 50`, s.now().UTC(), includeSynthetic, kind, strings.TrimSpace(city), strings.TrimSpace(q))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// List returns entries for editors/reviewers, newest first.
func (s *Store) List(ctx context.Context, status Status) ([]Entry, error) {
	if status != "" && !validStatus(status) {
		return nil, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT id, kind, name, city, description, contact_url,
		source_url, source_publisher, source_accessed_at, status, synthetic, version,
		verified_at, reverify_at, created_at, updated_at
		FROM care_directory_entries
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC
		LIMIT 200`, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// Create inserts a draft.
func (s *Store) Create(ctx context.Context, actor admin.AdminID, in EntryInput) (Entry, error) {
	if err := validateInput(in); err != nil {
		return Entry{}, err
	}
	id := newUUID()
	now := s.now().UTC()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO care_directory_entries
			(id, kind, name, city, description, contact_url, source_url, source_publisher,
			 source_accessed_at, status, synthetic, created_by, version, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11,1,$12,$12)`,
			id, in.Kind, strings.TrimSpace(in.Name), strings.TrimSpace(in.City), strings.TrimSpace(in.Description),
			strings.TrimSpace(in.ContactURL), strings.TrimSpace(in.SourceURL), strings.TrimSpace(in.SourcePublisher),
			in.SourceAccessedAt, in.Synthetic, actor, now)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "directory.create", "directory_entry", id.String(), "", "draft", "")
	})
	if err != nil {
		return Entry{}, err
	}
	return s.get(ctx, id)
}

// Update edits a draft or rejected entry with an optimistic version.
func (s *Store) Update(ctx context.Context, actor admin.AdminID, id string, expectedVersion int, in EntryInput) (Entry, error) {
	if err := validateInput(in); err != nil {
		return Entry{}, err
	}
	uid, err := parseUUID(id)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	now := s.now().UTC()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var status Status
		var version int
		if err := tx.QueryRow(ctx, `SELECT status, version FROM care_directory_entries WHERE id = $1 FOR UPDATE`, uid).Scan(&status, &version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if version != expectedVersion {
			return ErrConflict
		}
		if status != StatusDraft && status != StatusRejected {
			return ErrConflict
		}
		tag, err := tx.Exec(ctx, `UPDATE care_directory_entries SET
			kind=$2, name=$3, city=$4, description=$5, contact_url=$6, source_url=$7,
			source_publisher=$8, source_accessed_at=$9, synthetic=$10, version=version+1, updated_at=$11
			WHERE id=$1 AND version=$12`,
			uid, in.Kind, strings.TrimSpace(in.Name), strings.TrimSpace(in.City), strings.TrimSpace(in.Description),
			strings.TrimSpace(in.ContactURL), strings.TrimSpace(in.SourceURL), strings.TrimSpace(in.SourcePublisher),
			in.SourceAccessedAt, in.Synthetic, now, expectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		return s.audit(ctx, tx, actor, "directory.update", "directory_entry", id, string(status), string(status), "")
	})
	if err != nil {
		return Entry{}, err
	}
	return s.get(ctx, uid)
}

// Submit moves a draft or rejected entry to the review queue.
func (s *Store) Submit(ctx context.Context, actor admin.AdminID, id string, expectedVersion int) (Entry, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	return s.transition(ctx, actor, uid, expectedVersion, "directory.submit", []Status{StatusDraft, StatusRejected}, StatusPending, "")
}

// Verify publishes a pending entry. A real entry needs a reviewer other than
// its creator; a clearly synthetic demo entry may be self-verified.
func (s *Store) Verify(ctx context.Context, actor admin.AdminID, id string, expectedVersion int, reverifyAt time.Time) (Entry, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	if reverifyAt.IsZero() {
		reverifyAt = s.now().UTC().AddDate(0, 3, 0)
	}
	now := s.now().UTC()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var status Status
		var version int
		var creator [16]byte
		var synthetic bool
		if err := tx.QueryRow(ctx, `SELECT status, version, created_by, synthetic FROM care_directory_entries WHERE id=$1 FOR UPDATE`, uid).Scan(&status, &version, &creator, &synthetic); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if version != expectedVersion {
			return ErrConflict
		}
		if status != StatusPending {
			return ErrConflict
		}
		if !synthetic && creator == [16]byte(actor) {
			return ErrForbidden
		}
		tag, err := tx.Exec(ctx, `UPDATE care_directory_entries SET
			status='verified', verified_by=$2, verified_at=$3, reverify_at=$4,
			version=version+1, updated_at=$3 WHERE id=$1 AND version=$5`,
			uid, actor, now, reverifyAt, expectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		return s.audit(ctx, tx, actor, "directory.verify", "directory_entry", id, string(status), "verified", "")
	})
	if err != nil {
		return Entry{}, err
	}
	return s.get(ctx, uid)
}

// Reject keeps a pending entry out of the public directory.
func (s *Store) Reject(ctx context.Context, actor admin.AdminID, id string, expectedVersion int, reason string) (Entry, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	return s.transition(ctx, actor, uid, expectedVersion, "directory.reject", []Status{StatusPending}, StatusRejected, reason)
}

// Unpublish removes a verified entry from the public directory.
func (s *Store) Unpublish(ctx context.Context, actor admin.AdminID, id string, expectedVersion int, reason string) (Entry, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return Entry{}, ErrNotFound
	}
	return s.transition(ctx, actor, uid, expectedVersion, "directory.unpublish", []Status{StatusVerified, StatusStale}, StatusUnpublished, reason)
}

func (s *Store) transition(ctx context.Context, actor admin.AdminID, id pgtype.UUID, expectedVersion int, action string, from []Status, to Status, reason string) (Entry, error) {
	now := s.now().UTC()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var status Status
		var version int
		if err := tx.QueryRow(ctx, `SELECT status, version FROM care_directory_entries WHERE id=$1 FOR UPDATE`, id).Scan(&status, &version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if version != expectedVersion {
			return ErrConflict
		}
		allowed := false
		for _, f := range from {
			if status == f {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrConflict
		}
		tag, err := tx.Exec(ctx, `UPDATE care_directory_entries SET status=$2, version=version+1, updated_at=$3 WHERE id=$1 AND version=$4`, id, to, now, expectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		idText := pgtype.UUID{Bytes: id.Bytes, Valid: true}.String()
		return s.audit(ctx, tx, actor, action, "directory_entry", idText, string(status), string(to), reason)
	})
	if err != nil {
		return Entry{}, err
	}
	return s.get(ctx, id)
}

// CreateSandbox inserts a clearly fictional planned-support request.
func (s *Store) CreateSandbox(ctx context.Context, actor admin.AdminID, organization, professional, service, note string) (SandboxRequest, error) {
	organization, professional, service, note = strings.TrimSpace(organization), strings.TrimSpace(professional), strings.TrimSpace(service), strings.TrimSpace(note)
	if organization == "" || len(organization) > 160 || len(professional) > 160 || len(service) > 200 || len(note) > 1000 {
		return SandboxRequest{}, ErrInvalid
	}
	id := newUUID()
	now := s.now().UTC()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO care_sandbox_requests
			(id, organization, professional, service, status, note, created_by, version, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'prepared',$5,$6,1,$7,$7)`, id, organization, professional, service, note, actor, now)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "sandbox.create", "sandbox_request", id.String(), "", "prepared", "")
	})
	if err != nil {
		return SandboxRequest{}, err
	}
	return s.getSandbox(ctx, id)
}

// ListSandbox returns fictional requests, newest first.
func (s *Store) ListSandbox(ctx context.Context) ([]SandboxRequest, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, organization, professional, service, status, note, synthetic, version, created_at, updated_at
		FROM care_sandbox_requests ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxRequest
	for rows.Next() {
		var v SandboxRequest
		var id pgtype.UUID
		if err := rows.Scan(&id, &v.Organization, &v.Professional, &v.Service, &v.Status, &v.Note, &v.Synthetic, &v.Version, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		v.ID = id.String()
		out = append(out, v)
	}
	return out, rows.Err()
}

// TransitionSandbox moves a fictional request through the sandbox state
// machine. It never calls a partner, sends an email or creates a real case.
func (s *Store) TransitionSandbox(ctx context.Context, actor admin.AdminID, id string, expectedVersion int, next SandboxStatus, note string) (SandboxRequest, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return SandboxRequest{}, ErrNotFound
	}
	allowed := map[SandboxStatus][]SandboxStatus{
		SandboxPrepared:     {SandboxPending, SandboxRevoked},
		SandboxPending:      {SandboxReceived, SandboxRevoked},
		SandboxReceived:     {SandboxAcknowledged, SandboxRevoked},
		SandboxAcknowledged: {SandboxClosed, SandboxRevoked},
	}
	now := s.now().UTC()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var status SandboxStatus
		var version int
		if err := tx.QueryRow(ctx, `SELECT status, version FROM care_sandbox_requests WHERE id=$1 FOR UPDATE`, uid).Scan(&status, &version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if version != expectedVersion {
			return ErrConflict
		}
		ok := false
		for _, candidate := range allowed[status] {
			if candidate == next {
				ok = true
				break
			}
		}
		if !ok {
			return ErrConflict
		}
		tag, err := tx.Exec(ctx, `UPDATE care_sandbox_requests SET status=$2, note=$3, version=version+1, updated_at=$4 WHERE id=$1 AND version=$5`, uid, next, strings.TrimSpace(note), now, expectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		return s.audit(ctx, tx, actor, "sandbox.transition", "sandbox_request", id, string(status), string(next), note)
	})
	if err != nil {
		return SandboxRequest{}, err
	}
	return s.getSandbox(ctx, uid)
}

// Audit returns recent directory/sandbox events without user content.
func (s *Store) Audit(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, COALESCE(actor_id::text,''), action, object_kind, object_id,
		COALESCE(old_status,''), COALESCE(new_status,''), COALESCE(reason,''), created_at
		FROM care_audit ORDER BY created_at DESC LIMIT $1`, limit)
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

// SeedDemo inserts clearly synthetic entries and sandbox requests for a
// local demo. It is idempotent while synthetic rows already exist.
func (s *Store) SeedDemo(ctx context.Context, actor admin.AdminID) error {
	var existing int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM care_directory_entries WHERE synthetic`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	now := s.now().UTC()
	entries := []EntryInput{
		{Kind: KindOrganization, Name: "Ruang Pulih (simulasi)", City: "Surabaya", Description: "Contoh organisasi pendampingan non-klinis untuk demo. Bukan fasilitas nyata.", ContactURL: "https://example.org/ruang-pulih", SourceURL: "https://example.org/sumber-simulasi", SourcePublisher: "Sumber simulasi", SourceAccessedAt: now, Synthetic: true},
		{Kind: KindFacility, Name: "Klinik Teduh (simulasi)", City: "Bandung", Description: "Contoh fasilitas layanan konseling untuk demo. Bukan fasilitas nyata.", ContactURL: "https://example.org/klinik-teduh", SourceURL: "https://example.org/sumber-simulasi", SourcePublisher: "Sumber simulasi", SourceAccessedAt: now, Synthetic: true},
		{Kind: KindPractitioner, Name: "Praktisi Contoh (simulasi)", City: "Jakarta", Description: "Contoh profil praktisi untuk demo. Bukan orang nyata dan tidak ada kredensial yang diklaim.", ContactURL: "https://example.org/praktisi-contoh", SourceURL: "https://example.org/sumber-simulasi", SourcePublisher: "Sumber simulasi", SourceAccessedAt: now, Synthetic: true},
	}
	for _, in := range entries {
		id := newUUID()
		if _, err := s.pool.Exec(ctx, `INSERT INTO care_directory_entries
			(id, kind, name, city, description, contact_url, source_url, source_publisher,
			 source_accessed_at, status, synthetic, created_by, verified_by, verified_at, reverify_at,
			 version, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'verified',true,$10,$10,$11,$12,1,$11,$11)`,
			id, in.Kind, in.Name, in.City, in.Description, in.ContactURL, in.SourceURL, in.SourcePublisher,
			in.SourceAccessedAt, actor, now, now.AddDate(0, 3, 0)); err != nil {
			return err
		}
	}
	requests := []struct{ org, professional, service, note string }{
		{"Ruang Pulih (simulasi)", "Praktisi Contoh (simulasi)", "Konseling awal", "Permintaan fiktif untuk mendemokan status, bukan janji temu."},
		{"Klinik Teduh (simulasi)", "", "Konsultasi umum", "Contoh alur yang tidak dikirim ke fasilitas mana pun."},
	}
	for _, r := range requests {
		id := newUUID()
		if _, err := s.pool.Exec(ctx, `INSERT INTO care_sandbox_requests
			(id, organization, professional, service, status, note, created_by, version, created_at, updated_at)
			VALUES ($1,$2,$3,$4,'prepared',$5,$6,1,$7,$7)`, id, r.org, r.professional, r.service, r.note, actor, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) get(ctx context.Context, id pgtype.UUID) (Entry, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, kind, name, city, description, contact_url,
		source_url, source_publisher, source_accessed_at, status, synthetic, version,
		verified_at, reverify_at, created_at, updated_at FROM care_directory_entries WHERE id=$1`, id)
	return scanEntry(row)
}

func (s *Store) getSandbox(ctx context.Context, id pgtype.UUID) (SandboxRequest, error) {
	var v SandboxRequest
	err := s.pool.QueryRow(ctx, `SELECT id::text, organization, professional, service, status, note, synthetic, version, created_at, updated_at
		FROM care_sandbox_requests WHERE id=$1`, id).Scan(&v.ID, &v.Organization, &v.Professional, &v.Service, &v.Status, &v.Note, &v.Synthetic, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	return v, err
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

func (s *Store) audit(ctx context.Context, tx pgx.Tx, actor admin.AdminID, action, kind, id, old, new, reason string) error {
	if len(reason) > 300 {
		reason = reason[:300]
	}
	_, err := tx.Exec(ctx, `INSERT INTO care_audit
		(id, actor_id, action, object_kind, object_id, old_status, new_status, reason, created_at)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9)`,
		newUUID(), actor, action, kind, id, old, new, reason, s.now().UTC())
	return err
}

type entryScanner interface{ Scan(...any) error }

func scanEntry(row entryScanner) (Entry, error) {
	var e Entry
	var id pgtype.UUID
	var sourceAccessed time.Time
	var verifiedAt, reverify *time.Time
	err := row.Scan(&id, &e.Kind, &e.Name, &e.City, &e.Description, &e.ContactURL, &e.SourceURL,
		&e.SourcePublisher, &sourceAccessed, &e.Status, &e.Synthetic, &e.Version,
		&verifiedAt, &reverify, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	if err != nil {
		return Entry{}, err
	}
	e.ID = id.String()
	e.SourceAccessedAt = sourceAccessed
	e.VerifiedAt = verifiedAt
	e.ReverifyAt = reverify
	return e, nil
}

func scanEntries(rows pgx.Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func validKind(k Kind) bool {
	return k == KindOrganization || k == KindFacility || k == KindPractitioner
}

func validStatus(s Status) bool {
	switch s {
	case StatusDraft, StatusPending, StatusVerified, StatusRejected, StatusStale, StatusUnpublished:
		return true
	}
	return false
}

func validateInput(in EntryInput) error {
	if !validKind(in.Kind) {
		return fmt.Errorf("%w: kind", ErrInvalid)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 160 {
		return fmt.Errorf("%w: name", ErrInvalid)
	}
	if len(strings.TrimSpace(in.City)) > 120 || len(strings.TrimSpace(in.Description)) > 2000 || len(strings.TrimSpace(in.ContactURL)) > 2048 {
		return fmt.Errorf("%w: field length", ErrInvalid)
	}
	if !validHTTPURL(in.SourceURL) || len(strings.TrimSpace(in.SourcePublisher)) == 0 || len(strings.TrimSpace(in.SourcePublisher)) > 160 {
		return fmt.Errorf("%w: source", ErrInvalid)
	}
	if in.ContactURL != "" && !validHTTPURL(in.ContactURL) {
		return fmt.Errorf("%w: contact URL", ErrInvalid)
	}
	if in.SourceAccessedAt.IsZero() {
		return fmt.Errorf("%w: source date", ErrInvalid)
	}
	return nil
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil || !u.Valid {
		return pgtype.UUID{}, ErrNotFound
	}
	return u, nil
}

func newUUID() pgtype.UUID {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	return pgtype.UUID{Bytes: id, Valid: true}
}

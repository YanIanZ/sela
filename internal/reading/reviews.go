package reading

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/admin"
)

// Approval is one current (or most recently revoked) operator sign-off. The
// public API never serves ReviewerName, Note or the operator id.
type Approval struct {
	Slug           string     `json:"slug"`
	ReviewerName   string     `json:"reviewerName"`
	ReviewedOn     string     `json:"reviewedOn"`
	Note           string     `json:"note,omitempty"`
	VerifierStatus string     `json:"verifierStatus"`
	ContentHash    string     `json:"contentHash"`
	ApprovedBy     string     `json:"approvedBy"`
	ApprovedAt     time.Time  `json:"approvedAt"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
	Version        int        `json:"version"`
}

// publishes reports whether this approval still binds the exact embedded
// content and satisfies the non-negotiable evidence conditions.
func (a Approval) publishes(contentHash string) bool {
	if a.RevokedAt != nil {
		return false
	}
	if a.ContentHash != contentHash || a.VerifierStatus != "supported" {
		return false
	}
	if strings.TrimSpace(a.ReviewerName) == "" || !validReviewDate(a.ReviewedOn) {
		return false
	}
	return true
}

// ApproveInput is one human sign-off. The operator must state the reviewer
// name, the review date and the verifier verdict; nothing is inferred from
// the embedded file (which is currently unreviewed).
type ApproveInput struct {
	ReviewerName   string
	ReviewedOn     time.Time
	Note           string
	VerifierStatus string
	ContentHash    string
}

// AuditEvent is one reading review action, content-free.
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

// Review errors are deliberately generic.
var (
	ErrReviewNotFound = errors.New("reading: no review")
	ErrReviewInvalid  = errors.New("reading: invalid review")
)

// ReviewStore owns the operator approvals and their append-only audit.
type ReviewStore struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewReviewStore opens the reading review store.
func NewReviewStore(pool *pgxpool.Pool, now func() time.Time) (*ReviewStore, error) {
	if pool == nil {
		return nil, errors.New("reading: nil pool")
	}
	if now == nil {
		now = time.Now
	}
	return &ReviewStore{pool: pool, now: now}, nil
}

// List returns every approval row, current or revoked, oldest slug first.
func (s *ReviewStore) List(ctx context.Context) ([]Approval, error) {
	rows, err := s.pool.Query(ctx, `SELECT slug, reviewer_name, reviewed_on, note, verifier_status,
		content_hash, approved_by, approved_at, revoked_at, version
		FROM reading_reviews ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Approval{}
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Approve records a new current approval for slug. Re-approving a revoked or
// stale row replaces it and increments version.
func (s *ReviewStore) Approve(ctx context.Context, actor admin.AdminID, slug string, in ApproveInput) (Approval, error) {
	if err := validateApproval(slug, in); err != nil {
		return Approval{}, err
	}
	now := s.now().UTC()
	if in.ReviewedOn.Format(time.DateOnly) > now.Format(time.DateOnly) {
		return Approval{}, fmt.Errorf("%w: reviewedOn", ErrReviewInvalid)
	}
	reviewedOn := in.ReviewedOn.Format(time.DateOnly)
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var oldStatus string
		var revokedAt *time.Time
		err := tx.QueryRow(ctx, `SELECT revoked_at FROM reading_reviews WHERE slug=$1 FOR UPDATE`, slug).Scan(&revokedAt)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			oldStatus = "none"
		case err != nil:
			return err
		case revokedAt != nil:
			oldStatus = "revoked"
		default:
			oldStatus = "approved"
		}
		_, err = tx.Exec(ctx, `INSERT INTO reading_reviews
			(slug, reviewer_name, reviewed_on, note, verifier_status, content_hash,
			 approved_by, approved_at, revoked_at, version)
			VALUES ($1,$2,$3,$4,'supported',$5,$6,$7,NULL,1)
			ON CONFLICT (slug) DO UPDATE SET
				reviewer_name=EXCLUDED.reviewer_name,
				reviewed_on=EXCLUDED.reviewed_on,
				note=EXCLUDED.note,
				verifier_status=EXCLUDED.verifier_status,
				content_hash=EXCLUDED.content_hash,
				approved_by=EXCLUDED.approved_by,
				approved_at=EXCLUDED.approved_at,
				revoked_at=NULL,
				version=reading_reviews.version+1`,
			slug, strings.TrimSpace(in.ReviewerName), reviewedOn, strings.TrimSpace(in.Note),
			in.ContentHash, actor, now)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, actor, "reading.approve", slug, oldStatus, "approved", in.Note)
	})
	if err != nil {
		return Approval{}, err
	}
	return s.get(ctx, slug)
}

// Revoke ends publication at once. It is idempotent: revoking an already
// revoked row returns the row without a second audit event. A slug that was
// never approved is a not-found.
func (s *ReviewStore) Revoke(ctx context.Context, actor admin.AdminID, slug, reason string) (Approval, error) {
	if err := validateSlug(slug); err != nil {
		return Approval{}, err
	}
	now := s.now().UTC()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		a, err := scanApproval(tx.QueryRow(ctx, `SELECT slug, reviewer_name, reviewed_on, note, verifier_status,
			content_hash, approved_by, approved_at, revoked_at, version
			FROM reading_reviews WHERE slug=$1 FOR UPDATE`, slug))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReviewNotFound
		}
		if err != nil {
			return err
		}
		if a.RevokedAt != nil {
			return nil
		}
		tag, err := tx.Exec(ctx, `UPDATE reading_reviews SET revoked_at=$2, version=version+1 WHERE slug=$1 AND revoked_at IS NULL`, slug, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrReviewNotFound
		}
		return s.audit(ctx, tx, actor, "reading.revoke", slug, "approved", "revoked", reason)
	})
	if err != nil {
		return Approval{}, err
	}
	return s.get(ctx, slug)
}

// Audit returns the newest reading review events without user content.
func (s *ReviewStore) Audit(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT a.id::text, COALESCE(u.username,''), a.action, a.object_kind, a.object_id,
		COALESCE(a.old_status,''), COALESCE(a.new_status,''), COALESCE(a.reason,''), a.created_at
		FROM reading_review_audit a LEFT JOIN admin_users u ON u.id = a.actor_id
		ORDER BY a.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.ObjectKind, &e.ObjectID,
			&e.OldStatus, &e.NewStatus, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *ReviewStore) get(ctx context.Context, slug string) (Approval, error) {
	a, err := scanApproval(s.pool.QueryRow(ctx, `SELECT slug, reviewer_name, reviewed_on, note, verifier_status,
		content_hash, approved_by, approved_at, revoked_at, version
		FROM reading_reviews WHERE slug=$1`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return Approval{}, ErrReviewNotFound
	}
	return a, err
}

func (s *ReviewStore) tx(ctx context.Context, fn func(pgx.Tx) error) error {
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

func (s *ReviewStore) audit(ctx context.Context, tx pgx.Tx, actor admin.AdminID, action, slug, old, new, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) > 300 {
		reason = reason[:300]
	}
	_, err := tx.Exec(ctx, `INSERT INTO reading_review_audit
		(id, actor_id, action, object_kind, object_id, old_status, new_status, reason, created_at)
		VALUES ($1,$2,$3,'reading_item',$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8)`,
		newUUID(), actor, action, slug, old, new, reason, s.now().UTC())
	return err
}

type approvalScanner interface{ Scan(...any) error }

func scanApproval(row approvalScanner) (Approval, error) {
	var (
		a          Approval
		reviewedOn time.Time
		approvedBy pgtype.UUID
		revokedAt  *time.Time
	)
	if err := row.Scan(&a.Slug, &a.ReviewerName, &reviewedOn, &a.Note, &a.VerifierStatus,
		&a.ContentHash, &approvedBy, &a.ApprovedAt, &revokedAt, &a.Version); err != nil {
		return Approval{}, err
	}
	a.ReviewedOn = reviewedOn.Format(time.DateOnly)
	a.ApprovedBy = approvedBy.String()
	a.RevokedAt = revokedAt
	return a, nil
}

func validateApproval(slug string, in ApproveInput) error {
	if err := validateSlug(slug); err != nil {
		return err
	}
	name := strings.TrimSpace(in.ReviewerName)
	if name == "" || len(name) > 160 {
		return fmt.Errorf("%w: reviewerName", ErrReviewInvalid)
	}
	if in.ReviewedOn.IsZero() {
		return fmt.Errorf("%w: reviewedOn", ErrReviewInvalid)
	}
	if in.VerifierStatus != "supported" {
		return fmt.Errorf("%w: verifierStatus", ErrReviewInvalid)
	}
	if !validHash(in.ContentHash) {
		return fmt.Errorf("%w: contentHash", ErrReviewInvalid)
	}
	if len(strings.TrimSpace(in.Note)) > 1000 {
		return fmt.Errorf("%w: note", ErrReviewInvalid)
	}
	return nil
}

func validateSlug(slug string) error {
	if strings.TrimSpace(slug) == "" || len(slug) > 120 {
		return fmt.Errorf("%w: slug", ErrReviewInvalid)
	}
	return nil
}

func validHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
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

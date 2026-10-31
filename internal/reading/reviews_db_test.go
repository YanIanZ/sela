package reading

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

func newReviewStore(t *testing.T) (*ReviewStore, *time.Time) {
	t.Helper()
	db := dbtest.New(t)
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	s, err := NewReviewStore(db.Pool, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return s, &now
}

func reviewActor(n byte) admin.AdminID {
	var a admin.AdminID
	a[0] = n
	return a
}

func approveInput(hash string) ApproveInput {
	return ApproveInput{
		ReviewerName:   "Editor Manusia",
		ReviewedOn:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Note:           "read the current text",
		VerifierStatus: "supported",
		ContentHash:    hash,
	}
}

func TestReviewStoreApproveRevokeAndAudit(t *testing.T) {
	s, now := newReviewStore(t)
	ctx := context.Background()
	actor := reviewActor(1)
	hash := "a" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde"

	a, err := s.Approve(ctx, actor, "draft-one", approveInput(hash))
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug != "draft-one" || a.ReviewerName != "Editor Manusia" || a.ReviewedOn != "2026-11-01" ||
		a.VerifierStatus != "supported" || a.ContentHash != hash || a.Version != 1 || a.RevokedAt != nil {
		t.Fatalf("approval = %+v", a)
	}
	if a.ApprovedBy != actor.String() || a.ApprovedAt.IsZero() {
		t.Fatalf("operator/time not recorded: %+v", a)
	}

	*now = now.Add(time.Minute)
	a, err = s.Approve(ctx, actor, "draft-one", approveInput(hash))
	if err != nil || a.Version != 2 {
		t.Fatalf("re-approve = %+v, %v", a, err)
	}

	rows, err := s.List(ctx)
	if err != nil || len(rows) != 1 || rows[0].Slug != "draft-one" {
		t.Fatalf("list = %+v, %v", rows, err)
	}

	*now = now.Add(time.Minute)
	a, err = s.Revoke(ctx, actor, "draft-one", "text withdrawn")
	if err != nil {
		t.Fatal(err)
	}
	if a.RevokedAt == nil || a.Version != 3 {
		t.Fatalf("revoked = %+v", a)
	}
	// Revoking twice is idempotent and does not append a second event.
	a2, err := s.Revoke(ctx, actor, "draft-one", "again")
	if err != nil || a2.Version != a.Version || a2.RevokedAt == nil || !a2.RevokedAt.Equal(*a.RevokedAt) {
		t.Fatalf("second revoke = %+v, %v", a2, err)
	}

	events, err := s.Audit(ctx, 10)
	if err != nil || len(events) != 3 {
		t.Fatalf("audit = %+v, %v", events, err)
	}
	if events[0].Action != "reading.revoke" || events[0].NewStatus != "revoked" || events[0].ObjectID != "draft-one" {
		t.Fatalf("newest event = %+v", events[0])
	}
	approved := 0
	for _, e := range events {
		if e.Action == "reading.approve" && e.NewStatus == "approved" {
			approved++
		}
	}
	if approved != 2 {
		t.Fatalf("approve events = %d, want 2: %+v", approved, events)
	}

	if _, err := s.Revoke(ctx, actor, "never-approved", ""); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("revoking an unknown slug: %v", err)
	}
}

func TestReviewStoreRefusesIncompleteApproval(t *testing.T) {
	s, _ := newReviewStore(t)
	ctx := context.Background()
	actor := reviewActor(2)
	hash := "b" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde"
	cases := map[string]func(*ApproveInput){
		"blank reviewer":         func(in *ApproveInput) { in.ReviewerName = "  " },
		"future review date":     func(in *ApproveInput) { in.ReviewedOn = time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) },
		"verifier not supported": func(in *ApproveInput) { in.VerifierStatus = "not_run" },
		"bad hash":               func(in *ApproveInput) { in.ContentHash = "not-a-hash" },
		"bad slug":               func(in *ApproveInput) {},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := approveInput(hash)
			slug := "draft-one"
			if name == "bad slug" {
				slug = " "
			}
			mutate(&in)
			if _, err := s.Approve(ctx, actor, slug, in); !errors.Is(err, ErrReviewInvalid) {
				t.Fatalf("Approve(%s) = %v, want ErrReviewInvalid", name, err)
			}
		})
	}
	rows, _ := s.List(ctx)
	if len(rows) != 0 {
		t.Fatalf("invalid approvals were stored: %+v", rows)
	}
}

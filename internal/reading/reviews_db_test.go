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

// passRecord is the accessibility record of a clean fixture, as
// GateAccessibility returns it after a full checklist.
func passRecord() *ApprovalAccessibility {
	lib, err := LoadFS(fixtureFS(unreviewedItem("clean")))
	if err != nil {
		panic(err)
	}
	report, _, _ := lib.Accessibility("clean")
	rec, err := GateAccessibility(report, &A11yRequest{
		ChecksVersion: A11yChecksVersion, ManualVersion: A11yManualVersion, ManualChecks: fullChecklist(),
	})
	if err != nil {
		panic(err)
	}
	return &rec
}

func approveInput(hash string) ApproveInput {
	return ApproveInput{
		ReviewerName:   "Editor Manusia",
		ReviewedOn:     time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Note:           "read the current text",
		VerifierStatus: "supported",
		ContentHash:    hash,
		Accessibility:  passRecord(),
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
		"blank reviewer":          func(in *ApproveInput) { in.ReviewerName = "  " },
		"future review date":      func(in *ApproveInput) { in.ReviewedOn = time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) },
		"verifier not supported":  func(in *ApproveInput) { in.VerifierStatus = "not_run" },
		"bad hash":                func(in *ApproveInput) { in.ContentHash = "not-a-hash" },
		"bad slug":                func(in *ApproveInput) {},
		"no accessibility record": func(in *ApproveInput) { in.Accessibility = nil },
		"partial checklist": func(in *ApproveInput) {
			rec := *in.Accessibility
			rec.ManualConfirmed = rec.ManualConfirmed[:1]
			in.Accessibility = &rec
		},
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

func TestReviewStoreRecordsAccessibilityAndOverride(t *testing.T) {
	s, _ := newReviewStore(t)
	ctx := context.Background()
	actor := reviewActor(3)
	hash := "c" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde"

	a, err := s.Approve(ctx, actor, "clean-one", approveInput(hash))
	if err != nil {
		t.Fatal(err)
	}
	rec := a.Accessibility
	if rec == nil || rec.ChecksVersion != A11yChecksVersion || rec.Result != A11yApprovedPass ||
		rec.ManualVersion != A11yManualVersion || len(rec.ManualConfirmed) != len(ManualChecklist) ||
		len(rec.Checks) != 9 || rec.OverrideReason != "" {
		t.Fatalf("stored accessibility = %+v", rec)
	}

	lib, err := LoadFS(fixtureFS(a11yFixture("long", map[string]any{"summary": loc(
		"Kalimat ini sengaja dibuat sangat panjang supaya jumlah katanya melewati batas dua puluh kata yang ditetapkan oleh panduan gaya konten Sela.",
		"Short.")}, "")))
	if err != nil {
		t.Fatal(err)
	}
	report, _, _ := lib.Accessibility("long")
	reason := "Kalimat bantuan darurat dibiarkan utuh agar nomor 119 tidak terpisah."
	override, err := GateAccessibility(report, &A11yRequest{ChecksVersion: A11yChecksVersion,
		ManualVersion: A11yManualVersion, ManualChecks: fullChecklist(), OverrideReason: reason})
	if err != nil {
		t.Fatal(err)
	}
	in := approveInput(hash)
	in.Accessibility = &override
	a, err = s.Approve(ctx, actor, "long-one", in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Accessibility == nil || a.Accessibility.Result != A11yApprovedOverridden ||
		a.Accessibility.OverrideReason != reason || len(a.Accessibility.Overridden) != 1 ||
		a.Accessibility.Overridden[0] != "sentence_length" {
		t.Fatalf("override record = %+v", a.Accessibility)
	}
	events, err := s.Audit(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	var overrides int
	for _, e := range events {
		if e.Action == "reading.a11y_override" {
			overrides++
			if e.ObjectID != "long-one" || e.Reason != reason || e.OldStatus != "sentence_length" || e.NewStatus != "overridden" {
				t.Fatalf("override event = %+v", e)
			}
		}
	}
	if overrides != 1 || len(events) != 3 {
		t.Fatalf("events = %+v", events)
	}

	// The database refuses an inconsistent record even if the application
	// check were bypassed: an override result without a reason.
	_, err = s.pool.Exec(ctx, `UPDATE reading_reviews SET a11y_override_reason = NULL WHERE slug = 'long-one'`)
	if err == nil {
		t.Fatal("database accepted an override without a reason")
	}
	// A pre-1301 style row (no accessibility record) still lists and revokes.
	_, err = s.pool.Exec(ctx, `INSERT INTO reading_reviews (slug, reviewer_name, reviewed_on, verifier_status,
		content_hash, approved_by, approved_at) VALUES ('legacy-one','Editor','2026-10-31','supported',$1,$2,now())`,
		hash, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Revoke(ctx, actor, "legacy-one", "legacy"); err != nil {
		t.Fatalf("revoking a legacy approval: %v", err)
	}
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Slug == "legacy-one" && r.Accessibility != nil {
			t.Fatalf("legacy row reports an accessibility record: %+v", r)
		}
	}
}

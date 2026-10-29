package care

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

var bg = context.Background()

func newTestStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	db := dbtest.New(t)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	s, err := New(db.Pool, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return s, &now
}

func testActor(n byte) admin.AdminID {
	var a admin.AdminID
	a[0] = n
	return a
}

func testInput(synthetic bool) EntryInput {
	return EntryInput{
		Kind: KindOrganization, Name: "Ruang Contoh", City: "Bandung",
		Description: "Contoh sumber publik untuk pengujian.", ContactURL: "https://example.org/contact",
		SourceURL: "https://example.org/source", SourcePublisher: "Sumber Contoh",
		SourceAccessedAt: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Synthetic: synthetic,
	}
}

func TestDirectoryEditorialFlowAndPublicFilter(t *testing.T) {
	s, _ := newTestStore(t)
	author := testActor(1)
	reviewer := testActor(2)

	e, err := s.Create(bg, author, testInput(false))
	if err != nil || e.Status != StatusDraft || e.Version != 1 {
		t.Fatalf("create = %+v, %v", e, err)
	}
	if _, err := s.Public(bg, "", "", "", true); err != nil {
		t.Fatal(err)
	}
	pub, _ := s.Public(bg, "", "", "", true)
	if len(pub) != 0 {
		t.Fatal("draft leaked into public directory")
	}

	e, err = s.Submit(bg, author, e.ID, e.Version)
	if err != nil || e.Status != StatusPending {
		t.Fatalf("submit = %+v, %v", e, err)
	}
	if _, err := s.Verify(bg, author, e.ID, e.Version, time.Time{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("self-verify real entry: %v", err)
	}
	e, err = s.Verify(bg, reviewer, e.ID, e.Version, time.Date(2027, 1, 30, 0, 0, 0, 0, time.UTC))
	if err != nil || e.Status != StatusVerified || e.ReverifyAt == nil {
		t.Fatalf("verify = %+v, %v", e, err)
	}
	pub, _ = s.Public(bg, "Ruang", "organization", "Bandung", false)
	if len(pub) != 1 || pub[0].ID != e.ID {
		t.Fatalf("public = %+v", pub)
	}
	if _, err := s.Unpublish(bg, reviewer, e.ID, e.Version, "withdrawn"); err != nil {
		t.Fatal(err)
	}
	pub, _ = s.Public(bg, "", "", "", true)
	if len(pub) != 0 {
		t.Fatal("unpublished entry stayed public")
	}
}

func TestSyntheticEntryCanBeSelfVerifiedAndIncludeFlag(t *testing.T) {
	s, _ := newTestStore(t)
	author := testActor(1)
	e, err := s.Create(bg, author, testInput(true))
	if err != nil {
		t.Fatal(err)
	}
	e, _ = s.Submit(bg, author, e.ID, e.Version)
	e, err = s.Verify(bg, author, e.ID, e.Version, time.Time{})
	if err != nil || !e.Synthetic {
		t.Fatalf("synthetic verify = %+v, %v", e, err)
	}
	if pub, _ := s.Public(bg, "", "", "", false); len(pub) != 0 {
		t.Fatal("synthetic entry visible without the flag")
	}
	if pub, _ := s.Public(bg, "", "", "", true); len(pub) != 1 {
		t.Fatal("synthetic entry missing with the flag")
	}
}

func TestSandboxStateMachineAndSeedDemo(t *testing.T) {
	s, _ := newTestStore(t)
	actor := testActor(1)
	r, err := s.CreateSandbox(bg, actor, "Ruang Fiktif", "Praktisi Fiktif", "Konseling", "demo")
	if err != nil || r.Status != SandboxPrepared || !r.Synthetic {
		t.Fatalf("sandbox create = %+v, %v", r, err)
	}
	r, err = s.TransitionSandbox(bg, actor, r.ID, r.Version, SandboxPending, "queued")
	if err != nil || r.Status != SandboxPending {
		t.Fatalf("transition = %+v, %v", r, err)
	}
	if _, err := s.TransitionSandbox(bg, actor, r.ID, r.Version, SandboxClosed, "skipped"); !errors.Is(err, ErrConflict) {
		t.Fatalf("invalid transition: %v", err)
	}
	if err := s.SeedDemo(bg, actor); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedDemo(bg, actor); err != nil {
		t.Fatalf("idempotent seed: %v", err)
	}
	if pub, _ := s.Public(bg, "", "", "", true); len(pub) != 3 {
		t.Fatalf("seeded public = %d", len(pub))
	}
	events, err := s.Audit(bg, 20)
	if err != nil || len(events) < 2 {
		t.Fatalf("audit = %+v, %v", events, err)
	}
}

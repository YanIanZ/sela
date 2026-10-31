package account_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

// proposalSource creates one saved conversation record the proposals may
// cite, and returns it with the owner's exact words.
func proposalSource(t *testing.T, f *fixture, owner account.UserID) account.PersonalRecord {
	t.Helper()
	quote := "Synthetic quote: I write best in the morning."
	src, err := f.store.CreateRecord(ctx, owner, account.RecordInput{
		Kind: "conversation", StorageGranted: true,
		Turns: []account.ConversationTurn{{Role: "user", ReviewStatus: "confirmed", Text: quote, Provenance: "user_original"}},
	})
	if err != nil {
		t.Fatalf("conversation source: %v", err)
	}
	return src
}

func proposalInput(src account.PersonalRecord) account.MemoryProposalInput {
	return account.MemoryProposalInput{
		Text: "Synthetic proposal: you write best in the morning.",
		SourceRefs: []account.SourceRef{{
			RecordID: src.ID, Version: src.Version, Quote: "Synthetic quote: I write best in the morning.", TurnKey: "turn-0",
		}},
	}
}

func TestMemoryProposalsReviewTransitions(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	pa := f.principal(a.Token)
	grantMemory(t, f, pa)
	src := proposalSource(t, f, pa.UserID)

	created, err := f.store.CreateMemoryProposals(ctx, pa.UserID, []account.MemoryProposalInput{proposalInput(src)})
	if err != nil {
		t.Fatalf("create proposals: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("proposals = %d, want 1", len(created))
	}
	p := created[0]
	if p.Kind != account.KindMemory || p.Version != 1 || p.ReviewStatus != account.ReviewUnreviewed ||
		p.Provenance != "unconfirmed_interpretation" || p.Title != "" {
		t.Fatalf("proposal record: %+v", p)
	}
	if len(p.SourceRefs) != 1 || p.SourceRefs[0].RecordID != src.ID || p.SourceRefs[0].Version != src.Version ||
		p.SourceRefs[0].Quote != proposalInput(src).SourceRefs[0].Quote || p.SourceRefs[0].TurnKey != "turn-0" {
		t.Fatalf("proposal source: %+v", p.SourceRefs)
	}
	var sealed []byte
	if err := f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_records WHERE id=$1", p.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{p.Text, p.SourceRefs[0].Quote} {
		if bytes.Contains(sealed, []byte(plain)) {
			t.Fatalf("proposal plaintext at rest: %q", plain)
		}
	}

	// Confirm keeps the AI wording and the class, and appends a review
	// revision.
	confirmed, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 1, account.MemoryReviewConfirm, "")
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.ReviewStatus != account.ReviewConfirmed || confirmed.Version != 2 ||
		confirmed.Text != p.Text || confirmed.Provenance != "unconfirmed_interpretation" {
		t.Fatalf("confirmed: %+v", confirmed)
	}
	// The confirmed version is no longer the one a stale tab holds.
	if _, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 1, account.MemoryReviewReject, ""); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale reject overwrote a confirmation: %v", err)
	}

	// Correct replaces the text in a new revision with the user's class.
	corrected, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 2, account.MemoryReviewCorrect, "Synthetic correction: mornings are easier.")
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if corrected.ReviewStatus != account.ReviewCorrected || corrected.Version != 3 ||
		corrected.Provenance != "user_correction" || corrected.Text != "Synthetic correction: mornings are easier." {
		t.Fatalf("corrected: %+v", corrected)
	}
	if _, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 2, account.MemoryReviewCorrect, "Synthetic stale correction."); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale correction: %v", err)
	}

	// Reject is a status change, and the item can be reopened.
	rejected, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 3, account.MemoryReviewReject, "")
	if err != nil || rejected.ReviewStatus != account.ReviewRejected || rejected.Version != 4 {
		t.Fatalf("rejected: %+v %v", rejected, err)
	}
	reopened, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 4, account.MemoryReviewConfirm, "")
	if err != nil || reopened.ReviewStatus != account.ReviewConfirmed || reopened.Version != 5 {
		t.Fatalf("reopened: %+v %v", reopened, err)
	}
	if _, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 5, "undo", ""); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("invalid action: %v", err)
	}
	if _, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 5, account.MemoryReviewCorrect, " "); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("empty correction: %v", err)
	}
	note, err := f.store.CreateRecord(ctx, pa.UserID, account.RecordInput{Kind: "note", Text: "Synthetic note.", StorageGranted: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ReviewMemory(ctx, pa.UserID, note.ID, 1, account.MemoryReviewConfirm, ""); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("non-memory review: %v", err)
	}
}

func TestMemoryProposalsOwnerSourceAndConsent(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)
	src := proposalSource(t, f, pa.UserID)

	// No personal_memory consent: nothing is stored.
	if _, err := f.store.CreateMemoryProposals(ctx, pa.UserID, []account.MemoryProposalInput{proposalInput(src)}); !errors.Is(err, account.ErrConsentRequired) {
		t.Fatalf("proposal without consent: %v", err)
	}
	grantMemory(t, f, pa)
	created, err := f.store.CreateMemoryProposals(ctx, pa.UserID, []account.MemoryProposalInput{proposalInput(src)})
	if err != nil {
		t.Fatal(err)
	}
	p := created[0]
	if _, err := f.store.ReviewMemory(ctx, pb.UserID, p.ID, 1, account.MemoryReviewConfirm, ""); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign review: %v", err)
	}
	// A foreign owner cannot cite another owner's conversation, even with
	// their own memory consent on.
	grantMemory(t, f, pb)
	if _, err := f.store.CreateMemoryProposals(ctx, pb.UserID, []account.MemoryProposalInput{proposalInput(src)}); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign source: %v", err)
	}
	// A changed source revision is stale: the proposal pins what the
	// owner saw.
	updated, err := f.store.UpdateRecord(ctx, pa.UserID, src.ID, account.RecordInput{
		Kind: "conversation", StorageGranted: true, ExpectedVersion: src.Version,
		Turns: []account.ConversationTurn{{Role: "user", ReviewStatus: "confirmed", Text: "Synthetic later turn.", Provenance: "user_original"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreateMemoryProposals(ctx, pa.UserID, []account.MemoryProposalInput{proposalInput(src)}); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale source revision: %v", err)
	}
	if updated.Version == src.Version {
		t.Fatal("source version did not change")
	}

	// Withdrawal hides and blocks review, but deleting the owner's data is
	// never trapped.
	if _, _, err := f.store.SetConsent(ctx, pa.UserID, pa.SessionID, account.PolicyPersonalMemory, false, memoryV); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ReviewMemory(ctx, pa.UserID, p.ID, 1, account.MemoryReviewConfirm, ""); !errors.Is(err, account.ErrConsentRequired) {
		t.Fatalf("review after withdrawal: %v", err)
	}
	if _, err := f.store.MemoryItem(ctx, pa.UserID, p.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("detail after withdrawal: %v", err)
	}
	if err := f.store.DeleteMemory(ctx, pa.UserID, p.ID, 1); err != nil {
		t.Fatalf("delete after withdrawal: %v", err)
	}
}

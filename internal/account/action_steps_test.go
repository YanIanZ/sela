package account_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// TestActionStepsGoWithGuestPurge (BW-3, AB-T10): an expired guest's
// action steps, their revisions, stale marks and lineage edges go with the
// guest in the one deletion path.
func TestActionStepsGoWithGuestPurge(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	g := f.guest(account.AgeBand18Plus)
	owner := f.principal(g.Token).UserID
	note, err := f.store.CreateRecord(ctx, owner, account.RecordInput{Kind: account.KindNote, Title: "Sintetis", Text: "Catatan sintetis.", StorageGranted: true})
	if err != nil {
		t.Fatal(err)
	}
	step, err := f.store.CreateActionStep(ctx, owner, account.ActionStepInput{Text: "Langkah sintetis.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.UpdateRecord(ctx, owner, note.ID, account.RecordInput{Kind: account.KindNote, Title: "Sintetis",
		Text: "Catatan sintetis diubah.", StorageGranted: true, ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.ActionStep(ctx, owner, step.ID)
	if err != nil || got.Status != account.ActionNeedsReview {
		t.Fatalf("before purge: %+v %v", got, err)
	}
	f.clock.Advance(40 * 24 * time.Hour)
	if n, err := f.store.PurgeGuests(ctx); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	for _, q := range []string{
		`SELECT count(*) FROM personal_records WHERE user_id=$1`,
		`SELECT count(*) FROM personal_record_versions WHERE user_id=$1`,
		`SELECT count(*) FROM mie_edges WHERE user_id=$1`,
		`SELECT count(*) FROM mie_stale WHERE user_id=$1`,
	} {
		if n := f.count(q, owner); n != 0 {
			t.Fatalf("%s: %d after purge", q, n)
		}
	}
	if _, err := f.store.ActionStep(ctx, owner, step.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("read after purge: %v", err)
	}
}

// TestActionProposalTokenBinding: a proposal token is bound to its owner,
// its exact released text and its lifetime.
func TestActionProposalTokenBinding(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := f.principal(f.guest(account.AgeBand18Plus).Token).UserID
	b := f.principal(f.guest(account.AgeBand18Plus).Token).UserID
	text := "Mau menuliskan satu pertanyaan sintetis?"
	tok, err := f.store.SignActionProposal(ctx, a, account.ActionProposal{Text: text, Model: "test-generator", Draft: "test_double"})
	if err != nil {
		t.Fatal(err)
	}
	accept := func(owner account.UserID, token, released, saved string) error {
		_, err := f.store.CreateActionStep(ctx, owner, account.ActionStepInput{Text: saved, StorageGranted: true,
			Proposal: &account.ActionProposalAcceptance{Token: token, Text: released}})
		return err
	}
	for name, err := range map[string]error{
		"other owner":    accept(b, tok, text, text),
		"other text":     accept(a, tok, text+"!", text),
		"flipped byte":   accept(a, tok[:len(tok)-2]+"AA", text, text),
		"no signature":   accept(a, "abc", text, text),
		"empty proposal": accept(a, "", "", text),
	} {
		if !errors.Is(err, account.ErrInvalidInput) {
			t.Errorf("%s: %v", name, err)
		}
	}
	step, err := f.store.CreateActionStep(ctx, a, account.ActionStepInput{Text: "Versi milikku.", StorageGranted: true,
		Proposal: &account.ActionProposalAcceptance{Token: tok, Text: text}})
	if err != nil || step.Origin.Kind != account.ActionOriginMeta || !step.Origin.Edited || step.Origin.Model != "test-generator" {
		t.Fatalf("accepted: %+v %v", step, err)
	}
	f.clock.Advance(account.ActionProposalTTL + time.Second)
	if err := accept(a, tok, text, text); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("expired: %v", err)
	}
}

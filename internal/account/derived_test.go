package account_test

import (
	"errors"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

func derivedNote(t *testing.T, f *fixture, owner account.UserID, text string) account.PersonalRecord {
	t.Helper()
	r, err := f.store.CreateRecord(ctx, owner, account.RecordInput{
		Kind: "note", Title: "Synthetic source", Text: text, StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func derivedSummary(t *testing.T, f *fixture, owner account.UserID, source account.PersonalRecord, text string) account.PersonalRecord {
	t.Helper()
	r, err := f.store.CreateSummary(ctx, owner, account.RecordInput{
		Title: "Synthetic summary", Text: text, StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: source.ID, Version: source.Version}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDerivedImpactCountsOwnershipAndEffectiveApproval(t *testing.T) {
	f := newFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)

	source := derivedNote(t, f, pa.UserID, "Synthetic source body: a busy week.")
	other := derivedNote(t, f, pa.UserID, "Synthetic other source body.")
	first := derivedSummary(t, f, pa.UserID, source, "Synthetic summary body one.")
	second := derivedSummary(t, f, pa.UserID, other, "Synthetic summary body two.")
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, first.ID, 1); err != nil {
		t.Fatal(err)
	}
	share, _, _, err := f.store.CreateShare(ctx, pa.UserID, first.ID, "Dr. Synthetic", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}

	foreignSource := derivedNote(t, f, pb.UserID, "Synthetic foreign source body.")
	foreignSummary := derivedSummary(t, f, pb.UserID, foreignSource, "Synthetic foreign summary body.")

	impact, err := f.store.DerivedImpact(ctx, pa.UserID, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if impact.SourceID != source.ID || len(impact.Summaries) != 1 || impact.Totals.Summaries != 1 || impact.Totals.Shares != 1 {
		t.Fatalf("impact: %+v", impact)
	}
	one := impact.Summaries[0]
	if one.ID != first.ID || one.Title != first.Title || one.Version != first.Version || !one.Approved || one.ShareCount != 1 {
		t.Fatalf("impact summary: %+v", one)
	}
	if share.ID == "" {
		t.Fatal("share was not created")
	}

	// A draft summary is listed without claiming approval.
	otherImpact, err := f.store.DerivedImpact(ctx, pa.UserID, other.ID)
	if err != nil || len(otherImpact.Summaries) != 1 || otherImpact.Summaries[0].ID != second.ID ||
		otherImpact.Summaries[0].Approved || otherImpact.Totals.Shares != 0 {
		t.Fatalf("draft impact: %+v %v", otherImpact, err)
	}

	// Unknown, malformed and another owner's source are the same not-found.
	for _, tc := range []struct {
		name   string
		owner  account.UserID
		source string
	}{
		{"foreign", pb.UserID, source.ID},
		{"unknown", pa.UserID, "00000000-0000-4000-8000-000000000000"},
		{"malformed", pa.UserID, "not-a-uuid"},
	} {
		if _, err := f.store.DerivedImpact(ctx, tc.owner, tc.source); !errors.Is(err, account.ErrNotFound) {
			t.Fatalf("%s impact: %v", tc.name, err)
		}
	}

	// Another owner sees only their own summary for their own source.
	foreignImpact, err := f.store.DerivedImpact(ctx, pb.UserID, foreignSource.ID)
	if err != nil || len(foreignImpact.Summaries) != 1 || foreignImpact.Summaries[0].ID != foreignSummary.ID {
		t.Fatalf("foreign impact: %+v %v", foreignImpact, err)
	}

	// Editing the source demotes the effective approval, but the summary
	// still pins the same record and remains part of the impact.
	if _, err := f.store.UpdateRecord(ctx, pa.UserID, source.ID, account.RecordInput{
		Kind: "note", Title: source.Title, Text: "Synthetic edited source body.",
		StorageGranted: true, ExpectedVersion: source.Version,
	}); err != nil {
		t.Fatal(err)
	}
	stale, err := f.store.DerivedImpact(ctx, pa.UserID, source.ID)
	if err != nil || len(stale.Summaries) != 1 || stale.Summaries[0].Approved {
		t.Fatalf("stale impact: %+v %v", stale, err)
	}
}

func TestDeleteRecordDerivedKeepAndDelete(t *testing.T) {
	f := newFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)

	// Keep: the source goes, the summary and its frozen share stay.
	source := derivedNote(t, f, pa.UserID, "Synthetic keep source body.")
	summary := derivedSummary(t, f, pa.UserID, source, "Synthetic keep summary body.")
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, summary.ID, 1); err != nil {
		t.Fatal(err)
	}
	share, token, code, err := f.store.CreateShare(ctx, pa.UserID, summary.ID, "Dr. Keep", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}
	followup, err := f.store.CreateFollowup(ctx, pa.UserID, summary.ID, account.RecordInput{
		Title: "Synthetic keep follow-up", Text: "Synthetic follow-up body.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The exact version is still required before any derived deletion.
	if err := f.store.DeleteRecord(ctx, pa.UserID, source.ID, source.Version+1, account.DerivedDelete); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale delete: %v", err)
	}
	if _, err := f.store.Summary(ctx, pa.UserID, summary.ID); err != nil {
		t.Fatalf("stale delete changed data: %v", err)
	}
	if err := f.store.DeleteRecord(ctx, pa.UserID, source.ID, source.Version, "shred"); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("invalid mode: %v", err)
	}

	if err := f.store.DeleteRecord(ctx, pa.UserID, source.ID, source.Version, account.DerivedKeep); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Record(ctx, pa.UserID, source.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("deleted source visible")
	}
	kept, err := f.store.Summary(ctx, pa.UserID, summary.ID)
	if err != nil || kept.SummaryState != account.SummaryDraft {
		t.Fatalf("kept summary: %+v %v", kept, err)
	}
	if _, err := f.store.Record(ctx, pa.UserID, followup.ID); err != nil {
		t.Fatalf("kept follow-up: %v", err)
	}
	// The kept summary is owner history; its share stays listed but is
	// revoked for future readers (MIE-I05, D-078).
	shares, err := f.store.Shares(ctx, pa.UserID)
	if err != nil || len(shares) != 1 || shares[0].ID != share.ID || shares[0].State != account.ShareRevoked {
		t.Fatalf("kept share: %+v %v", shares, err)
	}
	if _, _, err := f.store.OpenShare(ctx, token, code); err == nil {
		t.Fatal("a share whose source was deleted still opens")
	}

	// Delete: the source, its summary, follow-up, share and access history
	// go together; another owner's data is untouched.
	source2 := derivedNote(t, f, pa.UserID, "Synthetic delete source body.")
	summary2 := derivedSummary(t, f, pa.UserID, source2, "Synthetic delete summary body.")
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, summary2.ID, 1); err != nil {
		t.Fatal(err)
	}
	share2, token2, code2, err := f.store.CreateShare(ctx, pa.UserID, summary2.ID, "Dr. Delete", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.store.OpenShare(ctx, token2, code2); err != nil {
		t.Fatal(err)
	}
	followup2, err := f.store.CreateFollowup(ctx, pa.UserID, summary2.ID, account.RecordInput{
		Title: "Synthetic delete follow-up", Text: "Synthetic deleted follow-up body.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	foreignSource := derivedNote(t, f, pb.UserID, "Synthetic foreign source body.")
	foreignSummary := derivedSummary(t, f, pb.UserID, foreignSource, "Synthetic foreign summary body.")

	if err := f.store.DeleteRecord(ctx, pa.UserID, source2.ID, source2.Version, account.DerivedDelete); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Record(ctx, pa.UserID, source2.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("deleted source2 visible")
	}
	if _, err := f.store.Summary(ctx, pa.UserID, summary2.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("derived summary survived")
	}
	if _, err := f.store.Record(ctx, pa.UserID, followup2.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("derived follow-up survived")
	}
	if n := f.count("SELECT count(*) FROM shares WHERE id=$1", share2.ID); n != 0 {
		t.Fatalf("deleted share rows: %d", n)
	}
	if n := f.count("SELECT count(*) FROM share_recipient_sessions WHERE share_id=$1", share2.ID); n != 0 {
		t.Fatalf("deleted recipient sessions: %d", n)
	}
	if n := f.count("SELECT count(*) FROM share_events WHERE share_id=$1", share2.ID); n != 0 {
		t.Fatalf("deleted share events: %d", n)
	}
	if _, err := f.store.Summary(ctx, pb.UserID, foreignSummary.ID); err != nil {
		t.Fatalf("foreign summary touched: %v", err)
	}
	// The first summary and its share are independent of the second delete.
	shares, err = f.store.Shares(ctx, pa.UserID)
	if err != nil || len(shares) != 1 || shares[0].ID != share.ID {
		t.Fatalf("surviving share after derived delete: %+v %v", shares, err)
	}

	// An unknown id is not found, and the default mode keeps summaries.
	if err := f.store.DeleteRecord(ctx, pa.UserID, "00000000-0000-4000-8000-000000000000", 1, account.DerivedDelete); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("unknown delete: %v", err)
	}
	source3 := derivedNote(t, f, pa.UserID, "Synthetic default source body.")
	summary3 := derivedSummary(t, f, pa.UserID, source3, "Synthetic default summary body.")
	if err := f.store.DeleteRecord(ctx, pa.UserID, source3.ID, source3.Version, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Summary(ctx, pa.UserID, summary3.ID); err != nil {
		t.Fatalf("default keep removed summary: %v", err)
	}
}

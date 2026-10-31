package account_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

// continuityFixture uses the shared fixture configuration, where the
// optional personal_memory purpose is active but not granted.
func continuityFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixture(t)
}

func grantMemory(t *testing.T, f *fixture, p account.Principal) {
	t.Helper()
	if _, _, err := f.store.SetConsent(ctx, p.UserID, p.SessionID, account.PolicyPersonalMemory, true, memoryV); err != nil {
		t.Fatalf("grant personal memory: %v", err)
	}
}

func TestContinuityMemoryConsentEncryptionAndIsolation(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)

	view, err := f.store.Memory(ctx, pa.UserID)
	if err != nil || view.Enabled || len(view.Items) != 0 {
		t.Fatalf("memory before consent: %+v %v", view, err)
	}
	in := account.RecordInput{Title: "Synthetic memory title", Text: "Synthetic memory secret: I want to sleep earlier.", StorageGranted: true}
	if _, err := f.store.CreateMemory(ctx, pa.UserID, in); !errors.Is(err, account.ErrConsentRequired) {
		t.Fatalf("create without consent: %v", err)
	}

	grantMemory(t, f, pa)
	created, err := f.store.CreateMemory(ctx, pa.UserID, in)
	if err != nil {
		t.Fatal(err)
	}
	if created.Kind != account.KindMemory || created.Version != 1 || created.Provenance != "user_original" || created.SummaryState != "" {
		t.Fatalf("memory record: %+v", created)
	}
	one, err := f.store.MemoryItem(ctx, pa.UserID, created.ID)
	if err != nil || one.ID != created.ID || one.Kind != account.KindMemory {
		t.Fatalf("memory detail: %+v %v", one, err)
	}
	if _, err := f.store.MemoryItem(ctx, pb.UserID, created.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign memory detail: %v", err)
	}
	note, err := f.store.CreateRecord(ctx, pa.UserID, account.RecordInput{Kind: "note", Text: "Synthetic note, not memory.", StorageGranted: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.MemoryItem(ctx, pa.UserID, note.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("non-memory record on memory detail: %v", err)
	}
	if _, err := f.store.MemoryItem(ctx, pa.UserID, "not-a-uuid"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("invalid memory id: %v", err)
	}
	var sealed, sealedVersion []byte
	if err := f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_records WHERE id=$1", created.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_record_versions WHERE record_id=$1 AND version=1", created.ID).Scan(&sealedVersion); err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{sealed, sealedVersion} {
		if bytes.Contains(body, []byte("Synthetic memory secret")) || bytes.Contains(body, []byte("Synthetic memory title")) {
			t.Fatal("memory plaintext at rest")
		}
	}

	// Another owner sees neither the item nor a way to mutate it.
	other, err := f.store.Memory(ctx, pb.UserID)
	if err != nil || other.Enabled || len(other.Items) != 0 {
		t.Fatalf("other owner memory: %+v %v", other, err)
	}
	upd := account.RecordInput{Title: "Synthetic memory title", Text: "Synthetic changed", ExpectedVersion: 1, StorageGranted: true}
	if _, err := f.store.UpdateMemory(ctx, pb.UserID, created.ID, upd); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if err := f.store.DeleteMemory(ctx, pb.UserID, created.ID, 1); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}

	upd.Text = "Synthetic correction: I want to sleep earlier and walk."
	changed, err := f.store.UpdateMemory(ctx, pa.UserID, created.ID, upd)
	if err != nil || changed.Version != 2 || changed.Provenance != "user_correction" {
		t.Fatalf("memory revision: %+v %v", changed, err)
	}
	if _, err := f.store.UpdateMemory(ctx, pa.UserID, created.ID, upd); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale memory update: %v", err)
	}

	// Revocation hides existing rows and blocks writes, but deleting one's
	// own data is never blocked.
	if _, _, err := f.store.SetConsent(ctx, pa.UserID, pa.SessionID, account.PolicyPersonalMemory, false, memoryV); err != nil {
		t.Fatal(err)
	}
	off, err := f.store.Memory(ctx, pa.UserID)
	if err != nil || off.Enabled || len(off.Items) != 0 {
		t.Fatalf("memory after revocation: %+v %v", off, err)
	}
	if _, err := f.store.Record(ctx, pa.UserID, created.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("generic record bypass after withdrawal: %v", err)
	}
	if _, err := f.store.MemoryItem(ctx, pa.UserID, created.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("memory detail bypass after withdrawal: %v", err)
	}
	if _, err := f.store.RecordHistory(ctx, pa.UserID, created.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("history bypass after withdrawal: %v", err)
	}
	visible, err := f.store.Records(ctx, pa.UserID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range visible {
		if row.Kind == account.KindMemory {
			t.Fatal("generic list reveals withdrawn memory")
		}
	}
	if _, err := f.store.CreateMemory(ctx, pa.UserID, in); !errors.Is(err, account.ErrConsentRequired) {
		t.Fatalf("create after revocation: %v", err)
	}
	if err := f.store.DeleteMemory(ctx, pa.UserID, created.ID, 1); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("delete with stale version: %v", err)
	}
	if err := f.store.DeleteMemory(ctx, pa.UserID, created.ID, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Record(ctx, pa.UserID, created.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("deleted memory visible")
	}
}

func TestContinuitySummarySourcesApprovalAndState(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)
	grantMemory(t, f, pa)

	note, err := f.store.CreateRecord(ctx, pa.UserID, account.RecordInput{
		Kind: "note", Title: "Synthetic source", Text: "Synthetic source text about a busy week.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := f.store.CreateRecord(ctx, pb.UserID, account.RecordInput{
		Kind: "note", Text: "Another owner's source.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	summaryIn := account.RecordInput{
		Title: "Synthetic consultation summary", Text: "Synthetic summary in my own words.",
		StorageGranted: true,
		SourceRefs:     []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
		Prepared:       &account.SummaryPrepared{Goal: "Synthetic goal", Questions: "Synthetic question", Bring: "Synthetic list"},
	}
	summary, err := f.store.CreateSummary(ctx, pa.UserID, summaryIn)
	if err != nil {
		t.Fatal(err)
	}
	if summary.SummaryState != account.SummaryDraft || summary.Provenance != "manual" || len(summary.SourceRefs) != 1 {
		t.Fatalf("summary create: %+v", summary)
	}
	var sealed []byte
	if err := f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_records WHERE id=$1", summary.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("Synthetic summary in my own words")) {
		t.Fatal("summary plaintext at rest")
	}

	approved, err := f.store.ApproveSummary(ctx, pa.UserID, summary.ID, 1)
	if err != nil || approved.SummaryState != account.SummaryApproved || approved.ApprovedVersion != 1 {
		t.Fatalf("approval: %+v %v", approved, err)
	}
	edited := account.RecordInput{
		Title: summary.Title, Text: "Synthetic summary corrected.", ExpectedVersion: 1, StorageGranted: true,
		SourceRefs: summary.SourceRefs,
	}
	next, err := f.store.UpdateSummary(ctx, pa.UserID, summary.ID, edited)
	if err != nil || next.Version != 2 || next.SummaryState != account.SummaryDraft {
		t.Fatalf("edit must revoke approval: %+v %v", next, err)
	}
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, summary.ID, 1); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("approve stale summary version: %v", err)
	}
	if approved, err = f.store.ApproveSummary(ctx, pa.UserID, summary.ID, 2); err != nil || approved.SummaryState != account.SummaryApproved {
		t.Fatalf("reapprove: %+v %v", approved, err)
	}

	// A source edit silently invalidates the stored approval on every read.
	noteIn := account.RecordInput{Kind: "note", Title: "Synthetic source", Text: "Synthetic changed source text.", ExpectedVersion: 1, StorageGranted: true}
	if _, err := f.store.UpdateRecord(ctx, pa.UserID, note.ID, noteIn); err != nil {
		t.Fatal(err)
	}
	stale, err := f.store.Summary(ctx, pa.UserID, summary.ID)
	if err != nil || stale.SummaryState != account.SummaryDraft {
		t.Fatalf("stale source must demote approval: %+v %v", stale, err)
	}
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, summary.ID, 2); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("approve with stale source: %v", err)
	}

	// Re-pinning the exact new revision makes approval possible again.
	edited.Text = "Synthetic summary pinned to the new source."
	edited.SourceRefs = []account.SourceRef{{RecordID: note.ID, Version: 2}}
	edited.ExpectedVersion = 2
	if _, err := f.store.UpdateSummary(ctx, pa.UserID, summary.ID, edited); err != nil {
		t.Fatal(err)
	}
	edited.Text = "Synthetic summary final."
	edited.SourceRefs = []account.SourceRef{{RecordID: note.ID, Version: 2}}
	edited.ExpectedVersion = 3
	if _, err := f.store.UpdateSummary(ctx, pa.UserID, summary.ID, edited); err != nil {
		t.Fatal(err)
	}
	if approved, err = f.store.ApproveSummary(ctx, pa.UserID, summary.ID, 4); err != nil || approved.SummaryState != account.SummaryApproved {
		t.Fatalf("approve re-pinned source: %+v %v", approved, err)
	}
	// Owner isolation and source validation.
	if _, err := f.store.Summary(ctx, pb.UserID, summary.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign summary read: %v", err)
	}
	foreignIn := summaryIn
	foreignIn.SourceRefs = []account.SourceRef{{RecordID: foreign.ID, Version: 1}}
	if _, err := f.store.CreateSummary(ctx, pa.UserID, foreignIn); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign source accepted: %v", err)
	}
	summaryIn.SourceRefs = []account.SourceRef{{RecordID: note.ID, Version: 1}}
	if _, err := f.store.CreateSummary(ctx, pa.UserID, summaryIn); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale source accepted: %v", err)
	}
	memory, err := f.store.CreateMemory(ctx, pa.UserID, account.RecordInput{Text: "Synthetic memory", StorageGranted: true})
	if err != nil {
		t.Fatal(err)
	}
	summaryIn.SourceRefs = []account.SourceRef{{RecordID: memory.ID, Version: 1}}
	if _, err := f.store.CreateSummary(ctx, pa.UserID, summaryIn); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("recursive memory source accepted: %v", err)
	}
	var tooMany []account.SourceRef
	for i := 0; i < account.MaxSourceRefs+1; i++ {
		tooMany = append(tooMany, account.SourceRef{RecordID: note.ID, Version: 2})
	}
	summaryIn.SourceRefs = tooMany
	if _, err := f.store.CreateSummary(ctx, pa.UserID, summaryIn); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("too many sources accepted: %v", err)
	}
	summaryIn.SourceRefs = nil
	if _, err := f.store.CreateSummary(ctx, pa.UserID, summaryIn); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("summary without sources accepted: %v", err)
	}

	// Deleting a quoted source also invalidates the approval on read.
	if err := f.store.DeleteRecord(ctx, pa.UserID, note.ID, 2, account.DerivedKeep); err != nil {
		t.Fatal(err)
	}
	deletedSource, err := f.store.Summary(ctx, pa.UserID, summary.ID)
	if err != nil || deletedSource.SummaryState != account.SummaryDraft {
		t.Fatalf("deleted source must demote approval: %+v %v", deletedSource, err)
	}
}

func TestContinuityFollowupsDeletionAndExport(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)
	grantMemory(t, f, pa)

	note, err := f.store.CreateRecord(ctx, pa.UserID, account.RecordInput{
		Kind: "note", Text: "Synthetic follow-up source.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	memory, err := f.store.CreateMemory(ctx, pa.UserID, account.RecordInput{
		Title: "Synthetic memory", Text: "Synthetic memory body.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := f.store.CreateSummary(ctx, pa.UserID, account.RecordInput{
		Title: "Synthetic summary", Text: "Synthetic summary body.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	followup, err := f.store.CreateFollowup(ctx, pa.UserID, summary.ID, account.RecordInput{
		Title: "Synthetic follow-up", Text: "Synthetic after-consultation note.", StorageGranted: true,
	})
	if err != nil || followup.Kind != account.KindFollowup || followup.ParentID != summary.ID {
		t.Fatalf("follow-up: %+v %v", followup, err)
	}
	if _, err := f.store.Record(ctx, pb.UserID, followup.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign follow-up: %v", err)
	}
	if _, err := f.store.CreateFollowup(ctx, pb.UserID, summary.ID, account.RecordInput{
		Text: "Foreign write.", StorageGranted: true,
	}); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("follow-up on foreign summary: %v", err)
	}
	list, err := f.store.Followups(ctx, pa.UserID, summary.ID)
	if err != nil || len(list) != 1 || list[0].ID != followup.ID {
		t.Fatalf("follow-up list: %+v %v", list, err)
	}

	corrected, err := f.store.UpdateRecord(ctx, pa.UserID, followup.ID, account.RecordInput{Kind: account.KindFollowup, Title: followup.Title, Text: "Synthetic after-consultation note. Corrected.", StorageGranted: true, ExpectedVersion: followup.Version})
	if err != nil || corrected.Version != 2 || corrected.ParentID != summary.ID {
		t.Fatalf("followup correction loses parent or version: %+v %v", corrected, err)
	}
	if _, err := f.store.UpdateRecord(ctx, pb.UserID, followup.ID, account.RecordInput{Kind: account.KindFollowup, Text: "Foreign correction.", StorageGranted: true, ExpectedVersion: 2}); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign followup correction: %v", err)
	}

	exported, err := f.store.Export(ctx, pa.UserID, pa.SessionID, "json", "en")
	if err != nil || !bytes.Contains(exported.Body, []byte("Synthetic memory body")) ||
		!bytes.Contains(exported.Body, []byte("Synthetic summary body")) ||
		!bytes.Contains(exported.Body, []byte("Synthetic after-consultation note")) {
		t.Fatalf("export misses continuity records: %v", err)
	}

	// Deleting a summary takes its own record, revisions and follow-ups.
	if err := f.store.DeleteSummary(ctx, pa.UserID, summary.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Record(ctx, pa.UserID, summary.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("deleted summary visible")
	}
	if _, err := f.store.Record(ctx, pa.UserID, followup.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("follow-up survived summary deletion")
	}
	if _, err := f.store.Followups(ctx, pa.UserID, summary.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("follow-ups of deleted summary still listed")
	}

	// Memory is deleted on its own version and disappears from export.
	if err := f.store.DeleteMemory(ctx, pa.UserID, memory.ID, 1); err != nil {
		t.Fatal(err)
	}
	after, err := f.store.Export(ctx, pa.UserID, pa.SessionID, "json", "en")
	if err != nil || bytes.Contains(after.Body, []byte("Synthetic memory body")) {
		t.Fatalf("deleted memory still exported: %v", err)
	}
}

func TestContinuityFollowupSectionsAndEdit(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)

	note, err := f.store.CreateRecord(ctx, pa.UserID, account.RecordInput{
		Kind: "note", Text: "Synthetic structured follow-up source.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := f.store.CreateSummary(ctx, pa.UserID, account.RecordInput{
		Text: "Synthetic summary for a structured follow-up.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherSummary, err := f.store.CreateSummary(ctx, pa.UserID, account.RecordInput{
		Text: "Synthetic second summary.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
	})
	if err != nil {
		t.Fatal(err)
	}

	sections := &account.FollowupSections{
		Feeling:  "Synthetic feeling after the consultation.",
		Remember: "Synthetic thing I want to remember.",
		Agreed:   "Synthetic agreement with the professional.",
		AskNext:  "Synthetic question for next time.",
	}
	followup, err := f.store.CreateFollowup(ctx, pa.UserID, summary.ID, account.RecordInput{
		Title: "Synthetic structured follow-up", StorageGranted: true, Sections: sections,
	})
	if err != nil {
		t.Fatalf("create sections-only follow-up: %v", err)
	}
	if followup.Text != "" || followup.Sections == nil || followup.Sections.Feeling != sections.Feeling ||
		followup.Sections.Remember != sections.Remember || followup.Sections.Agreed != sections.Agreed ||
		followup.Sections.AskNext != sections.AskNext || followup.ParentID != summary.ID {
		t.Fatalf("structured follow-up: %+v", followup)
	}
	var sealed []byte
	if err := f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_records WHERE id=$1", followup.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{sections.Feeling, sections.Remember, sections.Agreed, sections.AskNext} {
		if bytes.Contains(sealed, []byte(plain)) {
			t.Fatal("follow-up section plaintext at rest")
		}
	}
	list, err := f.store.Followups(ctx, pa.UserID, summary.ID)
	if err != nil || len(list) != 1 || list[0].Sections == nil || list[0].Sections.AskNext != sections.AskNext {
		t.Fatalf("follow-up list misses sections: %+v %v", list, err)
	}
	read, err := f.store.Record(ctx, pa.UserID, followup.ID)
	if err != nil || read.Sections == nil || read.Sections.Remember != sections.Remember {
		t.Fatalf("follow-up read misses sections: %+v %v", read, err)
	}

	// A legacy text-only note is still accepted and keeps Sections nil.
	legacy, err := f.store.CreateFollowup(ctx, pa.UserID, summary.ID, account.RecordInput{
		Title: "Synthetic legacy follow-up", Text: "Synthetic free-text follow-up.", StorageGranted: true,
	})
	if err != nil || legacy.Sections != nil || legacy.Text != "Synthetic free-text follow-up." {
		t.Fatalf("legacy follow-up: %+v %v", legacy, err)
	}

	// The dedicated update path replaces title/text/sections with a version check.
	nextSections := &account.FollowupSections{
		Feeling:  "Synthetic corrected feeling.",
		Remember: "Synthetic corrected memory.",
		Agreed:   "Synthetic corrected agreement.",
		AskNext:  "Synthetic corrected question.",
	}
	edited, err := f.store.UpdateFollowup(ctx, pa.UserID, summary.ID, followup.ID, account.RecordInput{
		Title: "Synthetic corrected follow-up", Text: "Synthetic free note after editing.",
		StorageGranted: true, ExpectedVersion: followup.Version, Sections: nextSections,
	})
	if err != nil || edited.Version != 2 || edited.ParentID != summary.ID || edited.Sections == nil ||
		edited.Sections.Feeling != nextSections.Feeling || edited.Text != "Synthetic free note after editing." {
		t.Fatalf("structured follow-up edit: %+v %v", edited, err)
	}
	if _, err := f.store.UpdateFollowup(ctx, pa.UserID, summary.ID, followup.ID, account.RecordInput{
		Title: "Synthetic stale follow-up", Text: "Synthetic stale.", StorageGranted: true,
		ExpectedVersion: 1, Sections: nextSections,
	}); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale follow-up edit: %v", err)
	}
	// The summary in the path must be the sealed parent.
	if _, err := f.store.UpdateFollowup(ctx, pa.UserID, otherSummary.ID, followup.ID, account.RecordInput{
		Title: "Synthetic wrong parent", Text: "Synthetic wrong parent.", StorageGranted: true,
		ExpectedVersion: 2, Sections: nextSections,
	}); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("follow-up edit through another summary: %v", err)
	}
	if _, err := f.store.UpdateFollowup(ctx, pb.UserID, summary.ID, followup.ID, account.RecordInput{
		Title: "Synthetic foreign", Text: "Synthetic foreign.", StorageGranted: true,
		ExpectedVersion: 2, Sections: nextSections,
	}); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign follow-up edit: %v", err)
	}

	// Sections-only edit clears the free text; an old text-only edit keeps
	// the stored sections; an explicit empty sections object clears them.
	onlySections, err := f.store.UpdateFollowup(ctx, pa.UserID, summary.ID, followup.ID, account.RecordInput{
		Title: "Synthetic structured follow-up", StorageGranted: true, ExpectedVersion: 2, Sections: nextSections,
	})
	if err != nil || onlySections.Version != 3 || onlySections.Text != "" || onlySections.Sections == nil {
		t.Fatalf("sections-only edit: %+v %v", onlySections, err)
	}
	legacyEdit, err := f.store.UpdateRecord(ctx, pa.UserID, followup.ID, account.RecordInput{
		Kind: account.KindFollowup, Title: "Synthetic legacy edit", Text: "Synthetic text-only correction.",
		StorageGranted: true, ExpectedVersion: 3,
	})
	if err != nil || legacyEdit.Sections == nil || legacyEdit.Sections.Feeling != nextSections.Feeling {
		t.Fatalf("text-only edit dropped sections: %+v %v", legacyEdit, err)
	}
	cleared, err := f.store.UpdateFollowup(ctx, pa.UserID, summary.ID, followup.ID, account.RecordInput{
		Title: "Synthetic cleared", Text: "Synthetic note without sections.", StorageGranted: true,
		ExpectedVersion: 4, Sections: &account.FollowupSections{Feeling: "  "},
	})
	if err != nil || cleared.Version != 5 || cleared.Sections != nil || cleared.Text != "Synthetic note without sections." {
		t.Fatalf("clear sections: %+v %v", cleared, err)
	}

	// Boundaries: 2000 runes is accepted, 2001 and invalid UTF-8 are not.
	maxRunes := strings.Repeat("a", account.FollowupSectionMax)
	atLimit, err := f.store.CreateFollowup(ctx, pa.UserID, summary.ID, account.RecordInput{
		Title: "Synthetic boundary", StorageGranted: true,
		Sections: &account.FollowupSections{Feeling: maxRunes},
	})
	if err != nil || atLimit.Sections == nil || len([]rune(atLimit.Sections.Feeling)) != account.FollowupSectionMax {
		t.Fatalf("2000-rune section: %+v %v", atLimit, err)
	}
	for name, in := range map[string]account.RecordInput{
		"2001 runes": {Title: "Synthetic too long", StorageGranted: true,
			Sections: &account.FollowupSections{AskNext: maxRunes + "a"}},
		"invalid utf8": {Title: "Synthetic invalid", StorageGranted: true,
			Sections: &account.FollowupSections{Remember: string([]byte{0xff})}},
		"nothing": {Title: "Synthetic empty", StorageGranted: true,
			Sections: &account.FollowupSections{Feeling: " "}},
	} {
		if _, err := f.store.CreateFollowup(ctx, pa.UserID, summary.ID, in); !errors.Is(err, account.ErrInvalidInput) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := f.store.CreateMemory(ctx, pa.UserID, account.RecordInput{
		Text: "Synthetic memory with sections.", StorageGranted: true, Sections: sections,
	}); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("sections on memory: %v", err)
	}
}

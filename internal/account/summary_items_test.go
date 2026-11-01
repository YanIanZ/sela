package account_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

func itemSummaryInput(note, other account.PersonalRecord) account.RecordInput {
	return account.RecordInput{
		Title: "Synthetic item summary", Text: "", StorageGranted: true,
		SourceRefs: []account.SourceRef{
			{RecordID: note.ID, Version: note.Version},
			{RecordID: other.ID, Version: other.Version},
		},
		Items: []account.SummaryItem{
			{
				Text: "Synthetic statement one.", Quote: "Synthetic item source one.",
				SourceRef: account.SummaryItemSource{RecordID: note.ID, Version: note.Version},
			},
			{
				Text: "Synthetic statement two.", Quote: "Synthetic item source two.",
				SourceRef: account.SummaryItemSource{RecordID: other.ID, Version: other.Version},
			},
		},
	}
}

func summaryItemSources(t *testing.T, f *fixture, owner account.UserID) (account.PersonalRecord, account.PersonalRecord) {
	t.Helper()
	note, err := f.store.CreateRecord(ctx, owner, account.RecordInput{
		// Every quote a test uses is verbatim in its source (MIE-I02, D-078).
		Kind: "note", Title: "Synthetic item source one", StorageGranted: true,
		Text: "Synthetic item source one. Synthetic exported quote. " + strings.Repeat("a", account.SummaryItemQuoteMax),
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.store.CreateRecord(ctx, owner, account.RecordInput{
		Kind: "note", Title: "Synthetic item source two", Text: "Synthetic item source two.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return note, other
}

// TestSummaryItemsCreateEditApproval covers the additive statement model:
// ids stay stable across edits, a corrected item is labelled as the owner's
// correction, and every edit returns the summary to draft.
func TestSummaryItemsCreateEditApproval(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	pa := f.principal(a.Token)
	note, other := summaryItemSources(t, f, pa.UserID)

	created, err := f.store.CreateSummary(ctx, pa.UserID, itemSummaryInput(note, other))
	if err != nil {
		t.Fatal(err)
	}
	if created.SummaryState != account.SummaryDraft || created.Text != "" || len(created.Items) != 2 {
		t.Fatalf("summary items create: %+v", created)
	}
	for i, item := range created.Items {
		if item.ID == "" || item.Position != i || item.Provenance != "user_original" ||
			item.ReviewStatus != account.ReviewConfirmed || item.SourceRef.RecordID == "" {
			t.Fatalf("normalized item %d: %+v", i, item)
		}
	}
	if created.Items[0].ID == created.Items[1].ID {
		t.Fatal("item ids are not unique")
	}
	// Every statement and quote stays inside the sealed content.
	var sealed []byte
	if err := f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_records WHERE id=$1", created.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	for _, plain := range []string{"Synthetic statement one.", "Synthetic item source one.", "Synthetic statement two."} {
		if bytes.Contains(sealed, []byte(plain)) {
			t.Fatalf("summary item plaintext at rest: %q", plain)
		}
	}

	approved, err := f.store.ApproveSummary(ctx, pa.UserID, created.ID, created.Version)
	if err != nil || approved.SummaryState != account.SummaryApproved || approved.ApprovedVersion != 1 {
		t.Fatalf("approve item summary: %+v %v", approved, err)
	}

	// Keep the second item, correct it, move it first, drop the first and
	// add one new item. The kept id survives; the corrected wording gets
	// the user_correction class; the new item starts user_original.
	editedIn := itemSummaryInput(note, other)
	editedIn.ExpectedVersion = created.Version
	editedIn.Items = []account.SummaryItem{
		{
			ID: created.Items[1].ID, Text: "Synthetic corrected statement two.",
			Quote:     created.Items[1].Quote,
			SourceRef: created.Items[1].SourceRef,
		},
		{
			ID: created.Items[0].ID, Text: created.Items[0].Text,
			Quote:     created.Items[0].Quote,
			SourceRef: created.Items[0].SourceRef,
		},
		{
			Text: "Synthetic new statement three.", Quote: "Synthetic item source one.",
			SourceRef: account.SummaryItemSource{RecordID: note.ID, Version: note.Version},
		},
	}
	edited, err := f.store.UpdateSummary(ctx, pa.UserID, created.ID, editedIn)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Version != 2 || edited.SummaryState != account.SummaryDraft || len(edited.Items) != 3 {
		t.Fatalf("edited item summary: %+v", edited)
	}
	if edited.Items[0].ID != created.Items[1].ID || edited.Items[0].Provenance != "user_correction" ||
		edited.Items[0].Text != "Synthetic corrected statement two." {
		t.Fatalf("corrected kept item: %+v", edited.Items[0])
	}
	if edited.Items[1].ID != created.Items[0].ID || edited.Items[1].Provenance != "user_original" ||
		edited.Items[1].Text != created.Items[0].Text {
		t.Fatalf("unchanged kept item: %+v", edited.Items[1])
	}
	if edited.Items[2].ID == "" || edited.Items[2].ID == created.Items[0].ID ||
		edited.Items[2].ID == created.Items[1].ID || edited.Items[2].Provenance != "user_original" {
		t.Fatalf("new item: %+v", edited.Items[2])
	}
	for i, item := range edited.Items {
		if item.Position != i || item.ReviewStatus != account.ReviewConfirmed {
			t.Fatalf("edited item %d: %+v", i, item)
		}
	}

	// The version that was approved is no longer current.
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, created.ID, 1); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("approve stale item version: %v", err)
	}
	if approved, err = f.store.ApproveSummary(ctx, pa.UserID, created.ID, 2); err != nil ||
		approved.SummaryState != account.SummaryApproved || approved.ApprovedVersion != 2 {
		t.Fatalf("reapprove item summary: %+v %v", approved, err)
	}

	// A text-only edit that omits items keeps the stored statements.
	keepIn := account.RecordInput{
		Title: "Synthetic item summary", Text: "Synthetic free wording alongside statements.",
		ExpectedVersion: 2, StorageGranted: true,
	}
	kept, err := f.store.UpdateSummary(ctx, pa.UserID, created.ID, keepIn)
	if err != nil || len(kept.Items) != 3 || kept.Text != "Synthetic free wording alongside statements." {
		t.Fatalf("text-only edit dropped items: %+v %v", kept, err)
	}
	// An explicit empty array with non-empty text clears them.
	cleared, err := f.store.UpdateSummary(ctx, pa.UserID, created.ID, account.RecordInput{
		Title: "Synthetic item summary", Text: "Synthetic text only again.",
		ExpectedVersion: 3, StorageGranted: true, Items: []account.SummaryItem{},
	})
	if err != nil || cleared.Items != nil || cleared.Text != "Synthetic text only again." {
		t.Fatalf("clear items: %+v %v", cleared, err)
	}

	// The owner export carries the statements when they exist.
	restored, err := f.store.UpdateSummary(ctx, pa.UserID, created.ID, account.RecordInput{
		Title: "Synthetic item summary", Text: "", ExpectedVersion: 4, StorageGranted: true,
		Items: []account.SummaryItem{{
			Text: "Synthetic exported statement.", Quote: "Synthetic exported quote.",
			SourceRef: account.SummaryItemSource{RecordID: note.ID, Version: note.Version},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, created.ID, restored.Version); err != nil {
		t.Fatal(err)
	}
	exported, err := f.store.Export(ctx, pa.UserID, pa.SessionID, "json", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(exported.Body, []byte("Synthetic exported statement.")) ||
		!bytes.Contains(exported.Body, []byte("Synthetic exported quote.")) {
		t.Fatal("owner export misses summary items")
	}
}

// TestSummaryItemsValidation checks bounds, source ownership and the
// compatibility rule that an old text-only summary stays exactly as it was.
func TestSummaryItemsValidation(t *testing.T) {
	f := continuityFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa, pb := f.principal(a.Token), f.principal(b.Token)
	grantMemory(t, f, pa)
	note, other := summaryItemSources(t, f, pa.UserID)
	foreign, err := f.store.CreateRecord(ctx, pb.UserID, account.RecordInput{
		Kind: "note", Text: "Synthetic foreign item source.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := itemSummaryInput(note, other)
	first := base.Items[0]

	valid := func(items ...account.SummaryItem) account.RecordInput {
		in := base
		in.Items = items
		return in
	}
	if _, err := f.store.CreateSummary(ctx, pa.UserID, valid(first)); err != nil {
		t.Fatalf("item without a quote should be accepted: %v", err)
	}

	tooLongText := first
	tooLongText.Text = strings.Repeat("a", account.SummaryItemTextMax+1)
	tooLongQuote := first
	tooLongQuote.Quote = strings.Repeat("a", account.SummaryItemQuoteMax+1)
	blankText := first
	blankText.Text = "   "
	duplicateIDs := []account.SummaryItem{first, first}
	duplicateIDs[0].ID = "same-item"
	duplicateIDs[1].ID = "same-item"
	unknownSource := first
	unknownSource.SourceRef = account.SummaryItemSource{RecordID: foreign.ID, Version: foreign.Version}
	staleSource := first
	staleSource.SourceRef = account.SummaryItemSource{RecordID: note.ID, Version: note.Version + 1}
	aiProvenance := first
	aiProvenance.Provenance = "ai_summary"
	unreviewed := first
	unreviewed.ReviewStatus = account.ReviewUnreviewed
	noSource := first
	noSource.SourceRef = account.SummaryItemSource{}
	var tooMany []account.SummaryItem
	for i := 0; i < account.MaxSummaryItems+1; i++ {
		item := first
		item.ID = ""
		tooMany = append(tooMany, item)
	}
	for name, in := range map[string]account.RecordInput{
		"blank item text":      valid(blankText),
		"801-rune item text":   valid(tooLongText),
		"1201-rune item quote": valid(tooLongQuote),
		"duplicate item ids":   valid(duplicateIDs...),
		"foreign item source":  valid(unknownSource),
		"stale item source":    valid(staleSource),
		"AI item provenance":   valid(aiProvenance),
		"unreviewed item":      valid(unreviewed),
		"item without source":  valid(noSource),
		"37 items":             valid(tooMany...),
		"empty text and items": {Title: "Synthetic empty", StorageGranted: true, SourceRefs: base.SourceRefs},
	} {
		_, err := f.store.CreateSummary(ctx, pa.UserID, in)
		if !errors.Is(err, account.ErrInvalidInput) {
			t.Fatalf("%s: %v", name, err)
		}
	}

	// Boundaries are inclusive.
	maxText := first
	maxText.Text = strings.Repeat("a", account.SummaryItemTextMax)
	maxQuote := first
	maxQuote.Quote = strings.Repeat("a", account.SummaryItemQuoteMax)
	if _, err := f.store.CreateSummary(ctx, pa.UserID, valid(maxText, maxQuote)); err != nil {
		t.Fatalf("item boundaries: %v", err)
	}
	var maxItems []account.SummaryItem
	for i := 0; i < account.MaxSummaryItems; i++ {
		item := first
		item.ID = ""
		item.Text = "Synthetic statement."
		item.Quote = ""
		maxItems = append(maxItems, item)
	}
	if _, err := f.store.CreateSummary(ctx, pa.UserID, valid(maxItems...)); err != nil {
		t.Fatalf("36 items: %v", err)
	}

	// A legacy text-only summary has no items and reads unchanged.
	legacy, err := f.store.CreateSummary(ctx, pa.UserID, account.RecordInput{
		Title: "Synthetic legacy summary", Text: "Synthetic legacy wording.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
	})
	if err != nil || legacy.Items != nil || legacy.Text != "Synthetic legacy wording." {
		t.Fatalf("legacy summary: %+v %v", legacy, err)
	}
	exported, err := f.store.Export(ctx, pa.UserID, pa.SessionID, "json", "en")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Sections struct {
			PersonalRecords struct {
				Records []map[string]any `json:"records"`
			} `json:"personalRecords"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(exported.Body, &doc); err != nil {
		t.Fatal(err)
	}
	var legacyExported map[string]any
	for _, rec := range doc.Sections.PersonalRecords.Records {
		if rec["id"] == legacy.ID {
			legacyExported = rec
		}
	}
	if legacyExported == nil {
		t.Fatal("legacy summary missing from the owner export")
	}
	if _, ok := legacyExported["items"]; ok {
		t.Fatalf("legacy summary grew items: %v", legacyExported)
	}

	// Statement items are refused on memory, follow-up and the generic
	// record surface; only a summary accepts them.
	if _, err := f.store.CreateMemory(ctx, pa.UserID, account.RecordInput{
		Text: "Synthetic memory with items.", StorageGranted: true, Items: []account.SummaryItem{first},
	}); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("items on a memory: %v", err)
	}
	if _, err := f.store.CreateFollowup(ctx, pa.UserID, legacy.ID, account.RecordInput{
		Text: "Synthetic follow-up with items.", StorageGranted: true, Items: []account.SummaryItem{first},
	}); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("items on a follow-up: %v", err)
	}
	if _, err := f.store.CreateRecord(ctx, pa.UserID, account.RecordInput{
		Kind: "note", Text: "Synthetic note with items.", StorageGranted: true, Items: []account.SummaryItem{first},
	}); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("items on a generic note: %v", err)
	}

	// Another owner cannot read or edit this summary or its statements.
	if _, err := f.store.Summary(ctx, pb.UserID, legacy.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign summary read: %v", err)
	}
	if _, err := f.store.UpdateSummary(ctx, pb.UserID, legacy.ID, account.RecordInput{
		Title: legacy.Title, Text: legacy.Text, ExpectedVersion: 1, StorageGranted: true,
	}); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign summary item edit: %v", err)
	}
}

// TestSummaryItemsShareProjection freezes the statements into the same
// immutable projection as the text, with source labels but no internal ids.
func TestSummaryItemsShareProjection(t *testing.T) {
	f := newFixture(t, func(c *account.Config) {
		c.Purposes = append(c.Purposes, account.Purpose{Name: "sharing", Kinds: []string{account.KindGuest, account.KindAccount}})
		c.PolicyVersions["sharing"] = "sharing-test.1"
	})
	a := f.guest(account.AgeBand18Plus)
	pa := f.principal(a.Token)
	note, other := summaryItemSources(t, f, pa.UserID)

	created, err := f.store.CreateSummary(ctx, pa.UserID, itemSummaryInput(note, other))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, created.ID, created.Version); err != nil {
		t.Fatal(err)
	}
	_, token, code, err := f.store.CreateShare(ctx, pa.UserID, created.ID, "Dr. Synthetic", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}
	projection, _, err := f.store.OpenShare(ctx, token, code)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Items) != 2 || projection.SummaryVersion != created.Version {
		t.Fatalf("shared items: %+v", projection)
	}
	if projection.Items[0].Text != "Synthetic statement one." ||
		projection.Items[0].Quote != "Synthetic item source one." ||
		projection.Items[0].SourceLabel != "Synthetic item source one · v1" ||
		projection.Items[0].Position != 0 {
		t.Fatalf("shared item one: %+v", projection.Items[0])
	}
	if projection.Items[1].SourceLabel != "Synthetic item source two · v1" {
		t.Fatalf("shared item two label: %+v", projection.Items[1])
	}

	// A legacy text-only summary freezes the same projection as before:
	// no items field at all.
	legacy, err := f.store.CreateSummary(ctx, pa.UserID, account.RecordInput{
		Title: "Synthetic legacy shared summary", Text: "Synthetic legacy shared text.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ApproveSummary(ctx, pa.UserID, legacy.ID, legacy.Version); err != nil {
		t.Fatal(err)
	}
	_, legacyToken, legacyCode, err := f.store.CreateShare(ctx, pa.UserID, legacy.ID, "Dr. Legacy", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}
	legacyProjection, _, err := f.store.OpenShare(ctx, legacyToken, legacyCode)
	if err != nil {
		t.Fatal(err)
	}
	if legacyProjection.Items != nil || legacyProjection.Text != "Synthetic legacy shared text." {
		t.Fatalf("legacy shared projection: %+v", legacyProjection)
	}
}

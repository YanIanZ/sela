package account_test

import (
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/account"
)

// Life Context Timeline (BW-002, LIFE_CONTEXT_TIMELINE.md LT-01..LT-12).
// All text below is synthetic.

func timelineAll(t *testing.T, s *account.Store, owner account.UserID, q account.TimelineQuery) []account.TimelineItem {
	t.Helper()
	var out []account.TimelineItem
	for i := 0; i < 100; i++ {
		page, err := s.ContextTimeline(ctx, owner, q)
		if err != nil {
			t.Fatalf("timeline page %d: %v", i, err)
		}
		if page.AIUsed || page.Count != len(page.Items) {
			t.Fatalf("page meta: %+v", page)
		}
		out = append(out, page.Items...)
		if page.NextCursor == nil {
			return out
		}
		q.Cursor = *page.NextCursor
	}
	t.Fatal("timeline did not end")
	return nil
}

func timelineIDs(items []account.TimelineItem) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Source.RecordID
	}
	return ids
}

func mustNote(t *testing.T, f *fixture, owner account.UserID, text string) account.PersonalRecord {
	t.Helper()
	r, err := f.store.CreateRecord(ctx, owner, account.RecordInput{Kind: "note", Text: text, StorageGranted: true})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	return r
}

// LT-01, BW-T01: an empty owner gets a useful, empty, AI-free page.
func TestContextTimelineEmpty(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p := f.principal(f.guest(account.AgeBand18Plus).Token)
	page, err := f.store.ContextTimeline(ctx, p.UserID, account.TimelineQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || len(page.Items) != 0 || page.Count != 0 || page.NextCursor != nil || page.AIUsed {
		t.Fatalf("empty page: %+v", page)
	}
}

// LT-02, BW-T12: owner isolation; a cursor naming a foreign record id and
// one naming an unknown id give the same answer.
func TestContextTimelineOwnerIsolation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	pa := f.principal(f.guest(account.AgeBand18Plus).Token)
	pb := f.principal(f.guest(account.AgeBand18Plus).Token)
	secret := mustNote(t, f, pa.UserID, "Synthetic secret of owner A only.")
	f.clock.Advance(time.Minute)
	own := mustNote(t, f, pb.UserID, "Synthetic note of owner B.")

	items := timelineAll(t, f.store, pb.UserID, account.TimelineQuery{})
	if len(items) != 1 || items[0].Source.RecordID != own.ID {
		t.Fatalf("owner B sees %+v", items)
	}
	for _, it := range items {
		if strings.Contains(it.Excerpt, "owner A") {
			t.Fatal("owner A text leaked")
		}
	}
	cursor := func(id string) string {
		raw := fmt.Sprintf("v1|desc|%d|%s", f.clock.Now().Add(time.Hour).UnixMicro(), id)
		return base64.RawURLEncoding.EncodeToString([]byte(raw))
	}
	foreign, err := f.store.ContextTimeline(ctx, pb.UserID, account.TimelineQuery{Cursor: cursor(secret.ID)})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := f.store.ContextTimeline(ctx, pb.UserID, account.TimelineQuery{Cursor: cursor("6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11")})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(timelineIDs(foreign.Items), timelineIDs(unknown.Items)) || len(foreign.Items) != 1 {
		t.Fatalf("foreign %+v vs unknown %+v", foreign.Items, unknown.Items)
	}
}

// LT-04, BW-T02, BW-T22: chronological order with equal timestamps and a
// UTC midnight boundary, stable pages in both orders, and a record inserted
// mid-pagination neither duplicates nor shifts a later page.
func TestContextTimelineOrderingAndPagination(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p := f.principal(f.guest(account.AgeBand18Plus).Token)
	f.clock.Advance(14*time.Hour + 59*time.Minute + 59*time.Second) // 23:59:59 UTC
	var made []account.PersonalRecord
	for i := 0; i < 3; i++ { // three records at the same instant
		made = append(made, mustNote(t, f, p.UserID, fmt.Sprintf("Synthetic same-instant note %d.", i)))
	}
	f.clock.Advance(2 * time.Second) // past midnight, next UTC day
	made = append(made, mustNote(t, f, p.UserID, "Synthetic note after midnight."))
	// The check-in gets its own instant so it is newest regardless of the
	// random record-id tie-break.
	f.clock.Advance(time.Second)
	ck, err := f.store.CreateRecord(ctx, p.UserID, account.RecordInput{Kind: "checkin", Day: "2026-11-03", Mood: "okay",
		Energy: "low", Text: "Synthetic check-in.", StorageGranted: true})
	if err != nil {
		t.Fatal(err)
	}
	made = append(made, ck)

	full := timelineAll(t, f.store, p.UserID, account.TimelineQuery{Limit: 50})
	if len(full) != len(made) {
		t.Fatalf("got %d items, want %d", len(full), len(made))
	}
	for i := 1; i < len(full); i++ {
		a, b := full[i-1], full[i]
		if a.OccurredAt.Before(b.OccurredAt) || (a.OccurredAt.Equal(b.OccurredAt) && a.Source.RecordID < b.Source.RecordID) {
			t.Fatalf("not newest first at %d: %+v then %+v", i, a, b)
		}
		if a.OccurredAt.Location() != time.UTC {
			t.Fatal("occurredAt not UTC")
		}
	}
	paged := timelineAll(t, f.store, p.UserID, account.TimelineQuery{Limit: 2})
	if !slices.Equal(timelineIDs(paged), timelineIDs(full)) {
		t.Fatalf("paged order differs:\n%v\n%v", timelineIDs(paged), timelineIDs(full))
	}
	asc := timelineAll(t, f.store, p.UserID, account.TimelineQuery{Limit: 2, Order: "asc"})
	rev := timelineIDs(full)
	slices.Reverse(rev)
	if !slices.Equal(timelineIDs(asc), rev) {
		t.Fatalf("asc is not the reverse:\n%v\n%v", timelineIDs(asc), rev)
	}
	if full[0].Source.Kind != "checkin" || full[0].Day != "2026-11-03" || full[0].Mood != "okay" || full[0].Energy != "low" {
		t.Fatalf("check-in item: %+v", full[0])
	}

	// A concurrent insert between pages.
	first, err := f.store.ContextTimeline(ctx, p.UserID, account.TimelineQuery{Limit: 2})
	if err != nil || first.NextCursor == nil {
		t.Fatalf("first page: %+v %v", first, err)
	}
	f.clock.Advance(time.Minute)
	late := mustNote(t, f, p.UserID, "Synthetic note written while paging.")
	rest := timelineAll(t, f.store, p.UserID, account.TimelineQuery{Limit: 2, Cursor: *first.NextCursor})
	got := append(timelineIDs(first.Items), timelineIDs(rest)...)
	if !slices.Equal(got, timelineIDs(full)) || slices.Contains(got, late.ID) {
		t.Fatalf("insert shifted pages:\n%v\n%v", got, timelineIDs(full))
	}
	// A cursor for the other order is refused.
	if _, err := f.store.ContextTimeline(ctx, p.UserID, account.TimelineQuery{Order: "asc", Cursor: *first.NextCursor}); account.CodeOf(err) != "invalid_input" {
		t.Fatalf("mixed-order cursor: %v", err)
	}
}

// Windows: days ends at the server clock; from inclusive, to exclusive;
// kinds filter.
func TestContextTimelineWindowAndKinds(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p := f.principal(f.register("timeline.window").Token)
	old := mustNote(t, f, p.UserID, "Synthetic note from long ago.")
	f.clock.Advance(40 * 24 * time.Hour)
	mid := mustNote(t, f, p.UserID, "Synthetic note from three weeks ago.")
	f.clock.Advance(20 * 24 * time.Hour)
	ck, err := f.store.CreateRecord(ctx, p.UserID, account.RecordInput{Kind: "checkin", Day: "2026-12-31", StorageGranted: true})
	if err != nil {
		t.Fatal(err)
	}
	ids := func(q account.TimelineQuery) []string { return timelineIDs(timelineAll(t, f.store, p.UserID, q)) }
	if got := ids(account.TimelineQuery{Days: 7}); !slices.Equal(got, []string{ck.ID}) {
		t.Fatalf("7 days: %v", got)
	}
	if got := ids(account.TimelineQuery{Days: 30}); !slices.Equal(got, []string{ck.ID, mid.ID}) {
		t.Fatalf("30 days: %v", got)
	}
	if got := ids(account.TimelineQuery{Days: 90}); len(got) != 3 {
		t.Fatalf("90 days: %v", got)
	}
	if got := ids(account.TimelineQuery{Days: 90, Kinds: []string{"note"}}); !slices.Equal(got, []string{mid.ID, old.ID}) {
		t.Fatalf("notes only: %v", got)
	}
	if got := ids(account.TimelineQuery{From: mid.CreatedAt, To: ck.CreatedAt}); !slices.Equal(got, []string{mid.ID}) {
		t.Fatalf("from inclusive / to exclusive: %v", got)
	}
	for name, q := range map[string]account.TimelineQuery{
		"limit":   {Limit: 51},
		"limit-":  {Limit: -1},
		"kinds":   {Kinds: []string{"memory"}},
		"summary": {Kinds: []string{"summary"}},
		"order":   {Order: "sideways"},
		"days":    {Days: 14},
		"both":    {Days: 7, From: old.CreatedAt},
		"to":      {From: ck.CreatedAt, To: mid.CreatedAt},
		"cursor":  {Cursor: "not-a-cursor"},
		"cursor2": {Cursor: base64.RawURLEncoding.EncodeToString([]byte("v1|desc|x|y"))},
	} {
		if _, err := f.store.ContextTimeline(ctx, p.UserID, q); account.CodeOf(err) != "invalid_input" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// LT-05, LT-06, LT-07: version and verbatim excerpt; a source edit marks
// the source stale while the summary built on the old version is stale;
// deletion removes it on the next read; non-source kinds never appear.
func TestContextTimelineVersionsStaleAndDeletion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p := f.principal(f.guest(account.AgeBand18Plus).Token)
	grantMemory(t, f, p)
	long := strings.Repeat("Aku menulis catatan sintetis ini—pelan-pelan. ", 12) // multibyte, > 280 runes
	note := mustNote(t, f, p.UserID, long)
	summary, err := f.store.CreateSummary(ctx, p.UserID, account.RecordInput{
		Title: "Synthetic summary", Text: "Synthetic summary in my words.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
		Prepared:   &account.SummaryPrepared{Goal: "g", Questions: "q", Bring: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreateMemory(ctx, p.UserID, account.RecordInput{Title: "Synthetic memory", Text: "Synthetic memory.", StorageGranted: true}); err != nil {
		t.Fatal(err)
	}
	items := timelineAll(t, f.store, p.UserID, account.TimelineQuery{})
	if len(items) != 1 {
		t.Fatalf("only the source is an item: %+v", items)
	}
	it := items[0]
	if it.Source.Version != 1 || it.Provenance != "user_original" || it.MeaningStatus != account.TimelineCurrent || it.Stale != nil {
		t.Fatalf("before edit: %+v", it)
	}
	if !it.Truncated || utf8.RuneCountInString(it.Excerpt) != account.TimelineExcerptRunes || !strings.HasPrefix(long, it.Excerpt) {
		t.Fatalf("excerpt is not a verbatim 280-rune prefix: %q", it.Excerpt)
	}

	f.clock.Advance(time.Hour)
	edited, err := f.store.UpdateRecord(ctx, p.UserID, note.ID, account.RecordInput{Kind: "note", Text: "Synthetic corrected note.",
		StorageGranted: true, ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	it = timelineAll(t, f.store, p.UserID, account.TimelineQuery{})[0]
	if it.Source.Version != 2 || it.Excerpt != "Synthetic corrected note." || it.Truncated || it.Provenance != "user_correction" ||
		it.MeaningStatus != account.TimelineStale || it.Stale == nil || it.Stale.Reason != account.MIESourceEdited {
		t.Fatalf("after edit: %+v", it)
	}
	if !it.OccurredAt.Equal(note.CreatedAt) || !it.UpdatedAt.Equal(edited.UpdatedAt) {
		t.Fatalf("an edit must not move the item: %+v", it)
	}

	// Removing the stale interpretation clears the marker.
	if err := f.store.DeleteSummary(ctx, p.UserID, summary.ID, summary.Version); err != nil {
		t.Fatal(err)
	}
	if it = timelineAll(t, f.store, p.UserID, account.TimelineQuery{})[0]; it.MeaningStatus != account.TimelineCurrent {
		t.Fatalf("after the summary is gone: %+v", it)
	}
	// Deleting the source removes it on the next read.
	if err := f.store.DeleteRecord(ctx, p.UserID, note.ID, 2, account.DerivedKeep); err != nil {
		t.Fatal(err)
	}
	if items := timelineAll(t, f.store, p.UserID, account.TimelineQuery{}); len(items) != 0 {
		t.Fatalf("deleted source still listed: %+v", items)
	}
}

// A saved conversation: the owner's own turns only, and an unreviewed Sela
// reflection is "needs_review".
func TestContextTimelineConversationNeedsReview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p := f.principal(f.guest(account.AgeBand18Plus).Token)
	conv, err := f.store.CreateRecord(ctx, p.UserID, account.RecordInput{Kind: "conversation", Title: "Synthetic room", StorageGranted: true,
		Turns: []account.ConversationTurn{
			{Role: "user", Text: "Synthetic words of mine.", Provenance: "user_original", ReviewStatus: "confirmed"},
			{Role: "assistant", Text: "SYNTHETIC-AI-REPLY", Provenance: "ai_draft", ReviewStatus: "unreviewed"},
			{Role: "user", Text: "Synthetic second turn.", Provenance: "user_original", ReviewStatus: "confirmed"},
		},
		Reflections: []account.Reflection{{ID: "r1", Quote: "Synthetic words of mine.", Text: "Synthetic tentative reflection.",
			ReviewStatus: account.ReflectionUnreviewed, Provenance: "ai_draft"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	it := timelineAll(t, f.store, p.UserID, account.TimelineQuery{})[0]
	if it.Source.RecordID != conv.ID || it.Source.Kind != "conversation" || it.Title != "Synthetic room" ||
		it.Excerpt != "Synthetic words of mine.\n\nSynthetic second turn." || strings.Contains(it.Excerpt, "AI-REPLY") ||
		it.MeaningStatus != account.TimelineNeedsReview || it.PendingReflections != 1 {
		t.Fatalf("conversation item: %+v", it)
	}
}

// LT-09 (store side): a timeline read sends only reads; it writes nothing.
func TestContextTimelineReadOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	p := f.principal(f.guest(account.AgeBand18Plus).Token)
	mustNote(t, f, p.UserID, "Synthetic note.")
	rec := &sqlRecorder{}
	s := f.tracedStore(rec)
	if _, err := s.ContextTimeline(ctx, p.UserID, account.TimelineQuery{Days: 30}); err != nil {
		t.Fatal(err)
	}
	sqls, errs := rec.take()
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	for _, q := range sqls {
		u := strings.ToUpper(strings.TrimSpace(q))
		for _, w := range []string{"INSERT", "UPDATE ", "DELETE", "MERGE"} {
			if strings.Contains(u, w) {
				t.Fatalf("timeline read sent a write: %s", q)
			}
		}
	}
	if len(sqls) == 0 {
		t.Fatal("no statements recorded")
	}
}

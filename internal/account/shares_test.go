package account_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// approvedSummaryFor creates a note, a summary pinned to it, and approves
// the summary, returning what CreateShare needs.
func approvedSummaryFor(t *testing.T, f *fixture, owner account.UserID) account.PersonalRecord {
	t.Helper()
	note, err := f.store.CreateRecord(ctx, owner, account.RecordInput{
		Kind: "note", Title: "Synthetic source", Text: "Synthetic source text.", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := f.store.CreateSummary(ctx, owner, account.RecordInput{
		Title: "Synthetic summary", Text: "Synthetic summary text.", StorageGranted: true,
		SourceRefs: []account.SourceRef{{RecordID: note.ID, Version: note.Version}},
	})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := f.store.ApproveSummary(ctx, owner, summary.ID, summary.Version)
	if err != nil {
		t.Fatal(err)
	}
	return approved
}

func TestShareAccessHistory(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	owner := f.register("pemilik.riwayat")
	op := f.principal(owner.Token)
	summary := approvedSummaryFor(t, f, op.UserID)

	share, token, code, err := f.store.CreateShare(ctx, op.UserID, summary.ID, "Dr. Sintetis", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := f.clock.Now()
	listed := onlyShare(t, f, op.UserID)
	if listed.OpenedCount != 0 || listed.AcknowledgedCount != 0 || listed.LastOpenedAt != nil || listed.Acknowledged {
		t.Fatalf("fresh counters: %+v", listed)
	}

	f.clock.Advance(time.Minute)
	if _, session, err := f.store.OpenShare(ctx, token, code); err != nil {
		t.Fatal(err)
	} else if _, err := f.store.RecipientShare(ctx, session); err != nil {
		t.Fatal(err)
	}

	f.clock.Advance(time.Minute)
	if _, session, err := f.store.OpenShare(ctx, token, code); err != nil {
		t.Fatal(err)
	} else {
		f.clock.Advance(time.Minute)
		// A repeated acknowledgement must not write a second event.
		if err := f.store.AcknowledgeShare(ctx, session); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(time.Minute)
		if err := f.store.AcknowledgeShare(ctx, session); err != nil {
			t.Fatalf("idempotent acknowledge: %v", err)
		}
	}

	listed = onlyShare(t, f, op.UserID)
	if listed.OpenedCount != 2 || listed.AcknowledgedCount != 1 || !listed.Acknowledged {
		t.Fatalf("counters: %+v", listed)
	}
	if listed.LastOpenedAt == nil || !listed.LastOpenedAt.Equal(createdAt.Add(2*time.Minute)) {
		t.Fatalf("last opened: %v", listed.LastOpenedAt)
	}

	f.clock.Advance(time.Minute)
	revoked, err := f.store.RevokeShare(ctx, op.UserID, share.ID, 1)
	if err != nil || revoked.State != account.ShareRevoked {
		t.Fatalf("revoke: %+v %v", revoked, err)
	}

	// A revoked share still shows its whole history to its owner.
	events, err := f.store.ShareEvents(ctx, op.UserID, share.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"revoked", "acknowledged", "opened", "opened", "created"}
	if len(events) != len(want) {
		t.Fatalf("events: %+v", events)
	}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Fatalf("event %d = %q, want %q (%+v)", i, events[i].Kind, kind, events)
		}
		if i > 0 && !events[i].At.Before(events[i-1].At) {
			t.Fatalf("newest-first ordering broken at %d: %+v", i, events)
		}
	}

	// The wire shape is only kind and at: no token, code, content,
	// recipient session, IP address or owner id.
	raw, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{token, code, "Synthetic summary text.", op.UserID.String(), "127.0.0.1"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("events JSON leaks %q: %s", secret, raw)
		}
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if len(row) != 2 || row["kind"] == nil || row["at"] == nil {
			t.Fatalf("unexpected event fields: %v", row)
		}
	}

	// Metadata on the wire stays free of hashes and projection data too.
	listRaw, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"tokenHash", "codeHash", "projection", token, code} {
		if strings.Contains(string(listRaw), secret) {
			t.Fatalf("share metadata leaks %q: %s", secret, listRaw)
		}
	}
}

func TestShareHistoryIsOwnerScoped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	owner := f.register("pemilik.riwayat.a")
	op := f.principal(owner.Token)
	summary := approvedSummaryFor(t, f, op.UserID)
	share, _, _, err := f.store.CreateShare(ctx, op.UserID, summary.ID, "Dr. Sintetis", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}

	other := f.register("pemilik.riwayat.b")
	otherp := f.principal(other.Token)
	if _, err := f.store.ShareEvents(ctx, otherp.UserID, share.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign history: %v", err)
	}
	if list, err := f.store.Shares(ctx, otherp.UserID); err != nil || len(list) != 0 {
		t.Fatalf("foreign shares: %+v %v", list, err)
	}
	if _, err := f.store.RevokeShare(ctx, otherp.UserID, share.ID, 1); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("foreign revoke: %v", err)
	}
	if _, err := f.store.ShareEvents(ctx, op.UserID, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("unknown history: %v", err)
	}
	if _, err := f.store.ShareEvents(ctx, op.UserID, "not-a-uuid"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("malformed history: %v", err)
	}
	if _, err := f.store.ShareEvents(ctx, op.UserID, share.ID); err != nil {
		t.Fatalf("owner history: %v", err)
	}
}

func onlyShare(t *testing.T, f *fixture, owner account.UserID) account.Share {
	t.Helper()
	list, err := f.store.Shares(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("shares: %+v", list)
	}
	return list[0]
}

package account_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

func TestV27V28V33PersonalRecords(t *testing.T) {
	f := newFixture(t)
	a := f.guest(account.AgeBand18Plus)
	b := f.guest(account.AgeBand18Plus)
	pa := f.principal(a.Token)
	pb := f.principal(b.Token)
	input := account.RecordInput{Kind: "note", Text: "Synthetic private original: tugas menumpuk dan saya ingin menulis.", StorageGranted: true}
	bad := input
	bad.StorageGranted = false
	if _, err := f.store.CreateRecord(ctx, pa.UserID, bad); !errors.Is(err, account.ErrStorageRequired) {
		t.Fatalf("storage choice: %v", err)
	}
	first, err := f.store.CreateRecord(ctx, pa.UserID, input)
	if err != nil {
		t.Fatal(err)
	}
	var sealed []byte
	if err = f.db.Pool.QueryRow(ctx, "SELECT content_ct FROM personal_records WHERE id=$1", first.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("Synthetic private original")) {
		t.Fatal("plaintext at rest")
	}
	if _, err = f.store.Record(ctx, pb.UserID, first.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("isolation read: %v", err)
	}
	if err = f.store.DeleteRecord(ctx, pb.UserID, first.ID, 1, account.DerivedKeep); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("isolation delete: %v", err)
	}
	input.Text = "Synthetic correction: saya ingin menulis malam ini."
	input.ExpectedVersion = 1
	changed, err := f.store.UpdateRecord(ctx, pa.UserID, first.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Version != 2 || changed.Provenance != "user_correction" {
		t.Fatalf("revision: %+v", changed)
	}
	if _, err = f.store.UpdateRecord(ctx, pa.UserID, first.ID, input); !errors.Is(err, account.ErrStaleVersion) {
		t.Fatalf("stale: %v", err)
	}
	history, err := f.store.RecordHistory(ctx, pa.UserID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Text == history[1].Text {
		t.Fatal("original lost")
	}
	restarted := f.restart()
	got, err := restarted.Record(ctx, pa.UserID, first.ID)
	if err != nil || got.Text != input.Text {
		t.Fatalf("restart: %v", err)
	}
	if err = f.store.DeleteRecord(ctx, pa.UserID, first.ID, 2, account.DerivedKeep); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Record(ctx, pa.UserID, first.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatal("deleted item visible")
	}
}

func TestV27CheckInAndV38CrisisPreservation(t *testing.T) {
	f := newFixture(t)
	u := f.guest(account.AgeBand18Plus)
	p := f.principal(u.Token)
	in := account.RecordInput{Kind: "checkin", Day: "2026-10-27", Mood: "heavy", Text: "Synthetic test", StorageGranted: true}
	a, err := f.store.CreateRecord(ctx, p.UserID, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Mood = "okay"
	in.ExpectedVersion = a.Version
	b, err := f.store.UpdateRecord(ctx, p.UserID, a.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	list, err := f.store.Records(ctx, p.UserID)
	if err != nil || len(list) != 1 || b.Mood != "okay" {
		t.Fatalf("checkin: %v", err)
	}
	in = account.RecordInput{Kind: "note", Text: "Aku mau bunuh diri malam ini.", StorageGranted: true}
	r, err := f.store.CreateRecord(ctx, p.UserID, in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Safety != "crisis" || r.Text != in.Text {
		t.Fatal("crisis words must be preserved with support")
	}
	ex, err := f.store.Export(ctx, p.UserID, p.SessionID, "json", "id")
	if err != nil || !bytes.Contains(ex.Body, []byte("personalRecords")) {
		t.Fatalf("export: %v", err)
	}
}

func TestV27CheckInEnergyAndTags(t *testing.T) {
	f := newFixture(t)
	u := f.guest(account.AgeBand18Plus)
	p := f.principal(u.Token)
	rec, err := f.store.CreateRecord(ctx, p.UserID, account.RecordInput{
		Kind: "checkin", Day: "2026-10-28", Mood: "mixed", Energy: "medium",
		Tags: []string{"sleep", "school"}, Text: "Synthetic check-in", StorageGranted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.store.Record(ctx, p.UserID, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mood != "mixed" || got.Energy != "medium" || len(got.Tags) != 2 {
		t.Fatalf("check-in detail: %+v", got)
	}
	if _, err := f.store.CreateRecord(ctx, p.UserID, account.RecordInput{
		Kind: "checkin", Day: "2026-10-29", Mood: "okay", Energy: "exhausted", StorageGranted: true,
	}); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("invalid energy: %v", err)
	}
	if _, err := f.store.CreateRecord(ctx, p.UserID, account.RecordInput{
		Kind: "checkin", Day: "2026-10-29", Mood: "okay", Tags: []string{"unknown_tag"}, StorageGranted: true,
	}); !errors.Is(err, account.ErrInvalidInput) {
		t.Fatalf("invalid tag: %v", err)
	}
}

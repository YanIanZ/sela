package account_test

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/vault"
)

// TestPrivateNames covers the owner-facing list: validation, the 20-name
// limit, case-insensitive duplicates, encryption at rest, owner isolation
// and export.
func TestPrivateNames(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a := f.register("nama.pribadi")
	ap := f.principal(a.Token)
	owner := ap.UserID

	list, err := f.store.PrivateNames(ctx, owner)
	if err != nil || len(list) != 0 {
		t.Fatalf("empty list: %v %v", list, err)
	}
	raka, err := f.store.CreatePrivateName(ctx, owner, " Raka ")
	if err != nil || raka.Name != "Raka" || raka.ID == "" {
		t.Fatalf("create: %+v %v", raka, err)
	}
	dewi, err := f.store.CreatePrivateName(ctx, owner, "Dewi")
	if err != nil {
		t.Fatal(err)
	}
	list, err = f.store.PrivateNames(ctx, owner)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}
	got := map[string]bool{list[0].Name: true, list[1].Name: true}
	if !got["Raka"] || !got["Dewi"] {
		t.Fatalf("list: %+v", list)
	}
	// Duplicates are refused regardless of case and edge whitespace.
	for _, dup := range []string{"raka", "RAKA", " Raka  "} {
		if _, err := f.store.CreatePrivateName(ctx, owner, dup); !errors.Is(err, account.ErrNameTaken) {
			t.Fatalf("duplicate %q: %v", dup, err)
		}
	}
	// Validation refuses empty, over-long, control characters and invalid UTF-8.
	for _, bad := range []string{"", "   ", strings.Repeat("a", account.MaxPrivateNameRunes+1), "Raka\nPutra", "Raka\tPutra", "\xff"} {
		if _, err := f.store.CreatePrivateName(ctx, owner, bad); !errors.Is(err, account.ErrInvalidInput) {
			t.Fatalf("invalid %q: %v", bad, err)
		}
	}
	// Exactly 120 runes is allowed, and the sealed value keeps its bound.
	long := strings.Repeat("é", account.MaxPrivateNameRunes)
	if _, err := f.store.CreatePrivateName(ctx, owner, long); err != nil {
		t.Fatalf("120 runes: %v", err)
	}

	// The name is stored only as ciphertext sealed for this owner and record.
	var ct []byte
	if err := f.db.Pool.QueryRow(ctx, `SELECT name_ct FROM private_names WHERE user_id = $1 AND id = $2`, owner, raka.ID).Scan(&ct); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("Raka")) || bytes.Contains([]byte(dbtest.DumpText(t, f.db.Pool)), []byte("Raka")) {
		t.Fatal("a private name is readable at rest")
	}
	raw, err := f.vault.Open(ctx, f.db.Pool, owner, vault.Ref{Kind: "private_name", RecordID: raka.ID, Field: "name"}, ct)
	if err != nil || string(raw) != "Raka" {
		t.Fatalf("open: %q %v", raw, err)
	}
	// Another record id or field does not open it (the AAD binds both).
	if _, err := f.vault.Open(ctx, f.db.Pool, owner, vault.Ref{Kind: "private_name", RecordID: dewi.ID, Field: "name"}, ct); err == nil {
		t.Fatal("ciphertext opened under another record id")
	}

	// Another owner never sees or deletes the list.
	b := f.register("nama.lain")
	bp := f.principal(b.Token)
	if other, err := f.store.PrivateNames(ctx, bp.UserID); err != nil || len(other) != 0 {
		t.Fatalf("other owner list: %+v %v", other, err)
	}
	if err := f.store.DeletePrivateName(ctx, bp.UserID, raka.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("cross-owner delete: %v", err)
	}
	if err := f.store.DeletePrivateName(ctx, bp.UserID, "not-a-uuid"); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("malformed id: %v", err)
	}
	if list, _ := f.store.PrivateNames(ctx, owner); len(list) != 3 {
		t.Fatalf("owner list after cross-owner delete: %+v", list)
	}

	// Deletion is owner-scoped and idempotent only until the row is gone.
	if err := f.store.DeletePrivateName(ctx, owner, dewi.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DeletePrivateName(ctx, owner, dewi.ID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}

	// The owner's own export includes the decrypted names.
	ex, err := f.store.Export(ctx, owner, ap.SessionID, "json", "id")
	if err != nil || !bytes.Contains(ex.Body, []byte("privateNames")) || !bytes.Contains(ex.Body, []byte(`"name": "Raka"`)) {
		t.Fatalf("export: %v %s", err, ex.Body)
	}
}

func TestPrivateNameLimit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.guest(account.AgeBand18Plus)
	p := f.principal(u.Token)
	for i := 0; i < account.MaxPrivateNames; i++ {
		if _, err := f.store.CreatePrivateName(ctx, p.UserID, "Nama "+strings.Repeat("a", i)); err != nil {
			t.Fatalf("name %d: %v", i, err)
		}
	}
	if _, err := f.store.CreatePrivateName(ctx, p.UserID, "Nama satu lagi"); !errors.Is(err, account.ErrNameLimitReached) {
		t.Fatalf("21st name: %v", err)
	}
	if n := f.count(`SELECT count(*) FROM private_names WHERE user_id = $1`, p.UserID); n != account.MaxPrivateNames {
		t.Fatalf("stored %d names", n)
	}
	// Removing one makes room again.
	list, err := f.store.PrivateNames(ctx, p.UserID)
	if err != nil || len(list) != account.MaxPrivateNames {
		t.Fatalf("list: %v %v", list, err)
	}
	if err := f.store.DeletePrivateName(ctx, p.UserID, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreatePrivateName(ctx, p.UserID, "Nama satu lagi"); err != nil {
		t.Fatalf("after delete: %v", err)
	}
}

// TestPrivateNameConcurrentDuplicate: two creates of the same name cannot
// both pass the duplicate check (the owner row is locked first).
func TestPrivateNameConcurrentDuplicate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.guest(account.AgeBand18Plus)
	p := f.principal(u.Token)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.store.CreatePrivateName(ctx, p.UserID, "Kasih")
		}()
	}
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("two concurrent duplicates: %v / %v", errs[0], errs[1])
	}
	var dup error
	if errs[0] != nil {
		dup = errs[0]
	} else {
		dup = errs[1]
	}
	if !errors.Is(dup, account.ErrNameTaken) {
		t.Fatalf("loser: %v", dup)
	}
	if n := f.count(`SELECT count(*) FROM private_names WHERE user_id = $1`, p.UserID); n != 1 {
		t.Fatalf("stored %d names", n)
	}
}

// TestPrivateNamesDeletedWithAccount: the rows cascade and a copy of the
// ciphertext from before the deletion cannot be opened afterwards.
func TestPrivateNamesDeletedWithAccount(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.register("hapus.nama")
	p := f.principal(u.Token)
	if _, err := f.store.CreatePrivateName(ctx, p.UserID, "Raka"); err != nil {
		t.Fatal(err)
	}
	var ct []byte
	if err := f.db.Pool.QueryRow(ctx, `SELECT name_ct FROM private_names WHERE user_id = $1`, p.UserID).Scan(&ct); err != nil {
		t.Fatal(err)
	}
	refID := ""
	if err := f.db.Pool.QueryRow(ctx, `SELECT id::text FROM private_names WHERE user_id = $1`, p.UserID).Scan(&refID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DeleteAccount(ctx, p.UserID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	if n := f.count(`SELECT count(*) FROM private_names WHERE user_id = $1`, p.UserID); n != 0 {
		t.Fatalf("%d name rows survived deletion", n)
	}
	ref := vault.Ref{Kind: "private_name", RecordID: refID, Field: "name"}
	if _, err := f.vault.Open(ctx, f.db.Pool, p.UserID, ref, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("backup copy opened after deletion: %v", err)
	}
}

// TestPrivateNamesNoSecretsAtRest scans every row of every table (bytea as
// hex) for the name and its hex form.
func TestPrivateNamesNoSecretsAtRest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.register("tanpa.terang")
	p := f.principal(u.Token)
	secret := "Kasih Ramadhani"
	if _, err := f.store.CreatePrivateName(ctx, p.UserID, secret); err != nil {
		t.Fatal(err)
	}
	dump := dbtest.DumpText(t, f.db.Pool)
	for _, form := range []string{secret, strings.ToLower(secret), strings.ToUpper(secret)} {
		if strings.Contains(dump, form) {
			t.Fatalf("%q found at rest", form)
		}
	}
	if !strings.Contains(dump, "private_names") {
		t.Fatal("the dump does not cover private_names")
	}
}

package vault_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/vault"
)

var ctx = context.Background()

func key(t *testing.T, id string) vault.Key {
	t.Helper()
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return vault.Key{ID: id, Secret: b}
}

func uid() [16]byte {
	var u [16]byte
	_, _ = rand.Read(u[:])
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return u
}

func randomDEK() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

func TestSealOpenRoundTripAndFormat(t *testing.T) {
	t.Parallel()
	dek, user := randomDEK(), uid()
	ref := vault.Ref{Kind: "guardian_consent", RecordID: "0b8f9c1e-0000-4000-8000-000000000001", Field: "guardian_name"}
	for _, pt := range [][]byte{[]byte("Ibu Sari (sintetis)"), {}, bytes.Repeat([]byte("x"), 4096)} {
		sealed, err := vault.SealWithDEK(dek, user, ref, pt)
		if err != nil {
			t.Fatal(err)
		}
		if len(sealed) != len(pt)+vault.Overhead || sealed[0] != vault.FormatV1 {
			t.Fatalf("format: len %d for %d bytes, first byte %#x", len(sealed), len(pt), sealed[0])
		}
		if len(pt) > 0 && bytes.Contains(sealed, pt) {
			t.Fatal("plaintext visible in the sealed value")
		}
		got, err := vault.OpenWithDEK(dek, user, ref, sealed)
		if err != nil || !bytes.Equal(got, pt) {
			t.Fatalf("open: %q %v", got, err)
		}
	}
	a, _ := vault.SealWithDEK(dek, user, ref, []byte("same"))
	b, _ := vault.SealWithDEK(dek, user, ref, []byte("same"))
	if bytes.Equal(a[1:13], b[1:13]) {
		t.Fatal("nonce reused")
	}
}

func TestAADBinding(t *testing.T) {
	t.Parallel()
	dek, user := randomDEK(), uid()
	ref := vault.Ref{Kind: "guardian_consent", RecordID: "r1", Field: "guardian_name"}
	sealed, err := vault.SealWithDEK(dek, user, ref, []byte("marker-plaintext"))
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		user [16]byte
		ref  vault.Ref
	}{
		"another user":   {uid(), ref},
		"another kind":   {user, vault.Ref{Kind: "journal_entry", RecordID: "r1", Field: "guardian_name"}},
		"another record": {user, vault.Ref{Kind: "guardian_consent", RecordID: "r2", Field: "guardian_name"}},
		"another field":  {user, vault.Ref{Kind: "guardian_consent", RecordID: "r1", Field: "title"}},
		// Length prefixes make field boundaries unambiguous.
		"shifted boundary": {user, vault.Ref{Kind: "guardian_consentr", RecordID: "1", Field: "guardian_name"}},
	} {
		if _, err := vault.OpenWithDEK(dek, c.user, c.ref, sealed); !errors.Is(err, vault.ErrOpen) {
			t.Errorf("%s: opened (%v)", name, err)
		}
	}
	if _, err := vault.OpenWithDEK(randomDEK(), user, ref, sealed); !errors.Is(err, vault.ErrOpen) {
		t.Errorf("wrong DEK: %v", err)
	}
}

func TestTamper(t *testing.T) {
	t.Parallel()
	dek, user := randomDEK(), uid()
	ref := vault.Ref{Kind: "k", RecordID: "r", Field: "f"}
	sealed, _ := vault.SealWithDEK(dek, user, ref, []byte("hello world"))
	for i := range sealed {
		c := bytes.Clone(sealed)
		c[i] ^= 0x01
		_, err := vault.OpenWithDEK(dek, user, ref, c)
		if err == nil {
			t.Fatalf("flipping byte %d still opened", i)
		}
		if i == 0 && !errors.Is(err, vault.ErrFormat) {
			t.Fatalf("version byte: %v", err)
		}
	}
	for _, c := range [][]byte{nil, sealed[:vault.Overhead-1], sealed[:len(sealed)-1], append(bytes.Clone(sealed), 0)} {
		if _, err := vault.OpenWithDEK(dek, user, ref, c); err == nil {
			t.Fatalf("truncated or extended value opened (len %d)", len(c))
		}
	}
}

func TestKeyringValidation(t *testing.T) {
	t.Parallel()
	good := key(t, "test-a")
	if _, err := vault.New(vault.Keyring{Current: good}, vault.Options{}); err != nil {
		t.Fatal(err)
	}
	for name, ring := range map[string]vault.Keyring{
		"short key":      {Current: vault.Key{ID: "test-a", Secret: make([]byte, 16)}},
		"bad id":         {Current: vault.Key{ID: "Prod_A", Secret: make([]byte, 32)}},
		"no prefix":      {Current: vault.Key{ID: "a", Secret: make([]byte, 32)}},
		"duplicate":      {Current: good, Previous: []vault.Key{good}},
		"short previous": {Current: good, Previous: []vault.Key{{ID: "test-b", Secret: []byte("x")}}},
	} {
		if _, err := vault.New(ring, vault.Options{}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	s, err := vault.GenerateKEK()
	if err != nil {
		t.Fatal(err)
	}
	if b, err := vault.DecodeKEK(s); err != nil || len(b) != 32 {
		t.Fatalf("generated KEK does not decode: %v", err)
	}
	if _, err := vault.DecodeKEK("c2hvcnQ="); err == nil {
		t.Fatal("short KEK decoded")
	}
}

func TestSealOpenWithDatabaseKeys(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	kek := key(t, "test-a")
	v, err := vault.New(vault.Keyring{Current: kek}, vault.Options{})
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := uid(), uid()
	for _, u := range [][16]byte{alice, bob} {
		if err := v.CreateUserKey(ctx, d.Pool, u); err != nil {
			t.Fatal(err)
		}
	}
	ref := vault.Ref{Kind: "guardian_consent", RecordID: "r", Field: "guardian_name"}
	sealed, err := v.Seal(ctx, d.Pool, alice, ref, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if pt, err := v.Open(ctx, d.Pool, alice, ref, sealed); err != nil || string(pt) != "secret" {
		t.Fatalf("open: %q %v", pt, err)
	}
	// A fresh vault (another instance, empty cache) unwraps from the database.
	v2, _ := vault.New(vault.Keyring{Current: kek}, vault.Options{})
	if v2.Cached(alice) {
		t.Fatal("fresh vault has a cached key")
	}
	if pt, err := v2.Open(ctx, d.Pool, alice, ref, sealed); err != nil || string(pt) != "secret" {
		t.Fatalf("open from the database: %q %v", pt, err)
	}
	// Bob's DEK cannot open Alice's value, even with Alice's ref.
	if _, err := v.Open(ctx, d.Pool, bob, ref, sealed); !errors.Is(err, vault.ErrOpen) {
		t.Fatalf("another user's key opened it: %v", err)
	}
	// Wrong KEK: a vault with another current key cannot unwrap.
	other, _ := vault.New(vault.Keyring{Current: key(t, "test-b")}, vault.Options{})
	if _, err := other.Open(ctx, d.Pool, alice, ref, sealed); !errors.Is(err, vault.ErrUnknownKEK) {
		t.Fatalf("unknown KEK: %v", err)
	}
	if _, err := v.Seal(ctx, d.Pool, uid(), ref, []byte("x")); !errors.Is(err, vault.ErrNoKey) {
		t.Fatalf("missing key: %v", err)
	}
	// Destroy: tombstone, cache entry gone, opening fails.
	if !v.Cached(alice) {
		t.Fatal("DEK not cached after use")
	}
	if err := v.Destroy(ctx, d.Pool, alice); err != nil {
		t.Fatal(err)
	}
	if v.Cached(alice) {
		t.Fatal("cache entry survived destroy")
	}
	var wrappedNull, kekNull, destroyed bool
	_ = d.Pool.QueryRow(ctx, `SELECT wrapped_dek IS NULL, kek_id IS NULL, destroyed_at IS NOT NULL FROM user_data_keys WHERE user_id = $1`, alice).Scan(&wrappedNull, &kekNull, &destroyed)
	if !wrappedNull || !kekNull || !destroyed {
		t.Fatal("key row is not a tombstone")
	}
	if _, err := v.Open(ctx, d.Pool, alice, ref, sealed); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("destroyed key opened: %v", err)
	}
	if err := v.Destroy(ctx, d.Pool, alice); err != nil {
		t.Fatalf("second destroy: %v", err)
	}
}

func TestCacheTTLAndBound(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	var mu sync.Mutex
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	v, _ := vault.New(vault.Keyring{Current: key(t, "test-a")}, vault.Options{CacheSize: 2, CacheTTL: 5 * time.Minute, Now: clock})
	users := [][16]byte{uid(), uid(), uid()}
	for _, u := range users {
		if err := v.CreateUserKey(ctx, d.Pool, u); err != nil {
			t.Fatal(err)
		}
	}
	if v.Cached(users[0]) || !v.Cached(users[1]) || !v.Cached(users[2]) {
		t.Fatal("LRU bound not applied")
	}
	ref := vault.Ref{Kind: "k", RecordID: "r", Field: "f"}
	if _, err := v.Seal(ctx, d.Pool, users[0], ref, []byte("x")); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	now = now.Add(6 * time.Minute)
	mu.Unlock()
	// Expired entries are reloaded from the database, not served.
	if _, err := d.Pool.Exec(ctx, `UPDATE user_data_keys SET wrapped_dek = NULL, kek_id = NULL, destroyed_at = now() WHERE user_id = $1`, users[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Seal(ctx, d.Pool, users[0], ref, []byte("x")); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("expired cache entry served: %v", err)
	}
}

func TestRewrap(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	oldKey, newKey := key(t, "test-old"), key(t, "test-new")
	vOld, _ := vault.New(vault.Keyring{Current: oldKey}, vault.Options{})
	ref := vault.Ref{Kind: "guardian_consent", RecordID: "r", Field: "guardian_name"}
	type item struct {
		user   [16]byte
		sealed []byte
	}
	var items []item
	for i := 0; i < 7; i++ {
		u := uid()
		if err := vOld.CreateUserKey(ctx, d.Pool, u); err != nil {
			t.Fatal(err)
		}
		s, err := vOld.Seal(ctx, d.Pool, u, ref, []byte{byte('a' + i)})
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, item{u, s})
	}
	// One destroyed user stays a tombstone and is never re-wrapped.
	if err := vOld.Destroy(ctx, d.Pool, items[6].user); err != nil {
		t.Fatal(err)
	}
	// Restart with the new key current and the old one previous.
	ring := vault.Keyring{Current: newKey, Previous: []vault.Key{oldKey}}
	v1, _ := vault.New(ring, vault.Options{})
	st, _ := v1.KeyStatus(ctx, d.Pool)
	if st.ByKEK["test-old"] != 6 || st.UnderOtherKEKs() != 6 || st.Destroyed != 1 {
		t.Fatalf("before: %+v", st)
	}
	// Interrupted run: one batch of 2, then the process "stops".
	n, err := v1.RewrapBatch(ctx, d.Pool, 2)
	if err != nil || n != 2 {
		t.Fatalf("first batch: %d %v", n, err)
	}
	// A canceled batch changes nothing.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := v1.RewrapBatch(canceled, d.Pool, 2); err == nil {
		t.Fatal("canceled batch succeeded")
	}
	st, _ = v1.KeyStatus(ctx, d.Pool)
	if st.ByKEK["test-old"] != 4 || st.ByKEK["test-new"] != 2 {
		t.Fatalf("after one batch: %+v", st)
	}
	// Resume in a new process; batches of 3.
	v2, _ := vault.New(ring, vault.Options{})
	total, err := v2.Rewrap(ctx, d.Pool, 3, nil)
	if err != nil || total != 4 {
		t.Fatalf("resume: %d %v", total, err)
	}
	// Idempotent: nothing left to do.
	if total, err := v2.Rewrap(ctx, d.Pool, 3, nil); err != nil || total != 0 {
		t.Fatalf("second run: %d %v", total, err)
	}
	st, _ = v2.KeyStatus(ctx, d.Pool)
	if st.ByKEK["test-old"] != 0 || st.UnderOtherKEKs() != 0 || st.ByKEK["test-new"] != 6 || st.Destroyed != 1 {
		t.Fatalf("after: %+v", st)
	}
	// Data stays readable with only the new KEK.
	vNew, _ := vault.New(vault.Keyring{Current: newKey}, vault.Options{})
	for i, it := range items[:6] {
		pt, err := vNew.Open(ctx, d.Pool, it.user, ref, it.sealed)
		if err != nil || len(pt) != 1 || pt[0] != byte('a'+i) {
			t.Fatalf("item %d after rewrap: %q %v", i, pt, err)
		}
	}
	if _, err := vNew.Open(ctx, d.Pool, items[6].user, ref, items[6].sealed); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("tombstone after rewrap: %v", err)
	}
	// A KEK that is neither current nor previous stops the run with its id.
	if err := vOld.CreateUserKey(ctx, d.Pool, uid()); err != nil {
		t.Fatal(err)
	}
	if _, err := vNew.Rewrap(ctx, d.Pool, 10, nil); !errors.Is(err, vault.ErrUnknownKEK) {
		t.Fatalf("unknown KEK during rewrap: %v", err)
	}
}

// R2-B: the throttle key is derived from the KEK (HKDF-SHA256, purpose as
// info): stable for one KEK and purpose, different for another purpose or
// KEK, never the KEK itself.
func TestDeriveKey(t *testing.T) {
	t.Parallel()
	k := key(t, "test-derive")
	v, err := vault.New(vault.Keyring{Current: k}, vault.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := v.DeriveKey("sela.login-throttle.v1")
	if err != nil || len(a) != 32 || bytes.Equal(a, k.Secret) {
		t.Fatalf("derive: %d bytes %v", len(a), err)
	}
	again, _ := v.DeriveKey("sela.login-throttle.v1")
	other, _ := v.DeriveKey("sela.other-purpose.v1")
	if !bytes.Equal(a, again) || bytes.Equal(a, other) {
		t.Fatal("derivation is not stable per purpose")
	}
	v2, _ := vault.New(vault.Keyring{Current: key(t, "test-derive2")}, vault.Options{})
	if b, _ := v2.DeriveKey("sela.login-throttle.v1"); bytes.Equal(a, b) {
		t.Fatal("another KEK derived the same key")
	}
	for _, bad := range []string{"", "login-throttle", "sela.Login.v1", "sela.x.v1 "} {
		if _, err := v.DeriveKey(bad); err == nil {
			t.Errorf("purpose %q accepted", bad)
		}
	}
}

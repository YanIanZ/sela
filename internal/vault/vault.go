// Package vault is Sela's per-user envelope encryption (ADR-0002 §5).
//
// A key-encryption key (KEK) from the environment wraps one random
// data-encryption key (DEK) per user; the DEK never changes for the life of
// the user, so destroying its wrapped copy (crypto-shredding) makes every
// copy of that user's ciphertext unreadable, older backups included. Fields
// are sealed with AES-256-GCM under the DEK, with associated data that
// binds each ciphertext to its user, record kind, record id and field.
//
// Key material and plaintext are never logged, serialised or returned in
// errors.
package vault

import (
	"container/list"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/db"
)

// Format constants.
const (
	// FormatV1 is the first byte of every sealed value and wrapped DEK.
	FormatV1  = 0x01
	NonceSize = 12
	TagSize   = 16
	KeySize   = 32
	// Overhead is the size a sealed value adds to its plaintext.
	Overhead = 1 + NonceSize + TagSize

	dekAAD   = "sela.dek.v1"
	fieldAAD = "sela.aad.v1"
)

// Errors. None carries key material or plaintext.
var (
	ErrKeyDestroyed = errors.New("vault: the user's data key was destroyed")
	ErrNoKey        = errors.New("vault: the user has no data key")
	ErrUnknownKEK   = errors.New("vault: data key wrapped by an unknown KEK")
	ErrOpen         = errors.New("vault: ciphertext failed authentication")
	ErrFormat       = errors.New("vault: unknown ciphertext format")
	ErrBadKey       = errors.New("vault: a KEK must be exactly 32 bytes")
)

// KEKIDPattern is the shape of a KEK id; the prefix names its environment.
var KEKIDPattern = regexp.MustCompile(`^(dev|test|demo|staging|prod)-[a-z0-9-]{1,40}$`)

// Key is one key-encryption key.
type Key struct {
	ID     string
	Secret []byte
}

// Keyring holds the current KEK (wraps new and re-wrapped DEKs) and the
// previous ones (unwrap only, during rotation).
type Keyring struct {
	Current  Key
	Previous []Key
}

func (k Keyring) lookup(id string) ([]byte, bool) {
	if id == k.Current.ID {
		return k.Current.Secret, true
	}
	for _, p := range k.Previous {
		if p.ID == id {
			return p.Secret, true
		}
	}
	return nil, false
}

// Ref names the place a ciphertext belongs to. It is bound into the AAD.
type Ref struct {
	Kind     string // record kind, e.g. guardian_consent
	RecordID string // the record's id
	Field    string // e.g. guardian_name
}

// Options configures a Vault.
type Options struct {
	// CacheSize bounds unwrapped DEKs kept in memory (default 1024).
	CacheSize int
	// CacheTTL bounds how long one stays cached (default 5 minutes).
	CacheTTL time.Duration
	// Now is the clock (tests inject one).
	Now func() time.Time
}

// Vault seals and opens fields with per-user DEKs.
type Vault struct {
	ring  Keyring
	cache *dekCache
	now   func() time.Time
}

// New validates the keyring and builds a vault.
func New(ring Keyring, o Options) (*Vault, error) {
	if !KEKIDPattern.MatchString(ring.Current.ID) {
		return nil, errors.New("vault: invalid current KEK id")
	}
	if len(ring.Current.Secret) != KeySize {
		return nil, ErrBadKey
	}
	seen := map[string]bool{ring.Current.ID: true}
	for _, p := range ring.Previous {
		if !KEKIDPattern.MatchString(p.ID) || seen[p.ID] {
			return nil, errors.New("vault: invalid or duplicate previous KEK id")
		}
		if len(p.Secret) != KeySize {
			return nil, ErrBadKey
		}
		seen[p.ID] = true
	}
	if o.CacheSize <= 0 {
		o.CacheSize = 1024
	}
	if o.CacheTTL <= 0 {
		o.CacheTTL = 5 * time.Minute
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Vault{ring: ring, cache: newCache(o.CacheSize, o.CacheTTL, o.Now), now: o.Now}, nil
}

// CurrentKEKID is the id new DEKs are wrapped with (not secret).
func (v *Vault) CurrentKEKID() string { return v.ring.Current.ID }

// purposePattern is the shape of a DeriveKey purpose ("sela.<name>.v<n>").
var purposePattern = regexp.MustCompile(`^sela\.[a-z0-9-]{1,40}\.v[0-9]{1,3}$`)

// DeriveKey returns a 32-byte key for one purpose, derived with
// HKDF-SHA256 from the current KEK with the purpose as info. Like the KEK
// it lives only in process memory, never in the database or a backup; it
// changes when the KEK rotates, so it must only key data that can be
// recomputed or forgotten (the login throttle). It never wraps DEKs.
func (v *Vault) DeriveKey(purpose string) ([]byte, error) {
	if !purposePattern.MatchString(purpose) {
		return nil, errors.New("vault: invalid key purpose")
	}
	return hkdf.Key(sha256.New, v.ring.Current.Secret, nil, purpose, KeySize)
}

// GenerateKEK returns a fresh 32-byte KEK in standard base64, for the
// operator to store in the secret manager. The server never persists one.
func GenerateKEK() (string, error) {
	b := make([]byte, KeySize)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeKEK decodes a standard-base64 KEK and checks its length.
func DecodeKEK(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != KeySize {
		return nil, ErrBadKey
	}
	return b, nil
}

func gcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func seal(key, plaintext, aad []byte) ([]byte, error) {
	a, err := gcm(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 1+NonceSize, 1+NonceSize+len(plaintext)+TagSize)
	out[0] = FormatV1
	if _, err := rand.Read(out[1 : 1+NonceSize]); err != nil {
		return nil, err
	}
	return a.Seal(out, out[1:1+NonceSize], plaintext, aad), nil
}

func open(key, sealed, aad []byte) ([]byte, error) {
	if len(sealed) < Overhead {
		return nil, ErrOpen
	}
	if sealed[0] != FormatV1 {
		return nil, ErrFormat
	}
	a, err := gcm(key)
	if err != nil {
		return nil, err
	}
	pt, err := a.Open(nil, sealed[1:1+NonceSize], sealed[1+NonceSize:], aad)
	if err != nil {
		return nil, ErrOpen
	}
	return pt, nil
}

func appendField(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

// FieldAAD is "sela.aad.v1" followed by the uvarint-length-prefixed user
// id (canonical uuid text), record kind, record id and field.
func FieldAAD(userID [16]byte, ref Ref) []byte {
	b := []byte(fieldAAD)
	b = appendField(b, uuidString(userID))
	b = appendField(b, ref.Kind)
	b = appendField(b, ref.RecordID)
	return appendField(b, ref.Field)
}

// dekAADFor is "sela.dek.v1" ‖ user_id (16 bytes) ‖ uvarint(len) kek_id ‖
// dek_version (2 bytes, big endian).
func dekAADFor(userID [16]byte, kekID string, version uint16) []byte {
	b := append([]byte(dekAAD), userID[:]...)
	b = appendField(b, kekID)
	return binary.BigEndian.AppendUint16(b, version)
}

func uuidString(u [16]byte) string { return pgtype.UUID{Bytes: u, Valid: true}.String() }

func pgUUID(u [16]byte) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

// SealWithDEK and OpenWithDEK expose the field format for tests and tools
// that hold a DEK; application code uses Seal and Open.
func SealWithDEK(dek []byte, userID [16]byte, ref Ref, plaintext []byte) ([]byte, error) {
	return seal(dek, plaintext, FieldAAD(userID, ref))
}

// OpenWithDEK opens a sealed field with a known DEK.
func OpenWithDEK(dek []byte, userID [16]byte, ref Ref, sealed []byte) ([]byte, error) {
	return open(dek, sealed, FieldAAD(userID, ref))
}

// CreateUserKey creates the user's DEK and stores it wrapped with the
// current KEK, in the caller's transaction (the same one that creates the
// user).
func (v *Vault) CreateUserKey(ctx context.Context, q db.DBTX, userID [16]byte) error {
	dek := make([]byte, KeySize)
	if _, err := rand.Read(dek); err != nil {
		return err
	}
	wrapped, err := seal(v.ring.Current.Secret, dek, dekAADFor(userID, v.ring.Current.ID, 1))
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO user_data_keys (user_id, wrapped_dek, kek_id, dek_version, created_at) VALUES ($1, $2, $3, 1, $4)`,
		pgUUID(userID), wrapped, v.ring.Current.ID, v.now())
	if err != nil {
		return err
	}
	v.cache.put(userID, dek)
	return nil
}

// Destroy crypto-shreds the user's DEK: the row becomes a tombstone
// (wrapped_dek and kek_id NULL, destroyed_at set) in the caller's
// transaction, and the cached copy is dropped at once. Destroying a key
// that is already destroyed is a no-op; a missing row gets a tombstone.
func (v *Vault) Destroy(ctx context.Context, q db.DBTX, userID [16]byte) error {
	v.cache.evict(userID)
	now := v.now()
	_, err := q.Exec(ctx, `INSERT INTO user_data_keys (user_id, wrapped_dek, kek_id, created_at, destroyed_at)
		VALUES ($1, NULL, NULL, $2, $2)
		ON CONFLICT (user_id) DO UPDATE SET wrapped_dek = NULL, kek_id = NULL,
		  destroyed_at = coalesce(user_data_keys.destroyed_at, EXCLUDED.destroyed_at)`, pgUUID(userID), now)
	return err
}

// Evict drops a cached DEK (after a deletion commits, or a rollback).
func (v *Vault) Evict(userID [16]byte) { v.cache.evict(userID) }

// Cached reports whether a DEK is in the in-process cache (tests).
func (v *Vault) Cached(userID [16]byte) bool { return v.cache.has(userID) }

func (v *Vault) dek(ctx context.Context, q db.DBTX, userID [16]byte) ([]byte, error) {
	if dek, ok := v.cache.get(userID); ok {
		return dek, nil
	}
	var wrapped []byte
	var kekID *string
	var version int16
	var destroyed *time.Time
	err := q.QueryRow(ctx, `SELECT wrapped_dek, kek_id, dek_version, destroyed_at FROM user_data_keys WHERE user_id = $1`,
		pgUUID(userID)).Scan(&wrapped, &kekID, &version, &destroyed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, err
	}
	if destroyed != nil || wrapped == nil || kekID == nil {
		return nil, ErrKeyDestroyed
	}
	kek, ok := v.ring.lookup(*kekID)
	if !ok {
		return nil, fmt.Errorf("%w (%s)", ErrUnknownKEK, *kekID)
	}
	dek, err := open(kek, wrapped, dekAADFor(userID, *kekID, uint16(version)))
	if err != nil {
		return nil, err
	}
	v.cache.put(userID, dek)
	return dek, nil
}

// Seal encrypts plaintext for one field of one record of the user.
func (v *Vault) Seal(ctx context.Context, q db.DBTX, userID [16]byte, ref Ref, plaintext []byte) ([]byte, error) {
	dek, err := v.dek(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	return SealWithDEK(dek, userID, ref, plaintext)
}

// Open decrypts a sealed field. It fails for another user, kind, record or
// field, for tampered bytes, and once the user's key is destroyed.
func (v *Vault) Open(ctx context.Context, q db.DBTX, userID [16]byte, ref Ref, sealed []byte) ([]byte, error) {
	dek, err := v.dek(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	return OpenWithDEK(dek, userID, ref, sealed)
}

// Status counts live DEKs per KEK id and destroyed tombstones.
type Status struct {
	CurrentKEKID string
	ByKEK        map[string]int
	Destroyed    int
}

// UnderOtherKEKs counts live DEKs not wrapped by the current KEK.
func (s Status) UnderOtherKEKs() int {
	n := 0
	for id, c := range s.ByKEK {
		if id != s.CurrentKEKID {
			n += c
		}
	}
	return n
}

// KeyStatus reports the key table (the "sela keys status" command).
func (v *Vault) KeyStatus(ctx context.Context, q db.DBTX) (Status, error) {
	st := Status{CurrentKEKID: v.ring.Current.ID, ByKEK: map[string]int{}}
	rows, err := q.Query(ctx, `SELECT coalesce(kek_id, ''), destroyed_at IS NOT NULL, count(*) FROM user_data_keys GROUP BY 1, 2`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var destroyed bool
		var n int
		if err := rows.Scan(&id, &destroyed, &n); err != nil {
			return st, err
		}
		if destroyed {
			st.Destroyed += n
		} else {
			st.ByKEK[id] += n
		}
	}
	return st, rows.Err()
}

// RewrapBatch re-wraps up to batch DEKs that are not under the current
// KEK, in one transaction. Rows are claimed with FOR UPDATE SKIP LOCKED and
// updated only while still under their old KEK, so concurrent or repeated
// runs are safe; it returns how many rows it re-wrapped.
func (v *Vault) RewrapBatch(ctx context.Context, b db.Beginner, batch int) (int, error) {
	if batch <= 0 {
		batch = 500
	}
	n := 0
	err := db.InTx(ctx, b, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT user_id, wrapped_dek, kek_id, dek_version FROM user_data_keys
			WHERE destroyed_at IS NULL AND kek_id <> $1 ORDER BY user_id LIMIT $2 FOR UPDATE SKIP LOCKED`, v.ring.Current.ID, batch)
		if err != nil {
			return err
		}
		type row struct {
			id      pgtype.UUID
			wrapped []byte
			kek     string
			version int16
		}
		list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
			var x row
			err := r.Scan(&x.id, &x.wrapped, &x.kek, &x.version)
			return x, err
		})
		if err != nil {
			return err
		}
		for _, r := range list {
			old, ok := v.ring.lookup(r.kek)
			if !ok {
				return fmt.Errorf("%w (%s); add it to SELA_DATA_KEK_PREVIOUS", ErrUnknownKEK, r.kek)
			}
			dek, err := open(old, r.wrapped, dekAADFor(r.id.Bytes, r.kek, uint16(r.version)))
			if err != nil {
				return err
			}
			wrapped, err := seal(v.ring.Current.Secret, dek, dekAADFor(r.id.Bytes, v.ring.Current.ID, uint16(r.version)))
			clear(dek)
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE user_data_keys SET wrapped_dek = $1, kek_id = $2, rewrapped_at = $3
				WHERE user_id = $4 AND kek_id = $5 AND destroyed_at IS NULL`, wrapped, v.ring.Current.ID, v.now(), r.id, r.kek)
			if err != nil {
				return err
			}
			n += int(tag.RowsAffected())
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Rewrap runs batches until no DEK is left under an old KEK (the
// "sela keys rewrap" command). It is idempotent and resumable: an
// interrupted run leaves every row either fully old or fully new.
func (v *Vault) Rewrap(ctx context.Context, b db.Beginner, batch int, progress func(done int)) (int, error) {
	total := 0
	for {
		n, err := v.RewrapBatch(ctx, b, batch)
		total += n
		if progress != nil && n > 0 {
			progress(total)
		}
		if err != nil || n == 0 {
			return total, err
		}
	}
}

// dekCache is a bounded LRU of unwrapped DEKs with a TTL. Evicted keys are
// zeroed.
type dekCache struct {
	mu    sync.Mutex
	items map[[16]byte]*list.Element
	lru   *list.List
	max   int
	ttl   time.Duration
	now   func() time.Time
}

type cached struct {
	id     [16]byte
	dek    []byte
	loaded time.Time
}

func newCache(max int, ttl time.Duration, now func() time.Time) *dekCache {
	return &dekCache{items: map[[16]byte]*list.Element{}, lru: list.New(), max: max, ttl: ttl, now: now}
}

func (c *dekCache) get(id [16]byte) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el := c.items[id]
	if el == nil {
		return nil, false
	}
	e := el.Value.(*cached)
	if c.now().Sub(e.loaded) > c.ttl {
		c.remove(el)
		return nil, false
	}
	c.lru.MoveToFront(el)
	// A copy: an eviction zeroes the cached bytes, which must never
	// corrupt a key another request is still using.
	return append([]byte(nil), e.dek...), true
}

func (c *dekCache) has(id [16]byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items[id] != nil
}

func (c *dekCache) put(id [16]byte, dek []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.items[id]; el != nil {
		c.remove(el)
	}
	for c.lru.Len() >= c.max {
		c.remove(c.lru.Back())
	}
	c.items[id] = c.lru.PushFront(&cached{id: id, dek: append([]byte(nil), dek...), loaded: c.now()})
}

func (c *dekCache) evict(id [16]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.items[id]; el != nil {
		c.remove(el)
	}
}

func (c *dekCache) remove(el *list.Element) {
	e := el.Value.(*cached)
	c.lru.Remove(el)
	delete(c.items, e.id)
	clear(e.dek)
}

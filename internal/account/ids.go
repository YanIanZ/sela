// Package account owns Sela v3 identities and their foundation records:
// users (guest or account), credentials (argon2id), recovery codes,
// server-side sessions, consents and their history, guardian consent
// records, the audit trail, guest purge, deletion with crypto-shredding,
// restore reconciliation and the exporter registry (ADR-0002 §3, §6-§11).
//
// Every owner-facing method takes context.Context and then the owner's
// UserID, and scopes every statement to that owner; another user's
// resource is indistinguishable from a missing one (ErrNotFound). System
// methods (sign-in, session lookup, purge, reconcile) are listed in
// SystemStore. Nothing here logs content, usernames, codes, tokens or ids.
package account

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// UserID identifies a user (uuid v4).
type UserID [16]byte

// SessionID identifies a session row (uuid v4). It is not the cookie
// token: the token is never stored, only its SHA-256.
type SessionID [16]byte

func newUUID() [16]byte {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return u
}

// NewUserID returns a random user id.
func NewUserID() UserID { return UserID(newUUID()) }

func newSessionID() SessionID { return SessionID(newUUID()) }

// ErrInvalidID is returned for a malformed id.
var ErrInvalidID = errors.New("account: invalid id")

// ParseUUID parses a canonical 8-4-4-4-12 uuid.
func ParseUUID(s string) ([16]byte, error) {
	if len(s) != 36 {
		return [16]byte{}, ErrInvalidID
	}
	var u pgtype.UUID
	if err := u.Scan(s); err != nil || !u.Valid {
		return [16]byte{}, ErrInvalidID
	}
	return u.Bytes, nil
}

func uuidText(u [16]byte) string { return pgtype.UUID{Bytes: u, Valid: true}.String() }

func (u UserID) String() string    { return uuidText(u) }
func (s SessionID) String() string { return uuidText(s) }

// UUIDValue lets pgx encode the id as uuid.
func (u UserID) UUIDValue() (pgtype.UUID, error) { return pgtype.UUID{Bytes: u, Valid: true}, nil }

// ScanUUID lets pgx decode a uuid column.
func (u *UserID) ScanUUID(v pgtype.UUID) error {
	if !v.Valid {
		return fmt.Errorf("account: NULL user id")
	}
	*u = v.Bytes
	return nil
}

// UUIDValue lets pgx encode the id as uuid.
func (s SessionID) UUIDValue() (pgtype.UUID, error) { return pgtype.UUID{Bytes: s, Valid: true}, nil }

// ScanUUID lets pgx decode a uuid column.
func (s *SessionID) ScanUUID(v pgtype.UUID) error {
	if !v.Valid {
		return fmt.Errorf("account: NULL session id")
	}
	*s = v.Bytes
	return nil
}

// Principal is the owner resolved from the session cookie. Handlers never
// take a user id from the client.
type Principal struct {
	UserID            UserID
	SessionID         SessionID
	Kind              string // guest | account
	TrustedDevice     bool
	AuthenticatedAt   time.Time
	AbsoluteExpiresAt time.Time
	IdleExpiresAt     time.Time
}

// Timestamp is an RFC 3339 UTC time with second precision in JSON.
type Timestamp time.Time

// MarshalJSON formats 2026-10-26T13:05:00Z.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).UTC().Format("2006-01-02T15:04:05Z") + `"`), nil
}

// String formats like MarshalJSON, without quotes.
func (t Timestamp) String() string { return time.Time(t).UTC().Format("2006-01-02T15:04:05Z") }

func ts(t time.Time) Timestamp { return Timestamp(t) }

func tsPtr(t *time.Time) *Timestamp {
	if t == nil {
		return nil
	}
	v := Timestamp(*t)
	return &v
}

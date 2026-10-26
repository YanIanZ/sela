package journey

import (
	"errors"
	"time"
)

// Store errors.
var (
	ErrNotFound = errors.New("journey: session not found")
	ErrCapacity = errors.New("journey: demo session capacity reached")
)

// Session lifetime policy (memory only; restart erases everything).
const (
	SessionIdle    = 30 * time.Minute // RetentionSession idle timeout
	SessionMaxLife = 2 * time.Hour    // RetentionSession absolute lifetime
	DayMaxLife     = 24 * time.Hour   // RetentionDay absolute lifetime (and idle)
)

// IdleTimeout returns the idle timeout for the session's retention.
func (s *Session) IdleTimeout() time.Duration {
	if s.Retention == RetentionDay {
		return DayMaxLife
	}
	return SessionIdle
}

// Expired reports whether the session must be purged at now.
func (s *Session) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresAt) || !now.Before(s.LastSeen.Add(s.IdleTimeout()))
}

// ShareRef locates a handoff from a share-token hash.
type ShareRef struct {
	SessionID string
	HandoffID string
}

// RecipientSession is the fictional recipient's principal. It is scoped to
// exactly one handoff of one owner session and is created only after the
// share token AND the separate access code were presented (SPEC V5, D-007).
type RecipientSession struct {
	ID             string
	SessionID      string
	HandoffID      string
	ProfessionalID string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// Stats is a metadata-only store summary.
type Stats struct {
	Sessions   int `json:"sessions"`
	Shares     int `json:"shares"`
	Recipients int `json:"recipients"`
}

// Store isolates sessions. View and Update serialise access per session and
// return ErrNotFound for unknown or expired sessions. Implementations must be
// safe for concurrent use and must never let one session reach another's data.
type Store interface {
	Create(s *Session) error
	View(id string, fn func(*Session) error) error
	Update(id string, fn func(*Session) error) error
	// Delete purges a session and every share/recipient entry pointing at it.
	Delete(id string)
	IndexShare(tokenHash string, ref ShareRef)
	UnindexShare(tokenHash string)
	LookupShare(tokenHash string) (ShareRef, bool)
	CreateRecipient(r RecipientSession) error
	Recipient(id string, now time.Time) (RecipientSession, bool)
	DeleteRecipient(id string)
	// DeleteRecipientsFor removes recipient sessions bound to one handoff.
	DeleteRecipientsFor(sessionID, handoffID string)
	// Sweep purges expired sessions and recipient sessions; returns the count purged.
	Sweep(now time.Time) int
	Stats() Stats
}

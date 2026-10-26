package journey

import (
	"errors"
	"time"
)

// Store errors.
var (
	ErrNotFound = errors.New("journey: session not found")
	ErrCapacity = errors.New("journey: demo session capacity reached")
	// ErrClientLimit: the client address already holds the maximum number of
	// live sessions.
	ErrClientLimit = errors.New("journey: too many live sessions for this client")
)

// Session lifetime policy (memory only; restart erases everything).
const (
	SessionIdle    = 30 * time.Minute // RetentionSession idle timeout
	SessionMaxLife = 2 * time.Hour    // RetentionSession absolute lifetime
	DayMaxLife     = 24 * time.Hour   // RetentionDay absolute lifetime (and idle)
)

// Per-session and per-share bounds (see CodeLimitReached).
const (
	// MaxHandoffs bounds the handoffs (receipts with their projections) one
	// session can create.
	MaxHandoffs = 20
	// MaxRecipientsPerHandoff bounds live recipient principals per handoff;
	// a new sign-in replaces the oldest, so one link can never exhaust the
	// recipient capacity of other visitors.
	MaxRecipientsPerHandoff = 3
)

// IdleTimeout returns the idle timeout for the session's retention.
func (s *Session) IdleTimeout() time.Duration {
	if s.Retention == RetentionDay {
		return DayMaxLife
	}
	return SessionIdle
}

// IdleDeadline is when the session expires without owner activity: the idle
// timeout after LastSeen, extended to the end of any active share (an owner
// is not cut off from a share they granted for being idle), never beyond the
// absolute ExpiresAt.
func (s *Session) IdleDeadline() time.Time {
	d := s.LastSeen.Add(s.IdleTimeout())
	for _, h := range s.Handoffs {
		if h.Share == ShareActive && h.ExpiresAt.After(d) {
			d = h.ExpiresAt
		}
	}
	if d.After(s.ExpiresAt) {
		d = s.ExpiresAt
	}
	return d
}

// Expired reports whether the session must be purged at now.
func (s *Session) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresAt) || !now.Before(s.IdleDeadline())
}

// Protected reports whether the user granted storage (24-hour retention) or
// a share is still active. Capacity eviction spares a protected session while
// an ordinary one can go, and until it has been idle for SessionIdle.
func (s *Session) Protected(now time.Time) bool {
	if s.Retention == RetentionDay {
		return true
	}
	for _, h := range s.Handoffs {
		if h.Share == ShareActive && now.Before(h.ExpiresAt) {
			return true
		}
	}
	return false
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
	// CreateRecipient stores a principal. At most MaxRecipientsPerHandoff
	// live principals exist per handoff: a new one replaces the oldest.
	CreateRecipient(r RecipientSession) error
	Recipient(id string, now time.Time) (RecipientSession, bool)
	DeleteRecipient(id string)
	// DeleteRecipientsFor removes recipient sessions bound to one handoff.
	DeleteRecipientsFor(sessionID, handoffID string)
	// Sweep purges expired sessions and recipient sessions; returns the count purged.
	Sweep(now time.Time) int
	Stats() Stats
}

// PurgeNotifier is implemented by stores that report every purged session
// (deletion, sweep, expiry on access, capacity eviction). The service uses
// it to cancel the session's AI operations and drop their event logs at
// once. fn must not call back into the store.
type PurgeNotifier interface {
	OnPurge(fn func(sessionID string))
}

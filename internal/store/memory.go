// Package store provides the bounded in-memory implementation of
// journey.Store. It is the only storage in the v2 demo (SPEC C7): nothing is
// written to disk, and a process restart erases every session.
package store

import (
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/journey"
)

// Memory is a bounded, per-session-locked in-memory store.
type Memory struct {
	mu          sync.RWMutex
	sessions    map[string]*entry
	shares      map[string]journey.ShareRef
	recipients  map[string]journey.RecipientSession
	maxSessions int
	maxRecips   int
	now         func() time.Time
}

type entry struct {
	mu      sync.Mutex
	s       *journey.Session
	deleted bool
}

// Options bounds the store.
type Options struct {
	MaxSessions   int
	MaxRecipients int
	Now           func() time.Time
}

// NewMemory returns an empty store.
func NewMemory(o Options) *Memory {
	if o.MaxSessions <= 0 {
		o.MaxSessions = 1000
	}
	if o.MaxRecipients <= 0 {
		o.MaxRecipients = 4 * o.MaxSessions
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Memory{
		sessions:    map[string]*entry{},
		shares:      map[string]journey.ShareRef{},
		recipients:  map[string]journey.RecipientSession{},
		maxSessions: o.MaxSessions,
		maxRecips:   o.MaxRecipients,
		now:         o.Now,
	}
}

// Create adds a new session. When full it first purges expired sessions and
// then evicts the least recently seen session idle for over 10 minutes;
// otherwise it returns ErrCapacity. Active visitors are never evicted.
func (m *Memory) Create(s *journey.Session) error {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sessions) >= m.maxSessions {
		m.sweepLocked(now)
	}
	if len(m.sessions) >= m.maxSessions {
		var victim string
		var oldest time.Time
		for id, e := range m.sessions {
			if !e.mu.TryLock() {
				continue // in use: not idle
			}
			seen := e.s.LastSeen
			e.mu.Unlock()
			if now.Sub(seen) < 10*time.Minute {
				continue
			}
			if victim == "" || seen.Before(oldest) {
				victim, oldest = id, seen
			}
		}
		if victim == "" {
			return journey.ErrCapacity
		}
		m.deleteLocked(victim)
	}
	if _, exists := m.sessions[s.ID]; exists {
		return journey.ErrCapacity // id collision: practically impossible with 256-bit ids
	}
	m.sessions[s.ID] = &entry{s: s}
	return nil
}

func (m *Memory) get(id string) *entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

func (m *Memory) with(id string, fn func(*journey.Session) error) error {
	e := m.get(id)
	if e == nil {
		return journey.ErrNotFound
	}
	e.mu.Lock()
	// Re-check membership under the entry lock: a concurrent Delete may have
	// removed the session after get() returned.
	if e.deleted || m.get(id) != e {
		e.mu.Unlock()
		return journey.ErrNotFound
	}
	if e.s.Expired(m.now()) {
		e.mu.Unlock()
		m.Delete(id)
		return journey.ErrNotFound
	}
	defer e.mu.Unlock()
	return fn(e.s)
}

// View runs fn with exclusive access to the session. fn must not retain s.
func (m *Memory) View(id string, fn func(*journey.Session) error) error { return m.with(id, fn) }

// Update runs fn with exclusive access to the session. fn must not retain s.
func (m *Memory) Update(id string, fn func(*journey.Session) error) error { return m.with(id, fn) }

// Delete purges the session and every index entry pointing at it.
func (m *Memory) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteLocked(id)
}

func (m *Memory) deleteLocked(id string) {
	e := m.sessions[id]
	delete(m.sessions, id)
	for h, ref := range m.shares {
		if ref.SessionID == id {
			delete(m.shares, h)
		}
	}
	for rid, r := range m.recipients {
		if r.SessionID == id {
			delete(m.recipients, rid)
		}
	}
	if e != nil {
		// Mark deleted without blocking on an in-flight Update (Delete may be
		// called from inside Update). Later accesses also fail the map
		// membership check in with().
		if e.mu.TryLock() {
			e.deleted = true
			e.s = &journey.Session{}
			e.mu.Unlock()
		} else {
			go func() {
				e.mu.Lock()
				e.deleted = true
				e.s = &journey.Session{}
				e.mu.Unlock()
			}()
		}
	}
}

// IndexShare maps a share-token hash to its handoff.
func (m *Memory) IndexShare(tokenHash string, ref journey.ShareRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[ref.SessionID]; ok {
		m.shares[tokenHash] = ref
	}
}

// UnindexShare removes a share-token hash.
func (m *Memory) UnindexShare(tokenHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.shares, tokenHash)
}

// LookupShare resolves a share-token hash.
func (m *Memory) LookupShare(tokenHash string) (journey.ShareRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ref, ok := m.shares[tokenHash]
	return ref, ok
}

// CreateRecipient stores a recipient principal bound to an existing session.
func (m *Memory) CreateRecipient(r journey.RecipientSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[r.SessionID]; !ok {
		return journey.ErrNotFound
	}
	if len(m.recipients) >= m.maxRecips {
		now := m.now()
		for id, x := range m.recipients {
			if !now.Before(x.ExpiresAt) {
				delete(m.recipients, id)
			}
		}
		if len(m.recipients) >= m.maxRecips {
			return journey.ErrCapacity
		}
	}
	m.recipients[r.ID] = r
	return nil
}

// Recipient returns an unexpired recipient principal whose owner session still exists.
func (m *Memory) Recipient(id string, now time.Time) (journey.RecipientSession, bool) {
	m.mu.RLock()
	r, ok := m.recipients[id]
	_, owner := m.sessions[r.SessionID]
	m.mu.RUnlock()
	if !ok || !owner || !now.Before(r.ExpiresAt) {
		return journey.RecipientSession{}, false
	}
	return r, true
}

// DeleteRecipient removes a recipient principal.
func (m *Memory) DeleteRecipient(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recipients, id)
}

// DeleteRecipientsFor removes recipient principals bound to one handoff.
func (m *Memory) DeleteRecipientsFor(sessionID, handoffID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, r := range m.recipients {
		if r.SessionID == sessionID && r.HandoffID == handoffID {
			delete(m.recipients, id)
		}
	}
}

// Sweep purges expired sessions and recipient principals.
func (m *Memory) Sweep(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sweepLocked(now)
}

func (m *Memory) sweepLocked(now time.Time) int {
	purged := 0
	for id, e := range m.sessions {
		if !e.mu.TryLock() {
			continue // in use right now, so not idle; checked again next sweep
		}
		expired := e.deleted || e.s.Expired(now)
		e.mu.Unlock()
		if expired {
			m.deleteLocked(id)
			purged++
		}
	}
	for id, r := range m.recipients {
		if !now.Before(r.ExpiresAt) {
			delete(m.recipients, id)
		}
	}
	return purged
}

// Stats returns counts only.
func (m *Memory) Stats() journey.Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return journey.Stats{Sessions: len(m.sessions), Shares: len(m.shares), Recipients: len(m.recipients)}
}

var _ journey.Store = (*Memory)(nil)

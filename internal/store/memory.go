// Package store provides the bounded in-memory implementation of
// journey.Store. It is the only storage in the v2 demo (SPEC C7): nothing is
// written to disk, and a process restart erases every session.
package store

import (
	"sort"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/journey"
)

// Memory is a bounded, per-session-locked in-memory store.
type Memory struct {
	mu            sync.RWMutex
	sessions      map[string]*entry
	shares        map[string]journey.ShareRef
	recipients    map[string]recipient
	recipSeq      uint64
	maxSessions   int
	maxRecips     int
	maxPerHandoff int
	onPurge       func(sessionID string)
	now           func() time.Time
}

// recipient is a stored principal; seq orders sign-ins for replacement.
type recipient struct {
	journey.RecipientSession
	seq uint64
}

type entry struct {
	mu      sync.Mutex
	s       *journey.Session
	deleted bool
}

// Options bounds the store.
type Options struct {
	MaxSessions int
	// MaxRecipients bounds all recipient principals. The default fits every
	// principal the per-handoff and per-session bounds allow, so one
	// visitor's sign-ins can never exhaust another's.
	MaxRecipients int
	// MaxRecipientsPerHandoff bounds live principals per handoff; a new
	// sign-in replaces the oldest (default journey.MaxRecipientsPerHandoff).
	MaxRecipientsPerHandoff int
	Now                     func() time.Time
}

// NewMemory returns an empty store.
func NewMemory(o Options) *Memory {
	if o.MaxSessions <= 0 {
		o.MaxSessions = 1000
	}
	if o.MaxRecipientsPerHandoff <= 0 {
		o.MaxRecipientsPerHandoff = journey.MaxRecipientsPerHandoff
	}
	if o.MaxRecipients <= 0 {
		o.MaxRecipients = o.MaxSessions * journey.MaxHandoffs * o.MaxRecipientsPerHandoff
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Memory{
		sessions:      map[string]*entry{},
		shares:        map[string]journey.ShareRef{},
		recipients:    map[string]recipient{},
		maxSessions:   o.MaxSessions,
		maxRecips:     o.MaxRecipients,
		maxPerHandoff: o.MaxRecipientsPerHandoff,
		now:           o.Now,
	}
}

// OnPurge registers fn (journey.PurgeNotifier). It is called with the id of
// every session the store purges (Delete, Sweep, expiry on access, capacity
// eviction), after the store's index lock is released. fn must not call
// back into the store: Delete may run inside that session's Update.
func (m *Memory) OnPurge(fn func(sessionID string)) {
	m.mu.Lock()
	m.onPurge = fn
	m.mu.Unlock()
}

func (m *Memory) notify(ids []string) {
	if len(ids) == 0 {
		return
	}
	m.mu.RLock()
	fn := m.onPurge
	m.mu.RUnlock()
	if fn == nil {
		return
	}
	for _, id := range ids {
		fn(id)
	}
}

// Capacity eviction thresholds. An ordinary session idle for evictIdle may
// make room for a new visitor. A protected session (storage consent or an
// active share, journey.Session.Protected) goes only when no ordinary session
// can, and only once idle for the ordinary idle timeout. Protection thus
// never ends a session sooner than it would have ended without it, and
// sessions left idle, protected or not, hold the store for at most that
// timeout: one client filling it cannot lock other visitors out for the
// 24-hour retention.
const (
	evictIdle          = 10 * time.Minute
	evictProtectedIdle = journey.SessionIdle
)

// Create adds a new session. When full it first purges expired sessions and
// then evicts one idle session (see evictIdle): an ordinary session before a
// protected one, then the least recently seen. Otherwise it returns
// ErrCapacity. Active visitors are never evicted.
func (m *Memory) Create(s *journey.Session) error {
	purged, err := m.create(s)
	m.notify(purged)
	return err
}

func (m *Memory) create(s *journey.Session) ([]string, error) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	var purged []string
	if len(m.sessions) >= m.maxSessions {
		purged = m.sweepLocked(now)
	}
	if len(m.sessions) >= m.maxSessions {
		var victim string
		var oldest time.Time
		var victimProtected bool
		for id, e := range m.sessions {
			if !e.mu.TryLock() {
				continue // in use: not idle
			}
			seen, protected := e.s.LastSeen, e.s.Protected(now)
			e.mu.Unlock()
			if idle := now.Sub(seen); idle < evictIdle || protected && idle < evictProtectedIdle {
				continue
			}
			if victim == "" || victimProtected && !protected || victimProtected == protected && seen.Before(oldest) {
				victim, oldest, victimProtected = id, seen, protected
			}
		}
		if victim == "" {
			return purged, journey.ErrCapacity
		}
		m.deleteLocked(victim)
		purged = append(purged, victim)
	}
	if _, exists := m.sessions[s.ID]; exists {
		return purged, journey.ErrCapacity // id collision: practically impossible with 256-bit ids
	}
	m.sessions[s.ID] = &entry{s: s}
	return purged, nil
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
	existed := m.deleteLocked(id)
	m.mu.Unlock()
	if existed {
		m.notify([]string{id})
	}
}

// deleteLocked removes the session and its index entries; it reports
// whether the session existed.
func (m *Memory) deleteLocked(id string) bool {
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
	return e != nil
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
// A handoff keeps at most maxPerHandoff principals: the oldest are replaced.
func (m *Memory) CreateRecipient(r journey.RecipientSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[r.SessionID]; !ok {
		return journey.ErrNotFound
	}
	var same []recipient
	for _, x := range m.recipients {
		if x.SessionID == r.SessionID && x.HandoffID == r.HandoffID {
			same = append(same, x)
		}
	}
	sort.Slice(same, func(i, j int) bool { return same[i].seq < same[j].seq })
	for i := 0; i <= len(same)-m.maxPerHandoff; i++ {
		delete(m.recipients, same[i].ID)
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
	m.recipSeq++
	m.recipients[r.ID] = recipient{RecipientSession: r, seq: m.recipSeq}
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
	return r.RecipientSession, true
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
	purged := m.sweepLocked(now)
	m.mu.Unlock()
	m.notify(purged)
	return len(purged)
}

func (m *Memory) sweepLocked(now time.Time) []string {
	var purged []string
	for id, e := range m.sessions {
		if !e.mu.TryLock() {
			continue // in use right now, so not idle; checked again next sweep
		}
		expired := e.deleted || e.s.Expired(now)
		e.mu.Unlock()
		if expired {
			m.deleteLocked(id)
			purged = append(purged, id)
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

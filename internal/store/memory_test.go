package store

import (
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/journey"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newSession(id string, now time.Time) *journey.Session {
	return &journey.Session{ID: id, CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(journey.SessionMaxLife), Retention: journey.RetentionSession}
}

func TestConcurrentUpdatesAreSerialisedPerSession(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{Now: c.now})
	if err := m.Create(newSession("a", c.now())); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Update("a", func(s *journey.Session) error { s.Version++; return nil })
		}()
	}
	wg.Wait()
	var v int
	_ = m.View("a", func(s *journey.Session) error { v = s.Version; return nil })
	if v != 200 {
		t.Fatalf("lost updates: version %d", v)
	}
}

func TestDeleteInsideUpdateAndConcurrentAccess(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{Now: c.now})
	_ = m.Create(newSession("a", c.now()))
	m.IndexShare("tok", journey.ShareRef{SessionID: "a", HandoffID: "h1"})
	_ = m.CreateRecipient(journey.RecipientSession{ID: "r1", SessionID: "a", HandoffID: "h1", ExpiresAt: c.now().Add(time.Hour)})
	if err := m.Update("a", func(*journey.Session) error { m.Delete("a"); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := m.View("a", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatalf("deleted session still visible: %v", err)
	}
	if _, ok := m.LookupShare("tok"); ok {
		t.Fatal("share index survived deletion")
	}
	if _, ok := m.Recipient("r1", c.now()); ok {
		t.Fatal("recipient survived deletion")
	}
	// Racing deletes and updates never panic or resurrect a session.
	_ = m.Create(newSession("b", c.now()))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _ = m.Update("b", func(s *journey.Session) error { s.Version++; return nil }) }()
		go func() { defer wg.Done(); m.Delete("b") }()
	}
	wg.Wait()
	if err := m.View("b", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatal("session resurrected")
	}
}

func TestExpiryAndSweep(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{Now: c.now})
	_ = m.Create(newSession("idle", c.now()))
	day := newSession("day", c.now())
	day.Retention, day.ExpiresAt = journey.RetentionDay, c.now().Add(journey.DayMaxLife)
	_ = m.Create(day)
	c.add(journey.SessionIdle + time.Second)
	if n := m.Sweep(c.now()); n != 1 {
		t.Fatalf("sweep purged %d, want 1", n)
	}
	if err := m.View("idle", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatal("idle session not purged")
	}
	if err := m.View("day", func(*journey.Session) error { return nil }); err != nil {
		t.Fatal("day-retention session purged early")
	}
	c.add(journey.DayMaxLife)
	if err := m.View("day", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatal("expired session still visible on access")
	}
}

func TestCapacityEvictsOnlyIdleSessions(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{MaxSessions: 2, Now: c.now})
	_ = m.Create(newSession("a", c.now()))
	_ = m.Create(newSession("b", c.now()))
	if err := m.Create(newSession("c", c.now())); err != journey.ErrCapacity {
		t.Fatalf("want capacity error, got %v", err)
	}
	c.add(11 * time.Minute)
	_ = m.Update("b", func(s *journey.Session) error { s.LastSeen = c.now(); return nil })
	if err := m.Create(newSession("c", c.now())); err != nil {
		t.Fatalf("idle eviction failed: %v", err)
	}
	if err := m.View("b", func(*journey.Session) error { return nil }); err != nil {
		t.Fatal("active session evicted")
	}
	if err := m.View("a", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatal("idle session not evicted")
	}
}

func TestRecipientNeedsLiveOwnerAndUnexpired(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{Now: c.now})
	if err := m.CreateRecipient(journey.RecipientSession{ID: "r", SessionID: "missing", ExpiresAt: c.now().Add(time.Hour)}); err != journey.ErrNotFound {
		t.Fatal("recipient created for a missing owner")
	}
	_ = m.Create(newSession("a", c.now()))
	_ = m.CreateRecipient(journey.RecipientSession{ID: "r", SessionID: "a", HandoffID: "h", ExpiresAt: c.now().Add(time.Minute)})
	if _, ok := m.Recipient("r", c.now()); !ok {
		t.Fatal("recipient missing")
	}
	if _, ok := m.Recipient("r", c.now().Add(2*time.Minute)); ok {
		t.Fatal("expired recipient accepted")
	}
	m.DeleteRecipientsFor("a", "h")
	if _, ok := m.Recipient("r", c.now()); ok {
		t.Fatal("recipient survived DeleteRecipientsFor")
	}
}

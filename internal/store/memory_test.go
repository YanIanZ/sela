package store

import (
	"strings"
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

func TestCapacitySparesRetainedOrSharedSessions(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{MaxSessions: 2, Now: c.now})
	day := newSession("day", c.now())
	day.Retention, day.ExpiresAt = journey.RetentionDay, c.now().Add(journey.DayMaxLife)
	_ = m.Create(day)
	shared := newSession("shared", c.now())
	shared.Handoffs = []*journey.Handoff{{ID: "h1", Share: journey.ShareActive, ExpiresAt: c.now().Add(time.Hour)}}
	_ = m.Create(shared)
	c.add(11 * time.Minute)
	if err := m.Create(newSession("c", c.now())); err != journey.ErrCapacity {
		t.Fatalf("protected session evicted: %v", err)
	}
	// Once the share ended, the idle session-only session may be evicted.
	_ = m.Update("shared", func(s *journey.Session) error { s.Handoffs[0].Share = journey.ShareRevoked; return nil })
	if err := m.Create(newSession("c", c.now())); err != nil {
		t.Fatalf("idle eviction failed: %v", err)
	}
	if err := m.View("day", func(*journey.Session) error { return nil }); err != nil {
		t.Fatal("storage-consented session evicted")
	}
}

// Protected sessions cannot hold the store for longer than the ordinary idle
// timeout, and an ordinary idle session always goes before a protected one.
func TestCapacityEvictsProtectedSessionsLast(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{MaxSessions: 3, Now: c.now})
	protect := func(id string) *journey.Session {
		s := newSession(id, c.now())
		s.Retention, s.ExpiresAt = journey.RetentionDay, c.now().Add(journey.DayMaxLife)
		return s
	}
	_ = m.Create(protect("day1"))
	c.add(time.Minute)
	_ = m.Create(protect("day2"))
	shared := newSession("shared", c.now())
	shared.Handoffs = []*journey.Handoff{{ID: "h1", Share: journey.ShareActive, ExpiresAt: c.now().Add(time.Hour)}}
	_ = m.Create(shared)
	c.add(journey.SessionIdle - 2*time.Minute)
	if err := m.Create(newSession("early", c.now())); err != journey.ErrCapacity {
		t.Fatalf("protected session evicted before the ordinary idle timeout: %v", err)
	}
	c.add(time.Minute)
	if err := m.Create(newSession("v1", c.now())); err != nil {
		t.Fatalf("idle protected sessions lock the store: %v", err)
	}
	if err := m.View("day1", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatal("the least recently seen protected session was not the one evicted")
	}
	for _, id := range []string{"day2", "shared"} {
		if err := m.View(id, func(*journey.Session) error { return nil }); err != nil {
			t.Fatalf("%s evicted too", id)
		}
	}
	// v1 is ordinary: once idle it goes before the older protected sessions.
	c.add(11 * time.Minute)
	if err := m.Create(newSession("v2", c.now())); err != nil {
		t.Fatal(err)
	}
	if err := m.View("v1", func(*journey.Session) error { return nil }); err != journey.ErrNotFound {
		t.Fatal("a protected session was evicted while an ordinary idle one could go")
	}
	for _, id := range []string{"day2", "shared", "v2"} {
		if err := m.View(id, func(*journey.Session) error { return nil }); err != nil {
			t.Fatalf("%s evicted", id)
		}
	}
}

func TestRecipientsPerHandoffReplaceTheOldest(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{MaxSessions: 1, Now: c.now})
	_ = m.Create(newSession("a", c.now()))
	exp := c.now().Add(time.Hour)
	for _, id := range []string{"r1", "r2", "r3", "r4"} {
		if err := m.CreateRecipient(journey.RecipientSession{ID: id, SessionID: "a", HandoffID: "h1", CreatedAt: c.now(), ExpiresAt: exp}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := m.Recipient("r1", c.now()); ok {
		t.Fatal("oldest principal not replaced")
	}
	for _, id := range []string{"r2", "r3", "r4"} {
		if _, ok := m.Recipient(id, c.now()); !ok {
			t.Fatalf("%s replaced", id)
		}
	}
	// Another handoff is unaffected, and the default global bound fits every
	// principal the per-handoff and per-session bounds allow.
	if err := m.CreateRecipient(journey.RecipientSession{ID: "x1", SessionID: "a", HandoffID: "h2", ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	if got := m.Stats().Recipients; got != journey.MaxRecipientsPerHandoff+1 {
		t.Fatalf("recipients = %d", got)
	}
	if m.maxRecips < journey.MaxHandoffs*journey.MaxRecipientsPerHandoff {
		t.Fatalf("default recipient bound %d too small", m.maxRecips)
	}
}

func TestOnPurgeReportsEveryPurge(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{MaxSessions: 3, Now: c.now})
	var mu sync.Mutex
	var purged []string
	m.OnPurge(func(id string) { mu.Lock(); purged = append(purged, id); mu.Unlock() })
	got := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string{}, purged...) }
	for _, id := range []string{"deleted", "swept", "accessed"} {
		_ = m.Create(newSession(id, c.now()))
	}
	// Delete inside the session's own Update reports the purge without deadlock.
	_ = m.Update("deleted", func(*journey.Session) error { m.Delete("deleted"); return nil })
	c.add(journey.SessionIdle + time.Second)
	_ = m.View("accessed", func(*journey.Session) error { return nil }) // expiry on access
	m.Sweep(c.now())
	_ = m.Create(newSession("idle1", c.now()))
	_ = m.Create(newSession("idle2", c.now()))
	_ = m.Create(newSession("idle3", c.now()))
	c.add(11 * time.Minute)
	_ = m.Create(newSession("new", c.now())) // evicts one idle session
	want := map[string]bool{"deleted": true, "swept": true, "accessed": true}
	evicted := 0
	for _, id := range got() {
		if strings.HasPrefix(id, "idle") {
			evicted++
		}
		delete(want, id)
	}
	if len(want) != 0 || evicted != 1 {
		t.Fatalf("purges reported %v (missing %v)", got(), want)
	}
	m.Delete("missing") // nothing to report
	if n := len(got()); n != 4 {
		t.Fatalf("reports = %d, want 4", n)
	}
}

func TestLiveSessionsArePerClientBounded(t *testing.T) {
	c := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	m := NewMemory(Options{MaxSessions: 100, MaxSessionsPerClient: 3, Now: c.now})
	mk := func(id, key string) error {
		s := newSession(id, c.now())
		s.ClientKey = key
		return m.Create(s)
	}
	for i := 0; i < 3; i++ {
		if err := mk("a"+string(rune('0'+i)), "198.51.100.7"); err != nil {
			t.Fatal(err)
		}
	}
	if err := mk("a3", "198.51.100.7"); err != journey.ErrClientLimit {
		t.Fatalf("fourth session from one client: %v", err)
	}
	// Another client and key-less sessions are unaffected.
	if err := mk("b0", "203.0.113.9"); err != nil {
		t.Fatal(err)
	}
	if err := mk("c0", ""); err != nil {
		t.Fatal(err)
	}
	// Deleted and expired sessions no longer count.
	m.Delete("a0")
	if err := mk("a4", "198.51.100.7"); err != nil {
		t.Fatalf("after delete: %v", err)
	}
	c.add(journey.SessionMaxLife + time.Minute)
	for i := 5; i < 8; i++ {
		if err := mk("a"+string(rune('0'+i)), "198.51.100.7"); err != nil {
			t.Fatalf("after expiry: %v", err)
		}
	}
}

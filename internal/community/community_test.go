package community

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/db/dbtest"
)

var bg = context.Background()

func newStore(t *testing.T) (*Store, func(n byte) [16]byte) {
	t.Helper()
	db := dbtest.New(t)
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	s, err := New(db.Pool, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	user := func(n byte) [16]byte {
		var u [16]byte
		u[0], u[6], u[8] = n, 0x40, 0x80
		if _, err := db.Pool.Exec(bg, `INSERT INTO users (id, kind, age_band, age_band_source, age_band_confirmed_at, locale, created_at, last_active_at, guest_expires_at)
			VALUES ($1, 'guest', '18_plus', 'self_declared', $2, 'id', $2, $2, $3) ON CONFLICT DO NOTHING`, u, now, now.Add(720*time.Hour)); err != nil {
			t.Fatal(err)
		}
		return u
	}
	return s, user
}

func TestFeedShowsOnlyPublishedToOthers(t *testing.T) {
	s, user := newStore(t)
	a, b := user(1), user(2)
	pub, err := s.CreatePost(bg, a, Draft{Body: "Hari ini berat tapi aku bertahan.", Topic: "daily", Status: StatusPublished})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePost(bg, a, Draft{Body: "dasar cengeng", Topic: "daily", Status: StatusHidden, Labels: []string{"bullying"}}); err != nil {
		t.Fatal(err)
	}
	feedB, _ := s.Feed(bg, b, "", 30)
	if len(feedB) != 1 || feedB[0].ID != pub.ID || feedB[0].Mine || feedB[0].Labels != nil {
		t.Fatalf("other's feed = %+v", feedB)
	}
	feedA, _ := s.Feed(bg, a, "", 30)
	if len(feedA) != 2 {
		t.Fatalf("author's feed = %d items, want own hidden too", len(feedA))
	}
	if _, err := s.CreatePost(bg, a, Draft{Body: "x", Topic: "nope", Status: StatusPublished}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad topic = %v", err)
	}
}

func TestRepliesAliasReportsAndQueue(t *testing.T) {
	s, user := newStore(t)
	a, b, c, d, e := user(1), user(2), user(3), user(4), user(5)
	post, _ := s.CreatePost(bg, a, Draft{Body: "Aku kesepian di kota baru.", Topic: "loneliness", Status: StatusPublished})
	r1, err := s.CreateReply(bg, b, post.ID, Draft{Body: "Aku juga pernah, pelan-pelan ya.", Status: StatusPublished})
	if err != nil {
		t.Fatal(err)
	}
	r2, _ := s.CreateReply(bg, b, post.ID, Draft{Body: "Semangat.", Status: StatusPublished})
	if r1.Alias != r2.Alias {
		t.Fatal("one person should keep one alias in a thread")
	}
	own, _ := s.CreateReply(bg, a, post.ID, Draft{Body: "Makasih.", Status: StatusPublished})
	if own.Alias != post.Alias {
		t.Fatal("the author should reply under the post's alias")
	}
	if err := s.Report(bg, b, "reply", r1.ID, "spam"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("self report = %v", err)
	}
	for _, rep := range [][16]byte{c, d, d, e} {
		if err := s.Report(bg, rep, "reply", r1.ID, "bullying"); err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	_, replies, _ := s.Thread(bg, c, post.ID)
	for _, r := range replies {
		if r.ID == r1.ID {
			t.Fatal("reply hidden after three reports is still visible")
		}
	}
	q, _ := s.Queue(bg, 50)
	if len(q) != 1 || q[0].ID != r1.ID || q[0].Status != StatusHidden || !slices.Contains(q[0].Labels, "reported") || q[0].Reports != 3 {
		t.Fatalf("queue = %+v", q)
	}
	if old, next, err := s.Decide(bg, "reply", r1.ID, "restore"); err != nil || old != StatusHidden || next != StatusPublished {
		t.Fatalf("restore = %s %s %v", old, next, err)
	}
	if _, _, err := s.Decide(bg, "reply", r1.ID, "remove"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Decide(bg, "reply", r1.ID, "restore"); !errors.Is(err, ErrForbidden) {
		t.Fatal("a removed item must stay removed")
	}
	if err := s.DeleteOwn(bg, c, "post", post.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleting someone else's post must fail")
	}
	if err := s.DeleteOwn(bg, a, "post", post.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Thread(bg, b, post.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted post still readable")
	}
}

func TestPostRateLimit(t *testing.T) {
	s, user := newStore(t)
	a := user(1)
	for i := 0; i < MaxPostsPerHour; i++ {
		if _, err := s.CreatePost(bg, a, Draft{Body: "cerita", Topic: "other", Status: StatusPublished}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreatePost(bg, a, Draft{Body: "cerita", Topic: "other", Status: StatusPublished}); !errors.Is(err, ErrRateLimit) {
		t.Fatalf("over limit = %v", err)
	}
}

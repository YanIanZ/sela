// Package community is Sela's anonymous peer-sharing space (D-072): adults
// share a story under a random alias and others reply with support. Nothing
// is shown before it passes the safety router and moderation; reports hide
// content automatically and operators decide in a moderation queue. Author
// ids never leave this package except as "mine" flags for the author.
package community

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status is the visibility of a post or reply.
type Status string

const (
	StatusPublished Status = "published"
	StatusHeld      Status = "held"
	StatusHidden    Status = "hidden"
	StatusRemoved   Status = "removed"
)

// Topics are the fixed topics a post can carry.
var Topics = []string{"daily", "work", "study", "family", "relationship", "loneliness", "other"}

// ReportReasons are the fixed reasons for a report.
var ReportReasons = []string{"bullying", "harassment", "hate", "sexual", "self_harm", "personal_info", "spam", "other"}

// ReportThreshold hides a post or reply once this many people report it.
const ReportThreshold = 3

// Limits per author per hour.
const (
	MaxPostsPerHour   = 5
	MaxRepliesPerHour = 30
	MaxPostRunes      = 2000
	MaxReplyRunes     = 1000
)

var (
	ErrNotFound  = errors.New("community: not found")
	ErrInvalid   = errors.New("community: invalid input")
	ErrRateLimit = errors.New("community: rate limited")
	ErrForbidden = errors.New("community: not allowed")
)

// Post is a story as a reader sees it.
type Post struct {
	ID        string    `json:"id"`
	Alias     string    `json:"alias"`
	Topic     string    `json:"topic"`
	Body      string    `json:"body"`
	Status    Status    `json:"status"`
	Labels    []string  `json:"labels,omitempty"`
	Replies   int       `json:"replies"`
	Mine      bool      `json:"mine"`
	CreatedAt time.Time `json:"createdAt"`
}

// Reply is a reply as a reader sees it.
type Reply struct {
	ID        string    `json:"id"`
	Alias     string    `json:"alias"`
	Body      string    `json:"body"`
	Status    Status    `json:"status"`
	Labels    []string  `json:"labels,omitempty"`
	Mine      bool      `json:"mine"`
	CreatedAt time.Time `json:"createdAt"`
}

// QueueItem is one post or reply waiting for an operator. It carries no
// author information.
type QueueItem struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	PostID    string    `json:"postId,omitempty"`
	Alias     string    `json:"alias"`
	Body      string    `json:"body"`
	Status    Status    `json:"status"`
	Labels    []string  `json:"labels"`
	Reports   int       `json:"reports"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store owns the community tables.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// New opens the community store.
func New(pool *pgxpool.Pool, now func() time.Time) (*Store, error) {
	if pool == nil {
		return nil, errors.New("community: nil pool")
	}
	if now == nil {
		now = time.Now
	}
	return &Store{pool: pool, now: now}, nil
}

// Draft is a moderated post or reply ready to store.
type Draft struct {
	Body   string
	Topic  string
	Status Status
	Labels []string
}

// CreatePost stores a post for author under a fresh alias.
func (s *Store) CreatePost(ctx context.Context, author [16]byte, d Draft) (Post, error) {
	if !slices.Contains(Topics, d.Topic) {
		return Post{}, ErrInvalid
	}
	now := s.now().UTC()
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM community_posts WHERE author_id = $1 AND created_at > $2`, author, now.Add(-time.Hour)).Scan(&n); err != nil {
		return Post{}, err
	}
	if n >= MaxPostsPerHour {
		return Post{}, ErrRateLimit
	}
	p := Post{ID: newID(), Alias: NewAlias(), Topic: d.Topic, Body: d.Body, Status: d.Status, Labels: labelsOrEmpty(d.Labels), Mine: true, CreatedAt: now}
	_, err := s.pool.Exec(ctx, `INSERT INTO community_posts (id, author_id, alias, topic, body, status, labels, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`, p.ID, author, p.Alias, p.Topic, p.Body, string(p.Status), p.Labels, now)
	return p, err
}

// CreateReply stores a reply to a published post. A person keeps one alias
// per thread, so a conversation stays readable without a profile.
func (s *Store) CreateReply(ctx context.Context, author [16]byte, postID string, d Draft) (Reply, error) {
	now := s.now().UTC()
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT status FROM community_posts WHERE id = $1`, postID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Reply{}, ErrNotFound
		}
		return Reply{}, err
	}
	if Status(status) != StatusPublished {
		return Reply{}, ErrNotFound
	}
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM community_replies WHERE author_id = $1 AND created_at > $2`, author, now.Add(-time.Hour)).Scan(&n); err != nil {
		return Reply{}, err
	}
	if n >= MaxRepliesPerHour {
		return Reply{}, ErrRateLimit
	}
	alias := ""
	err := s.pool.QueryRow(ctx, `SELECT alias FROM (
			SELECT alias, created_at FROM community_posts WHERE id = $1 AND author_id = $2
			UNION ALL SELECT alias, created_at FROM community_replies WHERE post_id = $1 AND author_id = $2
		) a ORDER BY created_at LIMIT 1`, postID, author).Scan(&alias)
	if errors.Is(err, pgx.ErrNoRows) {
		alias = NewAlias()
	} else if err != nil {
		return Reply{}, err
	}
	rp := Reply{ID: newID(), Alias: alias, Body: d.Body, Status: d.Status, Labels: labelsOrEmpty(d.Labels), Mine: true, CreatedAt: now}
	_, err = s.pool.Exec(ctx, `INSERT INTO community_replies (id, post_id, author_id, alias, body, status, labels, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`, rp.ID, postID, author, rp.Alias, rp.Body, string(rp.Status), rp.Labels, now)
	return rp, err
}

// Feed lists published posts, newest first, plus the viewer's own posts
// that are not shown to others (so they know why).
func (s *Store) Feed(ctx context.Context, viewer [16]byte, topic string, limit int) ([]Post, error) {
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	if topic != "" && !slices.Contains(Topics, topic) {
		return nil, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id, p.alias, p.topic, p.body, p.status, p.labels, p.author_id = $1, p.created_at,
			(SELECT count(*) FROM community_replies r WHERE r.post_id = p.id AND r.status = 'published')
		FROM community_posts p
		WHERE (p.status = 'published' OR (p.author_id = $1 AND p.status <> 'removed')) AND ($2 = '' OR p.topic = $2)
		ORDER BY p.created_at DESC LIMIT $3`, viewer, topic, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		var p Post
		var st string
		if err := rows.Scan(&p.ID, &p.Alias, &p.Topic, &p.Body, &st, &p.Labels, &p.Mine, &p.CreatedAt, &p.Replies); err != nil {
			return nil, err
		}
		p.Status = Status(st)
		if !p.Mine {
			p.Labels = nil
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Thread returns a published post (or the viewer's own) with its visible
// replies.
func (s *Store) Thread(ctx context.Context, viewer [16]byte, id string) (Post, []Reply, error) {
	var p Post
	var st string
	err := s.pool.QueryRow(ctx, `SELECT id, alias, topic, body, status, labels, author_id = $2, created_at FROM community_posts
		WHERE id = $1 AND (status = 'published' OR (author_id = $2 AND status <> 'removed'))`, id, viewer).
		Scan(&p.ID, &p.Alias, &p.Topic, &p.Body, &st, &p.Labels, &p.Mine, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, nil, ErrNotFound
	}
	if err != nil {
		return Post{}, nil, err
	}
	p.Status = Status(st)
	if !p.Mine {
		p.Labels = nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id, alias, body, status, labels, author_id = $2, created_at FROM community_replies
		WHERE post_id = $1 AND (status = 'published' OR (author_id = $2 AND status <> 'removed')) ORDER BY created_at`, id, viewer)
	if err != nil {
		return Post{}, nil, err
	}
	defer rows.Close()
	replies := []Reply{}
	for rows.Next() {
		var r Reply
		if err := rows.Scan(&r.ID, &r.Alias, &r.Body, &st, &r.Labels, &r.Mine, &r.CreatedAt); err != nil {
			return Post{}, nil, err
		}
		r.Status = Status(st)
		if !r.Mine {
			r.Labels = nil
		}
		replies = append(replies, r)
	}
	p.Replies = len(replies)
	return p, replies, rows.Err()
}

// Report records one person's report; at ReportThreshold distinct reports a
// published item is hidden and queued. Reporting your own item is refused,
// and reporting twice counts once.
func (s *Store) Report(ctx context.Context, reporter [16]byte, kind, id, reason string) error {
	table, ok := tableFor(kind)
	if !ok || !slices.Contains(ReportReasons, reason) {
		return ErrInvalid
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var author [16]byte
		var st string
		err := tx.QueryRow(ctx, `SELECT author_id, status FROM `+table+` WHERE id = $1 FOR UPDATE`, id).Scan(&author, &st)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && Status(st) != StatusPublished) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if author == reporter {
			return ErrForbidden
		}
		tag, err := tx.Exec(ctx, `INSERT INTO community_reports (target_kind, target_id, reporter_id, reason, created_at)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`, kind, id, reporter, reason, s.now().UTC())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE `+table+` SET reports = reports + 1,
			status = CASE WHEN reports + 1 >= $2 THEN 'hidden' ELSE status END,
			labels = CASE WHEN reports + 1 >= $2 AND NOT ('reported' = ANY(labels)) THEN array_append(labels, 'reported') ELSE labels END,
			updated_at = $3 WHERE id = $1`, id, ReportThreshold, s.now().UTC())
		return err
	})
}

// DeleteOwn deletes the author's own post (with its replies) or reply.
func (s *Store) DeleteOwn(ctx context.Context, author [16]byte, kind, id string) error {
	table, ok := tableFor(kind)
	if !ok {
		return ErrInvalid
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM `+table+` WHERE id = $1 AND author_id = $2`, id, author)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Queue lists held, hidden and reported items for operators, oldest change
// first, without any author information.
func (s *Store) Queue(ctx context.Context, limit int) ([]QueueItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT 'post', id, '', alias, body, status, labels, reports, updated_at FROM community_posts
			WHERE status IN ('held', 'hidden') OR (status = 'published' AND reports > 0)
		UNION ALL
		SELECT 'reply', id, post_id::text, alias, body, status, labels, reports, updated_at FROM community_replies
			WHERE status IN ('held', 'hidden') OR (status = 'published' AND reports > 0)
		ORDER BY 9 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QueueItem{}
	for rows.Next() {
		var q QueueItem
		var st string
		if err := rows.Scan(&q.Kind, &q.ID, &q.PostID, &q.Alias, &q.Body, &st, &q.Labels, &q.Reports, &q.UpdatedAt); err != nil {
			return nil, err
		}
		q.Status = Status(st)
		out = append(out, q)
	}
	return out, rows.Err()
}

// Decide applies an operator decision: "restore" publishes the item and
// clears its reports, "remove" removes it for good. It returns the status
// before the decision for the audit.
func (s *Store) Decide(ctx context.Context, kind, id, action string) (Status, Status, error) {
	table, ok := tableFor(kind)
	if !ok {
		return "", "", ErrInvalid
	}
	var next Status
	switch action {
	case "restore":
		next = StatusPublished
	case "remove":
		next = StatusRemoved
	default:
		return "", "", ErrInvalid
	}
	var old string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT status FROM `+table+` WHERE id = $1 FOR UPDATE`, id).Scan(&old); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if Status(old) == StatusRemoved {
			return ErrForbidden
		}
		if next == StatusPublished {
			if _, err := tx.Exec(ctx, `DELETE FROM community_reports WHERE target_kind = $1 AND target_id = $2`, kind, id); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE `+table+` SET status = 'published', reports = 0, labels = '{}', updated_at = $2 WHERE id = $1`, id, s.now().UTC())
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE `+table+` SET status = 'removed', updated_at = $2 WHERE id = $1`, id, s.now().UTC())
		return err
	})
	return Status(old), next, err
}

func tableFor(kind string) (string, bool) {
	switch kind {
	case "post":
		return "community_posts", true
	case "reply":
		return "community_replies", true
	}
	return "", false
}

func labelsOrEmpty(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

func newID() string {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		panic(err)
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

var aliasNature = []string{"Daun", "Embun", "Senja", "Ombak", "Angin", "Bintang", "Hujan", "Fajar", "Pelangi", "Awan", "Bulan", "Pohon", "Rumput", "Kabut", "Batu", "Sungai"}
var aliasMood = []string{"Tenang", "Hangat", "Teduh", "Lembut", "Sabar", "Pelan", "Terang", "Sejuk", "Riang", "Kuat", "Damai", "Jernih"}

// NewAlias returns a random, friendly alias such as "Embun Teduh 27".
func NewAlias() string {
	pick := func(list []string) string {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
		return list[n.Int64()]
	}
	num, _ := rand.Int(rand.Reader, big.NewInt(90))
	return strings.Join([]string{pick(aliasNature), pick(aliasMood), fmt.Sprint(num.Int64() + 10)}, " ")
}

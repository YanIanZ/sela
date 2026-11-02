package account

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Life Context Timeline (BW-002). A read-only, owner-scoped projection of
// the saved source records (note, check-in, saved conversation; a weekly
// reflection is a note) in chronological order, with each source's current
// version and its meaning status. It is computed on read from
// personal_records and the MIE tables: no second copy of history, no new
// table, no model call and no write of any kind. It never says what a
// person feels, how they are doing or why something happened.

// Timeline meaning statuses.
const (
	// TimelineCurrent: nothing that rests on this source needs the owner.
	TimelineCurrent = "current"
	// TimelineStale: the owner revised this source after an interpretation
	// (a memory claim or a summary) was built on an earlier version, and
	// that interpretation is still marked stale (D-078). It stays out of
	// new approved shares until the owner reviews it again.
	TimelineStale = "stale"
	// TimelineNeedsReview: a tentative Sela reflection inside this saved
	// conversation has not been reviewed by the owner yet.
	TimelineNeedsReview = "needs_review"
)

// Timeline bounds.
const (
	TimelineDefaultLimit = 20
	TimelineMaxLimit     = 50
	// TimelineExcerptRunes is the length of the excerpt; the full source
	// stays behind GET /api/v3/records/{id}.
	TimelineExcerptRunes = 280
)

// TimelineKinds are the source kinds the timeline projects.
var TimelineKinds = []string{KindNote, KindCheckin, KindConversation}

// TimelineQuery is one page request. Days (7, 30 or 90) is a window that
// ends now on the server clock, so a skewed client clock cannot move it;
// otherwise From is inclusive and To exclusive, a zero bound open. Order is "desc" (newest first, the default) or
// "asc". Cursor is the opaque nextCursor of the previous page.
type TimelineQuery struct {
	Kinds  []string
	Days   int
	From   time.Time
	To     time.Time
	Limit  int
	Order  string
	Cursor string
}

// TimelineSource identifies the exact source version an item shows.
type TimelineSource struct {
	RecordID string `json:"recordId"`
	Version  int    `json:"version"`
	Kind     string `json:"kind"`
}

// TimelineStaleInfo says why an item is stale: the MIE reason of the
// newest stale mark on an interpretation built on an older version.
type TimelineStaleInfo struct {
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// TimelineItem is one card. Excerpt is a verbatim prefix of the owner's
// own words in that version (for a conversation, the owner's turns only;
// AI replies are never part of it).
type TimelineItem struct {
	Source             TimelineSource     `json:"source"`
	OccurredAt         time.Time          `json:"occurredAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
	Day                string             `json:"day,omitempty"`
	Title              string             `json:"title"`
	Excerpt            string             `json:"excerpt"`
	Truncated          bool               `json:"truncated"`
	Mood               string             `json:"mood,omitempty"`
	Energy             string             `json:"energy,omitempty"`
	Provenance         string             `json:"provenance"`
	MeaningStatus      string             `json:"meaningStatus"`
	Stale              *TimelineStaleInfo `json:"stale,omitempty"`
	PendingReflections int                `json:"pendingReflections,omitempty"`
}

// TimelinePage is one page of the projection. AIUsed is always false: the
// projection never calls a provider.
type TimelinePage struct {
	Items      []TimelineItem `json:"items"`
	NextCursor *string        `json:"nextCursor"`
	Count      int            `json:"count"`
	AIUsed     bool           `json:"aiUsed"`
}

type timelineCursor struct {
	order string
	at    time.Time
	id    pgtype.UUID
}

// The cursor carries the sort key of the last item: the order, its
// created_at in microseconds (PostgreSQL's precision) and its record id.
// No text and no owner id; a cursor naming another owner's record id only
// positions the keyset and reads nothing of that record.
func (c timelineCursor) encode() string {
	raw := fmt.Sprintf("v1|%s|%d|%s", c.order, c.at.UnixMicro(), c.id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeTimelineCursor(s, order string) (timelineCursor, error) {
	var c timelineCursor
	if len(s) > 128 {
		return c, invalid("cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, invalid("cursor")
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != order {
		return c, invalid("cursor")
	}
	micros, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return c, invalid("cursor")
	}
	id, err := ParseUUID(parts[3])
	if err != nil {
		return c, invalid("cursor")
	}
	c.order, c.at = order, time.UnixMicro(micros).UTC()
	c.id = pgtype.UUID{Bytes: id, Valid: true}
	return c, nil
}

// NormalizeTimelineQuery validates q and fills its defaults.
func NormalizeTimelineQuery(q TimelineQuery) (TimelineQuery, error) {
	if q.Limit == 0 {
		q.Limit = TimelineDefaultLimit
	}
	if q.Limit < 1 || q.Limit > TimelineMaxLimit {
		return q, invalid("limit")
	}
	if q.Order == "" {
		q.Order = "desc"
	}
	if q.Order != "desc" && q.Order != "asc" {
		return q, invalid("order")
	}
	if len(q.Kinds) == 0 {
		q.Kinds = append([]string(nil), TimelineKinds...)
	}
	seen := map[string]bool{}
	kinds := make([]string, 0, len(q.Kinds))
	for _, k := range q.Kinds {
		if k != KindNote && k != KindCheckin && k != KindConversation {
			return q, invalid("kinds")
		}
		if !seen[k] {
			seen[k] = true
			kinds = append(kinds, k)
		}
	}
	q.Kinds = kinds
	switch q.Days {
	case 0, 7, 30, 90:
	default:
		return q, invalid("days")
	}
	if q.Days != 0 && (!q.From.IsZero() || !q.To.IsZero()) {
		return q, invalid("days")
	}
	if !q.From.IsZero() && !q.To.IsZero() && !q.From.Before(q.To) {
		return q, invalid("to")
	}
	return q, nil
}

// ContextTimeline returns one page of the owner's Life Context Timeline.
func (s *Store) ContextTimeline(ctx context.Context, owner UserID, q TimelineQuery) (TimelinePage, error) {
	q, err := NormalizeTimelineQuery(q)
	if err != nil {
		return TimelinePage{}, err
	}
	if q.Days != 0 {
		q.From = s.clock().UTC().Add(-time.Duration(q.Days) * 24 * time.Hour)
	}
	var cur *timelineCursor
	if q.Cursor != "" {
		c, err := decodeTimelineCursor(q.Cursor, q.Order)
		if err != nil {
			return TimelinePage{}, err
		}
		cur = &c
	}
	page := TimelinePage{Items: []TimelineItem{}}
	// One read-only snapshot, so the page and its meaning statuses agree.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return page, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	args := []any{owner, q.Kinds}
	where := []string{"user_id=$1", "kind = ANY($2::text[])"}
	if !q.From.IsZero() {
		args = append(args, q.From)
		where = append(where, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if !q.To.IsZero() {
		args = append(args, q.To)
		where = append(where, fmt.Sprintf("created_at < $%d", len(args)))
	}
	dir, cmp := "DESC", "<"
	if q.Order == "asc" {
		dir, cmp = "ASC", ">"
	}
	if cur != nil {
		args = append(args, cur.at, cur.id)
		where = append(where, fmt.Sprintf("(created_at, id) %s ($%d, $%d)", cmp, len(args)-1, len(args)))
	}
	args = append(args, q.Limit+1)
	sql := fmt.Sprintf(`SELECT id, version, created_at, updated_at, content_ct FROM personal_records
		WHERE %s ORDER BY created_at %s, id %s LIMIT $%d`, strings.Join(where, " AND "), dir, dir, len(args))
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return page, err
	}
	type sealed struct {
		id      pgtype.UUID
		v       int
		created time.Time
		updated time.Time
		ct      []byte
	}
	var data []sealed
	for rows.Next() {
		var x sealed
		if err := rows.Scan(&x.id, &x.v, &x.created, &x.updated, &x.ct); err != nil {
			rows.Close()
			return page, err
		}
		data = append(data, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return page, err
	}
	more := len(data) > q.Limit
	if more {
		data = data[:q.Limit]
	}
	ids := make([]pgtype.UUID, 0, len(data))
	for _, x := range data {
		// Only the rows of this page are decrypted.
		r, err := s.openRecord(ctx, tx, owner, x.id.String(), x.v, x.ct)
		if err != nil {
			return page, err
		}
		excerpt, truncated := timelineExcerpt(sourcePlainText(r))
		item := TimelineItem{
			Source:        TimelineSource{RecordID: r.ID, Version: x.v, Kind: r.Kind},
			OccurredAt:    x.created.UTC(),
			UpdatedAt:     x.updated.UTC(),
			Title:         r.Title,
			Excerpt:       excerpt,
			Truncated:     truncated,
			Provenance:    r.Provenance,
			MeaningStatus: TimelineCurrent,
		}
		if r.Kind == KindCheckin {
			item.Day, item.Mood, item.Energy = r.Day, r.Mood, r.Energy
		}
		for _, ref := range r.Reflections {
			if ref.ReviewStatus == ReflectionUnreviewed {
				item.PendingReflections++
			}
		}
		if item.PendingReflections > 0 {
			item.MeaningStatus = TimelineNeedsReview
		}
		page.Items = append(page.Items, item)
		ids = append(ids, x.id)
	}
	if len(ids) > 0 {
		stale, err := s.timelineStale(ctx, tx, owner, ids)
		if err != nil {
			return page, err
		}
		for i := range page.Items {
			if st, ok := stale[page.Items[i].Source.RecordID]; ok {
				st := st
				page.Items[i].Stale = &st
				page.Items[i].MeaningStatus = TimelineStale
			}
		}
	}
	if more {
		last := data[len(data)-1]
		next := timelineCursor{order: q.Order, at: last.created, id: last.id}.encode()
		page.NextCursor = &next
	}
	page.Count = len(page.Items)
	return page, tx.Commit(ctx)
}

// timelineStale finds, for the given source ids, the newest stale mark on
// a current memory claim or summary built on an older version of that
// source. Memory claims count only while personal memory is enabled, as
// everywhere else they are read. Ids, versions and reasons only.
func (s *Store) timelineStale(ctx context.Context, tx pgx.Tx, owner UserID, ids []pgtype.UUID) (map[string]TimelineStaleInfo, error) {
	memory, err := s.memoryEnabled(ctx, tx, owner)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON (e.parent_id) e.parent_id::text, st.reason, st.at
		FROM mie_edges e
		JOIN personal_records src ON src.user_id = e.user_id AND src.id = e.parent_id AND src.version > e.parent_version
		JOIN personal_records d ON d.user_id = e.user_id AND d.id = e.child_id AND d.version = e.child_version
		JOIN mie_stale st ON st.user_id = e.user_id AND st.record_id = e.child_id AND st.version = e.child_version
		WHERE e.user_id = $1 AND e.parent_kind = 'record' AND e.parent_id = ANY($2::uuid[])
		  AND (d.kind <> 'memory' OR $3)
		ORDER BY e.parent_id, st.at DESC`, owner, ids, memory)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]TimelineStaleInfo{}
	for rows.Next() {
		var id string
		var st TimelineStaleInfo
		if err := rows.Scan(&id, &st.Reason, &st.At); err != nil {
			return nil, err
		}
		st.At = st.At.UTC()
		out[id] = st
	}
	return out, rows.Err()
}

// timelineExcerpt is a verbatim rune prefix of text.
func timelineExcerpt(text string) (string, bool) {
	if utf8.RuneCountInString(text) <= TimelineExcerptRunes {
		return text, false
	}
	n := 0
	for i := range text {
		if n == TimelineExcerptRunes {
			return text[:i], true
		}
		n++
	}
	return text, false
}

package account

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/db"
)

// Meaning Integrity Engine 1.0 (D-078). Every derivation is recorded as a
// plain-id edge: a source record version feeds a memory claim or a summary,
// a confirmed memory claim feeds a summary statement, an approved summary
// version feeds a share, and a share feeds a care request. When the user
// corrects or rejects a claim, or edits or deletes a source, invalidate walks
// the graph in the same transaction: every current memory and summary
// descendant gets a stale mark, every active share built on it is revoked
// (end_reason meaning_invalidated) and every open care request on such a
// share is cancelled. Recipients, partners and exports therefore never read
// it as current afterwards. Old versions stay readable to the owner as
// history; a new version is clean until an invalidation reaches it.

// MIE node kinds (mie_edges.parent_kind / child_kind).
const (
	MIERecord  = "record"
	MIEMemory  = "memory"
	MIESummary = "summary"
	MIEShare   = "share"
	MIECare    = "care_request"
)

// Source record kinds a claim or statement may rest on.
const (
	KindNote         = "note"
	KindCheckin      = "checkin"
	KindConversation = "conversation"
)

// Invalidation reasons (mie_events.reason).
const (
	MIESourceEdited   = "source_edited"
	MIESourceDeleted  = "source_deleted"
	MIEClaimCorrected = "claim_corrected"
	MIEClaimRejected  = "claim_rejected"
	MIEClaimDeleted   = "claim_deleted"
)

// mieMaxNodes and mieMaxDepth bound one walk. A larger graph fails closed:
// the whole transaction (the edit or review that triggered it) is refused
// rather than leaving some descendants current.
const (
	mieMaxNodes = 500
	mieMaxDepth = 6
)

// ErrMIELimit refuses a change whose descendants exceed the walk bounds.
var ErrMIELimit = &Error{Code: "meaning_graph_limit"}

// StaleInfo marks a record version an invalidation reached.
type StaleInfo struct {
	Reason  string    `json:"reason"`
	EventID string    `json:"eventId"`
	At      time.Time `json:"at"`
}

// MIENode is one owner-visible node of the lineage graph: ids and versions
// only, never text.
type MIENode struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// MIEImpact counts what an invalidation reached (or would reach, for a
// preview).
type MIEImpact struct {
	EventID      string    `json:"eventId,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	Origin       MIENode   `json:"origin"`
	Memories     int       `json:"memories"`
	Summaries    int       `json:"summaries"`
	Shares       int       `json:"shares"`
	CareRequests int       `json:"careRequests"`
	Nodes        []MIENode `json:"nodes"`
	At           time.Time `json:"at,omitempty"`
}

// Total is the number of descendants reached.
func (m MIEImpact) Total() int { return m.Memories + m.Summaries + m.Shares + m.CareRequests }

func (s *Store) mieEdge(ctx context.Context, tx pgx.Tx, owner UserID, parent, child MIENode) error {
	if parent.ID == "" || child.ID == "" || parent.Version <= 0 || child.Version <= 0 || parent.ID == child.ID {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO mie_edges (user_id, parent_kind, parent_id, parent_version, child_kind, child_id, child_version, created_at)
		VALUES ($1,$2,$3::uuid,$4,$5,$6::uuid,$7,$8) ON CONFLICT DO NOTHING`,
		owner, parent.Kind, parent.ID, parent.Version, child.Kind, child.ID, child.Version, s.clock())
	return err
}

// mieRecordEdges records the parents of one memory or summary version: its
// pinned sources and, for a summary, the memory claims its statements cite.
func (s *Store) mieRecordEdges(ctx context.Context, tx pgx.Tx, owner UserID, r PersonalRecord) error {
	child := MIENode{Kind: r.Kind, ID: r.ID, Version: r.Version}
	if r.Kind != KindMemory && r.Kind != KindSummary {
		return nil
	}
	for _, ref := range r.SourceRefs {
		if err := s.mieEdge(ctx, tx, owner, MIENode{Kind: MIERecord, ID: ref.RecordID, Version: ref.Version}, child); err != nil {
			return err
		}
	}
	for _, item := range r.Items {
		if item.ClaimRef != nil {
			if err := s.mieEdge(ctx, tx, owner, MIENode{Kind: MIEMemory, ID: item.ClaimRef.RecordID, Version: item.ClaimRef.Version}, child); err != nil {
				return err
			}
		}
	}
	return nil
}

// mieLockOwner serialises graph walks per owner, so two concurrent
// invalidations or an invalidation and a new derivation cannot interleave.
func mieLockOwner(ctx context.Context, tx pgx.Tx, owner UserID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('sela-mie:' || $1::text, 0))`, owner)
	return err
}

// mieDescendants walks the current descendants of origin. A memory or
// summary child counts only at its current version (older versions are
// history), a share only while active and a care request only while open.
// For a record or memory origin every edge from that version or an older
// one is followed; a summary origin follows its exact version.
func (s *Store) mieDescendants(ctx context.Context, q db.DBTX, owner UserID, origin MIENode) ([]MIENode, error) {
	type item struct {
		node  MIENode
		depth int
	}
	queue := []item{{origin, 0}}
	seen := map[string]bool{origin.Kind + origin.ID: true}
	var out []MIENode
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= mieMaxDepth {
			return nil, ErrMIELimit
		}
		op := "<="
		if cur.node.Kind == MIESummary || cur.node.Kind == MIEShare {
			op = "="
		}
		rows, err := q.Query(ctx, `SELECT e.child_kind, e.child_id::text, e.child_version FROM mie_edges e
			WHERE e.user_id=$1 AND e.parent_id::text=$2 AND e.parent_kind=$3 AND e.parent_version `+op+` $4
			ORDER BY e.child_kind, e.child_id`, owner, cur.node.ID, cur.node.Kind, cur.node.Version)
		if err != nil {
			return nil, err
		}
		var children []MIENode
		for rows.Next() {
			var n MIENode
			if err := rows.Scan(&n.Kind, &n.ID, &n.Version); err != nil {
				rows.Close()
				return nil, err
			}
			children = append(children, n)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, n := range children {
			if seen[n.Kind+n.ID] {
				continue
			}
			live, err := mieLive(ctx, q, owner, n)
			if err != nil {
				return nil, err
			}
			if !live {
				continue
			}
			seen[n.Kind+n.ID] = true
			out = append(out, n)
			if len(out) > mieMaxNodes {
				return nil, ErrMIELimit
			}
			queue = append(queue, item{n, cur.depth + 1})
		}
	}
	return out, nil
}

// mieLive reports whether a child is still a current descendant.
func mieLive(ctx context.Context, q db.DBTX, owner UserID, n MIENode) (bool, error) {
	var ok bool
	var err error
	switch n.Kind {
	case MIEMemory, MIESummary:
		err = q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM personal_records WHERE user_id=$1 AND id::text=$2 AND version=$3)`,
			owner, n.ID, n.Version).Scan(&ok)
	case MIEShare:
		err = q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM shares WHERE user_id=$1 AND id::text=$2 AND state='active')`,
			owner, n.ID).Scan(&ok)
	case MIECare:
		err = q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM care_requests WHERE user_id=$1 AND id::text=$2 AND state IN ('sent','proposed'))`,
			owner, n.ID).Scan(&ok)
	}
	return ok, err
}

func tally(nodes []MIENode, impact *MIEImpact) {
	impact.Nodes = nodes
	if impact.Nodes == nil {
		impact.Nodes = []MIENode{}
	}
	for _, n := range nodes {
		switch n.Kind {
		case MIEMemory:
			impact.Memories++
		case MIESummary:
			impact.Summaries++
		case MIEShare:
			impact.Shares++
		case MIECare:
			impact.CareRequests++
		}
	}
}

// invalidate marks every current descendant of origin stale, revokes the
// active shares and cancels the open care requests among them, and records
// one content-free event, all inside tx. Nothing reached means no event.
func (s *Store) invalidate(ctx context.Context, tx pgx.Tx, owner UserID, origin MIENode, reason string) (MIEImpact, error) {
	impact := MIEImpact{Origin: origin, Reason: reason, Nodes: []MIENode{}}
	if err := mieLockOwner(ctx, tx, owner); err != nil {
		return impact, err
	}
	nodes, err := s.mieDescendants(ctx, tx, owner, origin)
	if err != nil {
		return impact, err
	}
	tally(nodes, &impact)
	if len(nodes) == 0 {
		return impact, nil
	}
	now := s.clock()
	impact.EventID = newID().String()
	impact.At = now
	if _, err := tx.Exec(ctx, `INSERT INTO mie_events (id, user_id, origin_kind, origin_id, origin_version, reason, memories, summaries, shares, care_requests, at)
		VALUES ($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9,$10,$11)`, impact.EventID, owner, origin.Kind, origin.ID, origin.Version, reason,
		impact.Memories, impact.Summaries, impact.Shares, impact.CareRequests, now); err != nil {
		return impact, err
	}
	for _, n := range nodes {
		switch n.Kind {
		case MIEMemory, MIESummary:
			if _, err := tx.Exec(ctx, `INSERT INTO mie_stale (record_id, user_id, version, kind, event_id, reason, at)
				VALUES ($1::uuid,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, n.ID, owner, n.Version, n.Kind, impact.EventID, reason, now); err != nil {
				return impact, err
			}
		case MIEShare:
			res, err := tx.Exec(ctx, `UPDATE shares SET state='revoked', version=version+1, updated_at=$3, revoked_at=$3, end_reason='meaning_invalidated'
				WHERE user_id=$1 AND id::text=$2 AND state='active'`, owner, n.ID, now)
			if err != nil {
				return impact, err
			}
			if res.RowsAffected() == 1 {
				if _, err := tx.Exec(ctx, "DELETE FROM share_recipient_sessions WHERE share_id::text=$1", n.ID); err != nil {
					return impact, err
				}
				if err := s.shareEvent(ctx, tx, n.ID, "revoked", now); err != nil {
					return impact, err
				}
			}
		case MIECare:
			res, err := tx.Exec(ctx, `UPDATE care_requests SET state='cancelled', version=version+1, updated_at=$3
				WHERE user_id=$1 AND id::text=$2 AND state IN ('sent','proposed')`, owner, n.ID, now)
			if err != nil {
				return impact, err
			}
			if res.RowsAffected() == 1 {
				if err := s.careEvent(ctx, tx, n.ID, CareCancelled, "system", now); err != nil {
					return impact, err
				}
			}
		}
	}
	return impact, nil
}

// MeaningImpact previews what changing or deleting record id (a source
// record or a memory claim, at its current version) would invalidate. It
// writes nothing.
func (s *Store) MeaningImpact(ctx context.Context, owner UserID, id string) (MIEImpact, error) {
	origin, err := s.mieOrigin(ctx, s.pool, owner, id)
	if err != nil {
		return MIEImpact{}, err
	}
	nodes, err := s.mieDescendants(ctx, s.pool, owner, origin)
	if err != nil {
		return MIEImpact{}, err
	}
	impact := MIEImpact{Origin: origin}
	tally(nodes, &impact)
	return impact, nil
}

func (s *Store) mieOrigin(ctx context.Context, q db.DBTX, owner UserID, id string) (MIENode, error) {
	if _, err := ParseUUID(id); err != nil {
		return MIENode{}, ErrNotFound
	}
	var kind string
	var version int
	err := q.QueryRow(ctx, "SELECT kind, version FROM personal_records WHERE user_id=$1 AND id=$2", owner, id).Scan(&kind, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return MIENode{}, ErrNotFound
	}
	if err != nil {
		return MIENode{}, err
	}
	switch kind {
	case KindMemory:
		return MIENode{Kind: MIEMemory, ID: id, Version: version}, nil
	case KindSummary:
		return MIENode{Kind: MIESummary, ID: id, Version: version}, nil
	case KindFollowup:
		return MIENode{}, ErrNotFound
	}
	return MIENode{Kind: MIERecord, ID: id, Version: version}, nil
}

// MIELineage is the owner's "why did Sela say this?" view of one record:
// what it was derived from and what currently rests on it.
type MIELineage struct {
	Node        MIENode     `json:"node"`
	Stale       *StaleInfo  `json:"stale,omitempty"`
	Parents     []MIEParent `json:"parents"`
	Descendants []MIENode   `json:"descendants"`
}

// MIEParent is one parent of the current version, with whether that
// parent is still at the pinned version.
type MIEParent struct {
	MIENode
	Current bool `json:"current"`
}

// MeaningLineage returns the parents and current descendants of record id.
func (s *Store) MeaningLineage(ctx context.Context, owner UserID, id string) (MIELineage, error) {
	node, err := s.mieOrigin(ctx, s.pool, owner, id)
	if err != nil {
		return MIELineage{}, err
	}
	out := MIELineage{Node: node, Parents: []MIEParent{}}
	stale, err := s.staleOf(ctx, s.pool, owner, id, node.Version)
	if err != nil {
		return out, err
	}
	out.Stale = stale
	rows, err := s.pool.Query(ctx, `SELECT e.parent_kind, e.parent_id::text, e.parent_version,
		COALESCE(pr.version = e.parent_version, false)
		FROM mie_edges e LEFT JOIN personal_records pr ON pr.id = e.parent_id AND pr.user_id = e.user_id
		WHERE e.user_id=$1 AND e.child_id::text=$2 AND e.child_version=$3 ORDER BY e.parent_kind, e.parent_id`, owner, id, node.Version)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p MIEParent
		if err := rows.Scan(&p.Kind, &p.ID, &p.Version, &p.Current); err != nil {
			rows.Close()
			return out, err
		}
		out.Parents = append(out.Parents, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.Descendants, err = s.mieDescendants(ctx, s.pool, owner, node)
	if out.Descendants == nil {
		out.Descendants = []MIENode{}
	}
	return out, err
}

// MeaningEvents lists the owner's latest invalidations, newest first.
func (s *Store) MeaningEvents(ctx context.Context, owner UserID) ([]MIEImpact, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, reason, origin_kind, origin_id::text, origin_version, memories, summaries, shares, care_requests, at
		FROM mie_events WHERE user_id=$1 ORDER BY at DESC LIMIT 20`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MIEImpact{}
	for rows.Next() {
		var m MIEImpact
		if err := rows.Scan(&m.EventID, &m.Reason, &m.Origin.Kind, &m.Origin.ID, &m.Origin.Version, &m.Memories, &m.Summaries,
			&m.Shares, &m.CareRequests, &m.At); err != nil {
			return nil, err
		}
		m.Nodes = []MIENode{}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) staleOf(ctx context.Context, q db.DBTX, owner UserID, id string, version int) (*StaleInfo, error) {
	var st StaleInfo
	err := q.QueryRow(ctx, `SELECT reason, event_id::text, at FROM mie_stale WHERE user_id=$1 AND record_id::text=$2 AND version=$3`,
		owner, id, version).Scan(&st.Reason, &st.EventID, &st.At)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// enrichStale sets Stale on every record whose current version an
// invalidation reached; a stale summary is never effectively approved.
func (s *Store) enrichStale(ctx context.Context, q db.DBTX, owner UserID, recs []PersonalRecord) error {
	if len(recs) == 0 {
		return nil
	}
	rows, err := q.Query(ctx, `SELECT record_id::text, version, reason, event_id::text, at FROM mie_stale WHERE user_id=$1`, owner)
	if err != nil {
		return err
	}
	type key struct {
		id string
		v  int
	}
	marks := map[key]StaleInfo{}
	for rows.Next() {
		var k key
		var st StaleInfo
		if err := rows.Scan(&k.id, &k.v, &st.Reason, &st.EventID, &st.At); err != nil {
			rows.Close()
			return err
		}
		marks[k] = st
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range recs {
		if st, ok := marks[key{recs[i].ID, recs[i].Version}]; ok {
			st := st
			recs[i].Stale = &st
			if recs[i].Kind == KindSummary && recs[i].SummaryState == SummaryApproved {
				recs[i].SummaryState = SummaryDraft
			}
		}
	}
	return nil
}

// mieForget removes the edges of a deleted record or share id.
func mieForget(ctx context.Context, tx pgx.Tx, owner UserID, id string) error {
	_, err := tx.Exec(ctx, `DELETE FROM mie_edges WHERE user_id=$1 AND (parent_id::text=$2 OR child_id::text=$2)`, owner, id)
	return err
}

// ErrQuoteNotInSource refuses a statement whose quote is not in the pinned
// source revision (MIE-I02).
var ErrQuoteNotInSource = &Error{Code: "quote_not_in_source", Field: "items"}

// ErrClaimNotUsable refuses a statement citing a claim that is not the
// owner's current, confirmed or corrected, non-stale memory claim.
var ErrClaimNotUsable = &Error{Code: "claim_not_usable", Field: "items"}

// sourcePlainText is the text a quote may come from: the user's own turns
// of a conversation, otherwise the record text (as the web client builds
// it).
func sourcePlainText(r PersonalRecord) string {
	if r.Kind == KindConversation {
		var parts []string
		for _, t := range r.Turns {
			if t.Role == "user" {
				parts = append(parts, t.Text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return r.Text
}

func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// validateItemBindings checks, inside the summary transaction, that every
// statement quote is verbatim in its pinned source revision (whitespace
// folded) and that every cited claim is usable.
func (s *Store) validateItemBindings(ctx context.Context, tx pgx.Tx, owner UserID, items []SummaryItem) error {
	sources := map[string]string{}
	for _, item := range items {
		if strings.TrimSpace(item.Quote) != "" {
			text, ok := sources[item.SourceRef.RecordID]
			if !ok {
				src, err := s.record(ctx, tx, owner, item.SourceRef.RecordID, false)
				if err != nil {
					return withField(ErrNotFound, "items")
				}
				if src.Version != item.SourceRef.Version {
					return withField(ErrStaleVersion, "items")
				}
				text = collapseSpace(sourcePlainText(src))
				sources[item.SourceRef.RecordID] = text
			}
			if !strings.Contains(text, collapseSpace(item.Quote)) {
				return ErrQuoteNotInSource
			}
		}
		if item.ClaimRef != nil {
			claim, err := s.record(ctx, tx, owner, item.ClaimRef.RecordID, false)
			if err != nil || claim.Kind != KindMemory || claim.Version != item.ClaimRef.Version ||
				(claim.ReviewStatus != ReviewConfirmed && claim.ReviewStatus != ReviewCorrected) {
				return ErrClaimNotUsable
			}
			stale, err := s.staleOf(ctx, tx, owner, claim.ID, claim.Version)
			if err != nil {
				return err
			}
			if stale != nil {
				return ErrClaimNotUsable
			}
		}
	}
	return nil
}

// MIEExport is the owner's lineage graph, invalidations and stale marks in
// the export: ids, versions, reasons and counts only.
type MIEExport struct {
	Edges  []MIEExportEdge  `json:"edges"`
	Events []MIEImpact      `json:"events"`
	Stale  []MIEExportStale `json:"stale"`
}

// MIEExportEdge is one derivation.
type MIEExportEdge struct {
	Parent MIENode   `json:"parent"`
	Child  MIENode   `json:"child"`
	At     time.Time `json:"at"`
}

// MIEExportStale is one stale record version.
type MIEExportStale struct {
	MIENode
	StaleInfo
}

func (s *Store) exportMeaning(ctx context.Context, tx pgx.Tx, owner UserID) (MIEExport, error) {
	out := MIEExport{Edges: []MIEExportEdge{}, Events: []MIEImpact{}, Stale: []MIEExportStale{}}
	rows, err := tx.Query(ctx, `SELECT parent_kind, parent_id::text, parent_version, child_kind, child_id::text, child_version, created_at
		FROM mie_edges WHERE user_id=$1 ORDER BY created_at`, owner)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e MIEExportEdge
		if err := rows.Scan(&e.Parent.Kind, &e.Parent.ID, &e.Parent.Version, &e.Child.Kind, &e.Child.ID, &e.Child.Version, &e.At); err != nil {
			rows.Close()
			return out, err
		}
		out.Edges = append(out.Edges, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text, reason, origin_kind, origin_id::text, origin_version, memories, summaries, shares, care_requests, at
		FROM mie_events WHERE user_id=$1 ORDER BY at`, owner)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var m MIEImpact
		if err := rows.Scan(&m.EventID, &m.Reason, &m.Origin.Kind, &m.Origin.ID, &m.Origin.Version, &m.Memories, &m.Summaries,
			&m.Shares, &m.CareRequests, &m.At); err != nil {
			rows.Close()
			return out, err
		}
		m.Nodes = []MIENode{}
		out.Events = append(out.Events, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT kind, record_id::text, version, reason, event_id::text, at FROM mie_stale WHERE user_id=$1 ORDER BY at`, owner)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var st MIEExportStale
		if err := rows.Scan(&st.Kind, &st.ID, &st.Version, &st.Reason, &st.EventID, &st.At); err != nil {
			return out, err
		}
		out.Stale = append(out.Stale, st)
	}
	return out, rows.Err()
}

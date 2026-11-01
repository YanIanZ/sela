package account

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/vault"
)

// Care requests (D-076) carry one approved consultation summary to one
// partner organisation through a share that has no usable link: the token
// and code are generated and discarded, so the organisation reads the frozen
// projection only through the request, and only while the share is active.
// The organisation answers with proposed times, a mode and its own contact
// instructions; the person confirms one time or cancels. There is no
// free-text chat, and Sela never passes on the person's contact details.

// Care request states.
const (
	CareSent      = "sent"
	CareProposed  = "proposed"
	CareDeclined  = "declined"
	CareConfirmed = "confirmed"
	CareCompleted = "completed"
	CareCancelled = "cancelled"
	CareExpired   = "expired"
)

// careRequestHours is how long the request's share lives: the request can
// be read and answered for seven days.
const careRequestHours = 168

var (
	careModes          = map[string]bool{"online": true, "in_person": true, "either": true}
	careDeclineReasons = map[string]bool{"no_capacity": true, "not_a_fit": true, "refer_elsewhere": true, "other": true}
	// ErrCareOrgUnavailable: the organisation is unknown, not verified, not
	// taking requests, or real while the deployment allows simulated only.
	ErrCareOrgUnavailable = &Error{Code: "care_org_unavailable", Field: "orgId"}
)

// CareProposal is the organisation's answer: up to three future times, a
// mode, how to reach them and an optional short note.
type CareProposal struct {
	Slots   []time.Time `json:"slots"`
	Mode    string      `json:"mode"`
	Contact string      `json:"contact"`
	Note    string      `json:"note,omitempty"`
}

// CareRequest is the owner's view of one request.
type CareRequest struct {
	ID               string        `json:"id"`
	Reference        string        `json:"reference"`
	OrgID            string        `json:"orgId"`
	OrgName          string        `json:"orgName"`
	OrgSynthetic     bool          `json:"orgSynthetic"`
	PractitionerID   string        `json:"practitionerId,omitempty"`
	PractitionerName string        `json:"practitionerName,omitempty"`
	ShareID          string        `json:"shareId"`
	SummaryID        string        `json:"summaryId"`
	SummaryVersion   int           `json:"summaryVersion"`
	Mode             string        `json:"mode"`
	State            string        `json:"state"`
	Proposal         *CareProposal `json:"proposal,omitempty"`
	ConfirmedSlot    *time.Time    `json:"confirmedSlot,omitempty"`
	DeclineReason    string        `json:"declineReason,omitempty"`
	Version          int           `json:"version"`
	ExpiresAt        time.Time     `json:"expiresAt"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
	RespondedAt      *time.Time    `json:"respondedAt,omitempty"`
}

// PartnerCareRequest is what an organisation sees. It carries no user,
// account or session id. Summary is set only for callers allowed to read
// the content, and only while the share is active.
type PartnerCareRequest struct {
	ID               string            `json:"id"`
	Reference        string            `json:"reference"`
	PractitionerID   string            `json:"practitionerId,omitempty"`
	PractitionerName string            `json:"practitionerName,omitempty"`
	Mode             string            `json:"mode"`
	State            string            `json:"state"`
	Synthetic        bool              `json:"synthetic"`
	Proposal         *CareProposal     `json:"proposal,omitempty"`
	ConfirmedSlot    *time.Time        `json:"confirmedSlot,omitempty"`
	DeclineReason    string            `json:"declineReason,omitempty"`
	Version          int               `json:"version"`
	ContentAvailable bool              `json:"contentAvailable"`
	ExpiresAt        time.Time         `json:"expiresAt"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	Summary          *SharedProjection `json:"summary,omitempty"`
}

// CareRequestInput creates a request.
type CareRequestInput struct {
	SummaryID      string `json:"summaryId"`
	OrgID          string `json:"orgId"`
	PractitionerID string `json:"practitionerId"`
	Mode           string `json:"mode"`
}

const careReferenceAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func careReference() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	out := make([]byte, 6)
	for i, x := range b {
		out[i] = careReferenceAlphabet[int(x)%len(careReferenceAlphabet)]
	}
	return "SR-" + string(out), nil
}

func (s *Store) careEvent(ctx context.Context, tx pgx.Tx, id, kind, actor string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO care_request_events (id, request_id, kind, actor, at) VALUES ($1,$2,$3,$4,$5)`,
		newID(), id, kind, actor, at)
	return err
}

// CreateCareRequest freezes an approved summary into a share for one
// verified organisation (and optionally one of its practitioners) and opens
// a request. allowReal mirrors config.CareNetworkReal: without it only a
// synthetic organisation can receive a request.
func (s *Store) CreateCareRequest(ctx context.Context, owner UserID, in CareRequestInput, allowReal bool) (CareRequest, error) {
	if !careModes[in.Mode] {
		return CareRequest{}, invalid("mode")
	}
	if strings.TrimSpace(in.SummaryID) == "" {
		return CareRequest{}, invalid("summaryId")
	}
	token, err := shareToken()
	if err != nil {
		return CareRequest{}, err
	}
	code, err := shareCode()
	if err != nil {
		return CareRequest{}, err
	}
	ref, err := careReference()
	if err != nil {
		return CareRequest{}, err
	}
	now := s.clock()
	id := newID()
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var orgName string
		var synthetic bool
		err := tx.QueryRow(ctx, `SELECT name, synthetic FROM care_orgs WHERE id::text=$1 AND status='verified' AND accepts_requests`, in.OrgID).
			Scan(&orgName, &synthetic)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCareOrgUnavailable
		}
		if err != nil {
			return err
		}
		if !synthetic && !allowReal {
			return ErrCareOrgUnavailable
		}
		if in.PractitionerID != "" {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM admin_users WHERE id::text=$1 AND care_org_id::text=$2
				AND role='practitioner' AND status='active')`, in.PractitionerID, in.OrgID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return invalid("practitionerId")
			}
		}
		label := orgName
		if len([]rune(label)) > 80 {
			label = string([]rune(label)[:80])
		}
		share := Share{ID: newShareID(), SummaryID: in.SummaryID, Label: label, Purpose: "consultation", State: ShareActive,
			Version: 1, CreatedAt: now, ExpiresAt: now.Add(careRequestHours * time.Hour)}
		if err := s.createShareTx(ctx, tx, owner, &share, token, code); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO care_requests (id, user_id, org_id, practitioner_id, share_id, reference, mode, state,
			synthetic, version, created_at, updated_at) VALUES ($1,$2,$3::uuid,NULLIF($4,'')::uuid,$5::uuid,$6,$7,'sent',$8,1,$9,$9)`,
			id, owner, in.OrgID, in.PractitionerID, share.ID, ref, in.Mode, synthetic, now); err != nil {
			return err
		}
		if err := s.careEvent(ctx, tx, id.String(), CareSent, "user", now); err != nil {
			return err
		}
		return s.mieEdge(ctx, tx, owner, MIENode{Kind: MIEShare, ID: share.ID, Version: 1}, MIENode{Kind: MIECare, ID: id.String(), Version: 1})
	})
	if err != nil {
		return CareRequest{}, err
	}
	return s.CareRequest(ctx, owner, id.String())
}

const careOwnerSelect = `SELECT r.id::text, r.reference, r.org_id::text, o.name, o.synthetic, COALESCE(r.practitioner_id::text,''),
	COALESCE(u.display_name,''), r.share_id::text, sh.summary_id::text, sh.summary_version, r.mode, r.state, r.proposal,
	r.confirmed_slot, COALESCE(r.decline_reason,''), r.version, sh.expires_at, r.created_at, r.updated_at, r.responded_at
	FROM care_requests r JOIN care_orgs o ON o.id = r.org_id JOIN shares sh ON sh.id = r.share_id
	LEFT JOIN admin_users u ON u.id = r.practitioner_id`

func scanOwnerCare(row pgx.Row) (CareRequest, error) {
	var c CareRequest
	var proposal []byte
	err := row.Scan(&c.ID, &c.Reference, &c.OrgID, &c.OrgName, &c.OrgSynthetic, &c.PractitionerID, &c.PractitionerName, &c.ShareID,
		&c.SummaryID, &c.SummaryVersion, &c.Mode, &c.State, &proposal, &c.ConfirmedSlot, &c.DeclineReason, &c.Version,
		&c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt, &c.RespondedAt)
	if err != nil {
		return c, err
	}
	if len(proposal) > 0 {
		var p CareProposal
		if json.Unmarshal(proposal, &p) == nil {
			c.Proposal = &p
		}
	}
	return c, nil
}

// settleCare ends open requests whose share was revoked (cancelled) or
// expired (expired). It runs before every read so both sides see the same
// state; scope limits it to one owner or one organisation.
func (s *Store) settleCare(ctx context.Context, scopeColumn string, scope any) error {
	now := s.clock()
	return s.tx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT r.id::text, CASE WHEN sh.state='revoked' THEN 'cancelled' ELSE 'expired' END
			FROM care_requests r JOIN shares sh ON sh.id = r.share_id
			WHERE r.`+scopeColumn+` = $1 AND r.state IN ('sent','proposed')
			  AND (sh.state <> 'active' OR sh.expires_at <= $2) FOR UPDATE OF r`, scope, now)
		if err != nil {
			return err
		}
		type end struct{ id, state string }
		var ends []end
		for rows.Next() {
			var e end
			if err := rows.Scan(&e.id, &e.state); err != nil {
				rows.Close()
				return err
			}
			ends = append(ends, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, e := range ends {
			if _, err := tx.Exec(ctx, `UPDATE care_requests SET state=$2, version=version+1, updated_at=$3 WHERE id::text=$1`, e.id, e.state, now); err != nil {
				return err
			}
			if err := s.careEvent(ctx, tx, e.id, e.state, "system", now); err != nil {
				return err
			}
		}
		return nil
	})
}

// CareRequests lists the owner's requests, newest first.
func (s *Store) CareRequests(ctx context.Context, owner UserID) ([]CareRequest, error) {
	if err := s.settleCare(ctx, "user_id", owner); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, careOwnerSelect+` WHERE r.user_id=$1 ORDER BY r.updated_at DESC LIMIT 100`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CareRequest{}
	for rows.Next() {
		c, err := scanOwnerCare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CareRequest returns one of the owner's requests.
func (s *Store) CareRequest(ctx context.Context, owner UserID, id string) (CareRequest, error) {
	if err := s.settleCare(ctx, "user_id", owner); err != nil {
		return CareRequest{}, err
	}
	c, err := scanOwnerCare(s.pool.QueryRow(ctx, careOwnerSelect+` WHERE r.user_id=$1 AND r.id::text=$2`, owner, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return CareRequest{}, ErrNotFound
	}
	return c, err
}

// ConfirmCareRequest accepts one of the proposed times.
func (s *Store) ConfirmCareRequest(ctx context.Context, owner UserID, id string, slot time.Time, expectedVersion int) (CareRequest, error) {
	if err := s.settleCare(ctx, "user_id", owner); err != nil {
		return CareRequest{}, err
	}
	now := s.clock()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var state string
		var version int
		var proposal []byte
		err := tx.QueryRow(ctx, `SELECT state, version, proposal FROM care_requests WHERE user_id=$1 AND id::text=$2 FOR UPDATE`, owner, id).
			Scan(&state, &version, &proposal)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != expectedVersion {
			return ErrStaleVersion
		}
		if state != CareProposed {
			return ErrInvalidTransition
		}
		var p CareProposal
		if err := json.Unmarshal(proposal, &p); err != nil {
			return err
		}
		match := false
		for _, t := range p.Slots {
			if t.Equal(slot) {
				match = true
			}
		}
		if !match || !slot.After(now) {
			return invalid("slot")
		}
		if _, err := tx.Exec(ctx, `UPDATE care_requests SET state='confirmed', confirmed_slot=$3, version=version+1, updated_at=$4
			WHERE user_id=$1 AND id::text=$2`, owner, id, slot.UTC(), now); err != nil {
			return err
		}
		return s.careEvent(ctx, tx, id, CareConfirmed, "user", now)
	})
	if err != nil {
		return CareRequest{}, err
	}
	return s.CareRequest(ctx, owner, id)
}

// CancelCareRequest ends a request the owner no longer wants and revokes
// its share, so the organisation can no longer read the summary.
func (s *Store) CancelCareRequest(ctx context.Context, owner UserID, id string, expectedVersion int) (CareRequest, error) {
	now := s.clock()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var state, shareID string
		var version int
		err := tx.QueryRow(ctx, `SELECT state, version, share_id::text FROM care_requests WHERE user_id=$1 AND id::text=$2 FOR UPDATE`, owner, id).
			Scan(&state, &version, &shareID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != expectedVersion {
			return ErrStaleVersion
		}
		if state != CareSent && state != CareProposed && state != CareConfirmed {
			return ErrInvalidTransition
		}
		if _, err := tx.Exec(ctx, `UPDATE care_requests SET state='cancelled', version=version+1, updated_at=$3
			WHERE user_id=$1 AND id::text=$2`, owner, id, now); err != nil {
			return err
		}
		res, err := tx.Exec(ctx, `UPDATE shares SET state='revoked', version=version+1, updated_at=$3, revoked_at=$3, end_reason='owner_revoked'
			WHERE user_id=$1 AND id::text=$2 AND state='active'`, owner, shareID, now)
		if err != nil {
			return err
		}
		if res.RowsAffected() == 1 {
			if err := s.shareEvent(ctx, tx, shareID, "revoked", now); err != nil {
				return err
			}
		}
		return s.careEvent(ctx, tx, id, CareCancelled, "user", now)
	})
	if err != nil {
		return CareRequest{}, err
	}
	return s.CareRequest(ctx, owner, id)
}

const carePartnerSelect = `SELECT r.id::text, r.reference, COALESCE(r.practitioner_id::text,''), COALESCE(u.display_name,''),
	r.mode, r.state, r.synthetic, r.proposal, r.confirmed_slot, COALESCE(r.decline_reason,''), r.version,
	(sh.state='active' AND sh.expires_at > $2 AND NOT EXISTS (SELECT 1 FROM mie_stale ms WHERE ms.record_id = sh.summary_id AND ms.version = sh.summary_version)), sh.expires_at, r.created_at, r.updated_at
	FROM care_requests r JOIN shares sh ON sh.id = r.share_id LEFT JOIN admin_users u ON u.id = r.practitioner_id`

func scanPartnerCare(row pgx.Row) (PartnerCareRequest, error) {
	var c PartnerCareRequest
	var proposal []byte
	err := row.Scan(&c.ID, &c.Reference, &c.PractitionerID, &c.PractitionerName, &c.Mode, &c.State, &c.Synthetic, &proposal,
		&c.ConfirmedSlot, &c.DeclineReason, &c.Version, &c.ContentAvailable, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	if len(proposal) > 0 {
		var p CareProposal
		if json.Unmarshal(proposal, &p) == nil {
			c.Proposal = &p
		}
	}
	return c, nil
}

// PartnerCareRequests lists one organisation's requests without content.
func (s *Store) PartnerCareRequests(ctx context.Context, orgID string) ([]PartnerCareRequest, error) {
	if err := s.settleCare(ctx, "org_id::text", orgID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, carePartnerSelect+` WHERE r.org_id::text=$1 ORDER BY r.updated_at DESC LIMIT 200`, orgID, s.clock())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PartnerCareRequest{}
	for rows.Next() {
		c, err := scanPartnerCare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PartnerCareRequest returns one request of the organisation. With content
// it opens the frozen summary (while the share is active) and records the
// opening for the owner's access history.
func (s *Store) PartnerCareRequest(ctx context.Context, orgID, id string, content bool) (PartnerCareRequest, error) {
	if err := s.settleCare(ctx, "org_id::text", orgID); err != nil {
		return PartnerCareRequest{}, err
	}
	now := s.clock()
	c, err := scanPartnerCare(s.pool.QueryRow(ctx, carePartnerSelect+` WHERE r.org_id::text=$1 AND r.id::text=$3`, orgID, now, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return PartnerCareRequest{}, ErrNotFound
	}
	if err != nil || !content || !c.ContentAvailable {
		return c, err
	}
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var owner pgtype.UUID
		var shareID string
		var ct []byte
		if err := tx.QueryRow(ctx, `SELECT sh.user_id, sh.id::text, sh.projection_ct FROM care_requests r JOIN shares sh ON sh.id = r.share_id
			WHERE r.id::text=$1 AND r.org_id::text=$2 AND sh.state='active' AND sh.expires_at > $3 AND NOT EXISTS (SELECT 1 FROM mie_stale ms WHERE ms.record_id = sh.summary_id AND ms.version = sh.summary_version)`, id, orgID, now).Scan(&owner, &shareID, &ct); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		raw, err := s.vault.Open(ctx, tx, owner.Bytes, vault.Ref{Kind: "share", RecordID: shareID, Field: "projection"}, ct)
		if err != nil {
			return err
		}
		var p SharedProjection
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		p.ShareID = ""
		c.Summary = &p
		if err := s.shareEvent(ctx, tx, shareID, "opened", now); err != nil {
			return err
		}
		return s.careEvent(ctx, tx, id, "opened", "partner", now)
	})
	return c, err
}

// PartnerRespondInput is the organisation's decision.
type PartnerRespondInput struct {
	Decision        string        `json:"decision"`
	Proposal        *CareProposal `json:"proposal"`
	DeclineReason   string        `json:"declineReason"`
	ExpectedVersion int           `json:"expectedVersion"`
}

// PartnerRespond proposes times or declines an open request of the
// organisation. A new proposal may replace an earlier one until the person
// confirms.
func (s *Store) PartnerRespond(ctx context.Context, orgID, id string, in PartnerRespondInput) (PartnerCareRequest, error) {
	if err := s.settleCare(ctx, "org_id::text", orgID); err != nil {
		return PartnerCareRequest{}, err
	}
	now := s.clock()
	var proposalJSON []byte
	switch in.Decision {
	case "propose":
		p := in.Proposal
		if p == nil || len(p.Slots) == 0 || len(p.Slots) > 3 || (p.Mode != "online" && p.Mode != "in_person") {
			return PartnerCareRequest{}, invalid("proposal")
		}
		p.Contact, p.Note = strings.TrimSpace(p.Contact), strings.TrimSpace(p.Note)
		if n := len([]rune(p.Contact)); n < 3 || n > 300 {
			return PartnerCareRequest{}, invalid("proposal.contact")
		}
		if len([]rune(p.Note)) > 500 {
			return PartnerCareRequest{}, invalid("proposal.note")
		}
		for i, t := range p.Slots {
			if !t.After(now) || t.After(now.Add(60*24*time.Hour)) {
				return PartnerCareRequest{}, invalid("proposal.slots")
			}
			p.Slots[i] = t.UTC().Truncate(time.Minute)
		}
		var err error
		if proposalJSON, err = json.Marshal(p); err != nil {
			return PartnerCareRequest{}, err
		}
	case "decline":
		if !careDeclineReasons[in.DeclineReason] {
			return PartnerCareRequest{}, invalid("declineReason")
		}
	default:
		return PartnerCareRequest{}, invalid("decision")
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var state string
		var version int
		err := tx.QueryRow(ctx, `SELECT state, version FROM care_requests WHERE org_id::text=$1 AND id::text=$2 FOR UPDATE`, orgID, id).
			Scan(&state, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != in.ExpectedVersion {
			return ErrStaleVersion
		}
		if state != CareSent && state != CareProposed {
			return ErrInvalidTransition
		}
		if in.Decision == "propose" {
			_, err = tx.Exec(ctx, `UPDATE care_requests SET state='proposed', proposal=$3, version=version+1, updated_at=$4, responded_at=$4
				WHERE org_id::text=$1 AND id::text=$2`, orgID, id, proposalJSON, now)
			if err != nil {
				return err
			}
			return s.careEvent(ctx, tx, id, CareProposed, "partner", now)
		}
		_, err = tx.Exec(ctx, `UPDATE care_requests SET state='declined', decline_reason=$3, proposal=NULL, version=version+1, updated_at=$4, responded_at=$4
			WHERE org_id::text=$1 AND id::text=$2`, orgID, id, in.DeclineReason, now)
		if err != nil {
			return err
		}
		return s.careEvent(ctx, tx, id, CareDeclined, "partner", now)
	})
	if err != nil {
		return PartnerCareRequest{}, err
	}
	return s.PartnerCareRequest(ctx, orgID, id, false)
}

// PartnerComplete marks a confirmed request as held.
func (s *Store) PartnerComplete(ctx context.Context, orgID, id string, expectedVersion int) (PartnerCareRequest, error) {
	now := s.clock()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var state string
		var version int
		err := tx.QueryRow(ctx, `SELECT state, version FROM care_requests WHERE org_id::text=$1 AND id::text=$2 FOR UPDATE`, orgID, id).
			Scan(&state, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != expectedVersion {
			return ErrStaleVersion
		}
		if state != CareConfirmed {
			return ErrInvalidTransition
		}
		if _, err := tx.Exec(ctx, `UPDATE care_requests SET state='completed', version=version+1, updated_at=$3
			WHERE org_id::text=$1 AND id::text=$2`, orgID, id, now); err != nil {
			return err
		}
		return s.careEvent(ctx, tx, id, CareCompleted, "partner", now)
	})
	if err != nil {
		return PartnerCareRequest{}, err
	}
	return s.PartnerCareRequest(ctx, orgID, id, false)
}

// ExportedCareRequest is one request in the owner's export, with its
// history. The summary itself is exported with the share and the records.
type ExportedCareRequest struct {
	CareRequest
	Events []CareRequestEvent `json:"events"`
}

// CareRequestEvent is one step of a request's history.
type CareRequestEvent struct {
	Kind  string    `json:"kind"`
	Actor string    `json:"actor"`
	At    time.Time `json:"at"`
}

func (s *Store) exportCareRequests(ctx context.Context, tx pgx.Tx, owner UserID) ([]ExportedCareRequest, error) {
	rows, err := tx.Query(ctx, careOwnerSelect+` WHERE r.user_id=$1 ORDER BY r.created_at`, owner)
	if err != nil {
		return nil, err
	}
	out := []ExportedCareRequest{}
	for rows.Next() {
		c, err := scanOwnerCare(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, ExportedCareRequest{CareRequest: c, Events: []CareRequestEvent{}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		ev, err := tx.Query(ctx, `SELECT kind, actor, at FROM care_request_events WHERE request_id::text=$1 ORDER BY at`, out[i].ID)
		if err != nil {
			return nil, err
		}
		for ev.Next() {
			var e CareRequestEvent
			if err := ev.Scan(&e.Kind, &e.Actor, &e.At); err != nil {
				ev.Close()
				return nil, err
			}
			out[i].Events = append(out[i].Events, e)
		}
		ev.Close()
		if err := ev.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

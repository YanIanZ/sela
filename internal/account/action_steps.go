package account

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/safety"
)

// Personal Action Bridge (BW-3, "Satu langkah, pilihanku"). An action step
// is one small, optional thing the owner chose to try. It reuses the sealed
// personal-record envelope (kind action_step): the text, status, optional
// due date, origin and source pins live inside the per-owner encrypted
// content, every change is an append-only revision guarded by
// expectedVersion, and export and account/guest deletion cover it with the
// other records.
//
// It is never a treatment, score or outcome: "tried" is self-reported, a
// declined or paused step records nothing about the person, and there are no
// reminders. An action pinned to a source record version is a leaf in the
// MIE graph: editing or deleting the source marks the action's current
// version stale, which reads as needs_review until the owner reaffirms,
// detaches or discards it. The text is never removed by an invalidation.

// KindActionStep is the personal-record kind of an action step.
const KindActionStep = "action_step"

// Action step statuses. needs_review is never stored: it is the read-time
// status of a non-discarded step whose current version an invalidation
// reached. "draft" is the client's unsaved editor and is never persisted.
const (
	ActionChosen      = "chosen"
	ActionTried       = "tried"
	ActionPaused      = "paused"
	ActionDiscarded   = "discarded"
	ActionNeedsReview = "needs_review"
)

// Action origins. A Meta proposal is recorded only after the owner accepted
// it, and only with a proposal token the server issued for that owner.
const (
	ActionOriginUser = "user_written"
	ActionOriginMeta = "meta_proposal"
)

// Bounds of one action step.
const (
	ActionTextMax       = 500
	ActionMaxSourceRefs = 3
	// ActionProposalTTL bounds how long a released proposal may be accepted.
	ActionProposalTTL = 30 * time.Minute
)

// ActionProposalKeyPurpose is the HKDF info of the proposal-token key
// (vault.DeriveKey). The key never reaches the database.
const ActionProposalKeyPurpose = "sela.action-proposal.v1"

// ActionDetails is the sealed state of one action-step version.
type ActionDetails struct {
	Status  string       `json:"status"`
	DueDate string       `json:"dueDate,omitempty"`
	Origin  ActionOrigin `json:"origin"`
}

// ActionOrigin says who wrote the text. Edited is true when the owner
// changed a Meta proposal before saving it; Model and Draft are the
// proposal's generator model and draft provenance (live_meta, test_double).
type ActionOrigin struct {
	Kind   string `json:"kind"`
	Edited bool   `json:"edited,omitempty"`
	Model  string `json:"model,omitempty"`
	Draft  string `json:"draft,omitempty"`
}

// ActionSourceRef is one owner-pinned source version with whether the
// source is still at that version.
type ActionSourceRef struct {
	RecordID string `json:"recordId"`
	Version  int    `json:"version"`
	Kind     string `json:"kind,omitempty"`
	Current  bool   `json:"current"`
}

// ActionReview is the owner-approval and MIE state of the current version.
type ActionReview struct {
	UserApproved bool       `json:"userApproved"`
	Stale        bool       `json:"stale"`
	StaleReason  string     `json:"staleReason,omitempty"`
	StaleAt      *time.Time `json:"staleAt,omitempty"`
}

// ActionStep is the owner's view of one action step.
type ActionStep struct {
	ID         string            `json:"id"`
	Version    int               `json:"version"`
	Text       string            `json:"text"`
	Status     string            `json:"status"`
	DueDate    string            `json:"dueDate,omitempty"`
	SourceRefs []ActionSourceRef `json:"sourceRefs"`
	Origin     ActionOrigin      `json:"origin"`
	Review     ActionReview      `json:"review"`
	Scope      string            `json:"scope"`
	Safety     string            `json:"safety"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// ActionProposalAcceptance carries a released proposal the owner accepted:
// the server-issued token and the proposal text exactly as released. The
// saved text may differ (the owner edited it).
type ActionProposalAcceptance struct {
	Token string `json:"token"`
	Text  string `json:"text"`
}

// ActionStepInput creates an action step. SourceRefs are set only when the
// owner attached a source; they must name the source's current version.
type ActionStepInput struct {
	Text           string                    `json:"text"`
	DueDate        string                    `json:"dueDate"`
	SourceRefs     []SourceRef               `json:"sourceRefs"`
	StorageGranted bool                      `json:"storageGranted"`
	Proposal       *ActionProposalAcceptance `json:"proposal"`
}

// ActionStepPatch changes an action step in a new revision. Nil fields are
// unchanged. SourceRefs replaces the pins (an empty list detaches every
// source); Reaffirm re-pins the existing sources to their current versions.
type ActionStepPatch struct {
	ExpectedVersion int          `json:"expectedVersion"`
	Text            *string      `json:"text"`
	DueDate         *string      `json:"dueDate"`
	Status          *string      `json:"status"`
	SourceRefs      *[]SourceRef `json:"sourceRefs"`
	Reaffirm        bool         `json:"reaffirm"`
}

// ActionProposal is a released Meta proposal the server signs for one owner.
// It is never persisted: only the owner's explicit save stores an action.
type ActionProposal struct {
	Text      string
	SourceRef *SourceRef
	Model     string
	Draft     string
}

// ErrProposalInvalid refuses an acceptance whose token is not one this
// server issued to this owner for that exact text, or has expired.
var ErrProposalInvalid = &Error{Code: ErrInvalidInput.Code, Field: "proposal"}

func validActionText(text string) bool {
	t := strings.TrimSpace(text)
	return t != "" && utf8.ValidString(text) && utf8.RuneCountInString(text) <= ActionTextMax &&
		!strings.ContainsAny(text, "\x00")
}

func validDueDate(d string) bool {
	if d == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

func validActionRefs(refs []SourceRef) error {
	if len(refs) > ActionMaxSourceRefs {
		return invalid("sourceRefs")
	}
	for _, ref := range refs {
		// Only the id and version are pinned: no quote or turn key is
		// copied out of the source into the action.
		if ref.Quote != "" || ref.TurnKey != "" {
			return invalid("sourceRefs")
		}
	}
	return nil
}

func plainRefs(refs []SourceRef) []SourceRef {
	out := make([]SourceRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, SourceRef{RecordID: r.RecordID, Version: r.Version})
	}
	return out
}

func actionRecord(id string, text string, refs []SourceRef, d ActionDetails, version int, created, now time.Time) PersonalRecord {
	a := safety.Assess(text)
	prov := "user_original"
	if d.Origin.Kind == ActionOriginMeta {
		prov = "user_accepted_proposal"
	}
	if version > 1 {
		prov = "user_correction"
	}
	return PersonalRecord{
		ID: id, Kind: KindActionStep, Text: text, Version: version,
		Provenance: prov, ReviewStatus: ReviewConfirmed,
		Safety: string(a.Level), Ruleset: a.Ruleset,
		SourceRefs: plainRefs(refs), Action: &d,
		CreatedAt: created, UpdatedAt: now,
	}
}

// mieActionEdges pins one action version under each of its sources.
func (s *Store) mieActionEdges(ctx context.Context, tx pgx.Tx, owner UserID, r PersonalRecord) error {
	child := MIENode{Kind: MIEAction, ID: r.ID, Version: r.Version}
	for _, ref := range r.SourceRefs {
		if err := s.mieEdge(ctx, tx, owner, MIENode{Kind: MIERecord, ID: ref.RecordID, Version: ref.Version}, child); err != nil {
			return err
		}
	}
	return nil
}

// CreateActionStep stores one action step the owner explicitly chose. A
// Meta origin needs a valid proposal token for this owner; the saved text is
// exactly what the owner sent. Source pins are checked against the current
// source versions under the MIE owner lock.
func (s *Store) CreateActionStep(ctx context.Context, owner UserID, in ActionStepInput) (ActionStep, error) {
	if !in.StorageGranted {
		return ActionStep{}, ErrStorageRequired
	}
	if !validActionText(in.Text) {
		return ActionStep{}, invalid("text")
	}
	if !validDueDate(in.DueDate) {
		return ActionStep{}, invalid("dueDate")
	}
	if err := validActionRefs(in.SourceRefs); err != nil {
		return ActionStep{}, err
	}
	origin := ActionOrigin{Kind: ActionOriginUser}
	if in.Proposal != nil {
		p, err := s.verifyActionProposal(owner, in.Proposal.Token, in.Proposal.Text)
		if err != nil {
			return ActionStep{}, err
		}
		origin = ActionOrigin{Kind: ActionOriginMeta, Model: p.Model, Draft: p.Draft,
			Edited: strings.TrimSpace(in.Text) != strings.TrimSpace(in.Proposal.Text)}
	}
	id := newID().String()
	now := s.clock()
	r := actionRecord(id, in.Text, in.SourceRefs, ActionDetails{Status: ActionChosen, DueDate: in.DueDate, Origin: origin}, 1, now, now)
	var out ActionStep
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, true); err != nil {
			return err
		}
		// Sources first (FOR SHARE), then the MIE lock: the same order a
		// source edit takes (row, then lock), so the two cannot deadlock.
		if err := s.validateSourceRefs(ctx, tx, owner, r.SourceRefs, true); err != nil {
			return err
		}
		if err := mieLockOwner(ctx, tx, owner); err != nil {
			return err
		}
		if err := s.insertContinuityRecord(ctx, tx, owner, r); err != nil {
			return err
		}
		if err := s.mieActionEdges(ctx, tx, owner, r); err != nil {
			return err
		}
		v, err := s.actionView(ctx, tx, owner, r)
		out = v
		return err
	})
	return out, err
}

// ActionSteps lists the owner's action steps, most recently changed first.
// Nothing is inferred: only steps the owner saved exist.
func (s *Store) ActionSteps(ctx context.Context, owner UserID) ([]ActionStep, error) {
	recs, err := s.recordsByKind(ctx, s.pool, owner, KindActionStep)
	if err != nil {
		return nil, err
	}
	out := make([]ActionStep, 0, len(recs))
	for _, r := range recs {
		v, err := s.actionView(ctx, s.pool, owner, r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// ActionStep returns one of the owner's action steps; any other id, a
// foreign one included, is not_found.
func (s *Store) ActionStep(ctx context.Context, owner UserID, id string) (ActionStep, error) {
	r, err := s.record(ctx, s.pool, owner, id, false)
	if err != nil {
		return ActionStep{}, err
	}
	if r.Kind != KindActionStep || r.Action == nil {
		return ActionStep{}, ErrNotFound
	}
	stale, err := s.staleOf(ctx, s.pool, owner, r.ID, r.Version)
	if err != nil {
		return ActionStep{}, err
	}
	r.Stale = stale
	return s.actionView(ctx, s.pool, owner, r)
}

// lockedAction loads the owner's action row for a change. FOR NO KEY UPDATE
// lets a concurrent invalidation still take the key-share lock its stale
// mark's foreign key needs, so it waits on the MIE lock instead of
// deadlocking on the row.
func (s *Store) lockedAction(ctx context.Context, tx pgx.Tx, owner UserID, id string) (PersonalRecord, error) {
	if _, err := ParseUUID(id); err != nil {
		return PersonalRecord{}, ErrNotFound
	}
	var v int
	var kind string
	var ct []byte
	err := tx.QueryRow(ctx, "SELECT kind,version,content_ct FROM personal_records WHERE user_id=$1 AND id=$2 FOR NO KEY UPDATE",
		owner, id).Scan(&kind, &v, &ct)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && kind != KindActionStep) {
		return PersonalRecord{}, ErrNotFound
	}
	if err != nil {
		return PersonalRecord{}, err
	}
	r, err := s.openRecord(ctx, tx, owner, id, v, ct)
	if err != nil {
		return PersonalRecord{}, err
	}
	if r.Action == nil {
		return PersonalRecord{}, ErrNotFound
	}
	return r, nil
}

// actionTransition applies the BW-3 lifecycle. from is the effective status
// (needs_review when stale); resolving says the patch reaffirms or replaces
// the source pins. It returns the stored status of the new revision.
func actionTransition(from string, to *string, resolving bool) (string, error) {
	target := ""
	if to != nil {
		target = *to
		switch target {
		case ActionChosen, ActionTried, ActionPaused, ActionDiscarded:
		default:
			return "", invalid("status")
		}
	}
	if target == ActionDiscarded {
		return ActionDiscarded, nil // the owner may discard at any time
	}
	switch from {
	case ActionNeedsReview:
		// Only a review (reaffirm, re-pin or detach) leaves needs_review,
		// and it lands on chosen.
		if !resolving || (target != "" && target != ActionChosen) {
			return "", ErrInvalidTransition
		}
		return ActionChosen, nil
	case ActionDiscarded:
		// A discarded step can only be restored as chosen (an undo).
		if target != ActionChosen {
			return "", ErrInvalidTransition
		}
		return ActionChosen, nil
	case ActionChosen:
		if target == "" {
			return ActionChosen, nil
		}
		return target, nil
	case ActionTried:
		if target == "" || target == ActionTried || target == ActionChosen {
			return orStatus(target, ActionTried), nil
		}
	case ActionPaused:
		if target == "" || target == ActionPaused || target == ActionChosen {
			return orStatus(target, ActionPaused), nil
		}
	}
	return "", ErrInvalidTransition
}

func orStatus(target, current string) string {
	if target == "" {
		return current
	}
	return target
}

// UpdateActionStep writes one new revision of the owner's action step
// after checking expectedVersion, the lifecycle and, under the MIE owner
// lock, whether the current version is stale.
func (s *Store) UpdateActionStep(ctx context.Context, owner UserID, id string, in ActionStepPatch) (ActionStep, error) {
	if in.ExpectedVersion <= 0 {
		return ActionStep{}, invalid("expectedVersion")
	}
	if in.Text == nil && in.DueDate == nil && in.Status == nil && in.SourceRefs == nil && !in.Reaffirm {
		return ActionStep{}, invalid("body")
	}
	if in.Reaffirm && in.SourceRefs != nil {
		return ActionStep{}, invalid("reaffirm")
	}
	if in.Text != nil && !validActionText(*in.Text) {
		return ActionStep{}, invalid("text")
	}
	if in.DueDate != nil && !validDueDate(*in.DueDate) {
		return ActionStep{}, invalid("dueDate")
	}
	if in.SourceRefs != nil {
		if err := validActionRefs(*in.SourceRefs); err != nil {
			return ActionStep{}, err
		}
	}
	var out ActionStep
	err := s.tx(ctx, func(tx pgx.Tx) error {
		before, err := s.lockedAction(ctx, tx, owner, id)
		if err != nil {
			return err
		}
		if before.Version != in.ExpectedVersion {
			return ErrStaleVersion
		}
		refs := before.SourceRefs
		switch {
		case in.SourceRefs != nil:
			refs = plainRefs(*in.SourceRefs)
		case in.Reaffirm:
			if len(refs) == 0 {
				return invalid("reaffirm")
			}
			repinned := make([]SourceRef, 0, len(refs))
			for _, ref := range refs {
				var kind string
				var v int
				err := tx.QueryRow(ctx, "SELECT kind,version FROM personal_records WHERE user_id=$1 AND id::text=$2 FOR SHARE",
					owner, ref.RecordID).Scan(&kind, &v)
				if errors.Is(err, pgx.ErrNoRows) {
					// A deleted source cannot be reaffirmed: detach or discard.
					return withField(ErrNotFound, "sourceRefs")
				}
				if err != nil {
					return err
				}
				repinned = append(repinned, SourceRef{RecordID: ref.RecordID, Version: v})
			}
			refs = repinned
		}
		if in.SourceRefs != nil || in.Reaffirm {
			if err := s.validateSourceRefs(ctx, tx, owner, refs, true); err != nil {
				return err
			}
		}
		if err := mieLockOwner(ctx, tx, owner); err != nil {
			return err
		}
		stale, err := s.staleOf(ctx, tx, owner, before.ID, before.Version)
		if err != nil {
			return err
		}
		from := before.Action.Status
		if stale != nil && from != ActionDiscarded {
			from = ActionNeedsReview
		}
		status, err := actionTransition(from, in.Status, in.SourceRefs != nil || in.Reaffirm)
		if err != nil {
			return err
		}
		if from == ActionDiscarded && (in.Text != nil || in.DueDate != nil || in.SourceRefs != nil || in.Reaffirm) {
			return ErrInvalidTransition // restore first, then edit
		}
		details := *before.Action
		details.Status = status
		text := before.Text
		if in.Text != nil {
			text = *in.Text
			if details.Origin.Kind == ActionOriginMeta && strings.TrimSpace(text) != strings.TrimSpace(before.Text) {
				details.Origin.Edited = true
			}
		}
		if in.DueDate != nil {
			details.DueDate = *in.DueDate
		}
		result := actionRecord(id, text, refs, details, before.Version+1, before.CreatedAt, s.clock())
		if err := s.updateContinuityRecord(ctx, tx, owner, result); err != nil {
			return err
		}
		if err := s.mieActionEdges(ctx, tx, owner, result); err != nil {
			return err
		}
		v, err := s.actionView(ctx, tx, owner, result)
		out = v
		return err
	})
	return out, err
}

// DeleteActionStep removes the owner's action step, every revision, its
// stale marks (by cascade) and its lineage edges in one transaction.
func (s *Store) DeleteActionStep(ctx context.Context, owner UserID, id string, version int) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		r, err := s.lockedAction(ctx, tx, owner, id)
		if err != nil {
			return err
		}
		if version != r.Version {
			return ErrStaleVersion
		}
		if err := mieLockOwner(ctx, tx, owner); err != nil {
			return err
		}
		if err := mieForget(ctx, tx, owner, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM personal_records WHERE user_id=$1 AND id=$2 AND kind=$3", owner, id, KindActionStep)
		return err
	})
}

// actionView builds the owner view of r (r.Stale must already be set).
func (s *Store) actionView(ctx context.Context, q db.DBTX, owner UserID, r PersonalRecord) (ActionStep, error) {
	d := ActionDetails{Status: ActionChosen, Origin: ActionOrigin{Kind: ActionOriginUser}}
	if r.Action != nil {
		d = *r.Action
	}
	v := ActionStep{
		ID: r.ID, Version: r.Version, Text: r.Text, Status: d.Status, DueDate: d.DueDate,
		SourceRefs: []ActionSourceRef{}, Origin: d.Origin, Scope: "private", Safety: r.Safety,
		Review:    ActionReview{UserApproved: true},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.Stale != nil {
		at := r.Stale.At
		v.Review.Stale, v.Review.StaleReason, v.Review.StaleAt = true, r.Stale.Reason, &at
		if d.Status != ActionDiscarded {
			v.Status = ActionNeedsReview
			// A step awaiting review is not an approved current fact.
			v.Review.UserApproved = false
		}
	}
	for _, ref := range r.SourceRefs {
		out := ActionSourceRef{RecordID: ref.RecordID, Version: ref.Version}
		var kind string
		var cur int
		err := q.QueryRow(ctx, "SELECT kind,version FROM personal_records WHERE user_id=$1 AND id::text=$2", owner, ref.RecordID).Scan(&kind, &cur)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return ActionStep{}, err
		default:
			out.Kind, out.Current = kind, cur == ref.Version
		}
		v.SourceRefs = append(v.SourceRefs, out)
	}
	return v, nil
}

// exportActionSteps is the owner's action steps with their effective
// status, for the export's actionSteps section.
func (s *Store) exportActionSteps(ctx context.Context, tx pgx.Tx, owner UserID) ([]ActionStep, error) {
	recs, err := s.recordsByKind(ctx, tx, owner, KindActionStep)
	if err != nil {
		return nil, err
	}
	out := make([]ActionStep, 0, len(recs))
	for _, r := range recs {
		v, err := s.actionView(ctx, tx, owner, r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// proposalClaims is the signed, content-free body of a proposal token: the
// owner, a hash of the released text, the optional source pin, the model
// and draft provenance, and the expiry.
type proposalClaims struct {
	Owner   string `json:"o"`
	Text    string `json:"t"`
	Source  string `json:"s,omitempty"`
	Version int    `json:"v,omitempty"`
	Model   string `json:"m"`
	Draft   string `json:"d"`
	Expires int64  `json:"e"`
}

func textDigest(text string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (s *Store) proposalKey() ([]byte, error) { return s.vault.DeriveKey(ActionProposalKeyPurpose) }

// SignActionProposal issues the token a client returns when the owner
// accepts a released Meta proposal. It persists nothing.
func (s *Store) SignActionProposal(_ context.Context, owner UserID, p ActionProposal) (string, error) {
	key, err := s.proposalKey()
	if err != nil {
		return "", err
	}
	c := proposalClaims{Owner: owner.String(), Text: textDigest(p.Text), Model: p.Model, Draft: p.Draft,
		Expires: s.clock().Add(ActionProposalTTL).Unix()}
	if p.SourceRef != nil {
		c.Source, c.Version = p.SourceRef.RecordID, p.SourceRef.Version
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Store) verifyActionProposal(owner UserID, token, text string) (ActionProposal, error) {
	if len(token) > 2048 || !validActionText(text) {
		return ActionProposal{}, ErrProposalInvalid
	}
	body, sig, ok := strings.Cut(token, ".")
	if !ok {
		return ActionProposal{}, ErrProposalInvalid
	}
	key, err := s.proposalKey()
	if err != nil {
		return ActionProposal{}, err
	}
	want := hmac.New(sha256.New, key)
	want.Write([]byte(body))
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, want.Sum(nil)) {
		return ActionProposal{}, ErrProposalInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return ActionProposal{}, ErrProposalInvalid
	}
	var c proposalClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return ActionProposal{}, ErrProposalInvalid
	}
	if c.Owner != owner.String() || c.Text != textDigest(text) || s.clock().Unix() > c.Expires {
		return ActionProposal{}, ErrProposalInvalid
	}
	p := ActionProposal{Text: text, Model: c.Model, Draft: c.Draft}
	if c.Source != "" {
		p.SourceRef = &SourceRef{RecordID: c.Source, Version: c.Version}
	}
	return p, nil
}

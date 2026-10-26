package journey

import (
	"fmt"
	"regexp"
	"time"
)

// HandoffInput creates a simulated request and its share grant.
type HandoffInput struct {
	ExpectedVersion int          `json:"expectedVersion"`
	LedgerVersion   int          `json:"ledgerVersion"`
	ProfessionalID  string       `json:"professionalId"`
	SlotID          string       `json:"slotId"`
	Fields          []ShareField `json:"fields"`
	DurationHours   int          `json:"durationHours"`
	ShareConsent    bool         `json:"shareConsent"`
	SharingPolicy   string       `json:"sharingPolicy"`
}

// HandoffCreated is returned once per idempotency key. Link secrets are
// returned to the owner only; the recipient needs both.
type HandoffCreated struct {
	Handoff HandoffView  `json:"handoff"`
	Token   string       `json:"token"`
	Code    string       `json:"code"`
	Replay  bool         `json:"replay"`
	View    *SessionView `json:"session"`
}

var idemKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// CreateHandoff shares the exact approved snapshot with one fictional
// recipient (SPEC V5, V11). Requirements: no crisis interruption, current
// approval of exactly LedgerVersion, explicit share consent for the current
// sharing policy, a valid fixture recipient and available slot, a field
// selection that includes the brief items, and an Idempotency-Key. The same
// key and payload return the same receipt; a changed payload conflicts. A
// session holds at most MaxHandoffs handoffs (limit_reached).
func (s *Service) CreateHandoff(sessionID, idemKey string, in HandoffInput) (*HandoffCreated, error) {
	if !idemKeyPattern.MatchString(idemKey) {
		return nil, errf(CodeIdempotencyKey, 400, "Idempotency-Key")
	}
	payload := hashJSON(struct {
		L  int
		P  string
		Sl string
		F  []ShareField
		D  int
	}{in.LedgerVersion, in.ProfessionalID, in.SlotID, in.Fields, in.DurationHours})
	var out *HandoffCreated
	err := s.update(sessionID, func(sess *Session) error {
		if prev, ok := sess.idem[idemKey]; ok {
			if prev.payloadHash != payload {
				return errf(CodeIdempotency, 409, "Idempotency-Key")
			}
			h := sess.handoff(prev.handoffID)
			if h == nil {
				return errf(CodeIdempotency, 409, "Idempotency-Key")
			}
			out = &HandoffCreated{Handoff: s.handoffView(sess, h), Token: h.Token, Code: h.Code, Replay: true, View: s.buildView(sess)}
			return nil
		}
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		snap, err := s.requireApproved(sess)
		if err != nil {
			return err
		}
		if snap.Version != in.LedgerVersion {
			return errf(CodeStale, 409, "ledgerVersion")
		}
		if len(sess.Handoffs) >= MaxHandoffs {
			return errLimit("handoffs")
		}
		if !in.ShareConsent {
			return errf(CodeConsentRequired, 403, "shareConsent")
		}
		if in.SharingPolicy != s.policy("sharing") {
			return errf(CodePolicyOutdated, 409, "sharingPolicy")
		}
		prof, ok := s.catalog.Professional(in.ProfessionalID)
		if !ok {
			return errf(CodeInvalid, 400, "professionalId")
		}
		slotProf, slot, ok := s.catalog.Slot(in.SlotID)
		if !ok || slotProf.ID != prof.ID {
			return errf(CodeInvalid, 400, "slotId")
		}
		if !slot.Available {
			return errf(CodeSlotUnavailable, 409, "slotId")
		}
		fields, err := validFields(in.Fields)
		if err != nil {
			return err
		}
		if fieldSet(fields)[FieldOpening] && (sess.Opening == nil || sess.Opening.BriefVersion != snap.Version) {
			return errf(CodeInvalid, 409, "fields.opening")
		}
		if fieldSet(fields)[FieldPreferences] && sess.Preferences == nil {
			return errf(CodeInvalid, 409, "fields.preferences")
		}
		if in.DurationHours != 1 && in.DurationHours != 24 && in.DurationHours != 72 {
			return errf(CodeInvalid, 400, "durationHours")
		}
		now := s.now()
		var replaces *Handoff
		for _, h := range sess.Handoffs {
			if h.ProfessionalID != prof.ID || s.shareState(h, now) != ShareActive {
				continue
			}
			switch h.Status {
			case HandoffClarification:
				replaces = h
			case HandoffRequested, HandoffAcknowledged:
				return errf(CodeHandoffExists, 409, h.ID)
			}
			if h.SlotID == slot.ID && h.Status != HandoffClarification {
				return errf(CodeSlotUnavailable, 409, "slotId")
			}
		}
		expires := now.Add(time.Duration(in.DurationHours) * time.Hour)
		if expires.After(sess.ExpiresAt) {
			expires = sess.ExpiresAt
		}
		sess.nextHandoff++
		h := &Handoff{
			ID:             fmt.Sprintf("h%d", sess.nextHandoff),
			Version:        1,
			ProfessionalID: prof.ID,
			SlotID:         slot.ID,
			Brief:          snap,
			Fields:         fields,
			Status:         HandoffRequested,
			Share:          ShareActive,
			CreatedAt:      now,
			ExpiresAt:      expires,
			ConsentPolicy:  in.SharingPolicy,
			Token:          randomToken(32),
			Code:           randomCode(),
			idemKey:        idemKey,
		}
		h.Projection = s.project(sess, h, snap)
		h.Timeline = []TimelineEvent{{Kind: "requested", At: now, Actor: "user"}}
		if replaces != nil {
			s.endShare(sess, replaces, ShareRevoked, "superseded", now)
			h.ReplacesID = replaces.ID
		}
		sess.Handoffs = append(sess.Handoffs, h)
		sess.idem[idemKey] = idemRecord{payloadHash: payload, handoffID: h.ID}
		s.store.IndexShare(HashToken(h.Token), ShareRef{SessionID: sess.ID, HandoffID: h.ID})
		bump(sess)
		s.metrics.add("handoff_requested")
		out = &HandoffCreated{Handoff: s.handoffView(sess, h), Token: h.Token, Code: h.Code, View: s.buildView(sess)}
		return nil
	})
	return out, err
}

func validFields(in []ShareField) ([]ShareField, error) {
	set := map[ShareField]bool{}
	for _, f := range in {
		ok := false
		for _, a := range AllShareFields {
			if f == a {
				ok = true
			}
		}
		if !ok {
			return nil, errf(CodeInvalid, 400, "fields")
		}
		set[f] = true
	}
	if !set[FieldItems] {
		return nil, errf(CodeInvalid, 400, "fields.items")
	}
	out := []ShareField{}
	for _, a := range AllShareFields {
		if set[a] {
			out = append(out, a)
		}
	}
	return out, nil
}

func fieldSet(fields []ShareField) map[ShareField]bool {
	m := map[ShareField]bool{}
	for _, f := range fields {
		m[f] = true
	}
	return m
}

// project computes the immutable recipient projection from the snapshot and
// the selected fields only. Nothing else from the session is included.
func (s *Service) project(sess *Session, h *Handoff, snap BriefSnapshot) Projection {
	set := fieldSet(h.Fields)
	p := Projection{Synthetic: true, Language: snap.Language, BriefVersion: snap.Version, SlotID: h.SlotID, Provenance: ProvenanceManual}
	for _, it := range snap.Items {
		if it.Provenance == ProvenanceLiveMeta || it.Provenance == ProvenanceTestDouble {
			p.Provenance = it.Provenance
		}
		pi := ProjectedItem{Kind: it.Kind, Text: it.Wording, Qualifiers: it.Qualifiers}
		if set[FieldQuotes] {
			pi.Quote = it.Quote
		}
		p.Items = append(p.Items, pi)
	}
	if set[FieldUnknowns] {
		p.Unknowns = snap.Unknowns
	}
	if set[FieldOpening] && sess.Opening != nil {
		for _, l := range sess.Opening.Lines {
			p.Opening = append(p.Opening, l.Text)
		}
		p.OpeningProvenance, p.OpeningUserEdited = sess.Opening.Provenance, sess.Opening.UserEdited
	}
	if set[FieldPreferences] && sess.Preferences != nil {
		pref := *sess.Preferences
		pref.Barrier = "" // the user's barrier is private context, not a preference for the recipient
		p.Preferences = &pref
	}
	if set[FieldDisplayName] {
		if sess.Identity.Mode == IdentityPseudonym {
			p.DisplayName = sess.Identity.Pseudonym
		} else {
			p.DisplayName = "anonymous"
		}
	}
	if set[FieldAgeRange] {
		p.AgeRange = "adult_18_plus"
	}
	if set[FieldLanguage] {
		p.Preferred = sess.Language
	}
	return p
}

// shareExpiry is the share's effective end: its own expiry, never beyond the
// owner session's absolute expiry (the idle timeout is suspended while a
// share is active, see Session.IdleDeadline).
func shareExpiry(sess *Session, h *Handoff) time.Time {
	if sess.ExpiresAt.Before(h.ExpiresAt) {
		return sess.ExpiresAt
	}
	return h.ExpiresAt
}

// shareState computes the effective share state at now.
func (s *Service) shareState(h *Handoff, now time.Time) ShareState {
	if h.Share == ShareActive && !now.Before(h.ExpiresAt) {
		return ShareExpired
	}
	return h.Share
}

// endShare ends access and removes every recipient principal for it.
func (s *Service) endShare(sess *Session, h *Handoff, state ShareState, reason string, now time.Time) {
	if h.Share != ShareActive {
		return
	}
	h.Share, h.ShareReason = state, reason
	h.Version++
	h.Timeline = append(h.Timeline, TimelineEvent{Kind: "share_" + string(state), At: now, Actor: actorFor(reason), Detail: reason})
	s.store.UnindexShare(HashToken(h.Token))
	s.store.DeleteRecipientsFor(sess.ID, h.ID)
}

func actorFor(reason string) string {
	switch reason {
	case "revoked_by_user", "superseded":
		return "user"
	}
	return "system"
}

// RevokeInput revokes a share. ExpectedVersion is accepted for
// compatibility and ignored: revoking is a privacy action that must never
// fail because the recipient (or another tab) changed the session.
type RevokeInput struct {
	ExpectedVersion int `json:"expectedVersion"`
}

// RevokeShare blocks all further recipient access, whatever the handoff
// status and the session version. It is idempotent: revoking a share that
// already ended succeeds and returns the current view. Already-viewed
// information cannot be recalled; the UI says so.
func (s *Service) RevokeShare(sessionID, handoffID string, _ RevokeInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		h := sess.handoff(handoffID)
		if h == nil {
			return errf(CodeNotFound, 404, "handoffId")
		}
		now := s.now()
		switch s.shareState(h, now) {
		case ShareActive:
			s.endShare(sess, h, ShareRevoked, "revoked_by_user", now)
			bump(sess)
			s.metrics.add("share_revoked")
		case ShareExpired:
			if h.Share == ShareActive {
				s.endShare(sess, h, ShareExpired, "expired", now)
				bump(sess)
			}
		}
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// CheckInInput records a seeded follow-up check-in.
type CheckInInput struct {
	ExpectedVersion int    `json:"expectedVersion"`
	HandoffID       string `json:"handoffId"`
	OptionID        string `json:"optionId"`
}

// CheckIn records one fixture check-in after a completed simulated
// consultation (once per option per handoff). Not available after a crisis
// route (SPEC V3).
func (s *Service) CheckIn(sessionID string, in CheckInInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if sess.Safety.Interrupted {
			return errCrisis
		}
		h := sess.handoff(in.HandoffID)
		if h == nil {
			return errf(CodeNotFound, 404, "handoffId")
		}
		if h.Status != HandoffCompleted {
			return errf(CodeInvalidTransition, 409, "handoffId")
		}
		if _, ok := s.catalog.CheckIn(in.OptionID); !ok {
			return errf(CodeInvalid, 400, "optionId")
		}
		for _, c := range sess.CheckIns {
			if c.HandoffID == h.ID && c.OptionID == in.OptionID {
				return errf(CodeInvalidTransition, 409, "optionId")
			}
		}
		sess.CheckIns = append(sess.CheckIns, CheckIn{ID: fmt.Sprintf("c%d", len(sess.CheckIns)+1), HandoffID: h.ID, OptionID: in.OptionID, At: s.now()})
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// PreviewInput selects what a share would contain.
type PreviewInput struct {
	ProfessionalID string       `json:"professionalId"`
	SlotID         string       `json:"slotId"`
	Fields         []ShareField `json:"fields"`
}

// PreviewHandoff returns exactly the projection CreateHandoff would store for
// the current approved snapshot, without creating anything (SPEC V5: the
// user sees what the recipient will see before consenting).
func (s *Service) PreviewHandoff(sessionID string, in PreviewInput) (*Projection, error) {
	var out *Projection
	err := s.view(sessionID, func(sess *Session) error {
		snap, err := s.requireApproved(sess)
		if err != nil {
			return err
		}
		prof, ok := s.catalog.Professional(in.ProfessionalID)
		if !ok {
			return errf(CodeInvalid, 400, "professionalId")
		}
		if slotProf, _, ok := s.catalog.Slot(in.SlotID); !ok || slotProf.ID != prof.ID {
			return errf(CodeInvalid, 400, "slotId")
		}
		fields, err := validFields(in.Fields)
		if err != nil {
			return err
		}
		p := s.project(sess, &Handoff{SlotID: in.SlotID, Fields: fields}, snap)
		out = &p
		return nil
	})
	return out, err
}

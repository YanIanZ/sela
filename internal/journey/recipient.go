package journey

import (
	"time"
)

// errAccess is deliberately uniform: wrong token, wrong code, revoked,
// expired, locked and deleted sessions are indistinguishable to a caller
// without the owner's session (no oracle for guessing).
var errAccess = errf(CodeAccessEnded, 403, "")

// RecipientAuthInput presents the share token and the separate access code.
type RecipientAuthInput struct {
	Token string `json:"token"`
	Code  string `json:"code"`
}

// AuthenticateRecipient creates a recipient principal for exactly one
// handoff. The token only locates the grant; the access code proves the
// recipient received it through a second channel. Five wrong codes lock the
// share; the owner must share again.
func (s *Service) AuthenticateRecipient(in RecipientAuthInput) (string, error) {
	if len(in.Token) < 20 || len(in.Token) > 100 || len(in.Code) > 20 {
		return "", errAccess
	}
	ref, ok := s.store.LookupShare(HashToken(in.Token))
	if !ok {
		s.metrics.add("recipient_auth_unknown")
		return "", errAccess
	}
	var principal RecipientSession
	err := s.store.Update(ref.SessionID, func(sess *Session) error {
		h := sess.handoff(ref.HandoffID)
		now := s.now()
		if h == nil || h.Token != in.Token {
			return errAccess
		}
		switch s.shareState(h, now) {
		case ShareActive:
		case ShareExpired:
			s.endShare(sess, h, ShareExpired, "expired", now)
			return errAccess
		default:
			return errAccess
		}
		if !codesEqual(in.Code, h.Code) {
			h.CodeAttempts++
			if h.CodeAttempts >= s.cfg.MaxCodeAttempts {
				s.endShare(sess, h, ShareLocked, "too_many_attempts", now)
				sess.Version++
			}
			s.metrics.add("recipient_auth_failed")
			return errAccess
		}
		exp := now.Add(s.cfg.RecipientTTL)
		if exp.After(h.ExpiresAt) {
			exp = h.ExpiresAt
		}
		principal = RecipientSession{ID: randomToken(32), SessionID: sess.ID, HandoffID: h.ID, ProfessionalID: h.ProfessionalID, CreatedAt: now, ExpiresAt: exp}
		if err := s.store.CreateRecipient(principal); err != nil {
			return errf(CodeCapacity, 503, "")
		}
		opened := false
		for _, e := range h.Timeline {
			opened = opened || e.Kind == "opened"
		}
		if !opened {
			h.Timeline = append(h.Timeline, TimelineEvent{Kind: "opened", At: now, Actor: "recipient"})
			h.Version++
			sess.Version++
		}
		s.metrics.add("recipient_auth_ok")
		return nil
	})
	if err != nil {
		if _, ok := err.(*Error); !ok {
			return "", errAccess
		}
		return "", err
	}
	return principal.ID, nil
}

// RecipientHandoff is the recipient's view of one shared handoff.
type RecipientHandoff struct {
	ID             string          `json:"id"`
	Version        int             `json:"version"`
	ProfessionalID string          `json:"professionalId"`
	Status         HandoffStatus   `json:"status"`
	Projection     Projection      `json:"projection"`
	ExpiresAt      time.Time       `json:"expiresAt"`
	Timeline       []TimelineEvent `json:"timeline"`
	ClarifyReason  string          `json:"clarifyReason,omitempty"`
	DeclineReason  string          `json:"declineReason,omitempty"`
	PlanID         string          `json:"planId,omitempty"`
	Plans          []string        `json:"plans"`
	Synthetic      bool            `json:"synthetic"`
}

// withRecipient resolves the principal and runs fn on its handoff only.
func (s *Service) withRecipient(recipientID string, fn func(sess *Session, h *Handoff, now time.Time) error) error {
	now := s.now()
	r, ok := s.store.Recipient(recipientID, now)
	if !ok {
		return errAccess
	}
	err := s.store.Update(r.SessionID, func(sess *Session) error {
		h := sess.handoff(r.HandoffID)
		if h == nil || h.ProfessionalID != r.ProfessionalID {
			return errAccess
		}
		switch s.shareState(h, now) {
		case ShareActive:
		case ShareExpired:
			s.endShare(sess, h, ShareExpired, "expired", now)
			return errAccess
		default:
			return errAccess
		}
		return fn(sess, h, now)
	})
	if err == ErrNotFound {
		s.store.DeleteRecipient(recipientID)
		return errAccess
	}
	if err == errAccess {
		s.store.DeleteRecipient(recipientID)
	}
	return err
}

func (s *Service) recipientView(h *Handoff) *RecipientHandoff {
	prof, _ := s.catalog.Professional(h.ProfessionalID)
	plans := []string{}
	if prof != nil {
		plans = append(plans, prof.Plans...)
	}
	return &RecipientHandoff{ID: h.ID, Version: h.Version, ProfessionalID: h.ProfessionalID, Status: h.Status, Projection: h.Projection,
		ExpiresAt: h.ExpiresAt, Timeline: append([]TimelineEvent{}, h.Timeline...), ClarifyReason: h.ClarifyReason, DeclineReason: h.DeclineReason,
		PlanID: h.PlanID, Plans: plans, Synthetic: true}
}

// RecipientGet returns the shared projection for the principal's handoff.
func (s *Service) RecipientGet(recipientID string) (*RecipientHandoff, error) {
	var out *RecipientHandoff
	err := s.withRecipient(recipientID, func(_ *Session, h *Handoff, _ time.Time) error {
		out = s.recipientView(h)
		return nil
	})
	return out, err
}

// RecipientActionInput is a scripted fictional-recipient action.
type RecipientActionInput struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Action          string `json:"action"` // acknowledge | clarify | decline | complete
	ReasonID        string `json:"reasonId,omitempty"`
	PlanID          string `json:"planId,omitempty"`
}

// RecipientAct applies a scripted transition. Allowed:
//
//	requested    → acknowledged | clarification_requested | declined
//	acknowledged → completed (scripted consultation; fixture plan only) |
//	               clarification_requested | declined
//
// Nothing is allowed after a crisis route in the owner's session (SPEC V3)
// or once access ended (SPEC V6). The plan must belong to the recipient.
func (s *Service) RecipientAct(recipientID string, in RecipientActionInput) (*RecipientHandoff, error) {
	var out *RecipientHandoff
	err := s.withRecipient(recipientID, func(sess *Session, h *Handoff, now time.Time) error {
		if in.ExpectedVersion != h.Version {
			return errf(CodeStale, 409, "expectedVersion")
		}
		if sess.Safety.Interrupted {
			return errf(CodeInvalidTransition, 409, "")
		}
		allowed := map[HandoffStatus][]string{
			HandoffRequested:    {"acknowledge", "clarify", "decline"},
			HandoffAcknowledged: {"complete", "clarify", "decline"},
		}
		if !oneOf(in.Action, allowed[h.Status]...) {
			return errf(CodeInvalidTransition, 409, "action")
		}
		switch in.Action {
		case "acknowledge":
			h.Status = HandoffAcknowledged
		case "clarify":
			if _, ok := s.catalog.ClarifyReason(in.ReasonID); !ok {
				return errf(CodeInvalid, 400, "reasonId")
			}
			h.Status, h.ClarifyReason = HandoffClarification, in.ReasonID
		case "decline":
			if _, ok := s.catalog.DeclineReason(in.ReasonID); !ok {
				return errf(CodeInvalid, 400, "reasonId")
			}
			h.Status, h.DeclineReason = HandoffDeclined, in.ReasonID
		case "complete":
			prof, _ := s.catalog.Professional(h.ProfessionalID)
			if prof == nil || !oneOf(in.PlanID, prof.Plans...) {
				return errf(CodeInvalid, 400, "planId")
			}
			h.Status, h.PlanID = HandoffCompleted, in.PlanID
		}
		h.Version++
		h.Timeline = append(h.Timeline, TimelineEvent{Kind: string(h.Status), At: now, Actor: "recipient"})
		sess.Version++
		s.metrics.add("handoff_" + string(h.Status))
		out = s.recipientView(h)
		return nil
	})
	return out, err
}

// RecipientLogout ends the principal.
func (s *Service) RecipientLogout(recipientID string) { s.store.DeleteRecipient(recipientID) }

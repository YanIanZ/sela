package journey

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/ledger"
)

func approvedItems(snap BriefSnapshot) []ledger.ApprovedItem {
	out := make([]ledger.ApprovedItem, 0, len(snap.Items))
	for _, it := range snap.Items {
		out = append(out, ledger.ApprovedItem{ID: it.ID, Kind: it.Kind, Text: it.Wording})
	}
	return out
}

func (s *Service) requireApproved(sess *Session) (BriefSnapshot, error) {
	if sess.Safety.Interrupted {
		return BriefSnapshot{}, errCrisis
	}
	snap, ok := approvedSnapshot(sess)
	if !ok {
		return BriefSnapshot{}, errf(CodeApprovalRequired, 409, "")
	}
	return snap, nil
}

// OpeningInput selects who the opening is for.
type OpeningInput struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Recipient       string `json:"recipient"` // professional | trusted_person
}

// TemplateOpening builds deterministic opening lines from the approved brief
// without a model (manual provenance).
func (s *Service) TemplateOpening(sessionID string, in OpeningInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		snap, err := s.requireApproved(sess)
		if err != nil {
			return err
		}
		lines := ledger.TemplateOpening(approvedItems(snap), sess.Language)
		v := 1
		if sess.Opening != nil {
			v = sess.Opening.Version + 1
		}
		sess.Opening = &Opening{Lines: lines, Provenance: ProvenanceManual, BriefVersion: snap.Version, Version: v}
		sess.openingRev++
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// StartOpeningDraft asks the model for opening lines grounded in the
// approved brief. Lines that add content are dropped by the validator.
func (s *Service) StartOpeningDraft(sessionID string, in OpeningInput) (*OpRecord, error) {
	if in.Recipient != "professional" && in.Recipient != "trusted_person" {
		return nil, errf(CodeInvalid, 400, "recipient")
	}
	var snapshot OpRecord
	var rt *opRuntime
	var items []ledger.ApprovedItem
	var lang string
	var briefVersion int
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		snap, err := s.requireApproved(sess)
		if err != nil {
			return err
		}
		r, err := s.startOp(sess, OpOpeningDraft)
		if err != nil {
			return err
		}
		snapshot, items, lang, briefVersion = *r, approvedItems(snap), sess.Language, snap.Version
		bump(sess)
		rt = s.register(sess, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.metrics.add("ai_op_started")
	s.launch(rt, func(ctx context.Context, rt *opRuntime) error {
		req := openingRequest(items, lang, in.Recipient)
		res, err := s.generate(ctx, rt, req, nil)
		if err != nil {
			return err
		}
		rt.emit("status", map[string]any{"state": "validating"})
		draft, perr := ledger.ParseOpening([]byte(res.Text))
		if perr != nil {
			return errf("ai_invalid_output", 502, "")
		}
		result := ledger.ValidateOpening(draft, items, lang)
		prov, model := s.provenance(res)
		if prov == "" {
			return errf("ai_model_mismatch", 502, "")
		}
		if len(result.Lines) == 0 {
			return errf("ai_ungrounded", 422, "")
		}
		var summary map[string]any
		err = s.store.Update(sessionID, func(sess *Session) error {
			cur := sess.op(snapshot.ID)
			if cur == nil || cur.State == OpCanceled {
				return errStaleResult
			}
			c := sess.consent(ConsentAI)
			snap, ok := approvedSnapshot(sess)
			// Lines the user wrote or chose while the model ran (any opening
			// change) are never overwritten.
			if sess.Safety.Interrupted || !c.Granted || c.Revision != snapshot.ConsentRevision || !ok || snap.Version != briefVersion ||
				sess.openingRev != snapshot.OpeningRevision {
				return errStaleResult
			}
			v := 1
			if sess.Opening != nil {
				v = sess.Opening.Version + 1
			}
			sess.Opening = &Opening{Lines: result.Lines, Provenance: prov, Model: model, BriefVersion: briefVersion, Version: v}
			sess.openingRev++
			cur.State, cur.Provenance, cur.EndedAt = OpSucceeded, prov, s.now()
			bump(sess)
			rejected := map[string]int{}
			for _, r := range result.Rejected {
				rejected[string(r.Reason)]++
			}
			summary = map[string]any{"lines": len(result.Lines), "rejected": rejected, "provenance": prov, "model": model, "latencyMs": res.Latency.Milliseconds(), "sessionVersion": sess.Version}
			return nil
		})
		if errors.Is(err, ErrNotFound) {
			return errStaleResult
		}
		if err != nil {
			return err
		}
		s.metrics.add("ai_op_succeeded")
		rt.emit("done", summary)
		return nil
	})
	return &snapshot, nil
}

// EditOpeningInput replaces the lines with the user's own words.
type EditOpeningInput struct {
	ExpectedVersion int      `json:"expectedVersion"`
	Lines           []string `json:"lines"`
}

// EditOpening stores user-written lines. They are the user's own words, so
// they are not vocabulary-restricted, but they pass the safety gate (before
// any other check) and the contact-detail check.
func (s *Service) EditOpening(sessionID string, in EditOpeningInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if sess.Safety.Interrupted {
			return errCrisis
		}
		texts := make([]string, 0, len(in.Lines))
		for _, l := range in.Lines {
			texts = append(texts, strings.TrimSpace(l))
		}
		if s.screen(sess, texts...) {
			bump(sess)
			view = s.buildView(sess)
			return nil
		}
		if len(texts) < ledger.MinOpeningLines || len(texts) > ledger.MaxOpeningLines {
			return errf(CodeInvalid, 400, "lines")
		}
		lines := make([]ledger.OpeningLine, 0, len(texts))
		for _, t := range texts {
			if t == "" || utf8.RuneCountInString(t) > ledger.MaxOpeningRunes || containsControl(t) {
				return errf(CodeInvalid, 400, "lines")
			}
			lines = append(lines, ledger.OpeningLine{Text: t, BasedOn: []string{"user"}})
		}
		snap, err := s.requireApproved(sess)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if containsContactDetails(l.Text) {
				return errf(CodeContactDetails, 422, "lines")
			}
		}
		v := 1
		if sess.Opening != nil {
			v = sess.Opening.Version + 1
		}
		sess.Opening = &Opening{Lines: lines, Provenance: ProvenanceManual, BriefVersion: snap.Version, UserEdited: true, Version: v}
		sess.openingRev++
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// PracticeInput is one rehearsal attempt. It is not stored.
type PracticeInput struct {
	Attempt string `json:"attempt"`
}

// PracticeResult is non-judgemental coverage feedback (never a score).
type PracticeResult struct {
	Coverage ledger.Coverage `json:"coverage"`
	Safety   string          `json:"safety"`
}

// Practice gives deterministic feedback on which approved items a rehearsal
// mentions. The attempt passes the safety gate (before any other check) and
// is discarded afterwards.
func (s *Service) Practice(sessionID string, in PracticeInput) (*PracticeResult, error) {
	t := strings.TrimSpace(in.Attempt)
	var out *PracticeResult
	err := s.update(sessionID, func(sess *Session) error {
		if sess.Safety.Interrupted {
			return errCrisis
		}
		a := s.assessText(sess, t)
		if a.Interrupts() {
			bump(sess)
			out = &PracticeResult{Safety: string(a.Level), Coverage: ledger.Coverage{Mentioned: []string{}, NotMentioned: []string{}}}
			return nil
		}
		if t == "" || utf8.RuneCountInString(t) > ledger.MaxPracticeRunes || containsControl(t) {
			return errf(CodeInvalid, 400, "attempt")
		}
		snap, err := s.requireApproved(sess)
		if err != nil {
			return err
		}
		out = &PracticeResult{Coverage: ledger.PracticeCoverage(t, approvedItems(snap)), Safety: string(a.Level)}
		return nil
	})
	return out, err
}

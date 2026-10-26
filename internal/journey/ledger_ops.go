package journey

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/ledger"
)

// changeLedger records a content change: new version, approval cleared
// (SPEC V5: editing a brief invalidates approval).
func changeLedger(sess *Session) {
	sess.Ledger.Version++
	sess.Ledger.Approved = nil
	touchLedger(sess)
}

// touchLedger records a ledger-state change that keeps the version
// (approval, skip). Running AI drafts then discard their result.
func touchLedger(sess *Session) { sess.ledgerRev++ }

func (l *Ledger) newItemID() string {
	l.nextItem++
	return fmt.Sprintf("i%d", l.nextItem)
}

func itemClaims(items []Item) []ledger.Claim {
	out := make([]ledger.Claim, 0, len(items))
	for _, it := range items {
		out = append(out, ledger.Claim{SegmentID: it.SegmentID, Quote: it.Quote, Kind: it.Kind, Wording: it.Wording, Qualifiers: it.Qualifiers})
	}
	return out
}

// recompute refreshes unknowns and the suggested clarification. A skipped or
// answered field is never suggested again.
func recompute(l *Ledger) {
	l.Unknowns = ledger.ComputeUnknowns(itemClaims(l.Items))
	if l.Unknowns == nil {
		l.Unknowns = []ledger.UnknownField{}
	}
	if l.Clarification != "" && (!containsField(l.Unknowns, l.Clarification) || l.asked(l.Clarification)) {
		l.Clarification = ""
	}
	if l.Clarification == "" {
		for _, f := range l.Unknowns {
			if !l.asked(f) {
				l.Clarification = f
				break
			}
		}
	}
}

// asked reports whether the user already skipped or answered the field.
func (l *Ledger) asked(f ledger.UnknownField) bool {
	return containsField(l.Skipped, f) || containsField(l.answered, f)
}

func containsField(list []ledger.UnknownField, f ledger.UnknownField) bool {
	for _, x := range list {
		if x == f {
			return true
		}
	}
	return false
}

func (s *Service) requireLedger(sess *Session) error {
	if sess.Safety.Interrupted {
		return errCrisis
	}
	if sess.Story == nil || sess.Ledger == nil {
		return errf(CodeNoStory, 409, "")
	}
	return nil
}

func segmentByID(story *Story, id string) (ledger.Segment, bool) {
	for _, seg := range story.Segments {
		if seg.ID == id {
			return seg, true
		}
	}
	return ledger.Segment{}, false
}

func validKind(k ledger.Kind) bool {
	for _, x := range ledger.AllKinds {
		if x == k {
			return true
		}
	}
	return false
}

// AddItemInput adds a manual item from one whole story segment.
type AddItemInput struct {
	ExpectedVersion int         `json:"expectedVersion"`
	SegmentID       string      `json:"segmentId"`
	Kind            ledger.Kind `json:"kind"`
}

// AddManualItem is the manual (no-AI) path: the user picks one of their own
// sentences and labels it. The item is the user's words verbatim.
func (s *Service) AddManualItem(sessionID string, in AddItemInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if err := s.requireLedger(sess); err != nil {
			return err
		}
		if !validKind(in.Kind) {
			return errf(CodeInvalid, 400, "kind")
		}
		seg, ok := segmentByID(sess.Story, in.SegmentID)
		if !ok {
			return errf(CodeInvalid, 400, "segmentId")
		}
		l := sess.Ledger
		for _, it := range l.Items {
			if it.SegmentID == seg.ID && it.Start == 0 && it.End == len(strings.TrimSpace(seg.Text)) {
				return errf(CodeInvalid, 409, "segmentId")
			}
		}
		if len(l.Items) >= MaxLedgerItems {
			return errLimit("items")
		}
		c := ledger.ManualClaim(seg, in.Kind)
		l.Items = append(l.Items, Item{
			ID: l.newItemID(), Kind: c.Kind, SegmentID: c.SegmentID, Quote: c.Quote, Start: c.Start, End: c.End,
			Wording: c.Wording, WordingOrigin: WordingQuote, Qualifiers: c.Qualifiers, Status: ItemConfirmed, Provenance: ProvenanceManual,
		})
		changeLedger(sess)
		recompute(l)
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// ItemAction is a review decision on one ledger item.
type ItemAction string

const (
	ActionConfirm  ItemAction = "confirm"   // keep Sela's wording as is
	ActionUseQuote ItemAction = "use_quote" // replace wording with my own words
	ActionEdit     ItemAction = "edit"      // replace wording with my edit
	ActionRemove   ItemAction = "remove"    // delete the item
	ActionSetKind  ItemAction = "set_kind"  // correct the label
)

// ItemInput applies one review action.
type ItemInput struct {
	ExpectedVersion int         `json:"expectedVersion"`
	Action          ItemAction  `json:"action"`
	Wording         string      `json:"wording,omitempty"`
	Kind            ledger.Kind `json:"kind,omitempty"`
}

// ReviewItem applies a correction, confirmation or deletion. Any content
// change creates a new ledger version and clears approval. A user edit is
// the user's own text and passes the safety gate before anything else.
func (s *Service) ReviewItem(sessionID, itemID string, in ItemInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if sess.Safety.Interrupted {
			return errCrisis
		}
		w := strings.TrimSpace(in.Wording)
		if w != "" && s.screen(sess, w) {
			bump(sess)
			view = s.buildView(sess)
			return nil
		}
		if err := s.requireLedger(sess); err != nil {
			return err
		}
		l := sess.Ledger
		idx := -1
		for i := range l.Items {
			if l.Items[i].ID == itemID {
				idx = i
			}
		}
		if idx < 0 {
			return errf(CodeNotFound, 404, "itemId")
		}
		it := &l.Items[idx]
		switch in.Action {
		case ActionConfirm:
			if it.Status == ItemConfirmed {
				view = s.buildView(sess)
				return nil
			}
			it.Status = ItemConfirmed
		case ActionUseQuote:
			it.Wording, it.WordingOrigin, it.Status = it.Quote, WordingQuote, ItemConfirmed
		case ActionEdit:
			if err := ledger.ValidateUserWording(w); err != nil {
				return errf(CodeInvalid, 400, "wording")
			}
			if containsContactDetails(w) {
				return errf(CodeContactDetails, 422, "wording")
			}
			it.Wording, it.WordingOrigin, it.Status = w, WordingUser, ItemConfirmed
		case ActionRemove:
			l.Items = append(l.Items[:idx], l.Items[idx+1:]...)
		case ActionSetKind:
			if !validKind(in.Kind) {
				return errf(CodeInvalid, 400, "kind")
			}
			it.Kind = in.Kind
		default:
			return errf(CodeInvalid, 400, "action")
		}
		changeLedger(sess)
		recompute(l)
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// ClarifyInput answers or skips the suggested clarification.
type ClarifyInput struct {
	ExpectedVersion int                 `json:"expectedVersion"`
	Field           ledger.UnknownField `json:"field"`
	Answer          string              `json:"answer,omitempty"`
	Skip            bool                `json:"skip,omitempty"`
}

var fieldKind = map[ledger.UnknownField]ledger.Kind{
	ledger.UnknownConcern:    ledger.KindExperience,
	ledger.UnknownDuration:   ledger.KindDuration,
	ledger.UnknownImpact:     ledger.KindImpact,
	ledger.UnknownGoal:       ledger.KindGoal,
	ledger.UnknownPreference: ledger.KindPreference,
}

// Clarify records the user's answer to one clarification question (as a new
// user-statement segment and a confirmed item) or a "not now / I don't
// know" choice. Only a field that is still unknown and was neither answered
// nor skipped can be answered or skipped (otherwise invalid_transition), so
// a skipped field is never asked again and the clarification text sent to
// the model stays bounded (limit_reached beyond the item and story caps).
func (s *Service) Clarify(sessionID string, in ClarifyInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if sess.Safety.Interrupted {
			return errCrisis
		}
		answer := strings.TrimSpace(in.Answer)
		if answer != "" && s.screen(sess, answer) {
			bump(sess)
			view = s.buildView(sess)
			return nil
		}
		if err := s.requireLedger(sess); err != nil {
			return err
		}
		kind, ok := fieldKind[in.Field]
		if !ok {
			return errf(CodeInvalid, 400, "field")
		}
		l := sess.Ledger
		if !containsField(l.Unknowns, in.Field) || l.asked(in.Field) {
			return errf(CodeInvalidTransition, 409, "field")
		}
		if in.Skip {
			l.Skipped = append(l.Skipped, in.Field)
			touchLedger(sess)
			recompute(l)
			bump(sess)
			view = s.buildView(sess)
			return nil
		}
		if n := utf8.RuneCountInString(answer); n < 2 || n > ledger.MaxEditRunes || containsControl(answer) {
			return errf(CodeInvalid, 400, "answer")
		}
		if containsContactDetails(answer) {
			return errf(CodeContactDetails, 422, "answer")
		}
		next := len(sess.Story.Segments) + 1
		segs := ledger.SplitSegments(answer, "clarification", next)
		if len(segs) == 0 {
			return errf(CodeInvalid, 400, "answer")
		}
		if len(l.Items) >= MaxLedgerItems {
			return errLimit("items")
		}
		if len(sess.Story.Segments)+len(segs) > MaxStorySegments || storyRunes(sess.Story.Segments)+storyRunes(segs) > MaxStoryRunes {
			return errLimit("answer")
		}
		l.answered = append(l.answered, in.Field)
		sess.Story.Segments = append(sess.Story.Segments, segs...)
		sess.Story.Version++
		// The answer as a whole becomes one item quoting its first segment;
		// further sentences remain selectable in the manual path.
		c := ledger.ManualClaim(segs[0], kind)
		l.Items = append(l.Items, Item{
			ID: l.newItemID(), Kind: kind, SegmentID: c.SegmentID, Quote: c.Quote, Start: c.Start, End: c.End,
			Wording: c.Wording, WordingOrigin: WordingQuote, Qualifiers: c.Qualifiers, Status: ItemConfirmed, Provenance: ProvenanceManual,
		})
		l.StoryVersion = sess.Story.Version
		s.cancelKind(sess, OpLedgerDraft, "story_changed")
		changeLedger(sess)
		recompute(l)
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// ApproveInput approves one exact ledger version.
type ApproveInput struct {
	ExpectedVersion int `json:"expectedVersion"`
	LedgerVersion   int `json:"ledgerVersion"`
}

// briefHash is the canonical content hash bound to an approval.
func briefHash(l *Ledger) string {
	type hashed struct {
		Version  int                   `json:"v"`
		Items    []Item                `json:"items"`
		Unknowns []ledger.UnknownField `json:"unknowns"`
	}
	return hashJSON(hashed{Version: l.Version, Items: l.Items, Unknowns: l.Unknowns})
}

// Approve binds approval to the exact current ledger version. Every item must
// have been reviewed (confirmed); unreviewed AI wording cannot be approved.
func (s *Service) Approve(sessionID string, in ApproveInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if err := s.requireLedger(sess); err != nil {
			return err
		}
		l := sess.Ledger
		if in.LedgerVersion != l.Version {
			return errf(CodeStale, 409, "ledgerVersion")
		}
		if len(l.Items) == 0 {
			return errf(CodeReviewIncomplete, 409, "items")
		}
		for _, it := range l.Items {
			if it.Status != ItemConfirmed {
				return errf(CodeReviewIncomplete, 409, "items")
			}
		}
		now := s.now()
		hash := briefHash(l)
		l.Approved = &Approval{Version: l.Version, Hash: hash, At: now}
		touchLedger(sess)
		snap := BriefSnapshot{Version: l.Version, Hash: hash, Language: sess.Language, Items: append([]Item{}, l.Items...), Unknowns: append([]ledger.UnknownField{}, l.Unknowns...), At: now}
		sess.Snapshots = append(sess.Snapshots, snap)
		if len(sess.Snapshots) > 8 {
			sess.Snapshots = sess.Snapshots[len(sess.Snapshots)-8:]
		}
		bump(sess)
		s.metrics.add("brief_approved")
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// approvedSnapshot returns the snapshot for the current approval, if valid.
func approvedSnapshot(sess *Session) (BriefSnapshot, bool) {
	if sess.Ledger == nil || sess.Ledger.Approved == nil || sess.Ledger.Approved.Version != sess.Ledger.Version {
		return BriefSnapshot{}, false
	}
	for i := len(sess.Snapshots) - 1; i >= 0; i-- {
		if sess.Snapshots[i].Version == sess.Ledger.Approved.Version && sess.Snapshots[i].Hash == sess.Ledger.Approved.Hash {
			return sess.Snapshots[i], true
		}
	}
	return BriefSnapshot{}, false
}

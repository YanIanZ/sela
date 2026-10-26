package journey

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/ledger"
)

// changeLedger records a content change: new version, approval cleared
// (SPEC V5: editing a brief invalidates approval).
func changeLedger(l *Ledger) {
	l.Version++
	l.Approved = nil
}

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

// recompute refreshes unknowns and the suggested clarification.
func recompute(l *Ledger) {
	l.Unknowns = ledger.ComputeUnknowns(itemClaims(l.Items))
	if l.Unknowns == nil {
		l.Unknowns = []ledger.UnknownField{}
	}
	if l.Clarification != "" && (!containsField(l.Unknowns, l.Clarification) || containsField(l.Skipped, l.Clarification)) {
		l.Clarification = ""
	}
	if l.Clarification == "" {
		for _, f := range l.Unknowns {
			if !containsField(l.Skipped, f) {
				l.Clarification = f
				break
			}
		}
	}
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
		if len(l.Items) >= ledger.MaxClaims*2 {
			return errf(CodeInvalid, 409, "items")
		}
		c := ledger.ManualClaim(seg, in.Kind)
		l.Items = append(l.Items, Item{
			ID: l.newItemID(), Kind: c.Kind, SegmentID: c.SegmentID, Quote: c.Quote, Start: c.Start, End: c.End,
			Wording: c.Wording, WordingOrigin: WordingQuote, Qualifiers: c.Qualifiers, Status: ItemConfirmed, Provenance: ProvenanceManual,
		})
		changeLedger(l)
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
// the user's own text and passes the safety gate first.
func (s *Service) ReviewItem(sessionID, itemID string, in ItemInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
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
			w := strings.TrimSpace(in.Wording)
			if err := ledger.ValidateUserWording(w); err != nil {
				return errf(CodeInvalid, 400, "wording")
			}
			if containsContactDetails(w) {
				return errf(CodeContactDetails, 422, "wording")
			}
			if a := s.assessText(sess, w); a.Interrupts() {
				bump(sess)
				view = s.buildView(sess)
				return nil
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
		changeLedger(l)
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
// know" choice. Skipping is always allowed and is never asked again.
func (s *Service) Clarify(sessionID string, in ClarifyInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if err := s.requireLedger(sess); err != nil {
			return err
		}
		kind, ok := fieldKind[in.Field]
		if !ok {
			return errf(CodeInvalid, 400, "field")
		}
		l := sess.Ledger
		if in.Skip {
			if !containsField(l.Skipped, in.Field) {
				l.Skipped = append(l.Skipped, in.Field)
			}
			recompute(l)
			bump(sess)
			view = s.buildView(sess)
			return nil
		}
		answer := strings.TrimSpace(in.Answer)
		if n := utf8.RuneCountInString(answer); n < 2 || n > ledger.MaxEditRunes || containsControl(answer) {
			return errf(CodeInvalid, 400, "answer")
		}
		if a := s.assessText(sess, answer); a.Interrupts() {
			sess.Story, sess.Ledger, sess.Opening = nil, nil, nil
			bump(sess)
			view = s.buildView(sess)
			return nil
		}
		if containsContactDetails(answer) {
			return errf(CodeContactDetails, 422, "answer")
		}
		next := len(sess.Story.Segments) + 1
		segs := ledger.SplitSegments(answer, "clarification", next)
		if len(segs) == 0 {
			return errf(CodeInvalid, 400, "answer")
		}
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
		changeLedger(l)
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
